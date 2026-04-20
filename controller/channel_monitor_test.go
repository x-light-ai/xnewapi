package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type channelMonitorAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type channelMonitorSummaryResponse struct {
	TotalRequests  int64   `json:"total_requests"`
	SuccessRate    float64 `json:"success_rate"`
	AvgLatencyMs   float64 `json:"avg_latency"`
	ActiveChannels int64   `json:"active_channels"`
	TotalChannels  int64   `json:"total_channels"`
	TimeRange      string  `json:"time_range"`
}

type channelMonitorPageResponse struct {
	Items    []model.ChannelMonitorChannelItem `json:"items"`
	Total    int64                             `json:"total"`
	Page     int                               `json:"page"`
	PageSize int                               `json:"page_size"`
}

func setupChannelMonitorControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	model.ResetChannelMonitorRuntimeStateForTest()
	service.ResetChannelSuccessRateHealthManagerForTest()

	if err = db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.ChannelMonitorStat{}, &model.ChannelCircuitEvent{}); err != nil {
		t.Fatalf("failed to migrate channel monitor tables: %v", err)
	}

	t.Cleanup(func() {
		model.ResetChannelMonitorRuntimeStateForTest()
		service.ResetChannelSuccessRateHealthManagerForTest()
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedChannelMonitorControllerChannel(t *testing.T, db *gorm.DB, name string, channelType int, status int) *model.Channel {
	t.Helper()

	channel := &model.Channel{
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

func seedChannelMonitorControllerStat(t *testing.T, db *gorm.DB, channelID int, bucket time.Time, requestCount int64, successCount int64, failureCount int64, avgLatencyMs float64, p95LatencyMs float64, lastActiveAt time.Time) {
	t.Helper()

	stat := &model.ChannelMonitorStat{
		ChannelID:    channelID,
		GroupName:    "default",
		ModelName:    "gpt-4",
		TimeBucket:   bucket,
		Granularity:  model.ChannelMonitorGranularityMinute,
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

func newChannelMonitorRequest(t *testing.T, method string, target string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, nil)
	return ctx, recorder
}

func decodeChannelMonitorResponse(t *testing.T, recorder *httptest.ResponseRecorder) channelMonitorAPIResponse {
	t.Helper()

	var response channelMonitorAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func seedChannelMonitorControllerPriorityChannel(t *testing.T, db *gorm.DB, id int, group string, modelName string, priority int64, status int) *model.Channel {
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
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create priority channel %d: %v", id, err)
	}
	if err := channel.AddAbilities(nil); err != nil {
		t.Fatalf("failed to add abilities for channel %d: %v", id, err)
	}
	t.Cleanup(func() {
		_ = model.DB.Where("channel_id = ?", id).Delete(&model.Ability{}).Error
		_ = model.DB.Delete(&model.Channel{}, "id = ?", id).Error
	})
	return channel
}

func TestGetChannelMonitorSummaryHandlerReturnsAggregatedData(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)
	now := time.Now()
	channel := seedChannelMonitorControllerChannel(t, db, "alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	seedChannelMonitorControllerStat(t, db, channel.Id, now.Add(-time.Hour).Truncate(time.Minute), 8, 6, 2, 120, 180, now.Add(-2*time.Minute))

	ctx, recorder := newChannelMonitorRequest(t, http.MethodGet, "/api/channel_monitor/summary?days=7")
	GetChannelMonitorSummary(ctx)

	response := decodeChannelMonitorResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var summary channelMonitorSummaryResponse
	if err := common.Unmarshal(response.Data, &summary); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if summary.TotalRequests != 8 {
		t.Fatalf("expected total requests 8, got %d", summary.TotalRequests)
	}
	if summary.SuccessRate != 0.75 {
		t.Fatalf("expected success rate 0.75, got %v", summary.SuccessRate)
	}
	if summary.AvgLatencyMs != 120 {
		t.Fatalf("expected avg latency 120, got %v", summary.AvgLatencyMs)
	}
	if summary.ActiveChannels != 1 || summary.TotalChannels != 1 {
		t.Fatalf("unexpected channel counts: %+v", summary)
	}
}

func TestGetChannelMonitorSummaryHandlerReturnsZeroValuesWhenEmpty(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)
	_ = seedChannelMonitorControllerChannel(t, db, "alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)

	ctx, recorder := newChannelMonitorRequest(t, http.MethodGet, "/api/channel_monitor/summary?days=7")
	GetChannelMonitorSummary(ctx)

	response := decodeChannelMonitorResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var summary channelMonitorSummaryResponse
	if err := common.Unmarshal(response.Data, &summary); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if summary.TotalRequests != 0 || summary.SuccessRate != 0 || summary.AvgLatencyMs != 0 {
		t.Fatalf("expected zero-value summary, got %+v", summary)
	}
	if summary.ActiveChannels != 0 || summary.TotalChannels != 1 {
		t.Fatalf("unexpected channel counts: %+v", summary)
	}
}

func TestGetChannelMonitorHealthHandlerReturnsDailyHealthData(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := dayStart.AddDate(0, 0, -1).Add(9 * time.Hour)
	today := dayStart.Add(10 * time.Hour)

	alpha := seedChannelMonitorControllerChannel(t, db, "Alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	beta := seedChannelMonitorControllerChannel(t, db, "Beta", constant.ChannelTypeAnthropic, common.ChannelStatusEnabled)

	seedChannelMonitorControllerStat(t, db, alpha.Id, yesterday, 10, 9, 1, 100, 120, yesterday)
	seedChannelMonitorControllerStat(t, db, beta.Id, yesterday, 10, 4, 6, 300, 360, yesterday)
	seedChannelMonitorControllerStat(t, db, alpha.Id, today, 4, 4, 0, 120, 140, today)

	ctx, recorder := newChannelMonitorRequest(t, http.MethodGet, "/api/channel_monitor/health?days=2")
	GetChannelMonitorHealth(ctx)

	response := decodeChannelMonitorResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var health []model.ChannelMonitorHealthPoint
	if err := common.Unmarshal(response.Data, &health); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if len(health) != 2 {
		t.Fatalf("expected 2 health points, got %d", len(health))
	}
	if health[0].Date != dayStart.AddDate(0, 0, -1).Format("2006-01-02") {
		t.Fatalf("unexpected first date %s", health[0].Date)
	}
	if health[0].TotalRequests != 20 || health[0].SuccessRate != 0.65 || health[0].AvgLatencyMs != 200 {
		t.Fatalf("unexpected first day aggregate: %+v", health[0])
	}
	if health[0].HealthyChannels != 1 || health[0].WarningChannels != 0 || health[0].ErrorChannels != 1 {
		t.Fatalf("unexpected first day channel states: %+v", health[0])
	}
	if health[1].Date != dayStart.Format("2006-01-02") {
		t.Fatalf("unexpected second date %s", health[1].Date)
	}
	if health[1].TotalRequests != 4 || health[1].SuccessRate != 1 || health[1].AvgLatencyMs != 120 {
		t.Fatalf("unexpected second day aggregate: %+v", health[1])
	}
	if health[1].HealthyChannels != 1 || health[1].WarningChannels != 0 || health[1].ErrorChannels != 0 {
		t.Fatalf("unexpected second day channel states: %+v", health[1])
	}
}

func TestGetChannelMonitorHealthHandlerReturnsEmptyBucketsWhenNoData(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)
	_ = seedChannelMonitorControllerChannel(t, db, "Alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)

	ctx, recorder := newChannelMonitorRequest(t, http.MethodGet, "/api/channel_monitor/health?days=2")
	GetChannelMonitorHealth(ctx)

	response := decodeChannelMonitorResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var health []model.ChannelMonitorHealthPoint
	if err := common.Unmarshal(response.Data, &health); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if len(health) != 2 {
		t.Fatalf("expected 2 health points, got %d", len(health))
	}
	for _, item := range health {
		if item.TotalRequests != 0 || item.SuccessRate != 0 || item.AvgLatencyMs != 0 {
			t.Fatalf("expected zero metrics, got %+v", item)
		}
		if item.HealthyChannels != 0 || item.WarningChannels != 0 || item.ErrorChannels != 0 {
			t.Fatalf("expected zero statuses, got %+v", item)
		}
	}
}

func TestGetChannelMonitorChannelsHandlerReturnsEmptyMetricsWhenNoData(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)
	channel := seedChannelMonitorControllerChannel(t, db, "Alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)

	ctx, recorder := newChannelMonitorRequest(t, http.MethodGet, "/api/channel_monitor/channels?page=1&page_size=20&days=2&sort=request_count&order=desc")
	GetChannelMonitorChannels(ctx)

	response := decodeChannelMonitorResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page channelMonitorPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode channel page response: %v", err)
	}
	if page.Page != 1 || page.PageSize != 20 {
		t.Fatalf("unexpected pagination: %+v", page)
	}
	if page.Total != 1 {
		t.Fatalf("expected total 1, got %d", page.Total)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page.Items))
	}
	if page.Items[0].Id != channel.Id {
		t.Fatalf("expected alpha item, got %+v", page.Items[0])
	}
	if page.Items[0].RequestCount != 0 || page.Items[0].FailureCount != 0 || page.Items[0].SuccessRate != 0 || page.Items[0].AvgLatencyMs != 0 || page.Items[0].P95LatencyMs != 0 {
		t.Fatalf("expected zero metrics, got %+v", page.Items[0])
	}
	if len(page.Items[0].HealthTrend) != 2 || page.Items[0].HealthTrend[0] != 0 || page.Items[0].HealthTrend[1] != 0 {
		t.Fatalf("expected zero trend, got %+v", page.Items[0].HealthTrend)
	}
	if page.Items[0].TemporaryCircuitOpen {
		t.Fatalf("expected temporary circuit closed, got %+v", page.Items[0])
	}
	if page.Items[0].CurrentWeightedScore != 0 {
		t.Fatalf("expected zero runtime score, got %+v", page.Items[0])
	}
}

func TestGetChannelMonitorChannelsHandlerShowsAutoDisabledChannelStatus(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)
	now := time.Now()
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -1).Add(9 * time.Hour)

	autoDisabled := seedChannelMonitorControllerChannel(t, db, "AutoDisabled", constant.ChannelTypeOpenAI, common.ChannelStatusAutoDisabled)
	seedChannelMonitorControllerStat(t, db, autoDisabled.Id, yesterday, 5, 2, 3, 220, 260, yesterday)
	model.InitChannelCache()
	service.StartChannelSuccessRateHealthManager()

	recorder := httptest.NewRecorder()
	observeCtx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(observeCtx, constant.ContextKeyChannelId, autoDisabled.Id)
	common.SetContextKey(observeCtx, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(observeCtx, constant.ContextKeyOriginalModel, "gpt-4")
	service.ObserveChannelRequestResult(observeCtx, false, types.NewErrorWithStatusCode(errors.New("upstream failed"), "", http.StatusInternalServerError))

	ctx, recorder := newChannelMonitorRequest(t, http.MethodGet, "/api/channel_monitor/channels?page=1&page_size=20&days=2&sort=request_count&order=desc")
	GetChannelMonitorChannels(ctx)

	response := decodeChannelMonitorResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page channelMonitorPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode channel page response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("unexpected auto-disabled page payload: %+v", page)
	}
	if page.Items[0].Id != autoDisabled.Id {
		t.Fatalf("expected auto-disabled channel item, got %+v", page.Items[0])
	}
	if page.Items[0].Status != common.ChannelStatusAutoDisabled {
		t.Fatalf("expected auto-disabled status %d, got %d", common.ChannelStatusAutoDisabled, page.Items[0].Status)
	}
	if page.Items[0].RequestCount != 6 || page.Items[0].FailureCount != 4 {
		t.Fatalf("unexpected auto-disabled metrics: %+v", page.Items[0])
	}
	if !page.Items[0].TemporaryCircuitOpen {
		t.Fatalf("expected temporary circuit open, got %+v", page.Items[0])
	}
	if page.Items[0].TemporaryCircuitReason == "" {
		t.Fatalf("expected temporary circuit reason, got %+v", page.Items[0])
	}
	if page.Items[0].CurrentWeightedScore <= 0 {
		t.Fatalf("expected positive runtime score, got %+v", page.Items[0])
	}
}

func TestGetChannelMonitorChannelsHandlerMergesRuntimeStateAndMetrics(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)
	now := time.Now()
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -1).Add(9 * time.Hour)

	channel := seedChannelMonitorControllerChannel(t, db, "Alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	seedChannelMonitorControllerStat(t, db, channel.Id, yesterday, 8, 6, 2, 120, 180, yesterday)
	model.InitChannelCache()

	service.StartChannelSuccessRateHealthManager()
	recorder := httptest.NewRecorder()
	observeCtx, _ := gin.CreateTestContext(recorder)
	common.SetContextKey(observeCtx, constant.ContextKeyChannelId, channel.Id)
	common.SetContextKey(observeCtx, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(observeCtx, constant.ContextKeyOriginalModel, "gpt-4")
	common.SetContextKey(observeCtx, constant.ContextKeyRequestStartTime, time.Now().Add(-150*time.Millisecond))
	service.ObserveChannelRequestResult(observeCtx, false, types.NewErrorWithStatusCode(errors.New("upstream failed"), "", http.StatusInternalServerError))

	ctx, pageRecorder := newChannelMonitorRequest(t, http.MethodGet, "/api/channel_monitor/channels?page=1&page_size=20&days=2&sort=request_count&order=desc")
	GetChannelMonitorChannels(ctx)

	response := decodeChannelMonitorResponse(t, pageRecorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page channelMonitorPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode channel page response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("unexpected page payload: %+v", page)
	}

	item := page.Items[0]
	if item.Id != channel.Id {
		t.Fatalf("expected channel %d, got %+v", channel.Id, item)
	}
	if item.RequestCount != 9 || item.FailureCount != 3 {
		t.Fatalf("unexpected merged counters: %+v", item)
	}
	if item.SuccessRate < 0.6 || item.SuccessRate > 0.7 {
		t.Fatalf("unexpected merged success rate: %+v", item)
	}
	if item.AvgLatencyMs <= 120 {
		t.Fatalf("expected merged avg latency above persisted value, got %+v", item)
	}
	if !item.TemporaryCircuitOpen {
		t.Fatalf("expected temporary circuit open, got %+v", item)
	}
	if item.TemporaryCircuitReason == "" {
		t.Fatalf("expected temporary circuit reason, got %+v", item)
	}
	if item.CurrentWeightedScore <= 0 {
		t.Fatalf("expected positive weighted score, got %+v", item)
	}
	if item.LastActiveAt.IsZero() {
		t.Fatalf("expected last active time, got %+v", item)
	}
}

func TestRelaySelectionFlowSamePriorityFailoverAndPriorityFallback(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)

	setting := operation_setting.GetChannelSuccessRateSetting()
	if setting == nil {
		t.Fatal("expected channel success rate setting")
	}
	originalSetting := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
		ImmediateDisable: operation_setting.ChannelSuccessRateImmediateDisableSetting{
			Enabled:     true,
			StatusCodes: []int{500},
		},
		HealthManager: operation_setting.ChannelSuccessRateHealthManagerSetting{
			CircuitScope:             "model",
			DisableThreshold:         0.2,
			EnableThreshold:          0.7,
			MinSampleSize:            1,
			RecoveryCheckInterval:    600,
			HalfOpenSuccessThreshold: 2,
		},
	}
	t.Cleanup(func() {
		*setting = originalSetting
	})

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRetryTimes := common.RetryTimes
	originalErrorLogEnabled := constant.ErrorLogEnabled
	common.MemoryCacheEnabled = true
	common.RetryTimes = 3
	constant.ErrorLogEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RetryTimes = originalRetryTimes
		constant.ErrorLogEnabled = originalErrorLogEnabled
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("relay-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-4-relay-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 980000

	a := seedChannelMonitorControllerPriorityChannel(t, db, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	b := seedChannelMonitorControllerPriorityChannel(t, db, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	c := seedChannelMonitorControllerPriorityChannel(t, db, baseID+3, group, modelName, 5, common.ChannelStatusEnabled)
	model.InitChannelCache()
	service.StartChannelSuccessRateHealthManager()

	ctx, _ := newChannelMonitorRequest(t, http.MethodPost, "/v1/chat/completions")
	ctx.Set("group", group)
	ctx.Set("token_id", 1)
	ctx.Set("token_name", "test-token")
	ctx.Set("id", 1)
	ctx.Set("auto_ban", false)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, modelName)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, group)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, group)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, group)

	relayInfo := &relaycommon.RelayInfo{
		TokenGroup:       group,
		UsingGroup:       group,
		UserGroup:        group,
		OriginModelName:  modelName,
		RelayFormat:      types.RelayFormatOpenAI,
		PriceData:        types.PriceData{},
		ChannelMeta:      &relaycommon.ChannelMeta{},
	}
	retryParam := &service.RetryParam{Ctx: ctx, TokenGroup: group, ModelName: modelName, Retry: common.GetPointer(0)}

	channel, channelErr := getChannel(ctx, relayInfo, retryParam)
	if channelErr != nil {
		t.Fatalf("expected first channel without error, got %v", channelErr)
	}
	if channel.Id != a.Id {
		t.Fatalf("expected first selected channel %d, got %d", a.Id, channel.Id)
	}
	addUsedChannel(ctx, channel.Id)
	service.ObserveChannelRequestResult(ctx, false, types.NewErrorWithStatusCode(errors.New("upstream failed"), "", http.StatusInternalServerError))

	if !shouldRetry(ctx, types.NewErrorWithStatusCode(errors.New("upstream failed"), "", http.StatusInternalServerError), common.RetryTimes-retryParam.GetRetry()) {
		t.Fatal("expected retry after first channel failure")
	}
	selectedB, selectedGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		t.Fatalf("expected second channel selection, got %v", err)
	}
	if selectedGroup != group || selectedB == nil || selectedB.Id != b.Id {
		t.Fatalf("expected same-priority failover to channel %d in group %s, got channel=%+v group=%s", b.Id, group, selectedB, selectedGroup)
	}
	_ = db
	addUsedChannel(ctx, selectedB.Id)
	newRetry := 1
	retryParam.Retry = &newRetry
	common.SetContextKey(ctx, constant.ContextKeyChannelId, selectedB.Id)
	service.ObserveChannelRequestResult(ctx, false, types.NewErrorWithStatusCode(errors.New("still failed"), "", http.StatusInternalServerError))

	selectedC, selectedGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		t.Fatalf("expected lower-priority channel selection, got %v", err)
	}
	if selectedGroup != group || selectedC == nil || selectedC.Id != c.Id {
		t.Fatalf("expected fallback to lower-priority channel %d in group %s, got channel=%+v group=%s", c.Id, group, selectedC, selectedGroup)
	}

	usedChannels := ctx.GetStringSlice("use_channel")
	if len(usedChannels) != 2 || usedChannels[0] != fmt.Sprintf("%d", a.Id) || usedChannels[1] != fmt.Sprintf("%d", b.Id) {
		t.Fatalf("unexpected used channel trace: %+v", usedChannels)
	}

	statsA := model.GetChannelRuntimeStats(group, modelName, a.Id)
	statsB := model.GetChannelRuntimeStats(group, modelName, b.Id)
	statsC := model.GetChannelRuntimeStats(group, modelName, c.Id)
	if statsA.FailureCount != 1 || statsB.FailureCount != 1 || statsC.RequestCount != 0 {
		t.Fatalf("unexpected runtime stats: a=%+v b=%+v c=%+v", statsA, statsB, statsC)
	}

	stateA := service.GetChannelSuccessRateRuntimeState(a.Id)
	stateB := service.GetChannelSuccessRateRuntimeState(b.Id)
	stateC := service.GetChannelSuccessRateRuntimeState(c.Id)
	if !stateA.TemporaryCircuitOpen || !stateB.TemporaryCircuitOpen || stateC.TemporaryCircuitOpen {
		t.Fatalf("unexpected runtime state: a=%+v b=%+v c=%+v", stateA, stateB, stateC)
	}
}

