package service

import (
	"errors"
	"fmt"
	"math"
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

type fixedSuccessRateRNG struct {
	float64Value float64
	intnValue    int
}

func (r fixedSuccessRateRNG) Float64() float64 { return r.float64Value }
func (r fixedSuccessRateRNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	if r.intnValue < 0 {
		return 0
	}
	return r.intnValue % n
}

func newTestChannelSuccessRateSelector(now time.Time) *channelSuccessRateSelector {
	return &channelSuccessRateSelector{
		maxKeys:      64,
		now:          func() time.Time { return now },
		rng:          fixedSuccessRateRNG{},
		state:        make(map[string]channelSuccessRateState),
		circuitState: make(map[string]channelSuccessRateState),
		rrCursors:    make(map[string]int),
	}
}

func buildSuccessRateTestChannels(ids ...int) []*model.Channel {
	channels := make([]*model.Channel, 0, len(ids))
	for _, id := range ids {
		channels = append(channels, &model.Channel{Id: id})
	}
	return channels
}

func prepareSuccessRateSelectionIntegrationTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NotNil(t, model.DB)
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}))

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalSelector := defaultChannelSuccessRateSelector
	common.MemoryCacheEnabled = true
	defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
	model.InitChannelCache()

	t.Cleanup(func() {
		defaultChannelSuccessRateSelector = originalSelector
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		model.InitChannelCache()
	})
}

func seedSuccessRateIntegrationRootUser(t *testing.T, suffix int64) *model.User {
	t.Helper()
	user := &model.User{
		Username: fmt.Sprintf("root-user-%d", suffix),
		Password: "password123",
		DisplayName: fmt.Sprintf("Root %d", suffix),
		Role: common.RoleRootUser,
		Status: common.UserStatusEnabled,
		Email: fmt.Sprintf("root-%d@example.com", suffix),
		Group: "default",
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func seedSuccessRateIntegrationChannel(t *testing.T, id int, group string, modelName string, priority int64, status int) *model.Channel {
	t.Helper()
	priorityValue := priority
	channel := &model.Channel{
		Id:          id,
		Name:        fmt.Sprintf("channel-%d", id),
		Type:        constant.ChannelTypeOpenAI,
		Key:         fmt.Sprintf("channel-key-%d", id),
		Status:      status,
		Group:       group,
		Models:      modelName,
		Priority:    &priorityValue,
		CreatedTime: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))

	t.Cleanup(func() {
		_ = model.DB.Where("channel_id = ?", id).Delete(&model.Ability{}).Error
		_ = model.DB.Delete(&model.Channel{}, "id = ?", id).Error
	})
	return channel
}

func TestChannelSuccessRateSelectorPickPrefersHigherScore(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	group := "default"
	modelName := "gpt-4o"
	channels := buildSuccessRateTestChannels(1, 2)

	selector.observe(group, modelName, 1, true, cfg)
	selector.observe(group, modelName, 1, true, cfg)
	selector.observe(group, modelName, 2, false, cfg)

	selected := selector.pick(group, modelName, channels, cfg)
	require.NotNil(t, selected)
	require.Equal(t, 1, selected.Id)
}

func TestChannelSuccessRateSelectorPickUsesRoundRobinOnTie(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	channels := buildSuccessRateTestChannels(11, 22)

	first := selector.pick("default", "gpt-4o", channels, cfg)
	second := selector.pick("default", "gpt-4o", channels, cfg)
	third := selector.pick("default", "gpt-4o", channels, cfg)

	require.NotNil(t, first)
	require.NotNil(t, second)
	require.NotNil(t, third)
	require.Equal(t, 11, first.Id)
	require.Equal(t, 22, second.Id)
	require.Equal(t, 11, third.Id)
}

func TestChannelSuccessRateSelectorExplorePathUsesRNG(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	selector.rng = fixedSuccessRateRNG{float64Value: 0.01, intnValue: 1}
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0.5,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	channels := buildSuccessRateTestChannels(11, 22)

	selected := selector.pick("default", "gpt-4o", channels, cfg)
	require.NotNil(t, selected)
	require.Equal(t, 22, selected.Id)
}

func TestChannelSuccessRateSelectorObserveQuickDowngradeAndReset(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	key := selector.scoreKey(successRateGroupKey("default"), successRateModelKey("gpt-4o"), 1)

	selector.observe("default", "gpt-4o", 1, false, cfg)
	selector.observe("default", "gpt-4o", 1, false, cfg)
	state := selector.state[key]
	require.Equal(t, 4.0, state.failure)
	require.Equal(t, 2, state.consecutiveFails)
	require.Equal(t, 2, state.observed)

	selector.observe("default", "gpt-4o", 1, true, cfg)
	state = selector.state[key]
	require.Equal(t, 1.0, state.success)
	require.Equal(t, 0, state.consecutiveFails)
	require.Equal(t, 3, state.observed)
}

func TestChannelSuccessRateSelectorScoreAppliesConsecutiveFailPenalty(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           false,
		ConsecutiveFailThreshold: 3,
	}
	key := selector.scoreKey(successRateGroupKey("default"), successRateModelKey("gpt-4o"), 7)
	selector.state[key] = channelSuccessRateState{
		success:          4,
		failure:          1,
		updated:          now,
		consecutiveFails: 3,
	}

	score := selector.getScore("default", "gpt-4o", 7, cfg)
	expected := ((4.0 + 1) / (4.0 + 1 + 2)) / 5.0
	require.InDelta(t, expected, score, 1e-12)
}

