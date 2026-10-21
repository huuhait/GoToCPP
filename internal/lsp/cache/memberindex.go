package cache

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"maps"
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
const maxMemberDeclNames = 64

type memberAnswer struct {
	names      []string
	nativeType string
	incomplete bool

	// receiver is the type that the member query used, and decls the completed declarations of
	// the members that the program writes on bindings of that type (by memberDeclKey). A
	// member without an entry has no proven declaration.
	receiver string
	decls    map[string][]probe.Declaration
}

func memberDeclKey(receiver, name string) string { return receiver + "\x00" + name }

type memberFacts struct {
	bindings map[*types.Var]memberAnswer
	bytes    int64

	// pending is the second step of the analysis (step 12.13c2): where the headers declare the
	// members that the program writes. It runs after the facts above are published, so a slow
	// or failing run delays and loses nothing of them, and the entry gets a copy of the facts
	// with decls when it succeeds. It is nil for facts that have nothing to look up.
	pending func(context.Context) (map[string][]probe.Declaration, int64, error)
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

	// stepStart and stepDone are hooks of the tests, called when a declaration step starts, before
	// it reads anything, and when it has ended.
	stepStart func()
	stepDone  func()
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
	if pkg.hasOpaqueVariableUse() {
		st.p.members.schedule(s.ctx, memberEntryKey(s, pkg), pkg)
	}
}

type opaqueVariableUseKey struct{}

// hasOpaqueVariableUse reports whether the package uses a variable that the checker took as
// opaque. It depends on the check alone: it is memoized, because every request that asks for the
// package schedules the work and must not scan the uses each time.
func (pkg *Package) hasOpaqueVariableUse() bool {
	return pkg.Memo(opaqueVariableUseKey{}, func() any {
		for id, object := range pkg.Check.Info.Uses {
			if variable, ok := object.(*types.Var); ok && !variable.IsField() &&
				pkg.Check.Opaque.IsOpaque(id) {
				return true
			}
		}
		return false
	}).(bool)
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
			var step func(context.Context) (map[string][]probe.Declaration, int64, error)
			var stepCtx context.Context
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
				if step = facts.pending; step != nil {
					stepCtx = entry.ctx
					ix.wg.Add(1)
				}
			}
			ix.mu.Unlock()
			if step != nil {
				go ix.locateStep(stepCtx, entry, facts, step)
			}
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
	bound := objects[:min(len(objects), maxMemberBindings)]
	queries := memberQueries(report, refs, bound)
	early := startEarlyLocate(ctx, runner, probeConfig, report.Includes,
		memberDeclRequest(pkg, earlyBindings(report, refs, bound, queries)))
	defer early.stop()
	queried, failed, firstError := queryMembersTogether(ctx, runner, probeConfig, report, queries)
	for _, answer := range queried {
		facts.bytes += 128
		for _, name := range answer.names {
			facts.bytes += 32 + int64(len(name))
		}
	}
	for _, object := range bound {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		query, ok := bindingQuery(report, refs[object])
		if !ok || failed[query] {
			continue
		}
		answer, seen := queried[query]
		if !seen {
			continue
		}
		result := report.Table.Results[refs[object].ID]
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
	early.wait(ctx)
	if earlyDone != nil {
		earlyDone()
	}
	facts.pending = memberDeclLookup(runner, probeConfig, report.Includes, pkg, facts, inputs,
		early)
	if !inputs.Unchanged(ctx) {
		return nil, fmt.Errorf("receiver analysis inputs changed")
	}
	if len(facts.bindings) == 0 && firstError != nil {
		return nil, firstError
	}
	return facts, ctx.Err()
}

// bindingQuery returns the member query of a bound variable's reference, or false when the
// reference has no usable type. A bound variable is a place even when its initializer was a
// prvalue. Do not apply this rule to arbitrary expressions; only these exact variable objects
// are published.
func bindingQuery(report *typedinterop.Report, ref *collect.Ref) (probe.MemberQuery, bool) {
	if ref.ID < 0 || ref.ID >= len(report.Table.Results) {
		return probe.MemberQuery{}, false
	}
	result := report.Table.Results[ref.ID]
	if !result.OK || result.Recovered != "" || result.Base == "" {
		return probe.MemberQuery{}, false
	}
	return probe.MemberQuery{Type: result.Base, Category: probe.CatLValue}, true
}

// earlyBindings returns the bindings of the variables whose member query is asked for, with the
// receiver of the query and every name accepted.
func earlyBindings(report *typedinterop.Report, refs map[*types.Var]*collect.Ref,
	bound []*types.Var, queries []probe.MemberQuery,
) map[*types.Var]memberBinding {
	bindings := make(map[*types.Var]memberBinding)
	for _, object := range bound {
		query, ok := bindingQuery(report, refs[object])
		if ok && slices.Contains(queries, query) {
			bindings[object] = memberBinding{receiver: query.Type, all: true}
		}
	}
	return bindings
}

