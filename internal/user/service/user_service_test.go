package service

import (
	"context"
	"errors"
	"testing"

	"gojo/internal/app/apperror"
	"gojo/internal/user/dto"
	"gojo/internal/user/model"
)

type fakeUserRepo struct {
	createUserFn        func(ctx context.Context, user *model.User) error
	getUserByUsernameFn func(ctx context.Context, username string) (*model.User, error)
	getUserByIDFn       func(ctx context.Context, id uint) (*model.User, error)
	getUserAuthByIDFn   func(ctx context.Context, id uint) (*model.User, error)
	updateUserFieldsFn  func(ctx context.Context, id uint, updates map[string]interface{}) error
	listUsersFn         func(ctx context.Context, keyword string, limit int) ([]model.User, error)
	getUsersByIDsFn     func(ctx context.Context, ids []uint) ([]model.User, error)
	getTopUsersFn       func(ctx context.Context, limit int) ([]model.User, error)
}

func (f *fakeUserRepo) CreateUser(ctx context.Context, user *model.User) error {
	return f.createUserFn(ctx, user)
}

func (f *fakeUserRepo) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	if f.getUserByUsernameFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.getUserByUsernameFn(ctx, username)
}

func (f *fakeUserRepo) GetUserByID(ctx context.Context, id uint) (*model.User, error) {
	if f.getUserByIDFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.getUserByIDFn(ctx, id)
}

func (f *fakeUserRepo) GetUserAuthByID(ctx context.Context, id uint) (*model.User, error) {
	if f.getUserAuthByIDFn == nil {
		return nil, errors.New("not implemented")
	}
	return f.getUserAuthByIDFn(ctx, id)
}

func (f *fakeUserRepo) UpdateUserFields(ctx context.Context, id uint, updates map[string]interface{}) error {
	if f.updateUserFieldsFn == nil {
		return nil
	}
	return f.updateUserFieldsFn(ctx, id, updates)
}

func (f *fakeUserRepo) ListUsers(ctx context.Context, keyword string, limit int) ([]model.User, error) {
	if f.listUsersFn == nil {
		return nil, nil
	}
	return f.listUsersFn(ctx, keyword, limit)
}

func (f *fakeUserRepo) GetUsersByIDs(ctx context.Context, ids []uint) ([]model.User, error) {
	if f.getUsersByIDsFn == nil {
		return nil, nil
	}
	return f.getUsersByIDsFn(ctx, ids)
}

func (f *fakeUserRepo) GetTopUsersBySolvedCount(ctx context.Context, limit int) ([]model.User, error) {
	if f.getTopUsersFn == nil {
		return nil, nil
	}
	return f.getTopUsersFn(ctx, limit)
}

func TestRegisterUser_UsernameExists(t *testing.T) {
	repo := &fakeUserRepo{
		createUserFn: func(ctx context.Context, user *model.User) error {
			return errors.New("duplicate username")
		},
	}

	svc := NewUserService(repo, nil, nil)
	err := svc.RegisterUser(context.Background(), dto.UserAuthRequest{
		Username: "alice",
		Password: "123456",
	})

	if !errors.Is(err, apperror.ErrUsernameExists) {
		t.Fatalf("expected ErrUsernameExists, got %v", err)
	}
}
