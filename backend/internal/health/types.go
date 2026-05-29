package health

import "time"

// Overall status for the report.
const (
	StatusHealthy  = "healthy"
	StatusDegraded = "degraded"
	StatusUnhealthy = "unhealthy"
)

// Per-check status.
const (
	CheckOK   = "ok"
	CheckWarn = "warn"
	CheckFail = "fail"
	CheckSkip = "skip"
)

// Report is the full diagnostics payload for the settings dashboard.
type Report struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
	Checks    []Check   `json:"checks"`
}

// Check is one health probe with optional structured details.
type Check struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	Message    string         `json:"message"`
	DurationMS int64          `json:"duration_ms"`
	Details    map[string]any `json:"details,omitempty"`
}

// LinkIssue describes a file that failed the hardlink (nlink >= 2) expectation.
type LinkIssue struct {
	Path   string `json:"path"`
	Zone   string `json:"zone"`
	NLink  uint64 `json:"nlink"`
	Reason string `json:"reason"`
}
