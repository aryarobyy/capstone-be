package user

import "time"

type ListUserRequest struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

type UserDetailRequest struct {
	ID int64 `json:"id" binding:"required"`
}

type UpdateUserRequest struct {
	ID     int64  `json:"id" binding:"required"`
	Name   string `json:"name" binding:"omitempty,min=2,max=100"`
	Email  string `json:"email" binding:"omitempty,email"`
	Msisdn string `json:"msisdn" binding:"omitempty,min=2,max=100"`
}

type DeleteUserRequest struct {
	ID int64 `json:"id" binding:"required"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Msisdn    string    `json:"msisdn"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ToUserResponse(u *User) *UserResponse {
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

func ToUserResponses(users []User) []UserResponse {
	res := make([]UserResponse, len(users))
	for i, u := range users {
		res[i] = *ToUserResponse(&u)
	}
	return res
}
