package model

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ChannelMonitorGranularityMinute = 1
	ChannelMonitorGranularityHour   = 2
	ChannelMonitorGranularityDay    = 3
)

const (
	channelMonitorMinuteRetention = 24 * time.Hour
	channelMonitorHourRetention   = 7 * 24 * time.Hour
	channelMonitorDayRetention    = 30 * 24 * time.Hour
)

type ChannelMonitorStat struct {
	Id           int       `json:"id"`
	ChannelID    int       `json:"channel_id" gorm:"index:idx_channel_monitor_bucket,priority:1;index:idx_channel_monitor_range,priority:2"`
	GroupName    string    `json:"group_name" gorm:"size:64;index:idx_channel_monitor_bucket,priority:2"`
	ModelName    string    `json:"model_name" gorm:"size:128;index:idx_channel_monitor_bucket,priority:3"`
	TimeBucket   time.Time `json:"time_bucket" gorm:"index:idx_channel_monitor_bucket,priority:4;index:idx_channel_monitor_range,priority:1"`
	Granularity  int       `json:"granularity" gorm:"index:idx_channel_monitor_bucket,priority:5;index:idx_channel_monitor_range,priority:3"`
	RequestCount int64     `json:"request_count" gorm:"default:0"`
	SuccessCount int64     `json:"success_count" gorm:"default:0"`
	FailureCount int64     `json:"failure_count" gorm:"default:0"`
	AvgLatencyMs float64   `json:"avg_latency_ms" gorm:"default:0"`
	P95LatencyMs float64   `json:"p95_latency_ms" gorm:"default:0"`
	LastActiveAt time.Time `json:"last_active_at" gorm:"index:idx_channel_monitor_range,priority:4"`
	CreatedAt    int64     `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64     `json:"updated_at" gorm:"bigint"`
}

func (ChannelMonitorStat) TableName() string {
	return "channel_monitor_stats"
}

type channelMonitorPersistedSnapshot struct {
	RequestCount       int64
	SuccessCount       int64
	FailureCount       int64
	TotalLatencyMicros int64
	LastActiveUnix     int64
}

type channelMonitorRuntimeDelta struct {
	RuntimeKey         string
	GroupName          string
	ModelName          string
	ChannelID          int
	RequestCount       int64
	SuccessCount       int64
	FailureCount       int64
	TotalLatencyMicros int64
	AvgLatencyMs       float64
	P95LatencyMs       float64
	LastActiveAt       time.Time
	Snapshot           channelMonitorPersistedSnapshot
}

type ChannelMonitorSummary struct {
	TotalRequests  int64   `json:"total_requests"`
	SuccessRate    float64 `json:"success_rate"`
	AvgLatencyMs   float64 `json:"avg_latency"`
	ActiveChannels int64   `json:"active_channels"`
	TotalChannels  int64   `json:"total_channels"`
	TimeRange      string  `json:"time_range"`
}

type ChannelMonitorHealthPoint struct {
	Date            string  `json:"date"`
	TotalRequests   int64   `json:"total_requests"`
	SuccessRate     float64 `json:"success_rate"`
	AvgLatencyMs    float64 `json:"avg_latency"`
	HealthyChannels int     `json:"healthy_channels"`
	WarningChannels int     `json:"warning_channels"`
	ErrorChannels   int     `json:"error_channels"`
}

type ChannelMonitorChannelItem struct {
	Id                     int       `json:"id"`
	Name                   string    `json:"name"`
	GroupName              string    `json:"group_name"`
	Type                   int       `json:"type"`
	Status                 int       `json:"status"`
	SuccessRate            float64   `json:"success_rate"`
	AvgLatencyMs           float64   `json:"avg_latency"`
	P95LatencyMs           float64   `json:"p95_latency"`
	RequestCount           int64     `json:"request_count"`
	FailureCount           int64     `json:"failure_count"`
	LastActiveAt           time.Time `json:"last_active"`
	HealthTrend            []float64 `json:"health_trend"`
	TemporaryCircuitOpen   bool      `json:"temporary_circuit_open"`
	TemporaryCircuitUntil  time.Time `json:"temporary_circuit_until"`
	TemporaryCircuitReason string    `json:"temporary_circuit_reason"`
	CurrentWeightedScore   float64   `json:"current_weighted_score"`
}

type ChannelTimelinePoint struct {
	ChannelID     int       `json:"channel_id"`
	ChannelName   string    `json:"channel_name"`
	ChannelType   int       `json:"channel_type"`
	ChannelStatus int       `json:"channel_status"`
	TimeBucket    time.Time `json:"time_bucket"`
	RequestCount  int64     `json:"request_count"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
}

type ChannelTimelineChannel struct {
	ChannelID     int                    `json:"channel_id"`
	ChannelName   string                 `json:"channel_name"`
	ChannelType   int                    `json:"channel_type"`
	ChannelStatus int                    `json:"channel_status"`
	Points        []ChannelTimelinePoint `json:"points"`
	RequestCount  int64                  `json:"request_count"`
	SuccessRate   float64                `json:"success_rate"`
}

