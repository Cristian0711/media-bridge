package indexer

import "context"

func processMovieItems(_ context.Context, items []IndexerItem) []Movie {
	movies := make([]Movie, 0, len(items))
	for _, item := range items {
		movie := Movie{
			ID:           parseID(item.ID),
			Name:         item.Name,
			Imdb:         item.ImdbID,
			Freeleech:    boolToInt(item.Freeleech),
			Size:         item.Size,
			Category:     item.Category,
			Seeders:      item.Seeders,
			Leechers:     item.Leechers,
			Downloads:    item.Downloads,
			DownloadLink: item.DownloadLink,
			Quality:      parseQuality(item.Name),
			IndexerName:  item.IndexerName,
		}
		movies = append(movies, movie)
	}
	return movies
}

func processShowItems(_ context.Context, items []IndexerItem) []Show {
	shows := make([]Show, 0, len(items))
	for _, item := range items {
		season, episode, complete := parseSeasonEpisode(item.Name)
		show := Show{
			ID:           parseID(item.ID),
			Name:         item.Name,
			Imdb:         item.ImdbID,
			Freeleech:    boolToInt(item.Freeleech),
			Size:         item.Size,
			Category:     item.Category,
			Seeders:      item.Seeders,
			Leechers:     item.Leechers,
			Downloads:    item.Downloads,
			DownloadLink: item.DownloadLink,
			Quality:      parseQuality(item.Name),
			IndexerName:  item.IndexerName,
			Season:       season,
			Episode:      episode,
			Complete:     complete,
		}
		shows = append(shows, show)
	}
	return shows
}
