// FORK-CUSTOM: Verify manual clearing of temporary channel circuits.
package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestChannelSuccessRateSelectorClearTemporaryCircuitForChannel(t *testing.T) {
	setupChannelSuccessRateLifecycleTestDB(t)

	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)

	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:       1800,
		RecoveryCheckInterval: 10 * time.Minute,
		CircuitScope:          "model",
	}
	group := "default"
	modelName := "gpt-4o"
	targetID := 987001
	otherID := 987002

	targetKey := selector.circuitKeyForScope(successRateGroupKey(group), modelName, targetID, cfg.CircuitScope)
	otherKey := selector.circuitKeyForScope(successRateGroupKey(group), modelName, otherID, cfg.CircuitScope)
	openState := channelSuccessRateState{
		temporaryOpenUntil:  now.Add(10 * time.Minute),
		temporaryOpenReason: "连续失败触发临时熔断",
		updated:             now,
	}
	selector.circuitState[targetKey] = openState
	selector.circuitState[otherKey] = openState

	targetScoreKey := selector.scoreKey(successRateGroupKey(group), successRateModelKey(modelName), targetID)
	selector.state[targetScoreKey] = channelSuccessRateState{
		failure:          10,
		consecutiveFails: 5,
		observed:         10,
		updated:          now,
	}

	require.True(t, isTemporaryCircuitOpenAt(selector.circuitState[targetKey], now))

	selector.clearTemporaryCircuitForChannel(targetID)

	_, circuitExists := selector.circuitState[targetKey]
	require.False(t, circuitExists, "目标渠道的熔断状态应被清除")
	_, scoreExists := selector.state[targetScoreKey]
	require.False(t, scoreExists, "目标渠道的内存统计应被清除")

	require.True(t, isTemporaryCircuitOpenAt(selector.circuitState[otherKey], now), "其它渠道的熔断状态不应受影响")

	runtime := selector.GetRuntimeStateForChannel(targetID, cfg)
	require.False(t, runtime.TemporaryCircuitOpen, "清除后目标渠道应恢复正常状态")
}
