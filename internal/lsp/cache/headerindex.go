package cache

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// Header indexing is on with the default interop mode (auto) and with typed, off with opaque. It never installs
// typed checking hooks: ordinary checks, diagnostics and requests don't wait for clang.
// Slots belong to one snapshot and one exact file include/flag set. The runner can reuse a
// completed product only after validating its compiler identity and dependency contents.
type headerIndexer struct {
	mu      sync.Mutex
	closed  bool
	entries map[string]*headerEntry
	clock   uint64
	bytes   int64
	wg      sync.WaitGroup
	sem     chan struct{}
	find    func(context.Context) (build.CXX, error)
	run     func(context.Context, probe.Config, []string) (*probe.HeaderIndex, error)

	// locate finds the declarations of the names that a program uses once its index is ready
	// (step 12.13c2). It has its own cache entries in the runner.
	locate func(context.Context, probe.Config, []string, probe.LocateRequest) (*probe.Locations, error)

	// bodies adds the bodies of the structs, unions and enums that typedefs name to the typedef
	// declarations (step 12.13e).
	bodies func(context.Context, probe.Config, []string, []probe.Declaration,
	) ([]probe.Declaration, error)

	// chain finds every declaration of a name after the includes (step 12.13e).
	chain func(ctx context.Context, cfg probe.Config, includes []string, scope, name string,
	) ([]probe.Declaration, error)

	// memos are what the sources of a package gave, kept from one snapshot to the next
	// (foreignmemo.go); memoBytes is their estimated size.
	memos     map[string]*sourceMemo
	memoClock uint64
	memoBytes int64

	// retry is how long a source whose lookup failed waits for its second try (0: revalidateAfter).
	retry time.Duration

	// beforePublish is a hook of the tests, called when the facts are complete and before they
	// are published.
	beforePublish func()
}

type headerEntry struct {
	ctx    context.Context
	cancel context.CancelFunc
	index  *probe.HeaderIndex
	used   uint64
	bytes  int64
	work   headerWork

	// decls are the completed declarations of the names that the program wrote after a header
	// package or a namespace, by foreignKey. A name that is missing has no proven declaration.
	decls map[string][]probe.Declaration

	// queries are the lookups that gave decls (by foreignKey); defs are the declarations that the
	// C and C++ sources of the package give for the functions and variables among them: where they
	// are defined (step 12.13e).
	queries map[string]probe.LocateName
	defs    map[string][]probe.Declaration

	// sourcesLeftOut is the number of C and C++ sources of the package folder that were not read
	// for defs (foreigndefs.go).
	sourcesLeftOut int
}

type headerWork struct {
	parent   context.Context
	key      string
	dir      string
	flags    []string
	includes []string
	names    []foreignName
}

const maxHeaderIndexes = 128
const maxHeaderIndexBytes = 32 << 20

func newHeaderIndexer(cfg Config) *headerIndexer {
	setting, err := typedinterop.ResolveMode("", os.Getenv)
	if err != nil || setting.Mode == typedinterop.Opaque {
		return nil
	}
	runner := new(probe.Runner)
	return &headerIndexer{
		entries: make(map[string]*headerEntry),
		sem:     make(chan struct{}, maxProbes),
		find: func(ctx context.Context) (build.CXX, error) {
			return probe.FindClangFor(ctx, cfg.GOOS, cfg.GOARCH)
		},
		run:    runner.Index,
		locate: runner.Locate,
		bodies: runner.LocateBodies,
		chain:  runner.LocateChain,
	}
}

// StopHeaderIndex cancels and joins C/C++ completion workers at protocol shutdown, before exit.
func (p *Pipeline) StopHeaderIndex() {
	p.adapt.close()
	p.members.close()
	p.headers.close()
}

func (ix *headerIndexer) close() {
	if ix == nil {
		return
	}
	ix.mu.Lock()
	ix.closed = true
	for _, entry := range ix.entries {
		entry.cancel()
	}
	clear(ix.entries)
	ix.bytes = 0
	ix.mu.Unlock()
	ix.wg.Wait()
}

