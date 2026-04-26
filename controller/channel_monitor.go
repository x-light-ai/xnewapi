package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetChannelMonitorSummary(c *gin.Context) {
	days, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("days", "7")))
	summary, err := model.GetChannelMonitorSummary(days)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func GetChannelMonitorHealth(c *gin.Context) {
	days, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("days", "7")))
	health, err := model.GetChannelMonitorHealth(days)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, health)
}

func GetChannelMonitorTimeline(c *gin.Context) {
	hours, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("hours", "24")))
	bucketMinutes, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("bucket_minutes", "10")))
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "20")))
	group := strings.TrimSpace(c.Query("group"))
	items, err := model.GetChannelMonitorTimelineByGroup(hours, bucketMinutes, limit, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func GetChannelMonitorRankings(c *gin.Context) {
	days, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("days", "1")))
	top, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("top", "10")))
	stabilityItems, latencyItems, err := model.GetChannelMonitorChannelRankings(days, top)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for i := range stabilityItems {
		runtimeState := service.GetChannelSuccessRateRuntimeState(stabilityItems[i].Id)
		stabilityItems[i].TemporaryCircuitOpen = runtimeState.TemporaryCircuitOpen
		stabilityItems[i].TemporaryCircuitUntil = runtimeState.TemporaryCircuitUntil
		stabilityItems[i].TemporaryCircuitReason = runtimeState.TemporaryCircuitReason
		stabilityItems[i].CurrentWeightedScore = runtimeState.CurrentWeightedScore
	}
	for i := range latencyItems {
		runtimeState := service.GetChannelSuccessRateRuntimeState(latencyItems[i].Id)
		latencyItems[i].TemporaryCircuitOpen = runtimeState.TemporaryCircuitOpen
		latencyItems[i].TemporaryCircuitUntil = runtimeState.TemporaryCircuitUntil
		latencyItems[i].TemporaryCircuitReason = runtimeState.TemporaryCircuitReason
		latencyItems[i].CurrentWeightedScore = runtimeState.CurrentWeightedScore
	}
	common.ApiSuccess(c, gin.H{
		"stability": stabilityItems,
		"latency":   latencyItems,
	})
}

func GetChannelMonitorChannels(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	if page, err := strconv.Atoi(strings.TrimSpace(c.Query("page"))); err == nil && page > 0 {
		pageInfo.Page = page
	}
	if pageInfo.Page < 1 {
		pageInfo.Page = 1
	}
	all, _ := strconv.ParseBool(strings.TrimSpace(c.DefaultQuery("all", "false")))
	if all {
		pageInfo.PageSize = 0
	} else {
		if pageInfo.PageSize <= 0 {
			pageInfo.PageSize = common.ItemsPerPage
		}
		if pageInfo.PageSize > 100 {
			pageInfo.PageSize = 100
		}
	}
	days, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("days", "7")))
	sortBy := strings.TrimSpace(c.DefaultQuery("sort", "request_count"))
	order := strings.TrimSpace(c.DefaultQuery("order", "desc"))
	group := strings.TrimSpace(c.Query("group"))
	items, total, err := model.GetChannelMonitorChannelPageByGroup(days, pageInfo.Page, pageInfo.PageSize, sortBy, order, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groups, err := model.GetChannelMonitorGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	for i := range items {
		runtimeState := service.GetChannelSuccessRateRuntimeState(items[i].Id)
		items[i].TemporaryCircuitOpen = runtimeState.TemporaryCircuitOpen
		items[i].TemporaryCircuitUntil = runtimeState.TemporaryCircuitUntil
		items[i].TemporaryCircuitReason = runtimeState.TemporaryCircuitReason
		items[i].CurrentWeightedScore = runtimeState.CurrentWeightedScore
	}
	common.ApiSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      pageInfo.Page,
		"page_size": pageInfo.PageSize,
		"groups":    groups,
	})
}

func GetChannelMonitorSelectionLogs(c *gin.Context) {
	channelID, _ := strconv.Atoi(strings.TrimSpace(c.Query("channel_id")))
	limit, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("limit", "100")))
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}
	abnormalOnly, _ := strconv.ParseBool(strings.TrimSpace(c.DefaultQuery("abnormal_only", "false")))
	common.ApiSuccess(c, service.ListChannelSelectionLogs(service.ChannelSelectionLogFilter{
		ChannelID:    channelID,
		ModelName:    strings.TrimSpace(c.Query("model")),
		Group:        strings.TrimSpace(c.Query("group")),
		Outcome:      strings.TrimSpace(c.Query("outcome")),
		AbnormalOnly: abnormalOnly,
		Limit:        limit,
	}))
}

func SetChannelScoreOverride(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var body struct {
		Score *float64 `json:"score"`
	}
	if err := common.DecodeJson(c.Request.Body, &body); err != nil {
		common.ApiError(c, err)
		return
	}
	service.SetChannelScoreOverride(id, body.Score)
	common.ApiSuccess(c, nil)
}
