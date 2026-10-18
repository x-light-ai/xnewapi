// FORK-CUSTOM: Verify provider-group profitability across multiple mapped channels.
package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProviderProfitRankingMergesKeysInTheSameGroup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.UpstreamProvider{},
		&model.UpstreamProviderAccount{},
		&model.UpstreamProviderGroup{},
		&model.UpstreamProviderGroupChannel{},
		&model.UpstreamProviderKey{},
		&model.UpstreamProviderGroupProfitDaily{},
	))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	provider := model.UpstreamProvider{Name: "provider", Status: model.UpstreamProviderStatusEnabled}
	require.NoError(t, db.Create(&provider).Error)
	account := model.UpstreamProviderAccount{
		ProviderId: provider.Id,
		Name:       "account",
		Status:     model.UpstreamProviderStatusEnabled,
	}
	require.NoError(t, db.Create(&account).Error)
	group := model.UpstreamProviderGroup{AccountId: account.Id, Name: "group-a"}
	require.NoError(t, db.Create(&group).Error)
	key := model.UpstreamProviderKey{
		AccountId: account.Id, ProviderGroupId: group.Id,
		ExternalId: "key-1", Name: "unmapped-key", Status: model.UpstreamProviderKeyStatusActive,
	}
	require.NoError(t, db.Create(&key).Error)
	secondKey := model.UpstreamProviderKey{
		AccountId: account.Id, ProviderGroupId: group.Id,
		ExternalId: "key-2", Name: "second-key", Status: model.UpstreamProviderKeyStatusActive,
	}
	require.NoError(t, db.Create(&secondKey).Error)

	date := time.Now().In(time.Local).Format("2006-01-02")
	require.NoError(t, db.Create(&model.UpstreamProviderGroupProfitDaily{
		Date:               date,
		ProviderGroupId:    group.Id,
		ProviderCost:       decimal.RequireFromString("0.023626"),
		ProviderUsageQuota: decimal.NewFromInt(47252),
		RevenueAmount:      decimal.NewFromInt(3),
		CostStatus:         "ready",
	}).Error)

	profits, err := GetProviderGroupProfitRanking(date, date)
	require.NoError(t, err)
	require.Len(t, profits, 1)
	assert.Equal(t, "group-a", profits[0].GroupName)
	assert.Equal(t, group.Id, profits[0].GroupID)
	assert.Equal(t, []int{key.Id, secondKey.Id}, profits[0].KeyIDs)
	assert.True(t, profits[0].Revenue.Equal(decimal.NewFromInt(3)))
	require.NotNil(t, profits[0].Cost)
	assert.True(t, profits[0].Cost.Equal(decimal.RequireFromString("0.023626")))
}