func TestRelaySelectionFlowHalfOpenRecoveryReentersSelection(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)

	setting := operation_setting.GetChannelSuccessRateSetting()
	if setting == nil {
		t.Fatal("expected channel success rate setting")
	}
	originalSetting := *setting
	*setting = operation_setting.ChannelSuccessRateSetting{
		Enabled:                  true,
		HalfLifeSeconds:          1800,
		ExploreRate:              0,
		QuickDowngrade:           true,
		ConsecutiveFailThreshold: 3,
		ImmediateDisable: operation_setting.ChannelSuccessRateImmediateDisableSetting{
			Enabled:     true,
			StatusCodes: []int{500},
		},
		HealthManager: operation_setting.ChannelSuccessRateHealthManagerSetting{
			CircuitScope:             "model",
			DisableThreshold:         0.2,
			EnableThreshold:          0.7,
			MinSampleSize:            1,
			RecoveryCheckInterval:    60,
			HalfOpenSuccessThreshold: 2,
		},
	}
	t.Cleanup(func() {
		*setting = originalSetting
	})

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		service.SetChannelSuccessRateNowForTest(nil)
	})

	suffix := time.Now().UnixNano()
	group := fmt.Sprintf("relay-half-open-group-%d", suffix)
	modelName := fmt.Sprintf("gpt-4-half-open-%d", suffix)
	baseID := int(suffix%1_000_000_000) + 990000
	openedAt := time.Unix(1710000000, 0)
	currentNow := openedAt

	a := seedChannelMonitorControllerPriorityChannel(t, db, baseID+1, group, modelName, 10, common.ChannelStatusEnabled)
	b := seedChannelMonitorControllerPriorityChannel(t, db, baseID+2, group, modelName, 10, common.ChannelStatusEnabled)
	model.InitChannelCache()
	service.SetChannelSuccessRateNowForTest(func() time.Time { return currentNow })
	service.StartChannelSuccessRateHealthManager()

	ctx, _ := newChannelMonitorRequest(t, http.MethodPost, "/v1/chat/completions")
	ctx.Set("group", group)
	ctx.Set("token_id", 1)
	ctx.Set("token_name", "test-token")
	ctx.Set("id", 1)
	ctx.Set("auto_ban", false)
	common.SetContextKey(ctx, constant.ContextKeyOriginalModel, modelName)
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, group)
	common.SetContextKey(ctx, constant.ContextKeyTokenGroup, group)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, group)

	relayInfo := &relaycommon.RelayInfo{
		TokenGroup:      group,
		UsingGroup:      group,
		UserGroup:       group,
		OriginModelName: modelName,
		RelayFormat:     types.RelayFormatOpenAI,
		PriceData:       types.PriceData{},
		ChannelMeta:     &relaycommon.ChannelMeta{},
	}
	retryParam := &service.RetryParam{Ctx: ctx, TokenGroup: group, ModelName: modelName, Retry: common.GetPointer(0)}

	channel, channelErr := getChannel(ctx, relayInfo, retryParam)
	if channelErr != nil {
		t.Fatalf("expected first channel without error, got %v", channelErr)
	}
	if channel.Id != a.Id {
		t.Fatalf("expected first selected channel %d, got %d", a.Id, channel.Id)
	}
	addUsedChannel(ctx, channel.Id)
	service.ObserveChannelRequestResult(ctx, false, types.NewErrorWithStatusCode(errors.New("probe failed"), "", http.StatusInternalServerError))

	stateA := service.GetChannelSuccessRateRuntimeState(a.Id)
	if !stateA.TemporaryCircuitOpen {
		t.Fatalf("expected channel A temporary circuit open, got %+v", stateA)
	}

	selectedB, selectedGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		t.Fatalf("expected fallback channel selection, got %v", err)
	}
	if selectedGroup != group || selectedB == nil || selectedB.Id != b.Id {
		t.Fatalf("expected fallback to channel %d, got channel=%+v group=%s", b.Id, selectedB, selectedGroup)
	}

	currentNow = currentNow.Add(61 * time.Second)
	service.ClearExpiredChannelSuccessRateCircuitsForTest()

	stateA = service.GetChannelSuccessRateRuntimeState(a.Id)
	if stateA.TemporaryCircuitOpen || !stateA.HalfOpen {
		t.Fatalf("expected channel A in half-open state, got %+v", stateA)
	}

	ctx.Set("use_channel", []string{fmt.Sprintf("%d", b.Id)})
	retryParam.Retry = common.GetPointer(0)
	selectedA, selectedGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		t.Fatalf("expected half-open channel reselected, got %v", err)
	}
	if selectedGroup != group || selectedA == nil || selectedA.Id != a.Id {
		t.Fatalf("expected channel A reselected in half-open, got channel=%+v group=%s", selectedA, selectedGroup)
	}

	common.SetContextKey(ctx, constant.ContextKeyChannelId, a.Id)
	service.ObserveChannelRequestResult(ctx, true, nil)
	stateA = service.GetChannelSuccessRateRuntimeState(a.Id)
	if !stateA.HalfOpen || stateA.HalfOpenSuccesses != 1 {
		t.Fatalf("expected half-open success progress, got %+v", stateA)
	}

	ctx.Set("use_channel", []string{fmt.Sprintf("%d", b.Id)})
	selectedA, selectedGroup, err = service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		t.Fatalf("expected second half-open probe selection, got %v", err)
	}
	if selectedGroup != group || selectedA == nil || selectedA.Id != a.Id {
		t.Fatalf("expected channel A second probe, got channel=%+v group=%s", selectedA, selectedGroup)
	}

	common.SetContextKey(ctx, constant.ContextKeyChannelId, a.Id)
	service.ObserveChannelRequestResult(ctx, true, nil)
	stateA = service.GetChannelSuccessRateRuntimeState(a.Id)
	if stateA.TemporaryCircuitOpen || stateA.HalfOpen {
		t.Fatalf("expected channel A fully recovered, got %+v", stateA)
	}

	statsA := model.GetChannelRuntimeStats(group, modelName, a.Id)
	statsB := model.GetChannelRuntimeStats(group, modelName, b.Id)
	if statsA.RequestCount != 3 || statsA.SuccessCount != 2 || statsA.FailureCount != 1 {
		t.Fatalf("unexpected channel A runtime stats: %+v", statsA)
	}
	if statsB.RequestCount != 0 {
		t.Fatalf("expected channel B stats unchanged by selection-only fallback, got %+v", statsB)
	}
}

