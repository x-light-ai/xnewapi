package model

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupChannelMonitorTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err = db.AutoMigrate(&Channel{}, &ChannelMonitorStat{}); err != nil {
		t.Fatalf("failed to migrate channel monitor tables: %v", err)
	}

	resetChannelMonitorTestState()

	t.Cleanup(func() {
		resetChannelMonitorTestState()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func resetChannelMonitorTestState() {
	channelRuntimeStatsStore = sync.Map{}
	channelAggregateRuntimeStatsStore = sync.Map{}
	channelLatencySampleStore = sync.Map{}
	channelMonitorPersistStateStore = sync.Map{}
}

func seedChannelMonitorTestChannel(t *testing.T, db *gorm.DB, name string, channelType int, status int) *Channel {
	t.Helper()

	channel := &Channel{
		Name:        name,
		Type:        channelType,
		Key:         name + "-key",
		Status:      status,
		Group:       "default",
		Models:      "gpt-4",
		CreatedTime: time.Now().Unix(),
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel %s: %v", name, err)
	}
	return channel
}

func seedChannelMonitorStat(t *testing.T, db *gorm.DB, channelID int, bucket time.Time, requestCount int64, successCount int64, failureCount int64, avgLatencyMs float64, p95LatencyMs float64, lastActiveAt time.Time) {
	t.Helper()

	stat := &ChannelMonitorStat{
		ChannelID:    channelID,
		GroupName:    "default",
		ModelName:    "gpt-4",
		TimeBucket:   bucket,
		Granularity:  ChannelMonitorGranularityMinute,
		RequestCount: requestCount,
		SuccessCount: successCount,
		FailureCount: failureCount,
		AvgLatencyMs: avgLatencyMs,
		P95LatencyMs: p95LatencyMs,
		LastActiveAt: lastActiveAt,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
	if err := db.Create(stat).Error; err != nil {
		t.Fatalf("failed to create channel monitor stat: %v", err)
	}
}

func TestGetChannelMonitorSummaryIncludesPersistedAndPendingStats(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	now := time.Now()

	alpha := seedChannelMonitorTestChannel(t, db, "alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	beta := seedChannelMonitorTestChannel(t, db, "beta", constant.ChannelTypeAnthropic, common.ChannelStatusEnabled)

	seedChannelMonitorStat(t, db, alpha.Id, now.Add(-2*time.Hour).Truncate(time.Minute), 10, 9, 1, 100, 180, now.Add(-2*time.Minute))

	ObserveChannelRuntime("default", "gpt-4", beta.Id, true, 200*time.Millisecond)
	ObserveChannelRuntime("default", "gpt-4", beta.Id, false, 400*time.Millisecond)

	summary, err := GetChannelMonitorSummary(7)
	if err != nil {
		t.Fatalf("expected summary without error, got %v", err)
	}
	if summary.TotalRequests != 12 {
		t.Fatalf("expected total requests 12, got %d", summary.TotalRequests)
	}
	if summary.SuccessRate != float64(10)/float64(12) {
		t.Fatalf("expected success rate %v, got %v", float64(10)/float64(12), summary.SuccessRate)
	}
	expectedAvgLatency := float64(10*100+2*300) / float64(12)
	if summary.AvgLatencyMs != expectedAvgLatency {
		t.Fatalf("expected avg latency %.6f, got %.6f", expectedAvgLatency, summary.AvgLatencyMs)
	}
	if summary.ActiveChannels != 2 {
		t.Fatalf("expected 2 active channels, got %d", summary.ActiveChannels)
	}
	if summary.TotalChannels != 2 {
		t.Fatalf("expected 2 total channels, got %d", summary.TotalChannels)
	}
	if summary.TimeRange != "last_7d" {
		t.Fatalf("expected time range last_7d, got %s", summary.TimeRange)
	}
}

func TestGetChannelMonitorSummaryReturnsZeroValuesWhenEmpty(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	_ = seedChannelMonitorTestChannel(t, db, "alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)

	summary, err := GetChannelMonitorSummary(7)
	if err != nil {
		t.Fatalf("expected empty summary without error, got %v", err)
	}
	if summary.TotalRequests != 0 || summary.SuccessRate != 0 || summary.AvgLatencyMs != 0 {
		t.Fatalf("expected zero-value metrics, got %+v", summary)
	}
	if summary.ActiveChannels != 0 {
		t.Fatalf("expected 0 active channels, got %d", summary.ActiveChannels)
	}
	if summary.TotalChannels != 1 {
		t.Fatalf("expected total channels 1, got %d", summary.TotalChannels)
	}
	if summary.TimeRange != "last_7d" {
		t.Fatalf("expected time range last_7d, got %s", summary.TimeRange)
	}
}

func TestGetChannelMonitorHealthBuildsDailyBucketsAndStatuses(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	alpha := seedChannelMonitorTestChannel(t, db, "alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	beta := seedChannelMonitorTestChannel(t, db, "beta", constant.ChannelTypeAnthropic, common.ChannelStatusEnabled)
	gamma := seedChannelMonitorTestChannel(t, db, "gamma", constant.ChannelTypeGemini, common.ChannelStatusEnabled)

	day1 := dayStart.AddDate(0, 0, -2).Add(10 * time.Hour)
	day2 := dayStart.AddDate(0, 0, -1).Add(11 * time.Hour)

	seedChannelMonitorStat(t, db, alpha.Id, day1, 10, 10, 0, 100, 120, day1)
	seedChannelMonitorStat(t, db, alpha.Id, day2, 10, 6, 4, 200, 260, day2)
	seedChannelMonitorStat(t, db, beta.Id, day2, 10, 4, 6, 300, 360, day2)

	ObserveChannelRuntime("default", "gpt-4", gamma.Id, true, 100*time.Millisecond)
	ObserveChannelRuntime("default", "gpt-4", gamma.Id, true, 200*time.Millisecond)

	health, err := GetChannelMonitorHealth(3)
	if err != nil {
		t.Fatalf("expected health without error, got %v", err)
	}
	if len(health) != 3 {
		t.Fatalf("expected 3 health points, got %d", len(health))
	}

	if health[0].Date != dayStart.AddDate(0, 0, -2).Format("2006-01-02") {
		t.Fatalf("unexpected first date %s", health[0].Date)
	}
	if health[0].TotalRequests != 10 || health[0].SuccessRate != 1 {
		t.Fatalf("unexpected first day aggregate: %+v", health[0])
	}
	if health[0].HealthyChannels != 1 || health[0].WarningChannels != 0 || health[0].ErrorChannels != 0 {
		t.Fatalf("unexpected first day status counts: %+v", health[0])
	}

	if health[1].Date != dayStart.AddDate(0, 0, -1).Format("2006-01-02") {
		t.Fatalf("unexpected second date %s", health[1].Date)
	}
	if health[1].TotalRequests != 20 {
		t.Fatalf("expected second day total requests 20, got %d", health[1].TotalRequests)
	}
	if health[1].SuccessRate != 0.5 {
		t.Fatalf("expected second day success rate 0.5, got %v", health[1].SuccessRate)
	}
	if health[1].AvgLatencyMs != 250 {
		t.Fatalf("expected second day avg latency 250, got %v", health[1].AvgLatencyMs)
	}
	if health[1].HealthyChannels != 0 || health[1].WarningChannels != 1 || health[1].ErrorChannels != 1 {
		t.Fatalf("unexpected second day status counts: %+v", health[1])
	}

	if health[2].Date != dayStart.Format("2006-01-02") {
		t.Fatalf("unexpected third date %s", health[2].Date)
	}
	if health[2].TotalRequests != 2 || health[2].SuccessRate != 1 || health[2].AvgLatencyMs != 150 {
		t.Fatalf("unexpected today aggregate: %+v", health[2])
	}
	if health[2].HealthyChannels != 1 || health[2].WarningChannels != 0 || health[2].ErrorChannels != 0 {
		t.Fatalf("unexpected today status counts: %+v", health[2])
	}
}

func TestGetChannelMonitorHealthReturnsEmptyBucketsWhenNoData(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	_ = seedChannelMonitorTestChannel(t, db, "alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)

	health, err := GetChannelMonitorHealth(2)
	if err != nil {
		t.Fatalf("expected empty health without error, got %v", err)
	}
	if len(health) != 2 {
		t.Fatalf("expected 2 health points, got %d", len(health))
	}
	for _, item := range health {
		if item.TotalRequests != 0 || item.SuccessRate != 0 || item.AvgLatencyMs != 0 {
			t.Fatalf("expected empty point metrics, got %+v", item)
		}
		if item.HealthyChannels != 0 || item.WarningChannels != 0 || item.ErrorChannels != 0 {
			t.Fatalf("expected empty point statuses, got %+v", item)
		}
	}
}

func TestGetChannelMonitorChannelPageSortsAndBuildsTrend(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	now := time.Now()
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -1).Add(9 * time.Hour)

	alpha := seedChannelMonitorTestChannel(t, db, "Alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	beta := seedChannelMonitorTestChannel(t, db, "Beta", constant.ChannelTypeAnthropic, common.ChannelStatusManuallyDisabled)
	gamma := seedChannelMonitorTestChannel(t, db, "Gamma", constant.ChannelTypeGemini, common.ChannelStatusEnabled)

	seedChannelMonitorStat(t, db, alpha.Id, yesterday, 10, 10, 0, 100, 120, yesterday)
	seedChannelMonitorStat(t, db, beta.Id, yesterday, 10, 5, 5, 400, 500, yesterday)

	ObserveChannelRuntime("default", "gpt-4", alpha.Id, true, 100*time.Millisecond)

	items, total, err := GetChannelMonitorChannelPage(2, 1, 2, "success_rate", "desc")
	if err != nil {
		t.Fatalf("expected channel page without error, got %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items on first page, got %d", len(items))
	}

	if items[0].Id != alpha.Id {
		t.Fatalf("expected alpha first, got channel id %d", items[0].Id)
	}
	if items[0].RequestCount != 11 || items[0].FailureCount != 0 || items[0].SuccessRate != 1 {
		t.Fatalf("unexpected alpha metrics: %+v", items[0])
	}
	if items[0].AvgLatencyMs != 100 {
		t.Fatalf("expected alpha avg latency 100, got %v", items[0].AvgLatencyMs)
	}
	if items[0].P95LatencyMs != 120 {
		t.Fatalf("expected alpha p95 latency 120, got %v", items[0].P95LatencyMs)
	}
	if len(items[0].HealthTrend) != 2 || items[0].HealthTrend[0] != 1 || items[0].HealthTrend[1] != 1 {
		t.Fatalf("unexpected alpha trend: %+v", items[0].HealthTrend)
	}

	if items[1].Id != beta.Id {
		t.Fatalf("expected beta second, got channel id %d", items[1].Id)
	}
	if items[1].RequestCount != 10 || items[1].FailureCount != 5 || items[1].SuccessRate != 0.5 {
		t.Fatalf("unexpected beta metrics: %+v", items[1])
	}
	if len(items[1].HealthTrend) != 2 || items[1].HealthTrend[0] != 0.5 || items[1].HealthTrend[1] != 0 {
		t.Fatalf("unexpected beta trend: %+v", items[1].HealthTrend)
	}

	pageTwoItems, pageTwoTotal, err := GetChannelMonitorChannelPage(2, 2, 2, "success_rate", "desc")
	if err != nil {
		t.Fatalf("expected second page without error, got %v", err)
	}
	if pageTwoTotal != 3 {
		t.Fatalf("expected second page total 3, got %d", pageTwoTotal)
	}
	if len(pageTwoItems) != 1 || pageTwoItems[0].Id != gamma.Id {
		t.Fatalf("expected gamma on second page, got %+v", pageTwoItems)
	}
	if pageTwoItems[0].RequestCount != 0 || pageTwoItems[0].FailureCount != 0 || pageTwoItems[0].SuccessRate != 0 {
		t.Fatalf("unexpected gamma metrics: %+v", pageTwoItems[0])
	}
	if len(pageTwoItems[0].HealthTrend) != 2 || pageTwoItems[0].HealthTrend[0] != 0 || pageTwoItems[0].HealthTrend[1] != 0 {
		t.Fatalf("unexpected gamma trend: %+v", pageTwoItems[0].HealthTrend)
	}
}

func TestGetChannelMonitorChannelPageReturnsEmptyMetricsWhenNoData(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	alpha := seedChannelMonitorTestChannel(t, db, "Alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)

	items, total, err := GetChannelMonitorChannelPage(2, 1, 20, "request_count", "desc")
	if err != nil {
		t.Fatalf("expected empty page without error, got %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Id != alpha.Id {
		t.Fatalf("expected alpha item, got %+v", items[0])
	}
	if items[0].RequestCount != 0 || items[0].FailureCount != 0 || items[0].SuccessRate != 0 || items[0].AvgLatencyMs != 0 || items[0].P95LatencyMs != 0 {
		t.Fatalf("expected zero metrics, got %+v", items[0])
	}
	if len(items[0].HealthTrend) != 2 || items[0].HealthTrend[0] != 0 || items[0].HealthTrend[1] != 0 {
		t.Fatalf("expected zero trend, got %+v", items[0].HealthTrend)
	}
}

func TestGetChannelMonitorSummaryAndPageMatchObservedRuntimeRequests(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	channel := seedChannelMonitorTestChannel(t, db, "runtime-only", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)

	ObserveChannelRuntime("default", "gpt-4", channel.Id, true, 100*time.Millisecond)
	ObserveChannelRuntime("default", "gpt-4", channel.Id, false, 300*time.Millisecond)
	ObserveChannelRuntime("default", "gpt-4", channel.Id, true, 500*time.Millisecond)

	summary, err := GetChannelMonitorSummary(1)
	if err != nil {
		t.Fatalf("expected runtime-only summary without error, got %v", err)
	}
	if summary.TotalRequests != 3 {
		t.Fatalf("expected total requests 3, got %d", summary.TotalRequests)
	}
	if summary.SuccessRate != float64(2)/float64(3) {
		t.Fatalf("expected success rate 2/3, got %v", summary.SuccessRate)
	}
	if summary.AvgLatencyMs != 300 {
		t.Fatalf("expected avg latency 300, got %v", summary.AvgLatencyMs)
	}
	if summary.ActiveChannels != 1 || summary.TotalChannels != 1 {
		t.Fatalf("unexpected channel counts in summary: %+v", summary)
	}
	if summary.TimeRange != "last_24h" {
		t.Fatalf("expected time range last_24h, got %s", summary.TimeRange)
	}

	items, total, err := GetChannelMonitorChannelPage(1, 1, 20, "request_count", "desc")
	if err != nil {
		t.Fatalf("expected runtime-only page without error, got %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 page item, got %d", len(items))
	}
	if items[0].Id != channel.Id {
		t.Fatalf("expected runtime-only channel item, got %+v", items[0])
	}
	if items[0].RequestCount != 3 || items[0].FailureCount != 1 {
		t.Fatalf("unexpected request/failure counts: %+v", items[0])
	}
	if items[0].SuccessRate != float64(2)/float64(3) {
		t.Fatalf("expected page success rate 2/3, got %v", items[0].SuccessRate)
	}
	if items[0].AvgLatencyMs != 300 {
		t.Fatalf("expected page avg latency 300, got %v", items[0].AvgLatencyMs)
	}
	if items[0].P95LatencyMs < 300 {
		t.Fatalf("expected page p95 latency to reflect runtime samples, got %v", items[0].P95LatencyMs)
	}
	if len(items[0].HealthTrend) != 1 || items[0].HealthTrend[0] != float64(2)/float64(3) {
		t.Fatalf("unexpected health trend: %+v", items[0].HealthTrend)
	}
}

func TestPersistChannelMonitorRuntimeStatsBuildsHourAndDayAggregates(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	channel := seedChannelMonitorTestChannel(t, db, "alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	hourStart := time.Now().Truncate(time.Hour).Add(-2 * time.Hour)
	bucket1 := hourStart.Add(10 * time.Minute)
	bucket2 := hourStart.Add(40 * time.Minute)

	seedChannelMonitorStat(t, db, channel.Id, bucket1, 10, 8, 2, 100, 120, bucket1)
	seedChannelMonitorStat(t, db, channel.Id, bucket2, 20, 18, 2, 200, 240, bucket2)

	if err := PersistChannelMonitorRuntimeStats(); err != nil {
		t.Fatalf("expected aggregate refresh without error, got %v", err)
	}

	var hourStat ChannelMonitorStat
	if err := db.Where("channel_id = ? AND granularity = ? AND time_bucket = ?", channel.Id, ChannelMonitorGranularityHour, hourStart).First(&hourStat).Error; err != nil {
		t.Fatalf("expected hour aggregate, got %v", err)
	}
	if hourStat.RequestCount != 30 || hourStat.SuccessCount != 26 || hourStat.FailureCount != 4 {
		t.Fatalf("unexpected hour aggregate counts: %+v", hourStat)
	}
	if hourStat.AvgLatencyMs != (float64(10*100)+float64(20*200))/30 {
		t.Fatalf("unexpected hour avg latency: %+v", hourStat)
	}
	if hourStat.P95LatencyMs != 240 {
		t.Fatalf("unexpected hour p95 latency: %+v", hourStat)
	}

	var dayStat ChannelMonitorStat
	dayBucket := time.Date(bucket1.Year(), bucket1.Month(), bucket1.Day(), 0, 0, 0, 0, bucket1.Location())
	if err := db.Where("channel_id = ? AND granularity = ? AND time_bucket = ?", channel.Id, ChannelMonitorGranularityDay, dayBucket).First(&dayStat).Error; err != nil {
		t.Fatalf("expected day aggregate, got %v", err)
	}
	if dayStat.RequestCount != 30 || dayStat.SuccessCount != 26 || dayStat.FailureCount != 4 {
		t.Fatalf("unexpected day aggregate counts: %+v", dayStat)
	}
}

func TestCleanupExpiredChannelMonitorStatsUsesTieredRetention(t *testing.T) {
	db := setupChannelMonitorTestDB(t)
	channel := seedChannelMonitorTestChannel(t, db, "alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	now := time.Now()

	seedChannelMonitorStat(t, db, channel.Id, now.Add(-25*time.Hour), 1, 1, 0, 100, 100, now.Add(-25*time.Hour))
	seedChannelMonitorStat(t, db, channel.Id, now.Add(-6*24*time.Hour), 1, 1, 0, 100, 100, now.Add(-6*24*time.Hour))
	seedChannelMonitorStat(t, db, channel.Id, now.Add(-8*24*time.Hour), 1, 1, 0, 100, 100, now.Add(-8*24*time.Hour))
	seedChannelMonitorStat(t, db, channel.Id, now.Add(-29*24*time.Hour), 1, 1, 0, 100, 100, now.Add(-29*24*time.Hour))
	seedChannelMonitorStat(t, db, channel.Id, now.Add(-31*24*time.Hour), 1, 1, 0, 100, 100, now.Add(-31*24*time.Hour))

	if err := db.Model(&ChannelMonitorStat{}).Where("time_bucket = ?", now.Add(-6*24*time.Hour)).Update("granularity", ChannelMonitorGranularityHour).Error; err != nil {
		t.Fatalf("failed to update hour stat granularity: %v", err)
	}
	if err := db.Model(&ChannelMonitorStat{}).Where("time_bucket = ?", now.Add(-8*24*time.Hour)).Update("granularity", ChannelMonitorGranularityHour).Error; err != nil {
		t.Fatalf("failed to update old hour stat granularity: %v", err)
	}
	if err := db.Model(&ChannelMonitorStat{}).Where("time_bucket = ?", now.Add(-29*24*time.Hour)).Update("granularity", ChannelMonitorGranularityDay).Error; err != nil {
		t.Fatalf("failed to update day stat granularity: %v", err)
	}
	if err := db.Model(&ChannelMonitorStat{}).Where("time_bucket = ?", now.Add(-31*24*time.Hour)).Update("granularity", ChannelMonitorGranularityDay).Error; err != nil {
		t.Fatalf("failed to update old day stat granularity: %v", err)
	}

	if err := CleanupExpiredChannelMonitorStats(); err != nil {
		t.Fatalf("expected cleanup without error, got %v", err)
	}

	var minuteCount int64
	if err := db.Model(&ChannelMonitorStat{}).Where("granularity = ?", ChannelMonitorGranularityMinute).Count(&minuteCount).Error; err != nil {
		t.Fatalf("failed to count minute stats: %v", err)
	}
	if minuteCount != 0 {
		t.Fatalf("expected expired minute stats to be deleted, got %d", minuteCount)
	}

	var hourCount int64
	if err := db.Model(&ChannelMonitorStat{}).Where("granularity = ?", ChannelMonitorGranularityHour).Count(&hourCount).Error; err != nil {
		t.Fatalf("failed to count hour stats: %v", err)
	}
	if hourCount != 1 {
		t.Fatalf("expected only one hour stat kept, got %d", hourCount)
	}

	var dayCount int64
	if err := db.Model(&ChannelMonitorStat{}).Where("granularity = ?", ChannelMonitorGranularityDay).Count(&dayCount).Error; err != nil {
		t.Fatalf("failed to count day stats: %v", err)
	}
	if dayCount != 1 {
		t.Fatalf("expected only one day stat kept, got %d", dayCount)
	}
}

func TestComputeChannelMonitorRuntimeDeltasCalculatesCorrectly(t *testing.T) {
	resetChannelMonitorTestState()
	t.Cleanup(resetChannelMonitorTestState)

	group := "test-group"
	modelName := "gpt-4o"
	channelID := 12345

	stats := &ChannelRuntimeStats{}
	stats.RequestCount.Store(10)
	stats.SuccessCount.Store(8)
	stats.FailureCount.Store(2)
	stats.TotalLatencyMicros.Store(50000)
	stats.LastActiveUnix.Store(time.Now().Unix())

	key := buildChannelRuntimeStatsKey(group, modelName, channelID)
	channelRuntimeStatsStore.Store(key, stats)

	deltas := computeChannelMonitorRuntimeDeltas()

	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}
	if deltas[0].ChannelID != channelID {
		t.Fatalf("expected channel ID %d, got %d", channelID, deltas[0].ChannelID)
	}
	if deltas[0].GroupName != group {
		t.Fatalf("expected group %s, got %s", group, deltas[0].GroupName)
	}
	if deltas[0].ModelName != modelName {
		t.Fatalf("expected model %s, got %s", modelName, deltas[0].ModelName)
	}
	if deltas[0].RequestCount != 10 {
		t.Fatalf("expected request count 10, got %d", deltas[0].RequestCount)
	}
	if deltas[0].SuccessCount != 8 {
		t.Fatalf("expected success count 8, got %d", deltas[0].SuccessCount)
	}
	if deltas[0].FailureCount != 2 {
		t.Fatalf("expected failure count 2, got %d", deltas[0].FailureCount)
	}
	if deltas[0].AvgLatencyMs < 4.9 || deltas[0].AvgLatencyMs > 5.1 {
		t.Fatalf("expected avg latency ~5.0ms, got %.2f", deltas[0].AvgLatencyMs)
	}
}

func TestComputeChannelMonitorRuntimeDeltasSkipsZeroRequestDelta(t *testing.T) {
	resetChannelMonitorTestState()
	t.Cleanup(resetChannelMonitorTestState)

	group := "test-group"
	modelName := "gpt-4o"
	channelID := 12346

	stats := &ChannelRuntimeStats{}
	stats.RequestCount.Store(10)
	stats.SuccessCount.Store(8)
	stats.FailureCount.Store(2)
	stats.TotalLatencyMicros.Store(50000)
	stats.LastActiveUnix.Store(time.Now().Unix())

	key := buildChannelRuntimeStatsKey(group, modelName, channelID)
	channelRuntimeStatsStore.Store(key, stats)

	snapshot := channelMonitorPersistedSnapshot{
		RequestCount:       10,
		SuccessCount:       8,
		FailureCount:       2,
		TotalLatencyMicros: 50000,
		LastActiveUnix:     time.Now().Unix(),
	}
	channelMonitorPersistStateStore.Store(key, snapshot)

	deltas := computeChannelMonitorRuntimeDeltas()

	if len(deltas) != 0 {
		t.Fatalf("expected 0 deltas, got %d", len(deltas))
	}
}

func TestWeightedLatencyCalculatesCorrectly(t *testing.T) {
	result := weightedLatency(10.0, 5, 20.0, 5)
	if result < 14.9 || result > 15.1 {
		t.Fatalf("expected ~15.0, got %.2f", result)
	}

	result = weightedLatency(10.0, 8, 20.0, 2)
	if result < 11.9 || result > 12.1 {
		t.Fatalf("expected ~12.0, got %.2f", result)
	}

	result = weightedLatency(0, 0, 20.0, 5)
	if result != 20.0 {
		t.Fatalf("expected 20.0, got %.2f", result)
	}
}
