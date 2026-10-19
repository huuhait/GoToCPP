# Where to go next

You have seen the language from a first program to C++ containers and macros. These are the places
to continue.

## If you know Go

[gocpp for Go developers](../guides/go-developers.md) lists what is different (reference counting,
threads, a reflect subset, 64-bit only) and links each difference to its place in the spec.

## If you know C++

[gocpp for C++ developers](../guides/cpp-developers.md) shows how each construct is mapped to C++,
how to read the generated code, and how the two worlds meet at the border.

## Build real programs

| I want to | Read |
|---|---|
| split a program into packages, use `gocpp.mod` and `gocpp get` | [modules guide](../guides/modules.md) and [spec](../spec/modules.md) |
| write tests with `_test.gp` files and `gocpp test` | [testing guide](../guides/testing.md) |
| call C and C++ libraries (SQLite, zlib, SDL2, raylib, your own code) | [interop guide](../spec/interop.md), [cookbook](../spec/interop-cookbook.md), [examples walkthrough](../guides/interop-examples.md) |
| set up an editor with completion and diagnostics | [editors guide](../guides/editors.md) |
| find a bug with `gocpp vet`, `gocpp debug`, `-race` or `-asan` | [vet](../guides/vet.md), [debugging](../guides/debugging.md), [race](../guides/race.md) |
| build for Windows from Linux | [Windows guide](../guides/windows-cross.md) |
| look up a package | [standard library reference](../stdlib/index.html) |
| understand an error message | `gocpp explain` ([guide](../guides/explain.md)) |

## The language in full

The [language spec](../spec/language.md) is a list of the differences from Go's spec, so everything
else is the Go spec. The [feature matrix](../spec/feature-matrix.md) says from which phase of the
plan each feature works.
