// FORK-CUSTOM: Keep fork-owned API routes out of the upstream route table.
package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func registerForkRoutes(apiRouter *gin.RouterGroup) {
	providerRoute := apiRouter.Group("/xnewapi/providers")
	providerRoute.Use(middleware.AdminAuth())
	{
		providerRoute.GET("", controller.GetUpstreamProviders)
		providerRoute.PUT("/workspace", controller.SaveUpstreamProviderWorkspace)
		providerRoute.GET("/channels", controller.GetUpstreamProviderChannelOptions)
		providerRoute.GET("/profit-ranking", controller.GetUpstreamProviderProfitRanking)
		providerRoute.GET("/profit-details", controller.GetUpstreamProviderProfitDetails)
		providerRoute.POST("/profit/rebuild", controller.RebuildUpstreamProviderProfit)
		providerRoute.GET("/sync-runs", controller.GetUpstreamProviderSyncRuns)
		providerRoute.POST("/:id/sync", controller.SyncUpstreamProvider)
		providerRoute.POST("/accounts/:account_id/sync", controller.SyncUpstreamProviderAccount)
		providerRoute.POST("/accounts/:account_id/recharge-adjust", controller.AdjustUpstreamProviderAccountRecharge)
		providerRoute.DELETE("/:id", controller.DeleteUpstreamProvider)
		providerRoute.DELETE("/accounts/:account_id", controller.DeleteUpstreamProviderAccount)
		providerRoute.DELETE("/groups/:group_id", controller.DeleteUpstreamProviderGroup)
	}

	channelMonitorRoute := apiRouter.Group("/xnewapi/channel-monitor")
	channelMonitorRoute.Use(middleware.AdminAuth())
	{
		channelMonitorRoute.GET("/summary", controller.GetChannelMonitorSummary)
		channelMonitorRoute.GET("/health", controller.GetChannelMonitorHealth)
		channelMonitorRoute.GET("/timeline", controller.GetChannelMonitorTimeline)
		channelMonitorRoute.GET("/channels", controller.GetChannelMonitorChannels)
		channelMonitorRoute.GET("/rankings", controller.GetChannelMonitorRankings)
		channelMonitorRoute.POST("/channels/:id/score_override", controller.SetChannelScoreOverride)
		channelMonitorRoute.POST("/channels/:id/clear_circuit", controller.ClearChannelTemporaryCircuit)
	}
}
