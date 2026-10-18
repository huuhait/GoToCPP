# tests/zoo

The **header zoo** (Phase 04, step 4.18): one program per header or library, each importing it
in opaque mode and using it in a few typed ways. `TestZoo` ([../zoo_test.go](../zoo_test.go))
checks every file with `gocpp check` (what an editor sees), builds it with `g++` and `clang++`,
and runs it against its `// Output:` block, like [tests/interop](../interop/README.md).

| File | Header or library |
|---|---|
| `stdio_h`, `stdlib_h`, `string_h`, `math_h`, `time_h`, `errno_h` | the C headers `<stdio.h>`, `<stdlib.h>`, `<string.h>` (imported as `cstring`), `<math.h>`, `<time.h>`, `<errno.h>` |
| `vector`, `string`, `map`, `unordered_map`, `memory`, `algorithm`, `filesystem`, `chrono`, `thread`, `optional`, `variant`, `span`, `regex` | the C++ standard headers of the same name |
| `sqlite3`, `zlib` | libraries found with `pkg-config` (skipped when pkg-config does not find them) |
| `sdl2`, `raylib` | **build only**: the file says `// display: required`, which in the zoo means that the program is checked and linked but never run (it would open a window) |

A new file needs the header lines of the other test kinds (`// phase: 04`, optional
`// pkg-config: lib`, `// display: required`) and a `// Output:` block unless it is build only.
Keep the usages deterministic: no clock readings, no hash-table order, no thread timing.
