// FORK-CUSTOM: Verify NewAPI provider collection headers, normalization, and key masking.
package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPIProviderSyncAdapterCollectsSnapshotAndKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "Bearer sync-secret", request.Header.Get("Authorization"))
		assert.Equal(t, "42", request.Header.Get("New-Api-User"))
		var payload map[string]any
		switch request.URL.Path {
		case "/api/user/self":
			payload = map[string]any{"data": map[string]any{"remain_quota": 500000, "total_recharge": 1000000}}
		case "/api/token/":
			assert.Equal(t, "1", request.URL.Query().Get("p"))
			assert.Equal(t, "100", request.URL.Query().Get("page_size"))
			payload = map[string]any{"data": map[string]any{"items": []any{map[string]any{
				"id": 7, "name": "primary", "group": "primary-group", "key": "sk-abcdefgh1234", "status": 1,
			}}}}
		case "/api/log/self/stat":
			assert.Equal(t, "0", request.URL.Query().Get("type"))
			assert.Empty(t, request.URL.Query().Get("token_name"))
			assert.Equal(t, "primary-group", request.URL.Query().Get("group"))
			start, err := strconv.ParseInt(request.URL.Query().Get("start_timestamp"), 10, 64)
			require.NoError(t, err)
			end, err := strconv.ParseInt(request.URL.Query().Get("end_timestamp"), 10, 64)
			require.NoError(t, err)
			assert.Equal(t, 0, time.Unix(start, 0).In(time.Local).Hour())
			assert.Equal(t, int64(24*time.Hour/time.Second-1), end-start)
			payload = map[string]any{"data": map[string]any{"quota": 250000}}
		default:
			http.NotFound(writer, request)
			return
		}
		body, err := common.Marshal(payload)
		require.NoError(t, err)
		writer.Header().Set("Content-Type", "application/json")
		_, err = writer.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	adapter := &newAPIProviderSyncAdapter{client: &http.Client{Timeout: time.Second}}
	provider := &model.UpstreamProvider{
		Endpoint:            server.URL,
		QuotaConversionBase: decimal.NewFromInt(500000), RechargeRatio: decimal.NewFromInt(1),
	}
	account := &model.UpstreamProviderAccount{ExternalId: "42"}
	snapshot, err := adapter.SyncAccountSnapshot(context.Background(), provider, account, ProviderSyncCredentials{SyncAPIKey: "sync-secret"})
	require.NoError(t, err)
	assert.True(t, snapshot.Balance.Equal(decimal.NewFromInt(1)))
	assert.True(t, snapshot.TotalRecharge.Equal(decimal.NewFromInt(2)))
	require.Len(t, snapshot.Keys, 1)
	assert.Equal(t, "7", snapshot.Keys[0].ExternalID)
	assert.NotContains(t, snapshot.Keys[0].MaskedKey, "abcdefgh")
	require.Len(t, snapshot.Groups, 1)
	assert.Equal(t, "primary-group", snapshot.Groups[0].GroupName)
	assert.True(t, snapshot.Groups[0].ProviderUsageQuota.Equal(decimal.NewFromInt(250000)))
	assert.True(t, snapshot.Groups[0].ProviderCost.Equal(decimal.NewFromFloat(0.5)))
	assert.Equal(t, "ready", snapshot.Groups[0].CostStatus)
}

func TestNewAPIProviderSyncAdapterResolvesRenamedKeyByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		switch request.URL.Path {
		case "/api/user/self":
			payload = map[string]any{"data": map[string]any{"remain_quota": 500000}}
		case "/api/token/":
			payload = map[string]any{"data": map[string]any{"items": []any{}}}
		case "/api/token/7":
			payload = map[string]any{"data": map[string]any{
				"id": 7, "name": "renamed-key", "group": "renamed-group", "key": "sk-abcdefgh1234", "status": 1,
			}}
		case "/api/log/self/stat":
			assert.Empty(t, request.URL.Query().Get("token_name"))
			assert.Equal(t, "renamed-group", request.URL.Query().Get("group"))
			payload = map[string]any{"data": map[string]any{"quota": 250000}}
		default:
			http.NotFound(writer, request)
			return
		}
		body, err := common.Marshal(payload)
		require.NoError(t, err)
		_, err = writer.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	adapter := &newAPIProviderSyncAdapter{client: &http.Client{Timeout: time.Second}}
	provider := &model.UpstreamProvider{
		Endpoint:            server.URL,
		QuotaConversionBase: decimal.NewFromInt(500000), RechargeRatio: decimal.NewFromInt(1),
	}
	account := &model.UpstreamProviderAccount{
		ExternalId: "42",
		Groups: []model.UpstreamProviderGroup{{
			Name: "old-group", Keys: []model.UpstreamProviderKey{{ExternalId: "7", Name: "old-key"}},
		}},
	}
	snapshot, err := adapter.SyncAccountSnapshot(context.Background(), provider, account, ProviderSyncCredentials{SyncAPIKey: "sync-secret"})
	require.NoError(t, err)
	require.Len(t, snapshot.Keys, 1)
	assert.Equal(t, "renamed-key", snapshot.Keys[0].Name)
	require.Len(t, snapshot.Groups, 1)
	assert.Equal(t, "renamed-group", snapshot.Groups[0].GroupName)
	assert.True(t, snapshot.Groups[0].ProviderCost.Equal(decimal.NewFromFloat(0.5)))
}

