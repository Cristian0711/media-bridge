package health

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Report returns diagnostics. Query ?full=1 runs filesystem hardlink audit.
// When ?persist=1 the result is appended to the scan log.
func (h *Handler) Report(c *gin.Context) {
	full, _ := strconv.ParseBool(c.Query("full"))
	persist, _ := strconv.ParseBool(c.Query("persist"))

	if persist {
		report, summary, err := h.svc.RunAndPersist(c.Request.Context(), full)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save scan"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":     report.Status,
			"checked_at": report.CheckedAt,
			"checks":     report.Checks,
			"scan_id":    summary.ID,
		})
		return
	}

	var report Report
	if full {
		report = h.svc.FullReport(c.Request.Context())
	} else {
		report = h.svc.QuickReport(c.Request.Context())
	}
	c.JSON(http.StatusOK, report)
}

func (h *Handler) ListScans(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	resp, err := h.svc.ListScans(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) LatestScan(c *gin.Context) {
	row, err := h.svc.LatestScan(c.Request.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"scan": nil})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"scan": row})
}

func (h *Handler) GetScan(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid scan id"})
		return
	}
	row, err := h.svc.GetScan(c.Request.Context(), uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "scan not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	var report Report
	if err := json.Unmarshal(row.Report, &report); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "corrupt scan record"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          row.ID,
		"checked_at":  row.CheckedAt,
		"status":      row.Status,
		"full_scan":   row.FullScan,
		"duration_ms": row.DurationMS,
		"fail_count":  row.FailCount,
		"warn_count":  row.WarnCount,
		"report":      report,
	})
}
