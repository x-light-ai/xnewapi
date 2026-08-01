// FORK-CUSTOM: Coordinate provider account synchronization without coupling it to relay execution.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

type ProviderAccountSnapshot struct {
	AccountExternalID      string
	Balance                decimal.Decimal
	TotalRecharge          decimal.Decimal
	BalanceAvailable       bool
	TotalRechargeAvailable bool
	ObservedAt             int64
	Keys                   []ProviderKeySnapshot
	Groups                 []ProviderGroupSnapshot
	RawSource              string
	MissingFields          []string
}

type ProviderKeySnapshot struct {
	ExternalID  string
	Name        string
	GroupName   string
	MaskedKey   string
	Fingerprint string
	Status      string
	LastUsageAt int64
}

type ProviderGroupSnapshot struct {
	GroupName          string
	CostDate           string
	ProviderUsageQuota decimal.Decimal
	ProviderCost       decimal.Decimal
	CostObservedAt     int64
	CostStatus         string
}

type ProviderSyncCredentials struct {
	Username   string
	SyncAPIKey string
}

type ProviderSyncAdapter interface {
	ValidateAccount(ctx context.Context, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount, credentials ProviderSyncCredentials) error
	SyncAccountSnapshot(ctx context.Context, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount, credentials ProviderSyncCredentials) (*ProviderAccountSnapshot, error)
}

var (
	providerSyncAdaptersMu sync.RWMutex
	providerSyncAdapters   = map[string]ProviderSyncAdapter{}
	providerSyncStartOnce  sync.Once
)

func RegisterProviderSyncAdapter(adapterType string, adapter ProviderSyncAdapter) error {
	adapterType = strings.ToLower(strings.TrimSpace(adapterType))
	if adapterType == "" || adapter == nil {
		return errors.New("provider sync adapter registration is invalid")
	}
	providerSyncAdaptersMu.Lock()
	defer providerSyncAdaptersMu.Unlock()
	if _, exists := providerSyncAdapters[adapterType]; exists {
		return fmt.Errorf("provider sync adapter already registered for %s", adapterType)
	}
	providerSyncAdapters[adapterType] = adapter
	return nil
}

func SyncUpstreamProvider(ctx context.Context, providerId int, accountId int, autoOnly bool) ([]model.UpstreamProviderSyncRun, error) {
	provider, err := model.GetUpstreamProviderForSync(providerId)
	if err != nil {
		return nil, err
	}
	if provider.Status != model.UpstreamProviderStatusEnabled {
		return nil, errors.New("provider is disabled")
	}
	adapterType := strings.ToLower(strings.TrimSpace(provider.AdapterType))
	if adapterType == "" {
		adapterType = "newapi"
		provider.AdapterType = adapterType
	}
	adapter := getProviderSyncAdapter(adapterType)
	if adapter == nil {
		return nil, fmt.Errorf("no sync adapter registered for %s", provider.AuthenticationMethod)
	}

	runs := make([]model.UpstreamProviderSyncRun, 0, len(provider.Accounts))
	var firstSyncError error
	for index := range provider.Accounts {
		account := &provider.Accounts[index]
		if account.Status != model.UpstreamProviderStatusEnabled || (autoOnly && !account.AutoSync) || (accountId > 0 && account.Id != accountId) {
			continue
		}
		run, err := syncUpstreamProviderAccount(ctx, adapter, provider, account)
		if run != nil {
			runs = append(runs, *run)
		}
		if err != nil && firstSyncError == nil {
			firstSyncError = err
		}
	}
	if accountId > 0 && len(runs) == 0 {
		return nil, errors.New("provider account is unavailable for synchronization")
	}
	if len(runs) > 0 {
		provider.LastSyncedAt = common.GetTimestamp()
		provider.LastSyncError = ""
		provider.SyncStatus = "success"
		for _, run := range runs {
			if run.Status == "error" {
				provider.SyncStatus = "error"
				provider.LastSyncError = run.Error
				break
			}
			if run.Status == "partial_success" {
				provider.SyncStatus = "partial_success"
			}
		}
		if err := model.UpdateUpstreamProviderSyncState(provider.Id, provider.SyncStatus, provider.LastSyncedAt, provider.LastSyncError); err != nil {
			return runs, err
		}
	}
	if firstSyncError != nil {
		return runs, firstSyncError
	}
	return runs, nil
}

