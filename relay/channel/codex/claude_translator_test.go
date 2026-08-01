// FORK-CUSTOM: Verify fork-owned Codex and channel-setting integration.
package codex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChannelOtherSettingsParsesCodexUpstreamProtocol(t *testing.T) {
	var settings dto.ChannelOtherSettings
	require.NoError(t, common.Unmarshal([]byte(`{"upstream_protocol":"codex"}`), &settings))
	require.Equal(t, dto.UpstreamProtocolCodex, settings.UpstreamProtocol)
}

func TestConvertClaudeRequestToCodexMapsToolsAndToolResult(t *testing.T) {
	raw := []byte(`{
		"model":"claude-sonnet",
		"system":"You are precise.",
		"messages":[
			{"role":"user","content":[{"type":"text","text":"weather?"}]},
			{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"GetWeather","input":{"city":"Paris"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"Sunny"}]}]}
		],
		"tools":[{"name":"GetWeather","description":"weather","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}],
		"tool_choice":{"type":"tool","name":"GetWeather"},
		"thinking":{"type":"enabled","budget_tokens":4096}
	}`)

	converted := ConvertClaudeRequestToCodex("gpt-5-codex", raw, true)
	require.Equal(t, "gpt-5-codex", gjson.GetBytes(converted, "model").String())
	require.Equal(t, "You are precise.", gjson.GetBytes(converted, "input.0.content.0.text").String())
	require.Equal(t, "function", gjson.GetBytes(converted, "tools.0.type").String())
	require.Equal(t, "GetWeather", gjson.GetBytes(converted, "tools.0.name").String())
	require.Equal(t, "function", gjson.GetBytes(converted, "tool_choice.type").String())
	require.Equal(t, "GetWeather", gjson.GetBytes(converted, "tool_choice.name").String())
	require.Equal(t, "function_call", gjson.GetBytes(converted, "input.2.type").String())
	require.Equal(t, "GetWeather", gjson.GetBytes(converted, "input.2.name").String())
	require.Equal(t, "function_call_output", gjson.GetBytes(converted, "input.3.type").String())
	require.Equal(t, "medium", gjson.GetBytes(converted, "reasoning.effort").String())
	require.True(t, gjson.GetBytes(converted, "stream").Bool())
	require.False(t, gjson.GetBytes(converted, "store").Bool())
}

func TestConvertCodexResponseToClaudeNonStreamRestoresToolUse(t *testing.T) {
	original := []byte(`{"tools":[{"name":"GetWeather","input_schema":{"type":"object"}}]}`)
	raw := []byte(`{
		"type":"response.completed",
		"response":{
			"id":"resp_1",
			"model":"gpt-5-codex",
			"output":[
				{"type":"function_call","call_id":"toolu_1","name":"GetWeather","arguments":"{\"city\":\"Paris\"}"}
			],
			"usage":{"input_tokens":10,"output_tokens":4,"input_tokens_details":{"cached_tokens":3}}
		}
	}`)

	out := ConvertCodexResponseToClaudeNonStream(context.Background(), "gpt-5-codex", original, nil, raw, nil)
	require.NotEmpty(t, out)
	require.Equal(t, "message", gjson.GetBytes(out, "type").String())
	require.Equal(t, "tool_use", gjson.GetBytes(out, "content.0.type").String())
	require.Equal(t, "GetWeather", gjson.GetBytes(out, "content.0.name").String())
	require.Equal(t, "Paris", gjson.GetBytes(out, "content.0.input.city").String())
	require.Equal(t, "tool_use", gjson.GetBytes(out, "stop_reason").String())
	require.Equal(t, int64(7), gjson.GetBytes(out, "usage.input_tokens").Int())
	require.Equal(t, int64(3), gjson.GetBytes(out, "usage.cache_read_input_tokens").Int())
}

func TestCodexAdaptorPlainKeyUsesThirdPartyResponsesEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://third-party.example",
			ApiKey:         "sk-third-party",
		},
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://third-party.example/v1/responses", url)

	headers := http.Header{}
	require.NoError(t, adaptor.SetupRequestHeader(ctx, &headers, info))
	require.Equal(t, "Bearer sk-third-party", headers.Get("Authorization"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Empty(t, headers.Get("chatgpt-account-id"))
}

func TestCodexAdaptorOAuthKeyUsesOfficialBackendEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeResponses,
		RequestURLPath: "/v1/responses",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://chatgpt.com",
			ApiKey:         `{"access_token":"access","account_id":"account"}`,
		},
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://chatgpt.com/backend-api/codex/responses", url)

	headers := http.Header{}
	require.NoError(t, adaptor.SetupRequestHeader(ctx, &headers, info))
	require.Equal(t, "Bearer access", headers.Get("Authorization"))
	require.Equal(t, "account", headers.Get("chatgpt-account-id"))
	require.Equal(t, "responses=experimental", headers.Get("OpenAI-Beta"))
}

