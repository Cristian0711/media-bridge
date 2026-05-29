package requests

// TorrentInfoInvalidator drops cached torrent-modal payloads when request state changes.
type TorrentInfoInvalidator interface {
	InvalidateTorrentInfo(requestID uint)
}

// torrentInfoCacheInvalidator is implemented by *repository.
type torrentInfoCacheInvalidator interface {
	SetTorrentInfoInvalidator(TorrentInfoInvalidator)
}
