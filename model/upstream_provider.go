// FORK-CUSTOM: Persist upstream provider management independently from relay channel execution.
package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	UpstreamProviderStatusEnabled     = "enabled"
	UpstreamProviderStatusDisabled    = "disabled"
	UpstreamProviderKeyStatusActive   = "active"
	UpstreamProviderKeyStatusDisabled = "disabled"
)

var ErrUpstreamProviderChannelMapped = errors.New("channel is already mapped to another provider group")
var ErrUpstreamProviderSyncRunning = errors.New("provider account synchronization is already running")
var ErrUpstreamProviderRechargeAmountTooLarge = errors.New("total recharge exceeds the maximum supported amount")

var maxUpstreamProviderRechargeAmount = decimal.RequireFromString("999999999999.99999999")

type UpstreamProvider struct {
	Id                   int                       `json:"id" gorm:"primaryKey"`
	Name                 string                    `json:"name" gorm:"type:varchar(128);not null;uniqueIndex:uk_xnewapi_upstream_provider_name"`
	Endpoint             string                    `json:"endpoint" gorm:"type:varchar(512);not null;default:''"`
	Status               string                    `json:"status" gorm:"type:varchar(16);not null;index"`
	AuthenticationMethod string                    `json:"authentication_method" gorm:"type:varchar(32);not null"`
	AdapterType          string                    `json:"adapter_type" gorm:"type:varchar(64);not null;default:'newapi';index"`
	SyncIntervalMinutes  int                       `json:"sync_interval_minutes" gorm:"not null"`
	RechargeRatio        decimal.Decimal           `json:"recharge_ratio" gorm:"type:decimal(20,8);not null"`
	QuotaConversionBase  decimal.Decimal           `json:"quota_conversion_base" gorm:"type:decimal(20,8);not null"`
	Currency             string                    `json:"currency" gorm:"type:varchar(8);not null"`
	SyncStatus           string                    `json:"sync_status" gorm:"type:varchar(16);not null"`
	LastSyncedAt         int64                     `json:"last_synced_at" gorm:"bigint;not null"`
	LastSyncError        string                    `json:"last_sync_error" gorm:"type:text"`
	CreatedAt            int64                     `json:"created_at" gorm:"bigint"`
	UpdatedAt            int64                     `json:"updated_at" gorm:"bigint"`
	Accounts             []UpstreamProviderAccount `json:"accounts,omitempty" gorm:"foreignKey:ProviderId"`
}

func (UpstreamProvider) TableName() string {
	return "xnewapi_upstream_providers"
}

func (provider *UpstreamProvider) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	provider.CreatedAt = now
	provider.UpdatedAt = now
	return nil
}

func (provider *UpstreamProvider) BeforeUpdate(_ *gorm.DB) error {
	provider.UpdatedAt = common.GetTimestamp()
	return nil
}

type UpstreamProviderAccount struct {
	Id                 int                     `json:"id" gorm:"primaryKey"`
	ProviderId         int                     `json:"provider_id" gorm:"not null;index"`
	Name               string                  `json:"name" gorm:"type:varchar(128);not null"`
	ExternalId         string                  `json:"external_id" gorm:"type:varchar(128);not null;default:''"`
	ExternalIdObserved bool                    `json:"-" gorm:"-"`
	LoginUsername      string                  `json:"-" gorm:"type:varchar(256);not null;default:''"`
	Status             string                  `json:"status" gorm:"type:varchar(16);not null;index"`
	AutoSync           bool                    `json:"auto_sync" gorm:"not null"`
	SyncAPIKey         string                  `json:"-" gorm:"type:text"`
	HasSyncAPIKey      bool                    `json:"has_sync_api_key" gorm:"-"`
	Balance            decimal.Decimal         `json:"balance" gorm:"type:decimal(20,8);not null"`
	TotalRecharge      decimal.Decimal         `json:"total_recharge" gorm:"type:decimal(20,8);not null"`
	BalanceUpdatedAt   int64                   `json:"balance_updated_at" gorm:"bigint;not null"`
	SyncStatus         string                  `json:"sync_status" gorm:"type:varchar(16);not null"`
	LastSyncedAt       int64                   `json:"last_synced_at" gorm:"bigint;not null"`
	LastSyncAttemptAt  int64                   `json:"last_sync_attempt_at" gorm:"bigint;not null;default:0"`
	NextSyncAt         int64                   `json:"next_sync_at" gorm:"bigint;not null;default:0"`
	LastSyncError      string                  `json:"last_sync_error" gorm:"type:text"`
	CreatedAt          int64                   `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64                   `json:"updated_at" gorm:"bigint"`
	Groups             []UpstreamProviderGroup `json:"groups,omitempty" gorm:"foreignKey:AccountId"`
}

func (UpstreamProviderAccount) TableName() string {
	return "xnewapi_upstream_provider_accounts"
}

func (account *UpstreamProviderAccount) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	account.CreatedAt = now
	account.UpdatedAt = now
	return nil
}

func (account *UpstreamProviderAccount) BeforeUpdate(_ *gorm.DB) error {
	account.UpdatedAt = common.GetTimestamp()
	return nil
}

type UpstreamProviderKey struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	AccountId       int    `json:"account_id" gorm:"not null;uniqueIndex:uk_xnewapi_provider_key_external,priority:1;index"`
	ProviderGroupId int    `json:"provider_group_id" gorm:"not null;index"`
	Name            string `json:"name" gorm:"type:varchar(128);not null"`
	ExternalId      string `json:"external_id" gorm:"type:varchar(128);not null;uniqueIndex:uk_xnewapi_provider_key_external,priority:2"`
	KeyMasked       string `json:"key_masked" gorm:"type:varchar(256);not null;default:''"`
	KeyFingerprint  string `json:"key_fingerprint" gorm:"type:varchar(64);not null;default:''"`
	Status          string `json:"status" gorm:"type:varchar(16);not null;index"`
	LastUsageAt     int64  `json:"last_usage_at" gorm:"bigint;not null"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
}

