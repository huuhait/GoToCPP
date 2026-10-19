package main

import (
	"fmt"
	"sort"
	"strings"
)

// Reason codes of a test that does not apply to gocpp: "skip:<code>" in
// the manifest. The table is the documentation (docs/guides/conformance.md
// prints it) and a test checks that every code the classifier gives is in
// it. A reason is "by Go" when Go's own runner skips the test too (it is
// not a gocpp decision, so the count of such tests is reported apart from
// the gocpp decisions).
var reasons = []struct {
	Code string
	By   string // "go" or "gocpp"
	Text string
}{
	{"build-tag", "go", "a //go:build constraint excludes linux/amd64 (as in Go's runner); cgo counts as off, so the tests that need cgo (`//go:build cgo`) are here"},
	{"upstream-skip", "go", "the first line says // skip"},
	{"malformed-recipe", "go", "no or unknown first-line directive"},
	{"assembly", "gocpp", "needs Go assembly (asmcheck, .s files): gocpp has no assembler and no register ABI"},
	{"gc-flags", "gocpp", "the directive passes gc compiler or linker flags (-d=, -m, -N, -l, -S, -gcflags, -ldflags, -lang, -race, -goexperiment, -godebug...): they test gc, not the language"},
	{"gc-pragma", "gocpp", "tests how gc treats a compiler pragma (//go:nosplit, //go:noescape, //go:systemstack, ...)"},
	{"linkname", "gocpp", "uses //go:linkname: user code can't link into the runtime (D6)"},
	{"cgo", "gocpp", "uses cgo (import \"C\"): gocpp has header imports instead (D2)"},
	{"runtime-internals", "gocpp", "uses runtime internals (memory statistics, stack traces, profiles, runtime/debug tuning)"},
	{"gc-finalizer", "gocpp", "needs garbage-collector behavior: finalizers, cleanups, unique/weak with a tracing GC (D1)"},
	{"stack-growth", "gocpp", "tests goroutine stack growth or the stack size limit (goroutines are OS threads: D7, D24)"},
	{"unsafe-layout", "gocpp", "depends on the sizes or the layout of Go's types (unsafe.Sizeof/Offsetof/Alignof: D15)"},
	{"reflect-beyond-subset", "gocpp", "uses reflect beyond the subset (MakeFunc, StructOf, FuncOf, Method calls: D6)"},
	{"syscall", "gocpp", "uses package syscall or a system call directly"},
	{"os-exec", "gocpp", "starts other programs (os/exec, os/signal), usually the go tool"},
	{"not-planned-package", "gocpp", "imports a package that gocpp does not plan to provide (net/http, plugin, go/*, debug/*, cmd/*, ...)"},
	{"go-tool", "gocpp", "tests the go tool or the gc toolchain (go:generate, GOROOT, objdump, vet...)"},
	{"spec-deviation", "gocpp", "relies on a documented deviation from Go (docs/spec/language.md): D19, a declaration named C (C is reserved for C's built-in types); D23, print and println of a slice or a map"},
	{"line-directive", "gocpp", "relies on //line directives rewriting file names and lines in compiler errors"},
	{"debug-info", "gocpp", "tests gc's DWARF output or its symbol table"},
}

// reasonCodes returns the set of codes.
func reasonCodes() map[string]bool {
	m := map[string]bool{}
	for _, r := range reasons {
		m[r.Code] = true
	}
	return m
}

// A Test is one entry of the manifest.
type Test struct {
	Path      string // slash path relative to test/: fixedbugs/bug1.go
	Directive string // the recipe action (run, errorcheck, ...)
	Status    string // "applicable" or "skip:<reason>"
	Needs     string // "" or "needs 7.3": the step that makes a failing test pass (expect.txt)
	Detail    string // why (not in the manifest)
}

// Skipped reports whether the test is not applicable.
func (t Test) Skipped() bool { return t.Status != "applicable" }

// Reason returns the code of a skipped test.
func (t Test) Reason() string {
	if c, ok := strings.CutPrefix(t.Status, "skip:"); ok {
		return c
	}
	return ""
}

// ClassifyOptions are the inputs of the classifier besides the source.
type ClassifyOptions struct {
	Target    Target
	Overrides map[string]string // path -> reason code (overrides.txt)
	HasAsm    func(dir string) bool
	// DirSources returns the text of the .go files of a .dir folder (for the
	// directory tests, whose files the first-line header does not show).
	DirSources func(dir string) []string
}

// runtimeOK are the functions of package runtime that gocpp provides or
// will provide (Phase 08/09): they don't make a test inapplicable.
var runtimeOK = map[string]bool{
	"Gosched": true, "GOMAXPROCS": true, "NumGoroutine": true, "NumCPU": true,
	"KeepAlive": true, "GOOS": true, "GOARCH": true, "Goexit": true,
	"GC": true, "LockOSThread": true, "UnlockOSThread": true, "Error": true,
	"Version": true, "Compiler": true, "Breakpoint": true, "TypeAssertionError": true, "Caller": true,
	"Func": true, "Frame": true, "Frames": true, "NumCgoCall": false,
}

