package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
)

// Step 12.3: the declarations of the names a program uses.
//
// The expression probes (gen.go) say what type an expression has. They do not
// list the members of that type. For that, clang's AST dump of the real
// declaration is needed, and the dump can only be filtered by a substring of
// the declaration's qualified name (-ast-dump-filter takes one string). So
// the extraction is a second, separate kind of run:
//
//   - A type (a class, struct, union or enum) is asked for by its canonical
//     spelling. The run's file has the package's includes and, for each type,
//     an alias whose name contains the type's last name (gp_d3_Vec for geo::Vec):
//     the filter "Vec" then dumps the real declaration (and whatever else has
//     "Vec" in its name, which the reader skips) and the alias, whose type tree names
//     the declaration's id. The id, not the name, selects the declaration: a
//     typedef of an unnamed struct has no name, and std::vector<long> is one
//     specialization among several. The size, alignment and underlying type of
//     an enum are asked as constants (a variable template that is -1 for an
//     incomplete type). Types with the same last name share a run; different
//     names run in parallel.
//   - A function is asked for by its qualified name. A using-declaration in a
//     namespace that carries a unique prefix (namespace gp_f0_fopen { using ::fopen; })
//     lets clang list every overload with its type, in one run for all functions.
//
// Member types are written relative to the record's scope in clang's dump
// ("Vec" inside namespace geo): the reader resolves the member typedefs, the
// nested types and the injected class name, and leaves the rest to the mapper,
// which knows the scope (Record.Scope).

// DeclConfig says how to extract declarations.
type DeclConfig struct {
	// Includes and StringToCStr are the probe file's (GenConfig).
	Includes     []string
	StringToCStr bool

	// Parallel is the number of this call's runs that are started at the same time
	// (Jobs() when 0); the compilers that run at once in the process are bounded by
	// Jobs() anyway (limiter.go). MaxRuns the largest number of type runs for one call (64 when 0): more
	// names than that are answered with an error, not probed.
	Parallel int
	MaxRuns  int

	// MaxJSON limits the dump of one run (32 MB when 0). A name like
	// std::basic_string has 40 MB of declarations with bodies in the library's
	// headers; reading them costs seconds, and the answer would be big and useless to
	// typing, so such a run stops and the names it was for have an error.
	MaxJSON int64
}

// maxSavedDecls bounds the bytes of candidate declarations kept in memory
// while a run is read (the ids come at the end of the dump).
const maxSavedDecls = 96 << 20

var (
	declAliasRx = regexp.MustCompile(`^gp_d(\d+)([csau]?)_`)
	identRx     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	kindRx      = regexp.MustCompile(`"kind": "(\w+)"`)
	idRx        = regexp.MustCompile(`"id": "(0x[0-9a-f]+)"`)
)

const declHelpers = `template <class T> constexpr long gocpp_decl_internal_size = [] { if constexpr (requires { sizeof(T); }) return (long)sizeof(T); else return -1L; }();
template <class T> constexpr long gocpp_decl_internal_align = [] { if constexpr (requires { sizeof(T); }) return (long)alignof(T); else return -1L; }();
template <class T> struct gocpp_decl_internal_ut { using type = void; };
template <class T> requires std::is_enum_v<T> struct gocpp_decl_internal_ut<T> { using type = std::underlying_type_t<T>; };
`

// unsafeName says why a name cannot be written into a probe file, or "": the text of a
// query comes from the program, and the file has one line per query, so a comment, a
// directive, a string, a semicolon or a brace in it would change the other queries' lines.
func unsafeName(s string) string {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c < 0x20 || c == 0x7f:
			return "a control character"
		case strings.IndexByte("#;\"\\`{}", c) >= 0:
			return "the character " + strconv.Quote(string(c))
		case i+1 < len(s) && (s[i:i+2] == "//" || s[i:i+2] == "/*" || s[i:i+2] == "*/"):
			return "a comment"
		}
	}
	return ""
}

// hasArgs reports whether a name has template arguments in any part.
func hasArgs(t *ctype.Type) bool {
	for _, s := range t.Path {
		if s.HasArgs {
			return true
		}
	}
	return false
}

// A typeRun is one compiler run for the types with the same last name.
type typeRun struct {
	last    string
	queries []DeclQuery
	types   []*ctype.Type
}

// declPart is the answer of one run.
type declPart struct {
	records map[string]*Record
	enums   map[string]*Enum
	canon   map[string]string
	funcs   map[string]*FuncSet
	errs    map[string]string
	header  []Diag
	stats   Stats
	deps    []string
	timing  Timing

	// refusal says that the run was stopped for the size of the dump: no dependency file.
	refusal bool
}

