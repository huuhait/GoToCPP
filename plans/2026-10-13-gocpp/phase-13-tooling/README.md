# Phase 13: Developer Tooling (diagnostics, debugging, vet, docs)

> Everything around the compiler and the editor: C++ error messages that read like Go, a
> debugger that shows gocpp values nicely, `gocpp vet` with gocpp-specific checks, `gocpp doc`,
> and helpers to see the generated C++. (`gocpp fmt` lives in Phase 01, and the language server
> has its own phase, Phase 11.)

| | |
|---|---|
| Depends on | Phase 02 (`#line`), Phase 04 (C++ errors), Phase 10 (vet checks for `!` and macros), Phase 12 (C++ type info for better messages) |
| Unlocks | Phase 14 (debugging on Windows builds on it), Phase 15 (release) |
| Size | medium (about 2–3 weeks; several pieces can start early) |
| Progress | [STATE.md](STATE.md) |

---

## 1. Idea

Using stock `go/ast` and `go/types` pays off again. Mature Go tools work on exactly those data
structures: `go/doc` (documentation) and `golang.org/x/tools/go/analysis` (vet analyzers). gocpp
plugs its own parser in front of them and gets much of this for free. The real work goes into the
parts unique to gocpp: translating C++ compiler errors into gocpp terms, debugger printers for
`gp::` types, vet checks for gocpp's own traps (reference cycles, raw C++ pointers, `!`), and
keeping Go's analyzers away from code they can't handle (§2.3).

## 2. How it works

### 2.1 Diagnostics that read like Go

- Source excerpt + caret + optional color (`-color=auto`).
- **C++ errors translated to gocpp names:** `gp::slice<int64_t>` → `[]int`,
  `gpkg_main::Vec2` → `main.Vec2`, `gpkg_github_com__huuhait__game__engine::Body` →
  `engine.Body` (package namespaces from Phase 06), `gp::ptr<T>` → `*T`, `gp::string` → `string`,
  `gp::map<K, V>` → `map[K]V`. Notes that point into runtime headers and template instantiation
  stacks inside `gp::` are hidden unless `-cpperrors=full`.

```
main.gp:14:7: C++: no member named 'push_bak' in std.vector[int]   (C++ type: std::vector<long>)
        nums.push_bak(4)
             ^
```

- Errors inside macro expansions or derive output name both places:
  `main.gp:9:7: in #util.Dbg (util/util.gp:21): ...`.

### 2.2 Debugging

- `#line` (Phase 02) already makes breakpoints and stepping work on `.gp` lines. The paths are
  absolute, so the debugger finds the files from any working directory. Builds with `-trimpath`
  (Phase 15) shorten them with `-ffile-prefix-map`, and `gocpp debug` maps them back
  (`set substitute-path` in GDB, `target.source-map` in LLDB).
- **Pretty printers** for GDB (Python) and LLDB (data formatters) for `gp::string` (shows the
  text), `gp::slice` (len/cap + elements), `gp::map`, `gp::ptr` (nil or pointee), `gp::iface`
  (dynamic type name from the type descriptor + value) and `gp::chan`.
- **Loading them.** A `.debug_gdb_scripts` ELF section names the printers, but GDB only
  auto-loads scripts from its *safe path*. So `gocpp debug` starts GDB with
  `-iex "add-auto-load-safe-path <dir>"`, and the docs show the one `gdbinit` line needed for
  plain `gdb`. LLDB has no auto-load for ELF files, so `gocpp debug` runs
  `command script import <formatters.py>` at startup.
- `gocpp debug [pkg] [args]` builds with `-debug` and starts gdb or lldb with all of the above.
- **VS Code:** the gocpp extension (Phase 11) declares `contributes.breakpoints` for the
  `gocpp` language, so breakpoints in `.gp` files work without the global
  `debug.allowBreakpointsEverywhere` setting. A `launch.json` template (CodeLLDB or cppdbg) loads
  the printers in its init/setup commands.

### 2.3 `gocpp vet`

**Driver.** The standard drivers load packages with the `go` command, which doesn't know `.gp`.
So `gocpp vet` builds `*packages.Package` values by hand from its own front-end (syntax after
`!`/macro expansion, `types.Package`, `types.Info`, imports) and runs them through
`golang.org/x/tools/go/analysis/checker.Analyze`, which also handles *facts* across packages
(for example, `printf` learning that `log.Infof` wraps `fmt.Printf`). Positions map back to the
original `.gp` source.

