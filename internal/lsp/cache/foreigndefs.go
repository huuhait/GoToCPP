package cache

import (
	"context"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// What the header index worker asks after the declarations (step 12.13e): where a function or a
// variable is defined, when a C or C++ source of the package does it, and the body of a struct,
// union or enum that a typedef names. Both are answers of the same kind as the declarations: a
// completed background fact of the slot, read by requests without a compiler and without waiting,
// and nothing when the compiler could not prove it.

const (
	// maxDefinitionSources is the number of sources of a package that are read for definitions.
	// Each is one compiler run of the includes of the package and the source (about 0.2 s of CPU
	// for a header of raylib's size, and four run at the same time), so a package of a few dozen
	// sources is still done in a few seconds. The sources that stand for a header come first.
	maxDefinitionSources = 16
	maxDefinitionBytes   = 512 << 10 // a larger source is not read for definitions
	maxSourceScan        = 32 << 10  // the start of a source that is read for its #include lines
	maxSourceCandidates  = 512       // sources of a folder that are ranked; the others are left out
	definitionParallel   = 4
)

// A sourceFile is a C or C++ source of the package.
type sourceFile struct {
	path string
	c    bool // a .c file: it is read as C code (extern "C")
}

var includedNameRx = regexp.MustCompile(`(?:"([^"]+)"|<([^>]+)>)\s*$`)

// headerNames returns the base names of the headers of an include set, for the order of the
// sources: the stem ("geo" for "./geo.h") and the file name.
func headerNames(includes []string) (stems, files map[string]bool) {
	stems, files = make(map[string]bool), make(map[string]bool)
	for _, inc := range includes {
		m := includedNameRx.FindStringSubmatch(inc)
		if m == nil {
			continue
		}
		name := path.Base(filepath.ToSlash(m[1] + m[2]))
		files[strings.ToLower(name)] = true
		stems[strings.ToLower(strings.TrimSuffix(name, path.Ext(name)))] = true
	}
	return stems, files
}

// includesAny reports whether the start of the source has an #include of one of the files.
func includesAny(src string, files map[string]bool) bool {
	f, err := os.Open(src)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, maxSourceScan)
	n, _ := io.ReadFull(f, head)
	for _, line := range strings.Split(string(head[:n]), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		rest, ok := strings.CutPrefix(line, "include")
		if !ok {
			continue
		}
		if m := includedNameRx.FindStringSubmatch(strings.TrimSpace(rest)); m != nil {
			name := path.Base(filepath.ToSlash(m[1] + m[2]))
			if files[strings.ToLower(name)] {
				return true
			}
		}
	}
	return false
}

// packageSources returns the C and C++ sources in the folder of the package that are read for
// definitions, as absolute paths, and the number of those that are not. The editor's file system
// lists the .gp files only, so the folder is read here, in the worker. A source that is over the
// size limit is not read and does not take one of the places. The others are ranked: first those
// whose name has the stem of a header of the include set (geo.cpp for geo.h), then those that
// include one of its headers, then the rest, each in name order; the first
// maxDefinitionSources are read. Build constraints are not applied: a source that does not compile
// on this target gives no definitions, and the others do.
func packageSources(dir string, includes []string) (read []sourceFile, leftOut int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0
	}
	stems, files := headerNames(includes)
	type ranked struct {
		file sourceFile
		rank int
	}
	var candidates []ranked
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		ext := filepath.Ext(name)
		if ext != ".c" && ext != ".cc" && ext != ".cpp" && ext != ".cxx" {
			continue
		}
		full := filepath.Join(dir, name)
		if info, err := os.Stat(full); err != nil || !info.Mode().IsRegular() ||
			info.Size() > maxDefinitionBytes {
			leftOut++
			continue
		}
		if len(candidates) == maxSourceCandidates {
			leftOut++
			continue
		}
		rank := 2
		if stems[strings.ToLower(strings.TrimSuffix(name, ext))] {
			rank = 0
		} else if includesAny(full, files) {
			rank = 1
		}
		candidates = append(candidates, ranked{sourceFile{path: full, c: ext == ".c"}, rank})
	}
	slices.SortStableFunc(candidates, func(a, b ranked) int { return a.rank - b.rank })
	for i, c := range candidates {
		if i == maxDefinitionSources {
			leftOut += len(candidates) - i
			break
		}
		read = append(read, c.file)
	}
	return read, leftOut
}

