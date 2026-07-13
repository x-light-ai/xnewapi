// FORK-CUSTOM: Keep fork-owned API routes out of the upstream route table.
package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func registerForkRoutes(apiRouter *gin.RouterGroup) {
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
