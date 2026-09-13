package middlewares

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
)

const rateLimitLua = `
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return {count, redis.call("PTTL", KEYS[1])}
`

func PublicIPRateLimit() gin.HandlerFunc {
	return ipRateLimit("public", config.GlobalConfig.RateLimit.PublicIPPerMinute, time.Minute)
}

func RegisterRateLimit() gin.HandlerFunc {
	return ipRateLimit("register", config.GlobalConfig.RateLimit.RegisterIPPerHour, time.Hour)
}

func LoginIPRateLimit() gin.HandlerFunc {
	return ipRateLimit("login", config.GlobalConfig.RateLimit.LoginIPPerMinute, time.Minute)
}

// LoginAccountRateLimit is called after the username has been parsed, so the
// same account cannot be brute-forced from many IP addresses.
func LoginAccountRateLimit(c *gin.Context, username string) bool {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return true
	}
	return enforceRateLimit(c, fmt.Sprintf("rate_limit:login:account:%s", username), config.GlobalConfig.RateLimit.LoginAccountPerMinute, time.Minute)
}

func SubmitRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := currentUserID(c)
		if !ok {
			return
		}
		if !enforceRateLimit(c, fmt.Sprintf("rate_limit:submit:short:user:%d", userID), config.GlobalConfig.RateLimit.SubmitPerFiveSeconds, 5*time.Second) {
			return
		}
		if !enforceRateLimit(c, fmt.Sprintf("rate_limit:submit:hour:user:%d", userID), config.GlobalConfig.RateLimit.SubmitPerHour, time.Hour) {
			return
		}
		c.Next()
	}
}

func ChatMessageRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := currentUserID(c)
		if !ok {
			return
		}

		// Chat quota resets at midnight in China Standard Time (UTC+8), rather
		// than 24 hours after a user's first message.
		now := time.Now().In(time.FixedZone("CST", 8*60*60))
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		key := fmt.Sprintf("rate_limit:chat:day:%s:user:%d", now.Format("2006-01-02"), userID)
		if !enforceRateLimit(c, key, config.GlobalConfig.RateLimit.ChatMessagesPerDay, nextMidnight.Sub(now)) {
			return
		}
		c.Next()
	}
}

func SearchRateLimit() gin.HandlerFunc {
	return ipRateLimit("search", config.GlobalConfig.RateLimit.SearchIPPerMinute, time.Minute)
}

func SSETicketRateLimit() gin.HandlerFunc {
	return userRateLimit("sse_ticket", config.GlobalConfig.RateLimit.SSETicketsPerMinute, time.Minute)
}

func ipRateLimit(scope string, maximum int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := strings.TrimSpace(c.ClientIP())
		if ip == "" {
			ip = "unknown"
		}
		if !enforceRateLimit(c, fmt.Sprintf("rate_limit:%s:ip:%s", scope, ip), maximum, window) {
			return
		}
		c.Next()
	}
}

func userRateLimit(scope string, maximum int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := currentUserID(c)
		if !ok {
			return
		}
		if !enforceRateLimit(c, fmt.Sprintf("rate_limit:%s:user:%d", scope, userID), maximum, window) {
			return
		}
		c.Next()
	}
}

func currentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("userID")
	userID, ok := value.(uint)
	if !exists || !ok || userID == 0 {
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "missing current user")
		c.Abort()
		return 0, false
	}
	return userID, true
}

func enforceRateLimit(c *gin.Context, key string, maximum int, window time.Duration) bool {
	if maximum <= 0 || window <= 0 {
		return true
	}

	result, err := cache.Rdb.Eval(c.Request.Context(), rateLimitLua, []string{key}, window.Milliseconds()).Result()
	if err != nil {
		response.FailWithMessage(c, http.StatusServiceUnavailable, ecode.InternalError, "rate limiter unavailable")
		c.Abort()
		return false
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		response.FailWithMessage(c, http.StatusServiceUnavailable, ecode.InternalError, "rate limiter returned an invalid result")
		c.Abort()
		return false
	}
	count, okCount := redisInt64(values[0])
	remainingMilliseconds, okTTL := redisInt64(values[1])
	if !okCount || !okTTL {
		response.FailWithMessage(c, http.StatusServiceUnavailable, ecode.InternalError, "rate limiter returned an invalid result")
		c.Abort()
		return false
	}
	if count <= int64(maximum) {
		return true
	}

	if remainingMilliseconds > 0 {
		c.Header("Retry-After", strconv.FormatInt(int64(math.Ceil(float64(remainingMilliseconds)/1000)), 10))
	}
	response.FailWithMessage(c, http.StatusTooManyRequests, ecode.TooManyRequests, "rate limit exceeded, please retry later")
	c.Abort()
	return false
}

func redisInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

const releaseRateLimitLua = `
local count = redis.call("GET", KEYS[1])
if not count then
  return 0
end
if tonumber(count) <= 1 then
  return redis.call("DEL", KEYS[1])
end
return redis.call("DECR", KEYS[1])
`

// ReserveSuccessfulRegistrationSlot reserves one of the global daily
// registration slots. The caller releases the reservation on a failed
// registration, so only successfully created users consume the daily quota.
func ReserveSuccessfulRegistrationSlot(c *gin.Context) (func(success bool), bool) {
	now := time.Now().In(time.FixedZone("CST", 8*60*60))
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	key := fmt.Sprintf("rate_limit:register:success:day:%s", now.Format("2006-01-02"))
	if !enforceRateLimit(c, key, config.GlobalConfig.RateLimit.SuccessfulRegistrationsPerDay, nextMidnight.Sub(now)) {
		return nil, false
	}

	return func(success bool) {
		if success {
			return
		}
		// Best effort: keeping a slot reserved until midnight is safer than
		// accidentally allowing more successful registrations than configured.
		_, _ = cache.Rdb.Eval(c.Request.Context(), releaseRateLimitLua, []string{key}).Result()
	}, true
}
