package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelAffinityHitPrefersCachedChannelOverSuccessRateSelection(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)
	gin.SetMode(gin.TestMode)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	originalSuccessRateSetting := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = originalSuccessRateSetting
	})

	affinitySetting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, affinitySetting)
	originalAffinitySetting := *affinitySetting
	affinitySetting.Enabled = true
	t.Cleanup(func() {
		*affinitySetting = originalAffinitySetting
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("affinity-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-5-affinity-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 400000

	affinityChannel := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	successRateChannel := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	cfg := buildChannelSuccessRateConfig()
	for i := 0; i < 5; i++ {
		defaultChannelSuccessRateSelector.observe(group, modelName, successRateChannel.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, affinityChannel.Id, false, cfg)
	}

	var codexRule *operation_setting.ChannelAffinityRule
	for i := range affinitySetting.Rules {
		rule := &affinitySetting.Rules[i]
		if strings.EqualFold(strings.TrimSpace(rule.Name), "codex cli trace") {
			codexRule = rule
			break
		}
	}
	require.NotNil(t, codexRule)

	affinityValue := fmt.Sprintf("affinity-hit-%d", suffix)
	cacheKeySuffix := buildChannelAffinityCacheKeySuffix(*codexRule, modelName, group, affinityValue)
	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(cacheKeySuffix, affinityChannel.Id, time.Minute))
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(fmt.Sprintf(`{"prompt_cache_key":"%s"}`, affinityValue)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	preferredChannelID, found := GetPreferredChannelByAffinity(ctx, modelName, group)
	require.True(t, found)
	require.Equal(t, affinityChannel.Id, preferredChannelID)
	require.True(t, model.IsChannelEnabledForGroupModel(group, modelName, preferredChannelID))

	preferred, err := model.CacheGetChannel(preferredChannelID)
	require.NoError(t, err)
	require.NotNil(t, preferred)
	require.Equal(t, affinityChannel.Id, preferred.Id)

	fallback, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group,
		ModelName:  modelName,
	})
	require.NoError(t, err)
	require.NotNil(t, fallback)
	require.Equal(t, group, selectedGroup)
	require.Equal(t, successRateChannel.Id, fallback.Id)

	MarkChannelAffinityUsed(ctx, group, preferred.Id)
	require.True(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))

	anyInfo, ok := ctx.Get(ginKeyChannelAffinityLogInfo)
	require.True(t, ok)
	info, ok := anyInfo.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, codexRule.Name, info["rule_name"])
	require.Equal(t, group, info["selected_group"])
	require.Equal(t, affinityChannel.Id, info["channel_id"])
}

