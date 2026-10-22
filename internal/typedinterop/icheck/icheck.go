// Package icheck is step 12.5 of typed interop (Phase 12): the interop checker.
//
// Stock go/types knows nothing about C and C++: every expression that touches
// a C/C++ value is invalid for it, so opaque mode leaves all of them to the C++
// compiler, whose errors come late (after the whole program is lowered) and in
// C++ words. The checker reads what clang said about each of them (the probe
// results of internal/typedinterop, with the declarations and the foreign view
// of step 12.4) and reports, at the position in the .gp file and in Go's words,
// the errors that the C++ compiler is going to report.
//
// The rule that the whole package serves is the compatibility rule of the
// Phase 12 plan: typed mode must never reject a program that opaque mode and
// the C++ compiler accept. So an error is reported only when it is certain:
//
//   - the probe is an exact mirror of the generated code: the same C++
//     expression, written with real types (see exact.go for what is not: the
//     stand-ins of Go's slices, arrays and maps, composite literals, values that
//     go through a lambda, code of a generic function that may never be
//     instantiated, and an expression that could be read two ways);
//   - the compiler printed a diagnostic for that very expression (not a limit of
//     the probe, a missing helper or an error that another probe caused);
//   - no header failed to compile and the probe ran with the target's own headers;
//   - and when the build uses g++ (clang's answer may differ: headers branch on
//     __clang__) the errors are warnings.
//
// Everything else stays silent: the expression is opaque and the C++ compiler
// decides, exactly as in opaque mode. Silent says why in Result.Silent, for the
// tests and the developer tool (gocpp __typed-check -v).
//
// Besides the failed probes, the checker applies the typed rules to the
// expressions that did compile (rules.go): a value of void used as a value, a
// value that no C++ conversion takes to the Go type it is assigned or passed to
// (ftype.Value.Compat says No), an assignment to a C++ value that is not an lvalue.
package icheck

import (
	"context"
	"fmt"
	"go/token"
	"path/filepath"
	"slices"
	"sort"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// Options of Check.
type Options struct {
	// Analyze are the options of the probe (the compiler for the probes, the
	// runner that remembers answers, the limits). Decls is always on.
	Analyze typedinterop.Options

	// Build is the family of the compiler that builds the program: with GCC
	// the errors of the checker are warnings, because clang (the probe) and g++
	// can disagree. The zero value means the compiler that gocpp build would use
	// (build.FindCXX); if there is none, Clang.
	Build build.Family

	// Cross says that the target is not this machine: the probes see this
	// machine's headers, so nothing is reported.
	Cross bool
}

// A Silent is something that the checker saw and left to the C++ compiler.
type Silent struct {
	Pos token.Position
	Key string // the reference
	Why string // why it is not an error
	Msg string // what it would have said, when it is a failed probe
}

// A Result is the answer of Check for one package.
type Result struct {
	// Diags are the errors and warnings, in source order.
	Diags []diag.Diagnostic

	// Silent are the references that failed or could be checked but were left
	// to the C++ compiler, with the reason.
	Silent []Silent

	// Skipped is not empty when the package was not checked at all, and says why
	// (no C/C++ in it, a header that does not compile, a cross target, a probe that
	// could not run).
	Skipped string

	// Report is the probe's report (nil when Skipped before the probe ran).
	Report *typedinterop.Report

	// Stats counts what the checker looked at.
	Stats Stats
}

// Stats are the numbers of one Check, for the developer tool and the tests.
type Stats struct {
	Refs     int // references of the package
	Probed   int // references with a probe result (compiled or failed)
	Compiled int // probes that compiled
	Failed   int // probes that failed
	Sites    int // uses checked (of the compiled and the failed ones)
	Adapt    int // refusals of the argument adaptations (adapt.go)
}

// Errors returns the number of errors (not warnings) in the result.
func (r *Result) Errors() int {
	n := 0
	for _, d := range r.Diags {
		if d.Severity == diag.Error {
			n++
		}
	}
	return n
}

// Check runs the typed checks on the checked package p. The error is
// typedinterop.ErrNoClang (wrapped) when there is no clang++, and the caller
// then stays in opaque mode; any other problem of the probe is a Skipped
// result, not an error.
func Check(ctx context.Context, p *check.Package, fset *token.FileSet, opts Options) (*Result, error) {
	if p == nil || !p.Checked() {
		return &Result{Skipped: "the package did not type-check"}, nil
	}
	if len(typedinterop.UnitIncludes(p)) == 0 {
		return &Result{Skipped: "no C/C++ headers are imported"}, nil
	}
	if opts.Cross {
		return &Result{Skipped: "the target is not this machine: the probes would read the wrong headers"}, nil
	}
	ao := opts.Analyze
	ao.Decls = true
	rep, err := typedinterop.Analyze(ctx, p, fset, ao)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr // interrupted: not a package that was skipped
		}
		if rep == nil {
			return nil, err
		}
		return &Result{Skipped: "the probe could not run: " + err.Error(), Report: rep}, nil
	}
	fam := opts.Build
	if fam == 0 {
		fam = build.Clang
		if cxx, err := build.FindCXX(ctx); err == nil {
			fam = cxx.Family
		}
	}
	return CheckReport(p, fset, rep, fam), nil
}

