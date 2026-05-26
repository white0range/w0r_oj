package middlewares

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"gojo/infrastructure/cache"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
)

func SubmitRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDAny, exists := c.Get("userID")
		if !exists {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "missing current user")
			c.Abort()
			return
		}

		userID, ok := userIDAny.(uint)
		if !ok {
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "invalid user identity")
			c.Abort()
			return
		}

		redisKey := fmt.Sprintf("rate_limit:submit:%d", userID)
		ctx := c.Request.Context()

		count, err := cache.Rdb.Incr(ctx, redisKey).Result()
		if err != nil {
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "rate limiter unavailable")
			c.Abort()
			return
		}

		if count == 1 {
			cache.Rdb.Expire(ctx, redisKey, 5*time.Second)
		} else {
			response.FailWithMessage(c, http.StatusTooManyRequests, ecode.Forbidden, "submit too fast, please retry later")
			c.Abort()
			return
		}

		c.Next()
	}
}

func AIRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDAny, exists := c.Get("userID")
		if !exists {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "missing current user")
			c.Abort()
			return
		}

		userID, ok := userIDAny.(uint)
		if !ok {
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "invalid user identity")
			c.Abort()
			return
		}

		today := time.Now().Format("2006-01-02")
		redisKey := fmt.Sprintf("rate_limit:ai:%d:%s", userID, today)
		ctx := c.Request.Context()

		count, err := cache.Rdb.Incr(ctx, redisKey).Result()
		if err != nil {
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "rate limiter unavailable")
			c.Abort()
			return
		}

		if count == 1 {
			cache.Rdb.Expire(ctx, redisKey, 24*time.Hour)
		}

		if count > 3 {
			response.FailWithMessage(c, http.StatusTooManyRequests, ecode.Forbidden, "daily AI quota exceeded")
			c.Abort()
			return
		}

		c.Next()
	}
}
