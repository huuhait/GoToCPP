package gotest

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// A Func is a test, benchmark, fuzz target or example of a test binary.
type Func struct {
	Pkg  string // the alias of its package in the main package: "_test" or "_xtest"
	Name string

	// Output and Unordered are the expected output of an example, from its "// Output:" or
	// "// Unordered output:" comment.
	Output    string
	Unordered bool
}

// Found is what the test files of a package declare.
type Found struct {
	Tests      []Func
	Benchmarks []Func
	Fuzz       []Func
	Examples   []Func // only the examples with an output comment (the others are compiled, not run)

	// TestMain is the TestMain(m *testing.M) function, or nil.
	TestMain *Func

	// ImportTest and ImportXTest say that the main package imports the package under test and
	// its external test package; NeedTest and NeedXTest, that it uses them.
	ImportTest, ImportXTest bool
	NeedTest, NeedXTest     bool
}

// HasTestFiles reports whether the package under test p or its external test package xt (which
// may be nil) have a _test.gp file: go test says "[no test files]" when neither has.
func HasTestFiles(p, xt *load.Package) bool {
	if xt != nil && len(xt.Files) > 0 {
		return true
	}
	for _, f := range p.Files {
		if f.Test {
			return true
		}
	}
	return false
}

// Scan finds the tests of the package p (its test variant) and of its external test package xt, or
// nil. The errors are those of go test ("wrong signature for TestX, must be: func TestX(t
// *testing.T)", "multiple definitions of TestMain").
func Scan(fset *token.FileSet, p, xt *load.Package) (*Found, []diag.Diagnostic) {
	f := &Found{}
	var errs []diag.Diagnostic
	scan := func(pkg *load.Package, alias string, files []*load.File) {
		// The functions of the package, for the fuzz targets that name their function (rewriteFuzz).
		decls := make(map[string]*ast.FuncDecl)
		for _, file := range files {
			for _, d := range file.AST.Decls {
				if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
					decls[fn.Name.Name] = fn
				}
			}
		}
		for _, file := range files {
			if !file.Test {
				continue
			}
			seen := false
			errs = append(errs, f.scanFile(fset, alias, file.AST, decls, &seen)...)
			if seen {
				if alias == "_test" {
					f.NeedTest = true
				} else {
					f.NeedXTest = true
				}
			}
		}
	}
	if p != nil {
		scan(p, "_test", p.Files)
		f.ImportTest = true
	}
	if xt != nil {
		scan(xt, "_xtest", xt.Files)
		f.ImportXTest = true
	}
	return f, errs
}

func (f *Found) scanFile(fset *token.FileSet, alias string, file *ast.File, decls map[string]*ast.FuncDecl, seen *bool) []diag.Diagnostic {
	var errs []diag.Diagnostic
	fail := func(pos token.Pos, format string, args ...any) {
		errs = append(errs, diag.Diagnostic{Pos: fset.Position(pos), Msg: fmt.Sprintf(format, args...)})
	}
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		name := fn.Name.Name
		switch {
		case name == "TestMain":
			if isTestFunc(fn, "T") {
				f.Tests = append(f.Tests, Func{Pkg: alias, Name: name})
				*seen = true
				continue
			}
			if msg := checkTestFunc(fn, "M"); msg != "" {
				fail(fn.Pos(), "%s", msg)
				continue
			}
			if f.TestMain != nil {
				fail(fn.Pos(), "multiple definitions of TestMain")
				continue
			}
			f.TestMain = &Func{Pkg: alias, Name: name}
			*seen = true
		case IsTest(name, "Test"):
			if msg := checkTestFunc(fn, "T"); msg != "" {
				fail(fn.Pos(), "%s", msg)
				continue
			}
			f.Tests = append(f.Tests, Func{Pkg: alias, Name: name})
			*seen = true
		case IsTest(name, "Benchmark"):
			if msg := checkTestFunc(fn, "B"); msg != "" {
				fail(fn.Pos(), "%s", msg)
				continue
			}
			f.Benchmarks = append(f.Benchmarks, Func{Pkg: alias, Name: name})
			*seen = true
		case IsTest(name, "Fuzz"):
			if msg := checkTestFunc(fn, "F"); msg != "" {
				fail(fn.Pos(), "%s", msg)
				continue
			}
			f.Fuzz = append(f.Fuzz, Func{Pkg: alias, Name: name})
			rewriteFuzz(fn, decls)
			*seen = true
		case IsTest(name, "Example"):
			if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 || fn.Type.Results != nil && len(fn.Type.Results.List) > 0 ||
				fn.Type.TypeParams != nil && fn.Type.TypeParams.NumFields() > 0 || fn.Body == nil {
				continue // not an example (go doc's rule)
			}
			*seen = true // the file is imported whether or not the example runs
			out, unordered, ok := exampleOutput(fn.Body, file.Comments)
			if !ok {
				continue // examples without an output comment are not run
			}
			f.Examples = append(f.Examples, Func{Pkg: alias, Name: name, Output: out, Unordered: unordered})
		}
	}
	return errs
}

// IsTest reports whether name is a test, benchmark, fuzz target or example name with the prefix
// (Go's isTest): the prefix alone, or followed by something that does not start with a lower-case
// letter.
func IsTest(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(r)
}

// isTestFunc reports whether fn has the signature func(*testing.arg) (arg is "T", "B", "M" or
// "F"). The type is checked by its syntax only: *arg or *pkg.arg.
func isTestFunc(fn *ast.FuncDecl, arg string) bool {
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 ||
		fn.Type.Params == nil || len(fn.Type.Params.List) != 1 || len(fn.Type.Params.List[0].Names) > 1 {
		return false
	}
	ptr, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	if name, ok := ptr.X.(*ast.Ident); ok && name.Name == arg {
		return true
	}
	if sel, ok := ptr.X.(*ast.SelectorExpr); ok && sel.Sel.Name == arg {
		return true
	}
	return false
}

// checkTestFunc returns go test's error for a function whose name says it is a test but whose
// signature is not, or "".
func checkTestFunc(fn *ast.FuncDecl, arg string) string {
	why := ""
	if !isTestFunc(fn, arg) {
		why = fmt.Sprintf("must be: func %s(%s *testing.%s)", fn.Name.Name, strings.ToLower(arg), arg)
	}
	if fn.Type.TypeParams != nil && fn.Type.TypeParams.NumFields() > 0 {
		why = "test functions cannot have type parameters"
	}
	if why == "" {
		return ""
	}
	return fmt.Sprintf("wrong signature for %s, %s", fn.Name.Name, why)
}

var outputPrefix = regexp.MustCompile(`(?i)^[[:space:]]*(unordered )?output:`)

// exampleOutput returns the expected output of an example from the last comment in its body that
// starts with "Output:" or "Unordered output:" (go/doc's rule), and whether there is one.
func exampleOutput(b *ast.BlockStmt, comments []*ast.CommentGroup) (output string, unordered, ok bool) {
	var last *ast.CommentGroup
	pos, end := b.Pos(), b.End()
	for _, c := range comments {
		if c.Pos() < pos {
			continue
		}
		if c.End() > end {
			break
		}
		last = c
	}
	if last == nil {
		return "", false, false
	}
	text := last.Text()
	loc := outputPrefix.FindStringSubmatchIndex(text)
	if loc == nil {
		return "", false, false
	}
	if loc[2] != -1 {
		unordered = true
	}
	text = text[loc[1]:]
	// Strip zero or more spaces followed by \n or a single space.
	text = strings.TrimLeft(text, " ")
	if len(text) > 0 && text[0] == '\n' {
		text = text[1:]
	}
	return text, unordered, true
}
