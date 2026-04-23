package openai

import (
	"bytes"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/reasonmap"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type ConvertOpenAIResponseToAnthropicParams struct {
	MessageID                   string
	Model                       string
	CreatedAt                   int64
	ToolNameMap                 map[string]string
	SawToolCall                 bool
	ContentAccumulator          strings.Builder
	ToolCallsAccumulator        map[int]*ToolCallAccumulator
	TextContentBlockStarted     bool
	ThinkingContentBlockStarted bool
	FinishReason                string
	ContentBlocksStopped        bool
	MessageDeltaSent            bool
	MessageStarted              bool
	MessageStopSent             bool
	ToolCallBlockIndexes        map[int]int
	TextContentBlockIndex       int
	ThinkingContentBlockIndex   int
	NextContentBlockIndex       int
}

type ToolCallAccumulator struct {
	ID        string
	Name      string
	Arguments strings.Builder
}

func ConvertOpenAIResponseToClaudeNonStreamBytes(originalRequestRawJSON, rawJSON []byte) []byte {
	toolNameMap := toolNameMapFromClaudeRequest(originalRequestRawJSON)
	root := gjson.ParseBytes(rawJSON)
	out := []byte(`{"id":"","type":"message","role":"assistant","model":"","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}`)
	out, _ = sjson.SetBytes(out, "id", root.Get("id").String())
	out, _ = sjson.SetBytes(out, "model", root.Get("model").String())

	hasToolCall := false
	stopReasonSet := false

	if choices := root.Get("choices"); choices.Exists() && choices.IsArray() && len(choices.Array()) > 0 {
		choice := choices.Array()[0]
		if finishReason := choice.Get("finish_reason"); finishReason.Exists() {
			out, _ = sjson.SetBytes(out, "stop_reason", mapOpenAIFinishReasonToAnthropic(finishReason.String()))
			stopReasonSet = true
		}
		if message := choice.Get("message"); message.Exists() {
			if reasoning := message.Get("reasoning_content"); reasoning.Exists() {
				for _, reasoningText := range collectOpenAIReasoningTexts(reasoning) {
					if reasoningText == "" {
						continue
					}
					block := []byte(`{"type":"thinking","thinking":""}`)
					block, _ = sjson.SetBytes(block, "thinking", reasoningText)
					out, _ = sjson.SetRawBytes(out, "content.-1", block)
				}
			}
			if content := message.Get("content"); content.Exists() && content.String() != "" {
				block := []byte(`{"type":"text","text":""}`)
				block, _ = sjson.SetBytes(block, "text", content.String())
				out, _ = sjson.SetRawBytes(out, "content.-1", block)
			}
			if toolCalls := message.Get("tool_calls"); toolCalls.Exists() && toolCalls.IsArray() {
				toolCalls.ForEach(func(_, toolCall gjson.Result) bool {
					hasToolCall = true
					toolUseBlock := []byte(`{"type":"tool_use","id":"","name":"","input":{}}`)
					toolUseBlock, _ = sjson.SetBytes(toolUseBlock, "id", sanitizeClaudeToolID(toolCall.Get("id").String()))
					toolUseBlock, _ = sjson.SetBytes(toolUseBlock, "name", mapToolName(toolNameMap, toolCall.Get("function.name").String()))
					argsStr := fixJSON(toolCall.Get("function.arguments").String())
					if argsStr != "" && gjson.Valid(argsStr) {
						argsJSON := gjson.Parse(argsStr)
						if argsJSON.IsObject() {
							toolUseBlock, _ = sjson.SetRawBytes(toolUseBlock, "input", []byte(argsJSON.Raw))
						}
					}
					out, _ = sjson.SetRawBytes(out, "content.-1", toolUseBlock)
					return true
				})
			}
		}
	}

	if respUsage := root.Get("usage"); respUsage.Exists() {
		inputTokens, outputTokens, cachedTokens := extractOpenAIUsage(respUsage)
		out, _ = sjson.SetBytes(out, "usage.input_tokens", inputTokens)
		out, _ = sjson.SetBytes(out, "usage.output_tokens", outputTokens)
		if cachedTokens > 0 {
			out, _ = sjson.SetBytes(out, "usage.cache_read_input_tokens", cachedTokens)
		}
	}

	if !stopReasonSet {
		if hasToolCall {
			out, _ = sjson.SetBytes(out, "stop_reason", "tool_use")
		} else {
			out, _ = sjson.SetBytes(out, "stop_reason", "end_turn")
		}
	}
	return out
}