func TestGetChannelMonitorChannelsHandlerNormalizesPagination(t *testing.T) {
	db := setupChannelMonitorControllerTestDB(t)
	now := time.Now()
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -1).Add(9 * time.Hour)

	alpha := seedChannelMonitorControllerChannel(t, db, "Alpha", constant.ChannelTypeOpenAI, common.ChannelStatusEnabled)
	beta := seedChannelMonitorControllerChannel(t, db, "Beta", constant.ChannelTypeAnthropic, common.ChannelStatusManuallyDisabled)

	seedChannelMonitorControllerStat(t, db, alpha.Id, yesterday, 10, 10, 0, 100, 120, yesterday)
	seedChannelMonitorControllerStat(t, db, beta.Id, yesterday, 10, 4, 6, 300, 360, yesterday)

	ctx, recorder := newChannelMonitorRequest(t, http.MethodGet, "/api/channel_monitor/channels?page=0&page_size=999&days=2&sort=name&order=asc")
	GetChannelMonitorChannels(ctx)

	response := decodeChannelMonitorResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page channelMonitorPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode channel page response: %v", err)
	}
	if page.Page != 1 {
		t.Fatalf("expected normalized page 1, got %d", page.Page)
	}
	if page.PageSize != 100 {
		t.Fatalf("expected normalized page size 100, got %d", page.PageSize)
	}
	if page.Total != 2 {
		t.Fatalf("expected total 2, got %d", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page.Items))
	}
	if page.Items[0].Name != "Alpha" || page.Items[1].Name != "Beta" {
		t.Fatalf("expected ascending name sort, got %+v", page.Items)
	}
}
