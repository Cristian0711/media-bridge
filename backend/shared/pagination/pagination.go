// Package pagination holds the page/size normalization and page-count math
// shared by the list endpoints (media, requests). Keeping it in one place avoids
// the per-package copies that drifted to different page-size caps.
package pagination

// DefaultPageSize is used when the requested size is missing or out of range.
const DefaultPageSize = 20

// MaxPageSize is the upper bound on a single page across all list endpoints.
const MaxPageSize = 100

// Normalize clamps page and pageSize to valid ranges: page >= 1, and pageSize
// in [1, MaxPageSize] (falling back to DefaultPageSize when out of range).
func Normalize(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}
	return page, pageSize
}

// TotalPages returns the number of pages needed to hold total rows at pageSize.
func TotalPages(total int64, pageSize int) int {
	if total <= 0 || pageSize <= 0 {
		return 0
	}
	return int((total + int64(pageSize) - 1) / int64(pageSize))
}