// Decls extracts the declarations that the queries ask for. A query that
// cannot be answered has an entry in DeclTable.Errors; the error result is for
// a failure of the whole call (the context, a compiler that cannot run at
// all).
func (rn *Runner) Decls(ctx context.Context, cfg Config, dc DeclConfig, queries []DeclQuery) (*DeclTable, error) {
	t0 := time.Now()
	out := &DeclTable{Records: map[string]*Record{}, Enums: map[string]*Enum{}, Funcs: map[string]*FuncSet{}, Canon: map[string]string{}, Errors: map[string]string{}}
	if dc.Parallel <= 0 {
		dc.Parallel = Jobs()
	}
	if dc.MaxRuns <= 0 {
		dc.MaxRuns = 64
	}
	seen := make(map[DeclQuery]bool)
	byLast := make(map[string]*typeRun)
	var order []string
	var funcs []DeclQuery
	for _, q := range queries {
		q.Name = strings.TrimPrefix(q.Name, "::")
		if seen[q] {
			continue
		}
		seen[q] = true
		if why := unsafeName(q.Name); why != "" && (q.Kind == DeclType || q.Kind == DeclFunc) {
			out.Errors[QueryKey(q)] = "the name has " + why + ", which a probe file cannot take"
			continue
		}
		switch q.Kind {
		case DeclType:
			t, err := ctype.Parse(q.Name)
			switch {
			case err != nil:
				out.Errors[QueryKey(q)] = err.Error()
			case t.Kind != ctype.Named || t.Const || t.Volatile || t.Tag != "":
				out.Errors[QueryKey(q)] = "not a plain class or enum name"
			case strings.HasPrefix(q.Name, "gocpp_probe_internal_"):
				out.Errors[QueryKey(q)] = "a stand-in of gocpp's runtime types"
			case !identRx.MatchString(t.Last()):
				out.Errors[QueryKey(q)] = "a name that cannot be used in a probe"
			default:
				r := byLast[t.Last()]
				if r == nil {
					r = &typeRun{last: t.Last()}
					byLast[t.Last()] = r
					order = append(order, t.Last())
				}
				r.queries = append(r.queries, q)
				r.types = append(r.types, t)
			}
		case DeclFunc:
			if t, err := ctype.Parse(q.Name); err != nil || t.Kind != ctype.Named || !identRx.MatchString(t.Last()) || t.Const || t.Volatile || t.Tag != "" || hasArgs(t) {
				out.Errors[QueryKey(q)] = "not a function name"
			} else {
				funcs = append(funcs, q)
			}
		default:
			out.Errors[QueryKey(q)] = "unknown kind of query"
		}
	}
	sort.Strings(order)
	if len(order) > dc.MaxRuns {
		for _, last := range order[dc.MaxRuns:] {
			for _, q := range byLast[last].queries {
				out.Errors[QueryKey(q)] = fmt.Sprintf("more than %d names to look up at once", dc.MaxRuns)
			}
		}
		order = order[:dc.MaxRuns]
	}

	if dc.MaxJSON <= 0 {
		dc.MaxJSON = 32 << 20
	}
	cfg.MaxJSON = dc.MaxJSON
	var jobs []func() (*declPart, bool, error)
	var jobQueries [][]DeclQuery
	for _, last := range order {
		r := byLast[last]
		jobs = append(jobs, func() (*declPart, bool, error) { return rn.typeRun(ctx, cfg, dc, r) })
		jobQueries = append(jobQueries, r.queries)
	}
	if len(funcs) > 0 {
		jobs = append(jobs, func() (*declPart, bool, error) { return rn.funcRun(ctx, cfg, dc, funcs) })
		jobQueries = append(jobQueries, funcs)
	}
	parts := make([]*declPart, len(jobs))
	cached := make([]bool, len(jobs))
	errs := make([]error, len(jobs))
	sem := make(chan struct{}, dc.Parallel)
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			parts[i], cached[i], errs[i] = jobs[i]()
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("extracting declarations was stopped: %w", err)
	}
	depSet := make(map[string]bool)
	allCached := true
	for i, p := range parts {
		if errs[i] != nil {
			for _, q := range jobQueries[i] {
				out.Errors[QueryKey(q)] = errs[i].Error()
			}
			continue
		}
		if !cached[i] {
			allCached = false
			out.Runs++
		}
		for k, v := range p.records {
			out.Records[k] = v
		}
		for k, v := range p.enums {
			out.Enums[k] = v
		}
		for k, v := range p.funcs {
			out.Funcs[k] = v
		}
		for k, v := range p.canon {
			out.Canon[k] = v
		}
		for k, v := range p.errs {
			out.Errors[k] = v
		}
		out.HeaderErrors = append(out.HeaderErrors, p.header...)
		for _, d := range p.deps {
			depSet[d] = true
		}
		out.Stats.Decls += p.stats.Decls
		out.Stats.JSONBytes += p.stats.JSONBytes
		out.Stats.LargestDecl = max(out.Stats.LargestDecl, p.stats.LargestDecl)
		out.Timing.Compile += p.timing.Compile
		out.Timing.CPU += p.timing.CPU
		out.Timing.Parse += p.timing.Parse
	}
	for d := range depSet {
		out.Deps = append(out.Deps, d)
	}
	sort.Strings(out.Deps)
	sort.Slice(out.HeaderErrors, func(i, j int) bool {
		return out.HeaderErrors[i].File+out.HeaderErrors[i].Msg < out.HeaderErrors[j].File+out.HeaderErrors[j].Msg
	})
	out.Cached = allCached && len(jobs) > 0
	out.Timing.Total = time.Since(t0)
	return out, nil
}

