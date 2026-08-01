// FORK-CUSTOM: Protect provider account credential update semantics.
package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyUpstreamProviderAccountRequestSyncAPIKey(t *testing.T) {
	autoSync := true
	account := &model.UpstreamProviderAccount{LoginUsername: "existing-user", SyncAPIKey: "existing-secret"}

	require.NoError(t, applyUpstreamProviderAccountRequest(
		account, "account", "external", nil, model.UpstreamProviderStatusEnabled, &autoSync, nil,
	))
	assert.Equal(t, "existing-secret", account.SyncAPIKey)
	assert.Equal(t, "existing-user", account.LoginUsername)

	replacement := "  replacement-secret  "
	replacementUsername := "  user@example.com  "
	require.NoError(t, applyUpstreamProviderAccountRequest(
		account, "account", "external", &replacementUsername, model.UpstreamProviderStatusEnabled, &autoSync, &replacement,
	))
	assert.Equal(t, "replacement-secret", account.SyncAPIKey)
	assert.Equal(t, "user@example.com", account.LoginUsername)

	clear := ""
	require.NoError(t, applyUpstreamProviderAccountRequest(
		account, "account", "external", nil, model.UpstreamProviderStatusEnabled, &autoSync, &clear,
	))
	assert.Empty(t, account.SyncAPIKey)
}

func TestBuildUpstreamProviderValidatesAdapterAuthenticationMethod(t *testing.T) {
	baseRequest := dto.UpstreamProviderUpsertRequest{
		Name: "provider", Endpoint: "https://provider.example.com", Status: model.UpstreamProviderStatusEnabled,
		AdapterType: "sub2api", AuthenticationMethod: "password", SyncIntervalMinutes: 60,
		RechargeRatio: decimal.NewFromInt(1), Currency: "USD",
	}

	provider, err := buildUpstreamProvider(nil, &baseRequest)
	require.NoError(t, err)
	assert.Equal(t, "sub2api", provider.AdapterType)
	assert.Equal(t, "password", provider.AuthenticationMethod)
	assert.True(t, provider.QuotaConversionBase.Equal(decimal.NewFromInt(1)))

	baseRequest.AuthenticationMethod = "api_key"
	_, err = buildUpstreamProvider(nil, &baseRequest)
	require.EqualError(t, err, "authentication method does not match provider adapter type")

	baseRequest.AdapterType = "newapi"
	_, err = buildUpstreamProvider(nil, &baseRequest)
	require.EqualError(t, err, "quota conversion base must be positive for NewAPI providers")

	baseRequest.QuotaConversionBase = decimal.NewFromInt(500000)
	_, err = buildUpstreamProvider(nil, &baseRequest)
	require.NoError(t, err)
}