func (ix *headerIndexer) invalidate() []headerWork {
	if ix == nil {
		return nil
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	var work []headerWork
	for _, entry := range ix.entries {
		work = append(work, entry.work)
		entry.cancel()
	}
	clear(ix.entries)
	ix.bytes = 0
	return work
}

// Header watcher events need not change namespace-probe answers or create a new snapshot.
// Forget ready names immediately and validate the current snapshot's inputs in the background.
func (ix *headerIndexer) refresh() {
	for _, work := range ix.invalidate() {
		ix.schedule(work.parent, work.key, work.dir, work.flags, work.includes, work.names...)
	}
}

func headerIncludes(pkg *Package, file *load.File) []string {
	var includes []string
	if file.Info == nil {
		return nil
	}
	for _, inc := range file.Info.Includes {
		text := inc.IncludeText()
		if pkg.Check.ExternC[text] {
			text = interop.ExternC(text)
		}
		if !slices.Contains(includes, text) {
			includes = append(includes, text)
		}
	}
	return includes
}

func headerEntryKey(snap *Snapshot, pkg *Package, includes []string) string {
	parts := []string{fmt.Sprintf("%p", snap.session), strconv.FormatUint(snap.id, 10), pkg.Check.Load.Dir,
		strconv.Itoa(len(pkg.Check.ProbeFlags))}
	parts = append(parts, pkg.Check.ProbeFlags...)
	parts = append(parts, includes...)
	return strings.Join(parts, "\x00")
}

// scheduleHeaders is called after an ordinary package check, including the background check
// triggered by a document update. No disk access or compiler discovery happens here.
func (s *Snapshot) scheduleHeaders(pkg *Package) {
	st, ok := s.state.(*state)
	if !ok || st.p.headers == nil || pkg == nil || pkg.Check == nil || pkg.IsVariant() {
		return
	}
	if !probe.ProbesTarget(st.p.cfg.GOOS, st.p.cfg.GOARCH) && !typedinterop.StdHeadersOnly(pkg.Check) {
		return
	}
	// clang reads disk, not editor overlays. Conservatively suppress disk facts for any
	// non-.gp overlay; supporting unsaved C/C++ headers needs a VFS overlay in a later subjob.
	for _, overlay := range s.overlays {
		if !strings.HasSuffix(overlay.URI().Path(), ".gp") {
			return
		}
	}
	for _, g := range pkg.headerGroups() {
		key := headerEntryKey(s, pkg, g.includes)
		st.p.headers.schedule(s.ctx, key, pkg.Check.Load.Dir, pkg.Check.ProbeFlags, g.includes,
			g.names...)
	}
}

// A headerGroup is the files of a package with the same includes: they share one slot and one run,
// and so the names.
type headerGroup struct {
	includes []string
	names    []foreignName
}

type headerGroupsKey struct{}

// headerGroups returns the groups of the package, in the order of its files. They depend on the
// check alone, so every request and snapshot that has the package shares them: a request must
// not walk the syntax trees again for work that is already scheduled.
func (pkg *Package) headerGroups() []*headerGroup {
	return pkg.Memo(headerGroupsKey{}, func() any {
		groups := make(map[string]*headerGroup)
		var order []*headerGroup
		for _, file := range pkg.Check.Load.Files {
			includes := headerIncludes(pkg, file)
			if len(includes) == 0 {
				continue
			}
			key := fmt.Sprintf("%q", includes)
			g := groups[key]
			if g == nil {
				g = &headerGroup{includes: includes}
				groups[key] = g
				order = append(order, g)
			}
			for _, name := range foreignNames(file) {
				if !slices.Contains(g.names, name) && len(g.names) < maxForeignNames {
					g.names = append(g.names, name)
				}
			}
		}
		return order
	}).([]*headerGroup)
}

func (ix *headerIndexer) schedule(ctx context.Context, key, dir string, flags, includes []string,
	names ...foreignName,
) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.closed || ctx.Err() != nil {
		return
	}
	ix.clock++
	if entry := ix.entries[key]; entry != nil {
		entry.used = ix.clock
		return
	}
	for key, entry := range ix.entries {
		if entry.ctx.Err() != nil {
			entry.cancel()
			ix.bytes -= entry.bytes
			delete(ix.entries, key)
		}
	}
	if len(ix.entries) >= maxHeaderIndexes {
		var oldest string
		for key, entry := range ix.entries {
			if oldest == "" || entry.used < ix.entries[oldest].used {
				oldest = key
			}
		}
		ix.entries[oldest].cancel()
		ix.bytes -= ix.entries[oldest].bytes
		delete(ix.entries, oldest)
	}
	work := headerWork{parent: ctx, key: key, dir: dir,
		flags: slices.Clone(flags), includes: slices.Clone(includes), names: slices.Clone(names)}
	ctx, cancel := context.WithCancel(ctx)
	entry := &headerEntry{ctx: ctx, cancel: cancel, used: ix.clock, work: work}
	ix.entries[key] = entry
	flags = work.flags
	includes = work.includes
	names = work.names
	ix.wg.Add(1)
	go func() {
		defer ix.wg.Done()
		// The probe slot is for the index only: the declaration facts below may wait for a retry,
		// and must not hold up the indexes of other snapshots.
		held := false
		release := func() {
			if held {
				held = false
				<-ix.sem
			}
		}
		select {
		case ix.sem <- struct{}{}:
			held = true
			defer release()
		case <-ctx.Done():
			return
		}
		// One retry covers headers just saved inside the runner's conservative race window.
		// Both attempts and the wait belong to this snapshot and are canceled on replacement.
		for attempt := 0; attempt < 2; attempt++ {
			cxx, err := ix.find(ctx)
			var index *probe.HeaderIndex
			if err == nil {
				index, err = ix.run(ctx, probe.Config{CXX: cxx, Dir: dir, Flags: flags}, includes)
			}
			if err == nil && index != nil {
				ix.mu.Lock()
				cost := index.Bytes()
				if ctx.Err() == nil && ix.entries[key] == entry && cost <= maxHeaderIndexBytes {
					for ix.bytes+cost > maxHeaderIndexBytes {
						var oldest string
						for otherKey, other := range ix.entries {
							if other == entry || other.index == nil {
								continue
							}
							if oldest == "" || other.used < ix.entries[oldest].used {
								oldest = otherKey
							}
						}
						other := ix.entries[oldest]
						other.cancel()
						ix.bytes -= other.bytes
						delete(ix.entries, oldest)
					}
					entry.index = index
					entry.bytes = cost
					ix.bytes += cost
				}
				ix.mu.Unlock()
				release()
				cfg := probe.Config{CXX: cxx, Dir: dir, Flags: flags}
				ix.locateNames(ctx, entry, key, cfg, includes, index, names)
				ix.locateMore(ctx, entry, key, cfg, includes)
				return
			}
			if attempt == 0 {
				timer := time.NewTimer(revalidateAfter)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return
				}
			}
		}
	}()
}

// HeaderSymbols reads only completed immutable names. In particular it does not call the
// runner, revalidate files, or schedule work on a request's behalf.
func (s *Snapshot) HeaderSymbols(pkg *Package, file *load.File, scope string) []probe.HeaderSymbol {
	st, ok := s.state.(*state)
	if !ok || st.p.headers == nil || pkg == nil || pkg.Check == nil {
		return nil
	}
	ix := st.p.headers
	key := headerEntryKey(s, pkg, headerIncludes(pkg, file))
	ix.mu.Lock()
	defer ix.mu.Unlock()
	entry := ix.entries[key]
	if entry == nil || entry.ctx.Err() != nil {
		return nil
	}
	return entry.index.Names(scope)
}
