# ADR-070: Minimal version selection over `gocpp.mod` and `go.mod`, and `gocpp get` that writes the whole build list

Status: accepted (2026-10-18, job 15.14)

## Context

[ADR-020](020-modules-gocpp-mod.md) and [ADR-023](023-module-downloads.md) left version selection out: only the
main module's `require` lines counted, so a program had to list every module it imports, directly or not, and
`gocpp get` set one line to one version. Phase 15 §2.5 asks for Go's minimal version selection (MVS), `gocpp get`
with upgrade, downgrade and removal, and the two catches found in the audit: `golang.org/x/mod` refuses paths
without a dot, and `x/mod/zip` knows only `go.mod` as a module boundary.

## Decision

- **The build list is Go's.** Every module version that a requirement names is read (its `gocpp.mod`, else its
  `go.mod`; a module with neither, a C library, requires nothing) and each module gets the highest version that
  anything requires. The main module is selected whatever others require of it; the `exclude` lines of the main
  module make a requirement on that version count as nothing; `replace` changes where a module's files and
  requirements come from, not its version; `retract` lines play no part in the selection of the build list (they matter to the queries of
  `gocpp get`, which skip retracted versions, and give a warning; and, as in Go's `CheckAllowed`, a downgrade does not
  fall back to a retracted version: `modgraph.Reqs.Previous` skips them, reading the retract lines of the module's
  highest version). The code is `internal/mvs` (the algorithm: `NewGraph`, `BuildList` and `Edit`),
  `internal/modgraph` (the requirement graph over the module cache, proxies and git hosts) and
  `internal/modget` (the command). It is **not** built on `golang.org/x/mod`: the repository has no dependency on it,
  and its `CheckPath` and zip rules are the two catches. Version comparison is `gocppmod.CompareVersions`
  (semantic versioning with Go's `+incompatible` and pseudo-versions); the zip rules are `modfetch.ListModuleFiles`
  and `modfetch.CreateZip`.
- **It is checked against the go command.** `TestDifferentialBuildListAndGraph` (50 random graphs, `go list -m all` and
  `go mod graph`, same edges in the same order) and `TestDifferentialGet` (random `get` commands with versions, `latest`,
  `upgrade`, `patch`, `none`, prefix and comparison queries; build lists and success or failure) run the real `go` on
  a `file://` module proxy with `go 1.16` module files (no pruning); the graphs have prereleases, `/v2` paths, and
  libraries without `go.mod` whose v2 and v3 tags are `+incompatible`. `GOCPP_DIFF_COUNT=N` runs N graphs.
  Go's old `mvs.Downgrade` (cap every root at its version) is not what `go get` does since Go 1.17, and the
  differential test found it wrong, so gocpp has no such function: `get` is `mvs.Edit`, a port of `modload.editRequirements` for an unpruned graph
  (every module of the build list is a root; only the named modules have a cap; a root that needs a version above
  a cap is replaced by its previous version, again and again; the upgrades that the graph needs are accepted at the end),
  so a downgrade may raise another module.
- **`gocpp get` writes the whole build list into `gocpp.mod`.** One `require` line for every module of the result:
  without a comment for the modules that the user named, and `// indirect` for the others. Existing lines keep their
  place and comments. Go's `go.mod` after Go 1.17 lists the modules that provide packages; gocpp has no package-level
  analysis of the dependencies (a C++ library's headers are not Go packages), and the loader reads only the main
  module's lines, so the file must list every module that a build may need. `gocpp.mod` is a complete, reproducible
  lock of the requirements; there is no `gocpp mod tidy` yet, so a line that nothing needs any more stays until
  you remove it with `gocpp get module@none`.
- **`gocpp get` rewrites `gocpp.mod` in go.mod style.** The file goes through `gocppmod.Format`: one statement per line,
  the words of a line separated by one space, the lines of a block indented by a tab, several blank lines become one;
  comments, blank lines and statements that gocpp does not know stay as they were. Whitespace in a hand-written file may
  therefore change on the first `get`. `-d` (go get's "download only") is not accepted: with no argument `get` downloads
  what `gocpp.mod` requires, and `-t` is accepted and ignored.
- **The module cache must be private to the user.** gocpp trusts the first download of a module (there is no checksum
  database yet) and then compares every use of the cache with `gocpp.sum`, but the hash sidecars that it compares live in
  the cache itself: a cache that other users can write to lets them replace a module and its sidecar together. Keep
  `GOCPP_MODCACHE` in a folder only your account can write (the default, in your user cache folder, is). `gocpp.sum`
  itself belongs in version control and its diffs deserve review, as `go.sum`'s do.
- **A killed run can leave temporary files** (`.gocpp.sum-<n>`, `.gocpp.mod-<n>`, `.gocpp-<n>`) next to `gocpp.mod`; the
  next write that finds such a file more than a day old removes it. Add them to `.gitignore` if you want to be sure that
  nobody commits one.
- **`gocpp.mod` and `gocpp.sum` change together or not at all.** `get` collects the hash lines (`modfetch.PendingSums`),
  computes everything, downloads, and only then writes `gocpp.sum` (merged with the file on disk) and `gocpp.mod`, each by
  one rename, under a lock of the module cache (`get-<hash of the folder>`) so that two `gocpp get` commands in one
  project run one after the other. A failure before the writes changes nothing.
- **Dotless module paths and boundaries.** A module path with no dot in its first element is a local module: it needs a
  `replace` line with a folder (the loader and `get` accept it there, `modfetch` refuses to download it). The module
  boundary of a zip, of a hash and of the cache is a folder with a `gocpp.mod` or a `go.mod`; version control folders and a
  `vendor/` folder made by `gocpp mod vendor` are left out. Any other `vendor/` folder belongs to the module (C
  libraries keep their dependencies there; Go would drop it).
- **Module proxies.** `GOCPP_MODPROXY` may name a `file://` folder in the layout of Go's proxy protocol (`@v/list`,
  `.info`, `.mod`, `.zip`): for tests, an offline mirror and a company's internal modules. The requirements of a module
  that a proxy serves come from the proxy's `.mod` file, which is a `go.mod` (a proxy cannot serve a `gocpp.mod`); the
  requirements of a module from a git host come from the archive.
- **Queries** are Go's (`latest`, `upgrade`, `patch`, `none`, versions, `vN` and `vN.M` prefixes, comparisons, branches, tags
  and commits), including Go's rule for `+incompatible` (`queryMatcher.filterVersions`): those versions are candidates when
  the version in use is one, when a bound names a major version that the path can't have, or when the highest compatible
  version has no module file of its own (a library that was never a Go module: its v2 and v3 tags are its releases, as
  for most C and C++ libraries). A repository whose v1 releases have a `go.mod` and whose v3 tags have none stays on v1 for
  `latest`: name the version (`gocpp get lib@v3.1.0`) to cross. A last `/vN` of any module path (not only on git hosts) is
  the major version suffix, as in `module.SplitPathVersion`. A bound that carries `+incompatible` (`<v3.0.0+incompatible`)
  names a version that a path with no `/vN` can have, so it does not ask for the incompatible versions. An explicit
  `lib@v2.0.0` of a path with no `/vN` is written `v2.0.0+incompatible` (as Go does), so a later plain `get lib` does not
  flip it to the other name of the same tree and download it twice. `upgrade` and `patch` (and so a plain `get lib`) never
  move from a pseudo-version to a release whose commit is chronologically older (Go's rule in `queryProxy`): the time of
  the release is the proxy's `.info` time, or, for a git host, the time of the files of its archive, so that case downloads
  the archive of the candidate tag once more than usual (and throws it away if the pseudo-version stays). `latest` and an
  explicit version still go where they are told.
- **The graph is not pruned.** Go reads a pruned graph for a main module that says `go 1.17` or more; gocpp always reads the
  full graph (as Go does for `go 1.16` and below), which is the graph the tests compare.

## Alternatives

- **Depend on `golang.org/x/mod`.** Rejected: a new dependency for semver, a module path check and a zip routine that
  all need patching for gocpp (dotless paths, `gocpp.mod` boundaries, no `go.mod` required).
- **Write only the named modules to `gocpp.mod`** (like Go before 1.17, or tidy). Rejected: the loader would then need MVS
  on every build, with a network or a cache full of module files; the explicit file is simpler and reproducible.
- **Package-level `get -u`.** Not possible without analysing imports of C/C++ headers; `-u` works on modules:
  it takes the named modules (all, if none is named) and what they require to their latest versions.

## Consequences

Good: `gocpp get` behaves like `go get` for the module graph and is tested against it; a project's `gocpp.mod` plus
`gocpp.sum` pins everything; builds and `mod vendor` keep reading one file.

Bad: `gocpp.mod` has more lines than Go's `go.mod` would; `-t` does nothing; `mod tidy`, `mod edit` and a checksum
database are still missing; a proxy cannot carry the requirements in a `gocpp.mod`.
