# runtime

The gocpp runtime: a C++20 library, mostly headers, in `namespace gp`. Generated code uses it for
`gp::string`, `gp::slice`, `gp::map`, `gp::ptr`, interfaces, panics, `defer` frames, channels and
goroutines. It is embedded in the `gocpp` binary (`go:embed`, ADR-009), so there is nothing
extra to install.

It is empty for now. Phase 02 writes runtime v0, and every later phase grows it.

| Folder | What goes there |
|---|---|
| `include/gocpp/` | the headers that generated code includes, starting with `rt.hpp` (`#include <gocpp/rt.hpp>`) |
| `src/` | the runtime's `.cpp` files |
| `src/os/linux/` | the Linux OS layer (Phase 02) |
| `src/os/windows/` | the Windows OS layer (Phase 14) |

## The OS rule (Phase 02)

The runtime may reach the operating system **only** through `src/os/`: writing to stdout and
stderr, exiting, program arguments, the environment, and threads from Phase 08. Everything else
uses standard C++ (`std::chrono`, `<atomic>`, `std::filesystem`), so the same code builds on
Linux and Windows. A CI check fails when a POSIX header is included outside `src/os/`.
