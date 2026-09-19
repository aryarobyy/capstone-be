package auth

import "time"

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Msisdn   string `json:"msisdn" binding:"required,min=2,max=100"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	InstallationID string `json:"installation_id" binding:"omitempty,uuid"`
	DeviceToken    string `json:"device_token" binding:"required_with=DeviceType,omitempty,max=4096"`
	DeviceType     string `json:"device_type" binding:"required_with=DeviceToken,omitempty,oneof=android ios web"`
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
}

type AuthResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Msisdn    string    `json:"msisdn"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoginResponse embeds the existing user fields for backwards compatibility.
type LoginResponse struct {
	RefreshToken   string `json:"refresh_token"`
	InstallationID string `json:"installation_id"`
	*AuthResponse
	AccessToken           string `json:"access_token"`
	TokenType             string `json:"token_type"`
	ExpiresIn             int64  `json:"expires_in"`
	DeviceTokenRegistered bool   `json:"device_token_registered"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required,len=43"`
}
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}