func TestProviderProfitDetailsFiltersGroupAndPreservesDailyQuota(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.UpstreamProvider{}, &model.UpstreamProviderAccount{},
		&model.UpstreamProviderGroup{}, &model.UpstreamProviderGroupProfitDaily{},
	))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	provider := model.UpstreamProvider{Name: "provider", Status: model.UpstreamProviderStatusEnabled}
	require.NoError(t, db.Create(&provider).Error)
	account := model.UpstreamProviderAccount{ProviderId: provider.Id, Name: "account", Status: model.UpstreamProviderStatusEnabled}
	require.NoError(t, db.Create(&account).Error)
	group := model.UpstreamProviderGroup{AccountId: account.Id, Name: "group-a"}
	otherGroup := model.UpstreamProviderGroup{AccountId: account.Id, Name: "group-b"}
	require.NoError(t, db.Create(&group).Error)
	require.NoError(t, db.Create(&otherGroup).Error)
	date := time.Now().In(time.Local).Format("2006-01-02")
	require.NoError(t, db.Create([]model.UpstreamProviderGroupProfitDaily{
		{
			Date: date, ProviderGroupId: group.Id, RevenueQuota: 635979,
			RevenueAmount: decimal.RequireFromString("1.271958"), ProviderUsageQuota: decimal.NewFromInt(635979),
			ProviderCost: decimal.RequireFromString("9.412489"), CostStatus: "ready",
		},
		{
			Date: date, ProviderGroupId: otherGroup.Id, RevenueQuota: 100,
			RevenueAmount: decimal.RequireFromString("0.0002"), CostStatus: "unavailable",
		},
	}).Error)

	items, err := GetProviderGroupProfitDetails(date, date, provider.Id, group.Id)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, int64(635979), items[0].RevenueQuota)
	assert.True(t, items[0].ProviderUsageQuota.Equal(decimal.NewFromInt(635979)))
	require.NotNil(t, items[0].Cost)
	assert.True(t, items[0].Cost.Equal(decimal.RequireFromString("9.412489")))
	require.NotNil(t, items[0].Profit)
	assert.True(t, items[0].Profit.Equal(decimal.RequireFromString("-8.140531")))

	allItems, err := GetProviderGroupProfitDetails(date, date, provider.Id, 0)
	require.NoError(t, err)
	require.Len(t, allItems, 2)
	assert.Equal(t, "unavailable", allItems[1].CostStatus)
	assert.Nil(t, allItems[1].Cost)
	assert.Nil(t, allItems[1].Profit)

	paged, err := GetProviderGroupProfitDetailsPage(date, date, provider.Id, 0, 2, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), paged.Total)
	assert.Equal(t, 2, paged.Page)
	assert.Equal(t, 1, paged.PageSize)
	require.Len(t, paged.Items, 1)
	assert.True(t, paged.Revenue.Equal(decimal.RequireFromString("1.272158")))
	assert.True(t, paged.Items[0].Revenue.Equal(decimal.RequireFromString("0.0002")))
	assert.Nil(t, paged.Cost)
}

func TestProviderProfitDailyTrendAggregatesGroupsAndMarksIncompleteCosts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.UpstreamProviderGroupProfitDaily{}))
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	day := time.Now().In(time.Local)
	firstDate := day.AddDate(0, 0, -1).Format("2006-01-02")
	secondDate := day.Format("2006-01-02")
	require.NoError(t, db.Create([]model.UpstreamProviderGroupProfitDaily{
		{
			Date: firstDate, ProviderGroupId: 1, RevenueAmount: decimal.NewFromInt(10),
			ProviderCost: decimal.NewFromInt(4), CostStatus: "ready",
		},
		{
			Date: firstDate, ProviderGroupId: 2, RevenueAmount: decimal.NewFromInt(5),
			ProviderCost: decimal.NewFromInt(2), CostStatus: "ready",
		},
		{
			Date: secondDate, ProviderGroupId: 1,
			RevenueAmount: decimal.NewFromInt(8), CostStatus: "unavailable",
		},
	}).Error)

	trend, err := GetProviderProfitDailyTrend(firstDate, secondDate)
	require.NoError(t, err)
	require.Len(t, trend, 2)
	assert.Equal(t, firstDate, trend[0].Date)
	assert.True(t, trend[0].Revenue.Equal(decimal.NewFromInt(15)))
	require.NotNil(t, trend[0].Cost)
	assert.True(t, trend[0].Cost.Equal(decimal.NewFromInt(6)))
	require.NotNil(t, trend[0].Profit)
	assert.True(t, trend[0].Profit.Equal(decimal.NewFromInt(9)))
	assert.Equal(t, "unavailable", trend[1].CostStatus)
	assert.Nil(t, trend[1].Cost)
	assert.Nil(t, trend[1].Profit)
}

