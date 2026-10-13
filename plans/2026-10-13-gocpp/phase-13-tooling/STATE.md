# Phase 13: Developer Tooling · STATE

**Status:** `[ ]` not started
**Legend:** `[ ]` not started · `[~]` in progress · `[x]` done
**Last updated:** 2026-10-13

## Steps

- [x] Test: x/tools on code that uses C/C++ names: go/types reports 0 errors, `refactor/satisfy` (printf) and `go/ssa` (nilness) panic → guard needed (`spikes/2026-10-13/audit-checks/xtools-opaque`, 2026-10-13)
- [ ] 13.1 Diagnostics polish (excerpts, color, C++ → gocpp names incl. `gpkg_…` namespaces, noise filter, "in expansion of" notes)
- [ ] 13.2 Debugging
  - [ ] GDB pretty printers
  - [ ] LLDB data formatters
  - [ ] `.debug_gdb_scripts` + safe-path handling (`-iex add-auto-load-safe-path`), LLDB `command script import`
  - [ ] path remapping for `-trimpath` builds (`-ffile-prefix-map` + substitute-path/source-map)
  - [ ] `gocpp debug`
  - [ ] VS Code: `contributes.breakpoints` for gocpp + launch template
- [ ] 13.3 `gocpp vet`
  - [ ] hand-built `packages.Package` + `checker.Analyze` (facts across packages)
  - [ ] `IllTyped` guard for packages with opaque expressions; `recover` per analyzer; `-v` lists skipped analyzers
  - [ ] Go analyzers (printf, shadow, copylock, nilness, ...)
  - [ ] gocpp analyzers (cycles, `&cppLocal`, `C.cstr` lifetime + "keeps the pointer" list + NUL bytes, narrowing, panicking `!` in libraries, ignored errors, `go` in `!` functions, unwaited `#body()`, unused lazy params)
  - [ ] CI: every analyzer over the Phase 04 interop examples
- [ ] 13.4 `gocpp doc`
- [ ] 13.5 `gocpp emit --annotate` + `gocpp explain`

## Exit criteria

- [ ] Scripted gdb/lldb sessions show gocpp values and stop on `.gp` breakpoints (also with `-trimpath`)
- [ ] vet finds a closure cycle, a printf mistake (also through a cross-package wrapper), and a panicking `!` in a library
- [ ] vet runs over every interop example without crashing
- [ ] ≥ 20 error tests show C++ errors with gocpp type names

## Decisions / notes

- 2026-10-13: Split from the old tooling phase. The language server is now Phase 11, and
  `gocpp fmt` moved to Phase 01 (step 1.15) so it's available from the start.
- 2026-10-13 (audit): Go's analyzers assume well-typed code, and opaque C names look well-typed
  to them (0 errors) while crashing `satisfy`/`ssa` (verified). Packages with opaque expressions
  are marked `IllTyped`, so only `RunDespiteErrors` analyzers run there; everything runs under
  `recover`. Debugging: GDB safe-path, LLDB import, VS Code `contributes.breakpoints`, path
  remapping for `-trimpath`.
