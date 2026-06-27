package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"gojo/config"
	"gojo/internal/app/apperror"
	"gojo/internal/user/dto"
	"gojo/internal/user/model"
	"gojo/internal/user/repository"
	"gojo/pkg/jwt"
	"gojo/pkg/password"
)

type SubmissionProvider interface {
	GetACProblemIDsByUserID(ctx context.Context, userID uint) ([]uint, error)
}

type UserService struct {
	repo        repository.UserRepository
	sessionRepo repository.RefreshSessionRepository
	subProvider SubmissionProvider
}

func NewUserService(r repository.UserRepository, sessionRepo repository.RefreshSessionRepository, sp SubmissionProvider) *UserService {
	return &UserService{
		repo:        r,
		sessionRepo: sessionRepo,
		subProvider: sp,
	}
}

func (s *UserService) RegisterUser(ctx context.Context, req dto.UserAuthRequest) error {
	hash, err := password.HashPassword(req.Password)
	if err != nil {
		return apperror.ErrPasswordHashFailed
	}

	user := model.User{
		Username:     req.Username,
		Password:     hash,
		Status:       model.UserStatusActive,
		TokenVersion: 1,
	}

	if err := s.repo.CreateUser(ctx, &user); err != nil {
		return apperror.ErrUsernameExists
	}

	return nil
}

func (s *UserService) LoginUser(ctx context.Context, req dto.UserAuthRequest) (*dto.TokenPairResponse, error) {
	user, err := s.repo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, apperror.ErrUserNotFound
	}

	if user.Status == model.UserStatusBanned {
		return nil, apperror.ErrUserBanned
	}

	if !password.CheckPasswordHash(req.Password, user.Password) {
		return nil, apperror.ErrWrongPassword
	}

	sessionID, err := newSessionID()
	if err != nil {
		return nil, apperror.ErrTokenGeneration
	}

	tokens, err := jwt.GenerateTokenPair(user, sessionID)
	if err != nil {
		return nil, apperror.ErrTokenGeneration
	}

	if err := s.sessionRepo.CreateSession(ctx, &repository.RefreshSession{
		SessionID:    sessionID,
		UserID:       user.ID,
		TokenVersion: user.TokenVersion,
		ExpiresAt:    time.Now().Add(refreshSessionTTL()),
	}, refreshSessionTTL()); err != nil {
		return nil, apperror.ErrTokenGeneration
	}

	return &dto.TokenPairResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}

func (s *UserService) RefreshSession(ctx context.Context, refreshToken string) (*dto.TokenPairResponse, string, error) {
	claims, err := jwt.ParseToken(refreshToken, jwt.TokenTypeRefresh)
	if err != nil {
		return nil, "", apperror.ErrInvalidToken
	}

	if claims.SessionID == "" {
		return nil, "", apperror.ErrInvalidToken
	}

	session, err := s.sessionRepo.GetSession(ctx, claims.SessionID)
	if err != nil {
		return nil, "", apperror.ErrInvalidToken
	}

	if session.UserID != claims.UserID || session.TokenVersion != claims.TokenVersion {
		return nil, "", apperror.ErrInvalidToken
	}

	user, err := s.repo.GetUserAuthByID(ctx, claims.UserID)
	if err != nil {
		return nil, "", apperror.ErrUserNotFound
	}

	if user.TokenVersion != claims.TokenVersion {
		return nil, "", apperror.ErrInvalidToken
	}

	if user.Status == model.UserStatusBanned {
		return nil, "", apperror.ErrUserBanned
	}

	newSessionID, err := newSessionID()
	if err != nil {
		return nil, "", apperror.ErrTokenGeneration
	}

	tokens, err := jwt.GenerateTokenPair(user, newSessionID)
	if err != nil {
		return nil, "", apperror.ErrTokenGeneration
	}

	if err := s.sessionRepo.CreateSession(ctx, &repository.RefreshSession{
		SessionID:    newSessionID,
		UserID:       user.ID,
		TokenVersion: user.TokenVersion,
		ExpiresAt:    time.Now().Add(refreshSessionTTL()),
	}, refreshSessionTTL()); err != nil {
		return nil, "", apperror.ErrTokenGeneration
	}

	_ = s.sessionRepo.DeleteSession(ctx, claims.SessionID)

	return &dto.TokenPairResponse{
		AccessToken: tokens.AccessToken,
	}, tokens.RefreshToken, nil
}

