package cache

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// What a C or C++ source of the package gave for the names of a slot is kept from one snapshot to
// the next (step 12.13e). The slot of a snapshot is new at every edit of a .gp file, and a lookup
// after each source is a compiler run unless the runner has its answer; the runner finds it by
// reading every file the run read (a header of the standard library has hundreds) to compare
// them. Here the files are compared by their status, which the compiler is not asked for, and
// only the names that the source has not answered yet are asked: an edit that changes no file the
// source reads, and asks no new name, costs no run and reads no file.
//
// An answer is kept only with the stamps of the files its run read (probe.Stamps: a file counts
// as unchanged by its status when it was old enough at the time that a later write cannot have
// left its times alone), and it is dropped when one differs. The compiler, the flags, the
// includes of the package and the source are in the key.

const (
	maxSourceMemos     = 128
	maxSourceMemoBytes = 16 << 20
)

// A sourceMemo is what one source gave for the names it was asked.
type sourceMemo struct {
	stamps *probe.Stamps
	asked  map[string]bool                // the names that were asked, by foreignKey
	defs   map[string][]probe.Declaration // the definitions found for them
	used   uint64
	bytes  int64
}

func sourceMemoKey(cfg probe.Config, includes []string, source sourceFile) string {
	parts := []string{cfg.Dir, cfg.CXX.Path, cfg.CXX.Version, cfg.CXX.GOOS, cfg.CXX.GOARCH,
		strconv.Itoa(len(cfg.CXX.Args))}
	parts = append(parts, cfg.CXX.Args...)
	parts = append(parts, strconv.Itoa(len(cfg.Flags)))
	parts = append(parts, cfg.Flags...)
	parts = append(parts, strconv.Itoa(len(includes)))
	parts = append(parts, includes...)
	parts = append(parts, source.path, strconv.FormatBool(source.c))
	return strings.Join(parts, "\x00")
}

// memo returns the memo of the key when the files it was made from are the same.
func (ix *headerIndexer) memo(key string) *sourceMemo {
	ix.mu.Lock()
	m := ix.memos[key]
	ix.memoClock++
	if m != nil {
		m.used = ix.memoClock
	}
	ix.mu.Unlock()
	if m == nil || !m.stamps.Unchanged() {
		return nil
	}
	return m
}

// keepMemo stores the memo and drops the ones used least recently when there are too many.
func (ix *headerIndexer) keepMemo(key string, m *sourceMemo) {
	for _, list := range m.defs {
		for _, d := range list {
			m.bytes += int64(len(d.File)+len(d.Signature)+len(d.Type)+len(d.Params)) + 96
		}
	}
	m.bytes += m.stamps.Bytes() + int64(len(m.asked))*64
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.closed {
		return
	}
	if ix.memos == nil {
		ix.memos = make(map[string]*sourceMemo)
	}
	if old := ix.memos[key]; old != nil {
		ix.memoBytes -= old.bytes
	}
	ix.memoClock++
	m.used = ix.memoClock
	ix.memos[key] = m
	ix.memoBytes += m.bytes
	for len(ix.memos) > maxSourceMemos || ix.memoBytes > maxSourceMemoBytes && len(ix.memos) > 1 {
		var oldest string
		for k, other := range ix.memos {
			if k != key && (oldest == "" || other.used < ix.memos[oldest].used) {
				oldest = k
			}
		}
		ix.memoBytes -= ix.memos[oldest].bytes
		delete(ix.memos, oldest)
	}
}

func (ix *headerIndexer) dropMemo(key string) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if old := ix.memos[key]; old != nil {
		ix.memoBytes -= old.bytes
		delete(ix.memos, key)
	}
}

// A definitionAsk is the question of locateDefinitions for the sources of a slot.
type definitionAsk struct {
	ix       *headerIndexer
	cfg      probe.Config
	includes []string
	wanted   map[string]probe.LocateName    // the names, by foreignKey
	decls    map[string][]probe.Declaration // the declarations of the headers, by foreignKey
}

