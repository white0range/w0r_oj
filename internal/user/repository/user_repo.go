package repository

import (
	"context"
	"gojo/infrastructure/mysql"
	"gojo/internal/user/model" // 假设你的 User 实体在这里
)

// 1. 仓管接口定义
type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByUsername(ctx context.Context, username string) (*model.User, error)
	GetUserByID(ctx context.Context, id uint) (*model.User, error)

	GetUsersByIDs(ctx context.Context, ids []uint) ([]model.User, error)
	GetTopUsersBySolvedCount(ctx context.Context, limit int) ([]model.User, error)
}

type userRepoMysql struct{}

func NewUserRepository() UserRepository {
	return &userRepoMysql{}
}

// 2. 落地实现
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
	// 剔除密码等敏感字段的大厂做法：只 Select 需要的字段
	err := mysql.DB.WithContext(ctx).Select("id", "username", "role", "solved_count").First(&user, id).Error
	return &user, err
}

// 2. 落地实现
func (r *userRepoMysql) GetUsersByIDs(ctx context.Context, ids []uint) ([]model.User, error) {
	var users []model.User

	// 🛡️ 架构师防御：如果传进来的空数组，直接返回空切片，绝不让 MySQL 执行 "IN ()" 导致语法报错
	if len(ids) == 0 {
		return users, nil
	}

	// 依然保持大厂好习惯：只 Select 绝对需要的字段（id 和 username），绝不把密码 Hash 查出来！
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
		Where("solved_count > 0").
		Order("solved_count desc, id asc").
		Limit(limit).
		Find(&users).Error

	return users, err
}
