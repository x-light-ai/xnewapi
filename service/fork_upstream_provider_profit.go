// FORK-CUSTOM: Aggregate provider-group daily revenue and cost without changing relay billing.
package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

type ProviderGroupProfit struct {
	GroupKey     string           `json:"group_key"`
	GroupID      int              `json:"group_id"`
	GroupName    string           `json:"group_name"`
	ProviderID   int              `json:"provider_id"`
	ProviderName string           `json:"provider_name"`
	AccountID    int              `json:"account_id"`
	AccountName  string           `json:"account_name"`
	KeyIDs       []int            `json:"key_ids"`
	Revenue      decimal.Decimal  `json:"revenue"`
	Cost         *decimal.Decimal `json:"cost"`
	Profit       *decimal.Decimal `json:"profit"`
	GrossMargin  *decimal.Decimal `json:"gross_margin"`
	CostStatus   string           `json:"cost_status"`
	LastSyncedAt int64            `json:"last_synced_at"`
}

type ProviderGroupProfitDailyDetail struct {
	Date               string           `json:"date"`
	GroupID            int              `json:"group_id"`
	GroupName          string           `json:"group_name"`
	ProviderID         int              `json:"provider_id"`
	ProviderName       string           `json:"provider_name"`
	AccountID          int              `json:"account_id"`
	AccountName        string           `json:"account_name"`
	RevenueQuota       int64            `json:"revenue_quota"`
	Revenue            decimal.Decimal  `json:"revenue"`
	ProviderUsageQuota decimal.Decimal  `json:"provider_usage_quota"`
	Cost               *decimal.Decimal `json:"cost"`
	Profit             *decimal.Decimal `json:"profit"`
	GrossMargin        *decimal.Decimal `json:"gross_margin"`
	CostStatus         string           `json:"cost_status"`
	CostObservedAt     int64            `json:"cost_observed_at"`
}

type ProviderGroupProfitDailyDetailPage struct {
	Items       []ProviderGroupProfitDailyDetail `json:"items"`
	Total       int64                            `json:"total"`
	Page        int                              `json:"page"`
	PageSize    int                              `json:"page_size"`
	Revenue     decimal.Decimal                  `json:"revenue"`
	Cost        *decimal.Decimal                 `json:"cost"`
	Profit      *decimal.Decimal                 `json:"profit"`
	GrossMargin *decimal.Decimal                 `json:"gross_margin"`
}

type ProviderProfitDailyTrendPoint struct {
	Date       string           `json:"date"`
	Revenue    decimal.Decimal  `json:"revenue"`
	Cost       *decimal.Decimal `json:"cost"`
	Profit     *decimal.Decimal `json:"profit"`
	CostStatus string           `json:"cost_status"`
}

func GetProviderGroupProfitRanking(startDate string, endDate string) ([]ProviderGroupProfit, error) {
	rows, err := getProviderProfitDailyRows(startDate, endDate)
	if err != nil {
		return nil, err
	}
	type groupTotal struct {
		context      model.UpstreamProviderGroupContext
		keyIDs       map[int]struct{}
		revenue      decimal.Decimal
		cost         decimal.Decimal
		costComplete bool
		costStatus   string
	}
	groupIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		groupIDs = append(groupIDs, row.ProviderGroupId)
	}
	contexts, err := model.GetUpstreamProviderGroupContexts(groupIDs)
	if err != nil {
		return nil, err
	}
	contextByGroup := make(map[int]model.UpstreamProviderGroupContext, len(contexts))
	for _, context := range contexts {
		contextByGroup[context.GroupID] = context
	}
	totals := map[string]*groupTotal{}
	for _, row := range rows {
		context, ok := contextByGroup[row.ProviderGroupId]
		if !ok {
			continue
		}
		groupKey := fmt.Sprintf("%d:%d:%d", context.ProviderID, context.AccountID, context.GroupID)
		total := totals[groupKey]
		if total == nil {
			total = &groupTotal{context: context, keyIDs: map[int]struct{}{}, costComplete: true, costStatus: "ready"}
			totals[groupKey] = total
		}
		total.revenue = total.revenue.Add(row.RevenueAmount)
		if row.CostStatus == "ready" {
			total.cost = total.cost.Add(row.ProviderCost)
			continue
		}
		total.costComplete = false
		if row.CostStatus == "error" || total.costStatus == "ready" {
			total.costStatus = row.CostStatus
		}
	}
	accountIDs := make([]int, 0, len(contexts))
	for _, context := range contexts {
		accountIDs = append(accountIDs, context.AccountID)
	}
	keys, err := model.GetUpstreamProviderKeysByAccountIDs(accountIDs)
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		context, ok := contextByGroup[key.ProviderGroupId]
		if !ok {
			continue
		}
		groupKey := fmt.Sprintf("%d:%d:%d", context.ProviderID, context.AccountID, context.GroupID)
		if total := totals[groupKey]; total != nil {
			total.keyIDs[key.Id] = struct{}{}
		}
	}
	result := make([]ProviderGroupProfit, 0, len(totals))
	for groupKey, total := range totals {
		keyIDs := make([]int, 0, len(total.keyIDs))
		for keyID := range total.keyIDs {
			keyIDs = append(keyIDs, keyID)
		}
		sort.Ints(keyIDs)
		item := ProviderGroupProfit{
			GroupKey: groupKey, GroupID: total.context.GroupID, GroupName: total.context.GroupName,
			ProviderID: total.context.ProviderID, ProviderName: total.context.ProviderName,
			AccountID: total.context.AccountID, AccountName: total.context.AccountName,
			KeyIDs: keyIDs, Revenue: total.revenue, CostStatus: total.costStatus,
			LastSyncedAt: total.context.LastSyncedAt,
		}
		if total.costComplete {
			cost := total.cost
			profit := total.revenue.Sub(cost)
			item.Cost = &cost
			item.Profit = &profit
			if total.revenue.IsPositive() {
				margin := profit.Div(total.revenue).Mul(decimal.NewFromInt(100))
				item.GrossMargin = &margin
			}
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].GrossMargin == nil {
			return false
		}
		if result[j].GrossMargin == nil {
			return true
		}
		if !result[i].GrossMargin.Equal(*result[j].GrossMargin) {
			return result[i].GrossMargin.GreaterThan(*result[j].GrossMargin)
		}
		return result[i].Revenue.GreaterThan(result[j].Revenue)
	})
	return result, nil
}

