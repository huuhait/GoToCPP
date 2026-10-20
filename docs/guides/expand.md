# `gocpp expand`: see what macros, `#Derive` and `!` become

Phase 10 (step 10.11). A `!`, a macro and a `#Derive` line are short to write, and each one is a
piece of plain Go that the compiler writes for you. `gocpp expand` prints that Go, so you can see
what your program does ([ADR-091](../adr/091-expand-prints-the-go-the-compiler-works-with.md)).

```
gocpp expand [-full] [-tags list] [-os goos] [-arch goarch] [-color mode] [-excerpt mode] [file.gp... | package]
```

The arguments are those of `gocpp check`: a folder, a list of `.gp` files, an import path, or a
pattern with `...`; without arguments it takes the package in the current folder. It loads and
checks the package like `gocpp check` does, and prints the errors the same way (exit status 1,
nothing on standard output). The macros, `#Derive` and `!` are always available, like in every gocpp
command: no environment variable opens them ([ADR-095](../adr/095-must-and-macros-are-always-available.md)).

## An example

```go
package main

import (
	"fmt"
	"os"
)

// Assert panics with the place of the call when cond is false.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(#file() + ":" + fmt.Sprint(#line()) + ": " + #text(cond) + ": " + msg())
	}
}

#Derive(Equal)
type Point struct{ X, Y int }

func size!(path string) int64 {
	f := os.Open(path)
	defer f.Close()
	info := f.Stat()
	#Assert(info.Size() >= 0, "negative size")
	return info.Size()
}

func main!() {
	fmt.Println(size!("go.mod"))
}
```

`gocpp expand main.gp` prints

```go
package main

import (
	"fmt"
	"os"
)

import gp_must "gocpp/must"

// Assert panics with the place of the call when cond is false.
func Assert[gp_LN_msg ~func() string](gp_file string, gp_line int, gp_func string, gp_text_cond string, cond bool, msg gp_LN_msg) {
	if !cond {
		panic(gp_file + ":" + fmt.Sprint(gp_line) + ": " + gp_text_cond + ": " + msg())
	}
}

type Point struct{ X, Y int }

func (p Point) Equal(other Point) bool { return p == other }

func size(path string) (must_r int64, must_r_1 error) {
	f, must_e1 := os.Open(path)
	if must_e1 != nil {
		return 0, must_e1
	}
	{
		must_t1 := f.Close
		defer func() {
			if must_e2 := must_t1(); must_e2 != nil && must_r_1 == nil {
				must_r_1 = must_e2
			}
		}()
	}
	info, must_e3 := f.Stat()
	if must_e3 != nil {
		return 0, must_e3
	}
	Assert("main.gp", 22, "main.size", "info.Size() >= 0", info.Size() >= 0, func() string { return "negative size" })
	return info.Size(), nil
}

func must_main() error {
	{
		must_t2, must_e4 := size("go.mod")
		if must_e4 != nil {
			return must_e4
		}
		_, must_e5 := fmt.Println(must_t2)
		if must_e5 != nil {
			return must_e5
		}
	}
	return nil
}

func main() { gp_must.Exit(must_main()) }
```

## What each form becomes

