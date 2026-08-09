// FORK-CUSTOM: Register CPA-derived translators at the application composition boundary.
package forkcustom

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
)

type claudeTranslatorRegistrar func(service.ClaudeTranslator) error

func buildClaudeTranslator() service.ClaudeTranslator {
	return service.ClaudeTranslator{
		Request: func(request dto.ClaudeRequest, info *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
			requestRawJSON, err := common.Marshal(&request)
			if err != nil {
				return nil, err
			}
			info.ForkTranslator.OriginalRequestRawJSON = requestRawJSON
			return openai.ConvertClaudeRequestToOpenAIRequest(&request, info.UpstreamModelName, info.IsStream)
		},
		Response: openai.ResponseOpenAI2ClaudeWithTranslator,
		Stream:   openai.StreamResponseOpenAI2ClaudeWithTranslator,
	}
}

func registerClaudeTranslatorWith(register claudeTranslatorRegistrar) error {
	return register(buildClaudeTranslator())
}

func registerClaudeTranslator() error {
	return registerClaudeTranslatorWith(service.RegisterClaudeTranslator)
}
