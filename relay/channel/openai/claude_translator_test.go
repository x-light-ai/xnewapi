// FORK-CUSTOM: Verify the CPA-derived OpenAI translator integration.
package openai

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestConvertClaudeRequestToOpenAIRequest_MapsSystemThinkingToolsAndToolResults(t *testing.T) {
	maxTokens := uint(256)
	stream := true
	request := &dto.ClaudeRequest{
		Model:     "claude-3-7-sonnet",
		System:    "You are a precise assistant.",
		MaxTokens: &maxTokens,
		Stream:    &stream,
		Thinking: &dto.Thinking{
			Type:         "enabled",
			BudgetTokens: common.GetPointer(4096),
		},
		Metadata: json.RawMessage(`{"user_id":"user-123"}`),
		Tools: []dto.Tool{
			{
				Name:        "GetWeather",
				Description: "Look up weather",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		ToolChoice: dto.ClaudeToolChoice{Type: "tool", Name: "GetWeather"},
		Messages: []dto.ClaudeMessage{
			{
				Role: "user",
				Content: []dto.ClaudeMediaMessage{
					{Type: dto.ContentTypeText, Text: common.GetPointer("What's the weather in Paris?")},
				},
			},
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{Type: "thinking", Thinking: common.GetPointer("Need weather tool")},
					{Type: "tool_use", Id: "toolu_1", Name: "GetWeather", Input: map[string]any{"city": "Paris"}},
				},
			},
			{
				Role: "user",
				Content: []dto.ClaudeMediaMessage{
					{Type: "tool_result", ToolUseId: "toolu_1", Content: []dto.ClaudeMediaMessage{{Type: dto.ContentTypeText, Text: common.GetPointer("Sunny")}}},
				},
			},
		},
	}

	converted, err := ConvertClaudeRequestToOpenAIRequest(request, "gpt-4.1", true)
	require.NoError(t, err)
	require.NotNil(t, converted)

	require.Equal(t, "gpt-4.1", converted.Model)
	require.NotNil(t, converted.Stream)
	require.True(t, *converted.Stream)
	require.Equal(t, "medium", converted.ReasoningEffort)
	require.Len(t, converted.Messages, 4)

	require.Equal(t, "system", converted.Messages[0].Role)
	systemParts := converted.Messages[0].ParseContent()
	require.Len(t, systemParts, 1)
	require.Equal(t, dto.ContentTypeText, systemParts[0].Type)
	require.Equal(t, "You are a precise assistant.", systemParts[0].Text)

	require.Equal(t, "user", converted.Messages[1].Role)
	userParts := converted.Messages[1].ParseContent()
	require.Len(t, userParts, 1)
	require.Equal(t, "What's the weather in Paris?", userParts[0].Text)

	require.Equal(t, "assistant", converted.Messages[2].Role)
	require.Empty(t, converted.Messages[2].ReasoningContent)
	var toolCalls []dto.ToolCallResponse
	require.NoError(t, common.Unmarshal(converted.Messages[2].ToolCalls, &toolCalls))
	require.Len(t, toolCalls, 1)
	require.Equal(t, "toolu_1", toolCalls[0].ID)
	require.Equal(t, "function", toolCalls[0].Type)
	require.Equal(t, "GetWeather", toolCalls[0].Function.Name)
	require.JSONEq(t, `{"city":"Paris"}`, toolCalls[0].Function.Arguments)

	require.Equal(t, "tool", converted.Messages[3].Role)
	require.Equal(t, "toolu_1", converted.Messages[3].ToolCallId)
	require.Equal(t, "Sunny", converted.Messages[3].Content)

	require.Len(t, converted.Tools, 1)
	require.Equal(t, "function", converted.Tools[0].Type)
	require.Equal(t, "GetWeather", converted.Tools[0].Function.Name)
	require.Equal(t, "Look up weather", converted.Tools[0].Function.Description)
	require.Equal(t, map[string]any{"type": "function", "function": map[string]any{"name": "GetWeather"}}, converted.ToolChoice)

	var userID string
	require.NoError(t, common.Unmarshal(converted.User, &userID))
	require.Equal(t, "user-123", userID)
}

