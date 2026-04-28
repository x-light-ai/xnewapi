package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestGetChannelSuccessRateRuntimeStateReturnsZeroWhenMissing(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	state := GetChannelSuccessRateRuntimeState(999999999)
	require.False(t, state.TemporaryCircuitOpen)
	require.Zero(t, state.CurrentWeightedScore)
	require.Zero(t, state.Observed)
}

func TestGetChannelSuccessRateRuntimeStateReturnsWeightedScore(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-runtime-score-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-runtime-score-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 910000

	channel := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	key := defaultChannelSuccessRateSelector.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), channel.Id)
	defaultChannelSuccessRateSelector.mu.Lock()
	defaultChannelSuccessRateSelector.state[key] = channelSuccessRateState{
		success:  3,
		failure:  1,
		updated:  time.Now(),
		observed: 4,
	}
	defaultChannelSuccessRateSelector.mu.Unlock()

	state := GetChannelSuccessRateRuntimeState(channel.Id)
	require.False(t, state.TemporaryCircuitOpen)
	require.Equal(t, 4, state.Observed)
	require.Equal(t, successRateGroupKey(group), state.Group)
	require.Equal(t, successRateModelKey(modelName), state.ModelName)
	require.Greater(t, state.CurrentWeightedScore, 0.0)
}

func TestChannelSuccessRateManualScoreOverrideAffectsRuntimeStateAndSelection(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-manual-override-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-manual-override-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 930000

	lowChannel := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	highChannel := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	lowKey := defaultChannelSuccessRateSelector.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), lowChannel.Id)
	highKey := defaultChannelSuccessRateSelector.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), highChannel.Id)
	defaultChannelSuccessRateSelector.mu.Lock()
	defaultChannelSuccessRateSelector.state[lowKey] = channelSuccessRateState{
		success:  1,
		failure:  9,
		updated:  time.Now(),
		observed: 10,
	}
	defaultChannelSuccessRateSelector.state[highKey] = channelSuccessRateState{
		success:  9,
		failure:  1,
		updated:  time.Now(),
		observed: 10,
	}
	defaultChannelSuccessRateSelector.mu.Unlock()

	overrideScore := 1.5
	SetChannelScoreOverride(lowChannel.Id, &overrideScore)

	state := GetChannelSuccessRateRuntimeState(lowChannel.Id)
	require.Equal(t, overrideScore, state.CurrentWeightedScore)

	selected, selectedScore, others := defaultChannelSuccessRateSelector.pickDetailed(group, modelName, []*model.Channel{lowChannel, highChannel}, buildChannelSuccessRateConfig())
	require.Equal(t, lowChannel.Id, selected.Id)
	require.Equal(t, overrideScore, selectedScore)
	require.Len(t, others, 1)
	require.Equal(t, highChannel.Id, others[0].channelID)
}

func TestHandleChannelSuccessRateFailureConsecutiveSkipsWhenSampleSizeInsufficient(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	originalAutoDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = originalAutoDisable
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-consecutive-sample-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-consecutive-sample-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 920000

	channel := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	key := defaultChannelSuccessRateSelector.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), channel.Id)
	defaultChannelSuccessRateSelector.mu.Lock()
	defaultChannelSuccessRateSelector.state[key] = channelSuccessRateState{
		success:          1,
		failure:          8,
		updated:          time.Now(),
		consecutiveFails: 5,
		observed:         5,
	}
	defaultChannelSuccessRateSelector.mu.Unlock()

	event := channelFailureEvent{
		Group:     group,
		ModelName: modelName,
		ChannelID: channel.Id,
		Reason:    "consecutive",
	}

	handleChannelSuccessRateFailure(event)

	updatedChannel, err := model.CacheGetChannel(channel.Id)
	require.NoError(t, err)
	require.NotNil(t, updatedChannel)
	require.Equal(t, common.ChannelStatusEnabled, updatedChannel.Status)
}

