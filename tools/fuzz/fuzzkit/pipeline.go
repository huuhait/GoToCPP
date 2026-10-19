// Package fuzzkit is the shared part of gocpp's fuzzers (Phase 15 step 15.8): the front end to
// back end pipeline for one source text (load, check, lower, emit) without touching the disk,
// the checks that every fuzz target makes on its result, the sampled C++ compile stage, the
// watchdog that turns a hang or a memory blowup into a crash that the fuzzing engine saves, and
// the seed corpus (every .gp file of the repository).
//
// The package is only for the tests and the tools of tools/fuzz: nothing in gocpp imports it.
package fuzzkit

import (
	"go/token"
	"go/types"
	"os"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// The names of the files of a fuzzed program. They are made up: Load reads the program from
// its overlay, so nothing is read from or written to these folders.
const (
	Dir     = "/fuzz/p"
	GenRoot = "/fuzz/gen"
)

// File is the one file of a fuzzed program.
const File = Dir + "/main.gp"

// DefaultPhase is the phase whose gates the fuzzers use (the phase that make ci tests up to:
// Makefile, CI_TEST_PHASE). A program that the checker accepts at this phase must lower and
// compile. GOCPP_FUZZ_PHASE changes it.
const DefaultPhase = 10

// Phase returns the phase of the gates of the fuzzers.
func Phase() int {
	if s := os.Getenv("GOCPP_FUZZ_PHASE"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return DefaultPhase
}

// A Result is what the pipeline made of one source text.
type Result struct {
	Fset *token.FileSet

	// Errors are the error diagnostics of the load and of the check ("file:line:col: message").
	// A program with errors goes no further.
	Errors []string

	// Internal are the messages with "internal error" in them, from any stage: for a program that
	// has no other error, each one is a finding.
	Internal []string

	// Syntax is true when the load found no syntax error; Checked, when every package checked
	// without errors.
	Syntax, Checked bool

	// Main is true when the program is a main package with a function main: what gocpp build
	// takes. Emit is only tried then.
	Main bool

	// HasHeaders is true when a file imports a C or C++ header: the emitter then needs the
	// compiler's probes, which the fuzzers don't run, so such a program is not emitted.
	HasHeaders bool

	// Files are the generated files and EmitErr why there are none (a *cppgen.Error for a
	// construct the emitter does not translate). MainPkg is the folder of the main package in
	// Files ("gpkg/<import path>/").
	Files   []cppgen.File
	EmitErr error
	MainPkg string
}

// Run loads, checks and (if emit is set, and the program is a main package that checks cleanly)
// emits the program src, a main.gp file. It does not recover from panics: a panic is a finding.
func Run(src []byte, emit bool) *Result {
	return RunPhase(src, Phase(), emit)
}

// RunPhase is Run with the gates of the phase.
func RunPhase(src []byte, phase int, emit bool) *Result {
	res := &Result{}
	prog := load.Load(load.Config{
		Dir: Dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64",
		Overlay: map[string][]byte{File: src}, AllErrors: true,
	}, ".")
	res.Fset = prog.Fset
	res.Syntax = true
	res.collect(prog.Errors)
	for _, p := range prog.Packages {
		if p.Errors != nil && p.Errors.HasErrors() {
			res.Syntax = false
		}
	}
	if len(prog.Errors.Diagnostics()) > 0 && prog.Errors.HasErrors() {
		res.Syntax = false
	}
	r := check.Check(prog, check.Config{Phase: phase, GOOS: "linux", GOARCH: "amd64", Partial: true})
	res.Checked = true
	for _, lp := range prog.Packages {
		cp := r.Package(lp)
		if cp == nil {
			res.Checked = false
			continue
		}
		res.collect(cp.Errors)
		if cp.Errors.HasErrors() || !cp.Checked() {
			res.Checked = false
		}
		for _, f := range lp.Files {
			if f.Info != nil && len(f.Info.Includes) > 0 && !lp.Std {
				res.HasHeaders = true
			}
		}
	}
	if len(res.Errors) > 0 {
		res.Checked = false
	}
	if !res.Checked || len(prog.Roots) != 1 || prog.Roots[0].Name != "main" {
		return res
	}
	mainPkg := r.Package(prog.Roots[0])
	if mainPkg == nil || !mainPkg.Checked() {
		return res
	}
	if _, ok := mainPkg.Types.Scope().Lookup("main").(*types.Func); !ok {
		return res
	}
	res.Main = true
	if !emit || res.HasHeaders {
		return res
	}
	// The packages of the program, as the driver (cmd/gocpp, driver.load) chooses them.
	notLowered := func(p *load.Package) bool {
		return p.Std && (p.ImportPath == "fmt" || p.ImportPath == "weak" || p.IsStub())
	}
	reach := map[*load.Package]bool{}
	var visit func(p *load.Package)
	visit = func(p *load.Package) {
		if reach[p] {
			return
		}
		reach[p] = true
		if notLowered(p) {
			return
		}
		for _, dep := range p.Imports {
			visit(dep)
		}
	}
	visit(prog.Roots[0])
	var pkgs []cppgen.ProgramPackage
	for _, lp := range prog.Packages {
		if !reach[lp] || notLowered(lp) {
			continue
		}
		cp := r.Package(lp)
		if cp == nil || !cp.Checked() {
			res.Internal = append(res.Internal, "package "+lp.ImportPath+" was not checked")
			return res
		}
		pkgs = append(pkgs, cppgen.ProgramPackage{Pkg: cp})
	}
	files, err := cppgen.EmitProgram(prog.Fset, pkgs, GenRoot, cppgen.Config{})
	if err != nil {
		res.EmitErr = err
		return res
	}
	res.Files = files.Files
	res.MainPkg = "gpkg/" + prog.Roots[0].ImportPath + "/"
	return res
}

// collect adds the error diagnostics of l to res.Errors, and the messages with "internal error"
// to res.Internal.
func (res *Result) collect(l *diag.List) {
	if l == nil {
		return
	}
	for _, d := range l.Diagnostics() {
		if strings.Contains(d.Msg, "internal error") {
			res.Internal = append(res.Internal, d.Pos.String()+": "+d.Msg)
		}
		if d.Severity == diag.Error {
			res.Errors = append(res.Errors, d.Pos.String()+": "+d.Msg)
		}
	}
}
