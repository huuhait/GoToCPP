# Installing gocpp: go install, release archives, Nix

**Release status:** v0.1 is not published by the current preparation work. The
[release-candidate notes and readiness checklist](release-candidate.md) distinguish available
source, historical packaging checks and open acceptance gates. Commands that name a remote
version below describe the installation mechanism, not an existing approved v0.1 artifact.

gocpp is one program: the C++ runtime and the standard library are inside the binary
([ADR-009](../adr/009-embedded-runtime.md)). The first build extracts the runtime into
`$XDG_CACHE_HOME/gocpp/rt/<version>-<hash>/` and compiles it once per compiler and flags. What it
does **not** carry is a C++ compiler: g++ 13 or newer, or clang++ 17 or newer, must be on the `PATH`
(`gocpp env` shows the one it found), except in the Nix package, which brings its own.

## Platforms and requirements

- **Linux, x86-64**: the platform that the tests run on every day.
- **Linux, arm64**: GoReleaser configures an archive for it and the Nix package evaluates for it;
  there is no native arm64 test run yet (plan step 15.12). A configured target is not a validated release.
- **Windows 10 or later, x86-64**: `gocpp.exe` builds and runs Windows programs on a Windows host (it is tested on a
  real Windows 11 PC), and Linux can cross-build them. The C++ compiler is a MinGW-w64 one, which gocpp downloads if there is none.
  Start with [gocpp on Windows](windows.md); [Windows from Linux](windows-cross.md) is the cross build.