// memoRun runs a declaration run once per probe text: the part is looked up in the cache (memory,
// then disk) under the key of the text and the contents of the headers that the run read, and
// made by run when it is not there. The runs of one key go one at a time in this process and
// on this machine (cache.go has the lock); the others find the part when the first is done.
func (rn *Runner) memoRun(ctx context.Context, cfg Config, includes []string, text, preamble string, run func() (*declPart, error)) (*declPart, bool, error) {
	dir, write := rn.diskFor(cfg)
	tc := rn.toolchain(ctx, cfg, dir, write)
	pre := withToolchain(PreKey(cfg, &Source{Text: text}), tc)
	name := entryName("d", selfID(), pre)
	fkey := "d" + pre
	lookup := func() *declPart {
		v := rn.findVariant(dir, name, "d")
		if v == nil {
			return nil
		}
		p, err := decodePart(v.Part, v.Deps)
		if err != nil {
			cacheCount.corrupt.Add(1)
			return nil
		}
		return p
	}
	for round := 0; round < 16; round++ {
		if p := lookup(); p != nil {
			return p, true, nil
		}
		rn.mu.Lock()
		if other := rn.flying[fkey]; other != nil {
			rn.mu.Unlock()
			select {
			case <-other.done:
			case <-ctx.Done():
				return nil, false, fmt.Errorf("%s was stopped: %w", cfg.CXX.Name(), ctx.Err())
			}
			if other.err != nil && !errors.Is(other.err, context.Canceled) {
				return nil, false, other.err
			}
			continue
		}
		fl := &flight{done: make(chan struct{})}
		if rn.flying == nil {
			rn.flying = make(map[string]*flight)
		}
		rn.flying[fkey] = fl
		rn.mu.Unlock()
		p, hit, err := rn.makePart(ctx, cfg, includes, text, preamble, dir, write, tc, name, fkey, lookup, run)
		rn.mu.Lock()
		delete(rn.flying, fkey)
		rn.mu.Unlock()
		fl.err = err
		close(fl.done)
		if err != nil {
			return nil, false, err
		}
		if p == nil {
			continue
		}
		return p, hit, nil
	}
	return nil, false, errors.New("the declaration run kept being taken by others")
}

