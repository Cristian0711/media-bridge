package qbittorrent

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	gotorrentparser "github.com/j-muller/go-torrent-parser"
)

func torrentHasRootFolder(fileBytes []byte) (bool, error) {
	torrent, err := gotorrentparser.Parse(bytes.NewReader(fileBytes))
	if err != nil {
		return false, err
	}
	if len(torrent.Files) == 0 {
		return false, fmt.Errorf("torrent has no files")
	}
	return len(torrent.Files[0].Path) > 1, nil
}

func torrentHash(fileBytes []byte) (string, error) {
	torrent, err := gotorrentparser.Parse(bytes.NewReader(fileBytes))
	if err != nil {
		return "", err
	}
	return NormalizeHash(torrent.InfoHash), nil
}

func extractRootFolder(filePath string) (string, error) {
	filePath = filepath.ToSlash(filepath.Clean(filePath))
	parts := strings.Split(filePath, "/")
	if len(parts) <= 1 || parts[0] == "" {
		return "", fmt.Errorf("flat file path")
	}
	return parts[0], nil
}