func TestConvertClaudeRequestToOpenAIRequest_SignedThinkingCompatibility(t *testing.T) {
	maxTokens := uint(256)
	request := &dto.ClaudeRequest{
		Model:     "claude-3-7-sonnet",
		MaxTokens: &maxTokens,
		Messages: []dto.ClaudeMessage{
			{
				Role: "assistant",
				Content: []dto.ClaudeMediaMessage{
					{Type: "thinking", Thinking: common.GetPointer("provider state"), Signature: validGPTChatReasoningSignature()},
					{Type: dto.ContentTypeText, Text: common.GetPointer("visible answer")},
				},
			},
		},
	}

	converted, err := ConvertClaudeRequestToOpenAIRequest(request, "gpt-4.1", false)
	require.NoError(t, err)
	require.Len(t, converted.Messages, 1)
	require.Equal(t, "provider state", converted.Messages[0].ReasoningContent)
	parts := converted.Messages[0].ParseContent()
	require.Len(t, parts, 1)
	require.Equal(t, "visible answer", parts[0].Text)
}

func TestConvertClaudeRequestToOpenAIRequest_StripsClaudeCodeAttribution(t *testing.T) {
	maxTokens := uint(256)
	request := &dto.ClaudeRequest{
		Model:     "claude-3-7-sonnet",
		MaxTokens: &maxTokens,
		System: []dto.ClaudeMediaMessage{
			{Type: dto.ContentTypeText, Text: common.GetPointer("x-anthropic-billing-header: cc_version=2.1.63; cch=123;")},
			{Type: dto.ContentTypeText, Text: common.GetPointer("User system prompt")},
		},
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: dto.ContentTypeText, Text: common.GetPointer("hi")}}},
		},
	}

	converted, err := ConvertClaudeRequestToOpenAIRequest(request, "gpt-4.1", false)
	require.NoError(t, err)
	require.Len(t, converted.Messages, 2)
	require.Equal(t, "system", converted.Messages[0].Role)
	parts := converted.Messages[0].ParseContent()
	require.Len(t, parts, 1)
	require.Equal(t, "User system prompt", parts[0].Text)
}

func TestResponseOpenAI2ClaudeWithTranslator_RestoresToolNameAndUsage(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			ForkTranslator: relaycommon.ClaudeTranslatorState{OriginalRequestRawJSON: []byte(`{"tools":[{"name":"Get_Weather","description":"Look up weather","input_schema":{"type":"object"}}]}`)},
		},
	}
	response := &dto.OpenAITextResponse{
		Id:     "chatcmpl_123",
		Model:  "gpt-4.1",
		Object: "chat.completion",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:             "assistant",
					Content:          "It is sunny.",
					ReasoningContent: "I checked the tool output.",
					ToolCalls:        json.RawMessage(`[{"id":"call:1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}]`),
				},
				FinishReason: "tool_calls",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     12,
			CompletionTokens: 7,
			TotalTokens:      19,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens: 5,
			},
		},
	}

	claudeResp, err := ResponseOpenAI2ClaudeWithTranslator(response, info)
	require.NoError(t, err)
	require.NotNil(t, claudeResp)
	require.Equal(t, "message", claudeResp.Type)
	require.Equal(t, "assistant", claudeResp.Role)
	require.Equal(t, "chatcmpl_123", claudeResp.Id)
	require.Equal(t, "gpt-4.1", claudeResp.Model)
	require.Equal(t, "tool_use", claudeResp.StopReason)
	require.Len(t, claudeResp.Content, 3)

	require.Equal(t, "thinking", claudeResp.Content[0].Type)
	require.NotNil(t, claudeResp.Content[0].Thinking)
	require.Equal(t, "I checked the tool output.", *claudeResp.Content[0].Thinking)
	require.Equal(t, "text", claudeResp.Content[1].Type)
	require.Equal(t, "It is sunny.", claudeResp.Content[1].GetText())
	require.Equal(t, "tool_use", claudeResp.Content[2].Type)
	require.Equal(t, "Get_Weather", claudeResp.Content[2].Name)
	require.Equal(t, map[string]any{"city": "Paris"}, claudeResp.Content[2].Input)
	require.NotEmpty(t, claudeResp.Content[2].Id)
	require.NotContains(t, claudeResp.Content[2].Id, ":")

	require.NotNil(t, claudeResp.Usage)
	require.Equal(t, 7, claudeResp.Usage.OutputTokens)
	require.Equal(t, 5, claudeResp.Usage.CacheReadInputTokens)
	require.Equal(t, 7, claudeResp.Usage.InputTokens)
}

