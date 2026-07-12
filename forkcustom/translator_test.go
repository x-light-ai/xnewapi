// FORK-CUSTOM: Verify explicit CPA translator registration at the application boundary.
package forkcustom

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

func TestRegisterClaudeTranslator(t *testing.T) {
	var translator service.ClaudeTranslator
	require.NoError(t, registerClaudeTranslatorWith(func(candidate service.ClaudeTranslator) error {
		translator = candidate
		return nil
	}))
	require.NotNil(t, translator.Request)
	require.NotNil(t, translator.Response)
	require.NotNil(t, translator.Stream)

	info := &relaycommon.RelayInfo{
		ChannelMeta:       &relaycommon.ChannelMeta{UpstreamModelName: "gpt-4.1"},
		IsStream:          true,
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{},
	}
	request := dto.ClaudeRequest{
		Model:    "claude-sonnet-4",
		Messages: []dto.ClaudeMessage{{Role: "user", Content: "hello"}},
	}

	converted, err := translator.Request(request, info)
	require.NoError(t, err)
	require.Equal(t, "gpt-4.1", converted.Model)
	require.NotEmpty(t, info.ClaudeConvertInfo.ForkTranslator.OriginalRequestRawJSON)
}
