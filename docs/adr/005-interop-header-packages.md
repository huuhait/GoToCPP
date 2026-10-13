# ADR-005: C/C++ headers and C++ namespaces are used like packages

Status: accepted (2026-10-13)

## Context

A `.gp` file must be able to use any C or C++ header with no glue code. cgo puts every C name
under `C.` and needs an inline preamble, and C++ is too big to model inside gocpp (templates,
overloads, ADL, macros). The user decided that C/C++ should look like Go packages
([Q3](../../plans/2026-10-13-gocpp/TODO.md#decisions-answered-2026-10-13)).

## Decision

- **A header is a package named after its file**: `"<SDL2/SDL.h>"` → `SDL`, and the local
  header `"./physics.h"` → `physics`. Its members are the header's C-level names: functions,
  types, globals, enum constants and macros (`raylib.InitWindow`, `raylib.RAYWHITE`). Rename it
  like a Go import (`import rl "<raylib.h>"`) or import it with `_`. A file name that is a Go
  keyword (`<map>`), a predeclared name (`<string.h>`) or already taken (`<math.h>` next to
  `"math"`) gives no name; then name it yourself: `import cstring "<string.h>"`.
- **C++ namespaces are packages and need no import**: `std.vector[int]`, `std.chrono.seconds`.
  An unnamed `import "::std"` is an error; to rename a namespace, write
  `import fs "::std::filesystem"`. A small compiler run, the namespace probe, checks the names,
  so a typo like `stdd` is a normal `undefined: stdd` error.
- **`C` keeps only C's built-in types and the boundary helpers**, as in cgo: `C.int`, `C.char`,
  `C.size_t`, `C.CString`, `C.GoString`, `C.cstr`. It works in every `.gp` file without an
  import, and a declaration named `C` is an error (D19).
- **Opaque first**: gocpp does not type-check C/C++ names; the C++ compiler does. Phase 12 adds
  real types from clang, and typed mode becomes the default once it passes the Phase 04 suite.

```go
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>" // package sqlite3
	"<vector>"    // makes the namespace std usable
)

func main() {
	fmt.Println("sqlite", C.GoString(sqlite3.sqlite3_libversion()))
	nums := std.vector[int]{1, 2, 3} // a real std::vector<int64_t>
	nums.push_back(4)
}
```

## Alternatives

- **Every C/C++ name under `C.`** (cgo style, the plan's first design; rejected by Q3):
  `raylib.InitWindow` reads like using a Go package.
- **A cgo-style inline preamble** (rejected for v1): a small helper goes into a header next to
  the `.gp` file instead (`"./mathx.h"` → `mathx.hypot3(1, 2, 2)`), and `.c`/`.cpp` files there
  are compiled too. That keeps one way of doing it, and the helper gets real C++ editor support.
  The preamble stays on the backlog.
- **Mandatory bindings** (rejected): they can't cover all of C++.

## Consequences

Good: almost any header, library or template works from day one, and the code looks like Go.

Bad:
- Until Phase 12, C/C++ mistakes are found late, by the C++ compiler (at the right `.gp` line),
  and two things don't work: promoted members of an embedded C++ type, C macros as constants.
- C++ has one global namespace, so `raylib.printf` compiles if `<stdio.h>` is included anywhere.
- C/C++ values follow C++ rules (D14): `m["k"]` on a `std::map` inserts, no bounds checks; and
  `range` over a C++ container or a C array gives C++'s element types (D21).
- Untyped constants become C literals with C's types: write `std.max(n, int(0))`.

## Evidence

- Spike 2026-10-13 with this import model: header packages (`SDL`, `cstdio`, `physics`) and
  namespaces (`std`, `std.filesystem`) type-checked with 0 false errors, and the namespace probe
  (clang++ and g++) rejected `stdd` and `fmtt` ([frontend](../../spikes/2026-10-13/frontend/)).

See: [Phase 01 §2.3](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#23-import-classification), [§2.5](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#25-preprocessing-before-gotypes-proven-by-the-spike), [Phase 04 §1](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#1-idea), [§2.3](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#23-name-translation), [§2.10](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md#210-small-cc-helpers-a-header-next-to-the-gp-file), [Phase 12 §2.8](../../plans/2026-10-13-gocpp/phase-12-typed-interop/README.md#28-requirements-and-modes), divergences [D2, D14, D19, D21](../../plans/2026-10-13-gocpp/phase-00-foundations/README.md#22-the-language-as-a-diff-against-go).