func ResponseOpenAI2ClaudeWithTranslator(openAIResponse *dto.OpenAITextResponse, info *relaycommon.RelayInfo) (*dto.ClaudeResponse, error) {
	responseBytes, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, err
	}
	converted := ConvertOpenAIResponseToClaudeNonStreamBytes(info.ClaudeConvertInfo.OriginalRequestRawJSON, responseBytes)
	var claudeResp dto.ClaudeResponse
	if err = common.Unmarshal(converted, &claudeResp); err != nil {
		return nil, err
	}
	return &claudeResp, nil
}

func StreamResponseOpenAI2ClaudeWithTranslator(openAIResponse *dto.ChatCompletionsStreamResponse, info *relaycommon.RelayInfo) ([]*dto.ClaudeResponse, error) {
	if info.ClaudeConvertInfo == nil {
		info.ClaudeConvertInfo = &relaycommon.ClaudeConvertInfo{LastMessagesType: relaycommon.LastMessageTypeNone}
	}
	if info.ClaudeConvertInfo.Done {
		return nil, nil
	}
	if openAIResponse.Usage != nil {
		info.ClaudeConvertInfo.Usage = openAIResponse.Usage
	}

	rawJSON, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, err
	}
	param := info.ClaudeConvertInfo.StreamTranslatorState
	resultBytes := convertOpenAIResponseToClaudeStreamBytes(info.ClaudeConvertInfo.OriginalRequestRawJSON, rawJSON, &param)
	info.ClaudeConvertInfo.StreamTranslatorState = param

	responses := make([]*dto.ClaudeResponse, 0, len(resultBytes))
	for _, item := range resultBytes {
		payload := bytes.TrimSpace(item)
		if bytes.HasPrefix(payload, []byte("event:")) {
			if idx := bytes.Index(payload, []byte("\ndata: ")); idx >= 0 {
				payload = payload[idx+7:]
			}
		}
		payload = bytes.TrimSpace(payload)
		if len(payload) == 0 {
			continue
		}
		var resp dto.ClaudeResponse
		if err = common.Unmarshal(payload, &resp); err != nil {
			return nil, err
		}
		if resp.Type == "message_delta" && resp.Delta != nil && resp.Delta.StopReason != nil {
			info.FinishReason = reasonmap.ClaudeStopReasonToOpenAIFinishReason(*resp.Delta.StopReason)
		}
		if resp.Type == "message_stop" {
			info.ClaudeConvertInfo.Done = true
		}
		responses = append(responses, &resp)
	}
	return responses, nil
}