func TestConvertClaudeRequestToCodex_ServiceTier(t *testing.T) {
	tests := []struct {
		name            string
		serviceTierJSON string
		want            string
		wantExists      bool
	}{
		{
			name:            "Priority passes through",
			serviceTierJSON: `"priority"`,
			want:            "priority",
			wantExists:      true,
		},
		{
			name:            "Fast normalizes to priority",
			serviceTierJSON: `"fast"`,
			want:            "priority",
			wantExists:      true,
		},
		{
			name:            "Unsupported tier is omitted",
			serviceTierJSON: `"default"`,
		},
		{
			name:            "Non-string tier is omitted",
			serviceTierJSON: `true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON := `{
				"model": "gpt-5.4",
				"service_tier": ` + tt.serviceTierJSON + `,
				"messages": [{"role": "user", "content": "Reply with OK"}]
			}`

			result := ConvertClaudeRequestToCodex("gpt-5.4", []byte(inputJSON), false)
			serviceTierResult := gjson.GetBytes(result, "service_tier")
			if serviceTierResult.Exists() != tt.wantExists {
				t.Fatalf("service_tier exists = %v, want %v. Output: %s", serviceTierResult.Exists(), tt.wantExists, string(result))
			}
			if !tt.wantExists {
				return
			}
			if got := serviceTierResult.String(); got != tt.want {
				t.Fatalf("service_tier = %q, want %q. Output: %s", got, tt.want, string(result))
			}
		})
	}
}

func TestConvertClaudeRequestToCodex_FastSpeedUsesPriorityTier(t *testing.T) {
	result := ConvertClaudeRequestToCodex("gpt-5.4", []byte(`{
		"model":"gpt-5.4",
		"speed":"fast",
		"service_tier":"default",
		"messages":[{"role":"user","content":"Reply with OK"}]
	}`), false)

	require.Equal(t, "priority", gjson.GetBytes(result, "service_tier").String())
}

func TestConvertClaudeRequestToCodex_DoesNotForceReasoningSummary(t *testing.T) {
	result := ConvertClaudeRequestToCodex("gpt-5.4", []byte(`{
		"model":"gpt-5.4",
		"thinking":{"type":"enabled","budget_tokens":1024},
		"messages":[{"role":"user","content":"Reply with OK"}]
	}`), false)

	require.False(t, gjson.GetBytes(result, "reasoning.summary").Exists())
}

func TestConvertClaudeRequestToCodex_MessageSystemRoleWrapsAsUserReminder(t *testing.T) {
	inputJSON := `{
		"model": "claude-3-opus",
		"system": [{"type": "text", "text": "Top-level rules"}],
		"messages": [
			{"role": "user", "content": "hello"},
			{"role": "system", "content": "Follow the project instructions"},
			{"role": "assistant", "content": [{"type": "text", "text": "ok"}]},
			{"role": "system", "content": [{"type": "text", "text": "Use the current repo"}]}
		]
	}`

	result := ConvertClaudeRequestToCodex("test-model", []byte(inputJSON), false)
	inputs := gjson.GetBytes(result, "input").Array()
	if len(inputs) != 5 {
		t.Fatalf("got %d input items, want 5: %s", len(inputs), gjson.GetBytes(result, "input").Raw)
	}

	if got := inputs[0].Get("role").String(); got != "developer" {
		t.Fatalf("top-level system role = %q, want developer", got)
	}
	if got := inputs[2].Get("role").String(); got != "user" {
		t.Fatalf("message-level system role = %q, want user", got)
	}
	if got := inputs[2].Get("content.0.text").String(); got != "<system-reminder>\nFollow the project instructions\n</system-reminder>" {
		t.Fatalf("unexpected first reminder text: %q", got)
	}
	if got := inputs[4].Get("role").String(); got != "user" {
		t.Fatalf("array message-level system role = %q, want user", got)
	}
	if got := inputs[4].Get("content.0.text").String(); got != "<system-reminder>\nUse the current repo\n</system-reminder>" {
		t.Fatalf("unexpected second reminder text: %q", got)
	}
}