// memberQueries returns the distinct queries of the bound variables in the order of their first
// use, at most maxMemberQueries of them.
func memberQueries(report *typedinterop.Report, refs map[*types.Var]*collect.Ref,
	bound []*types.Var,
) []probe.MemberQuery {
	var queries []probe.MemberQuery
	seen := make(map[probe.MemberQuery]bool)
	for _, object := range bound {
		query, ok := bindingQuery(report, refs[object])
		if !ok || seen[query] || len(queries) >= maxMemberQueries {
			continue
		}
		seen[query] = true
		queries = append(queries, query)
	}
	return queries
}

// queryMembersTogether asks the compiler for the members of every query at the same time: the
// queries are independent, each waits for its place among the process's compiler runs, and the
// facts wait for the slowest one instead of the sum. The error is the first one in query order.
func queryMembersTogether(ctx context.Context, runner *probe.Runner, cfg probe.Config,
	report *typedinterop.Report, queries []probe.MemberQuery,
) (map[probe.MemberQuery]memberAnswer, map[probe.MemberQuery]bool, error) {
	answers := make([]memberAnswer, len(queries))
	errs := make([]error, len(queries))
	var wg sync.WaitGroup
	for i, query := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			members, err := runner.Members(ctx, cfg, report.Includes, query)
			if err != nil {
				errs[i] = err
				return
			}
			answers[i] = memberAnswer{names: members.Names(), incomplete: members.Incomplete(),
				receiver: strings.Clone(query.Type)}
		}()
	}
	wg.Wait()
	queried := make(map[probe.MemberQuery]memberAnswer)
	failed := make(map[probe.MemberQuery]bool)
	var firstError error
	for i, query := range queries {
		if errs[i] != nil {
			failed[query] = true
			if firstError == nil {
				firstError = errs[i]
			}
			continue
		}
		queried[query] = answers[i]
	}
	return queried, failed, firstError
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

// A memberBinding is what the declaration lookup needs of a bound variable: the receiver type
// and the member names that are proven for it (all: every name, before the names are known).
type memberBinding struct {
	receiver string
	names    []string
	all      bool
}

// memberDeclRequest returns the lookup of the members that the program writes on the bindings
// (win.size, p.x), for each receiver type once, or an empty request when there are none.
func memberDeclRequest(pkg *Package, bindings map[*types.Var]memberBinding) probe.LocateRequest {
	used := make(map[string][]string) // receiver -> member names written on its bindings
	total := 0
	for _, file := range pkg.Check.Load.Files {
		if file.AST == nil {
			continue
		}
		ast.Inspect(file.AST, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := ast.Unparen(sel.X).(*ast.Ident)
			if !ok {
				return true
			}
			variable, ok := pkg.Check.Info.Uses[id].(*types.Var)
			if !ok {
				return true
			}
			binding, bound := bindings[variable]
			if !bound || binding.receiver == "" {
				return true
			}
			proven := binding.all || slices.Contains(binding.names, sel.Sel.Name)
			if !proven || slices.Contains(used[binding.receiver], sel.Sel.Name) ||
				total >= maxMemberDeclNames {
				return true
			}
			used[binding.receiver] = append(used[binding.receiver], sel.Sel.Name)
			total++
			return true
		})
	}
	var request probe.LocateRequest
	for _, receiver := range slices.Sorted(maps.Keys(used)) {
		request.Members = append(request.Members, probe.LocateMembers{Receiver: receiver, Names: used[receiver]})
	}
	return request
}

// covers reports whether every name of the request is a name of the other one.
func covers(wide, narrow probe.LocateRequest) bool {
	for _, group := range narrow.Members {
		i := slices.IndexFunc(wide.Members, func(g probe.LocateMembers) bool {
			return g.Receiver == group.Receiver
		})
		if i < 0 {
			return false
		}
		for _, name := range group.Names {
			if !slices.Contains(wide.Members[i].Names, name) {
				return false
			}
		}
	}
	return true
}

// memberDecls reads the declarations of the request from the answers of a lookup.
func memberDecls(locations *probe.Locations,
	request probe.LocateRequest,
) map[string][]probe.Declaration {
	decls := make(map[string][]probe.Declaration)
	for _, group := range request.Members {
		for _, name := range group.Names {
			if found := locations.Member(group.Receiver, name); len(found) > 0 {
				decls[memberDeclKey(strings.Clone(group.Receiver), strings.Clone(name))] = found
			}
		}
	}
	return decls
}

// earlyDone is a hook of the tests, called when the early lookup has ended and before the inputs
// of the analysis are compared with what they were.
var earlyDone func()

// earlyGrace is how long the facts wait for the early lookup after the member names are known.
var earlyGrace = 750 * time.Millisecond

// An earlyLocate is the declaration lookup of the second step, started while the member names are
// still asked for. It looks up every member name that the program writes on a bound variable,
// proven or not; the facts use the answers of the proven names only. The two compiler runs do not
// depend on each other, so the cold second step is not a chain behind the first. It is joined
// before the analysis ends, and the facts never wait for it for more than earlyGrace.
type earlyLocate struct {
	request   probe.LocateRequest
	cancel    context.CancelFunc
	done      chan struct{}
	locations *probe.Locations
	err       error
}

