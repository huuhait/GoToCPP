# Phase 13: Developer Tooling · STATE

**Status:** `[~]` in progress (13.1, 13.2, 13.4, 13.5 merged; 13.3 merged without most of its gocpp analyzers: 13.3b)
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-19

## Steps

- [x] Test: x/tools on code that uses C/C++ names: go/types reports 0 errors, `refactor/satisfy` (printf) and `go/ssa` (nilness) panic → guard needed (`spikes/2026-10-13/audit-checks/xtools-opaque`, 2026-10-13)
- [x] 13.1 Diagnostics polish (excerpts, color, C++ → gocpp names incl. `gpkg_…` namespaces, noise filter, "in expansion of" notes)
- [x] 13.2 Debugging
  - [x] GDB pretty printers
  - [x] LLDB data formatters
  - [x] `.debug_gdb_scripts` + safe-path handling (`-iex add-auto-load-safe-path`), LLDB `command script import`
  - [x] path remapping for `-trimpath` builds (`-ffile-prefix-map` + substitute-path/source-map)
  - [x] `gocpp debug`
  - [x] VS Code: `contributes.breakpoints` for gocpp + launch template
- [x] 13.3 `gocpp vet` (the gocpp analyzers are 13.3b, merged 2026-10-19)
  - [x] hand-built `packages.Package` + `checker.Analyze` (facts across packages)
  - [x] `IllTyped` guard for packages with opaque expressions; `recover` per analyzer; `-v` lists skipped analyzers
  - [x] Go analyzers (printf, shadow, copylock, nilness, ...)
  - [x] gocpp analyzers (cycles, `&cppLocal`, `C.cstr` lifetime + "keeps the pointer" list + NUL bytes, narrowing, panicking `!` in libraries, ignored errors, `go` in `!` functions, unwaited `#body()`, unused lazy params)
  - [x] CI: every analyzer over the Phase 04 interop examples
- [x] 13.3b The missing gocpp vet analyzers (the un-ticked sub-box above): `panicbang` (panicking `!` in libraries, replaces the placeholder), `&cppLocal`, `C.cstr` lifetime + "keeps the pointer" list + NUL bytes, narrowing, ignored errors, `go` in `!` functions, unwaited `#body()`, unused lazy params (internal/vet, docs/guides/vet.md; job running)
- [x] 13.4 `gocpp doc`
- [x] 13.5 `gocpp emit --annotate` + `gocpp explain`

## Exit criteria

- [x] Scripted gdb/lldb sessions show gocpp values and stop on `.gp` breakpoints (also with `-trimpath`)
- [ ] vet finds a closure cycle, a printf mistake (also through a cross-package wrapper), and a panicking `!` in a library
- [x] vet runs over every interop example without crashing
- [x] ≥ 20 error tests show C++ errors with gocpp type names

## Decisions / notes

- 2026-10-13: Split from the old tooling phase. The language server is now Phase 11, and
  `gocpp fmt` moved to Phase 01 (step 1.16) so it's available from the start.
- 2026-10-13 (audit): Go's analyzers assume well-typed code, and opaque C names look well-typed
  to them (0 errors) while crashing `satisfy`/`ssa` (verified). Packages with opaque expressions
  are marked `IllTyped`, so only `RunDespiteErrors` analyzers run there; everything runs under
  `recover`. Debugging: GDB safe-path, LLDB import, VS Code `contributes.breakpoints`, path
  remapping for `-trimpath`.
- 2026-10-14 (from the 2.11 debug test): gdb pretty-printer candidates for `gocpp debug`: `gp::string`
  (prints its C++ fields; `gp_data_` up to a NUL is not the string's end: `*s.gp_data_@s.gp_len_`),
  int8/uint8 printed with a character (`4 '\004'`), `info locals` shows `gp_t0`/`gp_i0` and
  not-yet-declared variables; `step` at `}` enters `gp::string::~string` (ship `skip -gfi */gocpp/rt/*`).