var runtimeFinalizer = map[string]bool{"SetFinalizer": true, "AddCleanup": true, "Cleanup": true}

var runtimeStack = map[string]bool{"Stack": true, "Callers": true, "CallersFrames": true, "FuncForPC": true}

var reflectBeyond = map[string]bool{
	"MakeFunc": true, "StructOf": true, "FuncOf": true, "MethodByName": true, "Method": true,
	"Select": true, "SelectCase": true, "SelectSend": true, "SelectRecv": true, "SelectDefault": true,
	"ArrayOf": true, "ChanOf": true, "MapOf": true, "SliceOf": true, "NewAt": true, "VisibleFields": false,
	"Swapper": false, "Call": true, "CallSlice": true,
}

var gcPragmas = map[string]bool{
	"nosplit": true, "noescape": true, "nowritebarrier": true, "nowritebarrierrec": true,
	"yeswritebarrierrec": true, "systemstack": true, "registerparams": true, "uintptrescapes": true,
	"uintptrkeepalive": true, "nocheckptr": true, "notinheap": true, "norace": true, "cgo_import_dynamic": true,
	"cgo_ldflag": true, "cgo_export_static": true, "cgo_export_dynamic": true, "cgo_import_static": true,
	"cgo_unsafe_args": true, "wasmimport": true, "wasmexport": true, "nocheckptr:": true,
}

// notPlanned is the set of import path prefixes that gocpp will not provide
// (the "Not planned" row of the Phase 09 tiers, and the tools).
var notPlanned = []string{"net/http", "plugin", "go/", "debug/", "cmd/", "internal/", "crypto/tls", "crypto/x509",
	"database/sql", "html/template", "expvar", "image", "net/rpc", "net/smtp", "net/mail", "net/textproto", "mime"}

// Classify decides whether the test at relPath (with source src) applies to
// gocpp, and why not. dirHasAsm reports whether the .dir folder of the test
// holds assembly files.
func Classify(relPath, src string, o ClassifyOptions) Test {
	t := Test{Path: relPath, Status: "applicable"}
	skip := func(code, detail string) Test {
		t.Status = "skip:" + code
		t.Detail = detail
		return t
	}
	if ok, why := o.Target.shouldTest(src); !ok {
		t.Directive = "?"
		if r, err := ParseRecipe(src); err == nil {
			t.Directive = r.Action
		}
		return skip("build-tag", why)
	}
	r, err := ParseRecipe(src)
	if err != nil {
		t.Directive = "?"
		return skip("malformed-recipe", err.Error())
	}
	t.Directive = r.Action
	if code, ok := o.Overrides[relPath]; ok {
		return skip(code, "overrides.txt")
	}
	switch r.Action {
	case "skip":
		return skip("upstream-skip", "")
	case "asmcheck":
		return skip("assembly", "asmcheck")
	}
	if strings.HasPrefix(relPath, "codegen/") {
		return skip("assembly", "test/codegen")
	}
	if strings.HasPrefix(relPath, "dwarf/") {
		return skip("debug-info", "test/dwarf")
	}
	var gcFlags []string
	for _, fl := range r.Flags {
		if !harmlessFlag(fl) {
			gcFlags = append(gcFlags, fl)
		}
	}
	for _, e := range r.GoExp {
		for _, x := range strings.Split(e, ",") {
			if !harmlessExperiments[x] {
				gcFlags = append(gcFlags, "-goexperiment "+x)
			}
		}
	}
	if len(gcFlags) > 0 || len(r.GoDebug) > 0 {
		return skip("gc-flags", strings.Join(gcFlags, " "))
	}
	if strings.HasSuffix(r.Action, "dir") && o.HasAsm != nil && o.HasAsm(strings.TrimSuffix(relPath, ".go")+".dir") {
		return skip("assembly", ".s files")
	}
	srcs := []string{src}
	if strings.HasSuffix(r.Action, "dir") && o.DirSources != nil {
		srcs = append(srcs, o.DirSources(strings.TrimSuffix(relPath, ".go")+".dir")...)
	}
	f := scanFacts(srcs[0])
	for _, other := range srcs[1:] {
		f = f.merge(scanFacts(other))
	}
	for _, imp := range f.Imports {
		switch {
		case imp == "C":
			return skip("cgo", "import \"C\"")
		case imp == "syscall" || strings.HasPrefix(imp, "syscall/"):
			return skip("syscall", imp)
		case imp == "os/exec" || imp == "os/signal":
			return skip("os-exec", imp)
		case imp == "runtime/pprof" || imp == "runtime/trace" || imp == "runtime/metrics" || imp == "runtime/race" ||
			imp == "runtime/cgo" || imp == "runtime/coverage":
			return skip("runtime-internals", imp)
		case imp == "runtime/debug":
			for _, u := range f.useList() {
				if name, ok := strings.CutPrefix(u, "debug."); ok {
					if name == "SetMaxStack" {
						return skip("stack-growth", u)
					}
				}
			}
			return skip("runtime-internals", imp)
		case imp == "unique":
			return skip("gc-finalizer", imp)
		case imp == "net":
			return skip("not-planned-package", imp)
		}
		for _, p := range notPlanned {
			if imp == p || strings.HasSuffix(p, "/") && strings.HasPrefix(imp, p) {
				return skip("not-planned-package", imp)
			}
		}
	}
	for _, u := range f.useList() {
		switch name, _ := strings.CutPrefix(u, "runtime."); {
		case !strings.HasPrefix(u, "runtime."):
		case runtimeFinalizer[name]:
			return skip("gc-finalizer", u)
		case runtimeStack[name]:
			return skip("runtime-internals", u)
		case runtimeOK[name]:
		case name == "MemStats" || name == "ReadMemStats" || name == "MemProfile" || name == "MemProfileRate" ||
			name == "SetBlockProfileRate" || name == "SetMutexProfileFraction" || name == "BlockProfile" ||
			name == "SetCPUProfileRate" || name == "ThreadCreateProfile" ||
			name == "GoroutineProfile" || name == "NumCgoCall" || name == "GOROOT" || name == "ReadTrace" ||
			name == "StartTrace" || name == "StopTrace" || name == "CPUProfile":
			return skip("runtime-internals", u)
		}
	}
	for _, u := range f.useList() {
		name, ok := strings.CutPrefix(u, "unsafe.")
		if ok && (name == "Sizeof" || name == "Offsetof" || name == "Alignof") && runsProgram(r.Action) {
			return skip("unsafe-layout", u)
		}
		if name, ok := strings.CutPrefix(u, "reflect."); ok && reflectBeyond[name] {
			return skip("reflect-beyond-subset", u)
		}
		if strings.HasPrefix(u, "//go:") {
			p := strings.TrimPrefix(u, "//go:")
			if p == "linkname" {
				return skip("linkname", u)
			}
			if gcPragmas[p] && (r.Action == "errorcheck" || r.Action == "compile" || strings.HasPrefix(r.Action, "errorcheck") || strings.HasPrefix(r.Action, "compile")) {
				return skip("gc-pragma", u)
			}
		}
	}
	for _, s := range srcs {
		if strings.Contains(s, "\n//line ") || strings.Contains(s, "\n/*line ") {
			if r.Action == "errorcheck" || strings.HasPrefix(r.Action, "errorcheck") {
				return skip("line-directive", "//line")
			}
		}
	}
	for _, s := range srcs {
		if declaresReservedC(s) {
			return skip("spec-deviation", "D19: declares C")
		}
	}
	return t
}