func TestChannelSuccessRateSelectorPruneLockedClearsStateWhenExceedingMaxKeys(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	selector.maxKeys = 5

	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}

	for i := 1; i <= 5; i++ {
		selector.observe("default", "gpt-4o", i, true, cfg)
	}

	selector.mu.Lock()
	stateCountBefore := len(selector.state)
	selector.mu.Unlock()
	require.Equal(t, 5, stateCountBefore)

	selector.observe("default", "gpt-4o", 6, true, cfg)

	selector.mu.Lock()
	stateCountAfter := len(selector.state)
	selector.mu.Unlock()

	require.Equal(t, 0, stateCountAfter)

	for i := 7; i <= 10; i++ {
		selector.observe("default", "gpt-4o", i, true, cfg)
	}

	selector.mu.Lock()
	stateCountFinal := len(selector.state)
	selector.mu.Unlock()

	require.Equal(t, 4, stateCountFinal)
}

func TestChannelSuccessRateSelectorDecayLockedWithZeroHalfLife(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds: 0,
	}

	state := channelSuccessRateState{
		success: 10,
		failure: 5,
		updated: now.Add(-time.Hour),
	}

	decayed := selector.decayLocked(state, now, cfg)

	require.Equal(t, state.success, decayed.success)
	require.Equal(t, state.failure, decayed.failure)
	require.True(t, decayed.updated.Equal(state.updated))
}

func TestBuildChannelSuccessRateConfigFiltersEmptyErrorCodesAndTypes(t *testing.T) {
	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	original := *setting
	t.Cleanup(func() {
		*setting = original
	})

	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled: true,
		ImmediateDisable: operation_setting.ChannelSuccessRateImmediateDisableSetting{
			Enabled:     true,
			StatusCodes: []int{401},
			ErrorCodes:  []string{"  ", "valid_code", "", "  another  "},
			ErrorTypes:  []string{"", "  valid_type  ", "  "},
		},
	}

	cfg := buildChannelSuccessRateConfig()

	require.True(t, cfg.ImmediateDisableEnabled)
	_, ok := cfg.ImmediateDisableErrors["valid_code"]
	require.True(t, ok)
	_, ok = cfg.ImmediateDisableErrors["another"]
	require.True(t, ok)
	_, ok = cfg.ImmediateDisableErrors[""]
	require.False(t, ok)
	_, ok = cfg.ImmediateDisableErrors["  "]
	require.False(t, ok)

	_, ok = cfg.ImmediateDisableTypes["valid_type"]
	require.True(t, ok)
	_, ok = cfg.ImmediateDisableTypes[""]
	require.False(t, ok)
}

func TestGetChannelSuccessRateScoreUsesChannelPriority(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-score-priority-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-score-priority-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 930000

	priorityHigh := int64(10)
	priorityLow := int64(0)
	channelHigh := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, priorityHigh, common.ChannelStatusEnabled)
	channelLow := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, priorityLow, common.ChannelStatusEnabled)
	model.InitChannelCache()

	cfg := buildChannelSuccessRateConfig()
	cfg.PriorityWeights = map[int]float64{
		10: 0.2,
		0:  -0.1,
	}

	for i := 0; i < 5; i++ {
		defaultChannelSuccessRateSelector.observe(group, modelName, channelHigh.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, channelLow.Id, true, cfg)
	}

	scoreHigh := GetChannelSuccessRateScore(group, modelName, channelHigh.Id)
	scoreLow := GetChannelSuccessRateScore(group, modelName, channelLow.Id)

	require.Greater(t, scoreHigh, scoreLow)
}

func TestGetChannelSuccessRateRuntimeStateUsesChannelPriority(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-runtime-priority-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-runtime-priority-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 940000

	priorityLow := int64(0)
	channel := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, priorityLow, common.ChannelStatusEnabled)
	model.InitChannelCache()

	key := defaultChannelSuccessRateSelector.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), channel.Id)
	defaultChannelSuccessRateSelector.mu.Lock()
	defaultChannelSuccessRateSelector.state[key] = channelSuccessRateState{
		success:  9,
		failure:  1,
		updated:  time.Now(),
		observed: 10,
	}
	defaultChannelSuccessRateSelector.mu.Unlock()

	state := GetChannelSuccessRateRuntimeState(channel.Id)
	require.Greater(t, state.CurrentWeightedScore, 0.0)
}
