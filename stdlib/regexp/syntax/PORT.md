# regexp/syntax

Ported by `tools/portstd` from go1.25.14's `src/regexp/syntax` (BSD-3-Clause, see `../../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/regexp__syntax.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd regexp/syntax` |

## Files taken

- `compile.gp`, from `compile.go`
- `doc.gp`, from `doc.go`
- `op_string.gp`, from `op_string.go`
- `parse.gp`, from `parse.go`
- `perl_groups.gp`, from `perl_groups.go`
- `prog.gp`, from `prog.go`
- `regexp.gp`, from `regexp.go`
- `simplify.gp`, from `simplify.go`

## Files not taken

- 3 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `parse.go`: `re1.Rune = re1.Rune0[:1]` makes the Regexp own itself (see the note); an allocated one-element slice does the same.
- `parse.go`: Same: a literal's rune slice must not alias Rune0.
- `parse.go`: Same: the Sub slice of a repeat must not alias Sub0.
- `parse.go`: Same: a copy that "reclaims storage" copies into new storage, not into Rune0.
- `parse.go`: Same: the Sub slice of a concat or alternation starts empty and grows by append.
- `parse.go`: Same: literalRegexp used Rune0 as storage for small strings; the rune slice of the string is the same value.
- `parse.go` (2 places): Same: the class parsers append to the slice they are given; it must not be Rune0 (and not nil, which would mean "not a class" to the callers: an empty class is a valid result).
- `parse.go`: Same: the Sub slice of a capture group must not alias Sub0.
- `parse.go`: Same: parseClass starts the rune slice of a class in Rune0; an empty allocated slice is the same (not nil: a class with no runes is a valid result).
- `simplify.go` (5 places): The simplified trees must not alias Sub0 either (see the note).

## Native functions

None.

## Findings

None.

## Known divergences from Go

- A syntax.Regexp keeps short Sub and Rune slices in its own fields Sub0 and Rune0 (`re.Sub = re.Sub0[:1]`). In gocpp a slice of an array field holds a counted reference to the block that owns the array, which is the Regexp itself, so such a Regexp owned itself and was never freed: every regexp.Compile (and every syntax.Parse) leaked its parse tree (a reference cycle, ADR-004). The fields stay (they are exported), but nothing aliases them: the slices are allocated, the results are Go's.
