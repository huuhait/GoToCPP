# Porting a Go standard package

Most of gocpp's standard library is Go's own source, copied by `tools/portstd` (ADR-018, Phase 09 README section 2.1).
This guide says how to run the tool, what it changes, and how to add a package. What blocks each tier-1 package is in
[porting-blockers.md](porting-blockers.md). For what is actually shipped (as opposed to what an upstream
package needs), start with the [stdlib inventory and Go differences](stdlib.md).

```sh
go run ./tools/portstd                     # port every package of tools/portstd/ports.txt into stdlib/
go run ./tools/portstd unicode/utf8        # one package (it need not be in ports.txt)
go run ./tools/portstd -list -tier tier1   # files, imports and blockers of a package and everything it imports
go run ./tools/portstd -check              # the committed ports are what the tool writes (make portstd-check)
go run ./tools/portstd -pin go1.25.15      # move to another Go version (network)
```

Run them in the dev shell (`nix develop`), from the repository root.

## The source

The tool reads **Go's official source tarball**, never the local GOROOT: on NixOS the GOROOT is patched (store paths in
`time/zoneinfo_unix.go` and four other files) and has no `test/` folder. The tarball, its URL and its SHA-256 are in
`tools/portstd/PINS` (now `go1.25.14.src.tar.gz`, hash `9e83f44f...`, the value of the official listing at
<https://go.dev/dl/?mode=json&include=all>).

- The tarball is cached in `$XDG_CACHE_HOME/gocpp/portstd` (`~/.cache/gocpp/portstd`), never in the repository, and
  extracted next to it (`src/` without `src/cmd`, `test/`, `VERSION`, `LICENSE`). The tool reads the tarball once, hashes
  those bytes, and extracts from the same bytes, so a file that changes in between does not matter. A mismatch is an error
  and nothing is kept. The hash is checked **when the tree is extracted**, not on every run: an extracted tree is used as
  it is (its `.portstd-complete` marker holds the pin's hash, and a tree with another hash is extracted again). Delete the
  folder of the cache to start over. The cache is as trusted as your user account: anyone who can write it can change the tree.
- **Limits:** a download or cached tarball over 256 MB, or more than 1 GB extracted, is refused; `PINS` must be an `https://` URL
  (`http://` only for localhost, for the tests); `extract-*` and `download-*` leftovers of a killed run that are older than an
  hour are removed at the next start.
- **Offline:** when the cache has the tarball, it is used; otherwise the tool fails with a message that names the file it
  wants. It downloads only when the cache lacks the tarball and neither `-offline` nor `GOCPP_OFFLINE=1` is set. `-check` never
  downloads unless `-fetch` is given (see below). The tests of the tool use a fixture tarball and never the network.
- **Re-pinning:** in the dev shell of the new Go, run `go run ./tools/portstd -pin go1.25.15`. It reads the SHA-256 from the
  official listing, downloads the tarball, checks that it has that hash, and rewrites `PINS`. Then run `go run ./tools/portstd`
  and review the diff of `stdlib/`: the changes are Go's own, plus a patch rule that no longer matches (the tool fails loudly,
  see "Patch files").

## Which files

For each package the tool asks `go/build` for the files of the target: the file name suffixes (`_linux.go`, `_amd64.go`) and the
`//go:build` lines, with `GOOS` of the target, the tag `purego`, no cgo, `gc`, and the release tags up to Go 1.25.

All `goexperiment.*` tags are **false**. Go 1.25 enables some experiments by default (`swissmap`, `synchashtriemap`, `regabiwrappers`,
`regabiargs`, `aliastypeparams`, `dwarf5`), so for the packages whose files differ by them (`internal/abi`, `internal/runtime/maps`,
`reflect`, `runtime`, `sync`) the tool takes the variant from before the experiment. That is the choice, not an accident: the
variants without the experiment are the simple ones (the plain `map` and the mutex-based `sync.Map`, no register-ABI
wrappers), and gocpp implements those packages as natives or shims anyway. `sync` (step 8.7) takes the non-hash-trie `Map`.
A package that needs an experiment on can say so with `tags=goexperiment.swissmap` in `ports.txt`.

gocpp has no Go assembly, so it must take the pure-Go variant of everything Go writes in assembly on some architectures. Go's
packages choose by architecture (`internal/bytealg`, `math`, `hash/crc32`: `//go:build !amd64 && !arm64 ...` on the generic Go
file) or by the tag `purego` (`crypto`, `math/big`). So the tool's default `GOARCH` is **`generic`**, a name that is none of Go's:
every architecture-specific file is out, every "no assembly here" file is in, and `purego` is true. A function that still has no
body afterwards has no pure-Go variant: it is reported as an `asm` finding. `.s` files are never taken.

`ports.txt` holds the options (`os=`, `arch=`, `tags=`) of the default and of single packages. `os=` is a list: the files are the
union over the systems, so Phase 14 can port the same package for Windows into the same folder and keep Go's file names
(`file_unix.gp`, `file_windows.gp`). The tool lists the files it left out, with the reason, in `PORT.md`: the file name excludes the
system or architecture, a `//go:build` line is false, cgo, assembly, or a patch (`@skip`). Go's `_test.go` files are not copied:
step 9.10 runs them.

An `own <pkg> <file.gp>...` line of `ports.txt` names the files that gocpp adds to a port (`own errors wrapf.gp`: `errors.Wrapf`,
which Go does not have): they stay beside the port, the tool neither refuses the package for them nor checks them, and a name
that the port writes itself is an error.

## What the tool changes

| Where | Change |
|---|---|
| file names | `x.go` becomes `x.gp` |
| first line | `// Code ported from Go 1.25.14 src/<pkg>/<file>.go; DO NOT EDIT by hand; see PORT.md.` (the check mode tells ported files from hand-written ones by it) |
| `//go:build` | rewritten to what is left of it when everything but the operating system is decided (`!purego && (linux \|\| darwin)` becomes `linux \|\| darwin`, `!amd64` goes away), because gocpp's loader reads the line again for the machine it builds for, without Go's tags. The operating system stays for the multi-system ports. `// +build` lines go. |
| `//go:linkname` | every directive goes. A bodyless function that it pulls from the runtime becomes a native declaration through `natives.txt` (below); a push (`//go:linkname f runtime.f` on a function with a body) and the one-argument export marker are dropped and reported; a `//go:linkname` on a **variable** is a blocker (patch it, see `math/bits`). |
| bodyless functions | Without `//go:linkname`: Go's assembly, or a function the runtime provides. They need an entry in `natives.txt`, else they are an `asm` blocker. |
| `//go:nosplit` and the other `//go:` comments | kept (they are comments), listed in `PORT.md` |
| `//go:embed`, `//go:cgo_*`, `import "C"` | blockers (`import "C"` files are not selected) |
| `unsafe`, `reflect` | listed (uses of `unsafe`, imports of `reflect` and `internal/reflectlite`) |

A package with a **blocker** is not written (`-force` writes it anyway, for a look). The port of a package that only waits for
shims is fine: its imports are checked by the build, not by the tool.

A folder that already holds `.gp` files the tool did not write is handled so that nothing is lost or declared twice: a stub of
`tools/stubgen` (its header says `Code generated by tools/stubgen`) is removed, with a message; a **hand-written** file that the port would
overwrite (`io.gp` of the Phase 05 `errors`), or that would stay beside the port and declare the same names (an old
`unicode.gp` next to `tables.gp`), makes the tool refuse the package until you delete it, or pass `-force` (which overwrites the
first and keeps the second with a warning). Never rely on `-force` for this: look at what the file holds first.

A hand-written `.gp` file that stays beside a port because it is gocpp's own (`errors/wrapf.gp`) has an `own <package> <file.gp>` line
in `tools/portstd/ports.txt`. `-check` (with Go's source and by hash) fails for a hand-written `.gp` file that has no such line, as
the port itself refuses to write beside it, and for an own file that is not there.

## Replacing a stub or a hand-written package

When a package that already exists in `stdlib/` becomes a port (`unicode`, `strings`, `io`, `errors`, ...), the port is only half of the change.
Do all of this in the same commit:

1. Delete the old file (the stub or the hand-written `.gp`) after reading it: a hand-written file may hold a fix the port needs
   (turn it into a patch rule), then run the tool.
2. `tools/stubgen/main.go`: set `Real: true` on the package's row, so the tool writes no stub there (it still makes the stub in
   memory, and `TestRealPackagesMatchStub` compares the port's exported declarations with it).
3. `internal/check/gates.go`: the package's row in `stdPackages` (the gate says from which phase the package may be imported; a
   package that was never gated needs a row, and the phase number must be the step that makes it usable).
4. `docs/spec/feature-matrix.md`: the line of the package.
5. `internal/check/gates_test.go`: `TestStdPackagesMatchStubs` counts the stubs and the real packages together (at least 25), so a
   package that stops being a stub changes nothing there; it must have a row in `stdPackages` (item 3).
6. A test program in `tests/run/` and, when the package replaces a hand-written one, the old package's tests.

`make portstd-check` fails on a stub that stays beside a port.

## Natives: `tools/portstd/natives.txt`

One line per function: a key and what it becomes.

```
runtime.nanotime      gp::time::nanotime        # //gocpp:native gp::time::nanotime in the doc comment (ADR-027)
other.Thing           local                     # //gocpp:native with no name: the body is in a .cpp next to the .gp
sync.runtime_Foo      TODO: written by 8.7      # no native yet: the port is refused until someone writes it
```

The key is the target of the function's `//go:linkname` (`runtime.nanotime`), or the package-qualified name of the function
(`sync.runtime_Semacquire`) when it has none. The marker goes at the end of the doc comment, after a `//` line, as ADR-027 shows.
A function-typed parameter is a `gp::func<...>` and a handle that the native owns is a `uintptr` (`iter`'s coroutines, ADR-078: `patches/iter.rules` turns Go's `*coro` into one).
**Only the named form works today.** The tool accepts `local` and emits an argument-less marker,
but `internal/load/native.go` rejects it with `usage: //gocpp:native C++function`. It is not a way
to build a package-local `.cpp` implementation. Existing natives (including `internal/bytealg`)
name runtime functions declared in a header included by `gocpp/rt.hpp` and implemented in
`runtime/src/`. The marker is standard-library-only, on a plain bodyless function: no methods or
generic functions. Keep a generic wrapper in `.gp` when it needs a non-generic native hook.

## Patch files: `tools/portstd/patches/<import path with / as __>.rules`

A patch is a search-and-replace rule on the **upstream** text (the rules run before anything else). Every rule needs a reason, and
fails the port when its old text does not occur exactly as often as the rule says (once by default), so a new Go version that
changes the text breaks the rule loudly.

```
# a comment
@note   A known divergence from Go, for the PORT.md of the package.
@skip   helper.go   the reason the upstream file is not taken
@asm    stub.s      the reason an assembly file that the build constraints keep is no blocker (a patch writes its work in Go)
@file   wrap.go
@why    The reason for the rules that follow, up to the next @why.
@replace [count=N]
old text, any number of lines, exact, tabs included
@with
new text (may be empty)
@end
```

Never edit a ported `.gp` file: `make portstd-check` fails, and the next run overwrites it. Put the change in a rule, or into the tool.

## What is written

- `stdlib/<pkg>/<file>.gp` for each file taken (files of an earlier port that the new one does not have are removed; hand-written
  files without the header, such as a native `.cpp`, stay).
- `stdlib/<pkg>/PORT.md`: the source version and hash, the systems and tags, files taken and left out (with reasons), patches (with
  reasons), natives, findings, known divergences, the command that regenerates it. It is not embedded in the `gocpp` binary.
- `stdlib/PORTED.txt`: one line per ported package (`<pkg> <version> os=... arch=... tags=... files=n natives=n patches=n`).
- `stdlib/PORTED.sha256`: the SHA-256 of every file the tool writes (the `.gp` files and `PORT.md`), in the format of `sha256sum`.

The output is deterministic: running the tool twice gives the same bytes, and files are only written when they change.

## The check

`make portstd-check` (part of `make ci`) has two levels.

- **With the tarball in the cache** it ports every package of `ports.txt` into memory and compares: each ported file, `PORT.md`,
  the `PORTED.txt` lines, the hashes of `PORTED.sha256`, a stale file that has the header but is no longer written, a stub of
  `tools/stubgen` beside a port, and the table of [porting-blockers.md](porting-blockers.md).
- **Without it** (offline CI, a new checkout) it compares each file of each package of `PORTED.txt` with its hash in `PORTED.sha256`,
  and looks for missing files, files with the header that the list does not know, and stubs. So a hand edit of a ported file is
  found on every machine. What this level cannot see is an edit of the file together with its line in `PORTED.sha256` (it
  looks consistent), or a tool that would write something else: the first level finds both. The message says that the byte
  comparison was skipped.

`-check` never downloads, not even with `-fetch` when `-offline` or `GOCPP_OFFLINE=1` is set. Run
`go run ./tools/portstd -check -fetch` once to fill the cache, and before you commit a port. `GOCPP_REQUIRE_PORTSTD=1` (or `-require`)
makes a missing tarball an error instead of the hash-only check: set it in the CI job that has the cache, as
`GOCPP_REQUIRE_PORTSTD=1 make ci` (like `GOCPP_REQUIRE_CXX`, which `make ci` sets itself; this one is opt-in because a laptop without the
tarball should still run `make ci`). The repository has no CI configuration of its own, so the switch belongs to whoever runs one.

A `patches/*.rules` file whose package is not in `ports.txt` is ignored with a warning (a typo in the name).

## Adding a package

1. **Inventory the dependency closure.** Run `go run ./tools/portstd -list <pkg>` for the target
   selection. Every imported package must be present first (`-list` prints the order;
   `boundary.txt` identifies providers such as `runtime`, `syscall`, `internal/bytealg`, `sync`
   and `reflect`). Check actual exported APIs too: a shim can import successfully without
   implementing the operation the new package needs. Compare with the [current inventory](stdlib.md),
   not just the upstream blocker table.
2. **Select files deliberately.** Add a line to `ports.txt`; specify `os=linux,windows` where
   OS-specific source is needed, and `arch=`/`tags=` only with a reason. Check the union of files
   in `PORT.md`, not just the Linux build. The usual `generic` architecture plus `purego` avoids
   assembly; changing to `amd64` can reintroduce assembly declarations. Keep the pinned tarball
   and hash of `PINS`, not a copy of the machine's GOROOT.
3. **Resolve each boundary.** Use `natives.txt` for runtime-provided operations and `.rules` for
   exact, reasoned source changes. A shim must preserve the API/semantics needed by its consumers;
   do not call a no-op a complete port. Distinguish Go-runtime layout assumptions, reference
   cycles under ARC, and unavailable OS operations. `portstd` refuses unresolved blockers;
   `-force` is an inspection aid, not a finished port.
4. **Generate and review.** Run `go run ./tools/portstd <pkg>`. Review every selected/skipped file,
   patch and native in `PORT.md`. Commit the `.gp` files, `PORT.md`, `PORTED.txt`,
   `PORTED.sha256`, configuration and patches together. If replacing a stub, also do the steps
   in [Replacing a stub](#replacing-a-stub-or-a-hand-written-package).
5. **Test a consumer and Go's tests.** Add a small `tests/run/` program (`// phase: 09`, output
   compared with real Go using `GOCPP_RUN_VS_GO=1`). For the upstream test suite, add a
   `tools/portstd/gotests/std/<pkg with / as __>.py` specification and use
   `convert_std.py`, not the older `convert.py` fixture-copy route. The converted tests must
   exercise the shipped package through `GOCPP_TEST_STDLIB`, not an independent copy of its
   implementation. [The test guide](stdlib-tests.md) explains test-only helpers, `SHIMS`,
   `EXTRA`, exact patches and expected failures.
6. **Count honestly and check regeneration.** Run `tools/stdgocount -check -detail` for that
   package. A skip, dropped test, excluded file or known compiler bug is not a pass. The budget
   is passed / (selected - documented divergences); also report passed / all, so excluding a
   difficult file cannot hide reduced coverage. Refresh the [inventory](stdlib.md#reading-the-inventory),
   run the focused test(s) and the offline port check, then review the generated diff.

If the ported code does not compile with gocpp, record `file:line: construct: error` and determine
whether it is a compiler bug or an unsupported construct. Do not silently rewrite a failing
assertion into success. A necessary adaptation belongs in a patch with its reason; an unsupported
test belongs in the manifest with its reason and remains visible in the count.

## Worked example: regenerate and test `path`

`path` is already shipped, small and portable; this exercise adds no dependency or new upstream
package. It uses the pinned Go 1.25.14 tarball already in the portstd cache. If the source is not
cached, the offline command stops: populate the cache with the approved source download first
(`go run ./tools/portstd -check -fetch` without `GOCPP_OFFLINE=1`), rather than falling back to
GOROOT. Run from the repository root in `nix develop`.

The following commands were exercised on Linux/amd64. The output directory is disposable;
none of them rewrites the shipped port or the committed converted tests.

```sh
work=$(mktemp -d)
go run ./tools/portstd -offline -o "$work/stdlib" path
cmp stdlib/path/path.gp "$work/stdlib/path/path.gp"
cmp stdlib/path/match.gp "$work/stdlib/path/match.gp"
cmp stdlib/path/PORT.md "$work/stdlib/path/PORT.md"

GOCPP_TEST_STDGO_OUT="$work/stdgo" python3 tools/portstd/gotests/convert_std.py path
cmp tests/stdgo/manifest/path.json "$work/stdgo/manifest/path.json"
go build -o "$work/gocpp" ./cmd/gocpp

GOCPP_GATE_PHASE=09 GOCPP_MODCACHE=off GOCPP_OFFLINE=1 XDG_CACHE_HOME="$work/cache" \
  go run ./tools/stdgocount -gocpp "$work/gocpp" -stdgo "$work/stdgo" \
  -work "$work/count" -j 1 -timeout 1m -check -detail path

GOCPP_GATE_PHASE=09 GOCPP_MODCACHE=off GOCPP_OFFLINE=1 XDG_CACHE_HOME="$work/cache" \
  GOCPP_TEST_STDLIB="$work/stdgo" "$work/gocpp" test -v -timeout 1m path
```

The port command reported `path: 2 file(s)`; all three source/provenance comparisons and the
manifest comparison were identical. `PORT.md` records `path.go` and `match.go`, `os=linux`,
`arch=generic`, `purego`, no patches and no natives. `std/path.py` is simply
`FILES = ['all']`; it selects three test files. `stdgocount` reported **17 selected, 17 passed,
0 failed/skipped/dropped/divergences, 100% selected and overall**. The direct `gocpp test`
confirmed the same nine tests and eight examples.

One important qualification: `TestCleanMallocs` prints “skipping AllocsPerRun checks;
GOMAXPROCS>1” and returns normally on this machine. The top-level function passes, but that is
**not evidence of Go-identical allocation counts**. Read the test log as well as the percentage.
The general allocation divergence is described in the [test guide](stdlib-tests.md#what-fails-and-why).

The tests run against the compiler's embedded `stdlib/path`, whose generated files were just
compared with the regenerated port. The overlay contains tests, not a second production
implementation. To port another package, follow the checklist above instead of copying
`path`'s lack of patches: runtime hooks, OS selection and test exclusions vary by package.