func TestResponseOpenAI2ClaudeWithTranslator_RestoresSanitizedToolName(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			ForkTranslator: relaycommon.ClaudeTranslatorState{OriginalRequestRawJSON: []byte(`{"tools":[{"name":"mcp/server/read","description":"Read tool","input_schema":{"type":"object"}}]}`)},
		},
	}
	response := &dto.OpenAITextResponse{
		Id:     "chatcmpl_sanitized",
		Model:  "gpt-4.1",
		Object: "chat.completion",
		Choices: []dto.OpenAITextResponseChoice{{
			Index: 0,
			Message: dto.Message{
				Role:      "assistant",
				ToolCalls: json.RawMessage(`[{"id":"call_1","type":"function","function":{"name":"mcp_server_read","arguments":"{}"}}]`),
			},
			FinishReason: "tool_calls",
		}},
	}

	claudeResp, err := ResponseOpenAI2ClaudeWithTranslator(response, info)
	require.NoError(t, err)
	require.NotNil(t, claudeResp)
	require.Len(t, claudeResp.Content, 1)
	require.Equal(t, "tool_use", claudeResp.Content[0].Type)
	require.Equal(t, "mcp/server/read", claudeResp.Content[0].Name)
}

func TestStreamResponseOpenAI2ClaudeWithTranslator_EmitsThinkingToolAndStop(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
			ForkTranslator:   relaycommon.ClaudeTranslatorState{OriginalRequestRawJSON: []byte(`{"tools":[{"name":"Get_Weather"}]}`)},
		},
	}

	responses, err := StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_stream",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ReasoningContent: common.GetPointer("Need to call tool."),
			},
		}},
	}, info)
	require.NoError(t, err)
	require.Len(t, responses, 3)
	require.Equal(t, "message_start", responses[0].Type)
	require.Equal(t, "content_block_start", responses[1].Type)
	require.Equal(t, "thinking", responses[1].ContentBlock.Type)
	require.Equal(t, "content_block_delta", responses[2].Type)
	require.NotNil(t, responses[2].Delta)
	require.NotNil(t, responses[2].Delta.Thinking)
	require.Equal(t, "Need to call tool.", *responses[2].Delta.Thinking)

	responses, err = StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_stream",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: common.GetPointer(0),
					ID:    "call:1",
					Type:  "function",
					Function: dto.FunctionResponse{
						Name: "get_weather",
					},
				}},
			},
		}},
	}, info)
	require.NoError(t, err)
	require.Len(t, responses, 2)
	require.Equal(t, "content_block_stop", responses[0].Type)
	require.Equal(t, "content_block_start", responses[1].Type)
	require.Equal(t, "tool_use", responses[1].ContentBlock.Type)
	require.Equal(t, "Get_Weather", responses[1].ContentBlock.Name)
	require.NotEmpty(t, responses[1].ContentBlock.Id)

	responses, err = StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_stream",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index: 0,
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: common.GetPointer(0),
					Function: dto.FunctionResponse{
						Arguments: `{"city":"Paris` + "\"}" + ``,
					},
				}},
			},
		}},
	}, info)
	require.NoError(t, err)
	require.Empty(t, responses)

	finishReason := "tool_calls"
	responses, err = StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_stream",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Index:        0,
			FinishReason: &finishReason,
		}},
		Usage: &dto.Usage{
			PromptTokens:     11,
			CompletionTokens: 4,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens: 3,
			},
		},
	}, info)
	require.NoError(t, err)
	require.Len(t, responses, 4)
	require.Equal(t, "content_block_delta", responses[0].Type)
	require.Equal(t, "input_json_delta", responses[0].Delta.Type)
	require.NotNil(t, responses[0].Delta.PartialJson)
	require.JSONEq(t, `{"city":"Paris"}`, *responses[0].Delta.PartialJson)
	require.Equal(t, "content_block_stop", responses[1].Type)
	require.Equal(t, "message_delta", responses[2].Type)
	require.NotNil(t, responses[2].Delta)
	require.NotNil(t, responses[2].Delta.StopReason)
	require.Equal(t, "tool_use", *responses[2].Delta.StopReason)
	require.NotNil(t, responses[2].Usage)
	require.Equal(t, 8, responses[2].Usage.InputTokens)
	require.Equal(t, 4, responses[2].Usage.OutputTokens)
	require.Equal(t, 3, responses[2].Usage.CacheReadInputTokens)
	require.Equal(t, "message_stop", responses[3].Type)
	require.True(t, info.ClaudeConvertInfo.Done)
	require.Equal(t, "tool_calls", info.FinishReason)
}

