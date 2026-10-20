// FORK-CUSTOM: Aggregate upstream cache usage from consume logs.
package model

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ChannelMonitorCacheGranularityFiveMinute = 1
	ChannelMonitorCacheGranularityHour       = 2
	ChannelMonitorCacheGranularityDay        = 3

	channelMonitorCacheBucketDuration      = 5 * time.Minute
	channelMonitorCacheFiveMinuteRetention = 7 * 24 * time.Hour
	channelMonitorCacheHourRetention       = 90 * 24 * time.Hour
	channelMonitorCacheDayRetention        = 180 * 24 * time.Hour
	channelMonitorCacheAggregationLookback = 30 * time.Minute
	channelMonitorCacheAggregationTick     = 5 * time.Minute
	channelMonitorCacheMaxCatchupWindow    = 6 * time.Hour
)

type ChannelMonitorCacheStat struct {
	Id                  int       `json:"id"`
	ChannelID           int       `json:"channel_id" gorm:"uniqueIndex:idx_xnewapi_channel_cache_bucket,priority:1;index:idx_xnewapi_channel_cache_range,priority:1"`
	GroupName           string    `json:"group_name" gorm:"size:64;uniqueIndex:idx_xnewapi_channel_cache_bucket,priority:2"`
	ModelName           string    `json:"model_name" gorm:"size:128;uniqueIndex:idx_xnewapi_channel_cache_bucket,priority:3;index:idx_xnewapi_channel_cache_range,priority:2"`
	TimeBucket          time.Time `json:"time_bucket" gorm:"uniqueIndex:idx_xnewapi_channel_cache_bucket,priority:4;index:idx_xnewapi_channel_cache_range,priority:3"`
	Granularity         int       `json:"granularity" gorm:"uniqueIndex:idx_xnewapi_channel_cache_bucket,priority:5;index:idx_xnewapi_channel_cache_range,priority:4"`
	RequestCount        int64     `json:"request_count" gorm:"type:bigint;not null;default:0"`
	InputTokens         int64     `json:"input_tokens" gorm:"type:bigint;not null;default:0"`
	CachedTokens        int64     `json:"cached_tokens" gorm:"type:bigint;not null;default:0"`
	CacheCreationTokens int64     `json:"cache_creation_tokens" gorm:"type:bigint;not null;default:0"`
	CreatedAt           int64     `json:"created_at" gorm:"type:bigint;not null"`
	UpdatedAt           int64     `json:"updated_at" gorm:"type:bigint;not null"`
}

func (ChannelMonitorCacheStat) TableName() string {
	return "xnewapi_channel_monitor_cache_stats"
}

type ChannelMonitorCacheAggregationState struct {
	Id        int   `json:"id"`
	CursorAt  int64 `json:"cursor_at" gorm:"type:bigint;not null"`
	UpdatedAt int64 `json:"updated_at" gorm:"type:bigint;not null"`
}

func (ChannelMonitorCacheAggregationState) TableName() string {
	return "xnewapi_channel_monitor_cache_aggregation_state"
}

type ChannelMonitorCacheModelStat struct {
	ModelName           string   `json:"model_name"`
	RequestCount        int64    `json:"request_count"`
	InputTokens         int64    `json:"input_tokens"`
	CachedTokens        int64    `json:"cached_tokens"`
	CacheCreationTokens int64    `json:"cache_creation_tokens"`
	CacheRate           *float64 `json:"cache_rate"`
}