func GetProviderGroupProfitDetails(startDate string, endDate string, providerID int, groupID int) ([]ProviderGroupProfitDailyDetail, error) {
	rows, err := getProviderProfitDailyRows(startDate, endDate)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		groupIDs = append(groupIDs, row.ProviderGroupId)
	}
	contexts, err := model.GetUpstreamProviderGroupContexts(groupIDs)
	if err != nil {
		return nil, err
	}
	contextByGroup := make(map[int]model.UpstreamProviderGroupContext, len(contexts))
	for _, context := range contexts {
		contextByGroup[context.GroupID] = context
	}
	items := make([]ProviderGroupProfitDailyDetail, 0, len(rows))
	for _, row := range rows {
		context, ok := contextByGroup[row.ProviderGroupId]
		if !ok || (providerID > 0 && context.ProviderID != providerID) || (groupID > 0 && context.GroupID != groupID) {
			continue
		}
		item := ProviderGroupProfitDailyDetail{
			Date: row.Date, GroupID: context.GroupID, GroupName: context.GroupName,
			ProviderID: context.ProviderID, ProviderName: context.ProviderName,
			AccountID: context.AccountID, AccountName: context.AccountName,
			RevenueQuota: row.RevenueQuota, Revenue: row.RevenueAmount,
			ProviderUsageQuota: row.ProviderUsageQuota, CostStatus: row.CostStatus,
			CostObservedAt: row.CostObservedAt,
		}
		if row.CostStatus == "ready" {
			cost := row.ProviderCost
			profit := row.RevenueAmount.Sub(cost)
			item.Cost = &cost
			item.Profit = &profit
			if row.RevenueAmount.IsPositive() {
				margin := profit.Div(row.RevenueAmount).Mul(decimal.NewFromInt(100))
				item.GrossMargin = &margin
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date > items[j].Date
		}
		if items[i].ProviderName != items[j].ProviderName {
			return items[i].ProviderName < items[j].ProviderName
		}
		return items[i].GroupName < items[j].GroupName
	})
	return items, nil
}

