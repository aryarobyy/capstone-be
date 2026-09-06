package user

import (
	"context"
)

type UserService interface {
	Detail(ctx context.Context, req UserDetailRequest) (*UserResponse, error)
	List(ctx context.Context, req ListUserRequest) (*ListUserResponse, error)
	Update(ctx context.Context, req UpdateUserRequest) (*UserResponse, error)
	Delete(ctx context.Context, req DeleteUserRequest) error
}

type userService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) UserService {
	return &userService{repo: repo}
}

func toUserResponse(u *User) *UserResponse {
	if u == nil {
		return nil
	}
	return &UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Msisdn:    u.Msisdn,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func (s *userService) Detail(ctx context.Context, req UserDetailRequest) (*UserResponse, error) {
	u, err := s.repo.Detail(ctx, req)
	if err != nil {
		return nil, err
	}
	return toUserResponse(u), nil
}

func (s *userService) List(ctx context.Context, req ListUserRequest) (*ListUserResponse, error) {
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

	data := make([]ListUserData, 0, len(users))
	for _, u := range users {
		data = append(data, ListUserData{
			ID:        u.ID,
			Name:      u.Name,
			Email:     u.Email,
			Msisdn:    u.Msisdn,
			CreatedAt: u.CreatedAt,
			UpdatedAt: u.UpdatedAt,
		})
	}

	return &ListUserResponse{
		Data:  data,
		Total: count,
		Limit: limit,
		Index: index,
	}, nil
}

func (s *userService) Update(ctx context.Context, req UpdateUserRequest) (*UserResponse, error) {
	if err := s.repo.Update(ctx, req); err != nil {
		return nil, err
	}

	u, err := s.repo.Detail(ctx, UserDetailRequest{ID: req.ID})
	if err != nil {
		return nil, err
	}

	return toUserResponse(u), nil
}

func (s *userService) Delete(ctx context.Context, req DeleteUserRequest) error {
	return s.repo.Delete(ctx, req)
}