func TestChannelSuccessRateSelectorDecayLocked(t *testing.T) {
	now := time.Unix(1710003600, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	decayed := selector.decayLocked(channelSuccessRateState{
		success: 8,
		failure: 4,
		updated: now.Add(-30 * time.Minute),
	}, now, cfg)

	require.InDelta(t, 4.0, decayed.success, 1e-12)
	require.InDelta(t, 2.0, decayed.failure, 1e-12)
	require.True(t, decayed.updated.Equal(now))
}

func TestChannelSuccessRateSelectorSampleSizeCountsObservedRequests(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}

	selector.observe("default", "gpt-4o", 9, false, cfg)
	selector.observe("default", "gpt-4o", 9, true, cfg)

	require.Equal(t, 2, selector.getSampleSize("default", "gpt-4o", 9))
}

func TestSelectBySuccessRateUsesRetryTierAndSkipsDisabledCandidates(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	original := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = original
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-sr-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 200000

	enabledHigh := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	disabledHigh := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusManuallyDisabled)
	autoDisabledHigh := seedSuccessRateIntegrationChannel(t, baseID+3, group, modelName, 10, common.ChannelStatusAutoDisabled)
	otherGroup := seedSuccessRateIntegrationChannel(t, baseID+4, group+"-other", modelName, 10, common.ChannelStatusEnabled)
	otherModel := seedSuccessRateIntegrationChannel(t, baseID+5, group, modelName+"-other", 10, common.ChannelStatusEnabled)
	fallback := seedSuccessRateIntegrationChannel(t, baseID+6, group, modelName, 5, common.ChannelStatusEnabled)
	model.InitChannelCache()

	cfg := buildChannelSuccessRateConfig()
	for i := 0; i < 4; i++ {
		defaultChannelSuccessRateSelector.observe(group, modelName, disabledHigh.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, autoDisabledHigh.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group+"-other", modelName, otherGroup.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName+"-other", otherModel.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, fallback.Id, true, cfg)
	}

	selectedHigh, err := SelectBySuccessRate(nil, group, modelName, 0)
	require.NoError(t, err)
	require.NotNil(t, selectedHigh)
	require.Equal(t, enabledHigh.Id, selectedHigh.Id)

	selectedFallback, err := SelectBySuccessRate(nil, group, modelName, 1)
	require.NoError(t, err)
	require.NotNil(t, selectedFallback)
	require.Equal(t, fallback.Id, selectedFallback.Id)
}

func TestSelectBySuccessRateShiftsPreferenceAfterObservedSuccessAndFailure(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	original := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = original
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-shift-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-shift-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 600000

	candidateA := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	candidateB := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	cfg := buildChannelSuccessRateConfig()
	for i := 0; i < 4; i++ {
		defaultChannelSuccessRateSelector.observe(group, modelName, candidateA.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, candidateB.Id, false, cfg)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	selectedBeforeShift, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group,
		ModelName:  modelName,
	})
	require.NoError(t, err)
	require.NotNil(t, selectedBeforeShift)
	require.Equal(t, candidateA.Id, selectedBeforeShift.Id)

	for i := 0; i < 5; i++ {
		defaultChannelSuccessRateSelector.observe(group, modelName, candidateA.Id, false, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, candidateB.Id, true, cfg)
	}

	selectedAfterShift, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group,
		ModelName:  modelName,
	})
	require.NoError(t, err)
	require.NotNil(t, selectedAfterShift)
	require.Equal(t, candidateB.Id, selectedAfterShift.Id)
	require.Greater(t, GetChannelSuccessRateScore(group, modelName, candidateB.Id), GetChannelSuccessRateScore(group, modelName, candidateA.Id))
}

