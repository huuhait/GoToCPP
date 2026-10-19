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
- **`panicbang`**: a placeholder. It will report `F!(...)` that would panic in a library
  package once `!` exists (Phase 10).

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
`examples/macros` and `examples/must-errors` can't be vetted until Phase 10 (exit status 2).

## Speed

The analysis of a 100-package chain (each package forwards to the next, with `printf` facts
travelling along it) takes about 15 ms after loading (`go test -bench ManyPackages
./internal/vet`); loading and type-checking are the same work as `gocpp check`.

## For editors

The language server will show findings as diagnostics of source `vet` in a later step; the
entry point is `vet.Run` in `internal/vet` (it returns `Finding` values with positions in the
`.gp` source).
