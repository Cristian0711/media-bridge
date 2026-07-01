package pagination_test

import (
	"testing"

	"github.com/Cristian0711/media-bridge/backend/shared/pagination"
)

func TestNormalize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		page, size        int
		wantPage, wantSiz int
	}{
		{"defaults below range", 0, 0, 1, pagination.DefaultPageSize},
		{"negative page", -5, 25, 1, 25},
		{"over max size", 1, pagination.MaxPageSize + 1, 1, pagination.DefaultPageSize},
		{"at max size", 2, pagination.MaxPageSize, 2, pagination.MaxPageSize},
		{"in range", 3, 50, 3, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotPage, gotSize := pagination.Normalize(tc.page, tc.size)
			if gotPage != tc.wantPage || gotSize != tc.wantSiz {
				t.Fatalf("Normalize(%d,%d) = (%d,%d), want (%d,%d)",
					tc.page, tc.size, gotPage, gotSize, tc.wantPage, tc.wantSiz)
			}
		})
	}
}

func TestTotalPages(t *testing.T) {
	t.Parallel()
	cases := []struct {
		total int64
		size  int
		want  int
	}{
		{0, 20, 0},
		{-1, 20, 0},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{100, 0, 0}, // guard against divide-by-zero
	}
	for _, tc := range cases {
		if got := pagination.TotalPages(tc.total, tc.size); got != tc.want {
			t.Fatalf("TotalPages(%d,%d) = %d, want %d", tc.total, tc.size, got, tc.want)
		}
	}
}
