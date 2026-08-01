// FORK-CUSTOM: Verify provider-group channel mapping and synchronized group persistence.
package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUpstreamProviderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	originalDB := DB
	originalLogDB := LOG_DB
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	initCol()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&Channel{},
		&UpstreamProvider{},
		&UpstreamProviderAccount{},
		&UpstreamProviderGroup{},
		&UpstreamProviderGroupChannel{},
		&UpstreamProviderKey{},
		&UpstreamProviderSyncRun{},
		&UpstreamProviderGroupProfitDaily{},
	))
	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
		initCol()
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func createProviderAccountFixture(t *testing.T, db *gorm.DB) (*UpstreamProvider, *UpstreamProviderAccount) {
	t.Helper()
	provider := &UpstreamProvider{
		Name: "provider", Status: UpstreamProviderStatusEnabled, AuthenticationMethod: "api_key",
		SyncIntervalMinutes: 60, RechargeRatio: decimal.NewFromInt(1),
		QuotaConversionBase: decimal.NewFromInt(500000), Currency: "USD",
	}
	require.NoError(t, db.Create(provider).Error)
	account := &UpstreamProviderAccount{ProviderId: provider.Id, Name: "account", Status: UpstreamProviderStatusEnabled}
	require.NoError(t, db.Create(account).Error)
	return provider, account
}

func TestLatestProviderSchemaMigratesCleanly(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	require.NoError(t, db.AutoMigrate(upstreamProviderMigrationModels()...))
	require.NoError(t, db.AutoMigrate(upstreamProviderMigrationModels()...))
	assert.True(t, db.Migrator().HasColumn(&UpstreamProviderKey{}, "provider_group_id"))
	assert.True(t, db.Migrator().HasColumn(&UpstreamProviderAccount{}, "login_username"))
	assert.False(t, db.Migrator().HasColumn(&UpstreamProviderKey{}, "group_name"))
	assert.False(t, db.Migrator().HasColumn(&UpstreamProviderKey{}, "channel_id"))
}

func TestLegacySyncEndpointColumnDoesNotBlockProviderWrites(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	require.NoError(t, db.Exec("ALTER TABLE xnewapi_upstream_providers ADD COLUMN sync_endpoint varchar(512) NOT NULL DEFAULT ''").Error)
	require.NoError(t, db.AutoMigrate(upstreamProviderMigrationModels()...))
	require.NoError(t, db.AutoMigrate(upstreamProviderMigrationModels()...))
	assert.True(t, db.Migrator().HasColumn("xnewapi_upstream_providers", "sync_endpoint"))

	provider := &UpstreamProvider{
		Name: "legacy-compatible", Status: UpstreamProviderStatusEnabled, AuthenticationMethod: "api_key",
		AdapterType: "newapi", SyncIntervalMinutes: 60, RechargeRatio: decimal.NewFromInt(1),
		QuotaConversionBase: decimal.NewFromInt(500000), Currency: "USD", SyncStatus: "never",
	}
	require.NoError(t, db.Create(provider).Error)
	var legacyValue string
	require.NoError(t, db.Raw("SELECT sync_endpoint FROM xnewapi_upstream_providers WHERE id = ?", provider.Id).Scan(&legacyValue).Error)
	assert.Empty(t, legacyValue)
}

func TestAdjustUpstreamProviderAccountRechargeClampsAtZero(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	_, account := createProviderAccountFixture(t, db)
	account.TotalRecharge = decimal.RequireFromString("25.50")
	require.NoError(t, db.Save(account).Error)

	adjusted, err := AdjustUpstreamProviderAccountRecharge(account.Id, decimal.RequireFromString("10.25"))
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("35.75").Equal(adjusted.TotalRecharge))

	adjusted, err = AdjustUpstreamProviderAccountRecharge(account.Id, decimal.RequireFromString("-100"))
	require.NoError(t, err)
	assert.True(t, adjusted.TotalRecharge.IsZero())

	var persisted UpstreamProviderAccount
	require.NoError(t, db.First(&persisted, account.Id).Error)
	assert.True(t, persisted.TotalRecharge.IsZero())

	_, err = AdjustUpstreamProviderAccountRecharge(
		account.Id,
		maxUpstreamProviderRechargeAmount.Add(decimal.NewFromInt(1)),
	)
	require.ErrorIs(t, err, ErrUpstreamProviderRechargeAmountTooLarge)
}

