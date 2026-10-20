# fmt

**Not ported.** Decision 9.7 = A ([ADR-100](../../docs/adr/100-fmt-stays-native-decision-9-7-a.md)): `fmt` is the native engine of step 5.11
([ADR-028](../../docs/adr/028-fmt-on-descriptors.md)), a C++ port of Go 1.25's `fmt/print.go` and `fmt/format.go` that walks values by
their type descriptors, plus the Scan family (`runtime/src/fmtscan.cpp`). `fmt.gp` here is only the declarations
(`tools/stubgen`, from go1.25.14): the package is a shim (`shimPackages` in `cmd/gocpp/build.go`), its calls are written by the lowering
(`internal/lower/fmtcall.go`) as calls of `gp::fmt::*` (`runtime/include/gocpp/fmt.hpp`), and it has no C++ of its own.

| | |
|---|---|
| Engine | `runtime/include/gocpp/fmt.hpp`, `fmtcxx.hpp`, `print.hpp`, `runtime/src/fmt.cpp` |
| Scan | `runtime/include/gocpp/fmtscan.hpp`, `runtime/src/fmtscan.cpp` |
| Go's tests | `tests/gotest/testdata/fmt_gotest` (`gocpp test` against `go test -v`), `tests/run/fmt_gotest_scan.gp` |
| Differential tests | `tests/run/fmt_*.gp` (output from real Go), `tools/fmtdiff` (generator, Go against gocpp) |

What differs from Go's `fmt` is in ADR-100 (allocation counts, nesting depth of about 10,000 levels, the internals of `sync` types,
`reflect.Value.Method`) and in [docs/spec/cpp-mapping.md](../../docs/spec/cpp-mapping.md#fmt-and-the-builtins).

A change to the engine: run `go test ./tests/gotest -run fmt_gotest`, `go test ./tests -run 'TestRun/fmt_'` and
`go run ./tools/fmtdiff` (see its README), and the benchmark rows `fmt_sprintf` and `fmt_println`.
