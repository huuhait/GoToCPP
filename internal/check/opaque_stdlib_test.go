package check

import (
	"go/ast"
	"go/build"
	goimporter "go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/testgo"
)

// TestOpaqueGoStdlib runs the opaque pass over the packages of Go's own
// standard library (with their in-package tests), checked by the stock
// go/types: pure Go code that go/types accepts must have no expression
// that is neither typed nor opaque. This shows that the expressions the
// pass doesn't label (see Opaque) are all the ones that go/types leaves out
// in valid code. A package with a go/types error (a few need cgo or a
// special setup) is skipped. It takes a few seconds, so -short skips it.
func TestOpaqueGoStdlib(t *testing.T) {
	if testing.Short() {
		t.Skip("-short: the whole Go standard library takes a few seconds")
	}
	ctx := build.Default
	// The go/types of this build reads the standard library of the same Go: another version may
	// use syntax that it refuses (the GOROOT that is built into a test binary may be that of
	// the machine that made it, and is not there on the machine that runs it).
	version, err := os.ReadFile(filepath.Join(ctx.GOROOT, "VERSION"))
	if err != nil {
		t.Skipf("no Go library at %s: %v", ctx.GOROOT, err)
	}
	testgo.SkipUnlessModuleVersion(t, "the Go library", strings.SplitN(string(version), "\n", 2)[0])
	src := filepath.Join(ctx.GOROOT, "src")
	fset := token.NewFileSet()
	imp := goimporter.ForCompiler(fset, "source", nil).(types.ImporterFrom)
	checked, labeled := 0, 0
	err = filepath.WalkDir(src, func(dir string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		switch base := filepath.Base(dir); {
		case base == "testdata" || base == "vendor" || dir == filepath.Join(src, "cmd"):
			return filepath.SkipDir
		case strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_"):
			return filepath.SkipDir
		}
		bp, err := ctx.ImportDir(dir, 0)
		if err != nil || bp.Name == "main" {
			return nil // no Go files for this system, or a command
		}
		for _, names := range [][]string{bp.GoFiles, append(append([]string(nil), bp.GoFiles...), bp.TestGoFiles...)} {
			files := parseFiles(fset, dir, names)
			if files == nil {
				continue
			}
			info := &types.Info{
				Types:      make(map[ast.Expr]types.TypeAndValue),
				Defs:       make(map[*ast.Ident]types.Object),
				Uses:       make(map[*ast.Ident]types.Object),
				Implicits:  make(map[ast.Node]types.Object),
				Selections: make(map[*ast.SelectorExpr]*types.Selection),
				Scopes:     make(map[ast.Node]*types.Scope),
				Instances:  make(map[*ast.Ident]types.Instance),
			}
			failed := false
			conf := types.Config{Importer: imp, Error: func(error) { failed = true }}
			pkg, _ := conf.Check(bp.ImportPath, fset, files, info)
			if failed {
				continue
			}
			checked++
			op := newOpaquePass(pkg, info, files)
			op.label(files)
			for _, e := range op.bad {
				t.Errorf("%s: %s (%T) is neither typed nor opaque", fset.Position(e.Pos()), types.ExprString(e), e)
			}
			for _, f := range files {
				ast.Inspect(f, func(n ast.Node) bool {
					if e, ok := n.(ast.Expr); ok && op.labeled(e) {
						labeled++
					}
					return true
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Go 1.25 has about 650 such packages (with and without their tests)
	// that go/types checks without errors here.
	if checked < 400 {
		t.Errorf("checked only %d packages of %s", checked, src)
	}
	t.Logf("%d packages, %d labeled expressions", checked, labeled)
}

// parseFiles parses the files of dir with the stock go/parser, or returns
// nil if there are none or one has a syntax error.
func parseFiles(fset *token.FileSet, dir string, names []string) []*ast.File {
	var files []*ast.File
	for _, name := range names {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		files = append(files, f)
	}
	return files
}