type channelMonitorCacheLogOther struct {
	CacheTokens         int64  `json:"cache_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	CacheWriteTokens    int64  `json:"cache_write_tokens"`
	InputTokensTotal    int64  `json:"input_tokens_total"`
	UsageSemantic       string `json:"usage_semantic"`
}

type channelMonitorCacheAggregateKey struct {
	ChannelID  int
	GroupName  string
	ModelName  string
	TimeBucket time.Time
}

type channelMonitorCacheAggregate struct {
	RequestCount        int64
	InputTokens         int64
	CachedTokens        int64
	CacheCreationTokens int64
}

func channelMonitorCacheDayStart(value time.Time) time.Time {
	value = value.In(channelMonitorLocation)
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, channelMonitorLocation)
}

func channelMonitorCacheBucket(value time.Time, granularity int) time.Time {
	value = value.In(channelMonitorLocation)
	switch granularity {
	case ChannelMonitorCacheGranularityFiveMinute:
		return value.Truncate(channelMonitorCacheBucketDuration)
	case ChannelMonitorCacheGranularityHour:
		return value.Truncate(time.Hour)
	case ChannelMonitorCacheGranularityDay:
		return channelMonitorCacheDayStart(value)
	default:
		return value
	}
}

func channelMonitorCacheCreationTokens(other channelMonitorCacheLogOther) int64 {
	return max(other.CacheCreationTokens, other.CacheWriteTokens)
}

func channelMonitorCacheInputTokens(log *Log, other channelMonitorCacheLogOther) int64 {
	if other.InputTokensTotal > 0 && !strings.EqualFold(other.UsageSemantic, "anthropic") {
		return other.InputTokensTotal
	}
	inputTokens := int64(log.PromptTokens)
	if strings.EqualFold(other.UsageSemantic, "anthropic") {
		inputTokens += max(other.CacheTokens, 0)
		inputTokens += channelMonitorCacheCreationTokens(other)
	}
	return max(inputTokens, 0)
}

func loadChannelMonitorCacheLogAggregates(start time.Time, end time.Time) (map[channelMonitorCacheAggregateKey]channelMonitorCacheAggregate, map[time.Time]struct{}, error) {
	aggregates := make(map[channelMonitorCacheAggregateKey]channelMonitorCacheAggregate)
	buckets := make(map[time.Time]struct{})
	if LOG_DB == nil || !end.After(start) {
		return aggregates, buckets, nil
	}

	query := LOG_DB.Model(&Log{}).
		Where("type = ? AND channel_id > 0 AND created_at >= ? AND created_at < ?", LogTypeConsume, start.Unix(), end.Unix())
	rows, err := query.Rows()
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var log Log
		if err := LOG_DB.ScanRows(rows, &log); err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(log.ModelName) == "" || log.CreatedAt <= 0 {
			continue
		}
		var other channelMonitorCacheLogOther
		if log.Other == "" || common.UnmarshalJsonStr(log.Other, &other) != nil {
			continue
		}
		bucket := channelMonitorCacheBucket(time.Unix(log.CreatedAt, 0), ChannelMonitorCacheGranularityFiveMinute)
		key := channelMonitorCacheAggregateKey{
			ChannelID:  log.ChannelId,
			GroupName:  strings.ToLower(strings.TrimSpace(log.Group)),
			ModelName:  strings.TrimSpace(log.ModelName),
			TimeBucket: bucket,
		}
		aggregate := aggregates[key]
		aggregate.RequestCount++
		aggregate.InputTokens += channelMonitorCacheInputTokens(&log, other)
		aggregate.CachedTokens += max(other.CacheTokens, 0)
		aggregate.CacheCreationTokens += channelMonitorCacheCreationTokens(other)
		aggregates[key] = aggregate
		buckets[bucket] = struct{}{}
	}
	return aggregates, buckets, rows.Err()
}

func sortedChannelMonitorCacheKeys(aggregates map[channelMonitorCacheAggregateKey]channelMonitorCacheAggregate) []channelMonitorCacheAggregateKey {
	keys := make([]channelMonitorCacheAggregateKey, 0, len(aggregates))
	for key := range aggregates {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].TimeBucket != keys[j].TimeBucket {
			return keys[i].TimeBucket.Before(keys[j].TimeBucket)
		}
		if keys[i].ChannelID != keys[j].ChannelID {
			return keys[i].ChannelID < keys[j].ChannelID
		}
		if keys[i].GroupName != keys[j].GroupName {
			return keys[i].GroupName < keys[j].GroupName
		}
		return keys[i].ModelName < keys[j].ModelName
	})
	return keys
}

func replaceChannelMonitorCacheBuckets(aggregates map[channelMonitorCacheAggregateKey]channelMonitorCacheAggregate, buckets map[time.Time]struct{}) error {
	if DB == nil || len(buckets) == 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for bucket := range buckets {
			if err := tx.Where("granularity = ? AND time_bucket = ?", ChannelMonitorCacheGranularityFiveMinute, bucket).Delete(&ChannelMonitorCacheStat{}).Error; err != nil {
				return err
			}
		}
		nowUnix := time.Now().Unix()
		for _, key := range sortedChannelMonitorCacheKeys(aggregates) {
			aggregate := aggregates[key]
			if _, ok := buckets[key.TimeBucket]; !ok {
				continue
			}
			stat := &ChannelMonitorCacheStat{
				ChannelID: key.ChannelID, GroupName: key.GroupName, ModelName: key.ModelName,
				TimeBucket: key.TimeBucket, Granularity: ChannelMonitorCacheGranularityFiveMinute,
				RequestCount: aggregate.RequestCount, InputTokens: aggregate.InputTokens,
				CachedTokens: aggregate.CachedTokens, CacheCreationTokens: aggregate.CacheCreationTokens,
				CreatedAt: nowUnix, UpdatedAt: nowUnix,
			}
			if err := tx.Create(stat).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func rebuildChannelMonitorCacheAggregate(targetGranularity int, bucket time.Time) error {
	if DB == nil {
		return nil
	}
	sourceGranularity := targetGranularity - 1
	if sourceGranularity < ChannelMonitorCacheGranularityFiveMinute {
		return nil
	}
	var source []ChannelMonitorCacheStat
	from := bucket
	var to time.Time
	if targetGranularity == ChannelMonitorCacheGranularityHour {
		to = bucket.Add(time.Hour)
	} else {
		to = bucket.AddDate(0, 0, 1)
	}
	if err := DB.Where("granularity = ? AND time_bucket >= ? AND time_bucket < ?", sourceGranularity, from, to).Find(&source).Error; err != nil {
		return err
	}
	aggregates := make(map[channelMonitorCacheAggregateKey]channelMonitorCacheAggregate)
	for _, stat := range source {
		key := channelMonitorCacheAggregateKey{ChannelID: stat.ChannelID, GroupName: stat.GroupName, ModelName: stat.ModelName, TimeBucket: bucket}
		aggregate := aggregates[key]
		aggregate.RequestCount += stat.RequestCount
		aggregate.InputTokens += stat.InputTokens
		aggregate.CachedTokens += stat.CachedTokens
		aggregate.CacheCreationTokens += stat.CacheCreationTokens
		aggregates[key] = aggregate
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("granularity = ? AND time_bucket = ?", targetGranularity, bucket).Delete(&ChannelMonitorCacheStat{}).Error; err != nil {
			return err
		}
		nowUnix := time.Now().Unix()
		for _, key := range sortedChannelMonitorCacheKeys(aggregates) {
			aggregate := aggregates[key]
			if err := tx.Create(&ChannelMonitorCacheStat{
				ChannelID: key.ChannelID, GroupName: key.GroupName, ModelName: key.ModelName,
				TimeBucket: key.TimeBucket, Granularity: targetGranularity,
				RequestCount: aggregate.RequestCount, InputTokens: aggregate.InputTokens,
				CachedTokens: aggregate.CachedTokens, CacheCreationTokens: aggregate.CacheCreationTokens,
				CreatedAt: nowUnix, UpdatedAt: nowUnix,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func refreshChannelMonitorCacheAggregates(buckets map[time.Time]struct{}) error {
	hours := make(map[time.Time]struct{})
	for bucket := range buckets {
		hours[channelMonitorCacheBucket(bucket, ChannelMonitorCacheGranularityHour)] = struct{}{}
	}
	for hour := range hours {
		if err := rebuildChannelMonitorCacheAggregate(ChannelMonitorCacheGranularityHour, hour); err != nil {
			return err
		}
	}
	days := make(map[time.Time]struct{})
	for hour := range hours {
		days[channelMonitorCacheBucket(hour, ChannelMonitorCacheGranularityDay)] = struct{}{}
	}
	for day := range days {
		if err := rebuildChannelMonitorCacheAggregate(ChannelMonitorCacheGranularityDay, day); err != nil {
			return err
		}
	}
	return nil
}

func AggregateChannelMonitorCacheStats(start time.Time, end time.Time) error {
	if DB == nil || LOG_DB == nil {
		return nil
	}
	start = channelMonitorCacheBucket(start, ChannelMonitorCacheGranularityFiveMinute)
	end = channelMonitorCacheBucket(end, ChannelMonitorCacheGranularityFiveMinute)
	if !end.After(start) {
		return nil
	}
	aggregates, buckets, err := loadChannelMonitorCacheLogAggregates(start, end)
	if err != nil {
		return err
	}
	if err := replaceChannelMonitorCacheBuckets(aggregates, buckets); err != nil {
		return err
	}
	return refreshChannelMonitorCacheAggregates(buckets)
}

var channelMonitorCacheAggregationMu sync.Mutex

func PersistChannelMonitorCacheStats() error {
	channelMonitorCacheAggregationMu.Lock()
	defer channelMonitorCacheAggregationMu.Unlock()
	if DB == nil || LOG_DB == nil {
		return nil
	}
	now := time.Now().In(channelMonitorLocation)
	end := channelMonitorCacheBucket(now.Add(-channelMonitorCacheAggregationTick), ChannelMonitorCacheGranularityFiveMinute)
	var state ChannelMonitorCacheAggregationState
	err := DB.First(&state, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		state = ChannelMonitorCacheAggregationState{
			Id: 1, CursorAt: end.Add(-channelMonitorCacheAggregationLookback).Unix(), UpdatedAt: now.Unix(),
		}
		if err := DB.Create(&state).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	start := time.Unix(state.CursorAt, 0).In(channelMonitorLocation)
	if !start.Before(end) {
		return nil
	}
	for start.Before(end) {
		chunkEnd := start.Add(channelMonitorCacheMaxCatchupWindow)
		if chunkEnd.After(end) {
			chunkEnd = end
		}
		if err := AggregateChannelMonitorCacheStats(start, chunkEnd); err != nil {
			return err
		}
		state.CursorAt = chunkEnd.Unix()
		state.UpdatedAt = now.Unix()
		if err := DB.Save(&state).Error; err != nil {
			return err
		}
		start = chunkEnd
	}
	return nil
}

func CleanupExpiredChannelMonitorCacheStats(now time.Time) error {
	if DB == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	if err := DB.Where("granularity = ? AND time_bucket < ?", ChannelMonitorCacheGranularityFiveMinute, now.Add(-channelMonitorCacheFiveMinuteRetention)).Delete(&ChannelMonitorCacheStat{}).Error; err != nil {
		return err
	}
	if err := DB.Where("granularity = ? AND time_bucket < ?", ChannelMonitorCacheGranularityHour, now.Add(-channelMonitorCacheHourRetention)).Delete(&ChannelMonitorCacheStat{}).Error; err != nil {
		return err
	}
	return DB.Where("granularity = ? AND time_bucket < ?", ChannelMonitorCacheGranularityDay, now.Add(-channelMonitorCacheDayRetention)).Delete(&ChannelMonitorCacheStat{}).Error
}

func StartChannelMonitorCacheAggregationTask() {
	go func() {
		_ = PersistChannelMonitorCacheStats()
		ticker := time.NewTicker(channelMonitorCacheAggregationTick)
		defer ticker.Stop()
		for range ticker.C {
			_ = PersistChannelMonitorCacheStats()
		}
	}()
}

func channelMonitorCacheGranularityForRange(start time.Time, end time.Time) int {
	duration := end.Sub(start)
	if duration <= channelMonitorCacheFiveMinuteRetention {
		return ChannelMonitorCacheGranularityFiveMinute
	}
	if duration <= channelMonitorCacheHourRetention {
		return ChannelMonitorCacheGranularityHour
	}
	return ChannelMonitorCacheGranularityDay
}

func GetChannelMonitorCacheModelStats(channelID int, start time.Time, end time.Time, groupName string) ([]ChannelMonitorCacheModelStat, error) {
	if DB == nil || channelID <= 0 || !end.After(start) {
		return []ChannelMonitorCacheModelStat{}, nil
	}
	start = start.In(channelMonitorLocation)
	end = end.In(channelMonitorLocation)
	granularity := channelMonitorCacheGranularityForRange(start, end)
	query := DB.Where("channel_id = ? AND granularity = ? AND time_bucket >= ? AND time_bucket < ?", channelID, granularity, start, end)
	if groupName = strings.ToLower(strings.TrimSpace(groupName)); groupName != "" {
		query = query.Where("group_name = ?", groupName)
	}
	var stats []ChannelMonitorCacheStat
	if err := query.Find(&stats).Error; err != nil {
		return nil, err
	}
	aggregates := make(map[string]ChannelMonitorCacheModelStat)
	for _, stat := range stats {
		item := aggregates[stat.ModelName]
		item.ModelName = stat.ModelName
		item.RequestCount += stat.RequestCount
		item.InputTokens += stat.InputTokens
		item.CachedTokens += stat.CachedTokens
		item.CacheCreationTokens += stat.CacheCreationTokens
		aggregates[stat.ModelName] = item
	}
	result := make([]ChannelMonitorCacheModelStat, 0, len(aggregates))
	for _, item := range aggregates {
		if item.InputTokens > 0 {
			rate := float64(item.CachedTokens) / float64(item.InputTokens)
			if rate < 0 {
				rate = 0
			}
			if rate > 1 {
				rate = 1
			}
			item.CacheRate = &rate
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CachedTokens != result[j].CachedTokens {
			return result[i].CachedTokens > result[j].CachedTokens
		}
		return result[i].ModelName < result[j].ModelName
	})
	return result, nil
}
