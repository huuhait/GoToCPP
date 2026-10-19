# ADR-089: Packaging: go install, GoReleaser archives and a Nix package with wrapped compilers

Status: accepted (2026-10-19)

## Context

ADR-009 made the runtime and the standard library part of the `gocpp` binary, so one file is all there
is to install. Step 15.13 had to prove it: that `go install github.com/huuhait/gocpp/cmd/gocpp@version`
works from a tag, that a release can ship the binary for linux amd64/arm64 and windows amd64, and that
NixOS, where no compiler is on the `PATH`, can run `gocpp` out of the box.

Findings of the first try:

- A Go module proxy refuses the repository: a module zip may not hold two files whose names differ only in
  case (11 pairs `tests/run/fmt_*_X*.gp` and `fmt_*_x*.gp`, which also could not be checked out on Windows
  or macOS), nor a file name with an apostrophe (`tests/pkgs/odd_file_names/lib/q'uote.gp`). `go install`
  of a local checkout worked, so nothing had noticed.
- A binary made by `go install pkg@v1.2.3` reported `gocpp dev`: the version was only set by `-ldflags -X`.
- `go mod vendor` (what `buildGoModule` does) wants the packages that the tests of `cmd/gocpp` import,
  so a Nix source that holds only the program fails.

## Decision

- **go install.** The module is `github.com/huuhait/gocpp`, `go.mod` has no `replace` or `exclude`, and
  every tracked file has a name that a module zip accepts. The 11 upper-case test programs are renamed
  (`fmt_uint_Xupper_uint8.gp`), the apostrophe fixture is `q#uote.gp`, and `cmd/gocpp` `TestApostropheInFileName`
  makes the apostrophe file in a temporary folder, so that coverage stays. `internal/packaging` has the
  guards: a case-collision and symlink check of the tracked files, and `TestGoInstallOfATaggedVersion`, which
  puts the module zip of the working tree in a `file://` proxy and runs the real `go install ...@v0.0.1-rc.1`
  (the go command applies the proxy's zip rules while unpacking), then runs the binary away from the source tree.
- **Version.** `internal/version.Version` stays a plain string `"dev"` that `-X` can set (`-X` does nothing to a
  variable that a function initializes; `TestLdflagsSetTheVersion` builds with the flag from `.goreleaser.yaml`
  and checks `gocpp version`). An `init` replaces `"dev"` with the module version that `go install` recorded
  in the binary (`debug.ReadBuildInfo`), unless Go stamped a repository (`vcs.revision`: a build in a checkout stays
  `dev`). `gocpp version` prints a second line, `runtime <hash>` (16 hex digits, the name of the cache folder
  of the extracted runtime, ADR-009): `build.RuntimeHash`.
- **GoReleaser** (`.goreleaser.yaml`, v2 format): `CGO_ENABLED=0`, `-trimpath`, `-s -w`, `-X` for the version
  (`v{{ .Version }}`); `.tar.gz` for linux and `.zip` for windows, each with `LICENSE` and `README.md` and nothing
  else; `checksums.txt` (sha256); the release is a **draft**; the notes name the pinned llvm-mingw release (a test
  keeps the text equal to `internal/toolchain.Release`). No Windows arm64 (phase 15 §2.8).
- **Nix.** `flake.nix` gets `packages.<system>.{default,gocpp}` and `apps.<system>.{default,gocpp}` for x86_64-linux
  and aarch64-linux (the dev shell is unchanged). The package is `buildGoModule` over a `lib.fileset` source of the
  program and the files it embeds, with `proxyVendor = true` (the module cache is downloaded: no need for the
  tests' imports) and a `vendorHash` that goes with `go.mod` and `go.sum` (a stamp of their sha256 in a comment,
  checked by `TestFlakeVendorHash`, says when to redo it). It is wrapped with `--prefix PATH` of Nix's
  `g++`, `clang++` (21), binutils (`ar`, `nm`), `llvm-ar` and `pkg-config`; the wrappers add their own libstdc++ and libc
  paths. The install check builds and runs a program with `g++` and with `clang++` inside the Nix build.
  `GOCPP_CXX`/`CXX` still choose another compiler.

## Alternatives

- **A `go.mod` in the folders with the odd names** (rejected): a nested module drops the folder from the zip, but the
  fixture is then a module of its own with a `go.mod`, which is not what it tests.
- **Moving `tests/` out of the module zip with a nested module** (rejected): `go test ./tests` from the root would stop working.
- **`vendorHash = null` with a vendor folder** (rejected): a committed vendor folder for one dependency is more to keep up to date than a hash.
- **Release notes from a file** (rejected): `release.header: from_file` is a GoReleaser Pro feature; the notes carry the pin as text and a test checks it.

## Consequences

Good: one `go install`, one archive or one `nix run` gives a working `gocpp`; a repository change that would break
`go install` (a new case-colliding or odd file name, a `replace` directive, a stale flake hash) fails a test.

Bad / left:

- The `vendorHash` is redone by hand after a change of `go.mod` or `go.sum` (the test says so).
- On a Windows host the toolchain download of ADR-055 has no Windows archive pinned yet, so the Windows zip needs a MinGW-w64 compiler on the `PATH`.
- The Nix package has no MinGW-w64 for Windows targets (the dev shell has); llvm-mingw's downloaded binaries do not run on NixOS.
- Only x86_64-linux of the Nix package was built; aarch64-linux was evaluated.
- Nothing was published: the release workflow that tags and runs `goreleaser release` belongs to step 15.12 (CI) and 15.16 (v1.0).

See: [docs/guides/packaging.md](../guides/packaging.md), [ADR-009](009-embedded-runtime.md), [ADR-055](055-cross-compilation-targets-and-toolchain.md), [Phase 15 §2.4](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#24-packaging-and-install).