func startEarlyLocate(ctx context.Context, runner *probe.Runner, cfg probe.Config,
	includes []string, request probe.LocateRequest,
) *earlyLocate {
	if len(request.Members) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	early := &earlyLocate{request: request, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(early.done)
		early.locations, early.err = runner.Locate(ctx, cfg, includes, request)
	}()
	return early
}

// wait gives the lookup the time of earlyGrace to end; one that is later is stopped.
func (early *earlyLocate) wait(ctx context.Context) {
	if early == nil {
		return
	}
	timer := time.NewTimer(earlyGrace)
	defer timer.Stop()
	select {
	case <-early.done:
	case <-timer.C:
	case <-ctx.Done():
	}
	early.stop()
}

// stop ends the lookup and joins it.
func (early *earlyLocate) stop() {
	if early == nil {
		return
	}
	select {
	case <-early.done:
	default:
		early.cancel()
		<-early.done
	}
	early.cancel() // the timer of the lookup's limit, whichever way it ended
}

// answers returns the lookup's locations when it ended well and asked for every name of the
// request.
func (early *earlyLocate) answers(request probe.LocateRequest) *probe.Locations {
	if early == nil || early.err != nil || early.locations == nil {
		return nil
	}
	if !covers(early.request, request) {
		return nil // a cap on the names cut the request short of a name that is wanted
	}
	return early.locations
}

// memberDeclLookup returns the second step of the member analysis: it asks the compiler where
// the headers declare the members that the program writes on the bound variables (win.size,
// p.x), for each receiver type once, or nil when there are none. The answers are those of an
// unqualified lookup in a class derived from the receiver, so overloads and inherited members
// are the set that the compiler finds. The step is valid only while the inputs of the analysis
// are the same; a failed run gives no declarations, and the names and types of the facts stay.
// The early lookup, when it ended and asked for every name, gives the answers without a run.
func memberDeclLookup(runner *probe.Runner, cfg probe.Config, includes []string, pkg *Package,
	facts *memberFacts, inputs *probe.InputGuard, early *earlyLocate,
) func(context.Context) (map[string][]probe.Declaration, int64, error) {
	bindings := make(map[*types.Var]memberBinding, len(facts.bindings))
	for variable, answer := range facts.bindings {
		bindings[variable] = memberBinding{receiver: answer.receiver, names: answer.names}
	}
	request := memberDeclRequest(pkg, bindings)
	if len(request.Members) == 0 {
		return nil
	}
	return func(ctx context.Context) (map[string][]probe.Declaration, int64, error) {
		locations := early.answers(request)
		if locations == nil {
			var err error
			if locations, err = runner.Locate(ctx, cfg, includes, request); err != nil {
				return nil, 0, err
			}
		}
		if !inputs.Unchanged(ctx) {
			return nil, 0, errors.New("member declaration inputs changed")
		}
		return memberDecls(locations, request), locations.Bytes(), nil
	}
}

// locateStep runs the second step of facts that are published, and adds its declarations to
// them while they are still the current ones of their entry.
func (ix *memberIndexer) locateStep(ctx context.Context, entry *memberEntry, facts *memberFacts,
	pending func(context.Context) (map[string][]probe.Declaration, int64, error),
) {
	defer ix.wg.Done()
	if ix.stepDone != nil {
		defer ix.stepDone()
	}
	if ix.stepStart != nil {
		ix.stepStart()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	decls, cost, err := pending(ctx)
	if err != nil && ctx.Err() == nil {
		// One retry covers headers just saved inside the runner's conservative race window.
		timer := time.NewTimer(revalidateAfter)
		select {
		case <-timer.C:
			decls, cost, err = pending(ctx)
		case <-ctx.Done():
		}
		timer.Stop()
	}
	if err != nil || len(decls) == 0 || ctx.Err() != nil {
		return
	}
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.entries[entry.key] != entry || entry.facts != facts || ctx.Err() != nil ||
		ix.bytes+cost > maxMemberIndexBytes {
		return
	}
	// The published facts are shared with readers: the entry gets a copy that has the
	// declarations.
	next := &memberFacts{bindings: make(map[*types.Var]memberAnswer, len(facts.bindings)),
		bytes: facts.bytes + cost}
	for variable, answer := range facts.bindings {
		answer.decls = decls
		next.bindings[variable] = answer
	}
	entry.facts = next
	entry.bytes += cost
	ix.bytes += cost
}

// ForeignMemberDeclarations returns the completed declarations of the member that the program
// writes on the bound variable (win.size), or false. Like MemberSymbols it reads only retained
// facts for the exact original binding: no compiler, dependency IO or scheduling. A member
// that is not proven, an overload set that is not complete, a repaired or stale binding and a
// partial run give nothing.
func (s *Snapshot) ForeignMemberDeclarations(pkg *Package, object *types.Var, name string) ([]probe.Declaration, bool) {
	answer, ready := s.memberFact(pkg, object)
	if !ready || answer.receiver == "" || !slices.Contains(answer.names, name) {
		return nil, false
	}
	found := answer.decls[memberDeclKey(answer.receiver, name)]
	if len(found) == 0 {
		return nil, false
	}
	return slices.Clone(found), true
}
