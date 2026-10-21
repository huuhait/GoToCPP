package cache

import (
	"context"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

const maxMemberIndexes = 16
const maxMemberIndexBytes = 32 << 20
const maxMemberQueries = 16
const maxMemberBindings = 256

type memberAnswer struct {
	names      []string
	nativeType string
	incomplete bool
}

type memberFacts struct {
	bindings map[*types.Var]memberAnswer
	bytes    int64
}

type memberEntry struct {
	key        string
	parent     context.Context
	ctx        context.Context
	cancel     context.CancelFunc
	pkg        *Package
	generation uint64
	running    bool
	queued     bool
	used       uint64
	bytes      int64
	facts      *memberFacts
}

// A fixed worker pool bounds both compiler work and queued Package/AST ownership. The byte
// budget includes retained input packages, including canceled work until its worker joins.
type memberIndexer struct {
	mu      sync.Mutex
	closed  bool
	entries map[string]*memberEntry
	jobs    chan *memberEntry
	stop    context.CancelFunc
	wg      sync.WaitGroup
	clock   uint64
	bytes   int64
	run     func(context.Context, *Package) (*memberFacts, error)
}

func newMemberIndexer(cfg Config) *memberIndexer {
	setting, err := typedinterop.ResolveMode("", os.Getenv)
	if err != nil || setting.Mode == typedinterop.Opaque {
		return nil
	}
	ctx, stop := context.WithCancel(context.Background())
	ix := &memberIndexer{
		entries: make(map[string]*memberEntry), jobs: make(chan *memberEntry, maxMemberIndexes),
		stop: stop, run: func(ctx context.Context, pkg *Package) (*memberFacts, error) {
			return analyzeMembers(ctx, cfg, pkg)
		},
	}
	for range maxProbes {
		ix.wg.Add(1)
		go ix.worker(ctx)
	}
	return ix
}

func (ix *memberIndexer) remove(entry *memberEntry) {
	delete(ix.entries, entry.key)
	entry.cancel()
	if !entry.running {
		ix.bytes -= entry.bytes
		entry.pkg = nil
		entry.facts = nil
	}
}

func (ix *memberIndexer) close() {
	if ix == nil {
		return
	}
	ix.mu.Lock()
	ix.closed = true
	ix.stop()
	for _, entry := range ix.entries {
		ix.remove(entry)
	}
	ix.mu.Unlock()
	ix.wg.Wait()
}

func (ix *memberIndexer) refresh() {
	if ix == nil {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.closed {
		return
	}
	for _, entry := range ix.entries {
		if entry.parent.Err() != nil {
			ix.remove(entry)
			continue
		}
		entry.cancel()
		entry.ctx, entry.cancel = context.WithCancel(entry.parent)
		entry.generation++
		if entry.facts != nil {
			ix.bytes -= entry.facts.bytes
			entry.bytes -= entry.facts.bytes
			entry.facts = nil
		}
		// Queued work reads the new generation when dequeued; running work requeues on return.
		if !entry.running && !entry.queued {
			select {
			case ix.jobs <- entry:
				entry.queued = true
			default:
				ix.remove(entry)
			}
		}
	}
}

func memberEntryKey(snap *Snapshot, pkg *Package) string {
	return headerEntryKey(snap, pkg, typedinterop.UnitIncludes(pkg.Check)) + fmt.Sprintf("\x00%p", pkg)
}

func (s *Snapshot) scheduleMembers(pkg *Package) {
	st, ok := s.state.(*state)
	if !ok || st.p.members == nil || pkg == nil || pkg.Check == nil || pkg.IsVariant() ||
		pkg.Check.Opaque == nil || pkg.Check.Info == nil || pkg.cost > maxMemberIndexBytes/2 {
		return
	}
	if !probe.ProbesTarget(st.p.cfg.GOOS, st.p.cfg.GOARCH) && !typedinterop.StdHeadersOnly(pkg.Check) {
		return
	}
	for _, overlay := range s.overlays {
		if !strings.HasSuffix(overlay.URI().Path(), ".gp") {
			return
		}
	}
	for id, object := range pkg.Check.Info.Uses {
		if variable, ok := object.(*types.Var); ok && !variable.IsField() && pkg.Check.Opaque.IsOpaque(id) {
			st.p.members.schedule(s.ctx, memberEntryKey(s, pkg), pkg)
			return
		}
	}
}

func (ix *memberIndexer) schedule(parent context.Context, key string, pkg *Package) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.closed || parent.Err() != nil || pkg.cost > maxMemberIndexBytes/2 {
		return
	}
	ix.clock++
	if entry := ix.entries[key]; entry != nil {
		entry.used = ix.clock
		return
	}
	for _, entry := range ix.entries {
		if entry.ctx.Err() != nil {
			ix.remove(entry)
		}
	}
	cost := max(pkg.cost, 256)
	for len(ix.entries) >= maxMemberIndexes || ix.bytes+cost > maxMemberIndexBytes {
		var oldest *memberEntry
		for _, entry := range ix.entries {
			if oldest == nil || entry.used < oldest.used {
				oldest = entry
			}
		}
		if oldest == nil {
			return // canceled active workers still own their inputs until they join
		}
		ix.remove(oldest)
	}
	ctx, cancel := context.WithCancel(parent)
	entry := &memberEntry{key: key, parent: parent, ctx: ctx, cancel: cancel,
		pkg: pkg, used: ix.clock, bytes: cost}
	ix.entries[key] = entry
	ix.bytes += cost
	select {
	case ix.jobs <- entry:
		entry.queued = true
	default:
		ix.remove(entry)
	}
}

func (ix *memberIndexer) worker(ctx context.Context) {
	defer ix.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-ix.jobs:
			ix.mu.Lock()
			entry.queued = false
			if ix.entries[entry.key] != entry || entry.running || entry.ctx.Err() != nil {
				if ix.entries[entry.key] == entry && entry.ctx.Err() != nil {
					ix.remove(entry)
				}
				ix.mu.Unlock()
				continue
			}
			entry.running = true
			generation := entry.generation
			pkg := entry.pkg
			workCtx, cancel := context.WithTimeout(entry.ctx, 30*time.Second)
			ix.mu.Unlock()

			facts, err := ix.runWork(workCtx, pkg)
			cancel()
			ix.mu.Lock()
			entry.running = false
			if ix.entries[entry.key] != entry {
				ix.bytes -= entry.bytes
				entry.pkg = nil
				entry.facts = nil
			} else if entry.generation != generation {
				select {
				case ix.jobs <- entry:
					entry.queued = true
				default:
					ix.remove(entry)
				}
			} else if err == nil && facts != nil && entry.ctx.Err() == nil &&
				ix.bytes+facts.bytes <= maxMemberIndexBytes {
				entry.facts = facts
				entry.bytes += facts.bytes
				ix.bytes += facts.bytes
			}
			ix.mu.Unlock()
		}
	}
}

