// FORK-CUSTOM: Verify fork-owned routes remain registered and admin-protected.
package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterForkRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("fork-route-test"))))
	apiRouter := engine.Group("/api")
	registerChannelRoutes(apiRouter)
	registerForkRoutes(apiRouter)

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	expected := []string{
		"GET /api/xnewapi/channel-monitor/summary",
		"GET /api/xnewapi/channel-monitor/health",
		"GET /api/xnewapi/channel-monitor/timeline",
		"GET /api/xnewapi/channel-monitor/channels",
		"GET /api/xnewapi/channel-monitor/rankings",
		"POST /api/xnewapi/channel-monitor/channels/:id/score_override",
		"POST /api/xnewapi/channel-monitor/channels/:id/clear_circuit",
	}
	for _, route := range expected {
		require.Contains(t, routes, route)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/xnewapi/channel-monitor/summary", nil)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)

	redirectRecorder := httptest.NewRecorder()
	redirectRequest := httptest.NewRequest(http.MethodGet, "/api/channel", nil)
	engine.ServeHTTP(redirectRecorder, redirectRequest)
	require.Equal(t, http.StatusMovedPermanently, redirectRecorder.Code)
	require.Equal(t, "/api/channel/", redirectRecorder.Header().Get("Location"))
}