func TestProviderGroupMapsMultipleChannelsAndKeepsChannelsExclusive(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	_, account := createProviderAccountFixture(t, db)
	firstGroup := &UpstreamProviderGroup{AccountId: account.Id, Name: "group-a"}
	secondGroup := &UpstreamProviderGroup{AccountId: account.Id, Name: "group-b"}
	require.NoError(t, db.Create(firstGroup).Error)
	require.NoError(t, db.Create(secondGroup).Error)
	channels := []Channel{
		{Name: "first", Status: common.ChannelStatusEnabled},
		{Name: "second", Status: common.ChannelStatusEnabled},
	}
	require.NoError(t, db.Create(&channels).Error)

	require.NoError(t, replaceUpstreamProviderGroupChannels(db, firstGroup.Id, []int{channels[0].Id, channels[1].Id}))
	var mappings []UpstreamProviderGroupChannel
	require.NoError(t, db.Where("provider_group_id = ?", firstGroup.Id).Order("channel_id asc").Find(&mappings).Error)
	require.Len(t, mappings, 2)
	assert.Equal(t, []int{channels[0].Id, channels[1].Id}, []int{mappings[0].ChannelId, mappings[1].ChannelId})

	err := replaceUpstreamProviderGroupChannels(db, secondGroup.Id, []int{channels[0].Id})
	require.ErrorIs(t, err, ErrUpstreamProviderChannelMapped)
}

func TestSaveProviderWorkspaceRollsBackConflictingGroupMappings(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	channel := &Channel{Name: "shared", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(channel).Error)
	workspace := &UpstreamProviderWorkspace{
		Provider: UpstreamProvider{
			Name: "transactional", Status: UpstreamProviderStatusEnabled, AuthenticationMethod: "api_key",
			AdapterType: "newapi", SyncIntervalMinutes: 60, RechargeRatio: decimal.NewFromInt(1),
			QuotaConversionBase: decimal.NewFromInt(500000), Currency: "USD", SyncStatus: "never",
		},
		Accounts: []UpstreamProviderWorkspaceAccount{{
			Account: UpstreamProviderAccount{Name: "account", Status: UpstreamProviderStatusEnabled, SyncStatus: "never"},
			Groups: []UpstreamProviderWorkspaceGroup{
				{Group: UpstreamProviderGroup{Name: "group-a"}, ChannelIds: []int{channel.Id}},
				{Group: UpstreamProviderGroup{Name: "group-b"}, ChannelIds: []int{channel.Id}},
			},
		}},
	}

	_, err := SaveUpstreamProviderWorkspace(workspace)
	require.ErrorIs(t, err, ErrUpstreamProviderChannelMapped)
	for name, value := range map[string]any{
		"providers": &UpstreamProvider{}, "accounts": &UpstreamProviderAccount{},
		"groups": &UpstreamProviderGroup{}, "mappings": &UpstreamProviderGroupChannel{},
	} {
		var count int64
		require.NoError(t, db.Model(value).Count(&count).Error)
		assert.Zero(t, count, name)
	}
}

func TestPersistProviderGroupDailyCostOverwritesSameDay(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	_, account := createProviderAccountFixture(t, db)
	group := UpstreamProviderGroup{AccountId: account.Id, Name: "group-a"}
	require.NoError(t, db.Create(&group).Error)
	input := UpstreamProviderGroupSnapshotInput{
		GroupName: "group-a", CostDate: "2026-07-30", ProviderUsageQuota: decimal.NewFromInt(100),
		ProviderCost: decimal.NewFromFloat(0.2), CostObservedAt: 1000, CostStatus: "ready",
	}
	groups := map[string]UpstreamProviderGroup{"group-a": group}
	require.NoError(t, persistUpstreamProviderGroupDailyCosts(db, groups, []UpstreamProviderGroupSnapshotInput{input}))
	input.ProviderUsageQuota = decimal.NewFromInt(150)
	input.ProviderCost = decimal.NewFromFloat(0.3)
	input.CostObservedAt = 2000
	require.NoError(t, persistUpstreamProviderGroupDailyCosts(db, groups, []UpstreamProviderGroupSnapshotInput{input}))

	var rows []UpstreamProviderGroupProfitDaily
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, group.Id, rows[0].ProviderGroupId)
	assert.True(t, rows[0].ProviderUsageQuota.Equal(decimal.NewFromInt(150)))
	assert.True(t, rows[0].ProviderCost.Equal(decimal.NewFromFloat(0.3)))
	assert.Equal(t, int64(2000), rows[0].CostObservedAt)
}

