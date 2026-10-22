package icheck

import (
	"fmt"
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The tests of this file need no clang: the Go program is checked (that takes a
// C++ compiler for the namespace probe), the collector lists its references, and
// the test writes the compiler's answers by hand. They are the tests of the
// decisions that make an error certain (what is an exact mirror, which uses are
// left to the C++ compiler), with the compiler's messages made up.

type fab struct {
	t    testing.TB
	p    *check.Package
	fset *token.FileSet
	rep  *typedinterop.Report
}

func newFab(t testing.TB, src string) *fab {
	t.Helper()
	p, fset := tsupport.CheckSource(t, src, nil)
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("the program does not check: %v", errs[0])
	}
	set := collect.Collect(p, fset)
	tab := &probe.Table{Results: make([]probe.Result, len(set.Refs))}
	for i, r := range set.Refs {
		tab.Results[i] = probe.Result{ID: i, Key: r.Key, Skipped: "not set by the test"}
	}
	rep := &typedinterop.Report{Set: set, Table: tab, Includes: typedinterop.UnitIncludes(p), Mapper: ftype.New(nil, ftype.LP64)}
	return &fab{t: t, p: p, fset: fset, rep: rep}
}

// ref returns the reference whose key contains key (exactly one).
func (f *fab) ref(key string) *collect.Ref {
	f.t.Helper()
	if r := f.rep.Set.Ref(key); r != nil {
		return r
	}
	var found *collect.Ref
	for _, r := range f.rep.Set.Refs {
		if strings.Contains(r.Key, key) {
			if found != nil {
				f.t.Fatalf("two references contain %q: %s and %s", key, found.Key, r.Key)
			}
			found = r
		}
	}
	if found == nil {
		f.t.Fatalf("no reference contains %q in\n%s", key, f.rep.Set)
	}
	return found
}

// fail makes the probe of the reference fail with a compiler error in the probe
// file.
func (f *fab) fail(key, msg string, notes ...probe.Diag) *probe.Result {
	f.t.Helper()
	r := f.ref(key)
	res := &f.rep.Table.Results[r.ID]
	*res = probe.Result{ID: r.ID, Key: r.Key, Error: msg, Notes: notes,
		ErrorAt: &probe.Diag{File: "gp_probe.cpp", Line: 1, Col: 1, Severity: "error", Msg: msg}}
	return res
}

// ok makes the probe compile with the type typ ("int", "int &", "void").
func (f *fab) ok(key, typ string, form probe.Form) *probe.Result {
	f.t.Helper()
	r := f.ref(key)
	res := &f.rep.Table.Results[r.ID]
	*res = probe.Result{ID: r.ID, Key: r.Key, OK: true, Form: form, Type: typ, Base: typ}
	if form == probe.FormExpr {
		res.Category = probe.CatPRValue
		if strings.HasSuffix(typ, " &&") {
			res.Base, res.Ref, res.Category = strings.TrimSuffix(typ, " &&"), "&&", probe.CatXValue
		} else if strings.HasSuffix(typ, " &") {
			res.Base, res.Ref, res.Category = strings.TrimSuffix(typ, " &"), "&", probe.CatLValue
		}
		res.Const = strings.HasPrefix(res.Base, "const ")
	}
	return res
}

// check runs the checker on the made-up report.
func (f *fab) check(fam build.Family) *Result {
	f.t.Helper()
	rep := f.rep
	rep.Values = make([]ftype.Value, len(rep.Table.Results))
	for i := range rep.Table.Results {
		rep.Values[i] = rep.Mapper.Value(&rep.Table.Results[i])
	}
	return CheckReport(f.p, f.fset, rep, fam)
}

func lines(r *Result) []string {
	var out []string
	for _, d := range r.Diags {
		sev := ""
		if d.Severity == diag.Warning {
			sev = "warning: "
		}
		out = append(out, fmt.Sprintf("%02d:%02d: %s%s", d.Pos.Line, d.Pos.Column, sev, d.Msg))
	}
	return out
}

func wantLines(t *testing.T, r *Result, want ...string) {
	t.Helper()
	got := lines(r)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("diagnostics:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func silentWhy(r *Result) []string {
	var out []string
	for _, s := range r.Silent {
		out = append(out, fmt.Sprintf("%02d: %s", s.Pos.Line, s.Why))
	}
	return out
}

const absFail = "too many arguments to function call, expected single argument '__x', have 2 arguments"

// The same failed expression in a plain function and in the places where the
// generated code is not what the probe wrote: only the plain use is an error.
func TestSiteExemptions(t *testing.T) {
	f := newFab(t, `package main

import "<cstdlib>"

func plain() {
	_ = cstdlib.abs(1, 2)
}

func inDefer() {
	defer cstdlib.abs(1, 2)
}

func inGo() {
	go cstdlib.abs(1, 2)
}

func generic[T any](x T) {
	_ = cstdlib.abs(1, 2)
}

type Box[T any] struct{ v T }

func (b *Box[T]) method() {
	_ = cstdlib.abs(1, 2)
}

func inClosure() {
	func() {
		_ = cstdlib.abs(1, 2)
	}()
}

func main() {}
`)
	f.fail("call(::abs|lit(1),lit(2))", absFail)
	r := f.check(build.Clang)
	wantLines(t, r,
		"06:06: too many arguments in call to cstdlib.abs (have 2, want 1)",
		"29:07: too many arguments in call to cstdlib.abs (have 2, want 1)")
	why := silentWhy(r)
	if len(why) != 4 {
		t.Fatalf("silent: %v", why)
	}
	for i, want := range []string{"inside a defer or go statement", "inside a defer or go statement", "inside a generic function", "inside a generic function"} {
		if !strings.Contains(why[i], want) {
			t.Errorf("silent[%d] = %q, want %q", i, why[i], want)
		}
	}
	if r.Errors() != 2 {
		t.Errorf("%d errors", r.Errors())
	}
}

// With g++ the same errors are warnings: clang's answer is not the build's.
func TestGCCMakesWarnings(t *testing.T) {
	f := newFab(t, `package main

import "<cstdlib>"

func main() {
	_ = cstdlib.abs(1, 2)
}
`)
	f.fail("call(::abs|lit(1),lit(2))", absFail)
	r := f.check(build.GCC)
	wantLines(t, r, "06:06: warning: too many arguments in call to cstdlib.abs (have 2, want 1)")
	if r.Errors() != 0 {
		t.Errorf("%d errors with g++", r.Errors())
	}
	if rel := r.Diags[0].Related; len(rel) != 1 || !strings.Contains(rel[0].Msg, "only a warning") {
		t.Errorf("related: %+v", rel)
	}
}

// A package whose headers do not compile, or cannot be included, is left to the compiler.
func TestHeaderErrorsSkip(t *testing.T) {
	src := "package main\n\nimport \"<cstdlib>\"\n\nfunc main() {\n\t_ = cstdlib.abs(1, 2)\n}\n"
	f := newFab(t, src)
	f.fail("call(::abs|lit(1),lit(2))", absFail)
	f.rep.Table.HeaderErrors = []probe.Diag{{File: "x.h", Line: 3, Severity: "error", Msg: "unknown type name 'foo'"}}
	if r := f.check(build.Clang); r.Skipped == "" || len(r.Diags) != 0 {
		t.Errorf("header error: skipped %q, %d diagnostics", r.Skipped, len(r.Diags))
	}
	f = newFab(t, src)
	f.fail("call(::abs|lit(1),lit(2))", absFail)
	f.rep.IncludeErrors = []interop.IncludeError{{Include: "<nosuch>", Msg: "'nosuch' file not found"}}
	if r := f.check(build.Clang); r.Skipped == "" || len(r.Diags) != 0 {
		t.Errorf("include error: skipped %q, %d diagnostics", r.Skipped, len(r.Diags))
	}
	f = newFab(t, src)
	f.rep.Mapper = nil
	if r := CheckReport(f.p, f.fset, f.rep, build.Clang); r.Skipped == "" {
		t.Errorf("no declarations: not skipped")
	}
}

// The failures that are not evidence: no diagnostic of the compiler, an
// expression that can be read two ways, errors that follow from another one, the
// stand-ins of Go's types, composite literals.
func TestInexactFailures(t *testing.T) {
	f := newFab(t, `package main

import "<cstdlib>"

func main() {
	var s string = "x"
	xs := []int{1}
	_ = cstdlib.abs(1, 2)
	_ = cstdlib.abs(3, 4)
	_ = cstdlib.abs(5, 6)
	_ = cstdlib.abs(xs)
	_ = cstdlib.abs(s)
	_ = cstdlib.abs(s, s)
	_ = cstdlib.div_t{1, 2}
	y := cstdlib.abs(7, 8)
	_ = cstdlib.abs(y)
}
`)
	// No diagnostic (a limit of the probe).
	res := f.fail("call(::abs|lit(1),lit(2))", "the probe's AST is larger than the limit")
	res.ErrorAt = nil
	// Both readings failed.
	res = f.fail("call(::abs|lit(3),lit(4))", absFail)
	res.AltError = "another error"
	// A slice: the stand-in is not the runtime's type.
	f.fail("call(::abs|arg&(stand:slice(", "no viable conversion from 'gocpp_probe_internal_pr::slice<long>' to 'int'")
	// A string: only its plain conversions count.
	f.fail("call(::abs|arg&(stand:string()))", "no viable conversion from 'gocpp_probe_internal_pr::string' to 'int'")
	f.fail("call(::abs|arg&(stand:string()),arg&(stand:string()))", "invalid operands to binary expression ('gocpp_probe_internal_pr::string' and 'int')")
	f.fail("lit{", "no matching constructor for initialization of 'div_t'")
	// A local variable whose declaration failed, and its use.
	f.fail("call(::abs|lit(5),lit(6))", absFail)
	f.fail("call(::abs|lit(7),lit(8))", absFail)
	f.fail("decay(call(::abs|lit(7),lit(8)))", absFail)
	f.fail("call(::abs|arg&(decay(call(::abs|lit(7),lit(8)))))", "depends on a local variable whose C/C++ type could not be probed")
	r := f.check(build.Clang)
	wantLines(t, r,
		"10:06: too many arguments in call to cstdlib.abs (have 2, want 1)", // 5, 6
		"12:06: cannot use an argument (value of type string) as C.int value in argument to cstdlib.abs",
		"15:07: too many arguments in call to cstdlib.abs (have 2, want 1)", // 7, 8: the cause
	)
	got := strings.Join(silentWhy(r), "\n")
	for _, want := range []string{"printed no diagnostic", "read two ways", "stand-in of Go's slice", "not one of its conversions", "composite literal", "declaration of a local variable"} {
		if !strings.Contains(got, want) {
			t.Errorf("silent reasons do not include %q:\n%s", want, got)
		}
	}
}

// The rules for a probe that compiled.
func TestTypedRules(t *testing.T) {
	f := newFab(t, `package main

import "<cstdlib>"

func take(p *int) {}

func get() *int {
	return cstdlib.abs(1)
}

func main() {
	var q *int = cstdlib.abs(2)
	var n int = cstdlib.abs(3)
	var f float64 = cstdlib.abs(4)
	take(cstdlib.abs(5))
	x := cstdlib.abs(6)
	cstdlib.abs(7)
	cstdlib.abs(8) = 9
	cstdlib.abs(10)++
	n = cstdlib.abs(11)
	_, _, _, _, _ = q, n, f, x, get
}
`)
	for key, typ := range map[string]string{
		"lit(1))": "int", "lit(2))": "int", "lit(3))": "int", "lit(4))": "int", "lit(5))": "int",
		"lit(7))": "int", "lit(8))": "int", "lit(10))": "int", "lit(11))": "int", "lit(6))": "void",
	} {
		f.ok("call(::abs|"+key, typ, probe.FormExpr)
	}
	f.ok("decay(call(::abs|lit(6)))", "void", probe.FormType)
	r := f.check(build.Clang)
	wantLines(t, r,
		"08:09: cannot use cstdlib.abs(1) (value of type C.int) as *int value in return statement",
		"12:15: cannot use cstdlib.abs(2) (value of type C.int) as *int value in variable declaration",
		"15:07: cannot use cstdlib.abs(5) (value of type C.int) as *int value in argument to take",
		"16:07: cstdlib.abs(6) (no value) used as value",
		"18:02: cannot assign to cstdlib.abs(8) (neither addressable nor a map index expression)",
		"19:02: cannot assign to cstdlib.abs(10) (neither addressable nor a map index expression)",
	)
}

// clang recovers from some errors and still gives a type: the error counts.
func TestRecoveredErrors(t *testing.T) {
	f := newFab(t, `package main

import "<cstdlib>"

func main() {
	_ = cstdlib.abs(1, 2)
}
`)
	res := f.ok("call(::abs|lit(1),lit(2))", "int", probe.FormExpr)
	res.Recovered = absFail
	res.RecoveredAt = &probe.Diag{File: "gp_probe.cpp", Line: 1, Col: 1, Severity: "error", Msg: absFail}
	r := f.check(build.Clang)
	wantLines(t, r, "06:06: too many arguments in call to cstdlib.abs (have 2, want 1)")
}

func TestCrossTargetSkips(t *testing.T) {
	p, fset := tsupport.CheckSource(t, "package main\n\nimport \"<cstdlib>\"\n\nfunc main() { _ = cstdlib.abs(1) }\n", nil)
	r, err := Check(t.Context(), p, fset, Options{Cross: true})
	if err != nil || r.Skipped == "" || len(r.Diags) != 0 {
		t.Errorf("cross: %+v, %v", r, err)
	}
	p, fset = tsupport.CheckSource(t, "package main\n\nfunc main() {}\n", nil)
	r, err = Check(t.Context(), p, fset, Options{})
	if err != nil || !strings.Contains(r.Skipped, "no C/C++") {
		t.Errorf("no headers: %+v, %v", r, err)
	}
}

// A class converts to a number only through a conversion operator: Compat says
// Unknown (a class may have one), and the declaration decides. A class without
// one (in itself and its bases) is an error where a number or a pointer is wanted;
// one with one, or a class whose bases are not known, is not.
func TestRecordConversions(t *testing.T) {
	f := newFab(t, `package main

import "<cstdlib>"

func takeF(f float64) {}

func takeP(p *int) {}

func main() {
	takeF(cstdlib.abs(1))
	takeF(cstdlib.abs(2))
	takeF(cstdlib.abs(3))
	takeF(cstdlib.abs(4))
	takeF(cstdlib.abs(5))
	takeP(cstdlib.abs(6))
}
`)
	tt := func(s string) probe.TypeText { return probe.TypeText{Text: s, T: ctype.MustParse(s)} }
	conv := probe.Method{Name: "operator double", Kind: probe.KindConversion, Sig: tt("double () const"), Access: probe.Public}
	f.rep.Mapper.Add(&probe.DeclTable{Records: map[string]*probe.Record{
		"Meters":  {Name: "Meters", Tag: "struct", Complete: true, Methods: []probe.Method{conv}},
		"Plain":   {Name: "Plain", Tag: "struct", Complete: true, Fields: []probe.Field{{Name: "v", Type: tt("int"), Access: probe.Public}}},
		"Derived": {Name: "Derived", Tag: "struct", Complete: true, Bases: []probe.Base{{Type: tt("Meters"), Access: probe.Public}}},
		"Unknown": {Name: "Unknown", Tag: "struct", Complete: true, Bases: []probe.Base{{Type: tt("Missing"), Access: probe.Public}}},
		"Tmpl":    {Name: "Tmpl", Tag: "struct", Complete: true, Methods: []probe.Method{{Name: "operator T", Kind: probe.KindOperator, Template: true, Access: probe.Public}}},
	}})
	for key, typ := range map[string]string{"lit(1))": "Meters", "lit(2))": "Plain", "lit(3))": "Derived", "lit(4))": "Unknown", "lit(5))": "Tmpl", "lit(6))": "Plain"} {
		f.ok("call(::abs|"+key, typ, probe.FormExpr)
	}
	r := f.check(build.Clang)
	wantLines(t, r,
		"11:08: cannot use cstdlib.abs(2) (value of type Plain) as float64 value in argument to takeF",
		"15:08: cannot use cstdlib.abs(6) (value of type Plain) as *int value in argument to takeP",
	)
}

func TestSiteWhyWithoutANode(t *testing.T) {
	if siteWhy(nil) == "" {
		t.Error("a use that is not in the syntax tree must be left to the compiler")
	}
	if siteWhy(&astInfo{}) != "" {
		t.Error("a plain use is a mirror")
	}
}

// A recovered error is a failure for the uses that depend on it too: one report,
// at the cause.
func TestRecoveredErrorIsTheCause(t *testing.T) {
	f := newFab(t, `package main

import "<cstdlib>"

func main() {
	x := cstdlib.abs(1, 2)
	_ = cstdlib.abs(x)
}
`)
	res := f.ok("call(::abs|lit(1),lit(2))", "int", probe.FormExpr)
	res.Recovered = absFail
	res.RecoveredAt = &probe.Diag{File: "gp_probe.cpp", Line: 1, Col: 1, Severity: "error", Msg: absFail}
	f.ok("decay(call(::abs|lit(1),lit(2)))", "int", probe.FormType)
	f.fail("call(::abs|arg&(decay(", "no matching function for call to 'abs'")
	r := f.check(build.Clang)
	wantLines(t, r, "06:07: too many arguments in call to cstdlib.abs (have 2, want 1)")
}

// What the rules on compiled probes do not apply to: a void call in a function
// without results, an explicit conversion, a class on the left of an assignment, a
// probe that is not a mirror.
func TestTypedRulesLeaveThese(t *testing.T) {
	f := newFab(t, `package main

import "<cstdlib>"

func nores() {
	return cstdlib.abs(1)
}

func main() {
	var xs []int
	_ = int(cstdlib.abs(2))
	cstdlib.abs(3) = cstdlib.abs(4)
	var q *int = cstdlib.abs(xs)
	_, _ = q, nores
}
`)
	f.ok("call(::abs|lit(1))", "void", probe.FormExpr)
	f.ok("call(::abs|lit(2))", "int *", probe.FormExpr)
	f.ok("call(::abs|lit(3))", "Plain", probe.FormExpr)
	f.ok("call(::abs|lit(4))", "Plain", probe.FormExpr)
	f.ok("call(::abs|arg&(stand:slice(", "int", probe.FormExpr)
	r := f.check(build.Clang)
	wantLines(t, r)
}
