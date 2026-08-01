// FORK-CUSTOM: Collect NewAPI account, provider-group cost, and token snapshots.
package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

type newAPIProviderSyncAdapter struct {
	client *http.Client
}

const (
	providerTokenPageSize = 100
	maxProviderTokenPages = 100
)

func RegisterUpstreamProviderSyncAdapters() error {
	if err := RegisterProviderSyncAdapter("newapi", &newAPIProviderSyncAdapter{client: &http.Client{Timeout: 30 * time.Second}}); err != nil {
		return err
	}
	return RegisterProviderSyncAdapter("sub2api", &sub2APIProviderSyncAdapter{client: newSub2APIHTTPClient()})
}

func (a *newAPIProviderSyncAdapter) ValidateAccount(_ context.Context, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount, credentials ProviderSyncCredentials) error {
	if strings.TrimSpace(provider.Endpoint) == "" {
		return fmt.Errorf("provider endpoint is required")
	}
	if strings.TrimSpace(credentials.SyncAPIKey) == "" {
		return fmt.Errorf("provider API key is required")
	}
	if strings.TrimSpace(account.ExternalId) == "" {
		return fmt.Errorf("account external ID is required for NewAPI synchronization")
	}
	return nil
}

func (a *newAPIProviderSyncAdapter) SyncAccountSnapshot(ctx context.Context, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount, credentials ProviderSyncCredentials) (*ProviderAccountSnapshot, error) {
	const snapshotPath = "/api/user/self"
	body, err := a.request(ctx, provider, account, credentials, snapshotPath)
	if err != nil {
		return nil, err
	}
	data := responseData(body)
	snapshot := &ProviderAccountSnapshot{ObservedAt: common.GetTimestamp(), RawSource: "/api/user/self"}
	if value, ok := firstProviderNumber(data, "remain_quota", "quota", "balance", "total_quota"); ok {
		snapshot.Balance = value.Div(provider.QuotaConversionBase).Div(provider.RechargeRatio)
		snapshot.BalanceAvailable = true
	}
	if value, ok := firstProviderNumber(data, "total_recharge", "recharged", "total_recharged"); ok {
		snapshot.TotalRecharge = value.Div(provider.QuotaConversionBase).Div(provider.RechargeRatio)
		snapshot.TotalRechargeAvailable = true
	}
	keys, groups, costsMissing, keyErr := a.collectKeys(ctx, provider, account, credentials)
	if keyErr != nil {
		snapshot.MissingFields = append(snapshot.MissingFields, "keys")
	} else {
		snapshot.Keys = keys
		snapshot.Groups = groups
		snapshot.RawSource = snapshotPath + ",/api/token/?p=1&page_size=100,/api/log/self/stat?group"
		if costsMissing {
			snapshot.MissingFields = append(snapshot.MissingFields, "costs")
		}
	}
	return snapshot, nil
}

func (a *newAPIProviderSyncAdapter) collectKeys(ctx context.Context, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount, credentials ProviderSyncCredentials) ([]ProviderKeySnapshot, []ProviderGroupSnapshot, bool, error) {
	items := make([]map[string]any, 0)
	itemsByExternalID := make(map[string]map[string]any)
	for page := 1; page <= maxProviderTokenPages; page++ {
		path := fmt.Sprintf("/api/token/?p=%d&page_size=%d", page, providerTokenPageSize)
		body, err := a.request(ctx, provider, account, credentials, path)
		if err != nil {
			return nil, nil, false, err
		}
		pageItems := providerTokenItems(responseData(body))
		for _, item := range pageItems {
			externalID := firstProviderString(item, "id", "token_id", "key")
			if externalID == "" || itemsByExternalID[externalID] != nil {
				continue
			}
			items = append(items, item)
			itemsByExternalID[externalID] = item
		}
		if len(pageItems) < providerTokenPageSize {
			break
		}
		if page == maxProviderTokenPages {
			return nil, nil, false, fmt.Errorf("provider token list exceeds %d pages", maxProviderTokenPages)
		}
	}
	for _, storedGroup := range account.Groups {
		for _, storedKey := range storedGroup.Keys {
			externalID := strings.TrimSpace(storedKey.ExternalId)
			if externalID == "" || itemsByExternalID[externalID] != nil {
				continue
			}
			tokenBody, tokenErr := a.request(ctx, provider, account, credentials, "/api/token/"+url.PathEscape(externalID))
			if tokenErr != nil {
				continue
			}
			item := responseData(tokenBody)
			if firstProviderString(item, "id", "token_id", "key") == "" {
				continue
			}
			items = append(items, item)
			itemsByExternalID[externalID] = item
		}
	}
	now := time.Now().In(time.Local)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	dayEnd := dayStart.AddDate(0, 0, 1).Add(-time.Second)
	costDate := dayStart.Format("2006-01-02")
	costObservedAt := now.Unix()
	keys := make([]ProviderKeySnapshot, 0, len(items))
	groups := make([]ProviderGroupSnapshot, 0)
	costsMissing := false
	collectedGroups := make(map[string]struct{})
	for _, item := range items {
		externalID := firstProviderString(item, "id", "token_id", "key")
		if externalID == "" {
			continue
		}
		group := firstProviderString(item, "group", "token_group")
		rawKey := firstProviderString(item, "key", "token")
		keys = append(keys, ProviderKeySnapshot{
			ExternalID:  externalID,
			Name:        firstProviderString(item, "name", "token_name"),
			GroupName:   group,
			MaskedKey:   maskProviderKey(rawKey),
			Fingerprint: common.GenerateHMAC(rawKey),
			Status:      providerKeyStatus(item),
			LastUsageAt: providerUnixTime(item, "accessed_time", "accessed_at", "last_used_at"),
		})
		if group == "" {
			costsMissing = true
			continue
		}
		if _, collected := collectedGroups[group]; collected {
			continue
		}
		collectedGroups[group] = struct{}{}
		query := url.Values{}
		query.Set("type", "0")
		query.Set("model_name", "")
		query.Set("start_timestamp", strconv.FormatInt(dayStart.Unix(), 10))
		query.Set("end_timestamp", strconv.FormatInt(dayEnd.Unix(), 10))
		query.Set("token_name", "")
		query.Set("group", group)
		statBody, statErr := a.request(ctx, provider, account, credentials, "/api/log/self/stat?"+query.Encode())
		quota, statReady := firstProviderNumber(responseData(statBody), "quota")
		statReady = statErr == nil && statReady && !quota.IsNegative()
		groupSnapshot := ProviderGroupSnapshot{
			GroupName: group, CostDate: costDate, CostObservedAt: costObservedAt,
			CostStatus: "unavailable",
		}
		if statReady {
			groupSnapshot.ProviderUsageQuota = quota
			groupSnapshot.ProviderCost = quota.Div(provider.QuotaConversionBase).Div(provider.RechargeRatio)
			groupSnapshot.CostStatus = "ready"
		} else {
			costsMissing = true
		}
		groups = append(groups, groupSnapshot)
	}
	return keys, groups, costsMissing, nil
}