// source returns the definitions that the source gives for the wanted names, by foreignKey, and
// false when its run failed.
func (a *definitionAsk) source(ctx context.Context, source sourceFile,
) (map[string][]probe.Declaration, bool) {
	ix := a.ix
	key := sourceMemoKey(a.cfg, a.includes, source)
	memo := ix.memo(key)
	var missing []string
	for name := range a.wanted {
		if memo == nil || !memo.asked[name] {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	if len(missing) == 0 {
		return restrictDefinitions(memo.defs, a.wanted), true
	}
	include := `"` + source.path + `"`
	if source.c {
		include = interop.ExternC(include)
	}
	withSource := append(slices.Clone(a.includes), include)
	var request probe.LocateRequest
	for _, name := range missing {
		request.Names = append(request.Names, a.wanted[name])
	}
	locations, err := ix.locate(ctx, a.cfg, withSource, request)
	if err != nil || locations == nil {
		return nil, false
	}
	found := make(map[string][]probe.Declaration)
	for _, name := range missing {
		query := a.wanted[name]
		latest := locations.Named(query.Scope, query.Name)
		defs, ok := a.definitions(ctx, withSource, query, a.decls[name], latest)
		if !ok {
			return nil, false
		}
		if len(defs) > 0 {
			found[name] = defs
		}
	}
	if stamps := locations.CaptureStamps(); stamps == nil {
		ix.dropMemo(key)
	} else {
		next := &sourceMemo{
			stamps: stamps,
			asked:  make(map[string]bool),
			defs:   make(map[string][]probe.Declaration),
		}
		if memo != nil && memo.stamps.Unchanged() {
			for name := range memo.asked {
				next.asked[name] = true
			}
			for name, list := range memo.defs {
				next.defs[name] = list
			}
		}
		for _, name := range missing {
			next.asked[name] = true
		}
		for name, list := range found {
			next.defs[name] = list
		}
		ix.keepMemo(key, next)
		return restrictDefinitions(next.defs, a.wanted), true
	}
	if memo != nil {
		merged := make(map[string][]probe.Declaration, len(memo.defs)+len(found))
		for name, list := range memo.defs {
			merged[name] = list
		}
		for name, list := range found {
			merged[name] = list
		}
		return restrictDefinitions(merged, a.wanted), true
	}
	return found, true
}

// definitions returns the definitions among the declarations that the lookup gave for a name
// after a source: those with a body that redeclare a declaration of the headers. When the latest
// declaration is a redeclaration after the definition, the chain of the name is asked for. It
// returns false when that failed.
func (a *definitionAsk) definitions(ctx context.Context, includes []string, query probe.LocateName,
	headers, found []probe.Declaration,
) ([]probe.Declaration, bool) {
	var defs []probe.Declaration
	redeclared := false
	for _, d := range found {
		switch {
		case d.Defined && d.Redeclares:
			defs = append(defs, d)
		case d.Redeclares && !slices.ContainsFunc(headers, func(h probe.Declaration) bool {
			return h.File == d.File && h.Line == d.Line && h.Col == d.Col
		}):
			redeclared = true
		}
	}
	if len(defs) > 0 || !redeclared {
		return defs, true
	}
	chain, err := a.ix.chain(ctx, a.cfg, includes, query.Scope, query.Name)
	if err != nil {
		return nil, false
	}
	for _, d := range chain {
		if d.Defined && d.Redeclares && !slices.Contains(defs, d) {
			defs = append(defs, d)
		}
	}
	return defs, true
}

// restrictDefinitions returns the definitions of the wanted names.
func restrictDefinitions(defs map[string][]probe.Declaration, wanted map[string]probe.LocateName,
) map[string][]probe.Declaration {
	out := make(map[string][]probe.Declaration)
	for name := range wanted {
		if list := defs[name]; len(list) > 0 {
			out[name] = list
		}
	}
	return out
}
