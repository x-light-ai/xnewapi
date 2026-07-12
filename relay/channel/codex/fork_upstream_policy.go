// FORK-CUSTOM: Isolate third-party Codex URL and API-key policy from the upstream adaptor.
package codex

import (
	"errors"
	"net/http"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func thirdPartyRequestURL(info *relaycommon.RelayInfo) (string, bool) {
	if isOfficialCodexOAuthKey(info.ApiKey) {
		return "", false
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, info.RequestURLPath, info.ChannelType), true
}

func setupThirdPartyRequestHeader(req *http.Header, info *relaycommon.RelayInfo) (bool, error) {
	key := strings.TrimSpace(info.ApiKey)
	if isOfficialCodexOAuthKey(key) {
		return false, nil
	}
	if key == "" {
		return true, errors.New("codex channel: api key is required")
	}

	req.Set("Authorization", "Bearer "+key)
	req.Set("Content-Type", "application/json")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}
	return true, nil
}

func isOfficialCodexOAuthKey(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), "{")
}