// CheckReport applies the checker to a report that Analyze made with
// Options.Decls (the probe, the declarations and the mapped values). fam is the
// family of the compiler that builds the program.
func CheckReport(p *check.Package, fset *token.FileSet, rep *typedinterop.Report, fam build.Family) *Result {
	return CheckReportWithout(p, fset, rep, fam, false)
}

// CheckReportWithout is CheckReport that leaves the argument adaptation refusals out when
// withoutAdaptations is set. Only a test uses it (GOCPP_TEST_NO_ADAPT_ERRORS in `gocpp build`), to
// see the refusals that the lowering makes on its own, which the pass would otherwise report first.
func CheckReportWithout(p *check.Package, fset *token.FileSet, rep *typedinterop.Report,
	fam build.Family, withoutAdaptations bool) *Result {
	res := &Result{Report: rep}
	if res.Skipped = skipReason(rep); res.Skipped != "" {
		return res
	}
	c := &checker{p: p, fset: fset, rep: rep, warn: fam == build.GCC, res: res, lines: map[string][]string{}}
	c.ix = buildIndex(p, fset)
	c.hdrs = headerNames(rep.Set)
	c.run()
	if !withoutAdaptations {
		c.adaptations()
	}
	sortDiags(res.Diags)
	return res
}

// sortDiags puts the diagnostics in source order.
func sortDiags(diags []diag.Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i].Pos, diags[j].Pos
		if a.Filename != b.Filename {
			return a.Filename < b.Filename
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}

// skipReason returns why the report cannot be checked, or "". The checks need the probe results,
// the extracted declarations, and headers that compile.
func skipReason(rep *typedinterop.Report) string {
	switch {
	case rep == nil || rep.Table == nil || rep.Set == nil:
		return "no probe results"
	case rep.Mapper == nil || len(rep.Values) != len(rep.Table.Results):
		return "the declarations were not extracted"
	case len(rep.IncludeErrors) > 0:
		return fmt.Sprintf("a header cannot be included (%s): the compiler decides", rep.IncludeErrors[0].Msg)
	case len(rep.Table.HeaderErrors) > 0:
		d := rep.Table.HeaderErrors[0]
		return fmt.Sprintf("a header has an error (%s: %s): the compiler decides", d.File, d.Msg)
	}
	return ""
}

