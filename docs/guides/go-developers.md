# gocpp for Go developers

gocpp is Go. A `.gp` file has Go's syntax, Go's types, Go's standard library (most of it is Go's
own source, ported) and Go's tools by other names. The language is the Go 1.25 spec with a short
list of differences, and this guide is that list with the reasons. The exact text, with examples
and the phase of each item, is in the [language spec](../spec/language.md), which numbers the
differences D1 to D28.

If your program uses only the basics, rename `main.go` to `main.gp` and run `gocpp run .`. The
other direction works too for programs that use none of the additions: the lessons of the
[tour](../tour/README.md) that say "valid Go" print the same with the `go` command, and the tests
check it.

## Why gocpp, if you have Go

You write gocpp when the program lives next to C or C++: you call a C or C++ library with no
binding code and no cgo, you ship a native binary that links with a C++ project, or you want
Go's syntax over a C++ toolchain. If you do not need that, use Go.

## What is the same

Packages, `:=`, multiple results, structs, methods, interfaces and embedding, slices and maps,
`defer`, `panic` and `recover`, closures with Go 1.22 loop variables, generics and range over
functions, goroutines, channels and `select`, `fmt` with Go's verbs and its output, and the packages
`errors`, `sort`, `strings`, `strconv`, `bufio`, `os`, `regexp`, `time` and `context`, which are Go's
own source (the standard library is a subset: [what is not there](#what-is-not-there)),
`_test.gp` files with `gocpp test`, and `gocpp fmt` which formats exactly like gofmt. The test suite runs many programs with both gocpp and the real Go, and compares
their output.

## What is different

| Topic | Go | gocpp | Spec |
|---|---|---|---|
| Memory | a garbage collector | reference counting; a cycle of pointers leaks, `weak.Pointer` breaks it | [D1](../spec/language.md#d1-reference-counting-instead-of-a-garbage-collector) |
| Goroutines | millions of cheap goroutines | one OS thread each; thousands are fine, millions are not | [D7](../spec/language.md#d7-goroutines-are-os-threads) |
| Data races | undefined behavior in practice | the same, and a race can corrupt a reference count; `gocpp run -race` finds them; swap a shared pointer with `atomic.Pointer` or a lock, not with a plain assignment | [D10](../spec/language.md#d10-data-races-can-break-reference-counts), [race guide](race.md) |
| `reflect` | everything | a subset: enough for `fmt`, `encoding/json`, `sort`, `errors`, `context`; no `MakeFunc`, `Value.Call`, `plugin` | [D6](../spec/language.md#d6-a-reflect-subset-and-no-plugin) |
| Sizes | `string` 16 bytes, slice 24, pointer 8 | `string` 32, slice 40, pointer 16; `unsafe.Sizeof` shows it | [D15](../spec/language.md#d15-different-sizes) |
| Word size | 32 or 64 bits | 64 bits only | [D9](../spec/language.md#d9-64-bit-targets-only) |
| C | `import "C"` and a cgo preamble | `import "<stdio.h>"`: a header is a package; C++ namespaces are packages | [D2](../spec/language.md#d2-header-imports-instead-of-cgo) |
| Build directives | `#cgo` lines | `//gocpp:pkg-config`, `//gocpp:link` and `gocpp.mod` | [D11](../spec/language.md#d11-gocpp-directives-instead-of-cgo-lines) |
| C and C++ values | Go's rules | C++'s rules (a `std::map` read inserts; a `std::vector` is not bounds checked) | [D14](../spec/language.md#d14-cc-values-follow-c-rules) |
| Panic in a callback | unwinds through the C frames | ends the program at the callback | [D27](../spec/language.md#d27-a-panic-that-leaves-a-callback) |
| Large values | the stack grows | a result or a temporary over 128 KiB can overflow the C++ stack | [D24](../spec/language.md#d24-large-values-on-the-c-stack) |
| `println` of a slice or map | prints addresses | an error: use `fmt.Println` | [D23](../spec/language.md#d23-print-and-println-of-a-slice-or-a-map) |

Go keywords can follow a dot (`node.type()`, D3), `T(a, b)` can call a C++ constructor (D5), and
`range` works over C++ containers (D21). The remaining items (D4, D8, D16 to D22, D25, D26, D28) are
corners that the spec lists with their examples.

<!-- program: 03 id=sizes -->
```go
package main

import (
	"fmt"
	"unsafe"
)

func main() {
	var s string
	var xs []int
	var p *int
	fmt.Println(unsafe.Sizeof(s), unsafe.Sizeof(xs), unsafe.Sizeof(p))
	// Go prints 16 24 8.
}
```

<!-- output -->
```text
32 40 16
```

## What gocpp adds

Three additions, each of them optional and each of them off unless you write the syntax:

- **Header imports** make C and C++ libraries ordinary packages: [tour 9](../tour/09-cpp-containers.md)
  and [tour 10](../tour/10-c-libraries.md), the [interop guide](../spec/interop.md) and the
  [cookbook](../spec/interop-cookbook.md).
- **Must functions** (`func f!() T`, `F!()`) return errors for you. It is sugar for the early returns
  you would write; code without `!` is plain Go ([tour 11](../tour/11-must-functions.md),
  [D12](../spec/language.md#d12-must-for-errors)).
- **Macros** (`macro`, `#Assert(...)`, `#Derive(String)`) run at compile time and know the place
  and the text of their use ([tour 12](../tour/12-macros.md), [D13](../spec/language.md#d13-macros)).

> **Experimental today.** Goroutines are phase 08 and the ported standard packages are phase 09. The
> compiler refuses a feature of a phase that is not finished, with an error that names the phase. For
> your own development set `GOCPP_GATE_PHASE` to the phase (`GOCPP_GATE_PHASE=09 gocpp run .` opens
> everything up to 09). The syntax of must functions and macros (phase 10) needs no switch;
> `gocpp version` prints the last finished phase, and the [feature matrix](../spec/feature-matrix.md)
> lists each feature with its phase. Typed interop is on by default when
> clang++ is installed (`-interop=opaque` or `GOCPP_INTEROP=opaque` turns it off): gocpp asks clang++ what the
> C and C++ names are and reports mistakes in them early. [typed interop](../spec/typed-interop.md).

## What is not there

- **The standard library is a subset of Go's.** gocpp has 51 packages (plus `unsafe`, which the
  language provides), and the [standard library reference](../stdlib/index.html) is generated from them.

  <!-- stdlib: present bufio bytes cmp container/heap container/list container/ring context embed encoding encoding/base64 encoding/binary encoding/hex encoding/json errors flag fmt hash hash/crc32 hash/fnv io io/fs iter log maps math math/bits math/rand math/rand/v2 os path path/filepath reflect regexp regexp/syntax runtime runtime/cgo runtime/debug slices sort strconv strings sync sync/atomic syscall testing text/tabwriter time unicode unicode/utf16 unicode/utf8 weak -->
  `bufio`, `bytes`, `cmp`, `container/heap`, `container/list`, `container/ring`, `context`, `embed`, `encoding`,
  `encoding/base64`, `encoding/binary`, `encoding/hex`, `encoding/json`, `errors`, `flag`, `fmt`, `hash`,
  `hash/crc32`, `hash/fnv`, `io`, `io/fs`, `iter`, `log`, `maps`, `math`, `math/bits`, `math/rand`, `math/rand/v2`, `os`, `path`, `path/filepath`, `reflect`, `regexp`,
  `regexp/syntax`, `runtime`, `runtime/cgo`, `runtime/debug`, `slices`, `sort`, `strconv`,
  `strings`, `sync`, `sync/atomic`, `syscall`, `testing`, `text/tabwriter`, `time`, `unicode`, `unicode/utf16`,
  `unicode/utf8` and `weak`.

  <!-- stdlib: stubs embed -->
  `embed` is a declaration stub today: a program that imports it type-checks, but it has no
  implementation yet.

  <!-- stdlib: native-stubs fmt weak -->
  `fmt` and `weak` have declaration stubs too, but the compiler/runtime implements them itself.

  `fmt` is deliberately a native print/scan engine, not a port of Go's package through `reflect`
  ([ADR-100](../adr/100-fmt-stays-native-decision-9-7-a.md)).

  The [stdlib status guide](stdlib.md) lists every public and internal package with its source,
  target selection, tier and Go-test manifest coverage. It also describes the smaller `runtime`
  and `os` surfaces, fixed stacks, timezone sources, and other [library differences](stdlib.md#what-differs-from-go).
  A package being present does not imply that every Go API is implemented; in particular
  `os.Root` and process creation are not built. Use the [porting guide](porting.md) for adding a package.

  The [feature matrix](../spec/feature-matrix.md) says from which phase each package works; several
  of them (for example `strings` and `bufio`) need `GOCPP_GATE_PHASE=09` today.
- **Packages you may expect and will not find** (`gocpp check` reports `package X is not available
  yet`, whatever phase the message names):

  <!-- stdlib: missing text/template net/url math/big hash/maphash hash/adler32 hash/crc64 testing/iotest image mime crypto crypto/sha256 crypto/md5 crypto/sha1 crypto/rand os/user html net net/http os/exec os/signal plugin encoding/csv compress/gzip archive/zip -->
  `text/template`, `net/url`, `math/big`,
  `hash/maphash`, `hash/adler32`, `hash/crc64`, `testing/iotest`, `image`, `mime`, `crypto` and its
  subpackages (`crypto/sha256`, `crypto/md5`, `crypto/sha1`, `crypto/rand`), `os/user`, `html`,
  and the packages that are not planned or come much later: `net`, `net/http`, `os/exec`,
  `os/signal`, `plugin`, `encoding/csv`, `compress/gzip` and `archive/zip`. The tiers of the plan
  are in the [Phase 09 README](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md).
  C++ libraries can stand in: any header you can include is a package.
- `//go:linkname` in your code, `reflect.MakeFunc` and the rest of the list in D6, and generic types
  declared inside a function (declare them at package level).
- cgo: use header imports. A cgo program changes in three places: `import "C"` and its preamble
  become imports of headers (a helper that was written in the preamble goes into a header or a
  `.c` file next to the `.gp` file), `C.func(...)` becomes `header.func(...)`, and the `#cgo`
  lines become `//gocpp:` directives.

## The tools

| Go | gocpp | |
|---|---|---|
| `go run .`, `go build`, `go test`, `go vet` | `gocpp run .`, `gocpp build`, `gocpp test`, `gocpp vet` | same flags where they make sense; the build compiles C++ ([build spec](../spec/build.md)) |
| `gofmt` | `gocpp fmt` | same output |
| `go doc` | `gocpp doc` | same text, byte for byte ([guide](doc.md)) |
| `go list` | `gocpp list` | |
| `go get`, `go.mod`, `go.sum` | `gocpp get`, `gocpp.mod`, `gocpp.sum` | modules of Go and of C++ projects; reads `go.mod` of dependencies ([guide](modules.md)) |
| `go env` | `gocpp env` | also shows the C++ compiler and the cache |
| `go clean` | `gocpp clean` | |
| `gopls` | `gocpp lsp` | a language server; see the [editor guide](editors.md) |
| the compiler's errors | `gocpp check`, `gocpp explain` | errors in Go's format; `explain` says what a construct or message means ([guide](explain.md)) |
| (none) | `gocpp emit` | prints the generated C++ ([gocpp for C++ developers](cpp-developers.md)) |

Compile time is the thing you will notice first. gocpp builds C++, so a first build also compiles the
runtime (once per compiler and flags, then cached), and generic code costs C++ compile time:
[the guide](generics-compile-time.md) has the numbers and the cheap ways down.

## Next

The [tour](../tour/README.md) shows every point above in a program. For a larger program in your
own style, read the [examples](../../examples/README.md): `wc` and `grep-lite` are command line
tools with tests, `workers` is goroutines, `shapes` is interfaces.
