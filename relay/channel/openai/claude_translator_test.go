package openai

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
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
	require.Equal(t, "Need weather tool", converted.Messages[2].ReasoningContent)
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

func TestResponseOpenAI2ClaudeWithTranslator_RestoresToolNameAndUsage(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			OriginalRequestRawJSON: []byte(`{"tools":[{"name":"Get_Weather","description":"Look up weather","input_schema":{"type":"object"}}]}`),
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
			OriginalRequestRawJSON: []byte(`{"tools":[{"name":"mcp/server/read","description":"Read tool","input_schema":{"type":"object"}}]}`),
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
			LastMessagesType:       relaycommon.LastMessageTypeNone,
			OriginalRequestRawJSON: []byte(`{"tools":[{"name":"Get_Weather"}]}`),
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
