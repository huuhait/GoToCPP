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

What the table says for the next steps:

- Nothing in `unicode/utf8`, `unicode`, `math/bits` blocks: they port as they are (step 9.1 ports `unicode/utf8`,
  `unicode/utf16` and `math/bits`).
- The packages from `errors` to `regexp` all wait for **`sync`** (8.7), **`internal/reflectlite`** (9.3) or the
  **`internal/bytealg`/`internal/abi`** shims (9.2); `strconv` also needs **`math`** (9.5), `context` needs **`time`** (8.8).
  Once those exist, the ports themselves have no `asm` and no `linkname` blocker: the tier-1 packages are plain Go.

<!-- portstd:begin: go run ./tools/portstd -list -tier tier1 -md -doc docs/guides/porting-blockers.md -->

Source: go1.25.14, target `linux`, GOARCH `generic`, tags `purego`. Roots: `unicode/utf8`, `unicode`, `strings`, `bytes`, `strconv`, `errors`, `sort`, `math/bits`, `io`, `bufio`, `context`, `regexp`.

### Packages to port, dependencies first

| # | Package | Files | Imports | Blocks the port | Needs, through imports too (not ported) | Notes |
|--:|---|--:|---|---|---|---|
| 1 | `unicode/utf8` | 1 | none | none | none |  |
| 2 | `unicode` | 5 | none | none | none |  |
| 3 | `errors` | 3 | `internal/reflectlite` | needs internal/reflectlite | `internal/reflectlite` | unsafe, reflect |
| 4 | `internal/stringslite` | 1 | `internal/bytealg` | needs internal/bytealg | `internal/bytealg` | unsafe |
| 5 | `io` | 3 | `errors`, `sync` | needs sync | `internal/reflectlite`, `sync` |  |
| 6 | `math/bits` | 3 | none | none | none |  |
| 7 | `strings` | 8 | `errors`, `internal/abi`, `internal/bytealg`, `internal/stringslite`, `io`, `iter`, `math/bits`, `sync`, `unicode`, `unicode/utf8` | needs internal/abi, internal/bytealg, iter, sync | `internal/abi`, `internal/bytealg`, `internal/reflectlite`, `iter`, `sync` | unsafe |
| 8 | `bytes` | 4 | `errors`, `internal/bytealg`, `io`, `iter`, `math/bits`, `unicode`, `unicode/utf8` | needs internal/bytealg, iter | `internal/bytealg`, `internal/reflectlite`, `iter`, `sync` |  |
| 9 | `math` | 53 | `math/bits` | none | none |  |
| 10 | `strconv` | 14 | `errors`, `internal/stringslite`, `math`, `math/bits`, `unicode/utf8` | none | `internal/bytealg`, `internal/reflectlite` |  |
| 11 | `sort` | 5 | `internal/reflectlite`, `math/bits`, `slices` | needs internal/reflectlite, slices | `internal/reflectlite`, `slices` | reflect |
| 12 | `bufio` | 2 | `bytes`, `errors`, `io`, `strings`, `unicode/utf8` | none | `internal/abi`, `internal/bytealg`, `internal/reflectlite`, `iter`, `sync` |  |
| 13 | `context` | 1 | `errors`, `internal/reflectlite`, `sync`, `sync/atomic`, `time` | needs internal/reflectlite, sync, sync/atomic, time | `internal/reflectlite`, `sync`, `sync/atomic`, `time` | reflect |
| 14 | `regexp/syntax` | 8 | `slices`, `sort`, `strconv`, `strings`, `sync`, `unicode`, `unicode/utf8` | needs slices, sync | `internal/abi`, `internal/bytealg`, `internal/reflectlite`, `iter`, `slices`, `sync` |  |
| 15 | `regexp` | 4 | `bytes`, `io`, `regexp/syntax`, `slices`, `strconv`, `strings`, `sync`, `unicode`, `unicode/utf8` | needs slices, sync | `internal/abi`, `internal/bytealg`, `internal/reflectlite`, `iter`, `slices`, `sync` |  |

### Provided another way (not ported)

| Package | Owner | What | Needed by |
|---|---|---|---|
| `internal/abi` | 9.2 | stub: NoEscape, FuncPCABI0, type layout is not used | `strings` |
| `internal/bytealg` | 9.2 | native (memchr, memcmp) | `internal/stringslite`, `strings`, `bytes` |
| `internal/reflectlite` | 9.3 | alias of the reflect subset | `errors`, `sort`, `context` |
| `iter` | 7.8 | Phase 07 (range over func) | `strings`, `bytes` |
| `slices` | 7.1 | Phase 07 | `sort`, `regexp/syntax`, `regexp` |
| `sync` | 8.7 | native (Phase 08) | `io`, `strings`, `context`, `regexp/syntax`, `regexp` |
| `sync/atomic` | 8.7 | native (Phase 08) | `context` |
| `time` | 8.8 | Phase 08 timers and clocks, Phase 09 formatting (9.8) | `context` |

<!-- portstd:end -->