func (a *newAPIProviderSyncAdapter) request(ctx context.Context, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount, credentials ProviderSyncCredentials, path string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	base := strings.TrimRight(strings.TrimSpace(provider.Endpoint), "/")
	if base == "" {
		return nil, fmt.Errorf("provider endpoint is required")
	}
	pathURL, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("invalid provider request path: %w", err)
	}
	if !strings.HasPrefix(pathURL.Path, "/") || pathURL.IsAbs() || pathURL.Host != "" {
		return nil, fmt.Errorf("provider request path must be relative")
	}
	joined, err := url.JoinPath(base, pathURL.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid provider endpoint")
	}
	u := joined
	if pathURL.RawQuery != "" {
		u += "?" + pathURL.RawQuery
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create provider request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+credentials.SyncAPIKey)
	if strings.TrimSpace(account.ExternalId) != "" {
		request.Header.Set("New-Api-User", strings.TrimSpace(account.ExternalId))
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("provider request failed")
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read provider response")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("provider request %s returned status %d", path, response.StatusCode)
	}
	var result map[string]any
	if err := common.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("decode provider response")
	}
	return result, nil
}

func responseData(payload map[string]any) map[string]any {
	if data, ok := payload["data"].(map[string]any); ok {
		return data
	}
	return payload
}

func providerTokenItems(data map[string]any) []map[string]any {
	for _, value := range []any{data["items"], data["data"], data["tokens"]} {
		if items, ok := value.([]any); ok {
			result := make([]map[string]any, 0, len(items))
			for _, item := range items {
				if entry, ok := item.(map[string]any); ok {
					result = append(result, entry)
				}
			}
			return result
		}
	}
	return nil
}

func firstProviderString(data map[string]any, names ...string) string {
	for _, name := range names {
		switch value := data[name].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		case int:
			return strconv.Itoa(value)
		}
	}
	return ""
}

func firstProviderNumber(data map[string]any, names ...string) (decimal.Decimal, bool) {
	for _, name := range names {
		switch value := data[name].(type) {
		case float64:
			return decimal.NewFromFloat(value), true
		case int:
			return decimal.NewFromInt(int64(value)), true
		case string:
			if parsed, err := decimal.NewFromString(strings.TrimSpace(value)); err == nil {
				return parsed, true
			}
		}
	}
	return decimal.Zero, false
}

func providerUnixTime(data map[string]any, names ...string) int64 {
	value, ok := firstProviderNumber(data, names...)
	if !ok {
		return 0
	}
	return value.IntPart()
}

func providerKeyStatus(data map[string]any) string {
	if value, ok := firstProviderNumber(data, "status"); ok && value.IntPart() != 1 {
		return model.UpstreamProviderKeyStatusDisabled
	}
	return model.UpstreamProviderKeyStatusActive
}

func maskProviderKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + "****" + key[len(key)-4:]
}