func TestChannelAffinityMissFallsBackToSuccessRateSelection(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)
	gin.SetMode(gin.TestMode)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	originalSuccessRateSetting := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = originalSuccessRateSetting
	})

	affinitySetting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, affinitySetting)
	originalAffinitySetting := *affinitySetting
	affinitySetting.Enabled = true
	t.Cleanup(func() {
		*affinitySetting = originalAffinitySetting
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("affinity-miss-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-5-affinity-miss-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 500000

	affinityChannel := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	successRateChannel := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	cfg := buildChannelSuccessRateConfig()
	for i := 0; i < 5; i++ {
		defaultChannelSuccessRateSelector.observe(group, modelName, successRateChannel.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, affinityChannel.Id, false, cfg)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"prompt_cache_key":"affinity-miss"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	preferredChannelID, found := GetPreferredChannelByAffinity(ctx, modelName, group)
	require.False(t, found)
	require.Zero(t, preferredChannelID)

	fallback, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: group,
		ModelName:  modelName,
	})
	require.NoError(t, err)
	require.NotNil(t, fallback)
	require.Equal(t, group, selectedGroup)
	require.Equal(t, successRateChannel.Id, fallback.Id)
}

func TestChannelAffinityRecordedMappingPersistsAcrossRequests(t *testing.T) {
	prepareSuccessRateSelectionIntegrationTest(t)
	gin.SetMode(gin.TestMode)

	setting := operation_setting.GetChannelSuccessRateSetting()
	require.NotNil(t, setting)
	originalSuccessRateSetting := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
	}
	t.Cleanup(func() {
		*setting = originalSuccessRateSetting
	})

	affinitySetting := operation_setting.GetChannelAffinitySetting()
	require.NotNil(t, affinitySetting)
	originalAffinitySetting := *affinitySetting
	affinitySetting.Enabled = true
	t.Cleanup(func() {
		*affinitySetting = originalAffinitySetting
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("affinity-persist-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-5-affinity-persist-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 800000

	affinityChannel := seedSuccessRateIntegrationChannel(t, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	successRateChannel := seedSuccessRateIntegrationChannel(t, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()

	cfg := buildChannelSuccessRateConfig()
	for i := 0; i < 5; i++ {
		defaultChannelSuccessRateSelector.observe(group, modelName, successRateChannel.Id, true, cfg)
		defaultChannelSuccessRateSelector.observe(group, modelName, affinityChannel.Id, false, cfg)
	}

	var codexRule *operation_setting.ChannelAffinityRule
	for i := range affinitySetting.Rules {
		rule := &affinitySetting.Rules[i]
		if strings.EqualFold(strings.TrimSpace(rule.Name), "codex cli trace") {
			codexRule = rule
			break
		}
	}
	require.NotNil(t, codexRule)

	affinityValue := fmt.Sprintf("affinity-persist-%d", suffix)
	cacheKeySuffix := buildChannelAffinityCacheKeySuffix(*codexRule, modelName, group, affinityValue)
	cache := getChannelAffinityCache()
	t.Cleanup(func() {
		_, _ = cache.DeleteMany([]string{cacheKeySuffix})
	})

	firstRec := httptest.NewRecorder()
	firstCtx, _ := gin.CreateTestContext(firstRec)
	firstCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(fmt.Sprintf(`{"prompt_cache_key":"%s"}`, affinityValue)))
	firstCtx.Request.Header.Set("Content-Type", "application/json")

	preferredChannelID, found := GetPreferredChannelByAffinity(firstCtx, modelName, group)
	require.False(t, found)
	require.Zero(t, preferredChannelID)

	fallback, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        firstCtx,
		TokenGroup: group,
		ModelName:  modelName,
	})
	require.NoError(t, err)
	require.NotNil(t, fallback)
	require.Equal(t, group, selectedGroup)
	require.Equal(t, successRateChannel.Id, fallback.Id)

	RecordChannelAffinity(firstCtx, affinityChannel.Id)

	secondRec := httptest.NewRecorder()
	secondCtx, _ := gin.CreateTestContext(secondRec)
	secondCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(fmt.Sprintf(`{"prompt_cache_key":"%s"}`, affinityValue)))
	secondCtx.Request.Header.Set("Content-Type", "application/json")

	preferredSecond, found := GetPreferredChannelByAffinity(secondCtx, modelName, group)
	require.True(t, found)
	require.Equal(t, affinityChannel.Id, preferredSecond)

	thirdRec := httptest.NewRecorder()
	thirdCtx, _ := gin.CreateTestContext(thirdRec)
	thirdCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(fmt.Sprintf(`{"prompt_cache_key":"%s"}`, affinityValue)))
	thirdCtx.Request.Header.Set("Content-Type", "application/json")

	preferredThird, found := GetPreferredChannelByAffinity(thirdCtx, modelName, group)
	require.True(t, found)
	require.Equal(t, affinityChannel.Id, preferredThird)

	fallbackAgain, selectedGroup, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        thirdCtx,
		TokenGroup: group,
		ModelName:  modelName,
	})
	require.NoError(t, err)
	require.NotNil(t, fallbackAgain)
	require.Equal(t, group, selectedGroup)
	require.Equal(t, successRateChannel.Id, fallbackAgain.Id)
}
