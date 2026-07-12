// FORK-CUSTOM: Provide an explicit composition boundary for CPA-derived Claude translators.
package service

import (
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type ClaudeTranslator struct {
	Request  func(dto.ClaudeRequest, *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error)
	Response func(*dto.OpenAITextResponse, *relaycommon.RelayInfo) (*dto.ClaudeResponse, error)
	Stream   func(*dto.ChatCompletionsStreamResponse, *relaycommon.RelayInfo) ([]*dto.ClaudeResponse, error)
}

var (
	claudeTranslatorMu         sync.RWMutex
	claudeTranslator           = defaultClaudeTranslator()
	claudeTranslatorRegistered bool
)

func defaultClaudeTranslator() ClaudeTranslator {
	return ClaudeTranslator{
		Request: ClaudeToOpenAIRequest,
		Response: func(response *dto.OpenAITextResponse, info *relaycommon.RelayInfo) (*dto.ClaudeResponse, error) {
			return ResponseOpenAI2Claude(response, info), nil
		},
		Stream: func(response *dto.ChatCompletionsStreamResponse, info *relaycommon.RelayInfo) ([]*dto.ClaudeResponse, error) {
			return StreamResponseOpenAI2Claude(response, info), nil
		},
	}
}

func RegisterClaudeTranslator(translator ClaudeTranslator) error {
	if translator.Request == nil || translator.Response == nil || translator.Stream == nil {
		return fmt.Errorf("claude translator requires request, response, and stream implementations")
	}

	claudeTranslatorMu.Lock()
	defer claudeTranslatorMu.Unlock()
	if claudeTranslatorRegistered {
		return fmt.Errorf("claude translator is already registered")
	}
	claudeTranslator = translator
	claudeTranslatorRegistered = true
	return nil
}

func TranslateClaudeRequest(claudeRequest dto.ClaudeRequest, info *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
	claudeTranslatorMu.RLock()
	translator := claudeTranslator.Request
	claudeTranslatorMu.RUnlock()
	return translator(claudeRequest, info)
}

func OpenAIResponseToClaude(response *dto.OpenAITextResponse, info *relaycommon.RelayInfo) (*dto.ClaudeResponse, error) {
	claudeTranslatorMu.RLock()
	translator := claudeTranslator.Response
	claudeTranslatorMu.RUnlock()
	return translator(response, info)
}

func OpenAIStreamResponseToClaude(response *dto.ChatCompletionsStreamResponse, info *relaycommon.RelayInfo) ([]*dto.ClaudeResponse, error) {
	claudeTranslatorMu.RLock()
	translator := claudeTranslator.Stream
	claudeTranslatorMu.RUnlock()
	return translator(response, info)
}
