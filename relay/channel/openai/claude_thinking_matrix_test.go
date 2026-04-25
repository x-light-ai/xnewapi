package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

// mapReasoningEffort mapping matrix
func TestMapReasoningEffort(t *testing.T) {
	cases := []struct {
		thinkingType string
		budget       *int
		effort       string
		want         string
	}{
		{"enabled", nil, "", "medium"},
		{"enabled", common.GetPointer(0), "", "none"},
		{"enabled", common.GetPointer(1024), "", "low"},
		{"enabled", common.GetPointer(2048), "", "low"},
		{"enabled", common.GetPointer(4096), "", "medium"},
		{"enabled", common.GetPointer(8192), "", "medium"},
		{"enabled", common.GetPointer(16384), "", "high"},
		{"enabled", common.GetPointer(32000), "", "high"},
		{"adaptive", nil, "high", "high"},
		{"adaptive", nil, "medium", "medium"},
		{"adaptive", nil, "low", "low"},
		{"adaptive", nil, "", "high"},
		{"auto", nil, "medium", "medium"},
		{"disabled", nil, "", "none"},
		{"", nil, "", ""},
	}
	for _, tc := range cases {
		got := mapReasoningEffort(tc.thinkingType, tc.budget, tc.effort)
		require.Equal(t, tc.want, got, "thinkingType=%q budget=%v effort=%q", tc.thinkingType, tc.budget, tc.effort)
	}
}

// ConvertClaudeRequestToOpenAIRequest: thinking -> reasoning_effort
func TestConvertClaudeRequest_ThinkingToReasoningEffort(t *testing.T) {
	cases := []struct {
		name    string
		thinking *dto.Thinking
		want    string
	}{
		{"enabled budget=4096 -> medium", &dto.Thinking{Type: "enabled", BudgetTokens: common.GetPointer(4096)}, "medium"},
		{"enabled budget=16384 -> high", &dto.Thinking{Type: "enabled", BudgetTokens: common.GetPointer(16384)}, "high"},
		{"enabled budget=1024 -> low", &dto.Thinking{Type: "enabled", BudgetTokens: common.GetPointer(1024)}, "low"},
		{"enabled no budget -> medium", &dto.Thinking{Type: "enabled"}, "medium"},
		{"disabled -> none", &dto.Thinking{Type: "disabled"}, "none"},
		{"nil thinking -> empty", nil, ""},
	}
	maxTokens := uint(1024)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &dto.ClaudeRequest{
				Model:     "claude-3-7-sonnet",
				MaxTokens: &maxTokens,
				Thinking:  tc.thinking,
				Messages: []dto.ClaudeMessage{
					{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: dto.ContentTypeText, Text: common.GetPointer("hi")}}},
				},
			}
			converted, err := ConvertClaudeRequestToOpenAIRequest(req, "gpt-4.1", false)
			require.NoError(t, err)
			require.Equal(t, tc.want, converted.ReasoningEffort)
		})
	}
}

// adaptive thinking with output_config.effort
func TestConvertClaudeRequest_AdaptiveThinkingEffort(t *testing.T) {
	maxTokens := uint(1024)
	req := &dto.ClaudeRequest{
		Model:        "claude-opus-4-7",
		MaxTokens:    &maxTokens,
		Thinking:     &dto.Thinking{Type: "adaptive"},
		OutputConfig: []byte(`{"effort":"high"}`),
		Messages: []dto.ClaudeMessage{
			{Role: "user", Content: []dto.ClaudeMediaMessage{{Type: dto.ContentTypeText, Text: common.GetPointer("hi")}}},
		},
	}
	converted, err := ConvertClaudeRequestToOpenAIRequest(req, "gpt-4.1", false)
	require.NoError(t, err)
	require.Equal(t, "high", converted.ReasoningEffort)
}

// Stream: thinking block -> content_block_start{type:thinking} + content_block_delta{type:thinking_delta}
func TestStreamThinkingBlock_EmitsCorrectEvents(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}

	// First chunk: reasoning content
	responses, err := StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ReasoningContent: common.GetPointer("thinking..."),
			},
		}},
	}, info)
	require.NoError(t, err)
	// message_start, content_block_start{thinking}, content_block_delta{thinking_delta}
	require.Len(t, responses, 3)
	require.Equal(t, "message_start", responses[0].Type)
	require.Equal(t, "content_block_start", responses[1].Type)
	require.Equal(t, "thinking", responses[1].ContentBlock.Type)
	require.Equal(t, "content_block_delta", responses[2].Type)
	require.NotNil(t, responses[2].Delta)
	require.Equal(t, "thinking_delta", responses[2].Delta.Type)
	require.NotNil(t, responses[2].Delta.Thinking)
	require.Equal(t, "thinking...", *responses[2].Delta.Thinking)

	// Second chunk: more reasoning
	responses, err = StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ReasoningContent: common.GetPointer(" more"),
			},
		}},
	}, info)
	require.NoError(t, err)
	// only delta, no new block_start
	require.Len(t, responses, 1)
	require.Equal(t, "content_block_delta", responses[0].Type)
	require.Equal(t, "thinking_delta", responses[0].Delta.Type)

	// Third chunk: text content (thinking block closes, text block opens)
	responses, err = StreamResponseOpenAI2ClaudeWithTranslator(&dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-4.1",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				Content: common.GetPointer("answer"),
			},
		}},
	}, info)
	require.NoError(t, err)
	// content_block_stop (thinking), content_block_start (text), content_block_delta (text_delta)
	require.Len(t, responses, 3)
	require.Equal(t, "content_block_stop", responses[0].Type)
	require.Equal(t, "content_block_start", responses[1].Type)
	require.Equal(t, "text", responses[1].ContentBlock.Type)
	require.Equal(t, "content_block_delta", responses[2].Type)
	require.Equal(t, "text_delta", responses[2].Delta.Type)
}