| You write | `gocpp expand` shows |
|---|---|
| `F!(args)` | the call, its error in a variable (`must_e1`), and the `if` that returns it (with zero values, the defers run), panics or exits, as the function around it decides ([language.md](../spec/language.md#must-functions-and-must-calls)). The value goes into a temporary (`must_t1`) when it is used in a larger expression, and the code is placed where Go's order of evaluation needs it |
| `func f!() T` | `func f() (T, error)`; every call whose error is dropped has the code above; `return x` is `return x, nil` |
| `func main!()` | `must_main() error` and a `main` that calls `gp_must.Exit(must_main())`, which prints `<program>: <error>` and exits with 1 (the one helper that is not Go: its import is printed after yours) |
| `macro Name(...)` | a generic function of the same name with the hidden parameters `gp_file`, `gp_line`, `gp_func` (the place of the use) and `gp_text_x` (the text of a parameter that the macro gives to `#text`); a `lazy` parameter is a type parameter `gp_L_x` (`gp_LN_x` when the macro only calls it) constrained to a `func() T` ([ADR-075](../adr/075-user-macros-are-generic-functions.md)) |
| `#Name(args)` in code | a call of that function, with the place of the use as the first arguments and a `lazy` argument as a `func() T { return arg }` |
| `#Name(args)` on top of a func (start style) | the first statement of the function: `if gp_defer1 := Name(...); gp_defer1 != nil { defer gp_defer1() }` for a macro that returns `func()`, an ordinary call for any other |
| `#Name(args)` on top of a func (wrap style, the macro uses `#body()`) | the code of the function moved into a func literal, which the macro runs (`gp_body()`); the function gets the results `gp_r0`, `gp_r1`... |
| `#Derive(String, Equal, Clone)` | the methods, with their bodies, after the type ([ADR-080](../adr/080-derive-writes-methods-from-the-types.md)) |
| a `#file()`, `#line()`, `#func()`, `#text(x)`, `#embed(...)` | the string or number that the compiler puts there (in a macro's body: the hidden parameter) |

Names that start with `gp_` or `must_` are the compiler's: they are chosen not to clash with yours
(`must_t2` if you have a `must_t1`).

## What is printed as you wrote it

The output is **your file with the expanded declarations replaced**. A declaration that has no macro, no
`#Derive` line and no `!` in it, and everything between declarations (comments, blank lines, spacing, line
ends, a byte order mark, a last line without a line end), is your file's text byte for byte: a file that
has nothing to expand comes back identical. A declaration that has one is printed by the Go printer
(gofmt's layout) with the comments that were in it, so a macro's declaration or a must function is
formatted, not copied; the comments above it (its documentation, and the ones on the lines of macros) are
copied above it as they are. When other declarations are on the same line (`var a = #Ex(1); var b = 2`),
only the expanded one is replaced. A `#Derive(...)` line is taken out (a comment on its line stays) and
the methods follow the type; the `main`
that calls `must_main` follows it, and the `import gp_must "gocpp/must"` line follows your last import
(or the package clause).

With more than one file (a package, or `./...`), each file starts with a line `//// name` (the path from
the current folder, with slashes also on Windows), in the order of the files, and the packages in the
order of the patterns. With one file there is no such line, so the output can be saved to a `.go` file
and read, diffed against the original, or run. `_test.gp` files are not part of the package that
`gocpp expand` loads and are not printed. A standard package (`gocpp expand errors`) is printed from the
copy embedded in gocpp.

The comments inside an expanded declaration are printed with it, but they can move: a comment goes before
the code that stands for the statement it was written before, and a few lines of that code (the loop
condition that `!` hoists, for example) can read a little oddly (`m, err :=` and the call on the next
line, a blank line after a brace). None is lost.

## Is it Go that runs?

Yes, for a program that uses no C or C++ and no `main!` (which needs `gp_must.Exit`) the output
is Go that `go run` builds (an `#embed` that is longer than 200 bytes is shortened in the output, so
use `-full` to run it, or to get it whole), and it does what the `.gp` program does: the tests run the expansion
of every must, macro and derive program of `tests/run` with real Go and compare its output with
the program's `// Output:`. That is also the definition of the output being right: it is the
Go that the compiler would otherwise write for you, not a picture of it. The C++ that gocpp makes from it
is what `gocpp emit` prints (the `!` and the macros are not special in the lowering).

C and C++ names stay as you wrote them (`std.vector[int]`, `int(nums.size())`): `gocpp expand`
turns the compiler's own forms for them (`C.std.vector`, `gp_cgoh.As[int](x)`) back into your
text, in the declarations it prints from the tree.

## In the editor

The language server has the same command as a code action, *Show expansion* (on a declaration that has a `!`, a
macro use or a `#Derive` in it): it prints this output for the file you are editing, from the text in the editor, and shows
it in a document of its own with the declaration selected ([the editors guide](editors.md#-macros-and-derive)). It uses the
same printer (`internal/expand`), so the two can not differ. The hover on a `!` shows the part of it that stands for one statement.

## Limits

- The output is for reading. It parses as Go, but it may not compile as it is: `gp_must` is not
  a package of Go, an `errors.Wrapf` is gocpp's own (divergence D17), and a program that uses C++
  has no Go meaning.
- Only the packages named on the command line are printed. A macro of another package is
  expanded in your package (a call of `util.Check(...)` with its hidden arguments); the macro's own
  declaration is printed when you name that package.
- The comments inside a declaration that was expanded may move by a statement: a comment is placed
  before the first token that comes after it in your source, and the code that `!` wrote takes the
  place of the statement it comes from.
