package health

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const scanLogRetention = 14 * 24 * time.Hour

type Repository interface {
	SaveScan(ctx context.Context, report Report, fullScan bool, durationMS int64) (*ScanLog, error)
	ListScans(ctx context.Context, page, pageSize int) ([]ScanLogSummary, int64, error)
	GetScan(ctx context.Context, id uint) (*ScanLog, error)
	LatestScan(ctx context.Context) (*ScanLogSummary, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// ScanLogSummary is the list-row view (no full report JSON).
type ScanLogSummary struct {
	ID         uint      `json:"id"`
	CheckedAt  time.Time `json:"checked_at"`
	Status     string    `json:"status"`
	FullScan   bool      `json:"full_scan"`
	DurationMS int64     `json:"duration_ms"`
	FailCount  int       `json:"fail_count"`
	WarnCount  int       `json:"warn_count"`
}

type PaginatedScanLogsResponse struct {
	Scans      []ScanLogSummary `json:"scans"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalCount int64            `json:"total_count"`
	TotalPages int              `json:"total_pages"`
}

func (r *repository) SaveScan(ctx context.Context, report Report, fullScan bool, durationMS int64) (*ScanLog, error) {
	raw, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	fail, warn := countCheckStatuses(report.Checks)
	row := &ScanLog{
		CheckedAt:  report.CheckedAt,
		Status:     report.Status,
		FullScan:   fullScan,
		DurationMS: durationMS,
		FailCount:  fail,
		WarnCount:  warn,
		Report:     raw,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	cutoff := time.Now().UTC().Add(-scanLogRetention)
	_ = r.db.WithContext(ctx).Where("checked_at < ?", cutoff).Delete(&ScanLog{}).Error
	return row, nil
}

func (r *repository) ListScans(ctx context.Context, page, pageSize int) ([]ScanLogSummary, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := r.db.WithContext(ctx).Model(&ScanLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []ScanLog
	if err := r.db.WithContext(ctx).
		Select("id", "checked_at", "status", "full_scan", "duration_ms", "fail_count", "warn_count").
		Order("checked_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]ScanLogSummary, len(rows))
	for i := range rows {
		out[i] = scanLogToSummary(&rows[i])
	}
	return out, total, nil
}

func (r *repository) GetScan(ctx context.Context, id uint) (*ScanLog, error) {
	var row ScanLog
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *repository) LatestScan(ctx context.Context) (*ScanLogSummary, error) {
	var row ScanLog
	if err := r.db.WithContext(ctx).
		Select("id", "checked_at", "status", "full_scan", "duration_ms", "fail_count", "warn_count").
		Order("checked_at DESC").
		First(&row).Error; err != nil {
		return nil, err
	}
	s := scanLogToSummary(&row)
	return &s, nil
}

func scanLogToSummary(row *ScanLog) ScanLogSummary {
	return ScanLogSummary{
		ID:         row.ID,
		CheckedAt:  row.CheckedAt,
		Status:     row.Status,
		FullScan:   row.FullScan,
		DurationMS: row.DurationMS,
		FailCount:  row.FailCount,
		WarnCount:  row.WarnCount,
	}
}

func countCheckStatuses(checks []Check) (fail, warn int) {
	for _, c := range checks {
		switch c.Status {
		case CheckFail:
			fail++
		case CheckWarn:
			warn++
		}
	}
	return fail, warn
}

func totalPages(total int64, pageSize int) int {
	if total == 0 {
		return 1
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}