type channelTimelineRow struct {
	ChannelID     int       `json:"channel_id"`
	ChannelName   string    `json:"channel_name"`
	ChannelType   int       `json:"channel_type"`
	ChannelStatus int       `json:"channel_status"`
	TimeBucket    time.Time `json:"time_bucket"`
	RequestCount  int64     `json:"request_count"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
}
type channelMonitorAggregate struct {
	RequestCount int64
	SuccessCount int64
	FailureCount int64
	LatencySumMs float64
	P95LatencyMs float64
	LastActiveAt time.Time
}

var channelMonitorPersistStateStore sync.Map

func parseChannelRuntimeStatsKey(key string) (string, string, int, bool) {
	lastSep := strings.LastIndex(key, ":")
	if lastSep <= 0 || lastSep >= len(key)-1 {
		return "", "", 0, false
	}
	channelID, err := strconv.Atoi(key[lastSep+1:])
	if err != nil {
		return "", "", 0, false
	}
	prefix := key[:lastSep]
	firstSep := strings.Index(prefix, ":")
	if firstSep < 0 {
		return "", "", 0, false
	}
	return prefix[:firstSep], prefix[firstSep+1:], channelID, true
}

func computeChannelMonitorRuntimeDeltas() []channelMonitorRuntimeDelta {
	deltas := make([]channelMonitorRuntimeDelta, 0)
	channelRuntimeStatsStore.Range(func(key, value any) bool {
		runtimeKey, ok := key.(string)
		if !ok {
			return true
		}
		groupName, modelName, channelID, ok := parseChannelRuntimeStatsKey(runtimeKey)
		if !ok || channelID <= 0 || strings.TrimSpace(modelName) == "" {
			return true
		}
		stats, ok := value.(*ChannelRuntimeStats)
		if !ok || stats == nil {
			return true
		}
		current := channelMonitorPersistedSnapshot{
			RequestCount:       stats.RequestCount.Load(),
			SuccessCount:       stats.SuccessCount.Load(),
			FailureCount:       stats.FailureCount.Load(),
			TotalLatencyMicros: stats.TotalLatencyMicros.Load(),
			LastActiveUnix:     stats.LastActiveUnix.Load(),
		}
		prev := channelMonitorPersistedSnapshot{}
		if prevAny, ok := channelMonitorPersistStateStore.Load(runtimeKey); ok {
			prev, _ = prevAny.(channelMonitorPersistedSnapshot)
		}
		deltaRequestCount := current.RequestCount - prev.RequestCount
		if deltaRequestCount <= 0 {
			return true
		}
		deltaSuccessCount := current.SuccessCount - prev.SuccessCount
		deltaFailureCount := current.FailureCount - prev.FailureCount
		deltaLatencyMicros := current.TotalLatencyMicros - prev.TotalLatencyMicros
		if deltaLatencyMicros < 0 {
			deltaLatencyMicros = 0
		}
		avgLatencyMs := 0.0
		if deltaRequestCount > 0 {
			avgLatencyMs = float64(deltaLatencyMicros) / float64(deltaRequestCount) / 1000
		}
		latencySnapshot := GetChannelLatencySnapshot(channelID)
		if avgLatencyMs <= 0 {
			avgLatencyMs = latencySnapshot.AvgLatencyMs
		}
		lastActiveAt := time.Time{}
		if current.LastActiveUnix > 0 {
			lastActiveAt = time.Unix(current.LastActiveUnix, 0)
		}
		deltas = append(deltas, channelMonitorRuntimeDelta{
			RuntimeKey:         runtimeKey,
			GroupName:          groupName,
			ModelName:          modelName,
			ChannelID:          channelID,
			RequestCount:       deltaRequestCount,
			SuccessCount:       deltaSuccessCount,
			FailureCount:       deltaFailureCount,
			TotalLatencyMicros: deltaLatencyMicros,
			AvgLatencyMs:       avgLatencyMs,
			P95LatencyMs:       latencySnapshot.P95LatencyMs,
			LastActiveAt:       lastActiveAt,
			Snapshot:           current,
		})
		return true
	})
	sort.Slice(deltas, func(i, j int) bool {
		if deltas[i].ChannelID != deltas[j].ChannelID {
			return deltas[i].ChannelID < deltas[j].ChannelID
		}
		return deltas[i].RuntimeKey < deltas[j].RuntimeKey
	})
	return deltas
}

func weightedLatency(currentAvg float64, currentCount int64, deltaAvg float64, deltaCount int64) float64 {
	totalCount := currentCount + deltaCount
	if totalCount <= 0 {
		return 0
	}
	return (currentAvg*float64(currentCount) + deltaAvg*float64(deltaCount)) / float64(totalCount)
}

func persistChannelMonitorDelta(bucket time.Time, delta channelMonitorRuntimeDelta) error {
	if DB == nil {
		return nil
	}
	var stat ChannelMonitorStat
	err := DB.Where(
		"channel_id = ? AND group_name = ? AND model_name = ? AND time_bucket = ? AND granularity = ?",
		delta.ChannelID,
		delta.GroupName,
		delta.ModelName,
		bucket,
		ChannelMonitorGranularityMinute,
	).First(&stat).Error
	nowUnix := time.Now().Unix()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		stat = ChannelMonitorStat{
			ChannelID:    delta.ChannelID,
			GroupName:    delta.GroupName,
			ModelName:    delta.ModelName,
			TimeBucket:   bucket,
			Granularity:  ChannelMonitorGranularityMinute,
			RequestCount: delta.RequestCount,
			SuccessCount: delta.SuccessCount,
			FailureCount: delta.FailureCount,
			AvgLatencyMs: delta.AvgLatencyMs,
			P95LatencyMs: delta.P95LatencyMs,
			LastActiveAt: delta.LastActiveAt,
			CreatedAt:    nowUnix,
			UpdatedAt:    nowUnix,
		}
		return DB.Create(&stat).Error
	}
	if err != nil {
		return err
	}
	stat.AvgLatencyMs = weightedLatency(stat.AvgLatencyMs, stat.RequestCount, delta.AvgLatencyMs, delta.RequestCount)
	if delta.P95LatencyMs > stat.P95LatencyMs {
		stat.P95LatencyMs = delta.P95LatencyMs
	}
	stat.RequestCount += delta.RequestCount
	stat.SuccessCount += delta.SuccessCount
	stat.FailureCount += delta.FailureCount
	if delta.LastActiveAt.After(stat.LastActiveAt) {
		stat.LastActiveAt = delta.LastActiveAt
	}
	stat.UpdatedAt = nowUnix
	return DB.Save(&stat).Error
}

func aggregateChannelMonitorStats(sourceGranularity int, targetGranularity int, bucketStart func(time.Time) time.Time, from time.Time) error {
	if DB == nil {
		return nil
	}
	stats := make([]ChannelMonitorStat, 0)
	if err := DB.Where("granularity = ? AND time_bucket >= ?", sourceGranularity, from).Find(&stats).Error; err != nil {
		return err
	}
	if len(stats) == 0 {
		return nil
	}
	type aggregateKey struct {
		ChannelID   int
		GroupName   string
		ModelName   string
		TimeBucket  time.Time
		Granularity int
	}
	aggregates := make(map[aggregateKey]*ChannelMonitorStat)
	for _, stat := range stats {
		bucket := bucketStart(stat.TimeBucket)
		key := aggregateKey{
			ChannelID:   stat.ChannelID,
			GroupName:   stat.GroupName,
			ModelName:   stat.ModelName,
			TimeBucket:  bucket,
			Granularity: targetGranularity,
		}
		agg := aggregates[key]
		if agg == nil {
			agg = &ChannelMonitorStat{
				ChannelID:   stat.ChannelID,
				GroupName:   stat.GroupName,
				ModelName:   stat.ModelName,
				TimeBucket:  bucket,
				Granularity: targetGranularity,
				CreatedAt:   time.Now().Unix(),
				UpdatedAt:   time.Now().Unix(),
			}
			aggregates[key] = agg
		}
		agg.AvgLatencyMs = weightedLatency(agg.AvgLatencyMs, agg.RequestCount, stat.AvgLatencyMs, stat.RequestCount)
		if stat.P95LatencyMs > agg.P95LatencyMs {
			agg.P95LatencyMs = stat.P95LatencyMs
		}
		agg.RequestCount += stat.RequestCount
		agg.SuccessCount += stat.SuccessCount
		agg.FailureCount += stat.FailureCount
		if stat.LastActiveAt.After(agg.LastActiveAt) {
			agg.LastActiveAt = stat.LastActiveAt
		}
		agg.UpdatedAt = time.Now().Unix()
	}
	for _, agg := range aggregates {
		var existing ChannelMonitorStat
		err := DB.Where(
			"channel_id = ? AND group_name = ? AND model_name = ? AND time_bucket = ? AND granularity = ?",
			agg.ChannelID,
			agg.GroupName,
			agg.ModelName,
			agg.TimeBucket,
			agg.Granularity,
		).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err = DB.Create(agg).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		existing.RequestCount = agg.RequestCount
		existing.SuccessCount = agg.SuccessCount
		existing.FailureCount = agg.FailureCount
		existing.AvgLatencyMs = agg.AvgLatencyMs
		existing.P95LatencyMs = agg.P95LatencyMs
		existing.LastActiveAt = agg.LastActiveAt
		existing.UpdatedAt = agg.UpdatedAt
		if err = DB.Save(&existing).Error; err != nil {
			return err
		}
	}
	return nil
}

func refreshChannelMonitorAggregates(now time.Time) error {
	if err := aggregateChannelMonitorStats(
		ChannelMonitorGranularityMinute,
		ChannelMonitorGranularityHour,
		func(t time.Time) time.Time { return t.Truncate(time.Hour) },
		now.Add(-channelMonitorHourRetention),
	); err != nil {
		return err
	}
	return aggregateChannelMonitorStats(
		ChannelMonitorGranularityHour,
		ChannelMonitorGranularityDay,
		func(t time.Time) time.Time {
			y, m, d := t.Date()
			return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
		},
		now.Add(-channelMonitorDayRetention),
	)
}

func PersistChannelMonitorRuntimeStats() error {
	deltas := computeChannelMonitorRuntimeDeltas()
	if len(deltas) == 0 {
		return refreshChannelMonitorAggregates(time.Now())
	}
	bucket := time.Now().Truncate(time.Minute)
	for _, delta := range deltas {
		if err := persistChannelMonitorDelta(bucket, delta); err != nil {
			return err
		}
		channelMonitorPersistStateStore.Store(delta.RuntimeKey, delta.Snapshot)
	}
	return refreshChannelMonitorAggregates(time.Now())
}

func StartChannelMonitorPersistenceTask() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			_ = PersistChannelMonitorRuntimeStats()
		}
	}()
}

func CleanupExpiredChannelMonitorStats() error {
	if DB == nil {
		return nil
	}
	now := time.Now()
	if err := DB.Where("granularity = ? AND time_bucket < ?", ChannelMonitorGranularityMinute, now.Add(-channelMonitorMinuteRetention)).Delete(&ChannelMonitorStat{}).Error; err != nil {
		return err
	}
	if err := DB.Where("granularity = ? AND time_bucket < ?", ChannelMonitorGranularityHour, now.Add(-channelMonitorHourRetention)).Delete(&ChannelMonitorStat{}).Error; err != nil {
		return err
	}
	return DB.Where("granularity = ? AND time_bucket < ?", ChannelMonitorGranularityDay, now.Add(-channelMonitorDayRetention)).Delete(&ChannelMonitorStat{}).Error
}

func StartChannelMonitorCleanupTask() {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			_ = CleanupExpiredChannelMonitorStats()
		}
	}()
}

func normalizeChannelMonitorDays(days int) int {
	if days <= 0 {
		return 7
	}
	if days > 30 {
		return 30
	}
	return days
}

func loadChannelMonitorStatsSince(start time.Time) ([]ChannelMonitorStat, error) {
	if DB == nil {
		return nil, nil
	}
	stats := make([]ChannelMonitorStat, 0)
	err := DB.Where("time_bucket >= ?", start).
		Order("time_bucket ASC").
		Order("granularity DESC").
		Find(&stats).Error
	if err != nil {
		return nil, err
	}
	if len(stats) == 0 {
		return stats, nil
	}
	type coverageKey struct {
		ChannelID  int
		GroupName  string
		ModelName  string
		TimeBucket time.Time
	}
	dayStart := func(t time.Time) time.Time {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	}
	dayCoverage := make(map[coverageKey]struct{})
	hourCoverage := make(map[coverageKey]struct{})
	for _, stat := range stats {
		switch stat.Granularity {
		case ChannelMonitorGranularityDay:
			dayCoverage[coverageKey{
				ChannelID:  stat.ChannelID,
				GroupName:  stat.GroupName,
				ModelName:  stat.ModelName,
				TimeBucket: dayStart(stat.TimeBucket),
			}] = struct{}{}
		case ChannelMonitorGranularityHour:
			hourCoverage[coverageKey{
				ChannelID:  stat.ChannelID,
				GroupName:  stat.GroupName,
				ModelName:  stat.ModelName,
				TimeBucket: stat.TimeBucket.Truncate(time.Hour),
			}] = struct{}{}
		}
	}
	filtered := make([]ChannelMonitorStat, 0, len(stats))
	for _, stat := range stats {
		dayKey := coverageKey{
			ChannelID:  stat.ChannelID,
			GroupName:  stat.GroupName,
			ModelName:  stat.ModelName,
			TimeBucket: dayStart(stat.TimeBucket),
		}
		hourKey := coverageKey{
			ChannelID:  stat.ChannelID,
			GroupName:  stat.GroupName,
			ModelName:  stat.ModelName,
			TimeBucket: stat.TimeBucket.Truncate(time.Hour),
		}
		switch stat.Granularity {
		case ChannelMonitorGranularityMinute:
			if _, ok := dayCoverage[dayKey]; ok {
				continue
			}
			if _, ok := hourCoverage[hourKey]; ok {
				continue
			}
		case ChannelMonitorGranularityHour:
			if _, ok := dayCoverage[dayKey]; ok {
				continue
			}
		}
		filtered = append(filtered, stat)
	}
	return filtered, nil
}

func loadChannelMonitorDataSince(start time.Time) ([]ChannelMonitorStat, []channelMonitorRuntimeDelta, error) {
	stats, err := loadChannelMonitorStatsSince(start)
	if err != nil {
		return nil, nil, err
	}
	return stats, computeChannelMonitorRuntimeDeltas(), nil
}

func applyChannelMonitorAggregate(agg *channelMonitorAggregate, requestCount int64, successCount int64, failureCount int64, avgLatencyMs float64, p95LatencyMs float64, lastActiveAt time.Time) {
	if agg == nil || requestCount <= 0 {
		return
	}
	agg.RequestCount += requestCount
	agg.SuccessCount += successCount
	agg.FailureCount += failureCount
	agg.LatencySumMs += avgLatencyMs * float64(requestCount)
	if p95LatencyMs > agg.P95LatencyMs {
		agg.P95LatencyMs = p95LatencyMs
	}
	if lastActiveAt.After(agg.LastActiveAt) {
		agg.LastActiveAt = lastActiveAt
	}
}

func channelMonitorTimeRangeLabel(days int) string {
	if days <= 1 {
		return "last_24h"
	}
	return "last_" + strconv.Itoa(days) + "d"
}

func GetChannelMonitorSummary(days int) (ChannelMonitorSummary, error) {
	days = normalizeChannelMonitorDays(days)
	now := time.Now()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)
	stats, pending, err := loadChannelMonitorDataSince(start)
	if err != nil {
		return ChannelMonitorSummary{}, err
	}
	activeSince := now.Add(-5 * time.Minute)
	activeChannels := make(map[int]struct{})
	var requestCount int64
	var successCount int64
	var latencySumMs float64
	for _, stat := range stats {
		if stat.RequestCount <= 0 {
			continue
		}
		requestCount += stat.RequestCount
		successCount += stat.SuccessCount
		latencySumMs += stat.AvgLatencyMs * float64(stat.RequestCount)
		if stat.LastActiveAt.After(activeSince) {
			activeChannels[stat.ChannelID] = struct{}{}
		}
	}
	for _, delta := range pending {
		requestCount += delta.RequestCount
		successCount += delta.SuccessCount
		latencySumMs += delta.AvgLatencyMs * float64(delta.RequestCount)
		if delta.LastActiveAt.After(activeSince) {
			activeChannels[delta.ChannelID] = struct{}{}
		}
	}
	totalChannels, err := CountAllChannels()
	if err != nil {
		return ChannelMonitorSummary{}, err
	}
	successRate := 0.0
	avgLatencyMs := 0.0
	if requestCount > 0 {
		successRate = float64(successCount) / float64(requestCount)
		avgLatencyMs = latencySumMs / float64(requestCount)
	}
	return ChannelMonitorSummary{
		TotalRequests:  requestCount,
		SuccessRate:    successRate,
		AvgLatencyMs:   avgLatencyMs,
		ActiveChannels: int64(len(activeChannels)),
		TotalChannels:  totalChannels,
		TimeRange:      channelMonitorTimeRangeLabel(days),
	}, nil
}

func GetChannelMonitorHealth(days int) ([]ChannelMonitorHealthPoint, error) {
	days = normalizeChannelMonitorDays(days)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))
	stats, pending, err := loadChannelMonitorDataSince(start)
	if err != nil {
		return nil, err
	}
	type dayAggregate struct {
		RequestCount int64
		SuccessCount int64
		LatencySumMs float64
		Channels     map[int]*channelMonitorAggregate
	}
	dayMap := make(map[string]*dayAggregate, days)
	getDayAggregate := func(date string) *dayAggregate {
		agg, ok := dayMap[date]
		if ok {
			return agg
		}
		agg = &dayAggregate{Channels: make(map[int]*channelMonitorAggregate)}
		dayMap[date] = agg
		return agg
	}
	applyRow := func(date string, channelID int, requestCount int64, successCount int64, failureCount int64, avgLatencyMs float64, p95LatencyMs float64, lastActiveAt time.Time) {
		if requestCount <= 0 {
			return
		}
		dayAgg := getDayAggregate(date)
		dayAgg.RequestCount += requestCount
		dayAgg.SuccessCount += successCount
		dayAgg.LatencySumMs += avgLatencyMs * float64(requestCount)
		channelAgg := dayAgg.Channels[channelID]
		if channelAgg == nil {
			channelAgg = &channelMonitorAggregate{}
			dayAgg.Channels[channelID] = channelAgg
		}
		applyChannelMonitorAggregate(channelAgg, requestCount, successCount, failureCount, avgLatencyMs, p95LatencyMs, lastActiveAt)
	}
	for _, stat := range stats {
		applyRow(stat.TimeBucket.Format("2006-01-02"), stat.ChannelID, stat.RequestCount, stat.SuccessCount, stat.FailureCount, stat.AvgLatencyMs, stat.P95LatencyMs, stat.LastActiveAt)
	}
	for _, delta := range pending {
		applyRow(now.Format("2006-01-02"), delta.ChannelID, delta.RequestCount, delta.SuccessCount, delta.FailureCount, delta.AvgLatencyMs, delta.P95LatencyMs, delta.LastActiveAt)
	}
	items := make([]ChannelMonitorHealthPoint, 0, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		date := day.Format("2006-01-02")
		dayAgg := getDayAggregate(date)
		item := ChannelMonitorHealthPoint{Date: date, TotalRequests: dayAgg.RequestCount}
		if dayAgg.RequestCount > 0 {
			item.SuccessRate = float64(dayAgg.SuccessCount) / float64(dayAgg.RequestCount)
			item.AvgLatencyMs = dayAgg.LatencySumMs / float64(dayAgg.RequestCount)
		}
		for _, channelAgg := range dayAgg.Channels {
			if channelAgg.RequestCount <= 0 {
				continue
			}
			channelSuccessRate := float64(channelAgg.SuccessCount) / float64(channelAgg.RequestCount)
			switch {
			case channelSuccessRate >= 0.9:
				item.HealthyChannels++
			case channelSuccessRate >= 0.5:
				item.WarningChannels++
			default:
				item.ErrorChannels++
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func normalizeChannelMonitorTimelineHours(hours int) int {
	if hours <= 0 {
		return 24
	}
	if hours > 24 {
		return 24
	}
	return hours
}

func normalizeChannelMonitorTimelineBucketMinutes(bucketMinutes int) int {
	if bucketMinutes <= 0 {
		return 10
	}
	if bucketMinutes < 1 {
		return 1
	}
	if bucketMinutes > 60 {
		return 60
	}
	return bucketMinutes
}

func normalizeChannelMonitorTimelineLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeChannelMonitorGroupFilter(group string) string {
	return strings.TrimSpace(group)
}

func buildChannelMonitorGroupCondition(columnExpr string) string {
	if common.UsingMySQL {
		return `CONCAT(',', ` + columnExpr + `, ',') LIKE ?`
	}
	return `(',' || ` + columnExpr + ` || ',') LIKE ?`
}

func channelMonitorGroupColumn(tableName string) string {
	if common.UsingPostgreSQL {
		if tableName == "" {
			return `"group"`
		}
		return tableName + `."group"`
	}
	if common.UsingSQLite {
		if tableName == "" {
			return `"group"`
		}
		return tableName + `."group"`
	}
	if tableName == "" {
		return "`group`"
	}
	return tableName + ".`group`"
}

func GetChannelMonitorGroups() ([]string, error) {
	if DB == nil {
		return []string{}, nil
	}
	channels, err := GetAllChannels(0, 0, true, false)
	if err != nil {
		return nil, err
	}
	groupSet := make(map[string]struct{})
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		for _, group := range channel.GetGroups() {
			group = strings.TrimSpace(group)
			if group == "" {
				continue
			}
			groupSet[group] = struct{}{}
		}
	}
	groups := make([]string, 0, len(groupSet))
	for group := range groupSet {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups, nil
}

func GetChannelMonitorTimeline(hours int, bucketMinutes int, limit int) ([]ChannelTimelineChannel, error) {
	return GetChannelMonitorTimelineByGroup(hours, bucketMinutes, limit, "")
}

func GetChannelMonitorTimelineByGroup(hours int, bucketMinutes int, limit int, groupFilter string) ([]ChannelTimelineChannel, error) {
	if DB == nil {
		return []ChannelTimelineChannel{}, nil
	}
	hours = normalizeChannelMonitorTimelineHours(hours)
	bucketMinutes = normalizeChannelMonitorTimelineBucketMinutes(bucketMinutes)
	limit = normalizeChannelMonitorTimelineLimit(limit)
	groupFilter = normalizeChannelMonitorGroupFilter(groupFilter)
	bucketDuration := time.Duration(bucketMinutes) * time.Minute
	start := time.Now().Add(-time.Duration(hours) * time.Hour).Truncate(bucketDuration)
	rows := make([]channelTimelineRow, 0)
	query := DB.Table((ChannelMonitorStat{}).TableName()+" AS cms").
		Select("cms.channel_id, channels.name AS channel_name, channels.type AS channel_type, channels.status AS channel_status, cms.time_bucket, SUM(cms.request_count) AS request_count, SUM(cms.success_count) AS success_count, SUM(cms.failure_count) AS failure_count").
		Joins("LEFT JOIN channels ON channels.id = cms.channel_id").
		Where("cms.granularity = ? AND cms.time_bucket >= ? AND cms.request_count > 0", ChannelMonitorGranularityMinute, start)
	if groupFilter != "" {
		query = query.Where(buildChannelMonitorGroupCondition(channelMonitorGroupColumn("channels")), "%,"+groupFilter+",%")
	}
	err := query.
		Group("cms.channel_id, channels.name, channels.type, channels.status, cms.time_bucket").
		Order("cms.channel_id ASC").
		Order("cms.time_bucket ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	type channelTimelineBucketKey struct {
		ChannelID  int
		TimeBucket time.Time
	}
	channelMap := make(map[int]*ChannelTimelineChannel)
	pointMap := make(map[channelTimelineBucketKey]*ChannelTimelinePoint)
	for _, row := range rows {
		if row.RequestCount <= 0 {
			continue
		}
		item := channelMap[row.ChannelID]
		if item == nil {
			item = &ChannelTimelineChannel{
				ChannelID:     row.ChannelID,
				ChannelName:   row.ChannelName,
				ChannelType:   row.ChannelType,
				ChannelStatus: row.ChannelStatus,
				Points:        make([]ChannelTimelinePoint, 0),
			}
			channelMap[row.ChannelID] = item
		}
		bucket := row.TimeBucket.Truncate(bucketDuration)
		key := channelTimelineBucketKey{ChannelID: row.ChannelID, TimeBucket: bucket}
		point := pointMap[key]
		if point == nil {
			item.Points = append(item.Points, ChannelTimelinePoint{
				ChannelID:     row.ChannelID,
				ChannelName:   row.ChannelName,
				ChannelType:   row.ChannelType,
				ChannelStatus: row.ChannelStatus,
				TimeBucket:    bucket,
			})
			point = &item.Points[len(item.Points)-1]
			pointMap[key] = point
		}
		point.RequestCount += row.RequestCount
		point.SuccessCount += row.SuccessCount
		point.FailureCount += row.FailureCount
		item.RequestCount += row.RequestCount
		item.SuccessRate += float64(row.SuccessCount)
	}
	items := make([]ChannelTimelineChannel, 0, len(channelMap))
	for _, item := range channelMap {
		if item == nil || item.RequestCount <= 0 || len(item.Points) == 0 {
			continue
		}
		sort.SliceStable(item.Points, func(i, j int) bool {
			return item.Points[i].TimeBucket.Before(item.Points[j].TimeBucket)
		})
		item.SuccessRate = item.SuccessRate / float64(item.RequestCount)
		items = append(items, *item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RequestCount == items[j].RequestCount {
			return strings.ToLower(items[i].ChannelName) < strings.ToLower(items[j].ChannelName)
		}
		return items[i].RequestCount > items[j].RequestCount
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func buildChannelMonitorChannelItems(days int) ([]ChannelMonitorChannelItem, error) {
	return buildChannelMonitorChannelItemsByGroup(days, "")
}

func buildChannelMonitorChannelItemsByGroup(days int, groupFilter string) ([]ChannelMonitorChannelItem, error) {
	days = normalizeChannelMonitorDays(days)
	groupFilter = normalizeChannelMonitorGroupFilter(groupFilter)
	start := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	stats, pending, err := loadChannelMonitorDataSince(start)
	if err != nil {
		return nil, err
	}
	channels := make([]*Channel, 0)
	query := DB.Omit("key")
	if groupFilter != "" {
		query = query.Where(buildChannelMonitorGroupCondition(channelMonitorGroupColumn("")), "%,"+groupFilter+",%")
	}
	if err = query.Find(&channels).Error; err != nil {
		return nil, err
	}
	allowedChannelIDs := make(map[int]struct{}, len(channels))
	metrics := make(map[int]*channelMonitorAggregate, len(channels))
	dailyMetrics := make(map[int]map[string]*channelMonitorAggregate, len(channels))
	applyMetric := func(date string, channelID int, requestCount int64, successCount int64, failureCount int64, avgLatencyMs float64, p95LatencyMs float64, lastActiveAt time.Time) {
		if requestCount <= 0 {
			return
		}
		if _, ok := allowedChannelIDs[channelID]; !ok {
			return
		}
		agg := metrics[channelID]
		if agg == nil {
			agg = &channelMonitorAggregate{}
			metrics[channelID] = agg
		}
		applyChannelMonitorAggregate(agg, requestCount, successCount, failureCount, avgLatencyMs, p95LatencyMs, lastActiveAt)
		dateMap := dailyMetrics[channelID]
		if dateMap == nil {
			dateMap = make(map[string]*channelMonitorAggregate)
			dailyMetrics[channelID] = dateMap
		}
		dayAgg := dateMap[date]
		if dayAgg == nil {
			dayAgg = &channelMonitorAggregate{}
			dateMap[date] = dayAgg
		}
		applyChannelMonitorAggregate(dayAgg, requestCount, successCount, failureCount, avgLatencyMs, p95LatencyMs, lastActiveAt)
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		allowedChannelIDs[channel.Id] = struct{}{}
	}
	for _, stat := range stats {
		applyMetric(stat.TimeBucket.Format("2006-01-02"), stat.ChannelID, stat.RequestCount, stat.SuccessCount, stat.FailureCount, stat.AvgLatencyMs, stat.P95LatencyMs, stat.LastActiveAt)
	}
	now := time.Now()
	for _, delta := range pending {
		applyMetric(now.Format("2006-01-02"), delta.ChannelID, delta.RequestCount, delta.SuccessCount, delta.FailureCount, delta.AvgLatencyMs, delta.P95LatencyMs, delta.LastActiveAt)
	}
	items := make([]ChannelMonitorChannelItem, 0, len(channels))
	trendStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1))
	for _, channel := range channels {
		item := ChannelMonitorChannelItem{
			Id:          channel.Id,
			Name:        channel.Name,
			GroupName:   channel.Group,
			Type:        channel.Type,
			Status:      channel.Status,
			HealthTrend: make([]float64, 0, days),
		}
		agg := metrics[channel.Id]
		if agg != nil && agg.RequestCount > 0 {
			item.RequestCount = agg.RequestCount
			item.FailureCount = agg.FailureCount
			item.SuccessRate = float64(agg.SuccessCount) / float64(agg.RequestCount)
			item.AvgLatencyMs = agg.LatencySumMs / float64(agg.RequestCount)
			item.P95LatencyMs = agg.P95LatencyMs
			item.LastActiveAt = agg.LastActiveAt
		}
		for i := 0; i < days; i++ {
			date := trendStart.AddDate(0, 0, i).Format("2006-01-02")
			dayAgg := dailyMetrics[channel.Id][date]
			if dayAgg == nil || dayAgg.RequestCount <= 0 {
				item.HealthTrend = append(item.HealthTrend, 0)
				continue
			}
			item.HealthTrend = append(item.HealthTrend, float64(dayAgg.SuccessCount)/float64(dayAgg.RequestCount))
		}
		items = append(items, item)
	}
	return items, nil
}

func isChannelMonitorLatencyLess(left ChannelMonitorChannelItem, right ChannelMonitorChannelItem) bool {
	leftHasLatency := left.AvgLatencyMs > 0
	rightHasLatency := right.AvgLatencyMs > 0
	if leftHasLatency != rightHasLatency {
		return leftHasLatency && !rightHasLatency
	}
	if left.AvgLatencyMs != right.AvgLatencyMs {
		return left.AvgLatencyMs < right.AvgLatencyMs
	}
	if left.P95LatencyMs != right.P95LatencyMs {
		return left.P95LatencyMs < right.P95LatencyMs
	}
	return strings.ToLower(left.Name) < strings.ToLower(right.Name)
}

func GetChannelMonitorChannelRankings(days int, top int) ([]ChannelMonitorChannelItem, []ChannelMonitorChannelItem, error) {
	items, err := buildChannelMonitorChannelItems(days)
	if err != nil {
		return nil, nil, err
	}
	top = normalizeChannelMonitorTimelineLimit(top)
	stabilityItems := append([]ChannelMonitorChannelItem(nil), items...)
	sort.SliceStable(stabilityItems, func(i, j int) bool {
		left := stabilityItems[i]
		right := stabilityItems[j]
		if left.SuccessRate != right.SuccessRate {
			return left.SuccessRate > right.SuccessRate
		}
		if left.FailureCount != right.FailureCount {
			return left.FailureCount < right.FailureCount
		}
		if left.RequestCount != right.RequestCount {
			return left.RequestCount > right.RequestCount
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	if len(stabilityItems) > top {
		stabilityItems = stabilityItems[:top]
	}
	latencyItems := append([]ChannelMonitorChannelItem(nil), items...)
	sort.SliceStable(latencyItems, func(i, j int) bool {
		return isChannelMonitorLatencyLess(latencyItems[i], latencyItems[j])
	})
	if len(latencyItems) > top {
		latencyItems = latencyItems[:top]
	}
	return stabilityItems, latencyItems, nil
}

func GetChannelMonitorChannelPage(days int, page int, pageSize int) ([]ChannelMonitorChannelItem, int64, error) {
	return GetChannelMonitorChannelPageByGroup(days, page, pageSize, "")
}

func GetChannelMonitorChannelPageByGroup(days int, page int, pageSize int, groupFilter string) ([]ChannelMonitorChannelItem, int64, error) {
	days = normalizeChannelMonitorDays(days)
	if page < 1 {
		page = 1
	}
	items, err := buildChannelMonitorChannelItemsByGroup(days, groupFilter)
	if err != nil {
		return nil, 0, err
	}
	total := int64(len(items))
	if pageSize <= 0 {
		return items, total, nil
	}
	if pageSize > 100 {
		pageSize = 100
	}
	startIdx := (page - 1) * pageSize
	if startIdx >= len(items) {
		return []ChannelMonitorChannelItem{}, total, nil
	}
	endIdx := startIdx + pageSize
	if endIdx > len(items) {
		endIdx = len(items)
	}
	return items[startIdx:endIdx], total, nil
}