- **macOS** is not a target.
- **C++ compiler**: `g++` 13 or newer, or `clang++` 17 or newer (the Nix package brings GCC 15 and clang 21).
  These are minimum requirements, not a tested-version matrix. Typed interop separately needs
  Clang 17+ for probes; its recorded matrix ran Clang 21 only, not 17–20, and the default remains
  opaque ([typed interop status](../spec/typed-interop.md#14-the-tests-of-step-1212)).
- **`go install`** needs a Go toolchain of version 1.25 or newer (`go.mod` says `go 1.25.0`); the release archives and
  the Nix package need no Go.

## go install

For an approved release, replace the placeholder with its exact published tag:

```sh
go install 'github.com/huuhait/gocpp/cmd/gocpp@<approved-tag>'
gocpp version
```

`@latest` is a moving module query (it can resolve a pseudo-version when there is no release),
not a pin to this candidate. No `v0.1` tag is created by these instructions.

The first line prints the version, completed-phase marker, Go build toolchain and host OS/architecture.
A build from this checkout reports version `dev` and `(phase 05)`, not `v0.1` or `phase 09`.
The second line is `runtime <16-hex-digit-hash>`; the extracted runtime uses the cache folder
`rt/<version>-<hash>`. The exact hash depends on the embedded sources, not on an example here.
The Go build toolchain version is distinct from the ported standard library's Go 1.25.14 provenance.

### Try the source without installing it globally

From a checkout at the revision you intend to test, with dependencies already available:

```sh
go build -o ./bin/gocpp ./cmd/gocpp
./bin/gocpp version
```

Or use `nix develop -c go build -o ./bin/gocpp ./cmd/gocpp` with the repository's development
shell. This writes only the local binary; it does not tag a release or alter a user-wide install.
Copy the following into `hello.gp`:

<!-- program: 09 go -->
```go
package main

import (
	"fmt"
	"strings"
)

func main() {
	fmt.Println(strings.Join([]string{"hello", "gocpp"}, ", "))
}
```

<!-- output -->
```text
hello, gocpp
```

Run or build it (POSIX shell, from that checkout):

```sh
GOCPP_GATE_PHASE=09 ./bin/gocpp run hello.gp
GOCPP_GATE_PHASE=09 ./bin/gocpp build -o hello hello.gp
./hello
```

Both executions print the output above. Phase 09 is opt-in; a package being embedded does not
remove its gate. On Windows use `$env:GOCPP_GATE_PHASE = "09"` in PowerShell and the `.exe`
paths; see the [Windows guide](windows.md) for the host toolchain and console setup.

Why it works, and what keeps it working (`internal/packaging`, `make package-check`):

- The module path is `github.com/huuhait/gocpp` and `go.mod` has no `replace` or `exclude` directive
  (`go install pkg@version` refuses a module with one).
- Tracked files must fit a Go **module zip**: no two names that differ only in case, no symbolic
  links, no reserved Windows basenames (such as `aux.go`), and no forbidden characters (such
  as an apostrophe). The proxy refuses the whole module otherwise, and a local
  `go install ./cmd/gocpp` does not notice.
  `TestGoInstallOfATaggedVersion` builds the module zip of the working tree, serves it from a `file://` proxy
  and runs the real `go install ...@v0.0.1-rc.1`, so the go command checks the names while it unpacks, and then
  runs the binary away from the source tree (version, and a program when a C++ compiler is there).
- The 9.13a review found that the local-proxy install failed before compilation because
  `internal/typedinterop/probe/aux.go` used a reserved Windows basename. Renaming it to
  `auxiliary.go` without changing its contents fixed the failure; the focused local-proxy
  installation then passed, including version and sample execution outside the source tree.
  This is not a remote installation or an archive/platform validation.
- Every file the program embeds is tracked: nothing needs `go generate` at install time.
- `internal/version.Version` is `"dev"`, and an `init` takes the module version that `go install` wrote into the
  binary (`debug.ReadBuildInfo`) unless Go stamped a repository (a build in a git checkout stays `dev`).

## Release archives (GoReleaser)

`.goreleaser.yaml` builds `gocpp` for linux amd64, linux arm64 and windows amd64, static (`CGO_ENABLED=0`),
`-trimpath`, with `internal/version.Version` set to the tag. Archives are
`gocpp_<version>_<os>_<arch>.tar.gz` (`.zip` for Windows): the program, `LICENSE`,
`THIRD_PARTY_NOTICES.txt` and `README.md`. The checksums are in `checksums.txt` (sha256), and
remote release creation is configured as a **draft**, not as permission to publish. The notes
name the pinned llvm-mingw release (`internal/toolchain.Release`, currently **20261006, ucrt**).
The C++ compiler itself is not in these archives. `CGO_ENABLED=0` makes the gocpp executable
static; it does not promise that every program it generates is statically linked.

Try the whole thing without a release (nothing is published, the output goes to `dist/`):

```sh
nix shell nixpkgs#goreleaser -c goreleaser check
nix shell nixpkgs#goreleaser -c goreleaser release --snapshot --clean --skip=publish
```

These are maintainer-only local snapshot commands, not required installation steps. A snapshot
is not a released or validated RC. The real-release pipeline (steps 15.12 and 15.16) remains
open; tagging, remote draft creation, upload and publication each require explicit release
approval. Do not run a real release command merely because local checks passed.

When an approved archive is available, verify its checksum against the matching trusted
release metadata, extract it into a new directory, and run the contained `gocpp version`
(`gocpp.exe version` on Windows). For example, on Linux:

```sh
sha256sum --check --ignore-missing checksums.txt
```

Keep the checksum file beside the downloaded archive and check that the intended filename
is reported `OK`. Checksums detect corruption; a checksum obtained from the same untrusted
source is not independent proof of authenticity. Put the binary on `PATH` only after checking
it, and keep the license/notice files alongside the distribution. No Go toolchain is needed
for an archive install; a C++ toolchain is still needed to build a program.

Windows: the zip holds `gocpp.exe`. A Windows program needs a MinGW-w64 `g++` (or `clang++`): one on the `PATH`
or in a known folder (MSYS2, Scoop and so on), else the pinned llvm-mingw, which `gocpp build` downloads once. The
download is pinned for Linux (amd64 and arm64) and Windows (amd64) hosts (`internal/toolchain.Archives`); the
rules are in [the build spec](../spec/build.md#gocppexe-on-a-windows-host).

## Nix

```sh
GOCPP_GATE_PHASE=09 nix run github:huuhait/gocpp -- run hello.gp
nix build github:huuhait/gocpp        # ./result/bin/gocpp
nix profile install github:huuhait/gocpp
```

The package is `gocpp` wrapped with Nix's `g++` and `clang++` (GCC 15 and clang 21 of the pinned nixpkgs),
binutils (`ar`, `nm`), `llvm-ar` and `pkg-config` first on the `PATH`, so it works on NixOS, where no compiler is
installed by default. The wrapped compilers bring their own libstdc++ and libc paths. `GOCPP_CXX` or `CXX`
still choose another compiler. The build's install check compiles and runs a program with both compilers.

The package is built for x86_64-linux (tested) and aarch64-linux (evaluated only). The dev shell
(`nix develop`) is separate and unchanged.

Remote `github:huuhait/gocpp` commands use a moving source revision, not this preparation
snapshot. From a checkout, `nix build .#gocpp` selects that source instead. Nix builds identify
as `dev-<revision>`; neither form establishes an approved release or native arm64 validation.

When `go.mod` or `go.sum` change, the `vendorHash` in `flake.nix` goes stale; `TestFlakeVendorHash` fails with
the steps (set `lib.fakeHash`, run `nix build .#gocpp`, copy the hash, update the stamp).

## Licensing and redistribution

Keep the [project BSD-3-Clause license](../../LICENSE) and
[third-party notices](../../THIRD_PARTY_NOTICES.txt) with binary distributions. The notices
reproduce Go's BSD terms (ported library, compiler copies and `golang.org/x/tools`), its patent
grant, and the fdlibm/Sun and Cephes notices retained in the ported math sources. The Nix output
installs these files in `share/doc/gocpp`; source/module distributions retain their original
copyright headers and license files too.

A generated application that incorporates these runtime/library components must preserve the
applicable notices in its distribution materials. External C/C++ libraries, Go modules and
compiler runtime libraries linked into that application have their own terms. The optional
llvm-mingw download is separate from the gocpp archive; retain its upstream notices if you
redistribute it. These files do not relicense dependencies or replace a review of the actual
binaries, dependencies and toolchains you choose to ship.
