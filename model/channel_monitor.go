package model

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

type ChannelRuntimeStats struct {
	RequestCount       atomic.Int64
	SuccessCount       atomic.Int64
	FailureCount       atomic.Int64
	TotalLatencyMicros atomic.Int64
	LastLatencyMicros  atomic.Int64
	LastActiveUnix     atomic.Int64
}

type ChannelRuntimeStatsSnapshot struct {
	RequestCount  int64     `json:"request_count"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	AvgLatencyMs  float64   `json:"avg_latency_ms"`
	LastLatencyMs float64   `json:"last_latency_ms"`
	LastActiveAt  time.Time `json:"last_active_at"`
}

var channelRuntimeStatsStore sync.Map
var channelAggregateRuntimeStatsStore sync.Map

func ResetChannelMonitorRuntimeStateForTest() {
	channelRuntimeStatsStore = sync.Map{}
	channelAggregateRuntimeStatsStore = sync.Map{}
	channelLatencySampleStore = sync.Map{}
	channelMonitorPersistStateStore = sync.Map{}
}

func ObserveChannelRuntime(group string, modelName string, channelID int, success bool, latency time.Duration) {
	if channelID <= 0 {
		return
	}
	if latency < 0 {
		latency = 0
	}
	now := time.Now()

	aggregateStats := loadChannelRuntimeStats(&channelAggregateRuntimeStatsStore, buildChannelAggregateRuntimeStatsKey(channelID))
	aggregateStats.observe(success, latency, now)
	recordChannelLatencySample(channelID, latency)

	if strings.TrimSpace(modelName) == "" {
		return
	}
	runtimeStats := loadChannelRuntimeStats(&channelRuntimeStatsStore, buildChannelRuntimeStatsKey(group, modelName, channelID))
	runtimeStats.observe(success, latency, now)
}

func GetChannelRuntimeStats(group string, modelName string, channelID int) ChannelRuntimeStatsSnapshot {
	if channelID <= 0 || strings.TrimSpace(modelName) == "" {
		return ChannelRuntimeStatsSnapshot{}
	}
	stats, ok := channelRuntimeStatsStore.Load(buildChannelRuntimeStatsKey(group, modelName, channelID))
	if !ok {
		return ChannelRuntimeStatsSnapshot{}
	}
	return stats.(*ChannelRuntimeStats).Snapshot()
}

func GetChannelAggregateRuntimeStats(channelID int) ChannelRuntimeStatsSnapshot {
	if channelID <= 0 {
		return ChannelRuntimeStatsSnapshot{}
	}
	stats, ok := channelAggregateRuntimeStatsStore.Load(buildChannelAggregateRuntimeStatsKey(channelID))
	if !ok {
		return ChannelRuntimeStatsSnapshot{}
	}
	return stats.(*ChannelRuntimeStats).Snapshot()
}

func (s *ChannelRuntimeStats) Snapshot() ChannelRuntimeStatsSnapshot {
	if s == nil {
		return ChannelRuntimeStatsSnapshot{}
	}
	requestCount := s.RequestCount.Load()
	totalLatencyMicros := s.TotalLatencyMicros.Load()
	lastActiveUnix := s.LastActiveUnix.Load()

	snapshot := ChannelRuntimeStatsSnapshot{
		RequestCount: requestCount,
		SuccessCount: s.SuccessCount.Load(),
		FailureCount: s.FailureCount.Load(),
		LastLatencyMs: func() float64 {
			return float64(s.LastLatencyMicros.Load()) / 1000
		}(),
	}
	if requestCount > 0 {
		snapshot.AvgLatencyMs = float64(totalLatencyMicros) / float64(requestCount) / 1000
	}
	if lastActiveUnix > 0 {
		snapshot.LastActiveAt = time.Unix(lastActiveUnix, 0)
	}
	return snapshot
}

func (s *ChannelRuntimeStats) observe(success bool, latency time.Duration, now time.Time) {
	if s == nil {
		return
	}
	latencyMicros := latency.Microseconds()
	s.RequestCount.Add(1)
	if success {
		s.SuccessCount.Add(1)
	} else {
		s.FailureCount.Add(1)
	}
	s.TotalLatencyMicros.Add(latencyMicros)
	s.LastLatencyMicros.Store(latencyMicros)
	s.LastActiveUnix.Store(now.Unix())
}

func loadChannelRuntimeStats(store *sync.Map, key string) *ChannelRuntimeStats {
	stats, _ := store.LoadOrStore(key, &ChannelRuntimeStats{})
	return stats.(*ChannelRuntimeStats)
}

func buildChannelRuntimeStatsKey(group string, modelName string, channelID int) string {
	return strings.ToLower(strings.TrimSpace(group)) + ":" + strings.TrimSpace(modelName) + ":" + fmt.Sprintf("%d", channelID)
}

func buildChannelAggregateRuntimeStatsKey(channelID int) string {
	return fmt.Sprintf("channel:%d", channelID)
}

type channelLatencySnapshot struct {
	AvgLatencyMs float64
	P95LatencyMs float64
}

type channelLatencyRing struct {
	mu      sync.Mutex
	samples []int64
	next    int
	count   int
}

func (r *channelLatencyRing) add(latency time.Duration) {
	if r == nil {
		return
	}
	latencyMicros := latency.Microseconds()
	if latencyMicros < 0 {
		latencyMicros = 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.samples) == 0 {
		r.samples = make([]int64, 100)
	}
	r.samples[r.next] = latencyMicros
	r.next = (r.next + 1) % len(r.samples)
	if r.count < len(r.samples) {
		r.count++
	}
}

func (r *channelLatencyRing) snapshot() channelLatencySnapshot {
	if r == nil {
		return channelLatencySnapshot{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.count == 0 || len(r.samples) == 0 {
		return channelLatencySnapshot{}
	}
	values := make([]int64, r.count)
	for i := 0; i < r.count; i++ {
		values[i] = r.samples[i]
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
	var total int64
	for _, value := range values {
		total += value
	}
	p95Idx := int(math.Ceil(float64(len(values))*0.95)) - 1
	if p95Idx < 0 {
		p95Idx = 0
	}
	if p95Idx >= len(values) {
		p95Idx = len(values) - 1
	}
	return channelLatencySnapshot{
		AvgLatencyMs: float64(total) / float64(len(values)) / 1000,
		P95LatencyMs: float64(values[p95Idx]) / 1000,
	}
}

var channelLatencySampleStore sync.Map

func recordChannelLatencySample(channelID int, latency time.Duration) {
	if channelID <= 0 {
		return
	}
	ringAny, _ := channelLatencySampleStore.LoadOrStore(channelID, &channelLatencyRing{})
	ringAny.(*channelLatencyRing).add(latency)
}

func GetChannelLatencySnapshot(channelID int) channelLatencySnapshot {
	if channelID <= 0 {
		return channelLatencySnapshot{}
	}
	ringAny, ok := channelLatencySampleStore.Load(channelID)
	if !ok {
		return channelLatencySnapshot{}
	}
	return ringAny.(*channelLatencyRing).snapshot()
}

func buildChannelRuntimeBucket(group string, modelName string, channelID int, bucket time.Time) ChannelMonitorStat {
	stats := GetChannelRuntimeStats(group, modelName, channelID)
	latency := GetChannelLatencySnapshot(channelID)
	if latency.AvgLatencyMs <= 0 {
		latency.AvgLatencyMs = stats.AvgLatencyMs
	}
	return ChannelMonitorStat{
		ChannelID:    channelID,
		GroupName:    strings.ToLower(strings.TrimSpace(group)),
		ModelName:    strings.TrimSpace(modelName),
		TimeBucket:   bucket,
		Granularity:  ChannelMonitorGranularityMinute,
		RequestCount: stats.RequestCount,
		SuccessCount: stats.SuccessCount,
		FailureCount: stats.FailureCount,
		AvgLatencyMs: latency.AvgLatencyMs,
		P95LatencyMs: latency.P95LatencyMs,
		LastActiveAt: stats.LastActiveAt,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
}

func GetSatisfiedChannels(group string, modelName string, retry int) ([]*Channel, error) {
	if !common.MemoryCacheEnabled {
		return getSatisfiedChannelsFromDB(group, modelName, retry)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channels := group2model2channels[group][modelName]
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
		channels = group2model2channels[group][normalizedModel]
	}
	if len(channels) == 0 {
		return nil, nil
	}
	if len(channels) == 1 {
		channel, ok := channelsIDM[channels[0]]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channels[0])
		}
		return []*Channel{channel}, nil
	}

	uniquePriorities := make(map[int]bool)
	for _, channelID := range channels {
		channel, ok := channelsIDM[channelID]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		uniquePriorities[int(channel.GetPriority())] = true
	}

	sortedUniquePriorities := make([]int, 0, len(uniquePriorities))
	for priority := range uniquePriorities {
		sortedUniquePriorities = append(sortedUniquePriorities, priority)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sortedUniquePriorities)))
	if len(sortedUniquePriorities) == 0 {
		return nil, nil
	}
	if retry >= len(sortedUniquePriorities) {
		retry = len(sortedUniquePriorities) - 1
	}
	targetPriority := int64(sortedUniquePriorities[retry])

	targetChannels := make([]*Channel, 0, len(channels))
	for _, channelID := range channels {
		channel, ok := channelsIDM[channelID]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		if channel.GetPriority() == targetPriority {
			targetChannels = append(targetChannels, channel)
		}
	}
	if len(targetChannels) == 0 {
		return nil, fmt.Errorf("no channel found, group: %s, model: %s, priority: %d", group, modelName, targetPriority)
	}
	return targetChannels, nil
}

// GetPriorityStageCount returns the number of distinct priority stages for the given group+model.
func GetPriorityStageCount(group string, modelName string) int {
	if !common.MemoryCacheEnabled {
		var count int64
		DB.Model(&Ability{}).
			Select("COUNT(DISTINCT priority)").
			Where(commonGroupCol+" = ? AND model = ? AND enabled = ?", group, modelName, commonTrueVal).
			Scan(&count)
		return int(count)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()

	channels := group2model2channels[group][modelName]
	if len(channels) == 0 {
		normalizedModel := ratio_setting.FormatMatchingModelName(modelName)
		channels = group2model2channels[group][normalizedModel]
	}
	if len(channels) == 0 {
		return 0
	}
	seen := make(map[int]struct{}, 4)
	for _, channelID := range channels {
		ch, ok := channelsIDM[channelID]
		if ok {
			seen[int(ch.GetPriority())] = struct{}{}
		}
	}
	return len(seen)
}

func getSatisfiedChannelsFromDB(group string, modelName string, retry int) ([]*Channel, error) {
	channelQuery, err := getChannelQuery(group, modelName, retry)
	if err != nil {
		return nil, err
	}

	var abilities []Ability
	err = channelQuery.Order("weight DESC").Find(&abilities).Error
	if err != nil {
		return nil, err
	}
	if len(abilities) == 0 {
		return nil, nil
	}

	channelIDs := make([]int, 0, len(abilities))
	seen := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, ok := seen[ability.ChannelId]; ok {
			continue
		}
		seen[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}

	channels := make([]*Channel, 0, len(channelIDs))
	if err = DB.Where("id in (?)", channelIDs).Find(&channels).Error; err != nil {
		return nil, err
	}
	channelByID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		channelByID[channel.Id] = channel
	}

	result := make([]*Channel, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		channel, ok := channelByID[channelID]
		if !ok {
			return nil, fmt.Errorf("数据库一致性错误，渠道# %d 不存在，请联系管理员修复", channelID)
		}
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		result = append(result, channel)
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}
