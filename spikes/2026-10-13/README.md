# Spikes from 2026-10-13

Small throwaway programs that back the claims in the plan
(`plans/2026-10-13-gocpp/README.md` §7 and the phase notes). They are **evidence, not product
code**: keep them so anyone can re-run the checks, and reuse ideas from them in Phase 01 and
Phase 10. Tested with Go 1.25.7, GCC 15.2 and Clang 21.1.8 on NixOS.

| Folder | What it proves | Re-run |
|---|---|---|
| `frontend/` | The final import model on stock `go/parser` + `go/types` (`FakeImportC`): headers are packages (`raylib.X` → `C.X`), C++ namespaces are packages (`std.vector` → `C.std.vector`); interop-heavy `.gp` files type-check with 0 false errors; a namespace probe (clang++ and g++) reports typos such as `stdd` and `fmtt` as `undefined`; `edge.gp` shows the known gaps (embedded C++ types, C macros as constants) and the hint for an unnamed `<string.h>`; the `case_*.gp` files are the plan's examples | `go run main.go ok.gp verbose`, `go run main.go edge.gp`, `go run main.go case_P04_raylib.gp` |
| `frontend/parsecheck/` | What stock `go/parser` rejects: `std.array[int, 5]`, `std.map` (the reason for parser patches #1 and #2) | `go run ./parsecheck/main.go` |
| `frontend/probe*.cpp` + `readprobe.py` | clang "probes": C++ expression types via `decltype`, macro values via `gp_val<BUFSIZ>`; filtered vs unfiltered JSON size | see the commands in the plan's Phase 12 README §1 |
| `printer/` | Stock `go/printer` prints `std.map[...]`, `std.array[int, 5]`, `node.type()`; `ast.SortImports` order | `go run main.go` |
| `go-output-formats/` | Go's exact `fmt` float format, builtin `println` format, float → int conversion results | `go run fmt-floats/main.go`, `go run builtin-println/main.go` |
| `cpp-idioms/` | The key C++ shapes compile and behave the same on GCC and Clang (ARC aliasing, single-exit defer/recover, `requires` dispatch, designated init, tuples) | `g++ -std=c++20 idioms.cpp && ./a.out`, then the same with `clang++` |
| `must-calls/` | `F!(...)` via hidden generic helpers works on stock `go/types` | `go run main.go` |
| `macros-v1-functions/` | First macro design: functions + call site + lazy arguments; generic `Dbg` | `go run .` |
| `macros-v2-keyword-and-on-top/` | What `macro` declarations and start-style on-top macros (`#Trace`, `#Locked`) compile to; race-clean | `go run -race .` |
| `macros-v3-wrap/` | Wrap-style on-top macros with `#body()` (`#VM`, `#Recover`, `#InThread`, `#Once`): the function's code moves into a closure, returns set the results; race-clean | `go run -race .` |
| `must-functions/` | Must functions `func f!() T` + must calls on stock `go/types`: first check, rewrite of every dropped error, second check with 0 errors, then the generated program runs for a good config, a missing port and a missing file | `go run main.go` (`go run main.go show` also prints the generated Go) |
| `lsp-speed/` | `go/types` re-check time per package with cached dependencies (basis for the Phase 11 budgets) | `go run .` |
| `language-tour/` | The tour examples: Go ones run under real Go; `.gp` ones (in the final import syntax, incl. a helper header `mathx.h` used as a package) type-check with the front-end spike | `go run basic/main.go`, `go run abstr/main.go`; `.gp`: `cd ../frontend && go run main.go ../language-tour/gp/stdlib.gp` |
| `tools/gpfmtcheck.py` | gofmt-checks gocpp code by mapping gocpp-only syntax to same-shape Go placeholders (used on every Go block in the plan) | `python3 tools/gpfmtcheck.py file.gp` |
| `audit-checks/xtools-opaque/` | From the plan audit: on code with `C.` names, `go/types` reports 0 errors, yet `refactor/satisfy` (used by `printf`) and `go/ssa` (used by `nilness`) panic. This is why Phase 13 guards `gocpp vet` | `go run .` (x/tools must be in the module cache, or allow downloads) |
| `audit-checks/clang-probes/` | From the plan audit: integer macro values come from the `ConstantExpr` value (the type name may show the expression); computed floats print with 7 digits, so they go through `bit_cast`; string macros come from `StringLiteral`; header index and code-completion flags (Phase 12) | `clang++ -std=c++20 -fsyntax-only -Xclang -ast-dump=json -Xclang -ast-dump-filter=gp_probe values.cpp \| python3 readvalues.py` (same for `float-digits.cpp`); `clang++ -std=c++20 -fsyntax-only -Xclang -ast-list index.cpp \| wc -l`; `clang++ -std=c++20 -fsyntax-only -Xclang -code-completion-at=complete.cpp:4:5 complete.cpp` |

Notes:
- `frontend/main.go` uses `importer.ForCompiler(fset, "source", nil)`, so the first run takes a
  few seconds while it type-checks the Go standard library from source.
- The `.gp` files that parse with stock `go/parser` don't use `std.map` or `std.array[T, N]`.
  The parser patches come in Phase 01.
