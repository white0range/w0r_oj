package repository

import (
	"context"

	"gojo/infrastructure/mysql"
	"gojo/internal/user/model"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByUsername(ctx context.Context, username string) (*model.User, error)
	GetUserByID(ctx context.Context, id uint) (*model.User, error)
	GetUserAuthByID(ctx context.Context, id uint) (*model.User, error)
	UpdateUserFields(ctx context.Context, id uint, updates map[string]interface{}) error
	ListUsers(ctx context.Context, keyword string, limit int) ([]model.User, error)

	GetUsersByIDs(ctx context.Context, ids []uint) ([]model.User, error)
	GetTopUsersBySolvedCount(ctx context.Context, limit int) ([]model.User, error)
}

type userRepoMysql struct{}

func NewUserRepository() UserRepository {
	return &userRepoMysql{}
}

func (r *userRepoMysql) CreateUser(ctx context.Context, user *model.User) error {
	return mysql.DB.WithContext(ctx).Create(user).Error
}

func (r *userRepoMysql) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var user model.User
	if err := mysql.DB.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepoMysql) GetUserByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	err := mysql.DB.WithContext(ctx).
		Select("id", "username", "role", "status", "ban_reason", "banned_at", "solved_count", "token_version", "created_at", "updated_at").
		First(&user, id).Error
	return &user, err
}

func (r *userRepoMysql) GetUserAuthByID(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	err := mysql.DB.WithContext(ctx).
		Select("id", "username", "role", "status", "token_version").
		First(&user, id).Error
	return &user, err
}

func (r *userRepoMysql) UpdateUserFields(ctx context.Context, id uint, updates map[string]interface{}) error {
	return mysql.DB.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

func (r *userRepoMysql) ListUsers(ctx context.Context, keyword string, limit int) ([]model.User, error) {
	var users []model.User
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	query := mysql.DB.WithContext(ctx).
		Select("id", "username", "role", "status", "ban_reason", "banned_at", "solved_count", "created_at").
		Order("id desc").
		Limit(limit)

	if keyword != "" {
		query = query.Where("username LIKE ?", "%"+keyword+"%")
	}

	err := query.Find(&users).Error
	return users, err
}

func (r *userRepoMysql) GetUsersByIDs(ctx context.Context, ids []uint) ([]model.User, error) {
	var users []model.User
	if len(ids) == 0 {
		return users, nil
	}

	err := mysql.DB.WithContext(ctx).
		Select("id", "username").
		Where("id IN ?", ids).
		Find(&users).Error

	return users, err
}

func (r *userRepoMysql) GetTopUsersBySolvedCount(ctx context.Context, limit int) ([]model.User, error) {
	var users []model.User
	if limit <= 0 {
		limit = 50
	}

	err := mysql.DB.WithContext(ctx).
		Select("id", "username", "solved_count").
		Where("solved_count > 0 AND status = ?", model.UserStatusActive).
		Order("solved_count desc, id asc").
		Limit(limit).
		Find(&users).Error

	return users, err
}