func TestDisableChannelAutoBanRemovesChannelFromSelection(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	originalSetting := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = originalSetting
	})

	originalAutoDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = originalAutoDisable
	})

	suffix := time.Now().UnixNano()
	rootUser := seedSuccessRateIntegrationRootUser(t, suffix)
	t.Cleanup(func() {
		_ = model.DB.Delete(&model.User{}, "id = ?", rootUser.Id).Error
	})

	group := fmt.Sprintf("sr-autoban-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-autoban-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 700000

	primary := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	fallback := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	cfg := buildChannelSuccessRateConfig()
	for i := 0; i < 4; i++ {
		defaultChannelSuccessRateSelector.observe(group, modelName, primary.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, fallback.Id, false, cfg)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	selectedBeforeDisable, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group,
		ModelName:  modelName,
	})
	require.NoError(t, err)
	require.NotNil(t, selectedBeforeDisable)
	require.Equal(t, primary.Id, selectedBeforeDisable.Id)

	channelErr := types.NewChannelError(primary.Id, primary.Type, primary.Name, false, primary.Key, true)
	apiErr := types.NewErrorWithStatusCode(errors.New("channel upstream failure"), types.ErrorCode("channel:upstream_failure"), http.StatusBadGateway)
	require.True(t, ShouldDisableChannel(apiErr))

	DisableChannel(*channelErr, apiErr.Error())

	updatedPrimary, err := model.CacheGetChannel(primary.Id)
	require.NoError(t, err)
	require.NotNil(t, updatedPrimary)
	require.Equal(t, common.ChannelStatusAutoDisabled, updatedPrimary.Status)
	require.False(t, model.IsChannelEnabledForGroupModel(group, modelName, primary.Id))

	var abilities []model.Ability
	require.NoError(t, model.DB.Where("channel_id = ?", primary.Id).Find(&abilities).Error)
	require.NotEmpty(t, abilities)
	for _, ability := range abilities {
		require.False(t, ability.Enabled)
	}

	selectedAfterDisable, _, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group,
		ModelName:  modelName,
	})
	require.NoError(t, err)
	require.NotNil(t, selectedAfterDisable)
	require.Equal(t, fallback.Id, selectedAfterDisable.Id)
}

func TestObserveChannelRequestResultFallsBackToTokenGroupAndUserGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
	t.Cleanup(func() {
		defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
	})

	t.Run("token group", func(t *testing.T) {
		channelID := int(time.Now().UnixNano()%1_000_000_000) + 300000
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		common.SetContextKey(ctx, constant.ContextKeyChannelId, channelID)
		common.SetContextKey(ctx, constant.ContextKeyTokenGroup, "token-fallback")
		common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-4o")

		ObserveChannelRequestResult(ctx, true, nil)

		snapshot := model.GetChannelRuntimeStats("token-fallback", "gpt-4o", channelID)
		require.EqualValues(t, 1, snapshot.RequestCount)
		require.EqualValues(t, 1, snapshot.SuccessCount)
	})

	t.Run("user group", func(t *testing.T) {
		channelID := int(time.Now().UnixNano()%1_000_000_000) + 400000
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		common.SetContextKey(ctx, constant.ContextKeyChannelId, channelID)
		common.SetContextKey(ctx, constant.ContextKeyUserGroup, "user-fallback")
		common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-4o")

		ObserveChannelRequestResult(ctx, true, nil)

		snapshot := model.GetChannelRuntimeStats("user-fallback", "gpt-4o", channelID)
		require.EqualValues(t, 1, snapshot.RequestCount)
		require.EqualValues(t, 1, snapshot.SuccessCount)
	})
}

