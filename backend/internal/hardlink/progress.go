package hardlink

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Cristian0711/media-bridge/backend/internal/media"
)

// Artifact is one torrent file and whether it is hardlinked into the library path.
type Artifact struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Linked bool   `json:"linked"`
}

// Progress summarizes hardlink work for a media row.
type Progress struct {
	Linked      int        `json:"linked"`
	Total       int        `json:"total"`
	Complete    bool       `json:"complete"`
	Done        []Artifact `json:"done"`
	Remaining   []Artifact `json:"remaining"`
	TorrentHash string     `json:"-"` // set when built; used by callers without reloading media
}

// clone returns a deep copy of the progress, including its slices, so callers
// (and the progress cache) can hand out independent copies. Mutating one copy —
// e.g. appending to Done/Remaining — must never race or corrupt another.
//
// The slices are copied into non-nil empties so an empty list stays an empty
// list (JSON []), not null: API/SSE consumers rely on done/remaining always
// being arrays. (append([]Artifact(nil), empty...) would yield nil.)
func (p *Progress) clone() *Progress {
	if p == nil {
		return nil
	}
	cp := *p
	cp.Done = append(make([]Artifact, 0, len(p.Done)), p.Done...)
	cp.Remaining = append(make([]Artifact, 0, len(p.Remaining)), p.Remaining...)
	return &cp
}

func (s *service) Progress(ctx context.Context, mediaID uint) (*Progress, error) {
	row, err := s.mediaService.GetMediaByID(ctx, mediaID)
	if err != nil {
		if errors.Is(err, media.ErrMediaNotFound) {
			return &Progress{}, nil
		}
		return nil, fmt.Errorf("load media %d: %w", mediaID, err)
	}
	return s.ProgressForMedia(ctx, row)
}

func (s *service) ProgressForMedia(ctx context.Context, row *media.Media) (*Progress, error) {
	if row == nil {
		return &Progress{}, nil
	}
	return s.progressForMedia(ctx, row)
}

func (s *service) progressForMedia(ctx context.Context, row *media.Media) (*Progress, error) {
	torrentHash, savePath, destinationPath, err := s.hardlinkPaths(row)
	if err != nil {
		return nil, err
	}

	out := &Progress{
		TorrentHash: torrentHash,
		Done:        []Artifact{},
		Remaining:   []Artifact{},
	}

	files, err := s.qbitService.GetTorrentFiles(ctx, torrentHash)
	if err != nil {
		return nil, fmt.Errorf("get torrent files: %w", err)
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		displayName := normalizeTorrentPath(file.Name)
		if displayName == "" {
			displayName = filepath.Base(file.Name)
		}
		sourcePath := buildSourcePath(savePath, file.Name)
		destPath := filepath.Join(destinationPath, displayName)

		out.Total++
		linked := hardlinkPresent(sourcePath, destPath)

		artifact := Artifact{Name: displayName, Size: file.Size, Linked: linked}
		if linked {
			out.Linked++
			out.Done = append(out.Done, artifact)
		} else {
			out.Remaining = append(out.Remaining, artifact)
		}
	}

	if out.Total == 0 {
		return out, nil
	}
	out.Complete = out.Linked == out.Total
	return out, nil
}

func (s *service) hardlinkPaths(row *media.Media) (torrentHash, savePath, destinationPath string, err error) {
	switch row.Type {
	case media.MediaTypeMovie:
		return s.movieHardlinkPaths(row)
	case media.MediaTypeShowFull, media.MediaTypeShowSeason, media.MediaTypeShowEpisode:
		return s.showHardlinkPaths(row)
	default:
		return "", "", "", fmt.Errorf("unsupported media type for hardlink: %s", row.Type)
	}
}
