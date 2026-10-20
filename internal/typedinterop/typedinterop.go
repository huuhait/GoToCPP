// Package typedinterop is the entry point of typed C/C++ interop (Phase 12).
//
// Analyze runs steps 12.1 and 12.2 on a checked package: the collector
// (internal/typedinterop/collect) lists what the package asks of C/C++, the
// kind probe says what each name is, and the probe (internal/typedinterop/
// probe) lets clang answer. The Report pairs each reference with its typed
// result; later steps (12.3 records, 12.4 type mapper, 12.5 interop
// checker) read it.
package typedinterop

import (
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"slices"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// ErrNoClang is probe.ErrNoClang: typed mode needs clang++.
var ErrNoClang = probe.ErrNoClang

// Options of Analyze.
type Options struct {
	// Clang is the compiler for the probes; the zero value means
	// probe.FindClang. It must be clang.
	Clang build.CXX

	// Runner remembers tables in memory; nil means a new one per call.
	Runner *probe.Runner

	// Timeout, MaxDecl, MaxJSON: see probe.Config.
	Timeout time.Duration
	MaxDecl int
	MaxJSON int64

	// StringToCStr: see probe.GenConfig.
	StringToCStr bool

	// Decls asks for step 12.3 and 12.4: after the probe, the declarations of the
	// classes, enums and functions the results mention are extracted, and every
	// result is mapped to the foreign view (Report.Decls, Mapper, Values). It costs
	// more compiler runs (one per distinct type name), so it is not the default.
	Decls bool

	// Target is the data model of the types (the zero value is LP64);
	// DeclRounds limits how many levels of types are followed from the
	// results into their members (3 when 0); DeclConfig has the limits of the extraction.
	Target     ftype.Target
	DeclRounds int
	DeclLimits probe.DeclConfig
}

// A Report is the references of a package with their results.
type Report struct {
	Set   *collect.Set
	Table *probe.Table

	// Includes are the #include texts of the probe file, Kinds the kind
	// probe's answer, KindTime the time it took (not part of Table.Timing,
	// and cached by interop across runs).
	Includes []string
	Kinds    map[string]interop.Kind
	KindTime time.Duration

	// IncludeErrors are the headers that the kind probe could not include.
	IncludeErrors []interop.IncludeError

	// Decls, Mapper and Values are set by Options.Decls: the extracted
	// declarations (all rounds merged), the mapper that holds them, and the foreign
	// view of each result, indexed by reference ID.
	Decls  *probe.DeclTable
	Mapper *ftype.Mapper
	Values []ftype.Value

	// StringToCStr says that the probe was made for the typed code generation (Options.StringToCStr):
	// that code converts a Go string to a const char* or a std::string parameter itself, so the
	// conversions of the string stand-in are not the whole truth (the interop checker is silent about
	// them).
	StringToCStr bool

	// Stable makes MarshalJSON leave out what describes the run and not the answers (timing,
	// statistics, whether the cache served it, the number of compiler runs), so that the JSON
	// of a cold run and of a warm run are the same bytes.
	Stable bool
}

// UnitIncludes returns the #include texts of the translation unit of the
// package p (the header imports of its files, each once, in file and source
// order, with the extern "C" mark of the build), as the lowerer gives them
// to the kind probe.
func UnitIncludes(p *check.Package) []string {
	var out []string
	seen := make(map[string]bool)
	for _, f := range p.Load.Files {
		if f.Info == nil {
			continue
		}
		for _, inc := range f.Info.Includes {
			t := inc.IncludeText()
			if p.ExternC[t] {
				t = interop.ExternC(t)
			}
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// Analyze collects the references of the checked package p and probes them.
// When there is no clang the error wraps ErrNoClang. A package without header
// imports gives an empty report without running anything.
func Analyze(ctx context.Context, p *check.Package, fset *token.FileSet, opts Options) (*Report, error) {
	cxx := opts.Clang
	if cxx.Path == "" {
		var err error
		if cxx, err = probe.FindClang(ctx); err != nil {
			return nil, err
		}
	}
	set := collect.Collect(p, fset)
	rep := &Report{Set: set, Includes: UnitIncludes(p), StringToCStr: opts.StringToCStr}
	var err error
	rn := opts.Runner
	if rn == nil {
		rn = new(probe.Runner)
	}
	cfg := probe.Config{CXX: cxx, Dir: p.Load.Dir, Flags: p.ProbeFlags, Timeout: opts.Timeout, MaxDecl: opts.MaxDecl, MaxJSON: opts.MaxJSON}
	if len(rep.Includes) == 0 && len(set.Refs) == 0 {
		rep.Table = &probe.Table{Compiler: cxx.Path + " " + cxx.Version}
		return rep, nil
	}
	t0 := time.Now()
	if rep.Kinds, rep.IncludeErrors, err = probeKinds(ctx, cxx, p.Load.Dir, p.ProbeFlags, rep.Includes, set); err != nil {
		return nil, err
	}
	rep.KindTime = time.Since(t0)
	// The overload sets of the functions the program calls do not depend on the probe's answers
	// (only on the references and the kinds), so their run goes beside the probe's.
	var funcs func() (*probe.DeclTable, error)
	fctx, stopFuncs := context.WithCancel(ctx)
	defer stopFuncs()
	if opts.Decls {
		funcs = startFuncDecls(fctx, rep, cfg, rn, opts)
	}
	t, err := rn.Probe(ctx, cfg, set, probe.GenConfig{Includes: rep.Includes, Kinds: rep.Kinds, StringToCStr: opts.StringToCStr})
	if err != nil {
		if funcs != nil {
			stopFuncs() // the overload run is of no use now: stop its compilers
			funcs()     // and wait for the goroutine
		}
		return rep, err
	}
	rep.Table = t
	if opts.Decls {
		if err := extractDecls(ctx, rep, cfg, rn, opts, funcs); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// probeKinds asks the kind probe about the paths of the references of set (and the header packages
// they go through), with the second run for a header package that is also a namespace.
func probeKinds(ctx context.Context, cxx build.CXX, dir string, flags, includes []string, set *collect.Set) (map[string]interop.Kind, []interop.IncludeError, error) {
	paths := set.KindPaths()
	hdrs := set.HeaderNames()
	if len(paths) == 0 || len(includes) == 0 {
		return nil, nil, nil
	}
	res, err := interop.ProbeKinds(ctx, cxx, dir, flags, includes, append(slices.Clone(paths), hdrs...))
	if err != nil {
		return nil, nil, fmt.Errorf("cannot probe the kinds of the C/C++ names: %w", err)
	}
	// A header package whose name is also a namespace (oop for a header that declares
	// namespace oop): its names are looked up in the namespace first, as the opaque build
	// does. Rare, so the second run is made only when it happens.
	ns := make(map[string]bool)
	for _, h := range hdrs {
		if res.Kinds[h] == interop.Namespace {
			ns[h] = true
		}
	}
	if len(ns) > 0 {
		paths = append(append(paths, hdrs...), set.NamespacePaths(ns)...)
		res, err = interop.ProbeKinds(ctx, cxx, dir, flags, includes, paths)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot probe the kinds of the C/C++ names: %w", err)
		}
	}
	return res.Kinds, res.IncludeErrors, nil
}

// declConfig returns the configuration of the declaration runs of opts for the report.
func declConfig(rep *Report, opts Options) probe.DeclConfig {
	dc := opts.DeclLimits
	dc.Includes, dc.StringToCStr = rep.Includes, opts.StringToCStr
	return dc
}

// startFuncDecls starts the extraction of the overload sets of the called functions and returns
// the function that waits for its table.
func startFuncDecls(ctx context.Context, rep *Report, cfg probe.Config, rn *probe.Runner, opts Options) func() (*probe.DeclTable, error) {
	var queries []probe.DeclQuery
	for _, name := range probe.FuncNames(rep.Set, rep.Kinds) {
		queries = append(queries, probe.DeclQuery{Kind: probe.DeclFunc, Name: name})
	}
	if len(queries) == 0 {
		return func() (*probe.DeclTable, error) { return nil, nil }
	}
	dc := declConfig(rep, opts)
	done := make(chan struct{})
	var tab *probe.DeclTable
	var err error
	go func() {
		defer close(done)
		tab, err = rn.Decls(ctx, cfg, dc, queries)
	}()
	return func() (*probe.DeclTable, error) {
		<-done
		return tab, err
	}
}

// extractDecls runs steps 12.3 and 12.4 on the probe results of rep: it asks
// for the declarations that the results need, then for those that the records
// found need (public fields, bases, function signatures), a few rounds deep,
// and maps every result.
func extractDecls(ctx context.Context, rep *Report, cfg probe.Config, rn *probe.Runner, opts Options, funcs func() (*probe.DeclTable, error)) error {
	dc := declConfig(rep, opts)
	rounds := opts.DeclRounds
	if rounds <= 0 {
		rounds = 3
	}
	m := ftype.New(nil, opts.Target)
	all := &probe.DeclTable{Records: map[string]*probe.Record{}, Enums: map[string]*probe.Enum{}, Funcs: map[string]*probe.FuncSet{}, Canon: map[string]string{}, Errors: map[string]string{}}
	if funcs != nil {
		tab, err := funcs()
		if err != nil {
			return fmt.Errorf("cannot extract the declarations: %w", err)
		}
		if tab != nil {
			m.Add(tab)
			mergeDecls(all, tab)
		}
	}
	queries := m.Needs(rep.Table.Results)
	round := 0
	if len(queries) == 0 {
		// The functions' run was the first round: go on with the types of their signatures.
		queries, round = m.NeedsMore(), 1
	}
	for ; round < rounds && len(queries) > 0; round++ {
		tab, err := rn.Decls(ctx, cfg, dc, queries)
		if err != nil {
			return fmt.Errorf("cannot extract the declarations: %w", err)
		}
		m.Add(tab)
		mergeDecls(all, tab)
		queries = m.NeedsMore()
	}
	rep.Decls, rep.Mapper = all, m
	rep.Values = make([]ftype.Value, len(rep.Table.Results))
	for i := range rep.Table.Results {
		rep.Values[i] = m.Value(&rep.Table.Results[i])
	}
	return nil
}

func mergeDecls(dst, src *probe.DeclTable) {
	for k, v := range src.Records {
		dst.Records[k] = v
	}
	for k, v := range src.Enums {
		dst.Enums[k] = v
	}
	for k, v := range src.Funcs {
		dst.Funcs[k] = v
	}
	for k, v := range src.Canon {
		dst.Canon[k] = v
	}
	for k, v := range src.Errors {
		dst.Errors[k] = v
	}
	dst.HeaderErrors = append(dst.HeaderErrors, src.HeaderErrors...)
	dst.Runs += src.Runs
	dst.Deps = append(dst.Deps, src.Deps...)
	dst.Timing.Compile += src.Timing.Compile
	dst.Timing.CPU += src.Timing.CPU
	dst.Timing.Total += src.Timing.Total
	dst.Stats.Decls += src.Stats.Decls
	dst.Stats.JSONBytes += src.Stats.JSONBytes
}

// Collect runs only the collector (step 12.1) for the checked package p and
// returns a report without probe results.
func Collect(p *check.Package, fset *token.FileSet) *Report {
	return &Report{Set: collect.Collect(p, fset), Includes: UnitIncludes(p)}
}

// MarshalJSON writes the report as the developer tool prints it: for each
// reference its key, kind, sites and probe result, then the table's
// header errors, timing and statistics.
func (r *Report) MarshalJSON() ([]byte, error) {
	type row struct {
		collect.RefJSON
		Result *probe.Result `json:"result"`
		Mapped string        `json:"mapped,omitempty"`
	}
	rows := make([]row, len(r.Set.Refs))
	for i, ref := range r.Set.Refs {
		rows[i] = row{RefJSON: ref.JSON()}
		if r.Table != nil {
			rows[i].Result = r.Table.Result(ref.ID)
		}
		if i < len(r.Values) {
			rows[i].Mapped = r.Values[i].String()
		}
	}
	src := ""
	if r.Table != nil && r.Table.Source != nil {
		src = r.Table.Source.Text
	}
	table, decls := r.Table, r.Decls
	if r.Stable {
		if table != nil {
			c := *table
			c.Timing, c.Stats, c.Cached = probe.Timing{}, probe.Stats{}, false
			table = &c
		}
		if decls != nil {
			c := *decls
			c.Timing, c.Stats, c.Cached, c.Runs = probe.Timing{}, probe.Stats{}, false, 0
			decls = &c
		}
	}
	kinds := make(map[string]string, len(r.Kinds))
	for k, v := range r.Kinds {
		kinds[k] = v.String()
	}
	return json.Marshal(struct {
		Refs     []row             `json:"refs"`
		Includes []string          `json:"includes"`
		Kinds    map[string]string `json:"kinds,omitempty"`
		Table    *probe.Table      `json:"table"`
		Source   string            `json:"source,omitempty"`
		Decls    *probe.DeclTable  `json:"decls,omitempty"`
	}{rows, r.Includes, kinds, table, src, decls})
}
