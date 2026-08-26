package user

import (
	"context"
)

type UserService interface {
	Detail(ctx context.Context, req UserDetailRequest) (*UserResponse, error)
	List(ctx context.Context, req ListUserRequest) ([]UserResponse, error)
	Update(ctx context.Context, req UpdateUserRequest) (*UserResponse, error)
	Delete(ctx context.Context, req DeleteUserRequest) error
}

type userService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) Detail(ctx context.Context, req UserDetailRequest) (*UserResponse, error) {
	u, err := s.repo.Detail(ctx, req)
	if err != nil {
		return nil, err
	}
	return ToUserResponse(u), nil
}

func (s *userService) List(ctx context.Context, req ListUserRequest) ([]UserResponse, error) {
	users, err := s.repo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return ToUserResponses(users), nil
}

func (s *userService) Update(ctx context.Context, req UpdateUserRequest) (*UserResponse, error) {
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}

	u, err := s.repo.Detail(ctx, UserDetailRequest{ID: req.ID})
	if err != nil {
		return nil, err
	}

	return ToUserResponse(u), nil
}

func (s *userService) Delete(ctx context.Context, req DeleteUserRequest) error {
	return s.repo.Delete(ctx, req)
}
