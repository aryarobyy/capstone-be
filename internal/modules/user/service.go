package user

import (
	"context"
)

type UserService interface {
	GetByID(ctx context.Context, id int64) (*UserResponse, error)
	GetAll(ctx context.Context) ([]UserResponse, error)
	Update(ctx context.Context, id int64, req UpdateUserRequest) (*UserResponse, error)
	Delete(ctx context.Context, id int64) error
}

type userService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) UserService {
	return &userService{repo: repo}
}

func (s *userService) GetByID(ctx context.Context, id int64) (*UserResponse, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return ToUserResponse(u), nil
}

func (s *userService) GetAll(ctx context.Context) ([]UserResponse, error) {
	users, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return ToUserResponses(users), nil
}

func (s *userService) Update(ctx context.Context, id int64, req UpdateUserRequest) (*UserResponse, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		u.Name = req.Name
	}
	if req.Email != "" {
		u.Email = req.Email
	}

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}

	return ToUserResponse(u), nil
}

func (s *userService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
