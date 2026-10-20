package cache

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// An ExpandCheck is the full check of a package for "show expansion" (step 11.13): the check
// of gocpp build, with the bodies of #Derive written, which the checks of the pipeline leave out
// (check.Config.NoDerivedBodies), and the nodes of the loader before the check changed them.
type ExpandCheck struct {
	Program *load.Program
	Result  *check.Result
	Root    *load.Package
	// Known holds the nodes of the files of the package as the loader made them: the user's nodes and the
	// ones the loader's rewrites wrote. The check adds its own after that (package expand needs the
	// difference).
	Known map[ast.Node]bool
}

// CheckExpand loads and checks the package in the folder dir like gocpp expand does, from the files of
// the snapshot (the open documents too). It costs a load of the package and all of its dependencies from
// source, like CheckTests: only an explicit command may call it, never a request that must be fast. The
// check is not cached.
func (s *Snapshot) CheckExpand(ctx context.Context, dir string) (*ExpandCheck, error) {
	st, ok := s.state.(*state)
	if !ok {
		return nil, errNoPipeline
	}
	dir = filepath.Clean(dir)
	p := st.p
	h, err := st.handle(ctx, pkgID{dir: dir}, "", nil)
	if err != nil {
		return nil, err
	}
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-p.sem }()

	pattern, loadDir := h.loadPattern()
	cfg := p.loadConfig(h, loadDir, false)
	cfg.Fset = token.NewFileSet()
	cfg.FS = stateFS{s: st}
	prog := load.Load(cfg, pattern)
	if len(prog.Roots) == 0 {
		return nil, fmt.Errorf("cache: %w in %s", ErrNoPackage, dir)
	}
	root := prog.Roots[0]
	if len(root.Files) == 0 {
		return nil, fmt.Errorf("cache: no .gp files in %s", dir)
	}
	known := make(map[ast.Node]bool)
	for _, f := range root.Files {
		if f.AST == nil {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if n != nil {
				known[n] = true
			}
			return true
		})
	}
	rec := &probeRecord{pr: p.prober}
	r := check.Check(prog, check.Config{
		Phase:     p.cfg.Phase,
		GOOS:      p.cfg.GOOS,
		GOARCH:    p.cfg.GOARCH,
		Context:   ctx,
		CXX:       p.prober.findCXX(rec),
		Probe:     p.prober.probeFor(rec),
		PkgConfig: p.prober.pkgConfigFor(rec),
		CHeader:   p.prober.cheaderFor(rec),
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &ExpandCheck{Program: prog, Result: r, Root: root, Known: known}, nil
}
