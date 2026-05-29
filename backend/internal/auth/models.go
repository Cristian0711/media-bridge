package auth

import (
	"time"

	"gorm.io/gorm"
)

// Key is a single-use registration token. Once consumed it is deactivated.
type Key struct {
	ID        uint           `gorm:"primarykey"        json:"id"`
	Value     string         `gorm:"uniqueIndex;not null" json:"value"`
	IsActive  bool           `gorm:"default:true"      json:"is_active"`
	UsedAt    *time.Time     `                          json:"used_at,omitempty"`
	CreatedAt time.Time      `                          json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"             json:"-"`
}