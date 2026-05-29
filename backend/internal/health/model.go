package health

import "time"

// ScanLog stores one completed health report for the diagnostics history UI.
type ScanLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CheckedAt  time.Time `gorm:"not null;index" json:"checked_at"`
	Status     string    `gorm:"type:varchar(20);not null;index" json:"status"`
	FullScan   bool      `gorm:"not null;default:false" json:"full_scan"`
	DurationMS int64     `gorm:"not null;default:0" json:"duration_ms"`
	FailCount  int       `gorm:"not null;default:0" json:"fail_count"`
	WarnCount  int       `gorm:"not null;default:0" json:"warn_count"`
	Report     []byte    `gorm:"type:jsonb;not null" json:"report"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ScanLog) TableName() string { return "health_scan_logs" }
