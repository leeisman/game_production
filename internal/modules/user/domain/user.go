package domain

import (
	"time"
)

// User represents a user in the system
type User struct {
	UserID       int64      `json:"user_id" gorm:"primaryKey;column:user_id;autoIncrement"`
	Username     string     `json:"username" gorm:"column:username;unique;not null"`
	PasswordHash string     `json:"-" gorm:"column:password_hash;not null"`
	Email        string     `json:"email" gorm:"column:email;unique;not null"`
	Status       int        `json:"status" gorm:"column:status;default:1"`
	CreatedAt    time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty" gorm:"column:last_login_at"`
}

// Session represents a user session
type Session struct {
	SessionID string    `json:"session_id" gorm:"primaryKey;column:session_id"`
	UserID    int64     `json:"user_id" gorm:"column:user_id;index"`
	Token     string    `json:"token" gorm:"column:token;index"`
	ExpiresAt time.Time `json:"expires_at" gorm:"column:expires_at;index"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

// User status constants
const (
	UserStatusActive    = 1
	UserStatusSuspended = 2
	UserStatusBanned    = 3
)

// IsActive checks if user is active
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}