func getProviderSyncAdapter(adapterType string) ProviderSyncAdapter {
	providerSyncAdaptersMu.RLock()
	defer providerSyncAdaptersMu.RUnlock()
	return providerSyncAdapters[strings.ToLower(strings.TrimSpace(adapterType))]
}

func syncUpstreamProviderAccount(ctx context.Context, adapter ProviderSyncAdapter, provider *model.UpstreamProvider, account *model.UpstreamProviderAccount) (*model.UpstreamProviderSyncRun, error) {
	now := common.GetTimestamp()
	adapterType := strings.ToLower(strings.TrimSpace(provider.AdapterType))
	if adapterType == "" {
		adapterType = "newapi"
	}
	run := &model.UpstreamProviderSyncRun{ProviderId: provider.Id, AccountId: account.Id, Status: "running", StartedAt: now, Adapter: adapterType}
	if err := model.ClaimUpstreamProviderSyncRun(run); err != nil {
		return nil, err
	}

	apiKey := account.SyncAPIKey
	var err error
	if apiKey == "" {
		err = errors.New("account sync API key is not configured")
	}
	credentials := ProviderSyncCredentials{Username: account.LoginUsername, SyncAPIKey: apiKey}
	var snapshot *ProviderAccountSnapshot
	if err == nil {
		err = adapter.ValidateAccount(ctx, provider, account, credentials)
	}
	if err == nil {
		snapshot, err = adapter.SyncAccountSnapshot(ctx, provider, account, credentials)
		if err == nil && snapshot == nil {
			err = errors.New("provider sync adapter returned an empty snapshot")
		}
		if err == nil && snapshot != nil {
			if strings.TrimSpace(snapshot.AccountExternalID) != "" {
				account.ExternalId = strings.TrimSpace(snapshot.AccountExternalID)
				account.ExternalIdObserved = true
			}
			if snapshot.BalanceAvailable {
				account.Balance = snapshot.Balance
			}
			if snapshot.TotalRechargeAvailable {
				account.TotalRecharge = snapshot.TotalRecharge
			}
			account.BalanceUpdatedAt = snapshot.ObservedAt
		}
	}
	finishedAt := common.GetTimestamp()
	run.FinishedAt = finishedAt
	if err != nil {
		publicError := sanitizeProviderSyncError(err, apiKey)
		run.Status = "error"
		run.Error = publicError
		account.SyncStatus = "error"
		account.LastSyncError = publicError
		account.LastSyncAttemptAt = finishedAt
		account.NextSyncAt = finishedAt + syncBackoffSeconds(account)
	} else if snapshot != nil && len(snapshot.MissingFields) > 0 {
		run.Status = "partial_success"
		run.Diagnostics = "missing fields: " + strings.Join(snapshot.MissingFields, ",")
		account.SyncStatus = "partial_success"
		account.LastSyncedAt = finishedAt
		account.LastSyncAttemptAt = finishedAt
		account.NextSyncAt = finishedAt + int64(provider.SyncIntervalMinutes)*60
		account.LastSyncError = ""
	} else {
		run.Status = "success"
		account.SyncStatus = "success"
		account.LastSyncedAt = finishedAt
		account.LastSyncAttemptAt = finishedAt
		account.NextSyncAt = finishedAt + int64(provider.SyncIntervalMinutes)*60
		account.LastSyncError = ""
	}
	if saveErr := model.FinishUpstreamProviderSync(run, account, providerKeySnapshotInputs(snapshot), providerGroupSnapshotInputs(snapshot)); saveErr != nil {
		return nil, saveErr
	}
	refreshedDates := map[string]struct{}{}
	if snapshot != nil {
		for _, group := range snapshot.Groups {
			if group.CostDate == "" {
				continue
			}
			if _, refreshed := refreshedDates[group.CostDate]; refreshed {
				continue
			}
			day, parseErr := time.ParseInLocation("2006-01-02", group.CostDate, time.Local)
			if parseErr != nil {
				common.SysError("parse provider daily profit date: " + parseErr.Error())
				continue
			}
			if refreshErr := refreshProviderProfitDailyDate(day); refreshErr != nil {
				common.SysError("refresh provider daily profit: " + refreshErr.Error())
			}
			refreshedDates[group.CostDate] = struct{}{}
		}
	}
	return run, err
}