func (UpstreamProviderKey) TableName() string {
	return "xnewapi_upstream_provider_keys"
}

func (key *UpstreamProviderKey) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	key.CreatedAt = now
	key.UpdatedAt = now
	return nil
}

func (key *UpstreamProviderKey) BeforeUpdate(_ *gorm.DB) error {
	key.UpdatedAt = common.GetTimestamp()
	return nil
}

type UpstreamProviderGroup struct {
	Id              int                            `json:"id" gorm:"primaryKey"`
	AccountId       int                            `json:"account_id" gorm:"not null;uniqueIndex:uk_xnewapi_provider_group,priority:1;index"`
	Name            string                         `json:"name" gorm:"type:varchar(128);not null;uniqueIndex:uk_xnewapi_provider_group,priority:2"`
	CreatedAt       int64                          `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64                          `json:"updated_at" gorm:"bigint"`
	ChannelMappings []UpstreamProviderGroupChannel `json:"channel_mappings,omitempty" gorm:"foreignKey:ProviderGroupId"`
	Keys            []UpstreamProviderKey          `json:"keys,omitempty" gorm:"foreignKey:ProviderGroupId"`
}

func (UpstreamProviderGroup) TableName() string {
	return "xnewapi_upstream_provider_groups"
}

func (group *UpstreamProviderGroup) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	group.CreatedAt = now
	group.UpdatedAt = now
	return nil
}

func (group *UpstreamProviderGroup) BeforeUpdate(_ *gorm.DB) error {
	group.UpdatedAt = common.GetTimestamp()
	return nil
}

type UpstreamProviderGroupChannel struct {
	Id              int   `json:"id" gorm:"primaryKey"`
	ProviderGroupId int   `json:"provider_group_id" gorm:"not null;uniqueIndex:uk_xnewapi_provider_group_channel,priority:1;index"`
	ChannelId       int   `json:"channel_id" gorm:"not null;uniqueIndex:uk_xnewapi_provider_group_channel,priority:2;uniqueIndex:uk_xnewapi_provider_channel"`
	CreatedAt       int64 `json:"created_at" gorm:"bigint"`
}

func (UpstreamProviderGroupChannel) TableName() string {
	return "xnewapi_upstream_provider_group_channels"
}

func (mapping *UpstreamProviderGroupChannel) BeforeCreate(_ *gorm.DB) error {
	mapping.CreatedAt = common.GetTimestamp()
	return nil
}

type UpstreamProviderKeySnapshotInput struct {
	ExternalID  string
	Name        string
	GroupName   string
	MaskedKey   string
	Fingerprint string
	Status      string
	LastUsageAt int64
}

type UpstreamProviderGroupSnapshotInput struct {
	GroupName          string
	CostDate           string
	ProviderUsageQuota decimal.Decimal
	ProviderCost       decimal.Decimal
	CostObservedAt     int64
	CostStatus         string
}

type UpstreamProviderWorkspace struct {
	Provider UpstreamProvider
	Accounts []UpstreamProviderWorkspaceAccount
}

type UpstreamProviderWorkspaceAccount struct {
	Account       UpstreamProviderAccount
	LoginUsername *string
	SyncAPIKey    *string
	Groups        []UpstreamProviderWorkspaceGroup
}

type UpstreamProviderWorkspaceGroup struct {
	Group      UpstreamProviderGroup
	ChannelIds []int
}

type UpstreamProviderSyncRun struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	ProviderId  int    `json:"provider_id" gorm:"not null;index"`
	AccountId   int    `json:"account_id" gorm:"not null;index"`
	Status      string `json:"status" gorm:"type:varchar(16);not null;index"`
	StartedAt   int64  `json:"started_at" gorm:"bigint;not null"`
	FinishedAt  int64  `json:"finished_at" gorm:"bigint;not null"`
	Error       string `json:"error" gorm:"type:text"`
	Adapter     string `json:"adapter" gorm:"type:varchar(64);not null;default:''"`
	Diagnostics string `json:"diagnostics" gorm:"type:text"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func (UpstreamProviderSyncRun) TableName() string {
	return "xnewapi_upstream_provider_sync_runs"
}

func (run *UpstreamProviderSyncRun) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	run.CreatedAt = now
	run.UpdatedAt = now
	return nil
}

