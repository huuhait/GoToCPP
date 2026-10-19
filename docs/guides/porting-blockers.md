# What blocks the tier-1 ports

The dependency and blocker table for the packages that step 9.4 ports (`unicode/utf8`, `unicode`, `strings`, `bytes`,
`strconv`, `errors`, `sort`, `math/bits`, `io`, `bufio`, `context`, `regexp`) and for everything they import. It is made
by `tools/portstd -list` from Go's pinned source ([porting.md](porting.md)), so the order is the order of the work: a
package comes after the packages it imports. `make portstd-check` fails when the table is out of date (it needs the Go
source in the cache; offline it skips).

How to read it:

- **Blocks the port** is what the tool finds in the package's own files for the target: `asm` (a bodyless function with
  no native entry, or an assembly file the constraints keep), `linkname` (a `//go:linkname` with no native entry, or on a
  variable), `cgo`, `go:embed`, and `needs X` for each direct import that is not ported (the last table says who provides
  it). `none` means the files are Go's, copied as they are.
- **Needs, through imports too** is every package outside the port that the package depends on, directly or through other
  packages. A package can only run when all of those exist.
- **Notes**: `unsafe` (the package uses `unsafe.String`, `unsafe.Pointer`...: Phase 04 supports them, the uses are listed in
  `PORT.md`), `reflect` (it imports `reflect` or `internal/reflectlite`: step 9.3).
- The tags `purego` and, for `strconv` and `math/bits`, `compiler_bootstrap` (Go's variants without `//go:linkname` and
  without `internal/bytealg`) are set; the architecture is `generic`, so no assembly variant is chosen
  (`tools/portstd/ports.txt`).

What the table says for the next steps (state after step 9.4b):

- Ported and real: `unicode/utf8`, `unicode/utf16`, `math/bits`, `math` (9.5), `unicode`, `strconv` and `errors` (9.4a), and, since
  9.4b, `io`, `io/fs`, `path`, `strings`, `bytes`, `bufio` and `context`. The table lists a package as "needs X" when X is not one
  of the packages the tool ports, also when X exists as a hand-written package of an earlier step (`internal/bytealg`,
  `internal/stringslite`, `internal/reflectlite`, `sync`, `sync/atomic`, `time`, ...): those rows are blocked for the tool's purposes
  only, the ports build on them. `strings` and `bytes` have `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq` and `FieldsFuncSeq` since step 7.10 (`iter.Seq` is a
  generic type; the `@skip iter.go` rules are gone); `io/fs` sorts `ReadDir` with `slices.SortFunc`, as Go does; `strings.Builder` keeps its
  address as a `uintptr` (a `*Builder` in the Builder itself would be a reference cycle: ADR-073).
- `cmp`, `slices`, `maps` and `iter` are ported (step 7.10, ADR-078). `maps` takes `maps.clone` as a native (`natives.txt`, `gp::maps::clone` over `gp::map_clone`); `iter` takes the two runtime coroutine functions as natives (`runtime.newcoro` and `runtime.coroswitch` in
  `natives.txt`, `patches/iter.rules`).
