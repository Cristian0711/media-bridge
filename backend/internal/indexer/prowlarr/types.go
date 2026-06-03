package prowlarr

// Release matches Prowlarr /api/v1/search JSON (see prowlar_response.json).
type Release struct {
	GUID         string     `json:"guid"`
	Title        string     `json:"title"`
	SortTitle    string     `json:"sortTitle"`
	IndexerID    int        `json:"indexerId"`
	Indexer      string     `json:"indexer"`
	IMDBID       int        `json:"imdbId"`
	TMDBID       int        `json:"tmdbId"`
	TVDBID       int        `json:"tvdbId"`
	Size         int64      `json:"size"`
	Files        int        `json:"files"`
	Grabs        int        `json:"grabs"`
	Seeders      int        `json:"seeders"`
	Leechers     int        `json:"leechers"`
	PublishDate  string     `json:"publishDate"`
	DownloadURL  string     `json:"downloadUrl"`
	InfoURL      string     `json:"infoUrl"`
	IndexerFlags []string   `json:"indexerFlags"`
	Categories   []Category `json:"categories"`
	Protocol     string     `json:"protocol"`
	FileName     string     `json:"fileName"`
}

type Category struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	SubCategories []Category `json:"subCategories"`
}

// Indexer is a configured Prowlarr indexer (GET /api/v1/indexer).
type Indexer struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Enable  bool   `json:"enable"`
	Enabled bool   `json:"enabled"` // some API versions use "enabled"
}

func (i Indexer) IsEnabled() bool {
	return i.Enable || i.Enabled
}