func convertOpenAIResponseToClaudeStreamBytes(originalRequestRawJSON, rawJSON []byte, param *any) [][]byte {
	if *param == nil {
		*param = &ConvertOpenAIResponseToAnthropicParams{
			ToolCallBlockIndexes:  make(map[int]int),
			TextContentBlockIndex: -1,
			ThinkingContentBlockIndex: -1,
		}
	}
	p := (*param).(*ConvertOpenAIResponseToAnthropicParams)
	root := gjson.ParseBytes(rawJSON)
	var results [][]byte
	if p.ToolNameMap == nil {
		p.ToolNameMap = toolNameMapFromClaudeRequest(originalRequestRawJSON)
	}
	if p.MessageID == "" {
		p.MessageID = root.Get("id").String()
	}
	if p.Model == "" {
		p.Model = root.Get("model").String()
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = root.Get("created").Int()
	}
	if !p.MessageStarted {
		messageStartJSON := []byte(`{"type":"message_start","message":{"id":"","type":"message","role":"assistant","model":"","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}`)
		messageStartJSON, _ = sjson.SetBytes(messageStartJSON, "message.id", p.MessageID)
		messageStartJSON, _ = sjson.SetBytes(messageStartJSON, "message.model", p.Model)
		results = append(results, appendSSEEventBytes(nil, "message_start", messageStartJSON, 2))
		p.MessageStarted = true
	}

	if len(root.Get("choices").Array()) == 0 {
		if p.FinishReason != "" {
			usage := root.Get("usage")
			if usage.Exists() && usage.Type != gjson.Null {
				inputTokens, outputTokens, cachedTokens := extractOpenAIUsage(usage)
				messageDeltaJSON := []byte(`{"type":"message_delta","delta":{"stop_reason":"","stop_sequence":null},"usage":{"input_tokens":0,"output_tokens":0}}`)
				messageDeltaJSON, _ = sjson.SetBytes(messageDeltaJSON, "delta.stop_reason", mapOpenAIFinishReasonToAnthropic(effectiveOpenAIFinishReason(p)))
				messageDeltaJSON, _ = sjson.SetBytes(messageDeltaJSON, "usage.input_tokens", inputTokens)
				messageDeltaJSON, _ = sjson.SetBytes(messageDeltaJSON, "usage.output_tokens", outputTokens)
				if cachedTokens > 0 {
					messageDeltaJSON, _ = sjson.SetBytes(messageDeltaJSON, "usage.cache_read_input_tokens", cachedTokens)
				}
				results = append(results, appendSSEEventBytes(nil, "message_delta", messageDeltaJSON, 2))
				emitMessageStopIfNeeded(p, &results)
			}
		}
		return results
	}

	delta := root.Get("choices.0.delta")
	if delta.Exists() {
		if reasoning := delta.Get("reasoning_content"); reasoning.Exists() {
			for _, reasoningText := range collectOpenAIReasoningTexts(reasoning) {
				if reasoningText == "" {
					continue
				}
				stopTextContentBlock(p, &results)
				if !p.ThinkingContentBlockStarted {
					if p.ThinkingContentBlockIndex == -1 {
						p.ThinkingContentBlockIndex = p.NextContentBlockIndex
						p.NextContentBlockIndex++
					}
					contentBlockStartJSON := []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`)
					contentBlockStartJSON, _ = sjson.SetBytes(contentBlockStartJSON, "index", p.ThinkingContentBlockIndex)
					results = append(results, appendSSEEventBytes(nil, "content_block_start", contentBlockStartJSON, 2))
					p.ThinkingContentBlockStarted = true
				}
				thinkingDeltaJSON := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":""}}`)
				thinkingDeltaJSON, _ = sjson.SetBytes(thinkingDeltaJSON, "index", p.ThinkingContentBlockIndex)
				thinkingDeltaJSON, _ = sjson.SetBytes(thinkingDeltaJSON, "delta.thinking", reasoningText)
				results = append(results, appendSSEEventBytes(nil, "content_block_delta", thinkingDeltaJSON, 2))
			}
		}
		if content := delta.Get("content"); content.Exists() && content.String() != "" {
			if !p.TextContentBlockStarted {
				stopThinkingContentBlock(p, &results)
				if p.TextContentBlockIndex == -1 {
					p.TextContentBlockIndex = p.NextContentBlockIndex
					p.NextContentBlockIndex++
				}
				contentBlockStartJSON := []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
				contentBlockStartJSON, _ = sjson.SetBytes(contentBlockStartJSON, "index", p.TextContentBlockIndex)
				results = append(results, appendSSEEventBytes(nil, "content_block_start", contentBlockStartJSON, 2))
				p.TextContentBlockStarted = true
			}
			contentDeltaJSON := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":""}}`)
			contentDeltaJSON, _ = sjson.SetBytes(contentDeltaJSON, "index", p.TextContentBlockIndex)
			contentDeltaJSON, _ = sjson.SetBytes(contentDeltaJSON, "delta.text", content.String())
			results = append(results, appendSSEEventBytes(nil, "content_block_delta", contentDeltaJSON, 2))
		}
		if toolCalls := delta.Get("tool_calls"); toolCalls.Exists() && toolCalls.IsArray() {
			if p.ToolCallsAccumulator == nil {
				p.ToolCallsAccumulator = make(map[int]*ToolCallAccumulator)
			}
			toolCalls.ForEach(func(_, toolCall gjson.Result) bool {
				p.SawToolCall = true
				index := int(toolCall.Get("index").Int())
				blockIndex := p.toolContentBlockIndex(index)
				if _, exists := p.ToolCallsAccumulator[index]; !exists {
					p.ToolCallsAccumulator[index] = &ToolCallAccumulator{}
				}
				acc := p.ToolCallsAccumulator[index]
				if id := toolCall.Get("id"); id.Exists() {
					acc.ID = id.String()
				}
				if function := toolCall.Get("function"); function.Exists() {
					if name := function.Get("name"); name.Exists() {
						acc.Name = mapToolName(p.ToolNameMap, name.String())
						stopThinkingContentBlock(p, &results)
						stopTextContentBlock(p, &results)
						contentBlockStartJSON := []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"","name":"","input":{}}}`)
						contentBlockStartJSON, _ = sjson.SetBytes(contentBlockStartJSON, "index", blockIndex)
						contentBlockStartJSON, _ = sjson.SetBytes(contentBlockStartJSON, "content_block.id", sanitizeClaudeToolID(acc.ID))
						contentBlockStartJSON, _ = sjson.SetBytes(contentBlockStartJSON, "content_block.name", acc.Name)
						results = append(results, appendSSEEventBytes(nil, "content_block_start", contentBlockStartJSON, 2))
					}
					if args := function.Get("arguments"); args.Exists() {
						argsText := args.String()
						if argsText != "" {
							acc.Arguments.WriteString(argsText)
						}
					}
				}
				return true
			})
		}
	}

	if finishReason := root.Get("choices.0.finish_reason"); finishReason.Exists() && finishReason.String() != "" {
		reason := finishReason.String()
		if p.SawToolCall {
			p.FinishReason = "tool_calls"
		} else {
			p.FinishReason = reason
		}
		stopThinkingContentBlock(p, &results)
		stopTextContentBlock(p, &results)
		if !p.ContentBlocksStopped {
			for index := range p.ToolCallsAccumulator {
				acc := p.ToolCallsAccumulator[index]
				blockIndex := p.toolContentBlockIndex(index)
				if acc.Arguments.Len() > 0 {
					inputDeltaJSON := []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":""}}`)
					inputDeltaJSON, _ = sjson.SetBytes(inputDeltaJSON, "index", blockIndex)
					inputDeltaJSON, _ = sjson.SetBytes(inputDeltaJSON, "delta.partial_json", fixJSON(acc.Arguments.String()))
					results = append(results, appendSSEEventBytes(nil, "content_block_delta", inputDeltaJSON, 2))
				}
				contentBlockStopJSON := []byte(`{"type":"content_block_stop","index":0}`)
				contentBlockStopJSON, _ = sjson.SetBytes(contentBlockStopJSON, "index", blockIndex)
				results = append(results, appendSSEEventBytes(nil, "content_block_stop", contentBlockStopJSON, 2))
			}
			p.ContentBlocksStopped = true
		}
	}

	if p.FinishReason != "" {
		usage := root.Get("usage")
		if usage.Exists() && usage.Type != gjson.Null {
			inputTokens, outputTokens, cachedTokens := extractOpenAIUsage(usage)
			messageDeltaJSON := []byte(`{"type":"message_delta","delta":{"stop_reason":"","stop_sequence":null},"usage":{"input_tokens":0,"output_tokens":0}}`)
			messageDeltaJSON, _ = sjson.SetBytes(messageDeltaJSON, "delta.stop_reason", mapOpenAIFinishReasonToAnthropic(effectiveOpenAIFinishReason(p)))
			messageDeltaJSON, _ = sjson.SetBytes(messageDeltaJSON, "usage.input_tokens", inputTokens)
			messageDeltaJSON, _ = sjson.SetBytes(messageDeltaJSON, "usage.output_tokens", outputTokens)
			if cachedTokens > 0 {
				messageDeltaJSON, _ = sjson.SetBytes(messageDeltaJSON, "usage.cache_read_input_tokens", cachedTokens)
			}
			results = append(results, appendSSEEventBytes(nil, "message_delta", messageDeltaJSON, 2))
			emitMessageStopIfNeeded(p, &results)
		}
	}

	return results
}