- What is left of step 9.4: `sort` (it needs `slices`, which exists now, and `internal/reflectlite`'s `Swapper`, 9.3, built), `regexp/syntax` and
  `regexp` (need `sort`). None of them has an `asm` or `linkname` blocker: the tier-1 packages are plain Go.
- `strconv` takes the files of Go on amd64 (`tags=purego`): `internal/bytealg.IndexByteString` is the native one, not the
  `compiler_bootstrap` loop.

<!-- portstd:begin: go run ./tools/portstd -list -tier tier1 -md -doc docs/guides/porting-blockers.md -->

Source: go1.25.14, target `linux`, GOARCH `generic`, tags `purego`. Roots: `unicode/utf8`, `unicode`, `strings`, `bytes`, `strconv`, `errors`, `sort`, `math/bits`, `io`, `bufio`, `context`, `regexp`.

### Packages to port, dependencies first

| # | Package | Files | Imports | Blocks the port | Needs, through imports too (not ported) | Notes |
|--:|---|--:|---|---|---|---|
| 1 | `unicode/utf8` | 1 | none | none | none |  |
| 2 | `unicode` | 5 | none | none | none |  |
| 3 | `errors` | 3 | none | none | none | unsafe |
| 4 | `internal/stringslite` | 1 | `internal/bytealg` | needs internal/bytealg | `internal/bytealg` | unsafe |
| 5 | `io` | 3 | `errors`, `sync` | needs sync | `sync` |  |
| 6 | `iter` | 1 | `internal/race`, `runtime` | needs internal/race, runtime | `internal/race`, `runtime` | unsafe |
| 7 | `math/bits` | 3 | none | none | none |  |
| 8 | `strings` | 8 | `errors`, `internal/bytealg`, `internal/stringslite`, `io`, `iter`, `math/bits`, `sync`, `unicode`, `unicode/utf8` | needs internal/bytealg, sync | `internal/bytealg`, `internal/race`, `runtime`, `sync` | unsafe |
| 9 | `bytes` | 4 | `errors`, `internal/bytealg`, `io`, `iter`, `math/bits`, `unicode`, `unicode/utf8` | needs internal/bytealg | `internal/bytealg`, `internal/race`, `runtime`, `sync` |  |
| 10 | `math` | 53 | `math/bits` | none | none |  |
| 11 | `strconv` | 14 | `errors`, `internal/bytealg`, `internal/stringslite`, `math`, `math/bits`, `unicode/utf8` | needs internal/bytealg | `internal/bytealg` |  |
| 12 | `cmp` | 1 | none | none | none |  |
| 13 | `slices` | 5 | `cmp`, `iter`, `math/bits` | none | `internal/race`, `runtime` | unsafe |
| 14 | `sort` | 5 | `internal/reflectlite`, `math/bits`, `slices` | needs internal/reflectlite | `internal/race`, `internal/reflectlite`, `runtime` | reflect |
| 15 | `bufio` | 2 | `bytes`, `errors`, `io`, `strings`, `unicode/utf8` | none | `internal/bytealg`, `internal/race`, `runtime`, `sync` |  |
| 16 | `context` | 1 | `errors`, `internal/reflectlite`, `sync`, `sync/atomic`, `time` | needs internal/reflectlite, sync, sync/atomic, time | `internal/reflectlite`, `sync`, `sync/atomic`, `time` | reflect |
| 17 | `regexp/syntax` | 8 | `slices`, `sort`, `strconv`, `strings`, `sync`, `unicode`, `unicode/utf8` | needs sync | `internal/bytealg`, `internal/race`, `internal/reflectlite`, `runtime`, `sync` |  |
| 18 | `regexp` | 4 | `bytes`, `io`, `regexp/syntax`, `slices`, `strconv`, `strings`, `sync`, `unicode`, `unicode/utf8` | needs sync | `internal/bytealg`, `internal/race`, `internal/reflectlite`, `runtime`, `sync` |  |

### Provided another way (not ported)

| Package | Owner | What | Needed by |
|---|---|---|---|
| `internal/bytealg` | 9.2 | native (memchr, memcmp) | `internal/stringslite`, `strings`, `bytes`, `strconv` |
| `internal/race` | 9.2 | stub: Enabled is false | `iter` |
| `internal/reflectlite` | 9.3 | alias of the reflect subset | `sort`, `context` |
| `runtime` | 9.2 | minimal surface: Error, PanicNilError, Gosched, GOMAXPROCS, ... (8.10) | `iter` |
| `sync` | 8.7 | native (Phase 08) | `io`, `strings`, `context`, `regexp/syntax`, `regexp` |
| `sync/atomic` | 8.7 | native (Phase 08) | `context` |
| `time` | 8.8 | Phase 08 timers and clocks, Phase 09 formatting (9.8) | `context` |

<!-- portstd:end -->
