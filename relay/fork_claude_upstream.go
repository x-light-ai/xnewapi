// FORK-CUSTOM: Isolate third-party Codex upstream dispatch from the upstream Claude handler.
package relay

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	codexchannel "github.com/QuantumNous/new-api/relay/channel/codex"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func handleForkClaudeUpstream(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest, passThrough bool) (bool, *types.NewAPIError) {
	if passThrough || !strings.EqualFold(info.ChannelOtherSettings.UpstreamProtocol, dto.UpstreamProtocolCodex) {
		return false, nil
	}

	usage, newAPIError := codexchannel.ClaudeHelper(c, info, request)
	if newAPIError != nil {
		return true, newAPIError
	}
	service.PostTextConsumeQuota(c, info, usage, nil)
	return true, nil
}
