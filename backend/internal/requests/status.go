package requests

import "github.com/Cristian0711/media-bridge/backend/internal/pipeline"

// Request types and lifecycle statuses are defined in internal/pipeline so the
// download/remove/health packages can share them without import cycles. These
// aliases keep the requests package terse.
const (
	TypeMovieDownload = pipeline.TypeMovieDownload
	TypeShowDownload  = pipeline.TypeShowDownload
	TypeMovieRemove   = pipeline.TypeMovieRemove
	TypeShowRemove    = pipeline.TypeShowRemove

	StatusPending     = pipeline.StatusPending
	StatusQueued      = pipeline.StatusQueued
	StatusDownloading = pipeline.StatusDownloading
	StatusDownloaded  = pipeline.StatusDownloaded
	StatusRemoving    = pipeline.StatusRemoving
	StatusRemoved     = pipeline.StatusRemoved
	StatusFailed      = pipeline.StatusFailed
	StatusCancelled   = pipeline.StatusCancelled
)

// activeDownloadStatuses lists the states that count as "this download is
// already in motion" — waiting in the requests queue or being processed by the
// download/hardlink queue. 'downloaded' is intentionally excluded; once a
// download is finalized, FindMovieIDsByExternalIDAndQuality catches the
// duplicate via the media row.
var activeDownloadStatuses = []string{StatusPending, StatusQueued, StatusDownloading}

// cancellableDownloadStatusesForRemove includes 'downloaded' so remove can clean
// up requests finalized just before cancel runs (R5).
var cancellableDownloadStatusesForRemove = []string{StatusPending, StatusQueued, StatusDownloading, StatusDownloaded}

// activeRemoveStatuses lists the states that count as "a remove is already in
// motion" for a media_id. Once a remove is fully done the media row is gone so a
// fresh remove POST simply 404s upstream.
var activeRemoveStatuses = []string{StatusPending, StatusRemoving}

// terminalRequestStatuses are the end states eligible for retention purge (R9).
var terminalRequestStatuses = []string{StatusDownloaded, StatusRemoved, StatusFailed, StatusCancelled}

// downloadTypes / removeTypes group request types by pipeline.
var (
	downloadTypes = pipeline.DownloadTypes
	removeTypes   = pipeline.RemoveTypes
)

func isDownloadType(t string) bool {
	return pipeline.IsDownloadType(t)
}
