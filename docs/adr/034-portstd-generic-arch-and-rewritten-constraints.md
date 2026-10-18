# ADR-034: `portstd` takes the pure-Go variants by a `generic` architecture and rewrites the build constraints it keeps

Status: accepted (2026-10-18), refines [ADR-018](018-stdlib-ported-from-go.md)

## Context

[ADR-018](018-stdlib-ported-from-go.md) says that `tools/portstd` copies the files that `go/build` picks for the target
OS with the `purego` tag, with no assembly. Building it (Phase 09 step 9.1) showed two things the sentence leaves open.

1. Go does not use one convention for "the Go version of an assembly function". `crypto` and `math/big` have the tag
   `purego`. `internal/bytealg`, `math`, `hash/crc32` and most others select by architecture: the assembly package
   has `_amd64.go` files with bodyless declarations next to `.s` files, and the generic Go file says `//go:build !amd64 &&
   !arm64 && ...`. With `GOARCH=amd64` and `purego` the tool would take the bodyless declarations.
2. gocpp's loader reads `//go:build` lines again when it builds a program (`internal/load/resolve.go`), for the machine it
   builds for, with its own tags (the compiler is `gocpp`, there is no `purego`, no `gc`). A kept file whose line says
   `!purego`, `amd64` or `!amd64 && !arm64` would be dropped or kept by the loader against what the tool decided.

## Decision

- The tool selects files for **`GOARCH=generic`**, a name that is none of Go's, with the tag `purego` (and the release tags,
  `gc`, no cgo). Every architecture-specific file is out, every "no assembly here" file is in. A bodyless function that is
  still there has no pure-Go variant and is reported (`asm`), not guessed. `ports.txt` can change the architecture, the tags
  (`compiler_bootstrap` for `strconv` and `math/bits`: Go's variants without `//go:linkname` and `internal/bytealg`) and the
  systems of a package.
- The `//go:build` line of a **kept** file is rewritten to what is left of it when everything except the operating system is
  decided (`!purego && (linux || darwin)` becomes `linux || darwin`; a line that is decided entirely is removed). The
  operating system stays: the tool takes the union of the files of several systems (`os=linux,windows`, Phase 14), and the
  loader chooses among them by the tag and the file name suffix. A rewrite that is false for a system the file was kept
  for is an error of the tool.
- All `goexperiment.*` tags are false, so the files that Go 1.25 selects by default experiments (`swissmap`,
  `synchashtriemap`, `regabi*`, `aliastypeparams`, `dwarf5`: in `internal/abi`, `internal/runtime/maps`, `reflect`, `runtime`,
  `sync`) are the variants from before the experiment. They are the simpler ones, and gocpp provides those packages as natives or
  shims; a port that needs one on sets the tag in `ports.txt`.
- The source is the pinned tarball and nothing else (SHA-256 in `tools/portstd/PINS`, cache outside the repository);
  a function without a body needs an entry in `natives.txt` (`//gocpp:native <C++ name>`, or `local` for the argument-less
  form that step 9.2 builds) or the port is refused; a patch is a search-and-replace rule with a reason that fails when its
  old text is not found as often as it says.
- `make portstd-check` (in `make ci`) regenerates every port from the cache and compares bytes. Without the tarball in the
  cache it compares every file with the SHA-256 recorded in `stdlib/PORTED.sha256` (a hand edit is found on every machine),
  says that the byte comparison was skipped, and passes, so offline CI is green; `GOCPP_REQUIRE_PORTSTD=1` makes the missing
  tarball an error where the cache is filled.

## Alternatives

- **`GOARCH=amd64` plus `purego`**: takes the declarations of the assembly functions.
- **Keep Go's `//go:build` lines unchanged**: the loader would then evaluate tags (`purego`, `gc`) that it does not have.
- **A unified-diff patch format**: needs a `patch` implementation or an external tool, and "applied with fuzz" hides what a new
  Go version changed.
- **Check the ports in `go test`**: the test would need the 32 MB tarball, so it would either download or skip silently.

## Consequences

Good: a port is reproducible and reviewable (the diff of a new Go version is Go's own); a hand edit is caught; Windows can reuse
Go's own `_windows.go` files; the loader and the tool never disagree about which file is in.

Bad: a package whose generic variant is slow (`crypto`, `hash/crc32`) stays slow until a native replaces it; `generic` is not a
real architecture, so a package that needs constants per architecture (`internal/goarch`, `internal/cpu`, `syscall`) is a shim, not
a port; without the cache the check is by hash, so an edit of a file together with its recorded hash is caught only where the cache
is (the maintainers, after `-check -fetch`, or a CI job with `GOCPP_REQUIRE_PORTSTD=1`).
