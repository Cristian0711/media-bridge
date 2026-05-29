package requests

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
	downloadTypes = []string{TypeMovieDownload, TypeShowDownload}
	removeTypes   = []string{TypeMovieRemove, TypeShowRemove}
)

func isDownloadType(t string) bool {
	return t == TypeMovieDownload || t == TypeShowDownload
}

func isRemoveType(t string) bool {
	return t == TypeMovieRemove || t == TypeShowRemove
}
