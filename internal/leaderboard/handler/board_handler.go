package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/leaderboard/service"
)

type LeaderboardHandler struct {
	svc *service.LeaderboardService
}

func NewLeaderboardHandler(svc *service.LeaderboardService) *LeaderboardHandler {
	return &LeaderboardHandler{
		svc: svc,
	}
}

func (h *LeaderboardHandler) GetGlobalLeaderboard(c *gin.Context) {
	var currentUserID uint
	if userIDRaw, exists := c.Get("userID"); exists {
		if uid, ok := userIDRaw.(uint); ok {
			currentUserID = uid
		}
	}

	data, err := h.svc.GetGlobalLeaderboard(c.Request.Context(), currentUserID)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get leaderboard failed")
		return
	}

	response.OK(c, data)
}
