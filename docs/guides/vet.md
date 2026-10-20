# gocpp vet

`gocpp vet` reports likely mistakes in a package: the checks of `go vet` that make sense for
gocpp programs, and checks of gocpp's own. It is built on `golang.org/x/tools/go/analysis`, so
the checks are the same code that `go vet` runs, over the type information that gocpp's front-end
produces for your `.gp` files. Findings point at the `.gp` source.

```
gocpp vet [-json] [-v] [-list] [-<analyzer>[=false]] [packages]
```

`packages` are as for `gocpp check` (a folder, a list of `.gp` files, an import path, a pattern
with `...`); without them, the package in the current folder is vetted, and its `_test.gp`
files too. Each finding is one line, `file:line:col: message`, under a `# <import path>` line,
on standard error, in a fixed order (package, file, line, column, analyzer, message).

| Exit status | Meaning |
|---|---|
| 0 | nothing found |
| 1 | at least one finding |
| 2 | the packages can't be vetted (load or type errors, a bad flag), or an analyzer crashed |

`-<analyzer>` runs only the analyzers you name; `-<analyzer>=false` leaves one out (as in
`go vet`). `-json` prints the findings as JSON on standard output, by package and analyzer, and
exits with 0 when it could vet; each finding has `posn`, `message`, and `end` when the analyzer gives one. `-list` shows the analyzers. `-v` also lists what was skipped
and why.

Only the packages you name are reported on. Their imports are analyzed too, because some
analyzers learn from them: `printf` finds out that `log.Infof(format, args...)` forwards to
`fmt.Printf`, also across packages, and then checks the calls of `log.Infof`.

## The analyzers

From `go vet`: `appends`, `assign`, `atomic`, `bools`, `composites`, `copylocks`, `defers`,
`errorsas`, `ifaceassert`, `lostcancel`, `nilfunc`, `nilness`, `printf`, `shift`, `stdmethods`,
`stringintconv`, `structtag`, `tests`, `timeformat`, `unreachable`, `unsafeptr`, `unusedresult`,
`waitgroup`.

Not taken, with the reason: `loopclosure` (a loop variable is per iteration, as in Go 1.22);
`asmdecl`, `buildtag`, `cgocall`, `directive`, `framepointer`, `httpresponse`, `sigchanyzer` (no
assembly, build tags, cgo or these packages here); `slog`, `stdversion`, `hostport`, `unmarshal`,
`testinggoroutine` (they need packages or version information that gocpp's standard library
does not have yet; they can be added later the way the others were). The per-analyzer flags of
`go vet` (`-printf.funcs`, ...) don't exist: the analyzers run with their defaults.

- `copylocks` works by type name (`sync.Mutex`, `sync.WaitGroup`, ...), so it checks the
  `sync` package of gocpp as soon as a program uses it.
- `tests` also checks `_test.gp` files (a test is named `TestX`, with the right signature).

Of gocpp's own:

- **`cycle`**: reference cycles. gocpp has no garbage collector: values are reference counted
  (ADR 004), so objects that keep each other alive are never freed. `cycle` reports, in one
  function, a closure that captures the variable that holds it
  (`var f func(); f = func() { f() }`, also through a decorator: `g = twice(func() { g() })`),
  a struct whose field holds a closure that captures the struct (`s.cb = func() { s.run() }`),
  a map, slice or channel that holds a closure or a pointer that holds the map, slice or
  channel again (`ch <- func() { len(ch) }`, `w.out <- w`), a method value with a pointer
  receiver stored in the object it binds, and objects with pointers to each other (a parent and
  its child, also built with `c := &N{parent: p}`). The message shows the cycle and says how to
  break it. Not reported: a method value of a value receiver (it copies the receiver); `p = q`
  (it moves a pointer); a cycle that is ended in the function, by setting the target to `nil`
  in a statement that always runs after the store (at the top level, or deferred, or at the top
  level of the stored closure: `h.f = func() { h.f = nil; ... }`); and a field that a method of
  the package sets to `nil` (`func (s *S) Close() { s.cb = nil }`). A `nil` in an `if` or a loop,
  or before the store, does not end the cycle: it may never run. The analysis is per function:
  a cycle through several functions is not found.
