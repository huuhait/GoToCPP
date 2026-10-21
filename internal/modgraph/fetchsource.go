package modgraph

import (
	"context"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
	"github.com/huuhait/gocpp/internal/modfetch"
)

// A FetcherSource is a Source on top of a modfetch.Fetcher: the module cache,
// the module proxies and the git hosts of the configuration.
type FetcherSource struct {
	F       *modfetch.Fetcher
	MainDir string // where gocpp.sum is

	// Pending, if set, collects the gocpp.sum lines of the module files that are
	// read (gocpp get writes them with the new gocpp.mod); with no Pending the
	// lines are written at once.
	Pending *modfetch.PendingSums

	// NoRecord makes a module file that gocpp.sum does not list an error
	// (modfetch.MissingSumError) instead of trusting its first download.
	NoRecord bool
}

// ModFile implements Source.
func (s *FetcherSource) ModFile(ctx context.Context, path, version string) (*modfetch.ModFileInfo, error) {
	return s.F.ModFile(ctx, s.MainDir, path, version, modfetch.FetchOptions{Record: !s.NoRecord, Pending: s.Pending})
}

// Versions implements Source.
func (s *FetcherSource) Versions(ctx context.Context, path string) ([]string, error) {
	return s.F.Versions(ctx, path)
}

// Retractions implements RetractionSource.
func (s *FetcherSource) Retractions(ctx context.Context, path string) ([]*gocppmod.Retract, error) {
	return s.F.Retractions(ctx, s.MainDir, path)
}
