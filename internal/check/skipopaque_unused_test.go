package check

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// The tests of the opaque pass that is left out for a package whose only errors are about an unused
// import or variable (collector.onlyUnused): a focused check always has such errors, and so has a
// file that is being typed in.

const unusedOnlySrc = `package main

import (
	"fmt"
	"os"
)

func main() {
	unused := 1
	fmt.Println("hi")
}
`

const hardErrorSrc = `package main

import "fmt"

func main() {
	fmt.Println(missing)
}
`

// checkSkippingOpaque checks the one-file program src with SkipOpaque set or not, and returns the
// checked root package and the diagnostics as gocpp check prints them.
func checkSkippingOpaque(t *testing.T, src string, skip bool) (*Package, string) {
	t.Helper()
	dir := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src})
	prog := load.Load(load.Config{
		Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64",
	}, ".")
	cfg := fakeProbeConfig(LastPhase)
	cfg.SkipOpaque = skip
	r := Check(prog, cfg)
	return mainPkg(t, prog, r), errorText(t, prog, r, filepath.Clean(dir), true)
}

func TestSkipOpaqueUnusedOnlyLeavesThePassOut(t *testing.T) {
	p, _ := checkSkippingOpaque(t, unusedOnlySrc, true)

	if !p.typesClean {
		t.Error("the opaque pass ran for a package with only unused import and variable errors")
	}
	if len(p.Opaque.exprs)+len(p.Opaque.typeExprs) != 0 {
		t.Errorf("the empty Opaque has %d entries", len(p.Opaque.exprs)+len(p.Opaque.typeExprs))
	}
}

func TestSkipOpaqueUnusedOnlyKeepsTheDiagnostics(t *testing.T) {
	_, skipped := checkSkippingOpaque(t, unusedOnlySrc, true)
	_, full := checkSkippingOpaque(t, unusedOnlySrc, false)

	if skipped != full {
		t.Errorf("diagnostics differ\nfull:\n%s\nwith SkipOpaque:\n%s", full, skipped)
	}
	if full == "" {
		t.Error("the program has no diagnostics: the test checks nothing")
	}
}

func TestSkipOpaqueRunsForAnotherError(t *testing.T) {
	p, _ := checkSkippingOpaque(t, hardErrorSrc, true)

	if p.typesClean {
		t.Error("the opaque pass was left out for a package with an undefined name")
	}
}

func TestSkipOpaqueRunsAfterAnUnusedAndAnotherError(t *testing.T) {
	p, _ := checkSkippingOpaque(t, unusedOnlySrc+"\nvar x int = \"s\"\n", true)

	if p.typesClean {
		t.Error("the opaque pass was left out for a package with a type mismatch")
	}
}

// unusedWithCSrc has an unused variable and C: the package imports a C header, so a value in it
// can be opaque, and nothing may leave the pass out.
const unusedWithCSrc = `package main

import "<cstdio>"

var Out *cstdio.FILE

func main() {
	unused := 1
	var local *cstdio.FILE = Out
	_ = local
}
`

func opaqueEntries(p *Package) int {
	return len(p.Opaque.exprs) + len(p.Opaque.typeExprs)
}

func TestSkipOpaqueDoesNotApplyToCInterop(t *testing.T) {
	skippedPackage, skipped := checkSkippingOpaque(t, unusedWithCSrc, true)
	fullPackage, full := checkSkippingOpaque(t, unusedWithCSrc, false)

	if skippedPackage.typesClean {
		t.Error("the opaque pass was left out for a package that imports a C header")
	}
	if opaqueEntries(fullPackage) == 0 {
		t.Error("the full check has no opaque entry: the test checks nothing")
	}
	if got, want := opaqueEntries(skippedPackage), opaqueEntries(fullPackage); got != want {
		t.Errorf("%d opaque entries with SkipOpaque, want %d as in the full check", got, want)
	}
	if skipped != full {
		t.Errorf("diagnostics differ\nfull:\n%s\nwith SkipOpaque:\n%s", full, skipped)
	}
	if full == "" {
		t.Error("the program has no diagnostics: the test checks nothing")
	}
}

func TestSkipOpaqueRunsAfterAnUnusedVariableAndAnUnusedLabel(t *testing.T) {
	src := "package main\n\nfunc main() {\n\tunused := 1\nlabel:\n}\n"

	p, skipped := checkSkippingOpaque(t, src, true)
	_, full := checkSkippingOpaque(t, src, false)

	if p.typesClean {
		t.Error("the opaque pass was left out for a package with an unused label")
	}
	if skipped != full {
		t.Errorf("diagnostics differ\nfull:\n%s\nwith SkipOpaque:\n%s", full, skipped)
	}
	if !strings.Contains(full, "label") {
		t.Errorf("no diagnostic about the label:\n%s", full)
	}
}

func TestCheckedBytesCountsDeclarationsOfSkippedFiles(t *testing.T) {
	src := "package p\n\nfunc F() int {\n\treturn 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8\n}\n"
	dir := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "p.gp": src})
	load1 := func(skip bool) *load.Package {
		prog := load.Load(load.Config{
			Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64",
			SkipBodies: func(string) bool { return skip },
		}, ".")
		return prog.Roots[0]
	}

	full := checkedBytes(load1(false))
	skipped := checkedBytes(load1(true))

	if full != len(src) {
		t.Errorf("a file with bodies: %d bytes, want its size %d", full, len(src))
	}
	if skipped != len("func F() int") {
		t.Errorf("a file without bodies: %d bytes, want the declarations only: %d",
			skipped, len("func F() int"))
	}
}
