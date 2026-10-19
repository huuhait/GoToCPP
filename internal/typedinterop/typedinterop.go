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
	rep := &Report{Set: set, Includes: UnitIncludes(p)}
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
	paths := set.KindPaths()
	hdrs := set.HeaderNames()
	if len(paths) > 0 && len(rep.Includes) > 0 {
		res, err := interop.ProbeKinds(ctx, cxx, p.Load.Dir, p.ProbeFlags, rep.Includes, append(slices.Clone(paths), hdrs...))
		if err != nil {
			return nil, fmt.Errorf("cannot probe the kinds of the C/C++ names: %w", err)
		}
		rep.Kinds, rep.IncludeErrors = res.Kinds, res.IncludeErrors
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
			res, err = interop.ProbeKinds(ctx, cxx, p.Load.Dir, p.ProbeFlags, rep.Includes, paths)
			if err != nil {
				return nil, fmt.Errorf("cannot probe the kinds of the C/C++ names: %w", err)
			}
			rep.Kinds, rep.IncludeErrors = res.Kinds, res.IncludeErrors
		}
	}
	rep.KindTime = time.Since(t0)
	t, err := rn.Probe(ctx, cfg, set, probe.GenConfig{Includes: rep.Includes, Kinds: rep.Kinds, StringToCStr: opts.StringToCStr})
	if err != nil {
		return rep, err
	}
	rep.Table = t
	return rep, nil
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
	}
	rows := make([]row, len(r.Set.Refs))
	for i, ref := range r.Set.Refs {
		rows[i] = row{RefJSON: ref.JSON()}
		if r.Table != nil {
			rows[i].Result = r.Table.Result(ref.ID)
		}
	}
	src := ""
	if r.Table != nil && r.Table.Source != nil {
		src = r.Table.Source.Text
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
	}{rows, r.Includes, kinds, r.Table, src})
}