func (run *UpstreamProviderSyncRun) BeforeUpdate(_ *gorm.DB) error {
	run.UpdatedAt = common.GetTimestamp()
	return nil
}

type UpstreamProviderGroupProfitDaily struct {
	Id                 int             `json:"id" gorm:"primaryKey"`
	Date               string          `json:"date" gorm:"type:char(10);not null;uniqueIndex:uk_xnewapi_group_profit_daily,priority:1"`
	ProviderGroupId    int             `json:"provider_group_id" gorm:"not null;uniqueIndex:uk_xnewapi_group_profit_daily,priority:2;index"`
	RevenueQuota       int64           `json:"revenue_quota" gorm:"bigint;not null"`
	RevenueAmount      decimal.Decimal `json:"revenue_amount" gorm:"type:decimal(20,8);not null"`
	ProviderUsageQuota decimal.Decimal `json:"provider_usage_quota" gorm:"type:decimal(20,8);not null"`
	ProviderCost       decimal.Decimal `json:"provider_cost" gorm:"type:decimal(20,8);not null"`
	CostStatus         string          `json:"cost_status" gorm:"type:varchar(16);not null;index"`
	CostObservedAt     int64           `json:"cost_observed_at" gorm:"bigint;not null"`
	CreatedAt          int64           `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64           `json:"updated_at" gorm:"bigint"`
}

type UpstreamProviderRevenueRow struct {
	ChannelID int
	Quota     int64
}

type UpstreamProviderChannelOption struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
}

func GetUpstreamProviderChannelOptions() ([]UpstreamProviderChannelOption, error) {
	var channels []UpstreamProviderChannelOption
	err := DB.Model(&Channel{}).Select("id", "name", "status").Order("id asc").Find(&channels).Error
	return channels, err
}

func GetUpstreamProviderRevenueRows(channelIDs []int, startAt int64, endAt int64) ([]UpstreamProviderRevenueRow, error) {
	if len(channelIDs) == 0 {
		return []UpstreamProviderRevenueRow{}, nil
	}
	var rows []UpstreamProviderRevenueRow
	err := LOG_DB.Model(&Log{}).
		Select("channel_id, quota").
		Where("type = ? AND channel_id IN ?", LogTypeConsume, channelIDs).
		Where("created_at >= ? AND created_at <= ?", startAt, endAt).
		Find(&rows).Error
	return rows, err
}

func (UpstreamProviderGroupProfitDaily) TableName() string {
	return "xnewapi_upstream_provider_group_profit_daily"
}

func (profit *UpstreamProviderGroupProfitDaily) BeforeCreate(_ *gorm.DB) error {
	now := common.GetTimestamp()
	profit.CreatedAt = now
	profit.UpdatedAt = now
	return nil
}

func (profit *UpstreamProviderGroupProfitDaily) BeforeUpdate(_ *gorm.DB) error {
	profit.UpdatedAt = common.GetTimestamp()
	return nil
}

func GetUpstreamProviderPage(offset int, limit int, keyword string) ([]UpstreamProvider, int64, error) {
	query := DB.Model(&UpstreamProvider{})
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		query = query.Where("name LIKE ? OR endpoint LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var providers []UpstreamProvider
	if err := preloadUpstreamProviderWorkspace(query).Order("id desc").Offset(offset).Limit(limit).Find(&providers).Error; err != nil {
		return nil, 0, err
	}
	for index := range providers {
		markUpstreamProviderSyncAPIKeyPresence(&providers[index])
	}
	return providers, total, nil
}

func preloadUpstreamProviderWorkspace(tx *gorm.DB) *gorm.DB {
	return tx.Preload("Accounts", func(query *gorm.DB) *gorm.DB {
		return query.Order("id asc")
	}).Preload("Accounts.Groups", func(query *gorm.DB) *gorm.DB {
		return query.Order("id asc")
	}).Preload("Accounts.Groups.ChannelMappings", func(query *gorm.DB) *gorm.DB {
		return query.Order("channel_id asc")
	}).Preload("Accounts.Groups.Keys", func(query *gorm.DB) *gorm.DB {
		return query.Order("id asc")
	})
}

func UpdateUpstreamProviderSyncState(id int, syncStatus string, lastSyncedAt int64, lastSyncError string) error {
	return DB.Model(&UpstreamProvider{}).Where("id = ?", id).Updates(map[string]any{
		"sync_status": syncStatus, "last_synced_at": lastSyncedAt, "last_sync_error": lastSyncError,
	}).Error
}

func SaveUpstreamProviderWorkspace(workspace *UpstreamProviderWorkspace) (*UpstreamProvider, error) {
	if workspace == nil {
		return nil, errors.New("provider workspace is required")
	}
	var saved UpstreamProvider
	err := DB.Transaction(func(tx *gorm.DB) error {
		provider := workspace.Provider
		if provider.Id == 0 {
			if err := tx.Omit("Accounts").Create(&provider).Error; err != nil {
				return err
			}
		} else {
			var existing UpstreamProvider
			if err := lockForUpdate(tx).First(&existing, "id = ?", provider.Id).Error; err != nil {
				return err
			}
			existing.Name = provider.Name
			existing.Endpoint = provider.Endpoint
			existing.Status = provider.Status
			existing.AuthenticationMethod = provider.AuthenticationMethod
			existing.AdapterType = provider.AdapterType
			existing.SyncIntervalMinutes = provider.SyncIntervalMinutes
			existing.RechargeRatio = provider.RechargeRatio
			existing.QuotaConversionBase = provider.QuotaConversionBase
			existing.Currency = provider.Currency
			provider = existing
			if err := tx.Model(&existing).Updates(map[string]any{
				"name": provider.Name, "endpoint": provider.Endpoint, "status": provider.Status,
				"authentication_method": provider.AuthenticationMethod, "adapter_type": provider.AdapterType,
				"sync_interval_minutes": provider.SyncIntervalMinutes,
				"recharge_ratio":        provider.RechargeRatio, "quota_conversion_base": provider.QuotaConversionBase,
				"currency": provider.Currency,
			}).Error; err != nil {
				return err
			}
		}
		for accountIndex := range workspace.Accounts {
			input := &workspace.Accounts[accountIndex]
			account := input.Account
			if account.Id == 0 {
				account.ProviderId = provider.Id
				if input.LoginUsername != nil {
					account.LoginUsername = strings.TrimSpace(*input.LoginUsername)
				}
				if input.SyncAPIKey != nil {
					account.SyncAPIKey = strings.TrimSpace(*input.SyncAPIKey)
				}
				if err := tx.Omit("Groups").Create(&account).Error; err != nil {
					return err
				}
			} else {
				var existing UpstreamProviderAccount
				if err := lockForUpdate(tx).First(&existing, "id = ? AND provider_id = ?", account.Id, provider.Id).Error; err != nil {
					return err
				}
				existing.Name = account.Name
				existing.ExternalId = account.ExternalId
				if input.LoginUsername != nil {
					existing.LoginUsername = strings.TrimSpace(*input.LoginUsername)
				}
				existing.Status = account.Status
				existing.AutoSync = account.AutoSync
				if input.SyncAPIKey != nil {
					existing.SyncAPIKey = strings.TrimSpace(*input.SyncAPIKey)
				}
				account = existing
				updates := map[string]any{
					"name": account.Name, "external_id": account.ExternalId, "status": account.Status,
					"auto_sync": account.AutoSync,
				}
				if input.LoginUsername != nil {
					updates["login_username"] = account.LoginUsername
				}
				if input.SyncAPIKey != nil {
					updates["sync_api_key"] = account.SyncAPIKey
				}
				if err := tx.Model(&existing).Updates(updates).Error; err != nil {
					return err
				}
			}

			for groupIndex := range input.Groups {
				groupInput := &input.Groups[groupIndex]
				group := groupInput.Group
				group.AccountId = account.Id
				if group.Id == 0 {
					if err := tx.Omit("ChannelMappings", "Keys").Create(&group).Error; err != nil {
						return err
					}
				} else {
					var existing UpstreamProviderGroup
					if err := lockForUpdate(tx).First(&existing, "id = ? AND account_id = ?", group.Id, account.Id).Error; err != nil {
						return err
					}
					existing.Name = group.Name
					group = existing
					if err := tx.Omit("ChannelMappings", "Keys").Save(&group).Error; err != nil {
						return err
					}
				}
				if err := replaceUpstreamProviderGroupChannels(tx, group.Id, groupInput.ChannelIds); err != nil {
					return err
				}
			}
		}
		if err := preloadUpstreamProviderWorkspace(tx).First(&saved, "id = ?", provider.Id).Error; err != nil {
			return err
		}
		markUpstreamProviderSyncAPIKeyPresence(&saved)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func ClaimUpstreamProviderSyncRun(run *UpstreamProviderSyncRun) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var account UpstreamProviderAccount
		if err := lockForUpdate(tx).First(&account, "id = ?", run.AccountId).Error; err != nil {
			return err
		}
		staleBefore := run.StartedAt - 10*60
		if err := tx.Model(&UpstreamProviderSyncRun{}).Where("account_id = ? AND status = ? AND started_at <= ?", run.AccountId, "running", staleBefore).
			Updates(map[string]interface{}{"status": "error", "finished_at": run.StartedAt, "error": "synchronization timed out"}).Error; err != nil {
			return err
		}
		var running UpstreamProviderSyncRun
		if err := tx.Where("account_id = ? AND status = ?", run.AccountId, "running").First(&running).Error; err == nil {
			return ErrUpstreamProviderSyncRunning
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(run).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrUpstreamProviderSyncRunning
			}
			return err
		}
		return tx.Model(&account).Updates(map[string]interface{}{
			"sync_status": "syncing", "last_sync_attempt_at": run.StartedAt, "last_sync_error": "",
		}).Error
	})
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate entry") || strings.Contains(message, "duplicate key")
}

func FinishUpstreamProviderSync(run *UpstreamProviderSyncRun, account *UpstreamProviderAccount, keyInputs []UpstreamProviderKeySnapshotInput, groupInputs []UpstreamProviderGroupSnapshotInput) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var storedAccount UpstreamProviderAccount
		if err := lockForUpdate(tx).First(&storedAccount, "id = ?", account.Id).Error; err != nil {
			return err
		}
		if err := tx.Save(run).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"balance": account.Balance, "total_recharge": account.TotalRecharge,
			"balance_updated_at": account.BalanceUpdatedAt, "sync_status": account.SyncStatus,
			"last_synced_at": account.LastSyncedAt, "last_sync_attempt_at": account.LastSyncAttemptAt,
			"next_sync_at": account.NextSyncAt, "last_sync_error": account.LastSyncError,
		}
		if account.ExternalIdObserved {
			updates["external_id"] = account.ExternalId
		}
		if err := tx.Model(&storedAccount).Updates(updates).Error; err != nil {
			return err
		}
		groupsByName, err := persistUpstreamProviderGroupsAndKeys(tx, account.Id, keyInputs)
		if err != nil {
			return err
		}
		return persistUpstreamProviderGroupDailyCosts(tx, groupsByName, groupInputs)
	})
}

func DeleteUpstreamProvider(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var provider UpstreamProvider
		if err := lockForUpdate(tx).First(&provider, "id = ?", id).Error; err != nil {
			return err
		}
		var accounts []UpstreamProviderAccount
		if err := tx.Select("id").Where("provider_id = ?", id).Find(&accounts).Error; err != nil {
			return err
		}
		accountIDs := make([]int, 0, len(accounts))
		for _, account := range accounts {
			accountIDs = append(accountIDs, account.Id)
		}
		if err := deleteUpstreamProviderAccounts(tx, accountIDs); err != nil {
			return err
		}
		return tx.Delete(&provider).Error
	})
}

func DeleteUpstreamProviderAccount(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var account UpstreamProviderAccount
		if err := lockForUpdate(tx).First(&account, "id = ?", id).Error; err != nil {
			return err
		}
		return deleteUpstreamProviderAccounts(tx, []int{account.Id})
	})
}

func DeleteUpstreamProviderGroup(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var group UpstreamProviderGroup
		if err := lockForUpdate(tx).First(&group, "id = ?", id).Error; err != nil {
			return err
		}
		var account UpstreamProviderAccount
		if err := lockForUpdate(tx).First(&account, "id = ?", group.AccountId).Error; err != nil {
			return err
		}
		if err := ensureUpstreamProviderAccountsNotSyncing(tx, []int{group.AccountId}); err != nil {
			return err
		}
		return deleteUpstreamProviderGroups(tx, []int{group.Id})
	})
}

func deleteUpstreamProviderAccounts(tx *gorm.DB, accountIDs []int) error {
	if len(accountIDs) == 0 {
		return nil
	}
	var accounts []UpstreamProviderAccount
	if err := lockForUpdate(tx).Where("id IN ?", accountIDs).Find(&accounts).Error; err != nil {
		return err
	}
	if len(accounts) != len(accountIDs) {
		return gorm.ErrRecordNotFound
	}
	lockedAccountIDs := make([]int, 0, len(accounts))
	for _, account := range accounts {
		lockedAccountIDs = append(lockedAccountIDs, account.Id)
	}
	if err := ensureUpstreamProviderAccountsNotSyncing(tx, lockedAccountIDs); err != nil {
		return err
	}
	var groups []UpstreamProviderGroup
	if err := tx.Select("id").Where("account_id IN ?", lockedAccountIDs).Find(&groups).Error; err != nil {
		return err
	}
	groupIDs := make([]int, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.Id)
	}
	if err := deleteUpstreamProviderGroups(tx, groupIDs); err != nil {
		return err
	}
	if err := tx.Where("account_id IN ?", lockedAccountIDs).Delete(&UpstreamProviderSyncRun{}).Error; err != nil {
		return err
	}
	return tx.Where("id IN ?", lockedAccountIDs).Delete(&UpstreamProviderAccount{}).Error
}

func ensureUpstreamProviderAccountsNotSyncing(tx *gorm.DB, accountIDs []int) error {
	if len(accountIDs) == 0 {
		return nil
	}
	var running int64
	if err := tx.Model(&UpstreamProviderSyncRun{}).Where("account_id IN ? AND status = ?", accountIDs, "running").Count(&running).Error; err != nil {
		return err
	}
	if running > 0 {
		return ErrUpstreamProviderSyncRunning
	}
	return nil
}

func deleteUpstreamProviderGroups(tx *gorm.DB, groupIDs []int) error {
	if len(groupIDs) == 0 {
		return nil
	}
	if err := tx.Where("provider_group_id IN ?", groupIDs).Delete(&UpstreamProviderGroupChannel{}).Error; err != nil {
		return err
	}
	if err := tx.Where("provider_group_id IN ?", groupIDs).Delete(&UpstreamProviderKey{}).Error; err != nil {
		return err
	}
	if err := tx.Where("provider_group_id IN ?", groupIDs).Delete(&UpstreamProviderGroupProfitDaily{}).Error; err != nil {
		return err
	}
	return tx.Where("id IN ?", groupIDs).Delete(&UpstreamProviderGroup{}).Error
}

func persistUpstreamProviderGroupsAndKeys(tx *gorm.DB, accountID int, inputs []UpstreamProviderKeySnapshotInput) (map[string]UpstreamProviderGroup, error) {
	groupsByName := make(map[string]UpstreamProviderGroup)
	movedGroups := make(map[int]map[int]struct{})
	for _, input := range inputs {
		groupName := strings.TrimSpace(input.GroupName)
		externalID := strings.TrimSpace(input.ExternalID)
		if externalID == "" || groupName == "" {
			continue
		}
		group, ok := groupsByName[groupName]
		if !ok {
			err := tx.Where("account_id = ? AND name = ?", accountID, groupName).First(&group).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				group = UpstreamProviderGroup{AccountId: accountID, Name: groupName}
				if err := tx.Create(&group).Error; err != nil {
					return nil, err
				}
			} else if err != nil {
				return nil, err
			}
			groupsByName[groupName] = group
		}
		var key UpstreamProviderKey
		err := tx.Where("account_id = ? AND external_id = ?", accountID, externalID).First(&key).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			key = UpstreamProviderKey{AccountId: accountID, ProviderGroupId: group.Id, ExternalId: externalID}
		} else if err != nil {
			return nil, err
		}
		if key.Id != 0 && key.ProviderGroupId != group.Id {
			destinations := movedGroups[key.ProviderGroupId]
			if destinations == nil {
				destinations = make(map[int]struct{})
				movedGroups[key.ProviderGroupId] = destinations
			}
			destinations[group.Id] = struct{}{}
		}
		key.ProviderGroupId = group.Id
		key.Name = strings.TrimSpace(input.Name)
		key.KeyMasked = strings.TrimSpace(input.MaskedKey)
		key.KeyFingerprint = strings.TrimSpace(input.Fingerprint)
		key.Status = strings.TrimSpace(input.Status)
		if key.Status == "" {
			key.Status = UpstreamProviderKeyStatusActive
		}
		key.LastUsageAt = input.LastUsageAt
		if key.Id == 0 {
			if err := tx.Create(&key).Error; err != nil {
				return nil, err
			}
		} else if err := tx.Save(&key).Error; err != nil {
			return nil, err
		}
	}
	for sourceGroupID, destinations := range movedGroups {
		if len(destinations) != 1 {
			continue
		}
		var remainingKeys int64
		if err := tx.Model(&UpstreamProviderKey{}).Where("provider_group_id = ?", sourceGroupID).Count(&remainingKeys).Error; err != nil {
			return nil, err
		}
		if remainingKeys != 0 {
			continue
		}
		var destinationGroupID int
		for id := range destinations {
			destinationGroupID = id
		}
		if err := tx.Model(&UpstreamProviderGroupChannel{}).Where("provider_group_id = ?", sourceGroupID).
			Update("provider_group_id", destinationGroupID).Error; err != nil {
			return nil, err
		}
		if err := tx.Where("provider_group_id = ?", sourceGroupID).Delete(&UpstreamProviderGroupProfitDaily{}).Error; err != nil {
			return nil, err
		}
		if err := tx.Delete(&UpstreamProviderGroup{}, sourceGroupID).Error; err != nil {
			return nil, err
		}
	}
	return groupsByName, nil
}

func persistUpstreamProviderGroupDailyCosts(tx *gorm.DB, groupsByName map[string]UpstreamProviderGroup, inputs []UpstreamProviderGroupSnapshotInput) error {
	for _, input := range inputs {
		groupName := strings.TrimSpace(input.GroupName)
		group, ok := groupsByName[groupName]
		if !ok || input.CostDate == "" {
			continue
		}
		var daily UpstreamProviderGroupProfitDaily
		err := tx.Where("date = ? AND provider_group_id = ?", input.CostDate, group.Id).First(&daily).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			daily = UpstreamProviderGroupProfitDaily{
				Date: input.CostDate, ProviderGroupId: group.Id,
				ProviderUsageQuota: input.ProviderUsageQuota, ProviderCost: input.ProviderCost,
				CostStatus: input.CostStatus,
			}
			if input.CostStatus == "ready" {
				daily.CostObservedAt = input.CostObservedAt
			}
			if err := tx.Create(&daily).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		updates := map[string]any{"cost_status": input.CostStatus}
		if input.CostStatus == "ready" {
			updates["provider_usage_quota"] = input.ProviderUsageQuota
			updates["provider_cost"] = input.ProviderCost
			updates["cost_observed_at"] = input.CostObservedAt
		}
		if err := tx.Model(&daily).Updates(updates).Error; err != nil {
			return err
		}
	}
	return nil
}

func GetUpstreamProviderSyncRuns(providerId int, accountId int, limit int) ([]UpstreamProviderSyncRun, error) {
	query := DB.Model(&UpstreamProviderSyncRun{})
	if providerId > 0 {
		query = query.Where("provider_id = ?", providerId)
	}
	if accountId > 0 {
		query = query.Where("account_id = ?", accountId)
	}
	var runs []UpstreamProviderSyncRun
	return runs, query.Order("id desc").Limit(limit).Find(&runs).Error
}

func GetUpstreamProviderForSync(id int) (*UpstreamProvider, error) {
	var provider UpstreamProvider
	if err := preloadUpstreamProviderWorkspace(DB).First(&provider, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func GetDueUpstreamProviderSyncs(now int64) ([]UpstreamProvider, error) {
	var providers []UpstreamProvider
	if err := DB.Preload("Accounts", func(tx *gorm.DB) *gorm.DB {
		return tx.Where("status = ? AND auto_sync = ?", UpstreamProviderStatusEnabled, true).Order("id asc")
	}).Where("status = ?", UpstreamProviderStatusEnabled).Find(&providers).Error; err != nil {
		return nil, err
	}
	for providerIndex := range providers {
		accounts := providers[providerIndex].Accounts[:0]
		for _, account := range providers[providerIndex].Accounts {
			if account.NextSyncAt == 0 || account.NextSyncAt <= now {
				accounts = append(accounts, account)
			}
		}
		providers[providerIndex].Accounts = accounts
	}
	return providers, nil
}

func GetUpstreamProviderAccountById(id int) (*UpstreamProviderAccount, error) {
	var account UpstreamProviderAccount
	if err := DB.First(&account, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func AdjustUpstreamProviderAccountRecharge(id int, delta decimal.Decimal) (*UpstreamProviderAccount, error) {
	var adjusted UpstreamProviderAccount
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&adjusted, "id = ?", id).Error; err != nil {
			return err
		}
		totalRecharge := adjusted.TotalRecharge.Add(delta)
		if totalRecharge.IsNegative() {
			totalRecharge = decimal.Zero
		}
		if totalRecharge.GreaterThan(maxUpstreamProviderRechargeAmount) {
			return ErrUpstreamProviderRechargeAmountTooLarge
		}
		adjusted.TotalRecharge = totalRecharge
		return tx.Model(&adjusted).Updates(map[string]any{
			"total_recharge": totalRecharge,
			"updated_at":     common.GetTimestamp(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &adjusted, nil
}

func GetUpstreamProviderKeysByAccountIDs(ids []int) ([]UpstreamProviderKey, error) {
	if len(ids) == 0 {
		return []UpstreamProviderKey{}, nil
	}
	var keys []UpstreamProviderKey
	return keys, DB.Where("account_id IN ?", ids).Order("id asc").Find(&keys).Error
}

type UpstreamProviderGroupContext struct {
	GroupID      int
	GroupName    string
	AccountID    int
	AccountName  string
	ProviderID   int
	ProviderName string
	LastSyncedAt int64
}

func GetUpstreamProviderGroupContexts(ids []int) ([]UpstreamProviderGroupContext, error) {
	if len(ids) == 0 {
		return []UpstreamProviderGroupContext{}, nil
	}
	var contexts []UpstreamProviderGroupContext
	err := DB.Table("xnewapi_upstream_provider_groups AS provider_group").
		Select("provider_group.id AS group_id, provider_group.name AS group_name, account.id AS account_id, account.name AS account_name, provider.id AS provider_id, provider.name AS provider_name, account.last_synced_at").
		Joins("JOIN xnewapi_upstream_provider_accounts AS account ON account.id = provider_group.account_id").
		Joins("JOIN xnewapi_upstream_providers AS provider ON provider.id = account.provider_id").
		Where("provider_group.id IN ?", ids).Find(&contexts).Error
	return contexts, err
}

func GetUpstreamProviderGroupChannels(groupIDs []int) ([]UpstreamProviderGroupChannel, error) {
	if len(groupIDs) == 0 {
		return []UpstreamProviderGroupChannel{}, nil
	}
	var mappings []UpstreamProviderGroupChannel
	return mappings, DB.Where("provider_group_id IN ?", groupIDs).Order("provider_group_id asc, channel_id asc").Find(&mappings).Error
}

func GetUpstreamProviderGroupProfitDailyRange(startDate string, endDate string) ([]UpstreamProviderGroupProfitDaily, error) {
	var rows []UpstreamProviderGroupProfitDaily
	return rows, DB.Where("date >= ? AND date <= ?", startDate, endDate).
		Order("date asc, provider_group_id asc").Find(&rows).Error
}

func GetUpstreamProviderGroupProfitDailyByDate(date string) ([]UpstreamProviderGroupProfitDaily, error) {
	var rows []UpstreamProviderGroupProfitDaily
	return rows, DB.Where("date = ?", date).Order("provider_group_id asc").Find(&rows).Error
}

func UpdateUpstreamProviderGroupProfitDailyRevenue(id int, revenueQuota int64, revenueAmount decimal.Decimal) error {
	return DB.Model(&UpstreamProviderGroupProfitDaily{}).Where("id = ?", id).Updates(map[string]any{
		"revenue_quota": revenueQuota, "revenue_amount": revenueAmount,
	}).Error
}

func markUpstreamProviderSyncAPIKeyPresence(provider *UpstreamProvider) {
	if provider == nil {
		return
	}
	for index := range provider.Accounts {
		provider.Accounts[index].HasSyncAPIKey = provider.Accounts[index].SyncAPIKey != ""
	}
}

func replaceUpstreamProviderGroupChannels(tx *gorm.DB, groupID int, channelIDs []int) error {
	uniqueChannelIDs := make(map[int]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		if channelID <= 0 {
			return errors.New("invalid channel id")
		}
		uniqueChannelIDs[channelID] = struct{}{}
	}
	if len(uniqueChannelIDs) > 0 {
		ids := make([]int, 0, len(uniqueChannelIDs))
		for channelID := range uniqueChannelIDs {
			ids = append(ids, channelID)
		}
		var channelCount int64
		if err := tx.Model(&Channel{}).Where("id IN ?", ids).Count(&channelCount).Error; err != nil {
			return err
		}
		if channelCount != int64(len(ids)) {
			return gorm.ErrRecordNotFound
		}
		var mapped UpstreamProviderGroupChannel
		err := lockForUpdate(tx).Where("channel_id IN ? AND provider_group_id <> ?", ids, groupID).First(&mapped).Error
		if err == nil {
			return ErrUpstreamProviderChannelMapped
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if err := tx.Where("provider_group_id = ?", groupID).Delete(&UpstreamProviderGroupChannel{}).Error; err != nil {
		return err
	}
	for channelID := range uniqueChannelIDs {
		if err := tx.Create(&UpstreamProviderGroupChannel{ProviderGroupId: groupID, ChannelId: channelID}).Error; err != nil {
			if isUniqueConstraintError(err) {
				return ErrUpstreamProviderChannelMapped
			}
			return err
		}
	}
	return nil
}
