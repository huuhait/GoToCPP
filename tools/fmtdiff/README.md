# tools/fmtdiff

A seeded differential generator for the print side of `fmt` (audit of the native engine,
decision 9.7 = A). It writes Go programs that are valid Go **and** valid gocpp, runs each with real
Go (`go run`) and with `gocpp run`, and reports every case where the two print something else.

Each program has the same fixed prelude (`prelude.go`: named types, structs, generics, types with
`String`, `Error`, `GoString`, `Format` methods, nil receivers, panicking methods) and a few
hundred cases `c(ID, <expr>)`. A case is a call of `Sprintf` (about 75%), `Sprint`, `Sprintln`,
`Append`, `Appendln`, `Appendf` or `Errorf` (with `%w`, then `errors.Unwrap` and `%T` of the
result), over:

- verbs: every verb of the doc and some bad ones (`%z`, `%!`, a lone `%`, `%w` in `Sprintf`),
  chosen by the kind of the value, with flags (`+-# 0`, in any order, repeated), widths and
  precisions (`0 1 5 20`, `.`, `.0`, `*`, `.*` with `int` and non-`int` operands), explicit
  argument indexes (also bad ones), missing and extra operands, and a list of hand-made odd
  formats (`%[1]*[1]d`, `%5%`, `%99999999999999d`, ...);
- values (`values.go`): all integer sizes with their limits, runes, `float32/64` (NaN, Inf, -0,
  denormals, 1e21, ...), complex, strings (invalid UTF-8, controls, long), bytes in every shape,
  slices, arrays, maps of many key types (a NaN key, struct and interface keys), structs (nested,
  embedded, unexported fields, tags, anonymous, empty), generic instances, nil pointers, funcs,
  chans and interfaces, typed nils, interfaces holding all of these, and the method types.

What it avoids: addresses (`%p` of a non-nil value, a non-nil pointer to a scalar with any verb
but `%T`) and maps whose order Go leaves to chance (interface keys of different types, several NaN
keys).

```sh
# in the dev shell (nix develop): go is Go 1.25
go build -o /tmp/gocpp ./cmd/gocpp
go run ./tools/fmtdiff -seed 1 -n 20000 -j 8 -gocpp /tmp/gocpp -work /tmp/fmtdiff-work
go run ./tools/fmtdiff -seed 1 -print 3                              # the source of program 3 (-per cases)
go run ./tools/fmtdiff -seed 1 -prog 3 -case 17,40 -emit-test fmt_gen_x   # tests/run/fmt_gen_x.gp
go run ./tools/fmtdiff -mode go -n 5000                              # only check the generator with real Go
```

`-n` is the number of cases in all, `-per` (250) the cases per program (a gocpp build is slow,
so a program has many), `-j` the programs in flight (at most 12; each is one `go run` and one
`gocpp run`), `-first` the index of the first program. The program with index `i` of a seed is
always the same.

`-work` names a parent directory, never a directory the tool takes ownership of. Each invocation
creates its own `fmtdiff-*` child there (by default in the system temporary directory). An interrupt
removes only that child, preserving the parent and any unrelated files. A normal run keeps the child
and its report; `-out` and `-xdg` can place the report and cache elsewhere, outside that cleanup.

The report is `<owned-child>/out/report-<seed>.txt`: for each mismatch the case, its `Sprintf` source, the
declaration of its values, and what Go and gocpp printed (`%q` of the whole result, so a space or
an escape shows). The summary counts them by verb and by value. A program whose cases make gocpp
fail to build or crash is cut by halving until single cases are left (`-budget` runs), and those
show as `<crash>` with the compiler's first error line. A case that makes real Go fail is a
generator bug (`GENERATOR BUG`).

`-emit-test NAME -prog K -case ID[,ID...]` turns cases into a run test: it removes everything the
cases do not use (`prune.go`: the types, values and methods they need, by name), runs real Go on
what is left and writes the program with its output as the `// Output:` block. Edit the
description (`-desc`) and check that the test is small.

`go test ./tools/fmtdiff` tests the generator (determinism, valid Go by `go/types`, the pruning, the
bisection). `GOCPP_FMTDIFF=1 GOCPP_FMTDIFF_GOCPP=/path/to/gocpp go test ./tools/fmtdiff -run
TestGocpp` also runs two programs (300 cases) with Go and gocpp: the always-on subset.

`-cxx clang++` (GOCPP_CXX) runs gocpp with the other compiler, `-run-flags -asan` passes flags to `gocpp run`. `known.go`
lists engine bugs that are already understood: their mismatches are counted apart (`-no-known` counts them) so they do not hide
new ones; remove an entry when the bug is fixed. The prelude imports `io` and asserts `*lim` is an `io.Writer` because
gocpp cannot pass a user type with a `Write` method to `Fprint*` in a program that does not import `io` (a lowering gap).
The generator changes the programs of a seed whenever the catalog changes, so quote the case source, not seed and index.