// headerNames maps the first name of each C path of a header package to the
// package (area to shapes for shapes.area), when it is unambiguous.
func headerNames(set *collect.Set) map[string]string {
	out := map[string]string{}
	bad := map[string]bool{}
	for _, r := range set.Refs {
		collect.Walk(r.Expr, func(e collect.Expr) bool {
			if pa, ok := e.(*collect.Path); ok && pa.Ns == "" && pa.Hdr != "" && len(pa.Parts) > 0 {
				n := pa.Parts[0]
				if h, seen := out[n]; seen && h != pa.Hdr {
					bad[n] = true
				}
				out[n] = pa.Hdr
			}
			return true
		})
	}
	for n := range bad {
		delete(out, n)
	}
	return out
}

type checker struct {
	p    *check.Package
	fset *token.FileSet
	rep  *typedinterop.Report
	ix   *astIndex
	hdrs map[string]string
	warn bool
	res  *Result

	// failed holds the keys of the references whose probe printed an error (also
	// the ones that are not reported: an error that follows from another one
	// is not a second error).
	failed map[string]bool

	lines   map[string][]string // source lines by file
	pl      []string            // the lines of the probe file
	typeHdr map[string]string   // see qualify
}

// run goes over the references.
func (c *checker) run() {
	set, tab := c.rep.Set, c.rep.Table
	c.failed = map[string]bool{}
	for _, r := range set.Refs {
		res := tab.Result(r.ID)
		if res != nil && res.Skipped == "" && (!res.OK || res.Recovered != "") {
			c.failed[r.Key] = true
		}
	}
	c.res.Stats.Refs = len(set.Refs)
	for _, r := range set.Refs {
		res := tab.Result(r.ID)
		if res == nil || res.Skipped != "" {
			continue
		}
		c.res.Stats.Probed++
		c.res.Stats.Sites += len(r.Sites)
		if res.OK && res.Recovered != "" {
			// The compiler printed an error and kept a typed expression: a failure.
			c.res.Stats.Failed++
			fres := *res
			fres.OK, fres.Error, fres.ErrorAt, fres.Notes = false, res.Recovered, res.RecoveredAt, res.RecoveredNotes
			c.checkFailed(r, &fres)
		} else if res.OK {
			c.res.Stats.Compiled++
			c.checkOK(r, res, c.rep.Values[r.ID])
		} else {
			c.res.Stats.Failed++
			c.checkFailed(r, res)
		}
	}
}

// pos returns the position of a site as go prints it (the file is absolute,
// as the loader's file set has it).
func (c *checker) pos(st collect.Site) token.Position {
	file := st.File
	if !filepath.IsAbs(file) {
		file = filepath.Join(c.rep.Set.Dir, file)
	}
	return token.Position{Filename: file, Line: st.Line, Column: st.Col}
}

// silent records an expression that is left to the C++ compiler.
func (c *checker) silent(st collect.Site, ref *collect.Ref, why, msg string) {
	c.res.Silent = append(c.res.Silent, Silent{Pos: c.pos(st), Key: ref.Key, Why: why, Msg: msg})
}

// report adds an error (a warning when the build uses g++).
func (c *checker) report(pos token.Position, msg string, related []diag.Related) {
	d := diag.Diagnostic{Pos: pos, Msg: msg, Related: related}
	if c.warn {
		d.Severity = diag.Warning
		d.Related = append(slices.Clone(related), diag.Related{Msg: "this is only a warning: the probe is clang++, and the build uses g++, which may accept it"})
	}
	c.res.Diags = append(c.res.Diags, d)
}

// srcLine returns line n (1-based) of a file, or "".
func (c *checker) srcLine(file string, n int) string {
	ls, ok := c.lines[file]
	if !ok {
		ls = readLines(file)
		c.lines[file] = ls
	}
	if n < 1 || n > len(ls) {
		return ""
	}
	return ls[n-1]
}

// valueOf returns the foreign view of the reference with the key, or nil.
func (c *checker) valueOf(key string) *ftype.Value {
	r := c.rep.Set.Ref(key)
	if r == nil || r.ID >= len(c.rep.Values) {
		return nil
	}
	res := c.rep.Table.Result(r.ID)
	if res == nil || !res.OK {
		return nil
	}
	return &c.rep.Values[r.ID]
}
