package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelSuccessRateSelectorPickAppliesPriorityWeights(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds: 1800,
		PriorityWeights: map[int]float64{
			10: 0.2,
			0:  -0.1,
		},
	}
	priorityHigh := int64(10)
	priorityLow := int64(0)
	channels := []*model.Channel{
		{Id: 1, Priority: &priorityHigh},
		{Id: 2, Priority: &priorityLow},
	}

	selected := selector.pick("default", "gpt-4o", channels, cfg)
	require.NotNil(t, selected)
	require.Equal(t, 1, selected.Id)
}

func TestChannelSuccessRateSelectorObserveDetailedEmitsImmediateFailureEvent(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:         1800,
		ImmediateDisableEnabled: true,
		ImmediateDisableStatus: map[int]struct{}{
			http.StatusTooManyRequests: {},
		},
	}
	var event *channelFailureEvent
	selector.RegisterFailureCallback(func(got channelFailureEvent) {
		copied := got
		event = &copied
	})

	apiErr := types.NewErrorWithStatusCode(errors.New("rate limited"), types.ErrorCode("rate_limit_exceeded"), http.StatusTooManyRequests)
	selector.observeDetailed("default", "gpt-4o", 1, false, apiErr, cfg)

	require.NotNil(t, event)
	require.Equal(t, "immediate", event.Reason)
	require.Equal(t, http.StatusTooManyRequests, event.StatusCode)
	require.Equal(t, "rate_limit_exceeded", event.ErrorCode)
}

func TestChannelSuccessRateSelectorObserveDetailedEmitsImmediateFailureEventByErrorType(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:         1800,
		ImmediateDisableEnabled: true,
		ImmediateDisableTypes: map[string]struct{}{
			string(types.ErrorTypeClaudeError): {},
		},
	}
	var event *channelFailureEvent
	selector.RegisterFailureCallback(func(got channelFailureEvent) {
		copied := got
		event = &copied
	})

	apiErr := types.WithClaudeError(types.ClaudeError{Type: "billing_error", Message: "quota exhausted"}, http.StatusTooManyRequests)
	selector.observeDetailed("default", "gpt-4o", 1, false, apiErr, cfg)

	require.NotNil(t, event)
	require.Equal(t, "immediate", event.Reason)
	require.Equal(t, string(types.ErrorTypeClaudeError), event.ErrorType)
}

func TestChannelSuccessRateSelectorObserveDetailedEmitsConsecutiveFailureEventWithoutError(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ConsecutiveFailThreshold: 3,
	}
	key := selector.scoreKey(successRateGroupKey("default"), successRateModelKey("gpt-4o"), 1)
	selector.state[key] = channelSuccessRateState{
		updated:          now,
		consecutiveFails: 2,
		observed:         2,
	}
	var event *channelFailureEvent
	selector.RegisterFailureCallback(func(got channelFailureEvent) {
		copied := got
		event = &copied
	})

	selector.observeDetailed("default", "gpt-4o", 1, false, nil, cfg)

	require.NotNil(t, event)
	require.Equal(t, "consecutive", event.Reason)
	require.Zero(t, event.StatusCode)
	require.Empty(t, event.ErrorCode)
	require.Empty(t, event.ErrorMsg)
}

