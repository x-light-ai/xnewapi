// FORK-CUSTOM: Adapt the CPA Claude-to-OpenAI request translator.
package openai

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func ConvertClaudeRequestToOpenAI(modelName string, inputRawJSON []byte, stream bool) []byte {
	rawJSON := inputRawJSON
	out := []byte(`{"model":"","messages":[]}`)

	root := gjson.ParseBytes(rawJSON)
	out, _ = sjson.SetBytes(out, "model", modelName)

	if maxTokens := root.Get("max_tokens"); maxTokens.Exists() {
		out, _ = sjson.SetBytes(out, "max_tokens", maxTokens.Int())
	}

	if temp := root.Get("temperature"); temp.Exists() {
		out, _ = sjson.SetBytes(out, "temperature", temp.Float())
	} else if topP := root.Get("top_p"); topP.Exists() {
		out, _ = sjson.SetBytes(out, "top_p", topP.Float())
	}

	if topK := root.Get("top_k"); topK.Exists() {
		out, _ = sjson.SetBytes(out, "top_k", topK.Int())
	}

	if stopSequences := root.Get("stop_sequences"); stopSequences.Exists() && stopSequences.IsArray() {
		var stops []string
		stopSequences.ForEach(func(_, value gjson.Result) bool {
			stops = append(stops, value.String())
			return true
		})
		if len(stops) == 1 {
			out, _ = sjson.SetBytes(out, "stop", stops[0])
		} else if len(stops) > 1 {
			out, _ = sjson.SetBytes(out, "stop", stops)
		}
	}

	out, _ = sjson.SetBytes(out, "stream", stream)

	thinkingType := strings.ToLower(strings.TrimSpace(root.Get("thinking.type").String()))
	var budgetPtr *int
	if budgetTokens := root.Get("thinking.budget_tokens"); budgetTokens.Exists() {
		v := int(budgetTokens.Int())
		budgetPtr = &v
	}
	if effort := mapReasoningEffort(thinkingType, budgetPtr, root.Get("output_config.effort").String()); effort != "" {
		out, _ = sjson.SetBytes(out, "reasoning_effort", effort)
	}

	messageCapacity := root.Get("messages.#").Int()
	if root.Get("system").Exists() {
		messageCapacity++
	}
	messageItems := newRawArrayItems(messageCapacity)

	systemContentItems := make([][]byte, 0, 2)
	if system := root.Get("system"); system.Exists() {
		if system.Type == gjson.String {
			if system.String() != "" && !isClaudeCodeAttributionSystemText(system.String()) {
				oldSystem := []byte(`{"type":"text","text":""}`)
				oldSystem, _ = sjson.SetBytes(oldSystem, "text", system.String())
				systemContentItems = append(systemContentItems, oldSystem)
			}
		} else if system.IsArray() {
			for _, item := range system.Array() {
				if contentItem, ok := convertClaudeContentPart(item); ok {
					systemContentItems = append(systemContentItems, []byte(contentItem))
				}
			}
		}
	}
	if len(systemContentItems) > 0 {
		systemMessage := []byte(`{"role":"system","content":[]}`)
		systemMessage, _ = sjson.SetRawBytes(systemMessage, "content", joinRawArray(systemContentItems))
		messageItems = append(messageItems, systemMessage)
	}

	if messages := root.Get("messages"); messages.Exists() && messages.IsArray() {
		messages.ForEach(func(_, message gjson.Result) bool {
			role := message.Get("role").String()
			contentResult := message.Get("content")

			if role == "system" {
				if reminderText, ok := claudeMessageSystemReminderText(contentResult); ok {
					msgJSON := []byte(`{"role":"user","content":[{"type":"text","text":""}]}`)
					msgJSON, _ = sjson.SetBytes(msgJSON, "content.0.text", reminderText)
					messageItems = append(messageItems, msgJSON)
				}
				return true
			}

			if contentResult.Exists() && contentResult.IsArray() {
				contentItems := make([][]byte, 0)
				var reasoningParts []string
				var toolCalls []any
				toolResults := make([][]byte, 0)

				contentResult.ForEach(func(_, part gjson.Result) bool {
					switch part.Get("type").String() {
					case "thinking":
						if role == "assistant" {
							if !shouldMapClaudeThinkingToGPTReasoning(part) {
								return true
							}
							thinkingText := strings.TrimSpace(part.Get("thinking").String())
							if thinkingText == "" {
								thinkingText = strings.TrimSpace(part.Get("text").String())
							}
							if thinkingText != "" {
								reasoningParts = append(reasoningParts, thinkingText)
							}
						}
					case "redacted_thinking":
					case "text", "image":
						if contentItem, ok := convertClaudeContentPart(part); ok {
							contentItems = append(contentItems, []byte(contentItem))
						}
					case "tool_use":
						if role == "assistant" {
							toolCallJSON := []byte(`{"id":"","type":"function","function":{"name":"","arguments":""}}`)
							toolCallJSON, _ = sjson.SetBytes(toolCallJSON, "id", part.Get("id").String())
							toolCallJSON, _ = sjson.SetBytes(toolCallJSON, "function.name", part.Get("name").String())
							if input := part.Get("input"); input.Exists() {
								toolCallJSON, _ = sjson.SetBytes(toolCallJSON, "function.arguments", input.Raw)
							} else {
								toolCallJSON, _ = sjson.SetBytes(toolCallJSON, "function.arguments", "{}")
							}
							toolCalls = append(toolCalls, gjson.ParseBytes(toolCallJSON).Value())
						}
					case "tool_result":
						toolResultJSON := []byte(`{"role":"tool","tool_call_id":"","content":""}`)
						toolResultJSON, _ = sjson.SetBytes(toolResultJSON, "tool_call_id", part.Get("tool_use_id").String())
						toolResultContent, toolResultContentRaw := convertClaudeToolResultContent(part.Get("content"))
						if toolResultContentRaw {
							toolResultJSON, _ = sjson.SetRawBytes(toolResultJSON, "content", []byte(toolResultContent))
						} else {
							toolResultJSON, _ = sjson.SetBytes(toolResultJSON, "content", toolResultContent)
						}
						toolResults = append(toolResults, toolResultJSON)
					}
					return true
				})

				reasoningContent := strings.Join(reasoningParts, "\n\n")
				hasContent := len(contentItems) > 0
				hasReasoning := reasoningContent != ""
				hasToolCalls := len(toolCalls) > 0
				hasToolResults := len(toolResults) > 0

				messageItems = append(messageItems, toolResults...)

				if role == "assistant" {
					if hasContent || hasReasoning || hasToolCalls {
						msgJSON := []byte(`{"role":"assistant"}`)
						if hasContent {
							msgJSON, _ = sjson.SetRawBytes(msgJSON, "content", joinRawArray(contentItems))
						} else {
							msgJSON, _ = sjson.SetBytes(msgJSON, "content", "")
						}
						if hasReasoning {
							msgJSON, _ = sjson.SetBytes(msgJSON, "reasoning_content", reasoningContent)
						}
						if hasToolCalls {
							msgJSON, _ = sjson.SetBytes(msgJSON, "tool_calls", toolCalls)
						}
						messageItems = append(messageItems, msgJSON)
					}
				} else {
					if hasContent {
						msgJSON := []byte(`{"role":""}`)
						msgJSON, _ = sjson.SetBytes(msgJSON, "role", role)
						msgJSON, _ = sjson.SetRawBytes(msgJSON, "content", joinRawArray(contentItems))
						messageItems = append(messageItems, msgJSON)
					} else if hasToolResults && !hasContent {
					}
				}
			} else if contentResult.Exists() && contentResult.Type == gjson.String {
				msgJSON := []byte(`{"role":"","content":""}`)
				msgJSON, _ = sjson.SetBytes(msgJSON, "role", role)
				msgJSON, _ = sjson.SetBytes(msgJSON, "content", contentResult.String())
				messageItems = append(messageItems, msgJSON)
			}

			return true
		})
	}

	if len(messageItems) > 0 {
		out = setRawArrayItems(out, "messages", messageItems)
	}

	if tools := root.Get("tools"); tools.Exists() && tools.IsArray() {
		var toolItems [][]byte
		tools.ForEach(func(_, tool gjson.Result) bool {
			openAIToolJSON := []byte(`{"type":"function","function":{"name":"","description":""}}`)
			openAIToolJSON, _ = sjson.SetBytes(openAIToolJSON, "function.name", tool.Get("name").String())
			openAIToolJSON, _ = sjson.SetBytes(openAIToolJSON, "function.description", tool.Get("description").String())
			if inputSchema := tool.Get("input_schema"); inputSchema.Exists() {
				openAIToolJSON, _ = sjson.SetBytes(openAIToolJSON, "function.parameters", normalizeObjectSchemaProperties(inputSchema.Value()))
			}
			toolItems = append(toolItems, openAIToolJSON)
			return true
		})
		if len(toolItems) > 0 {
			out, _ = sjson.SetRawBytes(out, "tools", joinRawArray(toolItems))
		}
	}

	if toolChoice := root.Get("tool_choice"); toolChoice.Exists() {
		switch toolChoice.Get("type").String() {
		case "auto":
			out, _ = sjson.SetBytes(out, "tool_choice", "auto")
		case "any":
			out, _ = sjson.SetBytes(out, "tool_choice", "required")
		case "tool":
			toolName := toolChoice.Get("name").String()
			toolChoiceJSON := []byte(`{"type":"function","function":{"name":""}}`)
			toolChoiceJSON, _ = sjson.SetBytes(toolChoiceJSON, "function.name", toolName)
			out, _ = sjson.SetRawBytes(out, "tool_choice", toolChoiceJSON)
		default:
			out, _ = sjson.SetBytes(out, "tool_choice", "auto")
		}
	}

	if user := root.Get("metadata.user_id"); user.Exists() && strings.TrimSpace(user.String()) != "" {
		out, _ = sjson.SetBytes(out, "user", user.String())
	} else if user := root.Get("user"); user.Exists() && strings.TrimSpace(user.String()) != "" {
		out, _ = sjson.SetBytes(out, "user", user.String())
	}

	return out
}

