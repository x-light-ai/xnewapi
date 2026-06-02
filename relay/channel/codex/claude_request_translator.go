package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertClaudeRequestToCodex transforms a Claude Messages request into the
// Codex Responses payload used by Codex CLI-compatible upstreams.
func ConvertClaudeRequestToCodex(modelName string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := inputRawJSON

	template := []byte(`{"model":"","instructions":"","input":[]}`)

	rootResult := gjson.ParseBytes(rawJSON)
	toolNameMap := buildReverseMapFromClaudeOriginalToShort(rawJSON)
	template, _ = sjson.SetBytes(template, "model", modelName)

	systemsResult := rootResult.Get("system")
	if systemsResult.Exists() {
		message := []byte(`{"type":"message","role":"developer","content":[]}`)
		contentIndex := 0

		appendSystemText := func(text string) {
			if text == "" || isClaudeCodeAttributionSystemText(text) {
				return
			}

			message, _ = sjson.SetBytes(message, fmt.Sprintf("content.%d.type", contentIndex), "input_text")
			message, _ = sjson.SetBytes(message, fmt.Sprintf("content.%d.text", contentIndex), text)
			contentIndex++
		}

		if systemsResult.Type == gjson.String {
			appendSystemText(systemsResult.String())
		} else if systemsResult.IsArray() {
			systemResults := systemsResult.Array()
			for i := 0; i < len(systemResults); i++ {
				systemResult := systemResults[i]
				if systemResult.Get("type").String() == "text" {
					appendSystemText(systemResult.Get("text").String())
				}
			}
		}

		if contentIndex > 0 {
			template, _ = sjson.SetRawBytes(template, "input.-1", message)
		}
	}

	messagesResult := rootResult.Get("messages")
	if messagesResult.IsArray() {
		messageResults := messagesResult.Array()

		for i := 0; i < len(messageResults); i++ {
			messageResult := messageResults[i]
			messageRole := messageResult.Get("role").String()
			if messageRole == "system" {
				messageRole = "developer"
			}

			newMessage := func() []byte {
				msg := []byte(`{"type":"message","role":"","content":[]}`)
				msg, _ = sjson.SetBytes(msg, "role", messageRole)
				return msg
			}

			message := newMessage()
			contentIndex := 0
			hasContent := false

			flushMessage := func() {
				if hasContent {
					template, _ = sjson.SetRawBytes(template, "input.-1", message)
					message = newMessage()
					contentIndex = 0
					hasContent = false
				}
			}

			appendTextContent := func(text string) {
				partType := "input_text"
				if messageRole == "assistant" {
					partType = "output_text"
				}
				message, _ = sjson.SetBytes(message, fmt.Sprintf("content.%d.type", contentIndex), partType)
				message, _ = sjson.SetBytes(message, fmt.Sprintf("content.%d.text", contentIndex), text)
				contentIndex++
				hasContent = true
			}

			appendImageContent := func(dataURL string) {
				message, _ = sjson.SetBytes(message, fmt.Sprintf("content.%d.type", contentIndex), "input_image")
				message, _ = sjson.SetBytes(message, fmt.Sprintf("content.%d.image_url", contentIndex), dataURL)
				contentIndex++
				hasContent = true
			}

			appendReasoningContent := func(part gjson.Result) {
				if messageRole != "assistant" {
					return
				}

				signature, ok := compatibleGPTSignature(part.Get("signature").String())
				if !ok {
					return
				}

				flushMessage()
				reasoningItem := []byte(`{"type":"reasoning","summary":[],"content":null}`)
				reasoningItem, _ = sjson.SetBytes(reasoningItem, "encrypted_content", signature)
				template, _ = sjson.SetRawBytes(template, "input.-1", reasoningItem)
			}

			messageContentsResult := messageResult.Get("content")
			if messageContentsResult.IsArray() {
				messageContentResults := messageContentsResult.Array()
				for j := 0; j < len(messageContentResults); j++ {
					messageContentResult := messageContentResults[j]
					contentType := messageContentResult.Get("type").String()

					switch contentType {
					case "text":
						appendTextContent(messageContentResult.Get("text").String())
					case "thinking":
						appendReasoningContent(messageContentResult)
					case "image":
						sourceResult := messageContentResult.Get("source")
						if sourceResult.Exists() {
							data := sourceResult.Get("data").String()
							if data == "" {
								data = sourceResult.Get("base64").String()
							}
							if data != "" {
								mediaType := sourceResult.Get("media_type").String()
								if mediaType == "" {
									mediaType = sourceResult.Get("mime_type").String()
								}
								if mediaType == "" {
									mediaType = "application/octet-stream"
								}
								dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, data)
								appendImageContent(dataURL)
							}
						}
					case "tool_use":
						flushMessage()
						functionCallMessage := []byte(`{"type":"function_call"}`)
						functionCallMessage, _ = sjson.SetBytes(functionCallMessage, "call_id", shortenCodexCallIDIfNeeded(messageContentResult.Get("id").String()))
						{
							name := messageContentResult.Get("name").String()
							if short, ok := toolNameMap[name]; ok {
								name = short
							} else {
								name = shortenNameIfNeeded(name)
							}
							functionCallMessage, _ = sjson.SetBytes(functionCallMessage, "name", name)
						}
						functionCallMessage, _ = sjson.SetBytes(functionCallMessage, "arguments", messageContentResult.Get("input").Raw)
						template, _ = sjson.SetRawBytes(template, "input.-1", functionCallMessage)
					case "tool_result":
						flushMessage()
						functionCallOutputMessage := []byte(`{"type":"function_call_output"}`)
						functionCallOutputMessage, _ = sjson.SetBytes(functionCallOutputMessage, "call_id", shortenCodexCallIDIfNeeded(messageContentResult.Get("tool_use_id").String()))

						contentResult := messageContentResult.Get("content")
						if contentResult.IsArray() {
							toolResultContentIndex := 0
							toolResultContent := []byte(`[]`)
							contentResults := contentResult.Array()
							for k := 0; k < len(contentResults); k++ {
								toolResultContentType := contentResults[k].Get("type").String()
								if toolResultContentType == "image" {
									sourceResult := contentResults[k].Get("source")
									if sourceResult.Exists() {
										data := sourceResult.Get("data").String()
										if data == "" {
											data = sourceResult.Get("base64").String()
										}
										if data != "" {
											mediaType := sourceResult.Get("media_type").String()
											if mediaType == "" {
												mediaType = sourceResult.Get("mime_type").String()
											}
											if mediaType == "" {
												mediaType = "application/octet-stream"
											}
											dataURL := fmt.Sprintf("data:%s;base64,%s", mediaType, data)

											toolResultContent, _ = sjson.SetBytes(toolResultContent, fmt.Sprintf("%d.type", toolResultContentIndex), "input_image")
											toolResultContent, _ = sjson.SetBytes(toolResultContent, fmt.Sprintf("%d.image_url", toolResultContentIndex), dataURL)
											toolResultContentIndex++
										}
									}
								} else if toolResultContentType == "text" {
									toolResultContent, _ = sjson.SetBytes(toolResultContent, fmt.Sprintf("%d.type", toolResultContentIndex), "input_text")
									toolResultContent, _ = sjson.SetBytes(toolResultContent, fmt.Sprintf("%d.text", toolResultContentIndex), contentResults[k].Get("text").String())
									toolResultContentIndex++
								}
							}
							if toolResultContentIndex > 0 {
								functionCallOutputMessage, _ = sjson.SetRawBytes(functionCallOutputMessage, "output", toolResultContent)
							} else {
								functionCallOutputMessage, _ = sjson.SetBytes(functionCallOutputMessage, "output", messageContentResult.Get("content").String())
							}
						} else {
							functionCallOutputMessage, _ = sjson.SetBytes(functionCallOutputMessage, "output", messageContentResult.Get("content").String())
						}

						template, _ = sjson.SetRawBytes(template, "input.-1", functionCallOutputMessage)
					}
				}
				flushMessage()
			} else if messageContentsResult.Type == gjson.String {
				appendTextContent(messageContentsResult.String())
				flushMessage()
			}
		}

	}

	toolsResult := rootResult.Get("tools")
	if toolsResult.IsArray() {
		template, _ = sjson.SetRawBytes(template, "tools", []byte(`[]`))
		webSearchToolNames := buildClaudeWebSearchToolNameSet(toolsResult)
		template, _ = sjson.SetRawBytes(template, "tool_choice", convertClaudeToolChoiceToCodex(rootResult.Get("tool_choice"), toolNameMap, webSearchToolNames))
		toolResults := toolsResult.Array()
		for i := 0; i < len(toolResults); i++ {
			toolResult := toolResults[i]
			if isClaudeWebSearchToolType(toolResult.Get("type").String()) {
				template, _ = sjson.SetRawBytes(template, "tools.-1", convertClaudeWebSearchToolToCodex(toolResult))
				continue
			}
			tool := []byte(toolResult.Raw)
			tool, _ = sjson.SetBytes(tool, "type", "function")
			if v := toolResult.Get("name"); v.Exists() {
				name := v.String()
				if short, ok := toolNameMap[name]; ok {
					name = short
				} else {
					name = shortenNameIfNeeded(name)
				}
				tool, _ = sjson.SetBytes(tool, "name", name)
			}
			tool, _ = sjson.SetRawBytes(tool, "parameters", []byte(normalizeToolParameters(toolResult.Get("input_schema").Raw)))
			tool, _ = sjson.DeleteBytes(tool, "input_schema")
			tool, _ = sjson.DeleteBytes(tool, "parameters.$schema")
			tool, _ = sjson.DeleteBytes(tool, "cache_control")
			tool, _ = sjson.DeleteBytes(tool, "defer_loading")
			tool, _ = sjson.SetBytes(tool, "strict", false)
			template, _ = sjson.SetRawBytes(template, "tools.-1", tool)
		}
	}

	parallelToolCalls := true
	if disableParallelToolUse := rootResult.Get("tool_choice.disable_parallel_tool_use"); disableParallelToolUse.Exists() {
		parallelToolCalls = !disableParallelToolUse.Bool()
	}

	template, _ = sjson.SetBytes(template, "parallel_tool_calls", parallelToolCalls)

	reasoningEffort := "medium"
	if thinkingConfig := rootResult.Get("thinking"); thinkingConfig.Exists() && thinkingConfig.IsObject() {
		switch thinkingConfig.Get("type").String() {
		case "enabled":
			if budgetTokens := thinkingConfig.Get("budget_tokens"); budgetTokens.Exists() {
				budget := int(budgetTokens.Int())
				if effort, ok := convertBudgetToLevel(budget); ok && effort != "" {
					reasoningEffort = effort
				}
			}
		case "adaptive", "auto":
			effort := ""
			if v := rootResult.Get("output_config.effort"); v.Exists() && v.Type == gjson.String {
				effort = strings.ToLower(strings.TrimSpace(v.String()))
			}
			if effort != "" {
				reasoningEffort = effort
			} else {
				reasoningEffort = "xhigh"
			}
		case "disabled":
			if effort, ok := convertBudgetToLevel(0); ok && effort != "" {
				reasoningEffort = effort
			}
		}
	}
	template, _ = sjson.SetBytes(template, "reasoning.effort", reasoningEffort)
	template, _ = sjson.SetBytes(template, "reasoning.summary", "auto")
	template, _ = sjson.SetBytes(template, "stream", true)
	template, _ = sjson.SetBytes(template, "store", false)
	template, _ = sjson.SetBytes(template, "include", []string{"reasoning.encrypted_content"})

	return template
}

