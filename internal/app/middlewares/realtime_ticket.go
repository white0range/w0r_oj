package middlewares

import (
	"net/http"
	"strconv"

	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/realtime"

	"github.com/gin-gonic/gin"
)

func RealtimeTicketAuth(scope realtime.TicketScope, turnIDParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var turnID uint
		if turnIDParam != "" {
			parsed, err := strconv.ParseUint(c.Param(turnIDParam), 10, 64)
			if err != nil || parsed == 0 {
				response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid turn id")
				c.Abort()
				return
			}
			turnID = uint(parsed)
		}

		ticket, err := realtime.ConsumeTicket(c.Request.Context(), c.Query("ticket"), scope, turnID)
		if err != nil {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid or expired realtime ticket")
			c.Abort()
			return
		}
		c.Set("userID", ticket.UserID)
		c.Next()
	}
}
