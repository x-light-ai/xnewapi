package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelSuccessRateLifecycleTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelCircuitEvent{}))

	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestChannelSuccessRateSelectorCircuitLifecycleIntegration(t *testing.T) {
	setupChannelSuccessRateLifecycleTestDB(t)
	prepareSuccessRateSelectionIntegrationTest(t)

	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	defaultChannelSuccessRateSelector = selector

	group := "default"
	modelName := fmt.Sprintf("gpt-4o-life-%d", time.Now().UnixNano())
	baseID := int(time.Now().UnixNano()%1_000_000_000) + 960000
	primary := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	fallback := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
		RecoveryCheckInterval:    10 * time.Minute,
		CircuitScope:             "model",
		HalfOpenSuccessThreshold: 2,
		MinSampleSize:            3,
		DisableThreshold:         0.5,
	}
	selector.RegisterFailureCallback(func(event channelFailureEvent) {
		if event.Reason != "consecutive" {
			return
		}
		if event.Observed < cfg.MinSampleSize {
			return
		}
		if event.Score < cfg.DisableThreshold {
			selector.openTemporaryCircuit(event, cfg)
		}
	})

	for i := 0; i < 3; i++ {
		selector.observe(group, modelName, primary.Id, false, cfg)
		model.ObserveChannelRuntime(group, modelName, primary.Id, false, 200*time.Millisecond)
	}
	for i := 0; i < 2; i++ {
		selector.observe(group, modelName, fallback.Id, true, cfg)
		model.ObserveChannelRuntime(group, modelName, fallback.Id, true, 100*time.Millisecond)
	}

	runtimeState := selector.GetRuntimeStateForChannel(primary.Id, cfg)
	require.True(t, runtimeState.TemporaryCircuitOpen)
	require.False(t, runtimeState.TemporaryCircuitUntil.IsZero())
	require.Contains(t, runtimeState.TemporaryCircuitReason, "连续失败触发临时熔断")
	require.Equal(t, 3, runtimeState.Observed)

	stats := model.GetChannelRuntimeStats(group, modelName, primary.Id)
	require.EqualValues(t, 3, stats.RequestCount)
	require.EqualValues(t, 3, stats.FailureCount)
	require.EqualValues(t, 0, stats.SuccessCount)
	require.InDelta(t, 200.0, stats.AvgLatencyMs, 0.01)

	selected := selector.pick(group, modelName, []*model.Channel{primary, fallback}, cfg)
	require.NotNil(t, selected)
	require.Equal(t, fallback.Id, selected.Id)

	now = now.Add(cfg.RecoveryCheckInterval + time.Second)
	selector.now = func() time.Time { return now }
	selector.clearExpiredTemporaryCircuits(cfg)

	runtimeState = selector.GetRuntimeStateForChannel(primary.Id, cfg)
	require.False(t, runtimeState.TemporaryCircuitOpen)
	require.True(t, runtimeState.HalfOpen)
	require.Equal(t, 0, runtimeState.HalfOpenSuccesses)

	halfOpenSelected := selector.pick(group, modelName, []*model.Channel{{Id: primary.Id, Name: primary.Name, Priority: primary.Priority}}, cfg)
	require.NotNil(t, halfOpenSelected)
	require.Equal(t, primary.Id, halfOpenSelected.Id)

	circuitKey := selector.circuitKeyForScope(successRateGroupKey(group), modelName, primary.Id, cfg.CircuitScope)
	selector.mu.Lock()
	circuitState := selector.circuitState[circuitKey]
	selector.mu.Unlock()
	require.True(t, circuitState.halfOpenProbeInFlight)

	selector.observe(group, modelName, primary.Id, true, cfg)
	model.ObserveChannelRuntime(group, modelName, primary.Id, true, 120*time.Millisecond)
	runtimeState = selector.GetRuntimeStateForChannel(primary.Id, cfg)
	require.True(t, runtimeState.HalfOpen)
	require.Equal(t, 1, runtimeState.HalfOpenSuccesses)

	halfOpenSelected = selector.pick(group, modelName, []*model.Channel{{Id: primary.Id, Name: primary.Name, Priority: primary.Priority}}, cfg)
	require.NotNil(t, halfOpenSelected)
	require.Equal(t, primary.Id, halfOpenSelected.Id)
	selector.observe(group, modelName, primary.Id, true, cfg)
	model.ObserveChannelRuntime(group, modelName, primary.Id, true, 110*time.Millisecond)

	runtimeState = selector.GetRuntimeStateForChannel(primary.Id, cfg)
	require.False(t, runtimeState.TemporaryCircuitOpen)
	require.False(t, runtimeState.HalfOpen)
	require.Equal(t, 5, runtimeState.Observed)

	stats = model.GetChannelRuntimeStats(group, modelName, primary.Id)
	require.EqualValues(t, 5, stats.RequestCount)
	require.EqualValues(t, 3, stats.FailureCount)
	require.EqualValues(t, 2, stats.SuccessCount)
}

func TestChannelSuccessRateSelectorChannelScopeIsolationIntegration(t *testing.T) {
	setupChannelSuccessRateLifecycleTestDB(t)
	prepareSuccessRateSelectionIntegrationTest(t)

	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	group := "default"
	channelID := int(time.Now().UnixNano()%1_000_000_000) + 970000
	priority := int64(10)
	channel := &model.Channel{Id: channelID, Name: "shared-channel", Type: constant.ChannelTypeOpenAI, Key: fmt.Sprintf("k-%d", channelID), Status: common.ChannelStatusEnabled, Group: group, Models: "gpt-4o,claude-3-7-sonnet", Priority: &priority, CreatedTime: time.Now().Unix()}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	model.InitChannelCache()
	defer func() {
		_ = model.DB.Where("channel_id = ?", channelID).Delete(&model.Ability{}).Error
		_ = model.DB.Delete(&model.Channel{}, "id = ?", channelID).Error
	}()

	cfgChannel := channelSuccessRateConfig{HalfLifeSeconds: 1800, RecoveryCheckInterval: 10 * time.Minute, CircuitScope: "channel", HalfOpenSuccessThreshold: 2}
	sharedKeyA := selector.circuitKeyForScope(successRateGroupKey(group), "gpt-4o", channelID, cfgChannel.CircuitScope)
	sharedKeyB := selector.circuitKeyForScope(successRateGroupKey(group), "claude-3-7-sonnet", channelID, cfgChannel.CircuitScope)
	require.Equal(t, sharedKeyA, sharedKeyB)
	selector.openTemporaryCircuit(channelFailureEvent{Group: group, ModelName: "gpt-4o", ChannelID: channelID, Reason: "immediate", CircuitScope: cfgChannel.CircuitScope}, cfgChannel)

	selected := selector.pick(group, "claude-3-7-sonnet", []*model.Channel{channel}, cfgChannel)
	require.Nil(t, selected)

	selector = newTestChannelSuccessRateSelector(now)
	cfgModel := channelSuccessRateConfig{HalfLifeSeconds: 1800, RecoveryCheckInterval: 10 * time.Minute, CircuitScope: "model", HalfOpenSuccessThreshold: 2}
	selector.openTemporaryCircuit(channelFailureEvent{Group: group, ModelName: "gpt-4o", ChannelID: channelID, Reason: "immediate", CircuitScope: cfgModel.CircuitScope}, cfgModel)

	selected = selector.pick(group, "claude-3-7-sonnet", []*model.Channel{channel}, cfgModel)
	require.NotNil(t, selected)
	require.Equal(t, channelID, selected.Id)
}
