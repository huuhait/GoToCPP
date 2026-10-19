# Hello, gocpp

Every gocpp program is a package `main` with a function `main`. This one prints a line.

<!-- program: 02 id=hello go -->
```go
package main

import "fmt"

func main() {
	fmt.Println("hello, gocpp")
}
```

<!-- output -->
```text
hello, gocpp
```

Save it as `main.gp` in a folder and run `gocpp run .` there. gocpp parses the file, checks the
types, writes C++20, compiles it with `g++` or `clang++` and runs the result. The first run also
builds the runtime library, so it takes longer; the next runs reuse it.

## What to notice

- The program is plain Go. The extension `.gp` is the only difference, and `go run` prints the
  same line from a copy named `main.go`.
- `gocpp emit .` prints the C++ that was generated. It is meant to be read:

<!-- emitted: hello -->
```cpp
void main() {
    gp::fmt::Println("hello, gocpp"_gs);
}
```

  The Go function became a C++ function in a namespace of its package, and the string constant
  became a string literal of the runtime. [gocpp for C++ developers](../guides/cpp-developers.md)
  explains how each construct is mapped.
- `gocpp check .` stops after the type check and needs no C++ compiler for a program like this one. It is what an editor
  runs while you type; see the [editor guide](../guides/editors.md) for the language server.
- `gocpp build .` leaves an executable. It is a normal native program: no gocpp runtime has to be
  installed where it runs.

Next: [Values and control flow](02-values-and-control-flow.md).
