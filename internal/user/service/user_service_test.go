package service

import (
	"context"
	"errors"
	"gojo/internal/app/apperror"
	"gojo/internal/user/dto"
	"gojo/internal/user/model"
	"testing"
)

type fakeUserRepo struct {
	// 这几个字段用来“注入”每个测试想要的行为
	createUserFn        func(ctx context.Context, user *model.User) error
	getUserByUsernameFn func(ctx context.Context, username string) (*model.User, error)
	getUserByIDFn       func(ctx context.Context, id uint) (*model.User, error)
	getUsersByIDsFn     func(ctx context.Context, ids []uint) ([]model.User, error)
	getTopUsersFn       func(ctx context.Context, limit int) ([]model.User, error)
}

// 这些方法是为了实现 UserRepository 接口。
// 真正返回什么，由上面的 xxxFn 决定。
func (f *fakeUserRepo) CreateUser(ctx context.Context, user *model.User) error {
	return f.createUserFn(ctx, user)
}

func (f *fakeUserRepo) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return f.getUserByUsernameFn(ctx, username)
}

func (f *fakeUserRepo) GetUserByID(ctx context.Context, id uint) (*model.User, error) {
	return f.getUserByIDFn(ctx, id)
}

func (f *fakeUserRepo) GetUsersByIDs(ctx context.Context, ids []uint) ([]model.User, error) {
	return f.getUsersByIDsFn(ctx, ids)
}

func (f *fakeUserRepo) GetTopUsersBySolvedCount(ctx context.Context, limit int) ([]model.User, error) {
	if f.getTopUsersFn == nil {
		return nil, nil
	}
	return f.getTopUsersFn(ctx, limit)
}

func TestRegisterUser_UsernameExists(t *testing.T) {
	// 1. 造一个假的 repo，并让它在创建用户时故意报错
	repo := &fakeUserRepo{
		createUserFn: func(ctx context.Context, user *model.User) error {
			return errors.New("duplicate username")
		},
	}

	// 2. 用假 repo 创建 service
	svc := NewUserService(repo, nil)

	// 3. 调用要测试的方法
	err := svc.RegisterUser(context.Background(), dto.UserAuthRequest{
		Username: "alice",
		Password: "123456",
	})

	// 4. 检查返回的错误是不是我们期望的业务错误
	if !errors.Is(err, apperror.ErrUsernameExists) {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
	}
}