// makePart is the part of memoRun that holds the flight: it looks again (a caller that came
// after another finished), takes the lock on disk, runs and stores. It returns a nil part
// when another process made it meanwhile, and the caller looks again.
func (rn *Runner) makePart(ctx context.Context, cfg Config, includes []string, text, preamble, dir string, write bool, tc toolchain, name, fkey string,
	lookup func() *declPart, run func() (*declPart, error)) (*declPart, bool, error) {
	if p := lookup(); p != nil {
		return p, true, nil
	}
	if dir != "" {
		release, waited := acquireLock(ctx, dir, fkey, effTimeout(cfg))
		if release != nil {
			defer release()
		}
		if waited {
			if p := lookup(); p != nil {
				return p, true, nil
			}
		}
	}
	start := time.Now()
	p, err := run()
	if err != nil {
		return nil, false, err
	}
	rn.mu.Lock()
	rn.declRuns++
	rn.mu.Unlock()
	cacheCount.runs.Add(1)
	deps := p.deps
	if len(deps) == 0 && p.refusal {
		// The run was stopped (the dump is too large) or a header is missing: the compiler wrote
		// no dependency file. What the answer depends on is still the headers of the file.
		deps = preprocessDeps(ctx, cfg, text)
	}
	// (The includes are those of the preamble: the query below it has no directive, and one question serves
	// every run of the same preamble.)
	deps = rn.finalDeps(ctx, cfg, preamble, includes, deps, p.header, tc)
	p.deps = deps
	body, err := encodePart(p)
	if err != nil {
		return p, false, nil // not cacheable; the run's own part is the answer
	}
	// The part the caller gets is the one read back, as a later call gets it.
	if back, err := decodePart(body, deps); err == nil {
		p = back
	}
	if racyDeps(deps, start) {
		return p, false, nil // a header changed while the compiler ran: not kept anywhere (racyDeps)
	}
	rn.addVariant(dir, name, "d", fmt.Sprintf("declarations %s %s", cfg.CXX.Name(), canonPath(cfg.Dir)),
		variant{Deps: deps, DepsHash: DepsHash(deps), Part: body}, write && len(deps) > 0)
	return p, false, nil
}

// DeclRuns returns the number of compiler runs of Decls so far.
func (rn *Runner) DeclRuns() int {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return rn.declRuns
}

// ---------------------------------------------------------------- type runs

func (rn *Runner) typeRun(ctx context.Context, cfg Config, dc DeclConfig, r *typeRun) (*declPart, bool, error) {
	g := &gen{lines: make(map[int]Line), skipped: make(map[int]string), aliases: make(map[string]bool)}
	g.preamble("// gocpp typed interop: declaration probe file (generated)", dc.Includes, dc.StringToCStr)
	preamble := g.b.String()
	g.b.WriteString(declHelpers)
	g.line += strings.Count(declHelpers, "\n")
	lineQ := make(map[int]int)
	for i := range r.queries {
		for _, role := range []string{"", "c", "s", "a", "u"} {
			var text string
			x := r.types[i].String() // the parsed type, not the text of the query
			switch role {
			case "":
				text = fmt.Sprintf("using gp_d%d_%s = %s;", i, r.last, x)
			case "c":
				text = fmt.Sprintf("using gp_d%dc_%s = gocpp_probe_internal_c<%s>;", i, r.last, x)
			case "s":
				text = fmt.Sprintf("using gp_d%ds_%s = gocpp_probe_internal_val<gocpp_decl_internal_size<%s>>;", i, r.last, x)
			case "a":
				text = fmt.Sprintf("using gp_d%da_%s = gocpp_probe_internal_val<gocpp_decl_internal_align<%s>>;", i, r.last, x)
			case "u":
				text = fmt.Sprintf("using gp_d%du_%s = gocpp_probe_internal_c<typename gocpp_decl_internal_ut<%s>::type>;", i, r.last, x)
			}
			g.emit(text, nil)
			lineQ[g.line] = i
		}
	}
	return rn.memoRun(ctx, cfg, dc.Includes, g.b.String(), preamble, func() (*declPart, error) {
		return runTypeDump(ctx, cfg, g.b.String(), r, lineQ)
	})
}

// A wanted is what the aliases of one query said.
type wanted struct {
	id, kind    string
	size, align int64
	ut          string
	canon       string
	haveSize    bool
}