func normalizeObjectSchemaProperties(schema any) any {
	switch value := schema.(type) {
	case map[string]any:
		if schemaType, ok := value["type"].(string); ok && schemaType == "object" {
			if _, ok := value["properties"]; !ok {
				value["properties"] = map[string]any{}
			}
		}
		for key, child := range value {
			value[key] = normalizeObjectSchemaProperties(child)
		}
		return value
	case []any:
		for i, child := range value {
			value[i] = normalizeObjectSchemaProperties(child)
		}
		return value
	default:
		return schema
	}
}

func ConvertClaudeRequestToOpenAIRequest(request *dto.ClaudeRequest, infoModel string, forceStream bool) (*dto.GeneralOpenAIRequest, error) {
	raw, err := common.Marshal(request)
	if err != nil {
		return nil, err
	}
	converted := ConvertClaudeRequestToOpenAI(infoModel, raw, forceStream)
	var out dto.GeneralOpenAIRequest
	if err = common.Unmarshal(converted, &out); err != nil {
		return nil, err
	}
	if out.Stream != nil && lo.FromPtr(out.Stream) {
		out.Stream = lo.ToPtr(true)
	}
	return &out, nil
}

func convertClaudeContentPart(part gjson.Result) (string, bool) {
	switch part.Get("type").String() {
	case "text":
		text := part.Get("text").String()
		if strings.TrimSpace(text) == "" || isClaudeCodeAttributionSystemText(text) {
			return "", false
		}
		textContent := []byte(`{"type":"text","text":""}`)
		textContent, _ = sjson.SetBytes(textContent, "text", text)
		return string(textContent), true
	case "image":
		var imageURL string
		if source := part.Get("source"); source.Exists() {
			switch source.Get("type").String() {
			case "base64":
				mediaType := source.Get("media_type").String()
				if mediaType == "" {
					mediaType = "application/octet-stream"
				}
				data := source.Get("data").String()
				if data != "" {
					imageURL = "data:" + mediaType + ";base64," + data
				}
			case "url":
				imageURL = source.Get("url").String()
			}
		}
		if imageURL == "" {
			imageURL = part.Get("url").String()
		}
		if imageURL == "" {
			return "", false
		}
		imageContent := []byte(`{"type":"image_url","image_url":{"url":""}}`)
		imageContent, _ = sjson.SetBytes(imageContent, "image_url.url", imageURL)
		return string(imageContent), true
	default:
		return "", false
	}
}

