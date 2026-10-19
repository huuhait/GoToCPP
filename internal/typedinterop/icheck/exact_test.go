package icheck

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// What is an exact mirror of the generated code (exact.go).
func TestInexactExpr(t *testing.T) {
	str := &collect.Stand{Name: "string"}
	slice := &collect.Stand{Name: "slice", Args: []collect.Expr{&collect.GoType{Cxx: "int64_t"}}}
	tests := []struct {
		name  string
		e     collect.Expr
		why   string // a part of the reason, "" for an exact mirror
		stand bool
	}{
		{"call", call(path("", "abs"), lit("1")), "", false},
		{"go variable", call(path("", "abs"), goInt(true)), "", false},
		{"nested", call(path("", "f"), call(path("", "g"), lit("1"))), "", false},
		{"string argument", call(path("", "puts"), &collect.Arg{T: str, LValue: true}), "", true},
		{"string in a template argument", &collect.Inst{X: path("", "vector"), Args: []collect.Expr{str}}, "stand-in of Go's string", false},
		{"string conversion", call(str, lit(`"x"`)), "stand-in of Go's string", false},
		{"slice argument", call(path("", "f"), &collect.Arg{T: slice}), "stand-in of Go's slice", false},
		{"composite", &collect.Composite{Type: path("", "S")}, "composite literal", false},
		{"composite inside a call", call(path("", "f"), &collect.Composite{Type: path("", "S")}), "composite literal", false},
		{"unknown", call(path("", "f"), &collect.Unknown{Why: "a function literal"}), "no C++ spelling", false},
		{"local alias", call(path("", "f"), &collect.DeclRef{Def: &collect.Ref{Key: "x"}}), "alias", false},
		{"tuple element", &collect.TupleElem{X: call(path("", "f")), I: 1}, "tuple", false},
		{"range element", &collect.RangeElem{X: path("", "v")}, "range", false},
		{"slice expression", &collect.SliceX{X: path("", "v")}, "slice expression", false},
		{"decay of a call", &collect.Decay{X: call(path("", "f"), lit("1"))}, "", false},
		{"decay of a composite", &collect.Decay{X: &collect.Composite{Type: path("", "S")}}, "composite literal", false},
		{"binary with a string", &collect.Binary{Op: "+", X: &collect.Arg{T: str, LValue: true}, Y: lit("1")}, "", true},
		{"member of a call", &collect.Select{X: call(path("", "f")), Name: "m"}, "", false},
		{"address", &collect.Addr{X: call(path("", "f"))}, "", false},
	}
	for _, tc := range tests {
		why, stand := inexactExpr(tc.e, false)
		if tc.why == "" && why != "" || tc.why != "" && !strings.Contains(why, tc.why) {
			t.Errorf("%s: reason %q, want %q", tc.name, why, tc.why)
		}
		if why == "" && stand != tc.stand {
			t.Errorf("%s: stand-in %v, want %v", tc.name, stand, tc.stand)
		}
	}
}

// The errors of a probe that uses the stand-in of string that are evidence:
// the conversions of the string (it converts to std::string_view only) and the
// number of arguments.
func TestStandErrorOK(t *testing.T) {
	s := "gocpp_probe_internal_pr::string"
	probeAt := &probe.Diag{File: "gp_probe.cpp", Severity: "error"}
	tests := []struct {
		name  string
		msg   string
		notes []string
		at    *probe.Diag
		want  bool
	}{
		{"no viable conversion", "no viable conversion from '" + s + "' to 'const char *'", nil, probeAt, true},
		{"parameter", "cannot initialize a parameter of type 'const char *' with an lvalue of type '" + s + "'", nil, probeAt, true},
		{"too many", "too many arguments to function call, expected 1, have 2", nil, probeAt, true},
		{"no matching, conversions", "no matching function for call to 'f'",
			[]string{"candidate function not viable: no known conversion from '" + s + "' to 'int' for 1st argument", "candidate function not viable: requires 2 arguments, but 1 was provided"}, probeAt, true},
		{"no matching, a template", "no matching function for call to 'f'",
			[]string{"candidate template ignored: could not match 'basic_ostream<_CharT, _Traits>' against '" + s + "'"}, probeAt, false},
		{"no matching, no notes", "no matching function for call to 'f'", nil, probeAt, false},
		{"operands", "invalid operands to binary expression ('std::ostream' and '" + s + "')", nil, probeAt, false},
		{"in a header", "no viable conversion from '" + s + "' to 'X'", nil, &probe.Diag{File: "/usr/include/c++/x.h", Severity: "error"}, false},
		{"no place", "no viable conversion from '" + s + "' to 'X'", nil, nil, false},
		{"another type", "no viable conversion from 'int' to '" + s + "'", nil, probeAt, false},
	}
	for _, tc := range tests {
		res := &probe.Result{Error: tc.msg, ErrorAt: tc.at}
		for _, n := range tc.notes {
			res.Notes = append(res.Notes, probe.Diag{Severity: "note", Msg: n})
		}
		if got := standErrorOK(res); got != tc.want {
			t.Errorf("%s: standErrorOK = %v, want %v", tc.name, got, tc.want)
		}
	}
}
