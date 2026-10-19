# Installing gocpp: go install, release archives, Nix

gocpp is one program: the C++ runtime and the standard library are inside the binary
([ADR-009](../adr/009-embedded-runtime.md)). The first build extracts the runtime into
`$XDG_CACHE_HOME/gocpp/rt/<version>-<hash>/` and compiles it once per compiler and flags. What it
does **not** carry is a C++ compiler: g++ 13 or newer, or clang++ 17 or newer, must be on the `PATH`
(`gocpp env` shows the one it found), except in the Nix package, which brings its own.

## Platforms and requirements

- **Linux, x86-64**: the platform that the tests run on every day.
- **Linux, arm64**: the release archives and the Nix package are built for it; there is no arm64 test run yet
  (plan step 15.12).
- **Windows 10 or later, x86-64**: `gocpp.exe` builds and runs Windows programs on a Windows host (it is tested on a
  real Windows 11 PC), and Linux can cross-build them ([guide](windows-cross.md)). The C++ compiler is a MinGW-w64
  one.
- **macOS** is not a target.
- **C++ compiler**: `g++` 13 or newer, or `clang++` 17 or newer (the Nix package brings GCC 15 and clang 21).
- **`go install`** needs a Go toolchain of version 1.25 or newer (`go.mod` says `go 1.25.0`); the release archives and
  the Nix package need no Go.

## go install

```sh
go install github.com/huuhait/gocpp/cmd/gocpp@latest      # or @v1.2.3
gocpp version
```

```
gocpp v1.2.3 (phase 15) go1.25.14 linux/amd64
runtime 8e22f710058748ee
```

The first line is the version (`dev` for a build in a checkout), the second names the embedded runtime: it
is the cache folder `rt/<version>-<hash>` (the hash is the same for every build of the same sources).

Why it works, and what keeps it working (`internal/packaging`, `make package-check`):

- The module path is `github.com/huuhait/gocpp` and `go.mod` has no `replace` or `exclude` directive
  (`go install pkg@version` refuses a module with one).
- Every tracked file fits a Go **module zip**: no two names that differ only in case, no symbolic links, no
  characters a zip name may not have (a name with an apostrophe, for example). The proxy refuses the whole
  module otherwise, and a local `go install ./cmd/gocpp` does not notice.
  `TestGoInstallOfATaggedVersion` builds the module zip of the working tree, serves it from a `file://` proxy
  and runs the real `go install ...@v0.0.1-rc.1`, so the go command checks the names while it unpacks, and then
  runs the binary away from the source tree (version, and a program when a C++ compiler is there).
- Every file the program embeds is tracked: nothing needs `go generate` at install time.
- `internal/version.Version` is `"dev"`, and an `init` takes the module version that `go install` wrote into the
  binary (`debug.ReadBuildInfo`) unless Go stamped a repository (a build in a git checkout stays `dev`).

## Release archives (GoReleaser)

`.goreleaser.yaml` builds `gocpp` for linux amd64, linux arm64 and windows amd64, static (`CGO_ENABLED=0`),
`-trimpath`, with `internal/version.Version` set to the tag. Archives are
`gocpp_<version>_<os>_<arch>.tar.gz` (`.zip` for Windows): the program, `LICENSE` and `README.md`. The
checksums are in `checksums.txt` (sha256), and a release is created as a **draft**. The notes name the pinned
llvm-mingw release (`internal/toolchain.Release`, checked by a test).

Try the whole thing without a release (nothing is published, the output goes to `dist/`):

```sh
nix shell nixpkgs#goreleaser -c goreleaser check
nix shell nixpkgs#goreleaser -c goreleaser release --snapshot --clean --skip=publish
```

A real release is `goreleaser release --clean` on a `vX.Y.Z` tag, by the release pipeline (steps 15.12 and 15.16),
not from a development machine.

Windows: the zip holds `gocpp.exe`. A Windows program needs a MinGW-w64 `g++` (or `clang++`): one on the `PATH`
or in a known folder (MSYS2, Scoop and so on), else the pinned llvm-mingw, which `gocpp build` downloads once. The
download is pinned for Linux (amd64 and arm64) and Windows (amd64) hosts (`internal/toolchain.Archives`); the
rules are in [the build spec](../spec/build.md#gocppexe-on-a-windows-host).

## Nix

```sh
nix run github:huuhait/gocpp -- run hello.gp
nix build github:huuhait/gocpp        # ./result/bin/gocpp
nix profile install github:huuhait/gocpp
```

The package is `gocpp` wrapped with Nix's `g++` and `clang++` (GCC 15 and clang 21 of the pinned nixpkgs),
binutils (`ar`, `nm`), `llvm-ar` and `pkg-config` first on the `PATH`, so it works on NixOS, where no compiler is
installed by default. The wrapped compilers bring their own libstdc++ and libc paths. `GOCPP_CXX` or `CXX`
still choose another compiler. The build's install check compiles and runs a program with both compilers.

The package is built for x86_64-linux (tested) and aarch64-linux (evaluated only). The dev shell
(`nix develop`) is separate and unchanged.

When `go.mod` or `go.sum` change, the `vendorHash` in `flake.nix` goes stale; `TestFlakeVendorHash` fails with
the steps (set `lib.fakeHash`, run `nix build .#gocpp`, copy the hash, update the stamp).
