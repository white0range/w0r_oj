package jwt

import (
	"errors"
	"time"

	"gojo/config"
	"gojo/internal/user/model"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	Role         int    `json:"role"`
	TokenVersion int    `json:"token_version"`
	TokenType    string `json:"token_type"`
	SessionID    string `json:"sid,omitempty"`
	jwtv5.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

func getJWTSecret() []byte {
	return []byte(config.GlobalConfig.JWT.Secret)
}

func GenerateTokenPair(user *model.User, sessionID string) (*TokenPair, error) {
	accessToken, err := generateToken(user, TokenTypeAccess, accessTokenTTL(), "")
	if err != nil {
		return nil, err
	}

	refreshToken, err := generateToken(user, TokenTypeRefresh, refreshTokenTTL(), sessionID)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func GenerateToken(user *model.User) (string, error) {
	return generateToken(user, TokenTypeAccess, accessTokenTTL(), "")
}

func ParseToken(tokenString, expectedType string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwtv5.ParseWithClaims(tokenString, claims, func(token *jwtv5.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return getJWTSecret(), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	if expectedType != "" && claims.TokenType != expectedType {
		return nil, errors.New("invalid token type")
	}

	return claims, nil
}

func generateToken(user *model.User, tokenType string, ttl time.Duration, sessionID string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
		TokenType:    tokenType,
		SessionID:    sessionID,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Subject:   "user-session",
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString(getJWTSecret())
}

func accessTokenTTL() time.Duration {
	minutes := config.GlobalConfig.JWT.AccessTTLMinutes
	if minutes <= 0 {
		minutes = 120
	}
	return time.Duration(minutes) * time.Minute
}

func refreshTokenTTL() time.Duration {
	hours := config.GlobalConfig.JWT.RefreshTTLHours
	if hours <= 0 {
		hours = 168
	}
	return time.Duration(hours) * time.Hour
}