func TestStreamResponseOpenAI2ClaudeWithTranslator_SuppressesEmptyToolName(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}

	responses, err := StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_empty_tool",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: common.GetPointer(0),
					ID:    "call_skip",
					Type:  "function",
					Function: dto.FunctionResponse{
						Name:      "",
						Arguments: "",
					},
				}},
			},
		}},
	}, info)
	require.NoError(t, err)
	require.Len(t, responses, 1)
	require.Equal(t, "message_start", responses[0].Type)

	finishReason := "tool_calls"
	responses, err = StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_empty_tool",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			FinishReason: &finishReason,
		}},
		Usage: &dto.Usage{PromptTokens: 1, CompletionTokens: 1},
	}, info)
	require.NoError(t, err)
	require.Len(t, responses, 2)
	require.Equal(t, "message_delta", responses[0].Type)
	require.NotNil(t, responses[0].Delta)
	require.NotNil(t, responses[0].Delta.StopReason)
	require.NotEqual(t, "tool_use", *responses[0].Delta.StopReason)
	require.Equal(t, "message_stop", responses[1].Type)
}

func TestStreamResponseOpenAI2ClaudeWithTranslator_BelatedToolStartUsesSyntheticID(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}

	responses, err := StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_belated_tool",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ToolCalls: []dto.ToolCallResponse{{
					Index: common.GetPointer(0),
					Type:  "function",
					Function: dto.FunctionResponse{
						Name:      "do_it",
						Arguments: `{"x":1}`,
					},
				}},
			},
		}},
	}, info)
	require.NoError(t, err)
	require.Len(t, responses, 1)
	require.Equal(t, "message_start", responses[0].Type)

	finishReason := "tool_calls"
	responses, err = StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_belated_tool",
		Object:  "chat.completion.chunk",
		Created: 1710000000,
		Model:   "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			FinishReason: &finishReason,
		}},
		Usage: &dto.Usage{PromptTokens: 1, CompletionTokens: 1},
	}, info)
	require.NoError(t, err)
	require.Len(t, responses, 5)
	require.Equal(t, "content_block_start", responses[0].Type)
	require.Equal(t, "tool_use", responses[0].ContentBlock.Type)
	require.Equal(t, "do_it", responses[0].ContentBlock.Name)
	require.Contains(t, responses[0].ContentBlock.Id, "toolu_")
	require.Equal(t, "content_block_delta", responses[1].Type)
	require.Equal(t, "content_block_stop", responses[2].Type)
	require.Equal(t, "message_delta", responses[3].Type)
	require.Equal(t, "message_stop", responses[4].Type)
}

func TestRestoreSanitizedToolNameAndSanitizedToolNameMap(t *testing.T) {
	raw := []byte(`{"tools":[{"name":"mcp/server/read","input_schema":{}},{"name":"tool@v2","input_schema":{}},{"name":"read/file","input_schema":{}},{"name":"read@file","input_schema":{}}]}`)
	m := sanitizedToolNameMap(raw)
	require.NotNil(t, m)
	require.Equal(t, "mcp/server/read", m["mcp_server_read"])
	require.Equal(t, "tool@v2", m["tool_v2"])
	require.Equal(t, "read/file", m["read_file"])
	require.Equal(t, "mcp/server/read", restoreSanitizedToolName(m, "mcp_server_read"))
	require.Equal(t, "unknown", restoreSanitizedToolName(m, "unknown"))
}

