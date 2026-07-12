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
	registerForkRoutes(engine.Group("/api"))

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	expected := []string{
		"GET /api/channel_monitor/summary",
		"GET /api/channel_monitor/health",
		"GET /api/channel_monitor/timeline",
		"GET /api/channel_monitor/channels",
		"GET /api/channel_monitor/rankings",
		"POST /api/channel_monitor/channels/:id/score_override",
		"POST /api/channel_monitor/channels/:id/clear_circuit",
	}
	for _, route := range expected {
		require.Contains(t, routes, route)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/channel_monitor/summary", nil)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}
