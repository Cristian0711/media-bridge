package users

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `gorm:"primarykey"        json:"id"`
	Username     string         `gorm:"uniqueIndex;not null" json:"username"`
	Role         string         `gorm:"not null;default:user" json:"role"`
	PasswordHash string         `gorm:"not null"          json:"-"`
	CreatedAt    time.Time      `                          json:"created_at"`
	UpdatedAt    time.Time      `                          json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index"             json:"-"`
}
