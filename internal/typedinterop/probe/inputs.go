package probe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// InputGuard binds the standard probe preamble and user includes across a composed background
// operation. Individual probe caches validate their own products; this also prevents joining
// a receiver type from one header version with member names from another version.
type InputGuard struct {
	required   indexDependencies
	resolution *indexResolution
	deps       []string
	hash       string
	start      time.Time
}

// CaptureInputs uses the existing dependency/shadow/identity machinery. Both this call and
// Unchanged perform compiler or filesystem work and belong only in background workers.
func CaptureInputs(ctx context.Context, cfg Config, includes []string) (*InputGuard, error) {
	includeBytes := 0
	for _, include := range includes {
		includeBytes += len(include) + 16
		if includeBytes > 65536 {
			return nil, errors.New("composed probe includes exceed their source budget")
		}
		if err := interop.ValidateInclude(include); err != nil {
			return nil, err
		}
	}
	start := time.Now()
	source := Generate(collect.NewSet(), GenConfig{Includes: includes})
	_, output, err := indexOutput(ctx, cfg, []string{"-E", "-o", os.DevNull}, source.Text)
	if err != nil {
		return nil, err
	}
	metadata := new(Runner)
	search, err := metadata.searchDirsChecked(ctx, cfg)
	if err != nil {
		return nil, err
	}
	// Clang may print relative -I directories supplied by its wrapper or CXX.Args. They
	// resolve in the compiler's working directory, not the process calling this guard.
	base := cfg.Dir
	if !filepath.IsAbs(base) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		base = joinRaw(cwd, base)
	}
	search = slices.Clone(search)
	for i, dir := range search {
		if !filepath.IsAbs(dir) {
			search[i] = joinRaw(base, dir)
		}
	}
	tc := toolchain{Dirs: search}
	read := interop.ParseDeps(output.depData, output.srcPath, cfg.Dir)
	slices.Sort(read)
	read = slices.Compact(read)
	deps := metadata.finalDeps(ctx, cfg, source.Text, includes, read, nil, tc)
	info := metadata.includeInfo(ctx, cfg, source.Text, read, false, "")
	if output.truncated || len(read) == 0 || !info.complete || !info.successful || racyDeps(deps, start) {
		return nil, errors.New("composed probe inputs could not be validated")
	}
	required, err := captureIndexDeps(read)
	if err != nil {
		return nil, err
	}
	resolution, err := captureIndexResolution(append(slices.Clone(deps), cfg.Dir))
	if err != nil {
		return nil, err
	}
	return &InputGuard{required, resolution, deps, DepsHash(deps), start}, nil
}

// Unchanged rejects both changed required files and newly present shadow candidates.
func (g *InputGuard) Unchanged(ctx context.Context) bool {
	return g != nil && ctx.Err() == nil && g.required.unchanged() &&
		g.resolution.unchanged() && !racyDeps(g.deps, g.start) && DepsHash(g.deps) == g.hash
}
