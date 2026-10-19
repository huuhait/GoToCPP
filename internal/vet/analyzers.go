package vet

import (
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/waitgroup"
)

// An Analyzer is one check of gocpp vet: an analysis.Analyzer of x/tools
// (the standard vet checks, which Go's own vet runs too) or one of gocpp's
// own.
type Analyzer struct {
	*analysis.Analyzer

	// Gocpp reports that the analyzer is gocpp's own: a trap of the
	// language (reference cycles, C++ reserved names), not a Go mistake.
	Gocpp bool

	// Opaque reports that the analyzer also runs in a package with C/C++
	// names (see illTyped), although x/tools doesn't declare it
	// RunDespiteErrors: it doesn't crash on invalid types, and it accepts
	// a value of an invalid type (a C++ value) wherever it checks types.
	// TestOpaqueCorpus and TestOpaqueNoFalsePositives are the evidence.
	Opaque bool

	// Placeholder reports that the analyzer is registered but does nothing
	// yet: it waits for the phase that builds what it checks (panicbang
	// waits for Phase 10).
	Placeholder bool
}

// Analyzers lists every analyzer of gocpp vet, in the order that -list
// shows them. The names are the ones of go vet's flags (-printf=false),
// and one flag per analyzer is what the command has.
var Analyzers = []*Analyzer{
	{Analyzer: appends.Analyzer, Opaque: true},
	{Analyzer: assign.Analyzer, Opaque: true},
	{Analyzer: atomic.Analyzer},
	{Analyzer: bools.Analyzer, Opaque: true},
	{Analyzer: composite.Analyzer},
	{Analyzer: copylocksAnalyzer},
	{Analyzer: defers.Analyzer, Opaque: true},
	{Analyzer: errorsas.Analyzer, Opaque: true},
	{Analyzer: ifaceassert.Analyzer, Opaque: true},
	{Analyzer: lostcancel.Analyzer, Opaque: true},
	{Analyzer: nilfunc.Analyzer, Opaque: true},
	{Analyzer: nilnessGuarded, Opaque: true},
	{Analyzer: printfGuarded, Opaque: true},
	{Analyzer: shift.Analyzer, Opaque: true},
	{Analyzer: stdmethods.Analyzer, Opaque: true},
	{Analyzer: stringintconv.Analyzer, Opaque: true},
	{Analyzer: structtag.Analyzer},
	{Analyzer: testsGuarded, Opaque: true},
	{Analyzer: timeformat.Analyzer, Opaque: true},
	{Analyzer: unreachable.Analyzer, Opaque: true},
	{Analyzer: unsafeptr.Analyzer, Opaque: true},
	{Analyzer: unusedresult.Analyzer, Opaque: true},
	{Analyzer: waitgroup.Analyzer, Opaque: true},
	{Analyzer: cycleAnalyzer, Gocpp: true},
	{Analyzer: reservedAnalyzer, Gocpp: true},
	{Analyzer: panicbangAnalyzer, Gocpp: true, Placeholder: true},
}

// Lookup returns the analyzer with the name, or nil.
func Lookup(name string) *Analyzer {
	for _, a := range Analyzers {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// Select returns the analyzers to run, in the order of Analyzers: like go
// vet, if some analyzers are named in on, only those; else all but the
// ones in off. A name that is no analyzer is an error of the caller.
func Select(on, off map[string]bool) []*Analyzer {
	var out []*Analyzer
	for _, a := range Analyzers {
		switch {
		case len(on) > 0:
			if on[a.Name] {
				out = append(out, a)
			}
		case !off[a.Name]:
			out = append(out, a)
		}
	}
	return out
}

// reservedAnalyzer reports names that C++ reserves for the implementation.
// gocpp keeps such a name as written (docs/spec/cpp-mapping.md: GCC and
// Clang accept them), but a header may define a macro of that name, so a
// program is safer without them.
var reservedAnalyzer = &analysis.Analyzer{
	Name:             "reserved",
	Doc:              "report names that C++ reserves (_X, a__b): gocpp keeps them as written, and a header may use them",
	RunDespiteErrors: true,
	Run: func(pass *analysis.Pass) (any, error) {
		for id, obj := range pass.TypesInfo.Defs {
			if obj == nil || id.Name == "_" {
				continue
			}
			if why := reservedWhy(id.Name); why != "" {
				pass.Report(analysis.Diagnostic{
					Pos:      id.Pos(),
					End:      id.End(),
					Category: "reserved",
					Message:  "the name " + id.Name + " is reserved in C++ (" + why + "): gocpp writes it as it is, but a C++ header may define a macro with this name; choose another name",
				})
			}
		}
		return nil, nil
	},
}

// reservedWhy returns why C++ reserves the name, or "".
func reservedWhy(name string) string {
	if strings.Contains(name, "__") {
		return "it contains __"
	}
	if len(name) > 1 && name[0] == '_' && unicode.IsUpper(rune(name[1])) {
		return "it starts with _ and a capital letter"
	}
	return ""
}

// panicbangAnalyzer is the place of the check for F!(...) that would panic
// inside a library package (libraries should return their errors). The
// check needs the must functions and must calls of Phase 10, which are not
// built yet: today the analyzer is registered, can be switched off by name,
// and reports nothing. The driver doesn't change when Phase 10 fills it in.
var panicbangAnalyzer = &analysis.Analyzer{
	Name:             "panicbang",
	Doc:              "report F!(...) that would panic inside a library package (placeholder: waits for Phase 10)",
	RunDespiteErrors: true,
	Run:              func(*analysis.Pass) (any, error) { return nil, nil },
}
