package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gojo/config"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) CreateEventsTicket(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	ticket, err := CreateTicket(c.Request.Context(), userID, ScopeEvents, 0)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "create realtime ticket failed")
		return
	}
	response.OK(c, gin.H{"ticket": ticket, "expires_in_seconds": TicketTTLSeconds()})
}

func (h *Handler) StreamEvents(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "streaming is not supported")
		return
	}

	releaseConnection, acquired := AcquireConnection(userID, config.GlobalConfig.RateLimit.SSEConnectionsPerUser)
	if !acquired {
		response.FailWithMessage(c, http.StatusTooManyRequests, ecode.TooManyRequests, "too many active SSE connections")
		return
	}
	defer releaseConnection()

	events, unsubscribe := Subscribe(userID)
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte(": connected\n\n"))
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			if err := writeEvent(c, event); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := c.Writer.Write([]byte(": keep-alive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func currentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("userID")
	userID, ok := value.(uint)
	if !exists || !ok || userID == 0 {
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid realtime ticket")
		return 0, false
	}
	return userID, true
}

func writeEvent(c *gin.Context, event Event) error {
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Name, payload)
	return err
}
