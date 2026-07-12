// FORK-CUSTOM: Verify registered Claude translator facades without leaking global test state.
package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func resetClaudeTranslatorForTest(t *testing.T) {
	t.Helper()
	claudeTranslatorMu.Lock()
	originalTranslator := claudeTranslator
	originalRegistered := claudeTranslatorRegistered
	claudeTranslator = defaultClaudeTranslator()
	claudeTranslatorRegistered = false
	claudeTranslatorMu.Unlock()
	t.Cleanup(func() {
		claudeTranslatorMu.Lock()
		claudeTranslator = originalTranslator
		claudeTranslatorRegistered = originalRegistered
		claudeTranslatorMu.Unlock()
	})
}

func TestClaudeTranslatorFacades(t *testing.T) {
	resetClaudeTranslatorForTest(t)
	info := &relaycommon.RelayInfo{}

	require.NoError(t, RegisterClaudeTranslator(ClaudeTranslator{
		Request: func(dto.ClaudeRequest, *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
			return &dto.GeneralOpenAIRequest{Model: "registered-request"}, nil
		},
		Response: func(*dto.OpenAITextResponse, *relaycommon.RelayInfo) (*dto.ClaudeResponse, error) {
			return &dto.ClaudeResponse{Type: "registered-response"}, nil
		},
		Stream: func(*dto.ChatCompletionsStreamResponse, *relaycommon.RelayInfo) ([]*dto.ClaudeResponse, error) {
			return []*dto.ClaudeResponse{{Type: "registered-stream"}}, nil
		},
	}))

	request, err := TranslateClaudeRequest(dto.ClaudeRequest{}, info)
	require.NoError(t, err)
	require.Equal(t, "registered-request", request.Model)

	response, err := OpenAIResponseToClaude(&dto.OpenAITextResponse{}, info)
	require.NoError(t, err)
	require.Equal(t, "registered-response", response.Type)

	stream, err := OpenAIStreamResponseToClaude(&dto.ChatCompletionsStreamResponse{}, info)
	require.NoError(t, err)
	require.Len(t, stream, 1)
	require.Equal(t, "registered-stream", stream[0].Type)

	require.Error(t, RegisterClaudeTranslator(defaultClaudeTranslator()))
}

func TestRegisterClaudeTranslatorRejectsIncompleteImplementation(t *testing.T) {
	resetClaudeTranslatorForTest(t)
	require.Error(t, RegisterClaudeTranslator(ClaudeTranslator{}))
}