func shortenCodexCallIDIfNeeded(id string) string {
	const limit = 64
	if len(id) <= limit {
		return id
	}

	sum := sha256.Sum256([]byte(id))
	suffix := "_" + hex.EncodeToString(sum[:8])
	prefixLen := limit - len(suffix)
	if prefixLen <= 0 {
		return suffix[len(suffix)-limit:]
	}
	return id[:prefixLen] + suffix
}

func isClaudeWebSearchToolType(toolType string) bool {
	return toolType == "web_search_20250305" || toolType == "web_search_20260209"
}

func buildClaudeWebSearchToolNameSet(tools gjson.Result) map[string]struct{} {
	names := map[string]struct{}{}
	if !tools.IsArray() {
		return names
	}

	tools.ForEach(func(_, tool gjson.Result) bool {
		toolType := tool.Get("type").String()
		if !isClaudeWebSearchToolType(toolType) {
			return true
		}

		if name := tool.Get("name").String(); name != "" {
			names[name] = struct{}{}
		}
		return true
	})

	return names
}

func convertClaudeToolChoiceToCodex(toolChoice gjson.Result, toolNameMap map[string]string, webSearchToolNames map[string]struct{}) []byte {
	if !toolChoice.Exists() || toolChoice.Type == gjson.Null {
		return []byte(`"auto"`)
	}

	choiceType := toolChoice.Get("type").String()
	if choiceType == "" && toolChoice.Type == gjson.String {
		choiceType = toolChoice.String()
	}

	switch choiceType {
	case "auto", "":
		return []byte(`"auto"`)
	case "any":
		return []byte(`"required"`)
	case "none":
		return []byte(`"none"`)
	case "tool":
		name := toolChoice.Get("name").String()
		if _, ok := webSearchToolNames[name]; ok {
			return []byte(`{"type":"web_search"}`)
		}
		if short, ok := toolNameMap[name]; ok {
			name = short
		} else {
			name = shortenNameIfNeeded(name)
		}
		if name == "" {
			return []byte(`"auto"`)
		}

		choice := []byte(`{"type":"function","name":""}`)
		choice, _ = sjson.SetBytes(choice, "name", name)
		return choice
	default:
		return []byte(`"auto"`)
	}
}