func TestBuildChannelSuccessRateConfigIncludesAdvancedSettings(t *testing.T) {
	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	original := *setting
	t.Cleanup(func() {
		*setting = original
	})

	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          900,
		ExploreRate:              0.15,
		QuickDowngrade:           false,
		ConsecutiveFailThreshold: 5,
		PriorityWeights: map[int]float64{
			10: 0.3,
			5:  -0.05,
		},
		ImmediateDisable: operation_setting.ChannelSuccessRateImmediateDisableSetting{
			Enabled:     true,
			StatusCodes: []int{401, 429},
			ErrorCodes:  []string{" insufficient_quota ", "rate_limit_exceeded"},
			ErrorTypes:  []string{" billing_error "},
		},
		HealthManager: operation_setting.ChannelSuccessRateHealthManagerSetting{
			CircuitScope:             "channel",
			DisableThreshold:         0.25,
			EnableThreshold:          0.8,
			MinSampleSize:            12,
			RecoveryCheckInterval:    120,
			HalfOpenSuccessThreshold: 3,
		},
	}

	cfg := buildChannelSuccessRateConfig()
	require.Equal(t, 900, cfg.HalfLifeSeconds)
	require.Equal(t, 0.15, cfg.ExploreRate)
	require.False(t, cfg.QuickDowngrade)
	require.Equal(t, 5, cfg.ConsecutiveFailThreshold)
	require.Equal(t, 0.3, cfg.PriorityWeights[10])
	require.Equal(t, -0.05, cfg.PriorityWeights[5])
	_, ok := cfg.ImmediateDisableStatus[401]
	require.True(t, ok)
	_, ok = cfg.ImmediateDisableErrors["insufficient_quota"]
	require.True(t, ok)
	_, ok = cfg.ImmediateDisableTypes["billing_error"]
	require.True(t, ok)
	require.Equal(t, 0.25, cfg.DisableThreshold)
	require.Equal(t, 0.8, cfg.EnableThreshold)
	require.Equal(t, 12, cfg.MinSampleSize)
	require.Equal(t, 120*time.Second, cfg.RecoveryCheckInterval)
	require.Equal(t, "channel", cfg.CircuitScope)
	require.Equal(t, 3, cfg.HalfOpenSuccessThreshold)
}

func TestRunChannelSuccessRateRecoveryCheckTransitionsToHalfOpen(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		RecoveryCheckInterval:    10 * time.Minute,
		HalfOpenSuccessThreshold: 2,
	}
	key := selector.circuitKeyForScope(successRateGroupKey("default"), "gpt-4o", 1, "model")
	selector.circuitState[key] = channelSuccessRateState{
		updated:             now.Add(-time.Minute),
		consecutiveFails:    4,
		observed:            7,
		temporaryOpenUntil:  now.Add(-time.Second),
		temporaryOpenReason: "连续失败触发临时熔断",
	}

	selector.clearExpiredTemporaryCircuits(cfg)

	state := selector.circuitState[key]
	require.True(t, state.temporaryOpenUntil.IsZero())
	require.Empty(t, state.temporaryOpenReason)
	require.Equal(t, 0, state.consecutiveFails)
	require.Equal(t, now.Add(cfg.RecoveryCheckInterval), state.halfOpenUntil)
	require.Equal(t, 2, state.halfOpenSuccessThreshold)
}

func TestChannelSuccessRateSelectorHalfOpenNeedsConsecutiveSuccesses(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		RecoveryCheckInterval:    10 * time.Minute,
		HalfOpenSuccessThreshold: 2,
	}
	scoreKey := selector.scoreKey(successRateGroupKey("default"), successRateModelKey("gpt-4o"), 1)
	circuitKey := selector.circuitKeyForScope(successRateGroupKey("default"), "gpt-4o", 1, "model")
	selector.circuitState[circuitKey] = channelSuccessRateState{
		updated:                  now,
		halfOpenUntil:            now.Add(cfg.RecoveryCheckInterval),
		halfOpenReason:           "进入半开探测",
		halfOpenSuccessThreshold: 2,
		halfOpenProbeInFlight:    true,
	}

	selector.observe("default", "gpt-4o", 1, true, cfg)
	state := selector.circuitState[circuitKey]
	require.Equal(t, 1, state.halfOpenSuccesses)
	require.False(t, state.halfOpenUntil.IsZero())
	require.Equal(t, 1, selector.state[scoreKey].observed)

	selector.circuitState[circuitKey] = channelSuccessRateState{
		updated:                  now,
		halfOpenUntil:            now.Add(cfg.RecoveryCheckInterval),
		halfOpenReason:           state.halfOpenReason,
		halfOpenSuccessThreshold: 2,
		halfOpenSuccesses:        state.halfOpenSuccesses,
		halfOpenProbeInFlight:    true,
	}
	selector.observe("default", "gpt-4o", 1, true, cfg)
	state = selector.circuitState[circuitKey]
	require.Zero(t, state.halfOpenSuccesses)
	require.True(t, state.halfOpenUntil.IsZero())
}

