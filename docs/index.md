# gocpp documentation

gocpp is a programming language that looks and feels like Go. You write Go syntax in `.gp` files,
and the `gocpp` tool translates them into readable C++20 and builds a native program with `g++` or
`clang++`. Because the output is plain C++, a `.gp` file can import a C or C++ header
(`"<vector>"`, `"<sqlite3.h>"`, `"./engine.hpp"`) and call it directly, with no binding code.
Memory is reference counted instead of garbage collected, and gocpp adds a few things in Go style:
must functions (`!`), macros and a language server.

<!-- illustration: a fragment that shows the idea; the tour has the complete programs -->
```go
package main

import (
	"fmt"

	"<vector>"
)

func main() {
	nums := std.vector[int]{5, 3, 8}
	nums.push_back(1)
	fmt.Println("size:", int(nums.size()))
}
```

> **Status.** gocpp is built in phases, and the compiler refuses a feature whose phase is not
> finished, with an error that names the phase. Phases 00 to 05 are finished (`gocpp version`
> prints the last one); generics work from phase 05 on; goroutines (08), the ported standard
> library (09) and must functions and macros (10) work and are behind `GOCPP_GATE_PHASE`. Pages
> and lessons that need it say so. The [feature matrix](spec/feature-matrix.md) has every feature
> with its phase.

## Start here

| I am | Go to |
|---|---|
| new to gocpp | [A Tour of gocpp](tour/README.md): thirteen short lessons, every program checked and run |
| a Go developer | [gocpp for Go developers](guides/go-developers.md): what is different and why |
| a C++ developer | [gocpp for C++ developers](guides/cpp-developers.md): how each construct becomes C++, and how to read it |
| looking for a library call | the [interop guide](spec/interop.md), the [cookbook](spec/interop-cookbook.md) and the [examples walkthrough](guides/interop-examples.md) |
| looking for a package | the [standard library reference](stdlib/index.html), made from the sources |
| in a hurry | the [FAQ](faq.md) |
| after a rule of the language | the [language spec](spec/language.md): Go 1.25 plus the list D1 to D28 |

## Install

```sh
go install github.com/huuhait/gocpp/cmd/gocpp@latest     # needs Go 1.25, and g++ 13+ or clang++ 17+ on the PATH
nix run github:huuhait/gocpp -- run main.gp               # brings its own compilers
```

gocpp runs on Linux (x86-64 tested, arm64 released) and on Windows 10 or later (x86-64); macOS is not a target.
[Installing gocpp](guides/packaging.md) has the platforms, the release archives and the details; the
[FAQ](faq.md) answers the usual questions.

## The pieces

- **The language**: [spec/language.md](spec/language.md) is a list of the differences from the Go
  spec; [spec/cpp-mapping.md](spec/cpp-mapping.md) shows the C++ each construct becomes;
  [spec/modules.md](spec/modules.md) and [spec/build.md](spec/build.md) cover modules and the build.
- **C and C++**: [spec/interop.md](spec/interop.md) (header packages, names, strings, pointers,
  callbacks), [spec/interop-cookbook.md](spec/interop-cookbook.md) (recipes, each one a test) and
  [spec/typed-interop.md](spec/typed-interop.md) (the experimental typed mode,
  `GOCPP_TYPED_INTEROP=1`).
- **Tools**: `gocpp run`, `build`, `test`, `check`, `fmt`, `vet`, `doc`, `explain`, `emit`, `lsp`,
  `get` and `debug`; the [guides](guides/testing.md) have one page for each.
- **Decisions**: [the ADRs](adr/README.md) record why things are as they are.
- **Contributing**: [CONTRIBUTING.md](CONTRIBUTING.md), and [how this site is built](guides/docs-site.md).
