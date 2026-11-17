package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"gojo/config"
	"gojo/internal/chat/model"
)

type agentTurnStub struct {
	turn       *model.ChatTurn
	session    *model.ChatSession
	turnErr    error
	sessionErr error
	lookups    int
}

func (s *agentTurnStub) GetTurnByID(context.Context, uint) (*model.ChatTurn, error) {
	s.lookups++
	return s.turn, s.turnErr
}

func (s *agentTurnStub) GetSessionByID(context.Context, uint) (*model.ChatSession, error) {
	return s.session, s.sessionErr
}

func TestAgentAuthAndTurnBoundary(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name          string
		configuredKey string
		providedKey   string
		turnID        string
		mutate        func(*agentTurnStub)
		wantStatus    int
		wantLookups   int
	}{
		{name: "missing key", configuredKey: "service-secret", turnID: "87", wantStatus: 401},
		{name: "wrong key", configuredKey: "service-secret", providedKey: "wrong", turnID: "87", wantStatus: 401},
		{name: "empty configuration fails closed", turnID: "87", wantStatus: 401},
		{name: "missing turn", configuredKey: "service-secret", providedKey: "service-secret", wantStatus: 400},
		{name: "zero turn", configuredKey: "service-secret", providedKey: "service-secret", turnID: "0", wantStatus: 400},
		{name: "negative turn", configuredKey: "service-secret", providedKey: "service-secret", turnID: "-1", wantStatus: 400},
		{name: "unknown turn", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.turnErr = gorm.ErrRecordNotFound }, wantStatus: 403, wantLookups: 1},
		{name: "turn database error", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.turnErr = errors.New("unavailable") }, wantStatus: 500, wantLookups: 1},
		{name: "pending turn", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.turn.Status = model.TaskStatusPending }, wantStatus: 403, wantLookups: 1},
		{name: "finished turn", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.turn.Status = model.TaskStatusSucceeded }, wantStatus: 403, wantLookups: 1},
		{name: "missing lease", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.turn.LeaseExpiresAt = nil }, wantStatus: 403, wantLookups: 1},
		{name: "expired lease", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { expired := time.Now().Add(-time.Minute); s.turn.LeaseExpiresAt = &expired }, wantStatus: 403, wantLookups: 1},
		{name: "unknown session", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.session = nil; s.sessionErr = gorm.ErrRecordNotFound }, wantStatus: 403, wantLookups: 1},
		{name: "session database error", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.sessionErr = errors.New("unavailable") }, wantStatus: 500, wantLookups: 1},
		{name: "archived session", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.session.Status = model.ChatSessionStatusArchived }, wantStatus: 403, wantLookups: 1},
		{name: "session owner mismatch", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.session.UserID = 99 }, wantStatus: 403, wantLookups: 1},
		{name: "stale execution", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.turn.ProcessingToken = strings.Repeat("b", 64) }, wantStatus: 403, wantLookups: 1},
		{name: "empty stored execution", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", mutate: func(s *agentTurnStub) { s.turn.ProcessingToken = "" }, wantStatus: 403, wantLookups: 1},
		{name: "valid service call without JWT", configuredKey: "service-secret", providedKey: "service-secret", turnID: "87", wantStatus: 204, wantLookups: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config.GlobalConfig.Chat.AgentServiceToken = tc.configuredKey
			lease := time.Now().Add(time.Minute)
			repo := &agentTurnStub{
				turn:    &model.ChatTurn{ID: 87, UserID: 42, SessionID: 12, Status: model.TaskStatusRunning, LeaseExpiresAt: &lease, ProcessingToken: strings.Repeat("a", 64)},
				session: &model.ChatSession{ID: 12, UserID: 42, Status: model.ChatSessionStatusActive},
			}
			if tc.mutate != nil {
				tc.mutate(repo)
			}
			r := gin.New()
			r.GET("/tool", AgentAuth(), AgentTurnContext(repo), func(c *gin.Context) {
				if c.GetUint("userID") != 42 {
					t.Errorf("user identity did not come from turn")
				}
				if _, exists := c.Get("role"); exists {
					t.Error("agent must not acquire a user role")
				}
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/tool?user_id=99&token=service-secret", nil)
			req.Header.Set("X-Agent-Service-Token", tc.providedKey)
			req.Header.Set("X-Agent-Turn-ID", tc.turnID)
			req.Header.Set("X-Agent-Processing-Token", strings.Repeat("a", 64))
			// A user JWT never substitutes for the service key or controls the data owner.
			req.Header.Set("Authorization", "Bearer ordinary-user-jwt")
			res := httptest.NewRecorder()
			r.ServeHTTP(res, req)
			if res.Code != tc.wantStatus {
				t.Fatalf("status=%d, want=%d; body=%s", res.Code, tc.wantStatus, res.Body.String())
			}
			if res.Code == 403 && !strings.Contains(res.Body.String(), `"code":40302`) {
				t.Fatalf("expected terminal execution error, got %s", res.Body.String())
			}
			if strings.Contains(res.Body.String(), strings.Repeat("a", 64)) {
				t.Fatal("execution token leaked in response")
			}
			if repo.lookups != tc.wantLookups {
				t.Fatalf("turn lookups=%d, want=%d", repo.lookups, tc.wantLookups)
			}
		})
	}
}