// One bounded retry covers newly saved headers inside the probe's conservative race window.
func (ix *memberIndexer) runWork(ctx context.Context, pkg *Package) (*memberFacts, error) {
	facts, err := ix.run(ctx, pkg)
	if err == nil || ctx.Err() != nil {
		return facts, err
	}
	timer := time.NewTimer(revalidateAfter)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ix.run(ctx, pkg)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// MemberSymbols is a read-only lookup for an exact original package/object in this snapshot.
// No compiler, dependency validation or scheduling is performed by a lookup. Variants and
// unsupported bindings have no ready facts and must not be presented as complete-empty.
func (s *Snapshot) MemberSymbols(pkg *Package, object *types.Var) ([]string, bool, bool) {
	answer, ready := s.memberFact(pkg, object)
	if !ready {
		return nil, true, false
	}
	return slices.Clone(answer.names), answer.incomplete, true
}

// ForeignVariableType returns the retained native type of the bound variable and whether its
// member list is partial. Like MemberSymbols, it performs no compiler, dependency IO or
// scheduling work. The type is the compiler's canonical spelling (typedef names resolved), not
// the spelling of the declaration. When the fact is not ready the result is the zero triple
// ("", false, false); only the final ready value says whether the first two are meaningful.
func (s *Snapshot) ForeignVariableType(pkg *Package, object *types.Var) (string, bool, bool) {
	answer, ready := s.memberFact(pkg, object)
	if !ready || answer.nativeType == "" {
		return "", false, false
	}
	return answer.nativeType, answer.incomplete, true
}

func (s *Snapshot) memberFact(pkg *Package, object *types.Var) (memberAnswer, bool) {
	st, ok := s.state.(*state)
	if !ok || st.p.members == nil || pkg == nil || pkg.Check == nil || pkg.IsVariant() || object == nil {
		return memberAnswer{}, false
	}
	ix := st.p.members
	ix.mu.Lock()
	defer ix.mu.Unlock()
	entry := ix.entries[memberEntryKey(s, pkg)]
	if entry == nil || entry.ctx.Err() != nil || entry.facts == nil {
		return memberAnswer{}, false
	}
	answer, ready := entry.facts.bindings[object]
	return answer, ready
}

func analyzeMembers(ctx context.Context, cfg Config, pkg *Package) (*memberFacts, error) {
	cxx, err := probe.FindClangFor(ctx, cfg.GOOS, cfg.GOARCH)
	if err != nil {
		return nil, err
	}
	probeConfig := probe.Config{CXX: cxx, Dir: pkg.Check.Load.Dir, Flags: pkg.Check.ProbeFlags}
	inputs, err := probe.CaptureInputs(ctx, probeConfig, typedinterop.UnitIncludes(pkg.Check))
	if err != nil {
		return nil, err
	}
	// Analyze without Decls only collects into fresh IR and runs probes. Its collector reads
	// Opaque, Info, conversions and ASTs; it does not install hooks or mutate the checked input.
	// Each work item owns its Runner, including Analyze's build-context/PCH lifecycle.
	runner := new(probe.Runner)
	report, err := typedinterop.Analyze(ctx, pkg.Check, pkg.Fset(), typedinterop.Options{
		Clang: cxx, Runner: runner, GOOS: cfg.GOOS, GOARCH: cfg.GOARCH,
		MaxJSON: 4 << 20, MaxDecl: 128 << 10,
	})
	if err != nil {
		return nil, err
	}
	if len(report.IncludeErrors) != 0 || len(report.Table.HeaderErrors) != 0 {
		return nil, nil
	}
	refs := make(map[*types.Var]*collect.Ref)
	for id, object := range pkg.Check.Info.Uses {
		variable, ok := object.(*types.Var)
		if !ok || variable.IsField() || !pkg.Check.Opaque.IsOpaque(id) {
			continue
		}
		ref := report.Set.RefOf(id)
		if ref == nil {
			ref = declaredMemberRef(pkg, report, variable)
		}
		if ref != nil && ref.Incomplete == "" {
			refs[variable] = ref
		}
	}
	objects := make([]*types.Var, 0, len(refs))
	for object := range refs {
		objects = append(objects, object)
	}
	slices.SortFunc(objects, func(a, b *types.Var) int { return int(a.Pos() - b.Pos()) })
	facts := &memberFacts{bindings: make(map[*types.Var]memberAnswer), bytes: 128}
	queried := make(map[probe.MemberQuery]memberAnswer)
	failed := make(map[probe.MemberQuery]bool)
	var firstError error
	for _, object := range objects[:min(len(objects), maxMemberBindings)] {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ref := refs[object]
		if ref.ID < 0 || ref.ID >= len(report.Table.Results) {
			continue
		}
		result := report.Table.Results[ref.ID]
		if !result.OK || result.Recovered != "" || result.Base == "" {
			continue
		}
		// A bound variable is a place even when its initializer was a prvalue. Do not apply
		// this rule to arbitrary expressions; only these exact variable objects are published.
		query := probe.MemberQuery{Type: result.Base, Category: probe.CatLValue}
		if failed[query] {
			continue
		}
		answer, seen := queried[query]
		if !seen {
			if len(queried)+len(failed) >= maxMemberQueries {
				continue
			}
			members, err := runner.Members(ctx, probeConfig, report.Includes, query)
			if err != nil {
				if firstError == nil {
					firstError = err
				}
				failed[query] = true
				continue
			}
			answer = memberAnswer{names: members.Names(), incomplete: members.Incomplete()}
			queried[query] = answer
			facts.bytes += 128
			for _, name := range answer.names {
				facts.bytes += 32 + int64(len(name))
			}
		}
		// Keep the compiler's canonical type of the declaration, not the member query's
		// reference-free base. Today both are equal for every published binding: variables are
		// probed as type forms, and a reference-typed declaration (an alias such as
		// `using Ref = Widget &`) has no member facts, so it is never published. Type stays the
		// source so that hover would show the declaration's real type if references were added.
		// Result.Written is not used: it is the spelling of the declaration's context
		// (`::Widget`, or a class-relative member typedef) and is not a type outside that context.
		answer.nativeType = strings.Clone(result.Type)
		facts.bindings[object] = answer
		facts.bytes += 128 + int64(len(answer.nativeType))
	}
	if !inputs.Unchanged(ctx) {
		return nil, fmt.Errorf("receiver analysis inputs changed")
	}
	if len(facts.bindings) == 0 && firstError != nil {
		return nil, firstError
	}
	return facts, ctx.Err()
}

// An explicit alias may not have an expression reference at its use. Follow the checker's
// exact TypeName binding to an existing collector reference, never the spelling of its name.
func declaredMemberRef(pkg *Package, report *typedinterop.Report, object types.Object) *collect.Ref {
	expr := pkg.Check.Opaque.TypeExpr(object)
	for range 8 {
		if expr == nil {
			return nil
		}
		if ref := report.Set.RefOf(expr); ref != nil && ref.Incomplete == "" {
			return ref
		}
		id, ok := ast.Unparen(expr).(*ast.Ident)
		if !ok {
			return nil
		}
		name, ok := pkg.Check.Info.Uses[id].(*types.TypeName)
		if !ok {
			return nil
		}
		expr = pkg.Check.Opaque.TypeExpr(name)
	}
	return nil
}
