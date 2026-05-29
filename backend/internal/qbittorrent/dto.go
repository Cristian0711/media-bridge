package qbittorrent

type AddTorrentRequest struct {
	TorrentBase64 string `json:"torrent_base64" binding:"required"`
	SavePath      string `json:"save_path"`
	TorrentName   string `json:"torrent_name" binding:"required"`
}

type AddTorrentResponse struct {
	Hash string `json:"hash"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type Torrent struct {
	Hash       string  `json:"hash"`
	Name       string  `json:"name"`
	Size       int64   `json:"size"`
	Progress   float64 `json:"progress"`
	State      string  `json:"state"`
	Seeders    int     `json:"seeders"`
	Leechers   int     `json:"leechers"`
	Downloaded int64   `json:"downloaded"`
	Uploaded   int64   `json:"uploaded"`
	DlSpeed    int64   `json:"dlspeed"`
	UpSpeed    int64   `json:"upspeed"`
	ETA        int64   `json:"eta"`
}

type PaginatedTorrentsResponse struct {
	Torrents   []Torrent `json:"torrents"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	TotalCount int       `json:"total_count"`
	TotalPages int       `json:"total_pages"`
}

type TorrentFile struct {
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float32 `json:"progress"`
}

type TorrentStatusResponse struct {
	Hash     string  `json:"hash"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
}