**Opaque code needs a guard (verified in the audit).** For code that uses C/C++ names,
`go/types` reports *zero* errors, because C names are silently given invalid types. Analyzers
therefore think the package is well-typed, and some of them crash. A test on 2026-10-13 showed
both `refactor/satisfy` (used by `printf`) and `go/ssa` (used by `nilness` and `unusedwrite`)
panic with a nil pointer dereference on a small file with `C.strlen(...)` (the form
`cstring.strlen(...)` takes after the Phase 01 rewrite). So:
- Every package where *any* expression has an invalid type (its own C/C++ names, or opaque
  types reaching it through an imported package's API) is marked `IllTyped` for the checker.
  The checker then runs only analyzers declared with `RunDespiteErrors` there (`copylock`,
  `composite`, `structtag`, `unreachable`, `atomic`, ...). The others (`printf`, `nilness`,
  `shadow`, `unusedresult`, ...) run on pure gocpp packages, which is most of a typical program.
  `gocpp vet -v` lists what was skipped and why.
- Every analyzer runs inside `recover`. A crash becomes a "vet: analyzer X failed on package Y"
  note instead of killing vet, and it is reported as a gocpp bug.
- A CI test runs every enabled analyzer over all interop examples (the Phase 04 suite).
- typed mode (Phase 12) doesn't change this, because it keeps stock `go/types` (C names stay
  invalid there). The gocpp analyzers below use the interop checker's types directly.

gocpp-specific analyzers:
- reference cycles (self-recursive closures, obvious parent↔child pointer cycles);
- `&cppLocal` (raw pointer to a C++ stack value) escaping its function;
- a `C.cstr(...)` result stored beyond its statement, and strings passed to C functions known to
  keep the pointer (the Phase 12 list), or strings that may contain a NUL byte;
- narrowing conversions at the C/C++ boundary (typed mode);
- `F!(...)` that would **panic** inside a library package (libraries should return errors);
- an ignored error result where `!` or explicit handling is expected;
- `go f()` inside a `!` function (the error can't come back), and `#body()` started in a `go`
  statement that nothing waits for;
- lazy macro parameters that are never used.

### 2.4 Small but useful

- `gocpp doc pkg.Name`: terminal docs built with `go/doc` (macros and derived methods included).
- `gocpp emit --annotate`: C++ output with the original `.gp` lines as comments.
- `gocpp explain file.gp:LINE`: shows the C++ generated for one line (great for learning and
  bug reports).
- Backlog: a web **playground** that compiles the transpiler to WebAssembly (Go → wasm) and shows
  `.gp` → C++ live.

## 3. Steps

1. **13.1 Diagnostics polish** (excerpts, color, C++ → gocpp name translation incl. package
   namespaces, noise filtering, macro/derive "in expansion of" notes).
2. **13.2 Debugging** (GDB + LLDB printers, safe-path/auto-load handling, path remapping for
   `-trimpath`, `gocpp debug`, VS Code breakpoints + launch template).
3. **13.3 `gocpp vet`** (hand-built packages + `checker.Analyze`, `IllTyped` guard for opaque
   packages, `recover` per analyzer, Go analyzers + gocpp analyzers incl. `!` and macros, CI run
   over the interop examples).
4. **13.4 `gocpp doc`.**
5. **13.5 `emit --annotate` and `explain`.**

## 4. Deliverables

- `gocpp vet`, `gocpp doc`, `gocpp debug`, `gocpp explain`, `emit --annotate`
- GDB and LLDB pretty printers
- C++ error translation in the driver

## 5. Exit criteria

- Scripted gdb and lldb sessions (started by `gocpp debug`) show `gp::string`/`slice`/`map`/`ptr`/
  `iface` values nicely, and stop at breakpoints set on `.gp` lines, also in a `-trimpath` build.
- `gocpp vet` finds, in tests: a closure reference cycle, a `printf` mistake (and a `printf`
  mistake through a wrapper in another package), and a panicking `!` in a library package.
- `gocpp vet` runs over every Phase 04 interop example without a crash.
- ≥ 20 error tests show C++ errors with gocpp type names.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Analyzers crash or mislead on opaque (invalid) types | `IllTyped` guard, `recover` per analyzer, CI run over the interop examples |
| x/tools changes its API or needs a newer Go | Pin x/tools in `go.mod` together with the Go toolchain; the driver is small |
| Debugger printers break when runtime layouts change | Printers tested in CI against the runtime version |
| C++ error formats differ between GCC and Clang | Parse both; golden tests per compiler |