func runTypeDump(ctx context.Context, cfg Config, text string, r *typeRun, lineQ map[int]int) (*declPart, error) {
	want := make([]wanted, len(r.queries))
	for i := range want {
		want[i].size, want[i].align = -1, -1
	}
	type saved struct {
		kind string
		id   string
		raw  []byte
	}
	var cands []saved
	var savedBytes int64
	overflow := false
	tooLarge := 0
	var parse time.Duration
	d, err := runDump(ctx, cfg, text, r.last, func(rd RawDecl) error {
		if m := declAliasRx.FindStringSubmatch(rd.Name); m != nil && !rd.TooLarge {
			idx, _ := strconv.Atoi(m[1])
			if idx >= len(want) {
				return nil
			}
			p0 := time.Now()
			defer func() { parse += time.Since(p0) }()
			var n dnode
			if err := json.Unmarshal(rd.Raw, &n); err != nil {
				return nil
			}
			switch m[2] {
			case "":
				want[idx].id, want[idx].kind = declIDOf(&n)
			case "c":
				if f, err := decodeDecl(rd.Raw); err == nil && f.wrapped {
					want[idx].canon = f.typ
				}
			case "s":
				if v, ok := n.firstConst(); ok && v.IsInt64() {
					want[idx].size, want[idx].haveSize = v.Int64(), true
				}
			case "a":
				if v, ok := n.firstConst(); ok && v.IsInt64() {
					want[idx].align = v.Int64()
				}
			case "u":
				if f, err := decodeDecl(rd.Raw); err == nil && f.wrapped && f.typ != "void" {
					want[idx].ut = f.typ
				}
			}
			return nil
		}
		if rd.TooLarge {
			if k := kindRx.FindSubmatch(rd.Head); k != nil && (string(k[1]) == "ClassTemplateDecl" || string(k[1]) == "ClassTemplateSpecializationDecl" || string(k[1]) == "CXXRecordDecl") {
				tooLarge++
			}
			return nil
		}
		k := kindRx.FindSubmatch(rd.Raw[:min(len(rd.Raw), 600)])
		if k == nil {
			return nil
		}
		switch string(k[1]) {
		case "CXXRecordDecl", "RecordDecl", "EnumDecl", "ClassTemplateDecl", "ClassTemplateSpecializationDecl":
		default:
			return nil
		}
		if savedBytes+int64(len(rd.Raw)) > maxSavedDecls {
			overflow = true
			return nil
		}
		savedBytes += int64(len(rd.Raw))
		s := saved{kind: string(k[1]), raw: append([]byte(nil), rd.Raw...)}
		if m := idRx.FindSubmatch(rd.Raw[:min(len(rd.Raw), 600)]); m != nil {
			s.id = string(m[1])
		}
		cands = append(cands, s)
		return nil
	})
	if errors.Is(err, ErrDumpTooLarge) {
		// The names of this run have no answer, and the answer is remembered: another call
		// would read the same 30 MB again.
		part := &declPart{errs: map[string]string{}, refusal: true}
		for _, q := range r.queries {
			part.errs[QueryKey(q)] = "the declaration is larger than the limit of the extraction"
		}
		return part, nil
	}
	if err != nil {
		return nil, err
	}
	part := &declPart{records: map[string]*Record{}, enums: map[string]*Enum{}, canon: map[string]string{}, errs: map[string]string{}, stats: d.stats, deps: d.deps}
	part.timing = Timing{Compile: d.compile, CPU: d.cpu, Parse: parse}
	errOf := compilerErrors(d, lineQ, part)
	for i, q := range r.queries {
		key := QueryKey(q)
		w := want[i]
		if w.canon != "" {
			part.canon[q.Name] = w.canon
		}
		if w.id == "" {
			msg := errOf[i]
			switch {
			case msg != "":
			case w.canon != "":
				continue // a type that is neither a record nor an enum (a typedef of a builtin): the canonical type is the answer
			default:
				msg = "no type of that name"
			}
			part.errs[key] = msg
			continue
		}
		spelling := q.Name
		if w.canon != "" {
			spelling = w.canon
		}
		quoted := []byte(`"` + w.id + `"`)
		// The declaration, preferably the one with the definition: a template that is
		// instantiated explicitly (extern template class basic_string<char>) is also
		// dumped as a stub.
		var node *dnode
		for _, c := range cands {
			if c.id != w.id && !bytes.Contains(c.raw, quoted) {
				continue
			}
			p0 := time.Now()
			var n dnode
			if err := json.Unmarshal(c.raw, &n); err == nil {
				if f := n.find(w.id); f != nil && (node == nil || f.CompleteDefinition && !node.CompleteDefinition) {
					node = f
				}
			}
			part.timing.Parse += time.Since(p0)
			if node != nil && (node.CompleteDefinition || node.Kind == "EnumDecl") {
				break
			}
		}
		if node != nil && node.Kind != "EnumDecl" && !node.CompleteDefinition && w.size >= 0 && (tooLarge > 0 || overflow) {
			node = nil // the compiler knows the size: this is a stub, and the definition was too large to read
		}
		if node == nil {
			switch {
			case tooLarge > 0 || overflow:
				part.errs[key] = "the declaration is larger than the limit of the extraction"
			default:
				part.errs[key] = "the declaration is not in the compiler's dump (more than the limit, or a name that the filter cannot reach)"
			}
			continue
		}
		if node.Kind == "EnumDecl" {
			en, err := decodeEnum(node, spelling, w.ut)
			if err != nil {
				part.errs[key] = err.Error()
				continue
			}
			en.Size = w.size
			part.enums[spelling] = en
			continue
		}
		rec, err := decodeRecord(node, spelling)
		if err != nil {
			part.errs[key] = err.Error()
			continue
		}
		rec.Size, rec.Align = w.size, w.align
		if !rec.Complete && rec.Partial == "" {
			rec.Size, rec.Align = -1, -1
		}
		part.records[spelling] = rec
	}
	return part, nil
}

