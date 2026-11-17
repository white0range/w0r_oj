package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"gojo/config"
)

func TestAgentRoutesAreIndependentOfAdminAndUserAuth(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = config.Config{}
	config.GlobalConfig.Chat.AgentServiceToken = "test-service-secret"
	gin.SetMode(gin.TestMode)
	// No handler or database is needed: rejected credentials must stop before either.
	r := SetupRouter(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, path := range []string{
		"/api/agent/context",
		"/api/agent/me/ac-history",
		"/api/agent/me/failed-submissions",
		"/api/agent/me/failed-submissions/1",
		"/api/agent/me/tag-stats",
		"/api/agent/problems/candidates",
		"/api/agent/problems/1",
	} {
		t.Run(path, func(t *testing.T) {
			for _, serviceKey := range []string{"", "wrong", "test-service-secret"} {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.Header.Set("Authorization", "Bearer user-jwt")
				req.Header.Set("X-Agent-Service-Token", serviceKey)
				res := httptest.NewRecorder()
				r.ServeHTTP(res, req)
				want := http.StatusUnauthorized
				if serviceKey == "test-service-secret" {
					want = http.StatusBadRequest
				} // Missing turn, service authentication passed.
				if res.Code != want {
					t.Fatalf("status=%d, want=%d", res.Code, want)
				}
			}
		})
	}
	for _, path := range []string{
		"/api/admin/agent/users/42/ac-history",
		"/api/admin/agent/users/42/failed-submissions",
		"/api/admin/agent/users/42/failed-submissions/1",
		"/api/admin/agent/users/42/tag-stats",
		"/api/admin/agent/problems/candidates",
		"/api/admin/agent/problems/1",
	} {
		res := httptest.NewRecorder()
		r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, path, nil))
		if res.Code != http.StatusNotFound {
			t.Errorf("old route %s still exists: %d", path, res.Code)
		}
	}
	for _, path := range []string{"/api/profile", "/api/admin/users"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Agent-Service-Token", "test-service-secret")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Errorf("service key must not grant user/admin access: %s status=%d", path, res.Code)
		}
	}
}