func (s *UserService) LogoutSession(ctx context.Context, refreshToken string) error {
	claims, err := jwt.ParseToken(refreshToken, jwt.TokenTypeRefresh)
	if err != nil {
		return nil
	}

	if claims.SessionID == "" {
		return nil
	}

	return s.sessionRepo.DeleteSession(ctx, claims.SessionID)
}

func (s *UserService) GetUserProfile(ctx context.Context, userID uint) (*dto.UserProfileResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, apperror.ErrUserNotFound
	}

	var solvedProblemIDs []uint
	if s.subProvider != nil {
		solvedProblemIDs, _ = s.subProvider.GetACProblemIDsByUserID(ctx, userID)
	}

	return &dto.UserProfileResponse{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		Status:      user.Status,
		SolvedCount: user.SolvedCount,
		SolvedList:  solvedProblemIDs,
	}, nil
}

func (s *UserService) ListUsers(ctx context.Context, keyword string, limit int) ([]dto.AdminUserItem, error) {
	users, err := s.repo.ListUsers(ctx, strings.TrimSpace(keyword), limit)
	if err != nil {
		return nil, err
	}

	items := make([]dto.AdminUserItem, 0, len(users))
	for _, user := range users {
		items = append(items, dto.AdminUserItem{
			ID:          user.ID,
			Username:    user.Username,
			Role:        user.Role,
			Status:      user.Status,
			SolvedCount: user.SolvedCount,
			BanReason:   user.BanReason,
			BannedAt:    user.BannedAt,
			CreatedAt:   user.CreatedAt,
		})
	}

	return items, nil
}

func (s *UserService) BanUser(ctx context.Context, actorID, targetID uint, reason string) error {
	if actorID == targetID {
		return apperror.ErrForbidden
	}

	user, err := s.repo.GetUserByID(ctx, targetID)
	if err != nil {
		return apperror.ErrUserNotFound
	}

	if user.Role == 1 {
		return apperror.ErrForbidden
	}

	if user.Status == model.UserStatusBanned {
		return nil
	}

	now := time.Now()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "banned by admin"
	}

	if err := s.repo.UpdateUserFields(ctx, targetID, map[string]interface{}{
		"status":        model.UserStatusBanned,
		"ban_reason":    reason,
		"banned_at":     &now,
		"token_version": user.TokenVersion + 1,
	}); err != nil {
		return err
	}

	if s.sessionRepo != nil {
		_ = s.sessionRepo.DeleteUserSessions(ctx, targetID)
	}

	return nil
}

func (s *UserService) UnbanUser(ctx context.Context, targetID uint) error {
	user, err := s.repo.GetUserByID(ctx, targetID)
	if err != nil {
		return apperror.ErrUserNotFound
	}

	if user.Role == 1 {
		return apperror.ErrForbidden
	}

	if user.Status == model.UserStatusActive {
		return nil
	}

	return s.repo.UpdateUserFields(ctx, targetID, map[string]interface{}{
		"status":        model.UserStatusActive,
		"ban_reason":    "",
		"banned_at":     nil,
		"token_version": user.TokenVersion + 1,
	})
}

func (s *UserService) GetUsersMapByIDs(ctx context.Context, userIDs []uint) (map[uint]string, error) {
	userMap := make(map[uint]string)
	if len(userIDs) == 0 {
		return userMap, nil
	}

	users, err := s.repo.GetUsersByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		userMap[u.ID] = u.Username
	}

	return userMap, nil
}

func (s *UserService) GetTopUsersBySolvedCount(ctx context.Context, limit int) ([]model.User, error) {
	return s.repo.GetTopUsersBySolvedCount(ctx, limit)
}

func (s *UserService) RevokeUserSessions(ctx context.Context, userID uint) error {
	return s.sessionRepo.DeleteUserSessions(ctx, userID)
}

func refreshSessionTTL() time.Duration {
	hours := config.GlobalConfig.JWT.RefreshTTLHours
	if hours <= 0 {
		hours = 168
	}
	return time.Duration(hours) * time.Hour
}

func newSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
