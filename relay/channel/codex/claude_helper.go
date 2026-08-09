// FORK-CUSTOM: Relay Claude requests through CPA-derived Codex translation.
package codex

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func ClaudeHelper(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (*dto.Usage, *types.NewAPIError) {
	requestRawJSON, err := common.Marshal(request)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	info.ForkTranslator.OriginalRequestRawJSON = requestRawJSON
	info.AppendRequestConversion(types.RelayFormatOpenAIResponses)

	converted := ConvertClaudeRequestToCodex(info.UpstreamModelName, requestRawJSON, info.IsStream)
	var responsesReq dto.OpenAIResponsesRequest
	if err := common.Unmarshal(converted, &responsesReq); err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if !info.IsStream {
		responsesReq.Stream = common.GetPointer(false)
	}

	adaptor := &Adaptor{}
	adaptor.Init(info)

	savedRelayMode := info.RelayMode
	savedRequestURLPath := info.RequestURLPath
	defer func() {
		info.RelayMode = savedRelayMode
		info.RequestURLPath = savedRequestURLPath
	}()

	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	convertedRequest, err := convertClaudeCodexResponsesRequest(c, info, adaptor, responsesReq)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid, types.ErrOptionWithSkipRetry())
		}
	}

	respAny, err := adaptor.DoRequest(c, info, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	resp, ok := respAny.(*http.Response)
	if !ok || resp == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	if resp.StatusCode != http.StatusOK {
		newApiErr := service.RelayErrorHandler(c.Request.Context(), resp, false)
		service.ResetStatusCode(newApiErr, statusCodeMappingStr)
		return nil, newApiErr
	}

	info.FinalRequestRelayFormat = types.RelayFormatClaude
	if info.IsStream || strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		if info.IsStream {
			return codexClaudeStreamHandler(c, info, resp)
		}
		return codexClaudeStreamToNonStreamHandler(c, info, resp)
	}
	return codexClaudeNonStreamHandler(c, info, resp)
}

func convertClaudeCodexResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, adaptor *Adaptor, responsesReq dto.OpenAIResponsesRequest) (any, error) {
	savedChannelSetting := info.ChannelSetting
	info.ChannelSetting.SystemPrompt = ""
	info.ChannelSetting.SystemPromptOverride = false
	defer func() {
		info.ChannelSetting = savedChannelSetting
	}()

	return adaptor.ConvertOpenAIResponsesRequest(c, info, responsesReq)
}

func codexClaudeStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	usage := &dto.Usage{UsageSemantic: "anthropic"}
	var streamErr *types.NewAPIError
	state := info.ForkTranslator.StreamState

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}

		eventPayload := []byte("data:" + data)
		for _, out := range ConvertCodexResponseToClaude(c.Request.Context(), info.UpstreamModelName, info.ForkTranslator.OriginalRequestRawJSON, nil, eventPayload, &state) {
			if len(out) == 0 {
				continue
			}
			if _, err := c.Writer.Write(out); err != nil {
				streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				sr.Stop(streamErr)
				return
			}
			if err := helper.FlushWriter(c); err != nil {
				streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				sr.Stop(streamErr)
				return
			}
		}

		if parsedUsage := codexClaudeUsageFromEvent([]byte(data)); parsedUsage != nil {
			usage = parsedUsage
			usage.UsageSemantic = "anthropic"
		}
		eventType := gjson.Get(data, "type").String()
		if eventType == "response.completed" || eventType == "response.incomplete" {
			sr.Done()
		}
	})

	info.ForkTranslator.StreamState = state
	if streamErr != nil {
		return nil, streamErr
	}
	if usage.TotalTokens == 0 {
		usage = service.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())
		usage.UsageSemantic = "anthropic"
	}
	return usage, nil
}

func codexClaudeStreamToNonStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var completed []byte
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, dataTag) {
			continue
		}
		payload := bytes.TrimSpace(line[len(dataTag):])
		if gjson.GetBytes(payload, "type").String() == "response.completed" ||
			gjson.GetBytes(payload, "type").String() == "response.incomplete" {
			completed = payload
		}
	}
	if len(completed) == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("missing completed response event"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	claudeBody := ConvertCodexResponseToClaudeNonStream(c.Request.Context(), info.UpstreamModelName, info.ForkTranslator.OriginalRequestRawJSON, nil, completed, nil)
	if len(claudeBody) == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("failed to convert codex response to claude"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, claudeBody)
	usage := codexClaudeUsageFromEvent(completed)
	if usage == nil {
		usage = service.ResponseText2Usage(c, string(claudeBody), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	usage.UsageSemantic = "anthropic"
	return usage, nil
}

func codexClaudeNonStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	wrapped := []byte(`{"type":"response.completed","response":null}`)
	if gjson.GetBytes(body, "type").String() == "response.completed" ||
		gjson.GetBytes(body, "type").String() == "response.incomplete" {
		wrapped = body
	} else {
		wrapped, _ = sjson.SetRawBytes(wrapped, "response", body)
	}

	claudeBody := ConvertCodexResponseToClaudeNonStream(c.Request.Context(), info.UpstreamModelName, info.ForkTranslator.OriginalRequestRawJSON, nil, wrapped, nil)
	if len(claudeBody) == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("failed to convert codex response to claude"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, claudeBody)
	usage := codexClaudeUsageFromEvent(wrapped)
	if usage == nil {
		usage = service.ResponseText2Usage(c, string(claudeBody), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	usage.UsageSemantic = "anthropic"
	return usage, nil
}

func codexClaudeUsageFromEvent(raw []byte) *dto.Usage {
	responseData := gjson.GetBytes(raw, "response")
	if !responseData.Exists() {
		responseData = gjson.ParseBytes(raw)
	}
	inputTokens, outputTokens, cachedTokens := extractResponsesUsage(responseData.Get("usage"))
	if inputTokens == 0 && outputTokens == 0 && cachedTokens == 0 {
		return nil
	}
	promptTokens := int(inputTokens)
	completionTokens := int(outputTokens)
	return &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		InputTokens:      promptTokens,
		OutputTokens:     completionTokens,
		UsageSemantic:    "anthropic",
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: int(cachedTokens),
		},
	}
}