func TestPersistProviderKeyMovesCurrentGroupByStableID(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	_, account := createProviderAccountFixture(t, db)
	oldGroup := &UpstreamProviderGroup{AccountId: account.Id, Name: "old-group"}
	require.NoError(t, db.Create(oldGroup).Error)
	key := &UpstreamProviderKey{
		AccountId: account.Id, ProviderGroupId: oldGroup.Id, ExternalId: "stable-7",
		Name: "old-name", Status: UpstreamProviderKeyStatusActive,
	}
	require.NoError(t, db.Create(key).Error)

	groups, err := persistUpstreamProviderGroupsAndKeys(db, account.Id, []UpstreamProviderKeySnapshotInput{{
		ExternalID: "stable-7", Name: "new-name", GroupName: "new-group", Status: UpstreamProviderKeyStatusActive,
	}})
	require.NoError(t, err)
	var updated UpstreamProviderKey
	require.NoError(t, db.First(&updated, key.Id).Error)
	assert.Equal(t, key.Id, updated.Id)
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, groups["new-group"].Id, updated.ProviderGroupId)
	var oldGroupCount int64
	require.NoError(t, db.Model(&UpstreamProviderGroup{}).Where("id = ?", oldGroup.Id).Count(&oldGroupCount).Error)
	assert.Zero(t, oldGroupCount)
}

func TestPersistProviderKeyMoveTransfersOldGroupChannels(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	_, account := createProviderAccountFixture(t, db)
	oldGroup := &UpstreamProviderGroup{AccountId: account.Id, Name: "old-group"}
	require.NoError(t, db.Create(oldGroup).Error)
	channel := &Channel{Name: "mapped", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&UpstreamProviderGroupChannel{ProviderGroupId: oldGroup.Id, ChannelId: channel.Id}).Error)
	require.NoError(t, db.Create(&UpstreamProviderKey{
		AccountId: account.Id, ProviderGroupId: oldGroup.Id, ExternalId: "stable-7",
		Name: "old-name", Status: UpstreamProviderKeyStatusActive,
	}).Error)

	groups, err := persistUpstreamProviderGroupsAndKeys(db, account.Id, []UpstreamProviderKeySnapshotInput{{
		ExternalID: "stable-7", Name: "new-name", GroupName: "new-group", Status: UpstreamProviderKeyStatusActive,
	}})
	require.NoError(t, err)
	var mapping UpstreamProviderGroupChannel
	require.NoError(t, db.Where("channel_id = ?", channel.Id).First(&mapping).Error)
	assert.Equal(t, groups["new-group"].Id, mapping.ProviderGroupId)
}

func TestProviderKeyExternalIDIsUniqueWithinAccount(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	_, account := createProviderAccountFixture(t, db)
	group := &UpstreamProviderGroup{AccountId: account.Id, Name: "group-a"}
	require.NoError(t, db.Create(group).Error)
	key := UpstreamProviderKey{
		AccountId: account.Id, ProviderGroupId: group.Id, ExternalId: "stable-7",
		Name: "first", Status: UpstreamProviderKeyStatusActive,
	}
	require.NoError(t, db.Create(&key).Error)
	duplicate := UpstreamProviderKey{
		AccountId: account.Id, ProviderGroupId: group.Id, ExternalId: "stable-7",
		Name: "duplicate", Status: UpstreamProviderKeyStatusActive,
	}
	require.Error(t, db.Create(&duplicate).Error)
}

func TestGetProviderForSyncPreloadsGroupsAndKeys(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	provider, account := createProviderAccountFixture(t, db)
	group := &UpstreamProviderGroup{AccountId: account.Id, Name: "group-a"}
	require.NoError(t, db.Create(group).Error)
	require.NoError(t, db.Create(&UpstreamProviderKey{
		AccountId: account.Id, ProviderGroupId: group.Id, ExternalId: "7",
		Name: "key", Status: UpstreamProviderKeyStatusActive,
	}).Error)

	synced, err := GetUpstreamProviderForSync(provider.Id)
	require.NoError(t, err)
	require.Len(t, synced.Accounts, 1)
	require.Len(t, synced.Accounts[0].Groups, 1)
	require.Len(t, synced.Accounts[0].Groups[0].Keys, 1)
	assert.Equal(t, "7", synced.Accounts[0].Groups[0].Keys[0].ExternalId)
}

func TestGetProviderRevenueRowsFiltersByChannel(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}))
	require.NoError(t, db.Create([]Log{
		{Type: LogTypeConsume, CreatedAt: 100, ChannelId: 9, Group: "group-a", Quota: 100},
		{Type: LogTypeConsume, CreatedAt: 101, ChannelId: 10, Group: "group-a", Quota: 200},
		{Type: LogTypeRefund, CreatedAt: 102, ChannelId: 9, Group: "group-a", Quota: 300},
	}).Error)

	rows, err := GetUpstreamProviderRevenueRows([]int{9}, 0, 200)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, 9, rows[0].ChannelID)
	assert.Equal(t, int64(100), rows[0].Quota)
}