- **`reserved`**: a name that C++ reserves (`__x`, `a__b`, `_X`). gocpp writes it as it is, but
  a C++ header may define a macro with that name.

Of gocpp's own, about `!` and macros (they work on the programs with Phase 10 syntax, which needs no switch; a package without
`!` and macros has nothing for them to find):

- **`panicbang`**: a `!` that panics inside a library. What `!` does on an error depends on the
  function that holds it: a function whose last result is `error` returns it, a function with a
  `*testing.T` parameter calls `t.Fatal`, `main!` and `init!` print it and exit, and **any other
  function panics** with the error (Go's `Must` convention). That is right in a program and wrong
  in a library, which should hand its errors to the caller. So in a package that is not `main`
  (and not in a `_test.gp` file), `panicbang` reports each `F!(...)` of a function with no error
  result, at the `!`; the message names the function and says what to do:
  `strconv.Atoi!("80") panics here: (*Server).Start has no error result, so an error from the
  call becomes a panic. A library should return its errors: give it an error result, or declare
  it with a ! after its name so that it returns the error`. It also follows the call graph across packages: a call in
  such a package of a function **of another package** that panics through a `!` of its own,
  directly or through the functions it calls, is reported at the call, with the place of the `!`
  (`dep.Parse can panic: dep.parse uses ! at dep.gp:6:9`). So the library you wrote hears about
  the `!` of the library you use, although `dep` is not a package you vet. The calls followed are
  the static ones (a call through an interface or a func value is not), and a function that calls
  `recover()` stops the chain (and its own `!` are not reported: it may stop the panic; only a
  `recover()` the user wrote counts: the one that the compiler writes into the deferred func of
  `defer F!(...)` does not, so a `defer F!(...)` in a function with no error result is reported like
  another `!`). Not
  reported: a function whose name starts with `Must` or `must` (the convention says that it
  panics). The func literals and the initializers of package variables are reported like
  functions. A call of another package's `MustX` is not followed either (it panics by name).
- **`gobang`**: a `go` statement that loses the error of a `!`. Another goroutine has no caller to
  return an error to. Reported: `go func() error { ... F!() ... }()` (the `!` returns from the
  literal, and `go` drops what it returns), anywhere; and `go f(...)` in a must function when `f`
  returns an error (the function handles the errors of its calls, but not this one).
  `go F!(...)` is a compile error already. A call of a `fmt` printer is left alone, as in
  `ignorederr`.
- **`unwaitedbody`**: a wrap macro that runs `#body()` in a `go` statement it does not wait for.
  The macro returns before the function's code has run (`InThread` waits: `<-done`). The analyzer
  looks at what the goroutine signals (`close(ch)`, `ch <- v`, `wg.Done()`) and whether the macro
  receives from that channel, ranges over it, selects on it, or calls `Wait` on that `WaitGroup`.
  A goroutine that signals through something with no name (a call's result) is not reported, and
  neither is one whose channel or `WaitGroup` is given to a call (`wait(done)`): the callee may be
  the one that waits.
- **`unusedlazy`**: a `lazy` macro parameter that the macro never uses. Its argument is not
  evaluated at the use, so a parameter that nothing calls does nothing. A parameter that is only
  given to `#text(p)` counts as used.
- **`ignorederr`**: a call statement that drops an error, **in a package that uses `!`**. A package
  that writes `!` has chosen to handle errors visibly, and in it `os.Remove(path)` as a statement,
  in a function that returns an error or has a `!` of its own, is a forgotten check, or a decision
  to write down as `_ = os.Remove(path)`. The gate is the whole package: **one `!` anywhere in a
  package turns the analyzer on for all of its functions**, also the ones that never use `!` (a
  package with a single must function gets a report for every dropped error in it), and a package
  with no `!` gets none. The message offers `os.Remove!(path)`; in a function
  whose `!` has no error to return, that means a panic on this error too, like the `!` the
  function already has. A package without `!` is plain Go, and nothing is reported (go vet has no
  such check either). Only a bare call statement counts (not an assignment to `_`, `defer`, or
  `go`), and only a call whose last result is exactly `error`. Not reported: a call right before a
  `return` of something other than `nil` in a function that returns an error (the cleanup of a
  failure path: `if err != nil { f.Close(); return err }`), the `fmt` printers, the `Write`
  methods of `strings.Builder`, `bytes.Buffer`, `bufio.Writer` (its error is sticky: `Flush` returns
  it) and hashes, and `math/rand`'s `Read`. Inside a must function the compiler handles dropped
  errors itself, so only the func literals in it are checked (they are separate functions).

Of gocpp's own, at the C/C++ boundary (they look at the shape of the code, not at the C++ types,
which `gocpp vet` does not probe):

- **`cpplocal`**: `&x` of a C/C++ local variable, kept where it outlives the variable. `&x` of a
  C/C++ value is a raw C++ address, and the variable ends with its block. `return &x` is an error
  of the build; the other ways to keep the address compile and dangle. Reported: `g = &x` (a package
  variable), `h.p = &x` or `out[i] = &x` through a parameter the caller keeps, `list = append(list, &x)`
  or `p = &x` where the target is declared outside the block of `x`, `ch <- &x` on a channel that
  the function does not own, `p := &x; return p`, and `return T{p: &x}`. `x` is a variable
  declared with a C/C++ type (`var x C.T`, `x := C.T{...}`, a parameter `x C.T`, also through a
  type name of the package for one: `type St = stat.struct_stat`) in the same function. Storing the
  address in a named result is reported (the function returns it). Not reported: `&x` as an argument (a borrow for the call), a store in a local variable
  or struct of the same block, and a store in an outer variable in a function that has a `defer`
  statement (gocpp holds the local until the function ends then, see
  [closures and defer](../spec/interop.md#cc-objects-in-closures-and-defer)). The analysis does
  not follow the address into other functions.
- **`cstr`**: a `C.cstr` string that does not last as long as C needs it. `C.cstr(s)` is valid
  until the end of the statement. Reported: `C.cstr(s)` given to a function that keeps the pointer
  (the list of typed interop, `internal/lower/keeps.go`: `openlog`, `getopt`, raylib's `InitWindow`
  and `SetWindowTitle`, `sqlite3_bind_text` with a destructor other than `SQLITE_TRANSIENT`,
  `curl_easy_setopt(CURLOPT_POSTFIELDS)`, names with `static`, `borrow`, `nocopy`, `noalloc` or
  `unowned`): use `C.CString` and `C.free`. Also `v := std.string_view(C.cstr(s))` (a view that is
  kept while the string is gone), and a string constant with a NUL byte given to `C.cstr` or
  `C.CString` (the C string ends there). A holder variable (`name := C.cstr(s)`) lives as long as
  the variable and is not reported. The function list is a copy of the tables of the lowering
  (`lower.KeeperFuncs`, `DestructorArgs`, `ConventionWords`), compared by a test. Two rules of
  typed mode are **not covered**: a call that also gets the address of a char pointer
  (`strtol(s, &end, 10)`), and the functions whose result points into the string (`strstr`,
  `basename`): they need the C++ declarations of the probes.
- **`narrowing`**: a conversion to a C number type that cuts the value: `C.int(n)` with `n` an
  `int64`, `uint64` or `uintptr`, `C.uchar(n)` with `n` an `int`, `C.float(x)` with `x` a
  `float64` (also through a type name of the package for a C type). Not reported: a constant, a plain `int` or `uint` converted to a type of 32 bits or
  more (`C.int(n)` is how every `int` goes to C; use a sized type where the range matters), `len(x)`
  and `cap(x)`, a value masked or reduced to the range (`x & 0xff`, `x % 100`), the same width
  with another sign, and `C.long`, whose width depends on the target. The implicit narrowing that
  typed interop accepts (`var c C.int = n`, a struct field `lib.Rect{x: n}`) needs the C++ types
  from the probes and is not reported yet: `gocpp vet` does not run the probes (Phase 13).

How the analyzers see `!` and macros: the front-end rewrites `!` and macros into plain Go before
`go/types` (ADR-037, ADR-075), so the trees that the analyzers read have no `!`. `check.Package.Must`
(`internal/check/mustinfo.go`) keeps what the rewrite took out (the must functions, and each
`F!(...)` with its place, its function and what it does on an error), and `load.Package.Macros`
has the macros; the analyzers get both through the hidden analyzer `gocppinputs`
([ADR-094](../adr/094-vet-reads-the-bang-forms-from-the-checker.md)). There is no comment that
switches a gocpp analyzer off for a line, as there is none in `go vet`: change the code (for
`ignorederr`, write `_ = f()`), or leave the analyzer out with `-<analyzer>=false`.

## Code with C/C++ names

`go/types` reports no error for code that uses C/C++ names (`C.strlen`, `cstring.strlen`): it
gives those names the invalid type, quietly. Some analyzers crash on that (`go/ssa`, under
`nilness`, and `refactor/satisfy`, under `printf`, do). So gocpp vet decides per package and per
analyzer:

- A package that imports `C`, or has an expression or a name whose type has an invalid part
  (its own C/C++ names, or an opaque type reaching it through an imported API), is **ill-typed**.
- Most analyzers run there as usual: the ones that x/tools declares safe on invalid types
  (`composites`, `copylocks`, `structtag`), and the ones that the tests over the interop corpus
  showed to accept them (`assign`, `bools`, `shift`, `unusedresult`, ...). `cycle` skips the
  stores that touch C/C++ values.
- `printf`, `nilness` and `tests` crash on such code, so they run over a copy of the syntax.
  For `printf`, the arguments of a call that come from C/C++ are replaced by values of unknown
  type, so the other arguments are still checked
  (`fmt.Printf("%d %s", "x", v.size())` is reported); a statement that can't be saved that way
  is left out. For `nilness` a function that touches a C/C++ value (a variable of a C++ type, a
  call into C++) is left out as a whole, because go/ssa builds a value for every variable:
  a nil dereference beside such code is not found. `-v` lists what was left out (counts and
  the first places). Types and facts are unchanged, so a wrapper of `fmt.Sprintf` is still found
  as one.
- Every analyzer runs inside `recover`. A crash or failure is printed as
  `vet: analyzer X failed on package Y`: that is a gocpp bug, please report it.

A test runs every analyzer over the interop tests, the header zoo and `examples/` of the
repository: no crash, and no finding except the one that is right (`unsafe_builder.gp` has a
`WriteByte` without the error result of `io.ByteWriter`, which `stdmethods` reports).
The programs of `tests/run` for `!` and macros, `examples/must-errors` and the macro packages of
`tests/pkgs` are vetted too (`TestMustCorpus`): the analyzers of gocpp find nothing there except
`umacro_lazy.gp`, where the macro `Never` has a lazy parameter that it never calls, on purpose.
The tests of
the C/C++ boundary do some of what the analyzers say, on purpose: a conversion that cuts
(`c_constant_precision.gp`, `float_conversions.gp`), a NUL byte in a C string (`cgo_helpers.gp`),
and the address of a parameter in a package variable that only a deferred call reads
(`defer_hold_order.gp`).

## Speed

The analysis of a 100-package chain (each package forwards to the next, with `printf` facts
travelling along it) takes about 15 ms after loading (`go test -bench ManyPackages
./internal/vet`); loading and type-checking are the same work as `gocpp check`.

## For editors

The language server will show findings as diagnostics of source `vet` in a later step; the
entry point is `vet.Run` in `internal/vet` (it returns `Finding` values with positions in the
`.gp` source).