func effectiveOpenAIFinishReason(param *ConvertOpenAIResponseToAnthropicParams) string {
	if param == nil {
		return ""
	}
	if param.SawToolCall {
		return "tool_calls"
	}
	return param.FinishReason
}

func mapOpenAIFinishReasonToAnthropic(openAIReason string) string {
	return reasonmap.OpenAIFinishReasonToClaudeStopReason(openAIReason)
}

func (p *ConvertOpenAIResponseToAnthropicParams) toolContentBlockIndex(openAIToolIndex int) int {
	if idx, ok := p.ToolCallBlockIndexes[openAIToolIndex]; ok {
		return idx
	}
	idx := p.NextContentBlockIndex
	p.NextContentBlockIndex++
	p.ToolCallBlockIndexes[openAIToolIndex] = idx
	return idx
}

func collectOpenAIReasoningTexts(node gjson.Result) []string {
	var texts []string
	if !node.Exists() {
		return texts
	}
	if node.IsArray() {
		node.ForEach(func(_, value gjson.Result) bool {
			texts = append(texts, collectOpenAIReasoningTexts(value)...)
			return true
		})
		return texts
	}
	switch node.Type {
	case gjson.String:
		if text := node.String(); text != "" {
			texts = append(texts, text)
		}
	case gjson.JSON:
		if text := node.Get("text"); text.Exists() {
			if textStr := text.String(); textStr != "" {
				texts = append(texts, textStr)
			}
		}
	}
	return texts
}