func TestConvertClaudeRequestToOpenAI_ToolSchemaAddsMissingObjectProperties(t *testing.T) {
	inputJSON := []byte(`{
		"model": "claude-3-opus",
		"tools": [
			{
				"name": "empty_params",
				"description": "No args",
				"input_schema": {"type": "object"}
			},
			{
				"name": "nested_params",
				"description": "Nested args",
				"input_schema": {
					"type": "object",
					"properties": {
						"nested": {"type": "object"},
						"items": {
							"type": "array",
							"items": {"type": "object"}
						}
					}
				}
			}
		],
		"messages": [{"role": "user", "content": "hello"}]
	}`)

	output := ConvertClaudeRequestToOpenAI("test-model", inputJSON, false)
	outputJSON := gjson.ParseBytes(output)

	if got := outputJSON.Get("tools.0.function.parameters.properties"); !got.Exists() || !got.IsObject() {
		t.Fatalf("root object properties missing or invalid: %s", outputJSON.Get("tools.0.function.parameters").Raw)
	}
	if got := outputJSON.Get("tools.1.function.parameters.properties.nested.properties"); !got.Exists() || !got.IsObject() {
		t.Fatalf("nested object properties missing or invalid: %s", outputJSON.Get("tools.1.function.parameters").Raw)
	}
	if got := outputJSON.Get("tools.1.function.parameters.properties.items.items.properties"); !got.Exists() || !got.IsObject() {
		t.Fatalf("array item object properties missing or invalid: %s", outputJSON.Get("tools.1.function.parameters").Raw)
	}
}

func TestConvertClaudeRequestToOpenAI_MessageSystemRoleWrapsAsUserReminder(t *testing.T) {
	inputJSON := `{
		"model": "claude-sonnet-4-5",
		"system": [{"type": "text", "text": "Top-level rules"}],
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]},
			{"role": "system", "content": "String mid-conversation rule"},
			{"role": "assistant", "content": [{"type": "text", "text": "Hi there"}]},
			{"role": "system", "content": [{"type": "text", "text": "Array mid-conversation rule"}]},
			{"role": "user", "content": [{"type": "text", "text": "Follow up"}]}
		]
	}`

	result := ConvertClaudeRequestToOpenAI("gpt-5", []byte(inputJSON), false)
	resultJSON := gjson.ParseBytes(result)
	messages := resultJSON.Get("messages").Array()

	if len(messages) != 6 {
		t.Fatalf("Expected 6 messages, got %d: %s", len(messages), resultJSON.Get("messages").Raw)
	}

	roles := make([]string, 0, len(messages))
	for _, message := range messages {
		roles = append(roles, message.Get("role").String())
	}
	if got, want := roles, []string{"system", "user", "user", "assistant", "user", "user"}; fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("Unexpected message roles: got %v, want %v", got, want)
	}

	systemContent := messages[0].Get("content").Array()
	if len(systemContent) != 1 {
		t.Fatalf("Expected only top-level system content, got %d items: %s", len(systemContent), messages[0].Get("content").Raw)
	}
	if got := systemContent[0].Get("text").String(); got != "Top-level rules" {
		t.Fatalf("system content = %q, want Top-level rules", got)
	}
	if got := messages[2].Get("content.0.text").String(); got != "<system-reminder>\nString mid-conversation rule\n</system-reminder>" {
		t.Fatalf("unexpected string reminder text: %q", got)
	}
	if got := messages[4].Get("content.0.text").String(); got != "<system-reminder>\nArray mid-conversation rule\n</system-reminder>" {
		t.Fatalf("unexpected array reminder text: %q", got)
	}
}

func validGPTChatReasoningSignature() string {
	raw := make([]byte, 1+8+16+16+32)
	raw[0] = 0x80
	raw[8] = 1
	for i := 9; i < len(raw); i++ {
		raw[i] = byte(i)
	}
	return base64.URLEncoding.EncodeToString(raw)
}
