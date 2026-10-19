package vet

import (
	"reflect"
	"sync"

	"golang.org/x/tools/go/analysis"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// pkgInputs is what the analyzers about Phase 10 forms need of a package besides the syntax trees
// and types: the front-end rewrites `!` and macros into plain Go before go/types (ADR-037,
// ADR-075), so the trees no longer say where the `!` were or which functions were macros.
type pkgInputs struct {
	path   string
	must   *check.MustInfo           // nil for a package with no `!` form
	macros map[string]*load.MacroSig // the macros the package declares, by name
}

// inputsOf maps the types.Package of a unit to its inputs while Run is running.
var inputsOf sync.Map

// inputsAnalyzer gives the analyzers that Require it the inputs of the package. It reports nothing.
// The inputs come from a table that Run fills, because the driver of this package does not take
// them from a packages.Package, as x/tools' checker would.
var inputsAnalyzer = &analysis.Analyzer{
	Name:             "gocppinputs",
	Doc:              "gives the analyzers of gocpp what the front-end knows besides the syntax trees (not run by itself)",
	RunDespiteErrors: true,
	ResultType:       reflect.TypeOf((*pkgInputs)(nil)),
	Run: func(pass *analysis.Pass) (any, error) {
		in, _ := inputsOf.Load(pass.Pkg)
		if in == nil {
			return &pkgInputs{}, nil
		}
		return in.(*pkgInputs), nil
	},
}

func inputs(pass *analysis.Pass) *pkgInputs {
	return pass.ResultOf[inputsAnalyzer].(*pkgInputs)
}