// ParseOverrides reads overrides.txt: lines "<path> <reason code>  # why".
func ParseOverrides(text string) (map[string]string, error) {
	m := map[string]string{}
	codes := reasonCodes()
	for _, cl := range configLines(text) {
		f := strings.Fields(cl.Text)
		if len(f) != 2 || !codes[f[1]] {
			return nil, fmt.Errorf("overrides.txt line %d: want <path> <reason code>, got %q", cl.N, cl.Text)
		}
		m[f[0]] = f[1]
	}
	return m, nil
}

// SortTests orders tests by path.
func SortTests(ts []Test) {
	sort.Slice(ts, func(i, j int) bool { return ts[i].Path < ts[j].Path })
}

// harmlessFlag reports whether a gc flag does not change what a program or
// an error message means, so that gocpp can ignore it: inlining and
// optimization switches, compiler concurrency, SSA checks.
func harmlessFlag(f string) bool {
	switch {
	case f == "-l", f == "-N", f == "-std", f == "-smallframes", f == "-d=panic", f == "-w=0", f == "-wb=0",
		strings.HasPrefix(f, "-l="), strings.HasPrefix(f, "-c="), strings.HasPrefix(f, "-p="),
		f == "-d=ssa/check/on", strings.HasPrefix(f, "-d=ssa/insert_resched_checks/"):
		return true
	}
	return false
}

// runsProgram reports whether the directive runs a program (so that its
// output depends on the layout of values).
func runsProgram(action string) bool {
	switch action {
	case "run", "runoutput", "rundir", "buildrun", "buildrundir", "runindir", "errorcheckandrundir":
		return true
	}
	return false
}

// harmlessExperiments are GOEXPERIMENT names that are the default in the
// pinned Go version (the language feature is always on) or that only change
// how gc compiles (the register ABI, the map implementation), not what a
// program does.
var harmlessExperiments = map[string]bool{
	"rangefunc": true, "aliastypeparams": true, "regabi": true, "regabiargs": true,
	"noswissmap": true, "swissmap": true, "nospinbitmutex": true,
}