func TestFinishProviderSyncDoesNotOverwriteWorkspaceFields(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	provider, account := createProviderAccountFixture(t, db)
	account.SyncAPIKey = "old-key"
	require.NoError(t, db.Save(account).Error)

	require.NoError(t, db.Model(provider).Updates(map[string]any{
		"name": "renamed-provider", "endpoint": "https://new.example.com",
	}).Error)
	require.NoError(t, db.Model(account).Updates(map[string]any{
		"name": "renamed-account", "sync_api_key": "new-key", "auto_sync": false,
	}).Error)

	staleAccount := *account
	staleAccount.Balance = decimal.NewFromInt(12)
	staleAccount.TotalRecharge = decimal.NewFromInt(30)
	staleAccount.BalanceUpdatedAt = 100
	staleAccount.SyncStatus = "success"
	staleAccount.LastSyncedAt = 100
	staleAccount.LastSyncAttemptAt = 100
	staleAccount.NextSyncAt = 200
	staleAccount.LastSyncError = ""
	run := &UpstreamProviderSyncRun{
		ProviderId: provider.Id, AccountId: account.Id, Status: "success", StartedAt: 50, FinishedAt: 100,
	}

	require.NoError(t, FinishUpstreamProviderSync(run, &staleAccount, nil, nil))
	require.NoError(t, UpdateUpstreamProviderSyncState(provider.Id, "success", 100, ""))

	var savedProvider UpstreamProvider
	require.NoError(t, db.First(&savedProvider, provider.Id).Error)
	assert.Equal(t, "renamed-provider", savedProvider.Name)
	assert.Equal(t, "https://new.example.com", savedProvider.Endpoint)
	assert.Equal(t, "success", savedProvider.SyncStatus)

	var savedAccount UpstreamProviderAccount
	require.NoError(t, db.First(&savedAccount, account.Id).Error)
	assert.Equal(t, "renamed-account", savedAccount.Name)
	assert.Equal(t, "new-key", savedAccount.SyncAPIKey)
	assert.False(t, savedAccount.AutoSync)
	assert.True(t, savedAccount.Balance.Equal(decimal.NewFromInt(12)))
	assert.True(t, savedAccount.TotalRecharge.Equal(decimal.NewFromInt(30)))
}

func TestFinishProviderSyncPersistsObservedExternalAccountID(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	provider, account := createProviderAccountFixture(t, db)
	account.ExternalId = "4841"
	account.ExternalIdObserved = true
	account.SyncStatus = "success"
	run := &UpstreamProviderSyncRun{
		ProviderId: provider.Id, AccountId: account.Id, Status: "success", StartedAt: 50, FinishedAt: 100,
	}

	require.NoError(t, FinishUpstreamProviderSync(run, account, nil, nil))
	var savedAccount UpstreamProviderAccount
	require.NoError(t, db.First(&savedAccount, account.Id).Error)
	assert.Equal(t, "4841", savedAccount.ExternalId)
}

func TestDeleteUpstreamProviderAccountRemovesOwnedRecords(t *testing.T) {
	db := setupUpstreamProviderTestDB(t)
	_, account := createProviderAccountFixture(t, db)
	group := &UpstreamProviderGroup{AccountId: account.Id, Name: "group-a"}
	require.NoError(t, db.Create(group).Error)
	require.NoError(t, db.Create(&UpstreamProviderGroupChannel{ProviderGroupId: group.Id, ChannelId: 1}).Error)
	require.NoError(t, db.Create(&UpstreamProviderKey{AccountId: account.Id, ProviderGroupId: group.Id, ExternalId: "key", Name: "key", Status: UpstreamProviderKeyStatusActive}).Error)
	require.NoError(t, db.Create(&UpstreamProviderGroupProfitDaily{Date: "2026-07-31", ProviderGroupId: group.Id, CostStatus: "ready"}).Error)
	require.NoError(t, db.Create(&UpstreamProviderSyncRun{ProviderId: account.ProviderId, AccountId: account.Id, Status: "success"}).Error)

	require.NoError(t, DeleteUpstreamProviderAccount(account.Id))
	for name, value := range map[string]any{
		"accounts": &UpstreamProviderAccount{}, "groups": &UpstreamProviderGroup{},
		"mappings": &UpstreamProviderGroupChannel{}, "keys": &UpstreamProviderKey{},
		"profits": &UpstreamProviderGroupProfitDaily{}, "runs": &UpstreamProviderSyncRun{},
	} {
		var count int64
		require.NoError(t, db.Model(value).Count(&count).Error)
		assert.Zero(t, count, name)
	}
}
