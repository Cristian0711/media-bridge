package indexer

type SearchRequest struct {
	Name     string   `json:"name"`
	Year     int      `json:"year,omitempty"`
	ImdbID   string   `json:"imdb_id,omitempty"`
	Season   int      `json:"season,omitempty"`
	Episode  int      `json:"episode,omitempty"`
	Quality  string   `json:"quality,omitempty"`
	Indexers []string `json:"indexers,omitempty"`
}

type IndexerItem struct {
	ID           string
	Name         string
	ImdbID       string
	Size         int64
	Seeders      int
	Leechers     int
	Downloads    int
	DownloadLink string
	Freeleech    bool
	Category     string
	UploadDate   string
	IndexerName  string
}

type Movie struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Imdb         string `json:"imdb"`
	Freeleech    int    `json:"freeleech"`
	Size         int64  `json:"size"`
	Category     string `json:"category"`
	Seeders      int    `json:"seeders"`
	Leechers     int    `json:"leechers"`
	Downloads    int    `json:"times_completed"`
	DownloadLink string `json:"download_link"`
	Quality      string `json:"quality"`
	IndexerName  string `json:"indexer_name"`
}

type Show struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Imdb         string `json:"imdb"`
	Freeleech    int    `json:"freeleech"`
	Size         int64  `json:"size"`
	Category     string `json:"category"`
	Seeders      int    `json:"seeders"`
	Leechers     int    `json:"leechers"`
	Downloads    int    `json:"times_completed"`
	DownloadLink string `json:"download_link"`
	Quality      string `json:"quality"`
	IndexerName  string `json:"indexer_name"`
	Season       int    `json:"season"`
	Episode      int    `json:"episode"`
	Complete     bool   `json:"complete_season"`
}
