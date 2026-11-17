package middlewares

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"gojo/config"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/chat/model"
)

// AgentAuth authenticates service calls independently of user JWTs and roles.
func AgentAuth() gin.HandlerFunc {
	expected := strings.TrimSpace(config.GlobalConfig.Chat.AgentServiceToken)
	expectedHash := sha256.Sum256([]byte(expected))
	return func(c *gin.Context) {
		provided := c.GetHeader("X-Agent-Service-Token")
		providedHash := sha256.Sum256([]byte(provided))
		if expected == "" || provided == "" || subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid agent service token")
			c.Abort()
			return
		}
		c.Next()
	}
}

type AgentTurnProvider interface {
	GetTurnByID(context.Context, uint) (*model.ChatTurn, error)
	GetSessionByID(context.Context, uint) (*model.ChatSession, error)
}

// AgentTurnContext resolves the data owner from a live turn, never from model arguments.
// The turn ID is context, not a credential; AgentAuth must run before this middleware.
func AgentTurnContext(repo AgentTurnProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		turnID, err := strconv.ParseUint(c.GetHeader("X-Agent-Turn-ID"), 10, strconv.IntSize)
		if err != nil || turnID == 0 {
			response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid agent turn id")
			c.Abort()
			return
		}
		processingToken := c.GetHeader("X-Agent-Processing-Token")
		if processingToken == "" {
			response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "missing agent execution id")
			c.Abort()
			return
		}

		turn, err := repo.GetTurnByID(c.Request.Context(), uint(turnID))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusForbidden, ecode.AgentExecutionExpired)
			c.Abort()
			return
		}
		if err != nil {
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "load agent turn failed")
			c.Abort()
			return
		}
		if turn == nil || turn.UserID == 0 || turn.Status != model.TaskStatusRunning || turn.LeaseExpiresAt == nil || !turn.LeaseExpiresAt.After(time.Now()) {
			response.Fail(c, http.StatusForbidden, ecode.AgentExecutionExpired)
			c.Abort()
			return
		}
		expectedAttempt := sha256.Sum256([]byte(turn.ProcessingToken))
		providedAttempt := sha256.Sum256([]byte(processingToken))
		if turn.ProcessingToken == "" || subtle.ConstantTimeCompare(expectedAttempt[:], providedAttempt[:]) != 1 {
			response.Fail(c, http.StatusForbidden, ecode.AgentExecutionExpired)
			c.Abort()
			return
		}

		session, err := repo.GetSessionByID(c.Request.Context(), turn.SessionID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "load agent session failed")
			c.Abort()
			return
		}
		if err != nil || session == nil || session.UserID != turn.UserID || session.Status != model.ChatSessionStatusActive {
			response.Fail(c, http.StatusForbidden, ecode.AgentExecutionExpired)
			c.Abort()
			return
		}

		c.Set("userID", turn.UserID)
		c.Set("agentTurnID", turn.ID)
		c.Next()
	}
}
