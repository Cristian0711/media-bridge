package search

import (
	"context"
	"strconv"

	"go.uber.org/zap"
)

// AvailabilityQuery identifies a title to check against the server library.
type AvailabilityQuery struct {
	Type string // "movie" | "show"
	IMDB string
	TMDB string
	TVDB string
}

// AvailabilityChecker reports, parallel to the given queries, whether each
// title is present in the library. Implemented by the app layer over the media
// service so this package stays decoupled from the media domain.
type AvailabilityChecker interface {
	Available(ctx context.Context, queries []AvailabilityQuery) ([]bool, error)
}

// SetAvailabilityChecker wires the library availability lookup. When unset,
// annotation is a no-op and results report Available=false.
func (s *Service) SetAvailabilityChecker(checker AvailabilityChecker) {
	s.availability = checker
}

func availabilityQueryFromResult(r Result) (AvailabilityQuery, bool) {
	switch {
	case r.Type == "movie" && r.Movie != nil:
		q := AvailabilityQuery{Type: "movie", IMDB: r.Movie.IDs.IMDB}
		if r.Movie.IDs.TMDB > 0 {
			q.TMDB = strconv.Itoa(r.Movie.IDs.TMDB)
		}
		return q, q.IMDB != "" || q.TMDB != ""
	case r.Type == "show" && r.Show != nil:
		q := AvailabilityQuery{Type: "show", IMDB: r.Show.IDs.IMDB}
		if r.Show.IDs.TVDB > 0 {
			q.TVDB = strconv.Itoa(r.Show.IDs.TVDB)
		}
		return q, q.IMDB != "" || q.TVDB != ""
	default:
		return AvailabilityQuery{}, false
	}
}

// annotateResults returns a copy of results with Available filled in from the
// library. The input slice is left untouched so cached data is never mutated.
func (s *Service) annotateResults(ctx context.Context, results []Result) []Result {
	if s.availability == nil || len(results) == 0 {
		return results
	}

	queries := make([]AvailabilityQuery, 0, len(results))
	idx := make([]int, 0, len(results))
	for i, r := range results {
		if q, ok := availabilityQueryFromResult(r); ok {
			queries = append(queries, q)
			idx = append(idx, i)
		}
	}
	if len(queries) == 0 {
		return results
	}

	avail, err := s.availability.Available(ctx, queries)
	if err != nil || len(avail) != len(queries) {
		if err != nil {
			s.log.Warn("availability annotate failed", zap.Error(err))
		}
		return results
	}

	out := make([]Result, len(results))
	copy(out, results)
	for j, i := range idx {
		out[i].Available = avail[j]
	}
	return out
}

// annotateCatalog returns a copy of the catalog with Available filled in across
// all lists using a single availability lookup. The cached catalog is untouched.
func (s *Service) annotateCatalog(ctx context.Context, catalog *BrowseCatalog) *BrowseCatalog {
	if s.availability == nil || catalog == nil || len(catalog.Lists) == 0 {
		return catalog
	}

	type loc struct{ list, res int }
	var queries []AvailabilityQuery
	var locs []loc
	for li, list := range catalog.Lists {
		for ri, r := range list.Results {
			if q, ok := availabilityQueryFromResult(r); ok {
				queries = append(queries, q)
				locs = append(locs, loc{li, ri})
			}
		}
	}
	if len(queries) == 0 {
		return catalog
	}

	avail, err := s.availability.Available(ctx, queries)
	if err != nil || len(avail) != len(queries) {
		if err != nil {
			s.log.Warn("availability annotate failed", zap.Error(err))
		}
		return catalog
	}

	outLists := make([]BrowseListRow, len(catalog.Lists))
	for li, list := range catalog.Lists {
		res := make([]Result, len(list.Results))
		copy(res, list.Results)
		outLists[li] = list
		outLists[li].Results = res
	}
	for k, l := range locs {
		outLists[l.list].Results[l.res].Available = avail[k]
	}
	return &BrowseCatalog{Lists: outLists}
}