func TestChannelSuccessRateSelectorPickSkipsHalfOpenAfterProbeAllocated(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		RecoveryCheckInterval:    10 * time.Minute,
		HalfOpenSuccessThreshold: 2,
	}
	key := selector.circuitKeyForScope(successRateGroupKey("default"), "gpt-4o", 11, "model")
	selector.circuitState[key] = channelSuccessRateState{
		updated:                  now,
		halfOpenUntil:            now.Add(cfg.RecoveryCheckInterval),
		halfOpenReason:           "进入半开探测",
		halfOpenSuccessThreshold: 2,
		halfOpenSuccesses:        1,
		halfOpenProbeInFlight:    true,
	}
	channels := []*model.Channel{{Id: 11}, {Id: 22}}

	selected := selector.pick("default", "gpt-4o", channels, cfg)
	require.NotNil(t, selected)
	require.Equal(t, 22, selected.Id)
}

func TestChannelSuccessRateSelectorChannelScopeSharesCircuitAcrossModels(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		CircuitScope:             "channel",
		RecoveryCheckInterval:    10 * time.Minute,
		HalfOpenSuccessThreshold: 2,
	}
	key := selector.circuitKeyForScope(successRateGroupKey("default"), "gpt-4o", 7, cfg.CircuitScope)
	selector.circuitState[key] = channelSuccessRateState{
		updated:             now,
		temporaryOpenUntil:  now.Add(time.Minute),
		temporaryOpenReason: "渠道级熔断",
	}
	selector.state[selector.scoreKey(successRateGroupKey("default"), successRateModelKey("gpt-4o"), 7)] = channelSuccessRateState{success: 5, observed: 5}
	selector.state[selector.scoreKey(successRateGroupKey("default"), successRateModelKey("claude-3-7-sonnet"), 7)] = channelSuccessRateState{success: 1, observed: 1}

	state, recovered := selector.decayLockedForKey(key, selector.circuitState[key], now, cfg)
	require.False(t, recovered)
	require.True(t, isTemporaryCircuitOpenAt(state, now))
	sharedKey := selector.circuitKeyForScope(successRateGroupKey("default"), "claude-3-7-sonnet", 7, cfg.CircuitScope)
	require.Equal(t, key, sharedKey)
	require.NotEqual(t,
		selector.scoreKey(successRateGroupKey("default"), successRateModelKey("gpt-4o"), 7),
		selector.scoreKey(successRateGroupKey("default"), successRateModelKey("claude-3-7-sonnet"), 7),
	)
	require.Equal(t, 5, selector.state[selector.scoreKey(successRateGroupKey("default"), successRateModelKey("gpt-4o"), 7)].observed)
	require.Equal(t, 1, selector.state[selector.scoreKey(successRateGroupKey("default"), successRateModelKey("claude-3-7-sonnet"), 7)].observed)
}


func TestChannelSuccessRateSelectorHalfOpenTimeoutReopensTemporaryCircuit(t *testing.T) {
	now := time.Unix(1710000000, 0)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		RecoveryCheckInterval:    10 * time.Minute,
		HalfOpenSuccessThreshold: 2,
	}
	state := transitionCircuitStateOnTime(channelSuccessRateState{
		halfOpenUntil:            now.Add(-time.Second),
		halfOpenReason:           "进入半开探测",
		halfOpenSuccesses:        1,
		halfOpenSuccessThreshold: 2,
		halfOpenProbeInFlight:    true,
	}, now, cfg)

	require.True(t, state.halfOpenUntil.IsZero())
	require.Equal(t, now.Add(cfg.RecoveryCheckInterval), state.temporaryOpenUntil)
	require.Equal(t, "半开探测超时，重新进入临时熔断", state.temporaryOpenReason)
	require.Zero(t, state.halfOpenSuccesses)
	require.False(t, state.halfOpenProbeInFlight)
}

func TestObserveChannelRequestResultFallsBackToUsingGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
	channelID := int(time.Now().UnixNano()%1_000_000_000) + 200000
	t.Cleanup(func() {
		defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, channelID)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "fallback-group")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-4o")

	ObserveChannelRequestResult(ctx, true, nil)

	snapshot := model.GetChannelRuntimeStats("fallback-group", "gpt-4o", channelID)
	require.EqualValues(t, 1, snapshot.RequestCount)
	require.EqualValues(t, 1, snapshot.SuccessCount)
}
