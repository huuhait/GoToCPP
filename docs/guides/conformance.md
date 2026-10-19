# Conformance: Go's own `test/` folder

The Go repository has a folder `test/` with about 2,600 small programs, most of them regression
tests of the compiler and the language. gocpp runs them as a conformance suite (step 15.10). The
goal for v1.0 is that at least 90 % of the **applicable** run-tests pass; this guide describes the
tool (`tools/gotest`), the files it keeps, and how to read and update the results.

Go's test files are never copied into this repository (they are Go's, under Go's license). The tool
downloads the **pinned Go source tarball** (the Go version of the dev shell, checked by SHA-256,
`tools/gotest/PINS`), unpacks only `test/` into the cache (`$XDG_CACHE_HOME/gocpp/portstd`, the
folder `tools/portstd` uses for the same tarball), and reads the tests from there. The repository
keeps only the *selection* (`manifest.txt`), the *expectations* (`expect.txt`, `passing.txt`) and
the snapshot of the last run (`docs/conformance/go-test.md`). Where a test needed a fix in gocpp,
the regression test in this repository is our own minimal program.

## The directives

The first comment line of a test says how Go's runner treats it (`go/test/run.go`). The tool reads
it the way the runner does, including the flags after the word:

| directive | what gocpp does |
|---|---|
| `// run` | build with `gocpp build`, run in `test/` (arguments of the line passed); exit status 0 and output equal to `x.out` (a missing file means no output) |
| `// runoutput` | real Go runs the test; its output is a program that gocpp builds and runs |
| `// errorcheck` | `gocpp check -e`: the diagnostics must match the `// ERROR "regexp"` comments line by line (see below) |
| `// compile`, `// build` | the program must build (a package that is not `main` is imported by a small `main`) |
| `// rundir`, `// buildrundir`, `// compiledir`, `// builddir`, `// errorcheckdir`, `// errorcheckandrundir`, `// runindir` | the folder `x.dir` is a module of the packages of its files, built in order |
| `// errorcheckoutput` | real Go generates a program, which is then error-checked |
| `// skip`, `// asmcheck`, ... | not applicable |

