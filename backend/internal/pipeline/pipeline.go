// Package pipeline holds the dependency-free vocabulary shared by the request
// processing pipeline: request types, lifecycle statuses, and queue names.
//
// It exists so that requests, download, remove, and health can agree on these
// values without import cycles (requests imports download and remove, so the
// constants cannot live in requests).
package pipeline

// Request types route a request to the download or remove pipeline.
const (
	TypeMovieDownload = "movie_download"
	TypeShowDownload  = "show_download"
	TypeMovieRemove   = "movie_remove"
	TypeShowRemove    = "show_remove"
)

// Request lifecycle statuses.
const (
	StatusPending     = "pending"
	StatusQueued      = "queued"
	StatusDownloading = "downloading"
	StatusDownloaded  = "downloaded"
	StatusRemoving    = "removing"
	StatusRemoved     = "removed"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
)

// Processing-queue names.
const (
	QueueRequests = "requests_processing_queue"
	QueueDownload = "download_processing_queue"
	QueueHardlink = "hardlink_processing_queue"
	QueueRemove   = "remove_processing_queue"
)

// DownloadTypes / RemoveTypes group request types by pipeline.
var (
	DownloadTypes = []string{TypeMovieDownload, TypeShowDownload}
	RemoveTypes   = []string{TypeMovieRemove, TypeShowRemove}
)

func IsDownloadType(t string) bool {
	return t == TypeMovieDownload || t == TypeShowDownload
}

func IsRemoveType(t string) bool {
	return t == TypeMovieRemove || t == TypeShowRemove
}
