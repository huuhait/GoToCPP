# Embedding files and the built-in macros

Phase 10 (step 10.8). A gocpp program can put the bytes of a file into the binary, and it can
ask the compiler where a piece of code is. Both are constants that the compiler writes into the
code, so they cost nothing at run time.

## Embedding a file: `#embed` and `//go:embed`

```go
package main

import (
	_ "embed"
	"fmt"
)

//go:embed banner.txt
var banner string // the bytes of banner.txt

//go:embed logo.png
var logo []byte // the bytes of logo.png

func main() {
	fmt.Print(banner)
	fmt.Println(len(logo), len(#embed("banner.txt"))) // #embed: a string constant
}
```

- **`//go:embed pattern`** on a package-level variable of type `string` or `[]byte` gives it the
  bytes of the file. It needs `import _ "embed"`. The rules are Go's: the pattern is a path with
  slashes, relative to the folder of the source file, written with the wildcards of `path.Match`
  (`logo.*`); a name with a space goes in quotes (`//go:embed "my file.txt"`); the pattern must match
  one file for a `string` or `[]byte` (a folder with exactly one file does). No `.`, `..` or absolute paths, no symbolic links, no folder
  with its own `gocpp.mod`. The variable has no initializer.
- **`#embed("file")`** is an expression: an untyped string constant with the bytes of the file. It
  works where a constant works (`const banner = #embed("banner.txt")`, `len(#embed("x"))`), and the
  file is named from the folder of the source file like a `//go:embed` pattern, but it is one file
  name, not a pattern.
- A file is at most **16 MiB** and a package embeds at most **64 MiB**; a bigger file is an error
  that says so. The C++ compiler reads the bytes as a string literal: a 10 MiB file is about 30 MB
  of C++ and about a second and 300 MB of memory; a package at the 64 MiB limit takes 6 to 11
  seconds and 1.4 to 1.8 GB. Use `var`, not `const`, for a big file: a constant is copied at every
  use that is not folded.
- The compiler reads the files when it builds, through the same file system as the sources, so a
  changed, added or removed file rebuilds the program.
- A variable of type `embed.FS` waits for `io/fs` (Phase 09): `//go:embed` on one is the error
  "embed.FS is not available yet".

The errors read like `go build`'s, at the pattern: `main.gp:5:12: pattern nope.txt: no matching files
found`, `pattern ../x: invalid pattern syntax`, `go:embed cannot apply to var of type int`,
`go:embed requires import "embed" (or import _ "embed", if package is not used)`. Errors about the
variable (its type, a second file) are reported at its name, as `go build` does.

## Where am I: `#file()`, `#line()`, `#func()`

```go
func Log(msg string) { fmt.Printf("%s:%d %s: %s\n", #file(), #line(), #func(), msg) }
```

These three describe the place where they are **written**: `#file()` is the path from the root of
the module (`util/util.gp`), or the file name for a program without `gocpp.mod`; `#line()` is an
`int`; `#func()` is the name that Go's runtime prints (`main.f`, `main.(*T).M`, `main.f.func1` for
a closure, `main.init.0` for an `init`, `main.F[...]` for a generic function). They are untyped
constants, so `const here = #line()` and `[#line() % 7]int` work. A `//line` comment changes `#file()` and
`#line()`; the name is the module-relative or the base name of what the comment says (`??` when it has none),
which can differ from what Go's runtime prints for a relative or an absolute name. Inside a macro that you write (step 10.9) they describe the place where the
macro is used.

## The text of an expression: `#text(x)`

```go
check(a+b == 8, #text(a+b == 8), #line()) // the message can say "a+b == 8"
```

`#text(x)` is the source text of `x` as you wrote it, as a string constant. Where `x` spans several
lines, each line break and its indentation, and each comment, become one space; strings, raw strings
and everything else stay as they are. `x` is not evaluated and not checked: it can use names that
do not exist. Because the argument is dropped, a variable that only `#text(v)` uses is "declared and not used".
`gocpp fmt` formats the argument like any code, so it can change the value of `#text` (`a + b` becomes `a+b` in
the arguments of a call). In the package-level variables of a package, the closures are numbered in declaration
order for `#func()`, where Go numbers them in initialization order; and for a folder with a dot in its name,
`#func()` gives Go's escaped `my%2epkg` where `runtime.Caller` gives it unescaped.

## In the tools

`gocpp fmt` keeps these macros as written, and the generated C++ keeps the line of each use for the
debugger and the C++ compiler's messages. Before Phase 10 is finished these macros and
`//go:embed` are gated: `gocpp` says "macros are not supported yet (Phase 10)".
