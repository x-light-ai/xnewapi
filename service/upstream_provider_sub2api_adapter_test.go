// FORK-CUSTOM: Verify Sub2API login, key-scoped actual cost collection, and normalization.
package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSub2APIProviderSyncAdapterCollectsBalanceKeysAndActualCost(t *testing.T) {
	loginRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var data any
		switch request.URL.Path {
		case "/api/v1/auth/login":
			loginRequests++
			assert.Equal(t, http.MethodPost, request.Method)
			origin := "http://" + request.Host
			assert.Equal(t, origin, request.Header.Get("Origin"))
			assert.Equal(t, origin+"/", request.Header.Get("Referer"))
			var credentials map[string]string
			require.NoError(t, common.DecodeJson(request.Body, &credentials))
			assert.Equal(t, "user@example.com", credentials["email"])
			assert.Equal(t, "sync-password", credentials["password"])
			data = map[string]any{"access_token": "access-token", "token_type": "Bearer"}
		case "/api/v1/auth/me":
			assert.Equal(t, "Bearer access-token", request.Header.Get("Authorization"))
			assert.Equal(t, sub2APITimezone, request.URL.Query().Get("timezone"))
			data = map[string]any{
				"id": 4841, "email": "user@example.com", "balance": 48.84449169, "total_recharged": 181,
			}
		case "/api/v1/keys":
			assert.Equal(t, "1", request.URL.Query().Get("page"))
			assert.Equal(t, "100", request.URL.Query().Get("page_size"))
			data = map[string]any{
				"items": []any{map[string]any{
					"id": 4050, "name": "production", "key": "sk-abcdefgh1234", "status": "active",
					"last_used_at": "2026-07-31T14:00:00Z",
				}},
				"page": 1, "page_size": 100, "pages": 1,
			}
		case "/api/v1/usage/dashboard/snapshot-v2":
			query := request.URL.Query()
			assert.Equal(t, "4050", query.Get("api_key_id"))
			assert.Equal(t, query.Get("start_date"), query.Get("end_date"))
			assert.Equal(t, "hour", query.Get("granularity"))
			assert.Equal(t, "true", query.Get("include_trend"))
			assert.Equal(t, "false", query.Get("include_model_stats"))
			assert.Equal(t, "true", query.Get("include_group_stats"))
			assert.Equal(t, sub2APITimezone, query.Get("timezone"))
			data = map[string]any{
				"trend": []any{
					map[string]any{"actual_cost": 0.057282426},
					map[string]any{"actual_cost": 0.5247279},
				},
			}
		default:
			http.NotFound(writer, request)
			return
		}
		payload, err := common.Marshal(map[string]any{"code": 0, "message": "success", "data": data})
		require.NoError(t, err)
		writer.Header().Set("Content-Type", "application/json")
		_, err = writer.Write(payload)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	adapter := &sub2APIProviderSyncAdapter{client: &http.Client{Timeout: time.Second}}
	provider := &model.UpstreamProvider{Endpoint: server.URL}
	account := &model.UpstreamProviderAccount{}
	snapshot, err := adapter.SyncAccountSnapshot(context.Background(), provider, account, ProviderSyncCredentials{Username: "user@example.com", SyncAPIKey: "sync-password"})
	require.NoError(t, err)
	assert.Equal(t, 1, loginRequests)
	assert.True(t, snapshot.Balance.Equal(decimal.RequireFromString("48.84449169")))
	assert.True(t, snapshot.TotalRecharge.Equal(decimal.NewFromInt(181)))
	assert.Equal(t, "4841", snapshot.AccountExternalID)
	require.Len(t, snapshot.Keys, 1)
	assert.Equal(t, "4050", snapshot.Keys[0].ExternalID)
	assert.Equal(t, "production (#4050)", snapshot.Keys[0].GroupName)
	assert.Equal(t, model.UpstreamProviderKeyStatusActive, snapshot.Keys[0].Status)
	assert.NotContains(t, snapshot.Keys[0].MaskedKey, "abcdefgh")
	assert.Equal(t, int64(1785506400), snapshot.Keys[0].LastUsageAt)
	require.Len(t, snapshot.Groups, 1)
	assert.True(t, snapshot.Groups[0].ProviderCost.Equal(decimal.RequireFromString("0.582010326")))
	assert.True(t, snapshot.Groups[0].ProviderUsageQuota.IsZero())
	assert.Equal(t, "ready", snapshot.Groups[0].CostStatus)
	assert.Empty(t, snapshot.MissingFields)
}

