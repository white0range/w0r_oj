package dto

import "time"

type UserAuthRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type UserProfileResponse struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	Role        int    `json:"role"`
	Status      string `json:"status"`
	SolvedCount int    `json:"solved_count"`
	SolvedList  []uint `json:"solved_list"`
}

type BanUserRequest struct {
	Reason string `json:"reason"`
}

type AdminUserItem struct {
	ID          uint       `json:"id"`
	Username    string     `json:"username"`
	Role        int        `json:"role"`
	Status      string     `json:"status"`
	SolvedCount int        `json:"solved_count"`
	BanReason   string     `json:"ban_reason"`
	BannedAt    *time.Time `json:"banned_at"`
	CreatedAt   time.Time  `json:"created_at"`
}