func TestChannelSuccessRateSelectorExplorePathSkipsTemporaryCircuitChannels(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	selector.rng = fixedSuccessRateRNG{float64Value: 0.01, intnValue: 0}
	cfg := channelSuccessRateConfig{
		HalfLifeSeconds: 1800,
		ExploreRate:     0.5,
	}
	channels := []*model.Channel{{Id: 11, Name: "blocked"}, {Id: 22, Name: "healthy"}}
	key := selector.circuitKeyForScope(successRateGroupKey("default"), "gpt-4o", 11, cfg.CircuitScope)
	selector.circuitState[key] = channelSuccessRateState{
		updated:            now,
		temporaryOpenUntil: now.Add(time.Minute),
		temporaryOpenReason: "temporary open",
	}

	selected, selectedScore, others := selector.pickDetailed("default", "gpt-4o", channels, cfg)
	require.NotNil(t, selected)
	require.Equal(t, 22, selected.Id)
	require.Equal(t, selector.getScore("default", "gpt-4o", 22, cfg), selectedScore)
	require.Len(t, others, 1)
	require.Equal(t, 11, others[0].channelID)
	require.Contains(t, others[0].reason, "temporary open")
}

func TestGetChannelSuccessRateRuntimeStatePrefersMostObservedState(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	suffix := time.Now().UnixNano()
	channel := seedSuccessRateIntegrationChannel(t, int(suffix%1_000_000_000)+950000, "default", "gpt-4o", 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	keyA := defaultChannelSuccessRateSelector.scoreKey(successRateGroupKey("group-a"), successRateModelKey("model-a"), channel.Id)
	keyB := defaultChannelSuccessRateSelector.scoreKey(successRateGroupKey("group-b"), successRateModelKey("model-b"), channel.Id)
	defaultChannelSuccessRateSelector.mu.Lock()
	defaultChannelSuccessRateSelector.state[keyA] = channelSuccessRateState{success: 2, failure: 1, updated: time.Now(), observed: 3}
	defaultChannelSuccessRateSelector.state[keyB] = channelSuccessRateState{success: 4, failure: 1, updated: time.Now(), observed: 8}
	defaultChannelSuccessRateSelector.mu.Unlock()

	state := GetChannelSuccessRateRuntimeState(channel.Id)
	require.Equal(t, 8, state.Observed)
	require.Equal(t, "group-b", state.Group)
	require.Equal(t, "model-b", state.ModelName)
}

func TestObserveChannelRequestResultUsesContextFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
	channelID := int(time.Now().UnixNano()%1_000_000_000) + 100000
	t.Cleanup(func() {
		defaultChannelSuccessRateSelector = newChannelSuccessRateSelector()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(ctx, constant.ContextKeyChannelId, channelID)
	common.SetContextKey(ctx, constant.ContextKeyAutoGroup, "auto-a")
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-4o")
	common.SetContextKey(ctx, constant.ContextKeyRequestStartTime, time.Now().Add(-50*time.Millisecond))

	ObserveChannelRequestResult(ctx, false, nil)

	snapshot := model.GetChannelRuntimeStats("auto-a", "gpt-4o", channelID)
	require.EqualValues(t, 1, snapshot.RequestCount)
	require.EqualValues(t, 1, snapshot.FailureCount)
	require.Zero(t, snapshot.SuccessCount)
	require.GreaterOrEqual(t, snapshot.LastLatencyMs, 1.0)
	require.False(t, snapshot.LastActiveAt.IsZero())

	score := GetChannelSuccessRateScore("auto-a", "gpt-4o", channelID)
	require.True(t, score > 0)
	require.True(t, score < 0.5)
	require.Equal(t, 1, GetChannelSuccessRateSampleSize("auto-a", "gpt-4o", channelID))
}

func TestBuildChannelSuccessRateConfigClampsValues(t *testing.T) {
	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	original := *setting
	t.Cleanup(func() {
		*setting = original
	})

	setting.HalfLifeSeconds = 0
	setting.ExploreRate = 2.5
	setting.QuickDowngrade = false
	setting.ConsecutiveFailThreshold = 0

	cfg := buildChannelSuccessRateConfig()
	require.Equal(t, 1800, cfg.HalfLifeSeconds)
	require.Equal(t, 1.0, cfg.ExploreRate)
	require.False(t, cfg.QuickDowngrade)
	require.Equal(t, 3, cfg.ConsecutiveFailThreshold)
	for _, code := range []int{400, 401, 403, 404, 500, 502} {
		_, ok := cfg.ImmediateDisableStatus[code]
		require.True(t, ok)
	}
	for _, code := range []int{408, 429, 501, 503, 504, 522, 524} {
		_, ok := cfg.ImmediateDisableStatus[code]
		require.False(t, ok)
	}

	setting.ExploreRate = -1
	cfg = buildChannelSuccessRateConfig()
	require.Equal(t, 0.0, cfg.ExploreRate)
}

func TestSelectChannelWithSuccessRateDoesNotFallbackToUsedChannel(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	original := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = original
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-used-channel-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-used-channel-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 800000

	primary := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	secondary := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set("use_channel", []string{fmt.Sprintf("%d", primary.Id)})

	selected, err := selectChannelWithSuccessRate(ctx, group, modelName, 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, secondary.Id, selected.Id)

	ctx.Set("use_channel", []string{fmt.Sprintf("%d", primary.Id), fmt.Sprintf("%d", secondary.Id)})
	selected, err = selectChannelWithSuccessRate(ctx, group, modelName, 0)
	require.NoError(t, err)
	require.Nil(t, selected)
}

func TestCacheGetRandomSatisfiedChannelRetryZeroStillMovesToNextChannelInSameGroup(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	originalSetting := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = originalSetting
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-retry-zero-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-retry-zero-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 810000

	first := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	second := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Set("use_channel", []string{fmt.Sprintf("%d", first.Id)})

	selected, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group,
		ModelName:  modelName,
		Retry:      common.GetPointer(0),
	})
	require.NoError(t, err)
	require.Equal(t, group, selectedGroup)
	require.NotNil(t, selected)
	require.Equal(t, second.Id, selected.Id)
}