func sanitizeProviderSyncError(err error, secrets ...string) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[redacted]")
		}
	}
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}

func providerKeySnapshotInputs(snapshot *ProviderAccountSnapshot) []model.UpstreamProviderKeySnapshotInput {
	if snapshot == nil || len(snapshot.Keys) == 0 {
		return nil
	}
	inputs := make([]model.UpstreamProviderKeySnapshotInput, 0, len(snapshot.Keys))
	for _, item := range snapshot.Keys {
		inputs = append(inputs, model.UpstreamProviderKeySnapshotInput{
			ExternalID: item.ExternalID, Name: item.Name, GroupName: item.GroupName, MaskedKey: item.MaskedKey,
			Fingerprint: item.Fingerprint, Status: item.Status, LastUsageAt: item.LastUsageAt,
		})
	}
	return inputs
}

func providerGroupSnapshotInputs(snapshot *ProviderAccountSnapshot) []model.UpstreamProviderGroupSnapshotInput {
	if snapshot == nil || len(snapshot.Groups) == 0 {
		return nil
	}
	inputs := make([]model.UpstreamProviderGroupSnapshotInput, 0, len(snapshot.Groups))
	for _, item := range snapshot.Groups {
		inputs = append(inputs, model.UpstreamProviderGroupSnapshotInput{
			GroupName: item.GroupName, CostDate: item.CostDate,
			ProviderUsageQuota: item.ProviderUsageQuota, ProviderCost: item.ProviderCost,
			CostObservedAt: item.CostObservedAt, CostStatus: item.CostStatus,
		})
	}
	return inputs
}

func StartUpstreamProviderSyncScheduler() {
	providerSyncStartOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				providers, err := model.GetDueUpstreamProviderSyncs(common.GetTimestamp())
				if err != nil {
					common.SysError("load due upstream provider syncs: " + err.Error())
					continue
				}
				for _, provider := range providers {
					adapterType := strings.ToLower(strings.TrimSpace(provider.AdapterType))
					if adapterType == "" {
						adapterType = "newapi"
					}
					if getProviderSyncAdapter(adapterType) == nil || len(provider.Accounts) == 0 {
						continue
					}
					for _, account := range provider.Accounts {
						if _, err := SyncUpstreamProvider(context.Background(), provider.Id, account.Id, true); err != nil && !errors.Is(err, model.ErrUpstreamProviderSyncRunning) {
							common.SysError(fmt.Sprintf("sync upstream provider %d account %d: %v", provider.Id, account.Id, err))
						}
					}
				}
			}
		}()
	})
}

func isSupportedProviderAdapter(adapterType string) bool {
	return adapterType == "newapi" || adapterType == "sub2api"
}

func syncBackoffSeconds(account *model.UpstreamProviderAccount) int64 {
	if account == nil || account.LastSyncAttemptAt == 0 {
		return 60
	}
	if account.NextSyncAt > account.LastSyncAttemptAt {
		backoff := account.NextSyncAt - account.LastSyncAttemptAt
		if backoff < 3600 {
			return backoff * 2
		}
	}
	return 60
}
