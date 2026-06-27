package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	UserStatusActive = "active"
	UserStatusBanned = "banned"
)

type User struct {
	gorm.Model

	Username string `gorm:"type:varchar(50);unique;not null" json:"username"`
	Password string `gorm:"type:varchar(255);not null" json:"-"`

	SolvedCount  int        `gorm:"default:0" json:"solved_count"`
	Role         int        `gorm:"type:tinyint;default:0;comment:0-normal user, 1-admin" json:"role"`
	Status       string     `gorm:"type:varchar(20);not null;default:'active'" json:"status"`
	BanReason    string     `gorm:"type:text" json:"ban_reason"`
	BannedAt     *time.Time `json:"banned_at"`
	TokenVersion int        `gorm:"not null;default:1" json:"token_version"`
}

type UserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}