// locateMore runs the two steps after the declarations; they do not depend on each other.
func (ix *headerIndexer) locateMore(ctx context.Context, entry *headerEntry, key string,
	cfg probe.Config, includes []string,
) {
	if ctx.Err() != nil {
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ix.locateDefinitions(ctx, entry, key, cfg, includes)
	}()
	go func() {
		defer wg.Done()
		ix.locateBodies(ctx, entry, key, cfg, includes)
	}()
	wg.Wait()
}

// current reports whether the entry is still the slot's, and its declarations when it is.
func (ix *headerIndexer) current(ctx context.Context, entry *headerEntry, key string,
) (decls map[string][]probe.Declaration, queries map[string]probe.LocateName, ok bool) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ctx.Err() != nil || ix.entries[key] != entry {
		return nil, nil, false
	}
	return entry.decls, entry.queries, true
}

// locateBodies asks for the body of the tag of every typedef among the declarations, and
// replaces their signatures while the slot is still the current one.
func (ix *headerIndexer) locateBodies(ctx context.Context, entry *headerEntry, key string,
	cfg probe.Config, includes []string,
) {
	decls, _, ok := ix.current(ctx, entry, key)
	if !ok || ix.bodies == nil {
		return
	}
	type place struct {
		key   string
		index int
	}
	var places []place
	var tags []probe.Declaration
	for _, name := range slices.Sorted(maps.Keys(decls)) {
		for i, d := range decls[name] {
			if d.Tag != nil && d.Kind == "type" {
				places = append(places, place{name, i})
				tags = append(tags, d)
			}
		}
	}
	if len(tags) == 0 {
		return
	}
	filled, err := ix.bodies(ctx, cfg, includes, tags)
	if err != nil || len(filled) != len(tags) || ctx.Err() != nil {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ctx.Err() != nil || ix.entries[key] != entry || len(entry.decls) != len(decls) {
		return
	}
	var cost int64
	next := make(map[string][]probe.Declaration, len(entry.decls))
	for name, list := range entry.decls {
		next[name] = slices.Clone(list)
	}
	for n, at := range places {
		if at.index >= len(next[at.key]) || next[at.key][at.index].File != tags[n].File {
			return // the slot's facts are not the ones the bodies were asked for
		}
		cost += int64(len(filled[n].Signature) - len(next[at.key][at.index].Signature))
		next[at.key][at.index] = filled[n]
	}
	if cost > 0 && ix.bytes+cost > maxHeaderIndexBytes {
		return
	}
	entry.decls = next
	entry.bytes += cost
	ix.bytes += cost
}

// locateDefinitions looks the functions and variables of the slot up again after the includes of
// each C or C++ source of the package. The latest declaration that the compiler finds is then
// the source's definition when it has one, and the answer is kept when it has a body and is
// another declaration of the entity that the header declares (Declaration.Redeclares: a static
// function, or one without a previous declaration, is not). When the latest declaration is a
// redeclaration after the definition, the declarations of the name are asked for and the one with
// the body is taken. The body may be in a file that the source includes (a unity build, a
// generated .inc): it is the definition all the same, and every source that includes it finds the
// same one. A source whose run failed is tried once more after a short wait, as the header index
// does: it may have been saved inside the runner's racy window. The sources that were not read
// are counted for the hint of the editor. What a source gave is kept from one snapshot to the
// next while the files its run read are the same (foreignmemo.go).
func (ix *headerIndexer) locateDefinitions(ctx context.Context, entry *headerEntry, key string,
	cfg probe.Config, includes []string,
) {
	decls, queries, ok := ix.current(ctx, entry, key)
	if !ok {
		return
	}
	sources, leftOut := packageSources(cfg.Dir, includes)
	wanted := make(map[string]probe.LocateName)
	for name, list := range decls {
		query, found := queries[name]
		if !found || query.Macro || !slices.ContainsFunc(list, definable) {
			continue
		}
		wanted[name] = query
	}
	if len(sources) == 0 && leftOut == 0 || len(wanted) == 0 {
		return
	}
	results := make([]map[string][]probe.Declaration, len(sources))
	asked := &definitionAsk{ix: ix, cfg: cfg, includes: includes, wanted: wanted, decls: decls}
	failed := ix.locateSources(ctx, asked, sources, results, nil)
	if len(failed) > 0 && ctx.Err() == nil {
		timer := time.NewTimer(ix.retryDelay())
		select {
		case <-timer.C:
			ix.locateSources(ctx, asked, sources, results, failed)
		case <-ctx.Done():
		}
		timer.Stop()
	}
	if ctx.Err() != nil {
		return
	}
	defs := make(map[string][]probe.Declaration)
	var cost int64
	for _, found := range results {
		for name, list := range found {
			for _, d := range list {
				if !slices.Contains(defs[name], d) {
					defs[name] = append(defs[name], d)
					cost += int64(len(d.File)+len(d.Signature)+len(d.Type)+len(d.Params)) + 96
				}
			}
		}
	}
	if len(defs) == 0 && leftOut == 0 {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ctx.Err() != nil || ix.entries[key] != entry || ix.bytes+cost > maxHeaderIndexBytes {
		return
	}
	entry.defs = defs
	entry.sourcesLeftOut = leftOut
	entry.bytes += cost
	ix.bytes += cost
}

// locateSources asks each source (those at the indexes in only, or all when it is nil), a few at
// a time, and stores what it gave in results. It returns the indexes of the sources whose run
// failed.
func (ix *headerIndexer) locateSources(ctx context.Context, ask *definitionAsk,
	sources []sourceFile, results []map[string][]probe.Declaration, only []int,
) []int {
	if only == nil {
		for i := range sources {
			only = append(only, i)
		}
	}
	var (
		mu     sync.Mutex
		failed []int
		wg     sync.WaitGroup
		slots  = make(chan struct{}, definitionParallel)
	)
	for _, i := range only {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			found, ok := ask.source(ctx, sources[i])
			mu.Lock()
			defer mu.Unlock()
			if !ok {
				failed = append(failed, i)
				return
			}
			results[i] = found
		}()
	}
	wg.Wait()
	slices.Sort(failed)
	return failed
}

// retryDelay is how long a source whose run failed waits for its second try.
func (ix *headerIndexer) retryDelay() time.Duration {
	if ix.retry > 0 {
		return ix.retry
	}
	return revalidateAfter
}

// definable says that a declaration can have a definition elsewhere: a function or a variable
// that its header does not define itself.
func definable(d probe.Declaration) bool {
	return (d.Kind == "function" || d.Kind == "variable") && d.Type != "" && !d.Defined
}

// ForeignDefinitions returns the declarations that a C or C++ source of the package gives for a
// name that the file writes after a header package or a namespace: the definitions of a function
// or a variable that the header only declares. Like ForeignDeclarations it reads a completed
// background fact of this snapshot and nothing else.
func (s *Snapshot) ForeignDefinitions(pkg *Package, file *load.File, scope, name string,
) ([]probe.Declaration, bool) {
	st, ok := s.state.(*state)
	if !ok || st.p.headers == nil || pkg == nil || pkg.Check == nil || pkg.IsVariant() ||
		file == nil {
		return nil, false
	}
	ix := st.p.headers
	key := headerEntryKey(s, pkg, headerIncludes(pkg, file))
	ix.mu.Lock()
	defer ix.mu.Unlock()
	entry := ix.entries[key]
	if entry == nil || entry.ctx.Err() != nil {
		return nil, false
	}
	found := entry.defs[foreignKey(scope, name)]
	if len(found) == 0 {
		return nil, false
	}
	return slices.Clone(found), true
}

// ForeignSourcesLeftOut returns the number of C and C++ sources of the package that were not read
// for definitions (over the size limit, or beyond the first maxDefinitionSources in the order of
// packageSources), and the limit. Like ForeignDefinitions it reads a completed fact only.
func (s *Snapshot) ForeignSourcesLeftOut(pkg *Package, file *load.File) (left, limit int) {
	st, ok := s.state.(*state)
	if !ok || st.p.headers == nil || pkg == nil || pkg.Check == nil || pkg.IsVariant() ||
		file == nil {
		return 0, maxDefinitionSources
	}
	ix := st.p.headers
	key := headerEntryKey(s, pkg, headerIncludes(pkg, file))
	ix.mu.Lock()
	defer ix.mu.Unlock()
	entry := ix.entries[key]
	if entry == nil || entry.ctx.Err() != nil {
		return 0, maxDefinitionSources
	}
	return entry.sourcesLeftOut, maxDefinitionSources
}