The errorcheck matcher follows Go's `errorCheck`: the output lines are `file:LINE: message`
(gocpp's column and `# package` lines are dropped); every `// ERROR "re1" "re2"` comment expects one
line per regexp; `LINE`, `LINE+1`, `LINE-1` in a regexp stand for line numbers; a regexp matches the
message, never the file name; `//// ERROR` disables a comment. A mismatch is `missing` (an expected
error was not reported), `nomatch` (the line has errors, none matches) or `unmatched` (an error
without a comment). The failure clusters of the report separate "gocpp accepts what Go rejects",
"gocpp rejects a valid program", "message text differs" and "extra error".

## Applicable or not

`go run ./tools/gotest manifest` classifies every test: `applicable`, or `skip:<reason>`. The
result is `tools/gotest/manifest.txt` (path, directive, status, `needs` column); it is deterministic
and checked with `manifest -check` (it depends on GOOS/GOARCH like Go's runner: the committed file is
linux/amd64). The classifier reads the header, the build constraints, the imports and the
qualified names a test uses. The reasons, and who decided:

| code | by | meaning |
|---|---|---|
| `build-tag` | go | a //go:build constraint excludes linux/amd64 (as in Go's runner); cgo counts as off, so the tests that need cgo (`//go:build cgo`) are here |
| `upstream-skip` | go | the first line says // skip |
| `malformed-recipe` | go | no or unknown first-line directive |
| `assembly` | gocpp | needs Go assembly (asmcheck, .s files): gocpp has no assembler and no register ABI |
| `gc-flags` | gocpp | the directive passes gc compiler or linker flags (-d=, -m, -N, -l, -S, -gcflags, -ldflags, -lang, -race, -goexperiment, -godebug...): they test gc, not the language |
| `gc-pragma` | gocpp | tests how gc treats a compiler pragma (//go:nosplit, //go:noescape, //go:systemstack, ...) |
| `linkname` | gocpp | uses //go:linkname: user code can't link into the runtime (D6) |
| `cgo` | gocpp | uses cgo (import "C"): gocpp has header imports instead (D2) |
| `runtime-internals` | gocpp | uses runtime internals (memory statistics, stack traces, profiles, runtime/debug tuning) |
| `gc-finalizer` | gocpp | needs garbage-collector behavior: finalizers, cleanups, unique/weak with a tracing GC (D1) |
| `stack-growth` | gocpp | tests goroutine stack growth or the stack size limit (goroutines are OS threads: D7, D24) |
| `unsafe-layout` | gocpp | depends on the sizes or the layout of Go's types (unsafe.Sizeof/Offsetof/Alignof: D15) |
| `reflect-beyond-subset` | gocpp | uses reflect beyond the subset (MakeFunc, StructOf, FuncOf, Method calls: D6) |
| `syscall` | gocpp | uses package syscall or a system call directly |
| `os-exec` | gocpp | starts other programs (os/exec, os/signal), usually the go tool |
| `not-planned-package` | gocpp | imports a package that gocpp does not plan to provide (net/http, plugin, go/*, debug/*, cmd/*, ...) |
| `go-tool` | gocpp | tests the go tool or the gc toolchain (go:generate, GOROOT, objdump, vet...) |
| `spec-deviation` | gocpp | relies on a documented deviation from Go (docs/spec/language.md): D19, a declaration named C (C is reserved for C's built-in types); D23, print and println of a slice or a map |
| `line-directive` | gocpp | relies on //line directives rewriting file names and lines in compiler errors |
| `debug-info` | gocpp | tests gc's DWARF output or its symbol table |

cgo is treated as off, like the line `// +build !cgo` of Go's runner when CGO_ENABLED=0: about 20 of the `build-tag`
tests need it (`//go:build cgo`); gocpp has header imports instead (D2).

Tests that Go's own runner skips (`by go`) are not gocpp decisions: the report counts them apart
from the ones gocpp decides. A reason is never a way to hide a failure: it must be true of the
test before it is run. `tools/gotest/overrides.txt` gives the reason for a single test the
classifier cannot see (`path code  # why`).

## Running

Everything runs in the dev shell. The first run downloads the tarball (`go run ./tools/gotest
fetch`; with `GOCPP_OFFLINE=1` only the cache is used).

```
go run ./tools/gotest run -out results.json              # all applicable tests
go run ./tools/gotest run -dir fixedbugs -shard 0/4 -j 2 -out s0.json
go run ./tools/gotest run -filter '^ken/' -v             # a subset, every result printed
go run ./tools/gotest run -failing results.json -out again.json
go run ./tools/gotest report results.json                # counts, pass rate, clusters
go run ./tools/gotest report -md docs/conformance/go-test.md s0.json s1.json s2.json s3.json
```

Flags that matter: `-shard i/n` (a stable hash of the path, so that a shard does not change when tests
are added), `-limit`, `-dir`, `-directive`, `-filter`, `-j` (1 to 3: this is a heavy job; campaigns
go through the machine's heavy-job gate), `-run-timeout`, `-build-timeout`, `-asan`, `-cxx` (`g++`
by default), `-gocpp-cache` (keep one across runs so that the runtime is compiled once), `-work`. Every
command runs in a process group of its own; a program that forks dies with the test, and Ctrl-C stops
everything that runs. The runner sets `GOCPP_GATE_PHASE=15`, so that a feature of a later phase fails
with its own message instead of the phase gate.

The committed baseline (`docs/conformance/go-test.md`) was run with g++ at gocpp's default build mode;
clang++ and `-asan` have not been run yet (`-cxx clang++`, `-asan`).

## Reading the results

A result is `pass`, `fail` or `timeout`, with the stage that failed (`build`, `check`, `run`,
`compare`, `errorcheck`, `go`) and a **cluster**: the first message of the failure with quoted text,
numbers and paths replaced (`gocpp rejects: undefined: Q`, `package not available: reflect`, `not
supported yet (Phase 08): ...`, `C++ does not compile: ...`, `run: panic: ...`, `output differs`).
`report` ranks the clusters by size.

`tools/gotest/clusters.txt` maps cluster regexps to the plan step that makes them pass (`<step>
TAB <regexp>`, first match wins). `gotest triage results.json` writes `expect.txt` (`path needs
<step>`) from it and makes the manifest again, whose last column then says what each failing test
waits for. A test that passes leaves `expect.txt` at the next triage: the progress of a step is the
number of tests that leave its `needs` column. Clusters without an owner are listed by `triage`: each
is a bug (small: fix it with a regression test of our own; large: add a rule that names the owner
step) or a decision for the manager.

## The regression gate

`tools/gotest/passing.txt` is the set of tests that passed in the committed snapshot. `make
conformance-check` runs exactly these tests and fails when one of them no longer passes, or is no
longer an applicable test of the manifest. The set may only grow: `gotest gate -update -results ...`
rewrites it from the results of a full run, and refuses to drop tests unless `-allow-shrink` (say why in
the commit message). The gate is opt-in and heavy, so it is not part of `make ci`; the tool's own
tests (fake gocpp, fixture tarball, matcher and classifier cases) are, as part of `go test ./...`.
Run the gate before merging a change to the checker, the lowering or the runtime.

```
make conformance-check                        # the gate
make conformance GOTESTFLAGS='-shard 0/4'     # one shard of a full run, to conformance-results.json
make conformance-report
```

`fixedbugs/issue29362.go` (1,000 goroutines, each an OS thread: D7) died from a signal once in a loaded
run and passed in the three reruns, so it is not in `passing.txt`: the gate must not flake. A later
`gate -update` adds it back when it passes in the run it is given; look at that before committing.

## When Go moves

`go run ./tools/portstd -pin go1.NN.M` (step 9.1) updates the pin of the standard-library port. Copy
the three lines to `tools/gotest/PINS` (a test checks that the two files name the same tarball), run
`manifest`, review the diff of `manifest.txt` (new, removed and reclassified tests), run the whole
suite, then update `passing.txt` and the snapshot.