func TestSelectChannelWithSuccessRateExhaustsHigherPriorityBeforeLowerPriority(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	originalSetting := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = originalSetting
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("sr-stage-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-4o-stage-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 820000

	a := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	b := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	c := seedSuccessRateIntegrationChannel(t, baseID+3, group, modelName, 5, common.ChannelStatusEnabled)
	d := seedSuccessRateIntegrationChannel(t, baseID+4, group, modelName, 0, common.ChannelStatusEnabled)
	model.InitChannelCache()

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)

	selected, err := selectChannelWithSuccessRate(ctx, group, modelName, 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	firstHighID := selected.Id
	require.Contains(t, []int{a.Id, b.Id}, firstHighID)

	used := []string{fmt.Sprintf("%d", firstHighID)}
	ctx.Set("use_channel", used)
	selected, err = selectChannelWithSuccessRate(ctx, group, modelName, 1)
	require.NoError(t, err)
	require.NotNil(t, selected)
	secondHighID := selected.Id
	require.Contains(t, []int{a.Id, b.Id}, secondHighID)
	require.NotEqual(t, firstHighID, secondHighID)

	used = append(used, fmt.Sprintf("%d", secondHighID))
	ctx.Set("use_channel", used)
	selected, err = selectChannelWithSuccessRate(ctx, group, modelName, 2)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, c.Id, selected.Id)

	used = append(used, fmt.Sprintf("%d", c.Id))
	ctx.Set("use_channel", used)
	selected, err = selectChannelWithSuccessRate(ctx, group, modelName, 3)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, d.Id, selected.Id)

	used = append(used, fmt.Sprintf("%d", d.Id))
	ctx.Set("use_channel", used)
	selected, err = selectChannelWithSuccessRate(ctx, group, modelName, 4)
	require.NoError(t, err)
	require.Nil(t, selected)
}

func TestSuccessRateGroupKeyNormalizesCaseAndSpaces(t *testing.T) {
	require.Equal(t, "default", successRateGroupKey("  DeFaUlT  "))
	require.Equal(t, "gpt-4o", successRateModelKey("  gpt-4o  "))
}

func TestChannelSuccessRateSelectorGetScoreReturnsZeroForInvalidInput(t *testing.T) {
	selector := newTestChannelSuccessRateSelector(time.Unix(1710000000, 0))
	cfg := buildChannelSuccessRateConfig()
	require.Zero(t, selector.getScore("default", "gpt-4o", 0, cfg))
	require.Zero(t, selector.getSampleSize("default", "gpt-4o", 0))
}

func TestChannelSuccessRateSelectorDecayLockedKeepsFutureStateUntouched(t *testing.T) {
	now := time.Unix(1710000000, 0)
	selector := newTestChannelSuccessRateSelector(now)
	cfg := channelSuccessRateConfig{HalfLifeSeconds: 1800}
	state := channelSuccessRateState{success: 2, failure: 1, updated: now.Add(time.Minute)}
	decayed := selector.decayLocked(state, now, cfg)
	require.True(t, math.Abs(decayed.success-state.success) < 1e-12)
	require.True(t, math.Abs(decayed.failure-state.failure) < 1e-12)
	require.True(t, decayed.updated.Equal(state.updated))
}