func TestNewAPIProviderSyncAdapterCollectsSharedGroupCostOnce(t *testing.T) {
	statRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		switch request.URL.Path {
		case "/api/user/self":
			payload = map[string]any{"data": map[string]any{"remain_quota": 500000}}
		case "/api/token/":
			payload = map[string]any{"data": map[string]any{"items": []any{
				map[string]any{"id": 7, "name": "current-key", "group": "key-group", "status": 1},
				map[string]any{"id": 8, "name": "second-key", "group": "key-group", "status": 1},
			}}}
		case "/api/log/self/stat":
			statRequests++
			assert.Empty(t, request.URL.Query().Get("token_name"))
			assert.Equal(t, "key-group", request.URL.Query().Get("group"))
			payload = map[string]any{"data": map[string]any{"quota": 125000}}
		default:
			http.NotFound(writer, request)
			return
		}
		body, err := common.Marshal(payload)
		require.NoError(t, err)
		_, err = writer.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	adapter := &newAPIProviderSyncAdapter{client: &http.Client{Timeout: time.Second}}
	provider := &model.UpstreamProvider{
		Endpoint:            server.URL,
		QuotaConversionBase: decimal.NewFromInt(500000), RechargeRatio: decimal.NewFromInt(1),
	}
	account := &model.UpstreamProviderAccount{ExternalId: "42"}
	snapshot, err := adapter.SyncAccountSnapshot(context.Background(), provider, account, ProviderSyncCredentials{SyncAPIKey: "sync-secret"})
	require.NoError(t, err)
	require.Len(t, snapshot.Keys, 2)
	require.Len(t, snapshot.Groups, 1)
	assert.Equal(t, 1, statRequests)
	assert.True(t, snapshot.Groups[0].ProviderCost.Equal(decimal.NewFromFloat(0.25)))
	assert.Equal(t, "ready", snapshot.Groups[0].CostStatus)
}

func TestNewAPIProviderSyncAdapterRejectsNonRelativeRequestPaths(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requestCount++
		http.Error(writer, "unexpected request", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	adapter := &newAPIProviderSyncAdapter{client: &http.Client{Timeout: time.Second}}
	provider := &model.UpstreamProvider{Endpoint: server.URL}
	account := &model.UpstreamProviderAccount{ExternalId: "42"}
	credentials := ProviderSyncCredentials{SyncAPIKey: "sync-secret"}
	for _, path := range []string{
		server.URL + "/collect",
		"//" + strings.TrimPrefix(server.URL, "http://") + "/collect",
		"api/user/self",
		"%",
	} {
		t.Run(path, func(t *testing.T) {
			_, err := adapter.request(context.Background(), provider, account, credentials, path)
			require.Error(t, err)
		})
	}
	assert.Zero(t, requestCount)
}

func TestNewAPIProviderSyncAdapterPaginatesTokenList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		switch request.URL.Path {
		case "/api/user/self":
			payload = map[string]any{"data": map[string]any{"remain_quota": 500000}}
		case "/api/token/":
			page := request.URL.Query().Get("p")
			items := make([]any, 0, 100)
			if page == "1" {
				for id := 1; id <= 100; id++ {
					items = append(items, map[string]any{"id": id, "group": "first", "status": 1})
				}
			} else if page == "2" {
				items = append(items, map[string]any{"id": 101, "group": "second", "status": 1})
			}
			payload = map[string]any{"data": map[string]any{"items": items}}
		case "/api/log/self/stat":
			payload = map[string]any{"data": map[string]any{"quota": 1}}
		default:
			http.NotFound(writer, request)
			return
		}
		body, err := common.Marshal(payload)
		require.NoError(t, err)
		_, err = writer.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	adapter := &newAPIProviderSyncAdapter{client: &http.Client{Timeout: time.Second}}
	provider := &model.UpstreamProvider{
		Endpoint: server.URL, QuotaConversionBase: decimal.NewFromInt(500000), RechargeRatio: decimal.NewFromInt(1),
	}
	snapshot, err := adapter.SyncAccountSnapshot(context.Background(), provider, &model.UpstreamProviderAccount{ExternalId: "42"}, ProviderSyncCredentials{SyncAPIKey: "sync-secret"})
	require.NoError(t, err)
	require.Len(t, snapshot.Keys, 101)
	require.Len(t, snapshot.Groups, 2)
}
