// FORK-CUSTOM: Keep model-provider management transport contracts isolated from existing channel DTOs.
package dto

import "github.com/shopspring/decimal"

type UpstreamProviderUpsertRequest struct {
	Name                 string          `json:"name"`
	Endpoint             string          `json:"endpoint"`
	Status               string          `json:"status"`
	AuthenticationMethod string          `json:"authentication_method"`
	AdapterType          string          `json:"adapter_type"`
	SyncIntervalMinutes  int             `json:"sync_interval_minutes"`
	RechargeRatio        decimal.Decimal `json:"recharge_ratio"`
	QuotaConversionBase  decimal.Decimal `json:"quota_conversion_base"`
	Currency             string          `json:"currency"`
}

type UpstreamProviderAccountRequest struct {
	Name          string  `json:"name"`
	ExternalId    string  `json:"external_id"`
	LoginUsername *string `json:"login_username"`
	LoginPassword *string `json:"login_password"`
	Status        string  `json:"status"`
	AutoSync      *bool   `json:"auto_sync"`
	SyncAPIKey    *string `json:"sync_api_key"`
}

type UpstreamProviderAccountRechargeAdjustRequest struct {
	Delta decimal.Decimal `json:"delta"`
}

type UpstreamProviderWorkspaceUpsertRequest struct {
	UpstreamProviderUpsertRequest
	ID       *int                                      `json:"id"`
	Accounts []UpstreamProviderWorkspaceAccountRequest `json:"accounts"`
}

type UpstreamProviderWorkspaceAccountRequest struct {
	UpstreamProviderAccountRequest
	ID     *int                                    `json:"id"`
	Groups []UpstreamProviderWorkspaceGroupRequest `json:"groups"`
}

type UpstreamProviderWorkspaceGroupRequest struct {
	ID         *int   `json:"id"`
	Name       string `json:"name"`
	ChannelIds []int  `json:"channel_ids"`
}

type UpstreamProviderProfitQuery struct {
	StartDate string `form:"start_date"`
	EndDate   string `form:"end_date"`
}

type UpstreamProviderResponse struct {
	ID                   int                               `json:"id"`
	Name                 string                            `json:"name"`
	Endpoint             string                            `json:"endpoint"`
	Status               string                            `json:"status"`
	AuthenticationMethod string                            `json:"authentication_method"`
	AdapterType          string                            `json:"adapter_type"`
	SyncIntervalMinutes  int                               `json:"sync_interval_minutes"`
	RechargeRatio        decimal.Decimal                   `json:"recharge_ratio"`
	QuotaConversionBase  decimal.Decimal                   `json:"quota_conversion_base"`
	Currency             string                            `json:"currency"`
	SyncStatus           string                            `json:"sync_status"`
	LastSyncedAt         int64                             `json:"last_synced_at"`
	LastSyncError        string                            `json:"last_sync_error"`
	CreatedAt            int64                             `json:"created_at"`
	UpdatedAt            int64                             `json:"updated_at"`
	Accounts             []UpstreamProviderAccountResponse `json:"accounts"`
}

type UpstreamProviderAccountResponse struct {
	ID                int                             `json:"id"`
	ProviderID        int                             `json:"provider_id"`
	Name              string                          `json:"name"`
	ExternalID        string                          `json:"external_id"`
	LoginUsername     string                          `json:"login_username"`
	Status            string                          `json:"status"`
	AutoSync          bool                            `json:"auto_sync"`
	HasSyncAPIKey     bool                            `json:"has_sync_api_key"`
	Balance           decimal.Decimal                 `json:"balance"`
	TotalRecharge     decimal.Decimal                 `json:"total_recharge"`
	BalanceUpdatedAt  int64                           `json:"balance_updated_at"`
	SyncStatus        string                          `json:"sync_status"`
	LastSyncedAt      int64                           `json:"last_synced_at"`
	LastSyncAttemptAt int64                           `json:"last_sync_attempt_at"`
	NextSyncAt        int64                           `json:"next_sync_at"`
	LastSyncError     string                          `json:"last_sync_error"`
	Groups            []UpstreamProviderGroupResponse `json:"groups"`
}

type UpstreamProviderGroupResponse struct {
	ID         int                           `json:"id"`
	AccountID  int                           `json:"account_id"`
	Name       string                        `json:"name"`
	ChannelIDs []int                         `json:"channel_ids"`
	Keys       []UpstreamProviderKeyResponse `json:"keys"`
}

type UpstreamProviderKeyResponse struct {
	ID              int    `json:"id"`
	AccountID       int    `json:"account_id"`
	ProviderGroupID int    `json:"provider_group_id"`
	Name            string `json:"name"`
	ExternalID      string `json:"external_id"`
	KeyMasked       string `json:"key_masked"`
	Status          string `json:"status"`
	LastUsageAt     int64  `json:"last_usage_at"`
}