func GetProviderGroupProfitDetailsPage(startDate string, endDate string, providerID int, groupID int, page int, pageSize int) (ProviderGroupProfitDailyDetailPage, error) {
	items, err := GetProviderGroupProfitDetails(startDate, endDate, providerID, groupID)
	if err != nil {
		return ProviderGroupProfitDailyDetailPage{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	total := int64(len(items))
	revenue := decimal.Zero
	costTotal := decimal.Zero
	costComplete := true
	for _, item := range items {
		revenue = revenue.Add(item.Revenue)
		if item.Cost == nil {
			costComplete = false
			continue
		}
		costTotal = costTotal.Add(*item.Cost)
	}
	var cost *decimal.Decimal
	var profit *decimal.Decimal
	var margin *decimal.Decimal
	if costComplete {
		cost = &costTotal
		profitValue := revenue.Sub(costTotal)
		profit = &profitValue
		if revenue.IsPositive() {
			marginValue := profitValue.Div(revenue).Mul(decimal.NewFromInt(100))
			margin = &marginValue
		}
	}
	result := ProviderGroupProfitDailyDetailPage{
		Items: items, Total: total, Page: page, PageSize: pageSize,
		Revenue: revenue, Cost: cost, Profit: profit, GrossMargin: margin,
	}
	offset := (int64(page) - 1) * int64(pageSize)
	if offset >= total {
		result.Items = []ProviderGroupProfitDailyDetail{}
		return result, nil
	}
	end := offset + int64(pageSize)
	if end > total {
		end = total
	}
	result.Items = items[int(offset):int(end)]
	return result, nil
}

func GetProviderProfitDailyTrend(startDate string, endDate string) ([]ProviderProfitDailyTrendPoint, error) {
	rows, err := getProviderProfitDailyRows(startDate, endDate)
	if err != nil {
		return nil, err
	}

	type dailyTotal struct {
		revenue      decimal.Decimal
		cost         decimal.Decimal
		costComplete bool
		costStatus   string
	}
	totals := make(map[string]*dailyTotal)
	for _, row := range rows {
		total := totals[row.Date]
		if total == nil {
			total = &dailyTotal{costComplete: true, costStatus: "ready"}
			totals[row.Date] = total
		}
		total.revenue = total.revenue.Add(row.RevenueAmount)
		if row.CostStatus == "ready" {
			total.cost = total.cost.Add(row.ProviderCost)
			continue
		}
		total.costComplete = false
		if row.CostStatus == "error" || total.costStatus == "ready" {
			total.costStatus = row.CostStatus
		}
	}

	dates := make([]string, 0, len(totals))
	for date := range totals {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	result := make([]ProviderProfitDailyTrendPoint, 0, len(dates))
	for _, date := range dates {
		total := totals[date]
		point := ProviderProfitDailyTrendPoint{
			Date:       date,
			Revenue:    total.revenue,
			CostStatus: total.costStatus,
		}
		if total.costComplete {
			cost := total.cost
			profit := total.revenue.Sub(cost)
			point.Cost = &cost
			point.Profit = &profit
		}
		result = append(result, point)
	}
	return result, nil
}

func getProviderProfitDailyRows(startDate string, endDate string) ([]model.UpstreamProviderGroupProfitDaily, error) {
	startAt, endAt, err := providerProfitRange(startDate, endDate)
	if err != nil {
		return nil, err
	}
	startDate = time.Unix(startAt, 0).In(time.Local).Format("2006-01-02")
	endDate = time.Unix(endAt, 0).In(time.Local).Format("2006-01-02")
	return model.GetUpstreamProviderGroupProfitDailyRange(startDate, endDate)
}

func RebuildProviderProfitDaily(startDate string, endDate string) ([]ProviderGroupProfit, error) {
	startAt, endAt, err := providerProfitRange(startDate, endDate)
	if err != nil {
		return nil, err
	}
	today := time.Now().In(time.Local)
	if today.Unix() >= startAt && today.Unix() <= endAt {
		if err := refreshProviderProfitDailyDate(today); err != nil {
			return nil, err
		}
	}
	return GetProviderGroupProfitRanking(startDate, endDate)
}

func refreshProviderProfitDailyDate(day time.Time) error {
	date := day.In(time.Local).Format("2006-01-02")
	if date != time.Now().In(time.Local).Format("2006-01-02") {
		return nil
	}
	rows, err := model.GetUpstreamProviderGroupProfitDailyByDate(date)
	if err != nil || len(rows) == 0 {
		return err
	}
	startAt := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local).Unix()
	endAt := time.Unix(startAt, 0).In(time.Local).AddDate(0, 0, 1).Add(-time.Second).Unix()
	groupIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		groupIDs = append(groupIDs, row.ProviderGroupId)
	}
	mappings, err := model.GetUpstreamProviderGroupChannels(groupIDs)
	if err != nil {
		return err
	}
	channelIDs := make([]int, 0, len(mappings))
	for _, mapping := range mappings {
		channelIDs = append(channelIDs, mapping.ChannelId)
	}
	revenueRows, err := model.GetUpstreamProviderRevenueRows(channelIDs, startAt, endAt)
	if err != nil {
		return err
	}
	revenueByChannel := make(map[int]int64, len(channelIDs))
	for _, revenueRow := range revenueRows {
		if revenueRow.Quota > 0 {
			revenueByChannel[revenueRow.ChannelID] += revenueRow.Quota
		}
	}
	revenueByGroup := make(map[int]int64, len(groupIDs))
	for _, mapping := range mappings {
		revenueByGroup[mapping.ProviderGroupId] += revenueByChannel[mapping.ChannelId]
	}
	for _, daily := range rows {
		revenueQuota := revenueByGroup[daily.ProviderGroupId]
		revenueAmount := decimal.NewFromInt(revenueQuota).Div(decimal.NewFromFloat(common.QuotaPerUnit))
		if err := model.UpdateUpstreamProviderGroupProfitDailyRevenue(daily.Id, revenueQuota, revenueAmount); err != nil {
			return err
		}
	}
	return nil
}

func providerProfitRange(startDate string, endDate string) (int64, int64, error) {
	location := time.Local
	now := time.Now().In(location)
	if startDate == "" {
		startDate = now.AddDate(0, 0, -29).Format("2006-01-02")
	}
	if endDate == "" {
		endDate = now.Format("2006-01-02")
	}
	start, err := time.ParseInLocation("2006-01-02", startDate, location)
	if err != nil {
		return 0, 0, errors.New("invalid start date")
	}
	end, err := time.ParseInLocation("2006-01-02", endDate, location)
	if err != nil {
		return 0, 0, errors.New("invalid end date")
	}
	if end.Before(start) || end.Sub(start) > 366*24*time.Hour {
		return 0, 0, errors.New("profit date range must be within 366 days")
	}
	return start.Unix(), end.Add(24*time.Hour - time.Second).Unix(), nil
}
