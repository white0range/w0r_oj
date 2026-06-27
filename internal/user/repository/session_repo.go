package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gojo/infrastructure/cache"
)

type RefreshSession struct {
	SessionID    string    `json:"session_id"`
	UserID       uint      `json:"user_id"`
	TokenVersion int       `json:"token_version"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type RefreshSessionRepository interface {
	CreateSession(ctx context.Context, session *RefreshSession, ttl time.Duration) error
	GetSession(ctx context.Context, sessionID string) (*RefreshSession, error)
	DeleteSession(ctx context.Context, sessionID string) error
	DeleteUserSessions(ctx context.Context, userID uint) error
}

type refreshSessionRepoRedis struct{}

func NewRefreshSessionRepository() RefreshSessionRepository {
	return &refreshSessionRepoRedis{}
}

func (r *refreshSessionRepoRedis) CreateSession(ctx context.Context, session *RefreshSession, ttl time.Duration) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}

	pipe := cache.Rdb.TxPipeline()
	pipe.Set(ctx, refreshSessionKey(session.SessionID), payload, ttl)
	pipe.SAdd(ctx, userSessionSetKey(session.UserID), session.SessionID)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *refreshSessionRepoRedis) GetSession(ctx context.Context, sessionID string) (*RefreshSession, error) {
	raw, err := cache.Rdb.Get(ctx, refreshSessionKey(sessionID)).Result()
	if err != nil {
		return nil, err
	}

	var session RefreshSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *refreshSessionRepoRedis) DeleteSession(ctx context.Context, sessionID string) error {
	session, err := r.GetSession(ctx, sessionID)
	if err != nil {
		return cache.Rdb.Del(ctx, refreshSessionKey(sessionID)).Err()
	}

	pipe := cache.Rdb.TxPipeline()
	pipe.Del(ctx, refreshSessionKey(sessionID))
	pipe.SRem(ctx, userSessionSetKey(session.UserID), sessionID)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *refreshSessionRepoRedis) DeleteUserSessions(ctx context.Context, userID uint) error {
	sessionIDs, err := cache.Rdb.SMembers(ctx, userSessionSetKey(userID)).Result()
	if err != nil {
		return err
	}

	pipe := cache.Rdb.TxPipeline()
	for _, sessionID := range sessionIDs {
		pipe.Del(ctx, refreshSessionKey(sessionID))
	}
	pipe.Del(ctx, userSessionSetKey(userID))
	_, err = pipe.Exec(ctx)
	return err
}

func refreshSessionKey(sessionID string) string {
	return fmt.Sprintf("auth:refresh:session:%s", sessionID)
}

func userSessionSetKey(userID uint) string {
	return fmt.Sprintf("auth:user:sessions:%d", userID)
}
