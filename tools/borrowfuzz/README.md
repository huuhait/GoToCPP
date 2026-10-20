# tools/borrowfuzz

A differential fuzzer for the borrowed parameters of step 15.3/15.3b (ADR-086). A program has many
statements, each one calling a function or method that reads its pointer (or string, slice, interface)
parameter after it dropped every other owner of the object (through package variables, fields,
elements, maps, closures, defers and goroutines). The call forms are a plain call, a method value, a
method expression, a func variable, an interface call, `defer` and `go`. Real Go prints the output;
gocpp must print the same, with no AddressSanitizer report, with both compilers.

- `genptr.py SEED [N]` prints one program whose parameters are pointers (`gp::bptr`, borrowed).
- `genval.py SEED [N]` prints one whose parameters are strings, slices, interfaces and structs
  (const references).
- `run.sh MODE FIRST LAST [GOCPP]` generates the seeds, runs them with `go run` and with
  `gocpp run -asan` for each compiler. `GOCPP` and `BORROWFUZZ_WORK` paths are resolved relative
  to the invocation directory, before workers enter their seed directories.
- `OK` requires successful generation, reference execution and candidate execution, matching
  stdout and stderr, and no sanitizer diagnostic. Failed stages, differences and sanitizer
  reports make the worker and the whole script exit nonzero; matching failed builds are not `OK`.
- Every command has a deadline (`BORROWFUZZ_TIMEOUT`, default `600s`). Parallelism is bounded by
  `BORROWFUZZ_JOBS` (default 4). The runner needs `go`, `python3`, `timeout`, `rg` and the compilers.
- Evidence is retained in unique `ptrSEED.*` or `valSEED.*` directories under `BORROWFUZZ_WORK`
  (a new temporary directory by default): generated source, stdout, stderr and stage exit statuses.
  The runner removes nothing, including on failure.

```sh
nix develop -c tools/borrowfuzz/run.sh ptr 1 80 ./gocpp
nix develop -c tools/borrowfuzz/run.sh val 1 30 ./gocpp
```

`go test ./tools/borrowfuzz -run '^TestBorrowFuzzRunner'` runs bounded synthetic tests of
successful execution, relative paths, failed generation/builds, differences, sanitizer diagnostics,
timeouts and failure propagation from parallel workers. These tests do not invoke a native compiler.

An earlier step 15.3 run reported 80 pointer and 30 value seeds as `OK` with both compilers.
The old runner did not require successful executions, so that historical report alone is not
validation of the corrected harness.