func TestRefreshProviderProfitSumsAllMappedChannels(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{}, &model.Log{}, &model.UpstreamProviderGroup{},
		&model.UpstreamProviderGroupChannel{}, &model.UpstreamProviderGroupProfitDaily{},
	))
	originalDB, originalLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() { model.DB, model.LOG_DB = originalDB, originalLogDB })

	group := model.UpstreamProviderGroup{AccountId: 1, Name: "group-a"}
	require.NoError(t, db.Create(&group).Error)
	channels := []model.Channel{
		{Name: "first", Status: common.ChannelStatusEnabled},
		{Name: "second", Status: common.ChannelStatusEnabled},
	}
	require.NoError(t, db.Create(&channels).Error)
	require.NoError(t, db.Create([]model.UpstreamProviderGroupChannel{
		{ProviderGroupId: group.Id, ChannelId: channels[0].Id},
		{ProviderGroupId: group.Id, ChannelId: channels[1].Id},
	}).Error)
	day := time.Now().In(time.Local)
	date := day.Format("2006-01-02")
	require.NoError(t, db.Create(&model.UpstreamProviderGroupProfitDaily{
		Date: date, ProviderGroupId: group.Id, CostStatus: "ready",
	}).Error)
	startAt := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create([]model.Log{
		{Type: model.LogTypeConsume, CreatedAt: startAt + 1, ChannelId: channels[0].Id, Quota: 100},
		{Type: model.LogTypeConsume, CreatedAt: startAt + 2, ChannelId: channels[1].Id, Quota: 200},
		{Type: model.LogTypeConsume, CreatedAt: startAt + 3, ChannelId: 999, Quota: 400},
	}).Error)

	require.NoError(t, refreshProviderProfitDailyDate(day))
	var daily model.UpstreamProviderGroupProfitDaily
	require.NoError(t, db.Where("provider_group_id = ?", group.Id).First(&daily).Error)
	assert.Equal(t, int64(300), daily.RevenueQuota)
	assert.True(t, daily.RevenueAmount.Equal(decimal.NewFromInt(300).Div(decimal.NewFromFloat(common.QuotaPerUnit))))
}

func TestRebuildProviderProfitKeepsHistoricalRevenueAfterMappingMoves(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{}, &model.Log{}, &model.UpstreamProvider{},
		&model.UpstreamProviderAccount{}, &model.UpstreamProviderGroup{},
		&model.UpstreamProviderGroupChannel{}, &model.UpstreamProviderKey{},
		&model.UpstreamProviderGroupProfitDaily{},
	))
	originalDB, originalLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() { model.DB, model.LOG_DB = originalDB, originalLogDB })

	provider := model.UpstreamProvider{Name: "provider", Status: model.UpstreamProviderStatusEnabled}
	require.NoError(t, db.Create(&provider).Error)
	account := model.UpstreamProviderAccount{
		ProviderId: provider.Id, Name: "account", Status: model.UpstreamProviderStatusEnabled,
	}
	require.NoError(t, db.Create(&account).Error)
	oldGroup := model.UpstreamProviderGroup{AccountId: account.Id, Name: "old-group"}
	newGroup := model.UpstreamProviderGroup{AccountId: account.Id, Name: "new-group"}
	require.NoError(t, db.Create(&oldGroup).Error)
	require.NoError(t, db.Create(&newGroup).Error)
	channel := model.Channel{Name: "moved", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.UpstreamProviderGroupChannel{
		ProviderGroupId: newGroup.Id, ChannelId: channel.Id,
	}).Error)

	day := time.Now().In(time.Local).AddDate(0, 0, -1)
	date := day.Format("2006-01-02")
	originalRevenue := decimal.RequireFromString("1.5")
	require.NoError(t, db.Create(&model.UpstreamProviderGroupProfitDaily{
		Date: date, ProviderGroupId: oldGroup.Id, RevenueQuota: 750000,
		RevenueAmount: originalRevenue, ProviderCost: decimal.RequireFromString("0.5"), CostStatus: "ready",
	}).Error)
	startAt := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.Log{
		Type: model.LogTypeConsume, CreatedAt: startAt + 1, ChannelId: channel.Id, Quota: 999999,
	}).Error)

	profits, err := RebuildProviderProfitDaily(date, date)
	require.NoError(t, err)
	require.Len(t, profits, 1)
	assert.True(t, profits[0].Revenue.Equal(originalRevenue))

	var daily model.UpstreamProviderGroupProfitDaily
	require.NoError(t, db.Where("provider_group_id = ? AND date = ?", oldGroup.Id, date).First(&daily).Error)
	assert.Equal(t, int64(750000), daily.RevenueQuota)
	assert.True(t, daily.RevenueAmount.Equal(originalRevenue))
}

