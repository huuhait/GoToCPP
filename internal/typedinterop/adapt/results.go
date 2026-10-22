package adapt

import (
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The result table (ADR-137): what a call of a C/C++ function is in the Go program, decided from the
// type that the declaration wrote for its result. The argument table (Table, adapt.go) is over the
// parameters; this one is over the result, and the two are independent: a call is lowered with the
// rows of its arguments and, around that, the row of its result.

// The rows of the result table (ResultRow.Name).
const (
	// RowHResult: the typedef HRESULT is Go's error. nil is SUCCEEDED (the code is not negative), else
	// a runtime.HResultError that keeps the code (gp::hresult_error, runtime/include/gocpp/hresult.hpp).
	RowHResult = "hresult"
)

// A ResultRow is one row of the result table.
type ResultRow struct {
	Name string
	// matches says whether the row adapts the result of the call whose probe result is r (the type
	// of the call as the compiler says it, and as the declaration wrote it).
	matches func(r *probe.Result) bool
}

// ResultTable is the table of result adaptations. A result has at most one row.
var ResultTable = []*ResultRow{
	{Name: RowHResult, matches: hresultResult},
}

// ResultRowOf returns the row that adapts the result of the call that the probe result r is the
// answer for, or nil: the result stays the C++ type.
func ResultRowOf(r *probe.Result) *ResultRow {
	if r == nil || !r.OK || r.Form != probe.FormExpr {
		return nil
	}
	for _, row := range ResultTable {
		if row.matches(r) {
			return row
		}
	}
	return nil
}

// hresultResult reports whether the call's type is the typedef HRESULT: the type that the
// declaration wrote is exactly the name HRESULT (the compiler's sugar of the call, probe.Result.Written),
// and the compiler's own type is a signed integer that holds a 32-bit code (int, or long: LONG).
// A typedef of HRESULT, an int that is not spelled HRESULT, LRESULT, NTSTATUS, BOOL and DWORD are not it.
func hresultResult(r *probe.Result) bool {
	if r.Ref != "" || r.Written == "" {
		return false
	}
	written, err := ctype.Parse(r.Written)
	if err != nil || written.Kind != ctype.Named || written.QualName() != "HRESULT" {
		return false
	}
	if last := written.Path[len(written.Path)-1]; last.HasArgs {
		return false
	}
	canonical, err := ctype.Parse(r.Type)
	if err != nil || canonical.Kind != ctype.Builtin {
		return false
	}
	return canonical.Name == "int" || canonical.Name == "long"
}

// SigsReturnHResult reports whether every overload of the callee returns the typedef HRESULT: the
// declarations say what the call is even when the call itself was not probed (an argument that the
// probe cannot spell, such as &n of a Go number variable, which the argument table adapts).
func SigsReturnHResult(m *ftype.Mapper, sigs []Sig) bool {
	if len(sigs) == 0 {
		return false
	}
	for _, s := range sigs {
		res := m.MapIn(s.T.Elem, s.Scope)
		if res.Kind != ftype.Scalar || res.Alias != "HRESULT" || res.C == nil {
			return false
		}
		if res.C.Kind != ctype.Builtin || res.C.Name != "int" && res.C.Name != "long" {
			return false
		}
	}
	return true
}
