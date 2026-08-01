// FORK-CUSTOM: Collect Sub2API account, API-key cost units, and daily actual costs.
package service

import (
	"bytes"
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

const (
	sub2APIPageSize = 100
	sub2APIMaxPages = 100
	sub2APITimezone = "Asia/Shanghai"
)

var sub2APILocation = time.FixedZone(sub2APITimezone, 8*60*60)

type sub2APIProviderSyncAdapter struct {
	client *http.Client
}

type sub2APIEnvelope[T any] struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	Data        T      `json:"data"`
	Requires2FA bool   `json:"requires_2fa"`
}

type sub2APILoginData struct {
	AccessToken string `json:"access_token"`
	Requires2FA bool   `json:"requires_2fa"`
}

type sub2APIMeData struct {
	ID             int64           `json:"id"`
	Email          string          `json:"email"`
	Balance        decimal.Decimal `json:"balance"`
	TotalRecharged decimal.Decimal `json:"total_recharged"`
}

type sub2APIKey struct {
	ID         int64      `json:"id"`
	Key        string     `json:"key"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type sub2APIKeyPage struct {
	Items []sub2APIKey `json:"items"`
	Pages int          `json:"pages"`
}

type sub2APICostPoint struct {
	ActualCost decimal.Decimal `json:"actual_cost"`
}

type sub2APIUsageSnapshot struct {
	Trend  []sub2APICostPoint `json:"trend"`
	Groups []sub2APICostPoint `json:"groups"`
}

func newSub2APIHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (a *sub2APIProviderSyncAdapter) ValidateAccount(_ context.Context, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount, credentials ProviderSyncCredentials) error {
	if strings.TrimSpace(provider.Endpoint) == "" {
		return fmt.Errorf("provider endpoint is required")
	}
	if strings.TrimSpace(credentials.Username) == "" {
		return fmt.Errorf("login username is required for Sub2API synchronization")
	}
	if strings.TrimSpace(credentials.SyncAPIKey) == "" {
		return fmt.Errorf("account password is required for Sub2API synchronization")
	}
	return nil
}

func (a *sub2APIProviderSyncAdapter) SyncAccountSnapshot(ctx context.Context, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount, credentials ProviderSyncCredentials) (*ProviderAccountSnapshot, error) {
	loginPayload := map[string]string{
		"email":    strings.TrimSpace(credentials.Username),
		"password": credentials.SyncAPIKey,
	}
	var login sub2APILoginData
	if err := a.request(ctx, provider, http.MethodPost, "/api/v1/auth/login", "", loginPayload, &login); err != nil {
		return nil, fmt.Errorf("Sub2API login failed: %w", err)
	}
	if login.Requires2FA || strings.TrimSpace(login.AccessToken) == "" {
		return nil, fmt.Errorf("Sub2API login requires an unsupported second authentication step")
	}

	var currentUser sub2APIMeData
	mePath := "/api/v1/auth/me?timezone=" + url.QueryEscape(sub2APITimezone)
	if err := a.request(ctx, provider, http.MethodGet, mePath, login.AccessToken, nil, &currentUser); err != nil {
		return nil, fmt.Errorf("fetch Sub2API account: %w", err)
	}
	if currentUser.Balance.IsNegative() || currentUser.TotalRecharged.IsNegative() {
		return nil, fmt.Errorf("Sub2API account returned a negative balance or recharge total")
	}

	keys, err := a.collectKeys(ctx, provider, login.AccessToken)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(sub2APILocation)
	costDate := now.Format("2006-01-02")
	snapshot := &ProviderAccountSnapshot{
		AccountExternalID: strconv.FormatInt(currentUser.ID, 10),
		Balance:           currentUser.Balance, TotalRecharge: currentUser.TotalRecharged,
		BalanceAvailable: true, TotalRechargeAvailable: true,
		ObservedAt: common.GetTimestamp(), RawSource: mePath + ",/api/v1/keys,/api/v1/usage/dashboard/snapshot-v2",
		Keys: make([]ProviderKeySnapshot, 0, len(keys)), Groups: make([]ProviderGroupSnapshot, 0, len(keys)),
	}
	for _, key := range keys {
		groupName := sub2APIKeyGroupName(key)
		lastUsageAt := int64(0)
		if key.LastUsedAt != nil {
			lastUsageAt = key.LastUsedAt.Unix()
		}
		status := model.UpstreamProviderKeyStatusDisabled
		if strings.EqualFold(strings.TrimSpace(key.Status), "active") {
			status = model.UpstreamProviderKeyStatusActive
		}
		snapshot.Keys = append(snapshot.Keys, ProviderKeySnapshot{
			ExternalID: strconv.FormatInt(key.ID, 10), Name: strings.TrimSpace(key.Name), GroupName: groupName,
			MaskedKey: maskProviderKey(key.Key), Fingerprint: common.GenerateHMAC(key.Key), Status: status, LastUsageAt: lastUsageAt,
		})

		actualCost, costErr := a.collectKeyCost(ctx, provider, login.AccessToken, key.ID, costDate)
		groupSnapshot := ProviderGroupSnapshot{
			GroupName: groupName, CostDate: costDate, CostObservedAt: now.Unix(), CostStatus: "unavailable",
		}
		if costErr != nil {
			snapshot.MissingFields = append(snapshot.MissingFields, "cost:"+strconv.FormatInt(key.ID, 10))
		} else {
			groupSnapshot.ProviderCost = actualCost
			groupSnapshot.ProviderUsageQuota = decimal.Zero
			groupSnapshot.CostStatus = "ready"
		}
		snapshot.Groups = append(snapshot.Groups, groupSnapshot)
	}
	return snapshot, nil
}

func (a *sub2APIProviderSyncAdapter) collectKeys(ctx context.Context, provider *model.UpstreamProvider, accessToken string) ([]sub2APIKey, error) {
	keys := make([]sub2APIKey, 0)
	seen := make(map[int64]struct{})
	for page := 1; page <= sub2APIMaxPages; page++ {
		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(sub2APIPageSize))
		var result sub2APIKeyPage
		if err := a.request(ctx, provider, http.MethodGet, "/api/v1/keys?"+query.Encode(), accessToken, nil, &result); err != nil {
			return nil, fmt.Errorf("fetch Sub2API keys: %w", err)
		}
		for _, key := range result.Items {
			if key.ID <= 0 {
				continue
			}
			if _, exists := seen[key.ID]; exists {
				continue
			}
			seen[key.ID] = struct{}{}
			keys = append(keys, key)
		}
		if len(result.Items) < sub2APIPageSize || (result.Pages > 0 && page >= result.Pages) {
			return keys, nil
		}
	}
	return nil, fmt.Errorf("Sub2API key list exceeds %d pages", sub2APIMaxPages)
}

func (a *sub2APIProviderSyncAdapter) collectKeyCost(ctx context.Context, provider *model.UpstreamProvider, accessToken string, keyID int64, costDate string) (decimal.Decimal, error) {
	query := url.Values{}
	query.Set("start_date", costDate)
	query.Set("end_date", costDate)
	query.Set("api_key_id", strconv.FormatInt(keyID, 10))
	query.Set("granularity", "hour")
	query.Set("include_trend", "true")
	query.Set("include_model_stats", "false")
	query.Set("include_group_stats", "true")
	query.Set("timezone", sub2APITimezone)
	var result sub2APIUsageSnapshot
	if err := a.request(ctx, provider, http.MethodGet, "/api/v1/usage/dashboard/snapshot-v2?"+query.Encode(), accessToken, nil, &result); err != nil {
		return decimal.Zero, err
	}
	points := result.Trend
	if points == nil {
		points = result.Groups
	}
	total := decimal.Zero
	for _, point := range points {
		if point.ActualCost.IsNegative() {
			return decimal.Zero, fmt.Errorf("Sub2API returned a negative actual cost")
		}
		total = total.Add(point.ActualCost)
	}
	return total, nil
}

func sub2APIKeyGroupName(key sub2APIKey) string {
	name := strings.TrimSpace(key.Name)
	if name == "" {
		name = "API Key"
	}
	return fmt.Sprintf("%s (#%d)", name, key.ID)
}

func (a *sub2APIProviderSyncAdapter) request(ctx context.Context, provider *model.UpstreamProvider, method string, path string, accessToken string, body any, output any) error {
	base := strings.TrimRight(strings.TrimSpace(provider.Endpoint), "/")
	endpointURL, err := url.Parse(base)
	if err != nil || (endpointURL.Scheme != "http" && endpointURL.Scheme != "https") || endpointURL.Host == "" {
		return fmt.Errorf("invalid provider endpoint")
	}
	origin := endpointURL.Scheme + "://" + endpointURL.Host
	pathURL, err := url.Parse(path)
	if err != nil || !strings.HasPrefix(pathURL.Path, "/") || pathURL.IsAbs() || pathURL.Host != "" {
		return fmt.Errorf("invalid Sub2API request path")
	}
	joined, err := url.JoinPath(origin, pathURL.Path)
	if err != nil {
		return fmt.Errorf("invalid provider endpoint")
	}
	if pathURL.RawQuery != "" {
		joined += "?" + pathURL.RawQuery
	}
	var requestBody io.Reader
	if body != nil {
		payload, marshalErr := common.Marshal(body)
		if marshalErr != nil {
			return fmt.Errorf("encode Sub2API request")
		}
		requestBody = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, joined, requestBody)
	if err != nil {
		return fmt.Errorf("create Sub2API request")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.7")
	request.Header.Set("Origin", origin)
	request.Header.Set("Referer", origin+"/")
	request.Header.Set("User-Agent", "XNewAPI-ProviderSync/1.0")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	client := a.client
	if client == nil {
		client = newSub2APIHTTPClient()
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Sub2API request failed")
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read Sub2API response")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Sub2API request returned status %d", response.StatusCode)
	}
	var envelope sub2APIEnvelope[any]
	if err := common.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("decode Sub2API response")
	}
	if envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = "request rejected"
		}
		return fmt.Errorf("Sub2API request rejected: %s", message)
	}
	if output == nil {
		return nil
	}
	data, err := common.Marshal(envelope.Data)
	if err != nil || common.Unmarshal(data, output) != nil {
		return fmt.Errorf("decode Sub2API response data")
	}
	return nil
}