func stopThinkingContentBlock(param *ConvertOpenAIResponseToAnthropicParams, results *[][]byte) {
	if !param.ThinkingContentBlockStarted {
		return
	}
	contentBlockStopJSON := []byte(`{"type":"content_block_stop","index":0}`)
	contentBlockStopJSON, _ = sjson.SetBytes(contentBlockStopJSON, "index", param.ThinkingContentBlockIndex)
	*results = append(*results, appendSSEEventBytes(nil, "content_block_stop", contentBlockStopJSON, 2))
	param.ThinkingContentBlockStarted = false
	param.ThinkingContentBlockIndex = -1
}

func stopTextContentBlock(param *ConvertOpenAIResponseToAnthropicParams, results *[][]byte) {
	if !param.TextContentBlockStarted {
		return
	}
	contentBlockStopJSON := []byte(`{"type":"content_block_stop","index":0}`)
	contentBlockStopJSON, _ = sjson.SetBytes(contentBlockStopJSON, "index", param.TextContentBlockIndex)
	*results = append(*results, appendSSEEventBytes(nil, "content_block_stop", contentBlockStopJSON, 2))
	param.TextContentBlockStarted = false
	param.TextContentBlockIndex = -1
}

func emitMessageStopIfNeeded(param *ConvertOpenAIResponseToAnthropicParams, results *[][]byte) {
	if param.MessageStopSent {
		return
	}
	*results = append(*results, appendSSEEventBytes(nil, "message_stop", []byte(`{"type":"message_stop"}`), 2))
	param.MessageStopSent = true
}

func extractOpenAIUsage(usage gjson.Result) (int64, int64, int64) {
	if !usage.Exists() || usage.Type == gjson.Null {
		return 0, 0, 0
	}
	inputTokens := usage.Get("prompt_tokens").Int()
	outputTokens := usage.Get("completion_tokens").Int()
	cachedTokens := usage.Get("prompt_tokens_details.cached_tokens").Int()
	if cachedTokens > 0 {
		if inputTokens >= cachedTokens {
			inputTokens -= cachedTokens
		} else {
			inputTokens = 0
		}
	}
	return inputTokens, outputTokens, cachedTokens
}
