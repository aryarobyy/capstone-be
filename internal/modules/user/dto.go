package user

import "time"

type ListUserRequest struct {
	Limit int `json:"limit"`
	Index int `json:"index"`
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

type ListUserData struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Msisdn    string    `json:"msisdn"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListUserResponse struct {
	Data  []ListUserData `json:"data"`
	Total int            `json:"total"`
	Limit int            `json:"limit"`
	Index int            `json:"index"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Msisdn    string    `json:"msisdn"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