// compilerErrors attributes the compiler's errors to the queries by line (the
// errors in headers go to the part); it returns the first message per query.
func compilerErrors(d *dump, lineQ map[int]int, part *declPart) map[int]string {
	out := make(map[int]string)
	for _, g := range ParseDiags(d.stderr, d.srcPath) {
		idx, ok := -1, false
		for _, x := range append([]Diag{g.Err}, g.Notes...) {
			if isProbeFile(x.File, "") {
				if i, found := lineQ[x.Line]; found {
					idx, ok = i, true
					break
				}
			}
		}
		if !ok {
			part.header = append(part.header, g.Err)
			continue
		}
		if _, dup := out[idx]; !dup {
			out[idx] = g.Err.Msg
		}
	}
	return out
}

// ---------------------------------------------------------------- function runs

func (rn *Runner) funcRun(ctx context.Context, cfg Config, dc DeclConfig, qs []DeclQuery) (*declPart, bool, error) {
	g := &gen{lines: make(map[int]Line), skipped: make(map[int]string), aliases: make(map[string]bool)}
	g.preamble("// gocpp typed interop: function declaration probe file (generated)", dc.Includes, dc.StringToCStr)
	preamble := g.b.String()
	lineQ := make(map[int]int)
	for i, q := range qs {
		t, _ := ctype.Parse(q.Name)
		g.emit(fmt.Sprintf("namespace gp_f%d_%s { using ::%s; }", i, t.Last(), t.String()), nil)
		lineQ[g.line] = i
	}
	text := g.b.String()
	return rn.memoRun(ctx, cfg, dc.Includes, text, preamble, func() (*declPart, error) {
		return runFuncDump(ctx, cfg, text, qs, lineQ)
	})
}

var funcNsRx = regexp.MustCompile(`^gp_f(\d+)_`)

func runFuncDump(ctx context.Context, cfg Config, text string, qs []DeclQuery, lineQ map[int]int) (*declPart, error) {
	sets := make([]*FuncSet, len(qs))
	var parse time.Duration
	d, err := runDump(ctx, cfg, text, "gp_f", func(rd RawDecl) error {
		m := funcNsRx.FindStringSubmatch(rd.Name)
		if m == nil || rd.TooLarge {
			return nil
		}
		idx, _ := strconv.Atoi(m[1])
		if idx >= len(qs) {
			return nil
		}
		p0 := time.Now()
		defer func() { parse += time.Since(p0) }()
		var n dnode
		if err := json.Unmarshal(rd.Raw, &n); err != nil {
			return nil
		}
		t, _ := ctype.Parse(qs[idx].Name)
		fs := &FuncSet{Name: qs[idx].Name, Scope: scopeOf(t)}
		env := &env{}
		for i := range n.Inner {
			c := &n.Inner[i]
			if c.Kind != "UsingShadowDecl" || c.Target == nil {
				continue
			}
			ov := Overload{Kind: "function"}
			switch c.Target.Kind {
			case "FunctionDecl":
			case "FunctionTemplateDecl":
				ov.Kind = "template"
			case "VarDecl":
				ov.Kind = "variable"
			default:
				ov.Kind = c.Target.Kind
			}
			if c.Target.Type != nil {
				ov.Sig = env.parseIn(normalizeType(c.Target.Type.QualType))
			}
			fs.Overloads = append(fs.Overloads, ov)
		}
		sets[idx] = fs
		return nil
	})
	if err != nil {
		return nil, err
	}
	part := &declPart{funcs: map[string]*FuncSet{}, errs: map[string]string{}, stats: d.stats, deps: d.deps}
	part.timing = Timing{Compile: d.compile, CPU: d.cpu, Parse: parse}
	errOf := compilerErrors(d, lineQ, part)
	for i, q := range qs {
		switch {
		case sets[i] != nil && len(sets[i].Overloads) > 0:
			part.funcs[q.Name] = sets[i]
		case errOf[i] != "":
			part.errs[QueryKey(q)] = errOf[i]
		default:
			part.errs[QueryKey(q)] = "the compiler lists no function of that name"
		}
	}
	return part, nil
}
