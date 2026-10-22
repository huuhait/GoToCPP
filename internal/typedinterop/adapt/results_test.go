package adapt

import (
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The result table (ADR-137): which result types are errors in the program.

func callResult(written, canonical string) *probe.Result {
	return &probe.Result{OK: true, Form: probe.FormExpr, Type: canonical, Written: written}
}

func TestResultRowHResultLong(t *testing.T) {
	row := ResultRowOf(callResult("HRESULT", "long"))
	if row == nil || row.Name != RowHResult {
		t.Fatalf("ResultRowOf(HRESULT as long) = %v, want the %s row", row, RowHResult)
	}
}

func TestResultRowHResultInt(t *testing.T) {
	if ResultRowOf(callResult("HRESULT", "int")) == nil {
		t.Fatal("HRESULT as int is not an error result")
	}
}

func TestResultRowLeavesOtherTypedefs(t *testing.T) {
	for _, c := range []struct{ written, canonical string }{
		{"LRESULT", "long"},
		{"NTSTATUS", "long"},
		{"BOOL", "int"},
		{"DWORD", "unsigned long"},
		{"MYRESULT", "long"}, // a typedef of HRESULT: the written name decides
		{"", "long"},         // the declaration wrote the builtin itself
		{"long", "long"},
	} {
		if row := ResultRowOf(callResult(c.written, c.canonical)); row != nil {
			t.Errorf("ResultRowOf(%q as %s) = %s, want no row", c.written, c.canonical, row.Name)
		}
	}
}

func TestResultRowNeedsASignedCodeOfTheRightSize(t *testing.T) {
	for _, canonical := range []string{"unsigned long", "unsigned int", "long long", "short", "double", "void *"} {
		if row := ResultRowOf(callResult("HRESULT", canonical)); row != nil {
			t.Errorf("ResultRowOf(HRESULT as %s) = %s, want no row", canonical, row.Name)
		}
	}
}

func TestResultRowNeedsTheUnqualifiedTypedefName(t *testing.T) {
	for _, written := range []string{"app::HRESULT", "HRESULT<int>", "const HRESULT *"} {
		if row := ResultRowOf(callResult(written, "long")); row != nil {
			t.Errorf("ResultRowOf(%q) = %s, want no row", written, row.Name)
		}
	}
}

func TestResultRowOnlyForACompiledExpression(t *testing.T) {
	failed := callResult("HRESULT", "long")
	failed.OK = false
	if ResultRowOf(failed) != nil {
		t.Error("a probe that failed has a row")
	}
	typeForm := callResult("HRESULT", "long")
	typeForm.Form = probe.FormType
	if ResultRowOf(typeForm) != nil {
		t.Error("a type has a row")
	}
	function := callResult("HRESULT", "long")
	function.Ref = "lvalue"
	if ResultRowOf(function) != nil {
		t.Error("a reference result has a row")
	}
	if ResultRowOf(nil) != nil {
		t.Error("no result has a row")
	}
}

func hresultMapper() *ftype.Mapper {
	decls := &probe.DeclTable{Canon: map[string]string{"HRESULT": "long", "LRESULT": "long"}}
	return ftype.New(decls, ftype.LP64)
}

func TestSigsReturnHResult(t *testing.T) {
	m := hresultMapper()
	if !SigsReturnHResult(m, sigs(t, "", "HRESULT (int *, int)")) {
		t.Error("a function that returns HRESULT is not")
	}
	if !SigsReturnHResult(m, sigs(t, "", "HRESULT (int)", "HRESULT (const char *)")) {
		t.Error("overloads that all return HRESULT are not")
	}
}

func TestSigsReturnHResultNeedsAllOverloads(t *testing.T) {
	m := hresultMapper()
	if SigsReturnHResult(m, sigs(t, "", "HRESULT (int)", "int (const char *)")) {
		t.Error("overloads that differ are an HRESULT")
	}
	if SigsReturnHResult(m, nil) {
		t.Error("no signature is an HRESULT")
	}
}

func TestSigsReturnHResultLeavesOtherTypedefs(t *testing.T) {
	m := hresultMapper()
	if SigsReturnHResult(m, sigs(t, "", "LRESULT (int)")) {
		t.Error("LRESULT is an HRESULT")
	}
	if SigsReturnHResult(m, sigs(t, "", "int (int)")) {
		t.Error("int is an HRESULT")
	}
}
