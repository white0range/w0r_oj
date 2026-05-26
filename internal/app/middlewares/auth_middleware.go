package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/pkg/jwt"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid authorization header")
				c.Abort()
				return
			}
			tokenString = parts[1]
		} else {
			tokenString = c.Query("token")
		}

		claims, err := jwt.ParseToken(tokenString)
		if err != nil {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		val, ok := (*claims)["user_id"].(float64)
		if !ok {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid token payload")
			c.Abort()
			return
		}

		username, ok := (*claims)["username"].(string)
		if !ok {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid token payload")
			c.Abort()
			return
		}

		role, ok := (*claims)["role"].(float64)
		if !ok {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid token payload")
			c.Abort()
			return
		}

		c.Set("userID", uint(val))
		c.Set("username", username)
		c.Set("role", uint(role))
		c.Next()
	}
}
