package pipeline_test

import (
	"testing"

	"github.com/Cristian0711/media-bridge/backend/internal/pipeline"
)

func TestIsDownloadType(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		pipeline.TypeMovieDownload: true,
		pipeline.TypeShowDownload:  true,
		pipeline.TypeMovieRemove:   false,
		pipeline.TypeShowRemove:    false,
		"nonsense":                 false,
		"":                         false,
	}
	for in, want := range cases {
		if got := pipeline.IsDownloadType(in); got != want {
			t.Errorf("IsDownloadType(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsRemoveType(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		pipeline.TypeMovieRemove:   true,
		pipeline.TypeShowRemove:    true,
		pipeline.TypeMovieDownload: false,
		pipeline.TypeShowDownload:  false,
		"nonsense":                 false,
	}
	for in, want := range cases {
		if got := pipeline.IsRemoveType(in); got != want {
			t.Errorf("IsRemoveType(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestTypeGroupsPartitionAllRequestTypes guards that every request type is
// classified as exactly one of download/remove and that AllRequestTypes is the
// union — a new type added without updating the groups would trip this.
func TestTypeGroupsPartitionAllRequestTypes(t *testing.T) {
	t.Parallel()
	if len(pipeline.DownloadTypes)+len(pipeline.RemoveTypes) != len(pipeline.AllRequestTypes) {
		t.Fatalf("download(%d)+remove(%d) != all(%d)",
			len(pipeline.DownloadTypes), len(pipeline.RemoveTypes), len(pipeline.AllRequestTypes))
	}
	for _, tp := range pipeline.AllRequestTypes {
		if pipeline.IsDownloadType(tp) == pipeline.IsRemoveType(tp) {
			t.Errorf("type %q must be exactly one of download/remove", tp)
		}
	}
	for _, tp := range pipeline.DownloadTypes {
		if !pipeline.IsDownloadType(tp) {
			t.Errorf("DownloadTypes contains %q but IsDownloadType is false", tp)
		}
	}
	for _, tp := range pipeline.RemoveTypes {
		if !pipeline.IsRemoveType(tp) {
			t.Errorf("RemoveTypes contains %q but IsRemoveType is false", tp)
		}
	}
}