func TestSub2APIProviderSyncAdapterKeepsSameNamedKeysInSeparateCostGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var data any
		switch request.URL.Path {
		case "/api/v1/auth/login":
			data = map[string]any{"access_token": "access-token"}
		case "/api/v1/auth/me":
			data = map[string]any{"balance": 1, "total_recharged": 2}
		case "/api/v1/keys":
			data = map[string]any{"items": []any{
				map[string]any{"id": 7, "name": "shared", "status": "active"},
				map[string]any{"id": 8, "name": "shared", "status": "inactive"},
			}, "pages": 1}
		case "/api/v1/usage/dashboard/snapshot-v2":
			cost := 0.25
			if request.URL.Query().Get("api_key_id") == "8" {
				cost = 0.75
			}
			data = map[string]any{"trend": []any{map[string]any{"actual_cost": cost}}}
		default:
			http.NotFound(writer, request)
			return
		}
		payload, err := common.Marshal(map[string]any{"code": 0, "message": "success", "data": data})
		require.NoError(t, err)
		_, err = writer.Write(payload)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	adapter := &sub2APIProviderSyncAdapter{client: &http.Client{Timeout: time.Second}}
	provider := &model.UpstreamProvider{Endpoint: server.URL}
	snapshot, err := adapter.SyncAccountSnapshot(context.Background(), provider, &model.UpstreamProviderAccount{}, ProviderSyncCredentials{Username: "user@example.com", SyncAPIKey: "password"})
	require.NoError(t, err)
	require.Len(t, snapshot.Groups, 2)
	assert.Equal(t, "shared (#7)", snapshot.Groups[0].GroupName)
	assert.Equal(t, "shared (#8)", snapshot.Groups[1].GroupName)
	assert.True(t, snapshot.Groups[0].ProviderCost.Equal(decimal.RequireFromString("0.25")))
	assert.True(t, snapshot.Groups[1].ProviderCost.Equal(decimal.RequireFromString("0.75")))
	assert.Equal(t, model.UpstreamProviderKeyStatusDisabled, snapshot.Keys[1].Status)
}

func TestSub2APIProviderSyncAdapterRejectsNegativeActualCost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var data any
		switch request.URL.Path {
		case "/api/v1/auth/login":
			data = map[string]any{"access_token": "access-token"}
		case "/api/v1/auth/me":
			data = map[string]any{"balance": 1, "total_recharged": 2}
		case "/api/v1/keys":
			data = map[string]any{"items": []any{map[string]any{"id": 7, "name": "key", "status": "active"}}, "pages": 1}
		case "/api/v1/usage/dashboard/snapshot-v2":
			data = map[string]any{"trend": []any{map[string]any{"actual_cost": -1}}}
		default:
			http.NotFound(writer, request)
			return
		}
		payload, err := common.Marshal(map[string]any{"code": 0, "message": "success", "data": data})
		require.NoError(t, err)
		_, err = writer.Write(payload)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	adapter := &sub2APIProviderSyncAdapter{client: &http.Client{Timeout: time.Second}}
	provider := &model.UpstreamProvider{Endpoint: server.URL}
	snapshot, err := adapter.SyncAccountSnapshot(context.Background(), provider, &model.UpstreamProviderAccount{}, ProviderSyncCredentials{Username: "user@example.com", SyncAPIKey: "password"})
	require.NoError(t, err)
	require.Len(t, snapshot.Groups, 1)
	assert.Equal(t, "unavailable", snapshot.Groups[0].CostStatus)
	assert.Equal(t, []string{"cost:7"}, snapshot.MissingFields)
}