func TestSaveProviderWorkspaceRefreshesTodayRevenue(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.Channel{}, &model.Log{}, &model.UpstreamProvider{},
		&model.UpstreamProviderAccount{}, &model.UpstreamProviderGroup{},
		&model.UpstreamProviderGroupChannel{}, &model.UpstreamProviderKey{},
		&model.UpstreamProviderGroupProfitDaily{},
	))
	originalDB, originalLogDB := model.DB, model.LOG_DB
	model.DB, model.LOG_DB = db, db
	t.Cleanup(func() { model.DB, model.LOG_DB = originalDB, originalLogDB })

	provider := model.UpstreamProvider{
		Name: "provider", Status: model.UpstreamProviderStatusEnabled, AuthenticationMethod: "api_key",
		AdapterType: "newapi", SyncIntervalMinutes: 60, RechargeRatio: decimal.NewFromInt(1),
		QuotaConversionBase: decimal.NewFromInt(500000), Currency: "USD", SyncStatus: "never",
	}
	require.NoError(t, db.Create(&provider).Error)
	account := model.UpstreamProviderAccount{
		ProviderId: provider.Id, Name: "account", ExternalId: "1", Status: model.UpstreamProviderStatusEnabled,
		AutoSync: true, SyncStatus: "never",
	}
	require.NoError(t, db.Create(&account).Error)
	group := model.UpstreamProviderGroup{AccountId: account.Id, Name: "group-a"}
	require.NoError(t, db.Create(&group).Error)
	channel := model.Channel{Name: "mapped", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	day := time.Now().In(time.Local)
	date := day.Format("2006-01-02")
	require.NoError(t, db.Create(&model.UpstreamProviderGroupProfitDaily{
		Date: date, ProviderGroupId: group.Id, CostStatus: "ready",
	}).Error)
	startAt := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local).Unix()
	require.NoError(t, db.Create(&model.Log{
		Type: model.LogTypeConsume, CreatedAt: startAt + 1, ChannelId: channel.Id, Quota: 250000,
	}).Error)

	autoSync := true
	providerID, accountID, groupID := provider.Id, account.Id, group.Id
	_, err = SaveUpstreamProviderWorkspace(&dto.UpstreamProviderWorkspaceUpsertRequest{
		UpstreamProviderUpsertRequest: dto.UpstreamProviderUpsertRequest{
			Name: provider.Name, Status: provider.Status, AuthenticationMethod: provider.AuthenticationMethod,
			AdapterType: provider.AdapterType, SyncIntervalMinutes: provider.SyncIntervalMinutes,
			RechargeRatio: provider.RechargeRatio, QuotaConversionBase: provider.QuotaConversionBase, Currency: provider.Currency,
		},
		ID: &providerID,
		Accounts: []dto.UpstreamProviderWorkspaceAccountRequest{{
			UpstreamProviderAccountRequest: dto.UpstreamProviderAccountRequest{
				Name: account.Name, ExternalId: account.ExternalId, Status: account.Status, AutoSync: &autoSync,
			},
			ID: &accountID,
			Groups: []dto.UpstreamProviderWorkspaceGroupRequest{{
				ID: &groupID, Name: group.Name, ChannelIds: []int{channel.Id},
			}},
		}},
	})
	require.NoError(t, err)
	var daily model.UpstreamProviderGroupProfitDaily
	require.NoError(t, db.Where("provider_group_id = ? AND date = ?", group.Id, date).First(&daily).Error)
	assert.Equal(t, int64(250000), daily.RevenueQuota)
	assert.True(t, daily.RevenueAmount.Equal(decimal.RequireFromString("0.5")))
}
