package service

import (
	"context"

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
	subProvider SubmissionProvider
}

func NewUserService(r repository.UserRepository, sp SubmissionProvider) *UserService {
	return &UserService{
		repo:        r,
		subProvider: sp,
	}
}

func (s *UserService) RegisterUser(ctx context.Context, req dto.UserAuthRequest) error {
	hash, err := password.HashPassword(req.Password)
	if err != nil {
		return apperror.ErrPasswordHashFailed
	}

	user := model.User{
		Username: req.Username,
		Password: hash,
	}

	if err := s.repo.CreateUser(ctx, &user); err != nil {
		return apperror.ErrUsernameExists
	}

	return nil
}

func (s *UserService) LoginUser(ctx context.Context, req dto.UserAuthRequest) (string, error) {
	user, err := s.repo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return "", apperror.ErrUserNotFound
	}

	if !password.CheckPasswordHash(req.Password, user.Password) {
		return "", apperror.ErrWrongPassword
	}

	token, err := jwt.GenerateToken(user)
	if err != nil {
		return "", apperror.ErrTokenGeneration
	}

	return token, nil
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
		SolvedCount: user.SolvedCount,
		SolvedList:  solvedProblemIDs,
	}, nil
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
