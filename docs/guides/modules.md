# Using modules: `gocpp get`, `gocpp.mod` and `gocpp.sum`

This guide is for a project that uses libraries from the internet: C and C++ libraries on GitHub, or gocpp and Go
modules. The reference is [docs/spec/modules.md](../spec/modules.md); the decisions are in
[ADR-023](../adr/023-module-downloads.md) and [ADR-070](../adr/070-version-selection-and-gocpp-get.md).

## Add a library

```text
$ gocpp get github.com/ocornut/imgui
gocpp: added github.com/ocornut/imgui v1.91.5
```

`gocpp get` asks the repository for its tags, picks the newest release, downloads it into the module cache
(`GOCPP_MODCACHE`), and writes two files: `gocpp.mod` (a `require` line) and `gocpp.sum` (the hash of what it
downloaded). Both belong in version control. Then import its header:

```go
import "github.com/ocornut/imgui/imgui.h"
```

A library that has a `gocpp.mod` (or a `go.mod`) says which other modules it needs; `gocpp get` adds those too, with
`// indirect`, at the versions that minimal version selection picks (the highest that anything requires).

## Choose a version

| Command | What it does |
|---|---|
| `gocpp get x` | the newest release (upgrade, if `x` is required already) |
| `gocpp get x@v1.2.3` | exactly that version; a lower one than you have is a downgrade |
| `gocpp get x@v1.2` | the newest release of the `v1.2` line |
| `gocpp get x@patch` | the newest patch release of the version you use |
| `gocpp get x@<v1.5.0` | the newest version below `v1.5.0` |
| `gocpp get x@main`, `x@abc1234` | a branch or a commit, as a pseudo-version |
| `gocpp get x@none` | remove `x`, and what needs it |
| `gocpp get -u` | upgrade every module to its latest version |
| `gocpp get -u x` | upgrade `x` and what it needs |
| `gocpp get -u=patch` | the same, to the latest patch releases only |

A downgrade lowers every module that needs more than you allow, to the highest version that does not (never a version
its author retracted), or removes it when it has no such version; everything else stays. Every change is printed: `added`, `upgraded a v1.0.0 => v1.1.0`,
`downgraded`, `removed`. Nothing is written unless all of it worked.

A tag `v2.0.0` of a library that has no `go.mod` is `v2.0.0+incompatible`, and for libraries that were never Go
modules (most C and C++ libraries: nlohmann/json has v1 and v3 tags) `latest` takes the newest tag, whatever its
major version. A library whose older releases *have* a `go.mod` stays on its compatible line for `latest`, as `go get`
does; to cross to the new major version, name it: `gocpp get lib@v3.1.0` (or `lib@v3`). Once the version in use is
`+incompatible`, `gocpp get lib` and `lib@latest` go on from there and never fall back to the older line. `gocpp get
lib@v2.0.0` is written `v2.0.0+incompatible` for such a library, so a later `gocpp get lib` finds the same version.

`gocpp get lib@main` records a pseudo-version, `v0.0.0-<commit time>-<commit>`. A later `gocpp get lib` (an upgrade) does not
move back to a release that is older than that commit; it moves to a newer one. To go back to a release on purpose,
name it (`lib@v1.4.0`) or use `lib@latest`.

`gocpp get lib@v1.0.0` for a library that is already required at that version but is not in the module cache changes
nothing, so it downloads and verifies nothing; `gocpp get` with no arguments (or `gocpp mod download`) fetches it.

## See why a module is there

```text
$ gocpp mod graph
example.com/app github.com/acme/lib@v1.1.0
github.com/acme/lib@v1.1.0 github.com/acme/dep@v1.2.0

$ gocpp mod why -m github.com/acme/dep
# github.com/acme/dep
example.com/app
github.com/acme/lib
github.com/acme/dep
```

## `gocpp.sum` and trust

The first time you download a module its hash is trusted and recorded; afterwards every download and every use of the
cache is compared with `gocpp.sum`, and a difference is a **SECURITY ERROR** that stops the build. The hashes are the
same as Go's (`go.sum`) for the same files. In CI, set `GOCPP_MODFLAGS=-mod=readonly`, so that a build never trusts a
module that nobody reviewed, and use `gocpp mod vendor` for builds without a network. `gocpp mod verify` hashes the
cache again.

## Local modules, forks and offline work

- A module on your disk: `replace example.com/lib => ../lib` in `gocpp.mod`. A module name with no dot (`mylib`) is
  only possible this way (it can't be downloaded).
- A fork: `replace github.com/x/y => github.com/me/y v1.2.4`.
- `exclude github.com/x/y v1.3.0` keeps a bad version out of the build, whatever other modules ask for.
- `GOCPP_OFFLINE=1`: nothing is downloaded. A query that needs the list of versions (`latest`) fails and says so; a
  version that is in the cache works.
- Two `gocpp get` commands at once in one project wait for each other (a lock on the project folder); the module cache is
  shared safely. A `gocpp.mod` with Windows line ends keeps them.
- gocpp reads the full module graph, as Go does for `go 1.16`; Go prunes it for a main module that says `go 1.17` or more.
- A company mirror: `GOCPP_MODPROXY=https://mirror.example/...`, or a folder in the layout of Go's module proxy:
  `GOCPP_MODPROXY=file:///srv/gomodules`.

## What `gocpp get` does not do

- It works on modules, not on packages: `-t` is accepted and changes nothing, and `-u` upgrades whole modules. `-d` is not
  accepted (`gocpp get` with no module downloads what `gocpp.mod` requires).
- It writes `gocpp.mod` in go.mod style: one space between words, a tab in blocks, one blank line at most. Comments and lines
  it does not know stay, but the whitespace of a hand-written file may change.
- A run that is killed can leave `.gocpp.sum-<n>` or `.gocpp.mod-<n>` files next to `gocpp.mod`; the next run removes the
  ones that are a day old (put them in `.gitignore` if you like).
- The module cache must be private to you: gocpp trusts the first download of a module (there is no checksum database yet) and
  keeps the hashes it compares with next to the module, so keep `GOCPP_MODCACHE` in a folder that only your account can
  write, and review the diffs of `gocpp.sum` like Go's `go.sum`.
- There is no `gocpp mod tidy`: a line that nothing needs any more stays until `gocpp get module@none` removes it.
- It never runs code from a module, and a module with a symbolic link in it can't be downloaded (vendor a copy).
