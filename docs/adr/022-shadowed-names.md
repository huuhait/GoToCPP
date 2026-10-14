# ADR-022: Shadowed variables keep their Go names in C++

Status: accepted (2026-10-14). Supersedes the `v := v` row of [ADR-010](010-cpp-naming.md).

## Context

[ADR-010](010-cpp-naming.md) renamed a variable that reads the name it declares: `v := v`
became `int64_t v_1 = v;`, because in C++ a new name is already in scope inside its own
initializer, so `int64_t v = v;` would read garbage. The Phase 02 debug test (step 2.11) showed
the cost: in gdb, `print v` after the declaration printed the **outer** `v`, a wrong value under
the Go name, with no error. Debugging by Go names is a promise of
[ADR-003](003-readable-cpp20-output.md).

## Decision

- Every user variable keeps its Go name (escaped as ADR-010 says) wherever C++ allows it.
- A declaration that shadows a variable or a package function (a local, a parameter, a result,
  a package variable or function) opens a C++ block that runs from the declaration to the end of its Go block. When its value
  reads a name it declares, the value goes into a `gp_` temporary first, outside that block:

  ```cpp
  int64_t gp_t0 = v + 1; { int64_t v = gp_t0;
      // the rest of the Go block
  }
  ```

  The temporary, the `{` and the declaration share one line, so the declaration's line is
  computed outside the new block.

- So gdb shows the outer variable on every line before the declaration, and the inner one after
  it, as in Go. On the declaration's own line it shows the outer one when the value reads the
  name (the temporary is computed outside the block), and the inner, not yet set one otherwise
  (`x := 7 + i`): a temporary for every shadowing declaration (every `if err := f()`) isn't worth
  that one line.
- A rename (`name_1`) stays only as a guard for a second declaration in the same C++ block, which
  the Phase 02 emitter never produces.

## Alternatives

- **Rename (ADR-010)** (superseded): simple, but gdb prints the wrong variable under the Go name.
- **A temporary without the extra block** (rejected): gdb then shows the inner variable on the
  lines of the block *above* the declaration too (garbage, or the previous loop iteration's value).

## Consequences

- More braces in the generated C++ where Go code shadows names (none when the declaration is the
  first thing in its C++ block); the names stay Go's.
- `goto` stays valid: Go rejects a jump over a declaration into its scope, and a jump out of the
  new block is legal C++.
- A class-type temporary (`gp::string`) is moved into the variable, so shadowing costs no extra
  reference count.

## Evidence

- Step 2.11's debug test and the 2.10b review: gdb on g++ 15 and clang++ 21, `print` before, on
  and after the declaration line, in loops and nested blocks.

## See

- [Phase 02 README §2.2](../../plans/2026-10-13-gocpp/phase-02-walking-skeleton/README.md#22-backend-structure-internalcppgen)
  and §2.8; `internal/cppgen/names.go` and `stmt.go`; [cpp-mapping.md](../spec/cpp-mapping.md).
