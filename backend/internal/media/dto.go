package media

import "errors"

var ErrMediaNotFound = errors.New("media not found")

type PaginatedMediaResponse struct {
	Media          []Media `json:"media"`
	Page           int     `json:"page"`
	PageSize       int     `json:"page_size"`
	TotalCount     int64   `json:"total_count"`
	TotalSizeBytes int64   `json:"total_size_bytes"`
	TotalPages     int     `json:"total_pages"`
}
