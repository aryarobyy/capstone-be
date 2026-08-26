package user

import (
	"context"

	responsehandler "capstone-be/internal/utils"
)

type UserService interface {
	Detail(ctx context.Context, req UserDetailRequest) (*User, error)
	List(ctx context.Context, req ListUserRequest) (*responsehandler.ListResponse[User], error)
	Update(ctx context.Context, req UpdateUserRequest) (*User, error)
	Delete(ctx context.Context, req DeleteUserRequest) error
}

type userService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) Detail(ctx context.Context, req UserDetailRequest) (*User, error) {
	return s.repo.Detail(ctx, req)
}

func (s *userService) List(ctx context.Context, req ListUserRequest) (*responsehandler.ListResponse[User], error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	index := req.Index
	if index < 0 {
		index = 0
	}

	users, count, err := s.repo.List(ctx, limit, index)
	if err != nil {
		return nil, err
	}

	return &responsehandler.ListResponse[User]{
		List:  users,
		Count: count,
		Index: index,
		Limit: limit,
	}, nil
}

func (s *userService) Update(ctx context.Context, req UpdateUserRequest) (*User, error) {
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}

	u, err := s.repo.Detail(ctx, UserDetailRequest{ID: req.ID})
	if err != nil {
		return nil, err
	}

	return u, nil
}

func (s *userService) Delete(ctx context.Context, req DeleteUserRequest) error {
	return s.repo.Delete(ctx, req)
}