func convertClaudeWebSearchToolToCodex(tool gjson.Result) []byte {
	out := []byte(`{"type":"web_search"}`)
	if allowedDomains := tool.Get("allowed_domains"); allowedDomains.Exists() && allowedDomains.IsArray() {
		out, _ = sjson.SetRawBytes(out, "filters.allowed_domains", []byte(allowedDomains.Raw))
	}
	if userLocation := tool.Get("user_location"); userLocation.Exists() && userLocation.IsObject() {
		out, _ = sjson.SetRawBytes(out, "user_location", []byte(userLocation.Raw))
	}
	return out
}

func shortenNameIfNeeded(name string) string {
	const limit = 64
	if len(name) <= limit {
		return name
	}
	if strings.HasPrefix(name, "mcp__") {
		idx := strings.LastIndex(name, "__")
		if idx > 0 {
			cand := "mcp__" + name[idx+2:]
			if len(cand) > limit {
				return cand[:limit]
			}
			return cand
		}
	}
	return name[:limit]
}

func buildShortNameMap(names []string) map[string]string {
	const limit = 64
	used := map[string]struct{}{}
	m := map[string]string{}

	baseCandidate := func(n string) string {
		if len(n) <= limit {
			return n
		}
		if strings.HasPrefix(n, "mcp__") {
			idx := strings.LastIndex(n, "__")
			if idx > 0 {
				cand := "mcp__" + n[idx+2:]
				if len(cand) > limit {
					cand = cand[:limit]
				}
				return cand
			}
		}
		return n[:limit]
	}

	makeUnique := func(cand string) string {
		if _, ok := used[cand]; !ok {
			return cand
		}
		base := cand
		for i := 1; ; i++ {
			suffix := "_" + strconv.Itoa(i)
			allowed := limit - len(suffix)
			if allowed < 0 {
				allowed = 0
			}
			tmp := base
			if len(tmp) > allowed {
				tmp = tmp[:allowed]
			}
			tmp = tmp + suffix
			if _, ok := used[tmp]; !ok {
				return tmp
			}
		}
	}

	for _, n := range names {
		cand := baseCandidate(n)
		uniq := makeUnique(cand)
		used[uniq] = struct{}{}
		m[n] = uniq
	}
	return m
}

func buildReverseMapFromClaudeOriginalToShort(original []byte) map[string]string {
	tools := gjson.GetBytes(original, "tools")
	m := map[string]string{}
	if !tools.IsArray() {
		return m
	}
	var names []string
	arr := tools.Array()
	for i := 0; i < len(arr); i++ {
		n := arr[i].Get("name").String()
		if n != "" {
			names = append(names, n)
		}
	}
	if len(names) > 0 {
		m = buildShortNameMap(names)
	}
	return m
}

func normalizeToolParameters(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || !gjson.Valid(raw) {
		return `{"type":"object","properties":{}}`
	}
	result := gjson.Parse(raw)
	schema := []byte(raw)
	schemaType := result.Get("type").String()
	if schemaType == "" {
		schema, _ = sjson.SetBytes(schema, "type", "object")
		schemaType = "object"
	}
	if schemaType == "object" && !result.Get("properties").Exists() {
		schema, _ = sjson.SetRawBytes(schema, "properties", []byte(`{}`))
	}
	return string(schema)
}
