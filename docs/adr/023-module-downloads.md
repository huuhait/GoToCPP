# ADR-023: Modules are downloaded from git hosts, with hashes in `gocpp.sum` and trust on first use

Status: accepted (2026-10-17, job 6.1c)

## Context

[ADR-020](020-modules-gocpp-mod.md) planned downloads for Phase 15 and Go's proxy and checksum database.
The user decided on 2026-10-17 (TODO Q14, Q16) that libraries from the internet are used now: C and C++
libraries on GitHub and similar hosts are modules, and a project downloads them like Go modules, with a hash
in `gocpp.sum`, a read-only module cache and `gocpp mod vendor` for offline builds. Those repositories have
no `go.mod`, no `gocpp.mod` and no module proxy: Go's `proxy.golang.org` would make a zip with a made-up `go.mod`
(a different tree), and its checksum database knows only Go modules.

## Decision

- **Source**: the archive of a tag (`vX.Y.Z`, also tried as `X.Y.Z`) or of a commit (a pseudo-version
  `v0.0.0-<time>-<commit12>`) from the git host named by the module path: GitHub, GitLab and Codeberg (and
  hosts with the same URLs, or any URL template, set with `GOCPP_MODURL`). A corporate mirror is
  `GOCPP_MODPROXY` (Go's `GOPROXY` list syntax; the mirror serves `<module>/@v/<version>.tar.gz` or `.zip`
  and `@v/list`). Versions for `gocpp get` come from the git smart HTTP listing (`info/refs`), the one request
  that every git host answers with no token and no API. We do not use Go's proxy protocol as the default.
- **Hash**: Go's `h1:` directory hash of the extracted tree, with the names `module@version/file`, so the
  lines in `gocpp.sum` are in the format of `go.sum`, and the hash does not depend on how the archive was
  compressed (GitHub changed its compression once). The tree, not the archive, is what counts.
- **Trust on first use (TOFU)**: a module that `gocpp.sum` does not list is downloaded and its hash is
  recorded; every later download or use of the cache must match, and a mismatch is a hard `SECURITY ERROR`
  that keeps the module out of the cache. `GOCPP_MODFLAGS=-mod=readonly` records nothing and refuses a module
  that is not listed. The first download is trusted: that is the same trust as `go get` without
  `GONOSUMDB`, minus the checksum database.
- **No checksum database for now.** Go's sumdb is a transparency log of hashes of Go modules; building one
  for arbitrary repositories (or operating one) is a project of its own, and nothing in this design prevents a
  later `GOCPP_SUMDB` that is asked on first use. Until then, `gocpp.sum` belongs in version control, and
  reading a module's source before adding it is the user's defense against a bad first download.
- **Safe extraction**: nothing is executed; every symbolic and hard link is rejected (a link can point out of
  the tree, and library repositories that need links are rare); also `..`, absolute names, devices, `.git`
  folders (also as Windows and macOS see the name), names with special meaning on Windows or NTFS, refused on every system so the hash is the same everywhere (a library with such a file name must be vendored from a fixed copy), case-insensitive name clashes, one top-level folder only, size, header-count and decompressed-size limits; modes `0444`/`0555`.
- **The cache** (`<user cache>/gocpp/mod/<escaped path>@<version>/`, read-only) is made by a rename of a
  complete, hashed folder, so readers never need a lock; a file lock only saves the second download (a file system
  without locks works too). A build compares the hash file in the cache with `gocpp.sum`; it does not hash the
  files again (as Go): `gocpp mod verify` does. CI should use `GOCPP_MODFLAGS=-mod=readonly`.
- **Not Go's minimal version selection yet**: only the main module's `require` lines count (ADR-020 and
  `modules.md`). C libraries have no `gocpp.mod` to read requirements from.

## Alternatives

- **Go's proxy and sumdb**: only for Go modules; the zips have invented `go.mod` files; rejected as the default.
- **`git clone`**: needs git, runs the hooks of `git`'s configuration and `.gitattributes` filters on code that is
  not trusted yet, and is slower; the archive URLs are plain HTTPS GETs of data.
- **Hash the archive**: the hosts do not promise the same bytes twice; the tree is stable.
- **Follow safe symlinks**: possible later (links that stay in the tree); rejected for now since it is hard to get
  exactly right on every platform and the tree hash would depend on link handling.
- **Always readonly like `go build`**: Go's default is to fail on a missing `go.sum` line; the user asked for
  trust on first use, with `-mod=readonly` as the strict mode.

## Consequences

Good: `gocpp get github.com/x/y` and a build work like Go's, with a reproducible, auditable `gocpp.sum` and
offline builds through `gocpp mod vendor`; the code is plain Go in `internal/modfetch`, with no new dependency.

Bad: the first download is not independently verified; a library with symbolic links in its repository
can't be downloaded (vendor a fixed copy, or fork it); a Go proxy zip of a repository does not have the
hash of the host's archive (do not mix them for one module).

See [docs/spec/modules.md](../spec/modules.md#downloads).