func convertClaudeToolResultContent(content gjson.Result) (string, bool) {
	if !content.Exists() {
		return "", false
	}
	if content.Type == gjson.String {
		return content.String(), false
	}
	if content.IsArray() {
		var parts []string
		contentItems := make([][]byte, 0, 4)
		hasImagePart := false
		content.ForEach(func(_, item gjson.Result) bool {
			switch {
			case item.Type == gjson.String:
				text := item.String()
				parts = append(parts, text)
				textContent := []byte(`{"type":"text","text":""}`)
				textContent, _ = sjson.SetBytes(textContent, "text", text)
				contentItems = append(contentItems, textContent)
			case item.IsObject() && item.Get("type").String() == "text":
				text := item.Get("text").String()
				parts = append(parts, text)
				textContent := []byte(`{"type":"text","text":""}`)
				textContent, _ = sjson.SetBytes(textContent, "text", text)
				contentItems = append(contentItems, textContent)
			case item.IsObject() && item.Get("type").String() == "image":
				contentItem, ok := convertClaudeContentPart(item)
				if ok {
					contentItems = append(contentItems, []byte(contentItem))
					hasImagePart = true
				} else {
					parts = append(parts, item.Raw)
				}
			case item.IsObject() && item.Get("text").Exists() && item.Get("text").Type == gjson.String:
				parts = append(parts, item.Get("text").String())
			default:
				parts = append(parts, item.Raw)
			}
			return true
		})
		if hasImagePart {
			return string(joinRawArray(contentItems)), true
		}
		joined := strings.Join(parts, "\n\n")
		if strings.TrimSpace(joined) != "" {
			return joined, false
		}
		return content.Raw, false
	}
	if content.IsObject() {
		if content.Get("type").String() == "image" {
			contentItem, ok := convertClaudeContentPart(content)
			if ok {
				return string(joinRawArray([][]byte{[]byte(contentItem)})), true
			}
		}
		if text := content.Get("text"); text.Exists() && text.Type == gjson.String {
			return text.String(), false
		}
		return content.Raw, false
	}
	return content.Raw, false
}
