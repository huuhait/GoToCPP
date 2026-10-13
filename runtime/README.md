# runtime

The gocpp runtime: a C++20 library, mostly headers, in `namespace gp`. Generated code uses it for
`gp::string`, `gp::slice`, `gp::map`, `gp::ptr`, interfaces, panics, `defer` frames, channels and
goroutines. It is embedded in the `gocpp` binary (`go:embed`, ADR-009), so there is nothing
extra to install. The `//go:embed` directive is in `runtime/files.go` (package `rtfiles`,
Phase 02), and the driver imports that package: a `//go:embed` pattern can't reach a parent
folder, so the directive must be inside `runtime/`.

It is empty for now. Phase 02 writes runtime v0, and every later phase grows it.

| Folder | What goes there |
|---|---|
| `include/gocpp/` | the headers: `rt.hpp`, which generated code includes (`#include <gocpp/rt.hpp>`), the headers behind it, and `nofortify.h` (below) |
| `src/` | the runtime's `.cpp` files |
| `src/os/linux/` | the Linux OS layer (Phase 02) and the Linux side of the thread layer `gp::os` (Phase 08) |
| `src/os/windows/` | the Windows OS layer and the Windows side of `gp::os` (Phase 14) |

## The OS rule (Phase 02)

The runtime may reach the operating system **only** through `src/os/`: writing to stdout and
stderr, exiting, program arguments and the environment. The same folder holds the thread layer
`gp::os` from Phase 08: starting a thread with a stack size, thread-local storage and the
stack-overflow hook. Everything else uses standard C++ (`std::chrono`, `<atomic>`,
`std::filesystem`), so the same code builds on Linux and Windows. A CI check fails when a POSIX
header is included outside `src/os/`.

## `nofortify.h` (Phase 02)

A Nix shell's compiler wrapper turns on "hardening" by default. It defines `_FORTIFY_SOURCE`
after the user's flags, so `-U_FORTIFY_SOURCE` doesn't help, and every file built without
optimization (`-O0`) then gets a warning. `gocpp/nofortify.h` undefines `_FORTIFY_SOURCE` when
the compiler doesn't optimize (`__OPTIMIZE__` is not defined). `rt.hpp` includes it before
anything else, and the driver passes it with `-include` to the user's own C/C++ files
(Phase 04). The project's dev shell turns hardening off, but a user's Nix shell may not.
