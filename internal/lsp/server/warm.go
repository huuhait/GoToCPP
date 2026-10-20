package server

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/safe"
)

// Warming. The first references or rename of an exported name checks every
// package that imports its package, because it looks for the uses in their
// types: on a project of 90,000 lines that is 100 ms for the first request and
// 5 ms for the next. gopls has checked its packages by then. So, after the first
// diagnostics of a package, the server checks the packages that import it, in
// the background, one at a time (two for a large number), while it is idle:
//
//   - a request that comes finds them in the cache of checks, or waits for the one
//     that is under way (the check of a package runs once: memoize.Promise);
//   - it waits for a quiet moment before each package (warmQuiet), so a
//     keystroke or a request is not slowed down by it more than by the one
//     check that may be under way, and it stops when the server ends;
//   - it checks at most warmMax importers, and none once the cache of checks is
//     half full: what it checks is what the request would check, and the cache
//     is capped.
//
// Only the direct importers are warmed (the scope of an exported function, type,
// variable or constant), not the whole module (an exported method is looked
// for in all of it: that request stays as slow as before on the first call).
const (
	warmMax      = 96
	warmQuiet    = 10 * time.Millisecond
	warmQuietMax = 100 // waits of warmQuiet before abandoning this package
	warmWorkers  = 2
)

// A warmer is the state of the warming of a server.
type warmer struct {
	on   bool
	mu   sync.Mutex
	done map[string]bool // the packages whose importers were warmed
	runs int             // the warmings started (the tests count them)
}

// warmImporters starts the warming of the importers of the package in dir, once.
func (s *Server) warmImporters(dir string) {
	w := &s.warm
	if !w.on {
		return
	}
	w.mu.Lock()
	if w.done == nil {
		w.done = make(map[string]bool)
	}
	if w.done[dir] {
		w.mu.Unlock()
		return
	}
	w.done[dir] = true
	w.runs++
	w.mu.Unlock()
	go func() {
		if p := safe.Call(func() { s.warmRun(dir) }); p != nil {
			s.log.Printf("warming the importers of %s: panic: %v\n%s", dir, p.Value, p.Stack)
		}
	}()
}

func (s *Server) warmRun(dir string) {
	start := time.Now()
	var n int
	defer func() {
		if n > 0 {
			s.log.Printf("warmed the %d importers of %s in %v", n, dir, time.Since(start).Round(time.Millisecond))
		}
	}()
	ctx := s.comp.index.ctx
	snap, release := s.session.Snapshot()
	pkg, err := snap.Package(ctx, dir)
	release()
	if err != nil || pkg == nil || len(pkg.Program.Roots) == 0 {
		return
	}
	root := pkg.Program.Roots[0]
	if root.Module == nil || root.Module.Path == "" || root.Std {
		return
	}
	pkgs, err := s.comp.index.moduleWait(ctx, root.Module)
	if err != nil || len(pkgs) == 0 {
		return
	}
	snap, release = s.session.Snapshot()
	var mu sync.Mutex
	var importers []string
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, p := range pkgs {
		if p.dir == dir {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			safe.Call(func() {
				refs, err := snap.ImportsOf(ctx, p.dir)
				if err != nil {
					return
				}
				for _, ref := range refs {
					if ref.ImportPath == root.ImportPath {
						mu.Lock()
						importers = append(importers, p.dir)
						mu.Unlock()
						return
					}
				}
			})
		}()
	}
	wg.Wait()
	release()
	slices.Sort(importers)
	if len(importers) > warmMax {
		importers = importers[:warmMax]
	}
	next := make(chan string)
	var workers sync.WaitGroup
	nw := 1
	if len(importers) > 16 {
		nw = warmWorkers
	}
	for range nw {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for d := range next {
				if s.warmOne(ctx, d) {
					mu.Lock()
					n++
					mu.Unlock()
				}
			}
		}()
	}
loop:
	for _, d := range importers {
		select {
		case next <- d:
		case <-ctx.Done():
			break loop
		}
	}
	close(next)
	workers.Wait()
}

// warmOne checks the package in dir when the server is quiet and the cache has
// room. Its result keeps skipped or failed packages out of the warming count.
func (s *Server) warmOne(ctx context.Context, dir string) bool {
	quiet := false
	for range warmQuietMax {
		if ctx.Err() != nil {
			return false
		}
		if s.obs.gc.quiet() && !s.diags.deb.Busy() {
			quiet = true
			break
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(warmQuiet):
		}
	}
	if !quiet {
		return false // staying busy is not permission to start background work
	}
	if m := s.pipeline.Metrics().Caches["check"]; m.MaxBytes > 0 && m.Bytes > m.MaxBytes/2 {
		return false
	}

	snap, release := s.session.Snapshot()
	defer release()
	var warmed bool
	safe.Call(func() {
		pkg, err := snap.Package(ctx, filepath.Clean(dir))
		warmed = err == nil && pkg != nil
	})
	return warmed
}
