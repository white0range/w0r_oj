package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
)

func AdminCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleAny, exists := c.Get("role")
		if !exists {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "missing role information")
			c.Abort()
			return
		}

		role, ok := roleAny.(uint)
		if !ok {
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "invalid role information")
			c.Abort()
			return
		}

		if role != 1 {
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "admin access required")
			c.Abort()
			return
		}

		c.Next()
	}
}
