# Modules and `gocpp.mod`

A gocpp project is a **module**: a folder tree with a `gocpp.mod` file at its top. The file
names the module, says which other modules it needs and where they are on disk, and holds the
settings for the whole module (the C++ standard, compiler flags, `pkg-config` libraries). This
document says what the file looks like, how gocpp finds the module of a folder, and how an import
path becomes a folder.

| | |
|---|---|
| Status | v0, written with [Phase 06 step 6.1](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#3-steps): the file, the module root, `require`, `replace` and `vendor/`; and with job 6.1c: **module downloads** (`gocpp get`, the module cache, `gocpp.sum`, `gocpp mod vendor`, see [Downloads](#downloads)). Building a program of several packages is job 6.2/6.3 (the packages of the module, of required, replaced and downloaded modules; [cpp-mapping.md](cpp-mapping.md#files), [ADR-025](../adr/025-split-layout.md)); the cached, parallel build graph is steps 6.5 to 6.7; importing C/C++ headers by module path (6.1b) and building a library from its `cxx` recipe (6.1d, [C and C++ libraries](#c-and-c-libraries-cxx-recipes)) are in |
| Source | [ADR-020](../adr/020-modules-gocpp-mod.md), [ADR-023](../adr/023-module-downloads.md), [ADR-070](../adr/070-version-selection-and-gocpp-get.md) (version selection, step 15.14), [Phase 06 §2.1](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#21-the-module-file-gocppmod), [Phase 01 §2.6](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#26-package-loading-and-the-importer) |
| Code | `internal/load` (`modules.go`, `resolve.go`), `internal/load/gocppmod` (the file format) and `internal/modfetch` (downloads) |
| See also | [language.md](language.md) ([D11](language.md#d11-gocpp-directives-instead-of-cgo-lines)), [interop.md](interop.md) |

**Not here yet:** the checksum database, and the shared recipe index and CMake fallback for
big C and C++ libraries (open, TODO Q14). C and C++ libraries with a build recipe are in
[C and C++ libraries](#c-and-c-libraries-cxx-recipes). A module that is required and neither replaced by a local folder nor vendored is
downloaded, as [Downloads](#downloads) says.

## The file

```text
module github.com/huuhait/game

gocpp 0.1                    // minimum gocpp version
cxx c++20                    // C++ standard for generated + user C++ files

require github.com/huuhait/mathx v0.3.0
replace github.com/huuhait/mathx => ../mathx

cflags -DGAME_DEBUG
pkg-config sdl2
```

The syntax is that of `go.mod`: one statement per line, a verb and its arguments, `//` comments, and
blocks with the verb once:

```text
require (
	example.com/a v1.2.3
	example.com/b v0.1.0 // indirect
)
```

Strings may be quoted (`"..."` or `` `...` ``). A `//` outside quotes starts a comment, so a flag
with `//` in it must be quoted: `cflags "-DURL=http://x"`. A file may be at most 16 MiB (also
`vendor/modules.txt`): a larger one is an error, since a dependency's file is read, too.

| Statement | Meaning |
|---|---|
| `module path` | The module path, the import path of the module's root folder. Exactly one. A local module may have a path without a dot (`myapp`) |
| `gocpp 0.1` | The oldest gocpp that can build the module: `0.1` or `0.1.2`. It is read and checked for its form; gocpp does not compare it with its own version yet |
| `cxx c++20` | The C++ standard of the generated code and of the module's C++ files: `c++NN`, `gnu++NN` (`c++2b`, `c++2a` too) |
| `require path version` | A module that the module needs, with a version `vX.Y.Z` (a prerelease, such as `-rc.1`, without leading zeros in its numbers, and a `+build` part are allowed; `v1` and `v1.2` are not, which is stricter than Go). `+build` parts are dropped, except `+incompatible`, as the go command writes versions. `// indirect` marks one that only another module needs |
| `replace old [version] => dir` | Use the local folder `dir` for the module `old` (see below). With a version, only for that version |
| `exclude path version` | Never use that version: a requirement on it (in any module file of the graph) counts as nothing, and `gocpp get` does not choose it. Only the main module's lines count. For a module whose path has no `/vN`, `exclude lib v2.0.0` and `exclude lib v2.0.0+incompatible` are the same line (Go insists on the second spelling and rejects the first when it reads `go.mod`; gocpp reads `gocpp.mod` leniently and matches both) |
| `retract version` or `retract [low, high]` | In a module's own file: its author says that those versions must not be used (the comment above or after the line is the reason). Read from the highest version of the module by `gocpp get`, which skips them and warns |
| `cflags flag...`, `ldflags flag...` | Compile and link flags for the whole module |
| `pkg-config package...` | `pkg-config` libraries for the whole module |

The arguments of `cflags`, `ldflags` and `pkg-config` are split like the arguments of a `//gocpp:`
directive: a single, double or back quote keeps white space (also in the middle of an argument:
`-DMSG="a b"` is one flag, `-DMSG=a b`), and a backslash makes the next character ordinary. They
are only read here; the flag allow-list of the directives
([Phase 04 §2.9](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md)) is applied by the
build, when it uses them.

**Unknown statements are kept.** A statement with a verb that gocpp does not know (`go 1.25`,
`toolchain ...`, a verb of a later gocpp) and a block of such a verb are read and written back
unchanged. A verb is lower-case letters, digits and `-`; anything else at the start of a line (a module
path outside its `require ( ... )` block, a quote, a version) is an error, and so is a keyword of
gocpp.mod in a wrong shape: `require(` with no space, `require a (`, a stray `)` or `}`. So a mistake
is never silently ignored.

**Recipes are opaque.** A brace block, such as the recipe of a C++ library (`cxx path { ... }`), is not read at
all: its lines are kept as they are up to the `}` that matches the `{`. A line that ends with `{` or `(`
inside opens a block of its own, and a line that is only `}` or `)` closes it (a line that starts with `}` and ends with
`{`, such as `} else {` of an `if` block in a recipe, closes one block and opens the next), so
```text
cxx example.com/x {
	define {
		A=1
	}
	pkg-config glfw3
}
```
is one block; nothing in it is a module-wide setting, and it may hold anything (also `/* */` or
characters that are not allowed elsewhere). `gocpp.mod` is therefore safe to share with tools and
newer gocpp versions. The lines of a block are read, with their own rules, only for a module whose
header the program imports ([C and C++ libraries](#c-and-c-libraries-cxx-recipes)); gocpp.mod itself, and
`Format`, never look inside. A block that is not closed before the end of the file is an error at the last line,
as in Go, and its lines are not read as statements.

**Errors** read like the go command's: `gocpp.mod:3: usage: require module/path v1.2.3`, with the
column after the line when the statement is inside a block (`gocpp.mod:5:2: ...`). All the
statements that are wrong are reported at once, and the valid ones are still used: a mistake in a
`require` line does not hide the module path. The messages include `usage: ...` for a wrong number of
arguments, `repeated module statement`, `invalid module path "x/../y"`, `invalid module version
"v1": must be of the form v1.2.3`, `replacement module without version must be directory path
(rooted or starting with . or ..)`, `multiple replacements for x (the first is at line 3)`,
`mod files must use // comments (not /* */ comments)` and `syntax error (unterminated block ...)`.

`require` of one module twice is allowed, also with two versions (go.mod files have them): the highest
counts, at the place of the first line. `gocpp get` writes one line for each module.

## The module root

The module of a folder is the one whose `gocpp.mod` is in the folder or in the nearest folder
above it. This works from any subfolder, and the nearest file wins: a `gocpp.mod` inside a module
starts another module. A folder with no `gocpp.mod` above it is in **no module**, and nothing changes
for it: a program of `.gp` files named on the command line (`gocpp run main.gp`) has the import path
`command-line-arguments` and can import only the standard library. A `gocpp.mod` with no usable
`module` line is still the module root (the errors say what is wrong), so the rest of the tree is not
silently taken for another module.

The import path of a package is the module path followed by its folder relative to the module root
(`github.com/huuhait/game/engine/physics`).

## Import path to folder

For an import of `path` in a package, gocpp looks in this order:

1. **The standard library** (`fmt`, `strings`, ...). A standard package can only import standard
   packages.
2. **The module that contains the path.** Of the package's own module and the modules that the main
   module requires, the one with the longest module path that is a prefix of `path` wins (as in Go, so
   `example.com/m/sub` can be its own module inside `example.com/m`).
   - The package's own module: the folder is the module root plus the rest of the path. A folder
     with its own `gocpp.mod` inside the module is another module, and its packages need a `require`.
   - A required module: see below.
3. **Otherwise**: `no required module provides package path`, or `... gocpp.mod file not found in
   DIR or any parent directory` outside every module. If a `replace` line names the module but no
   `require` line does, the message says so and what to add.

The **main module** is the one of the root packages (the folder of `gocpp check` and `gocpp build`).
Only its `require`, `replace`, `exclude` and `vendor/` count, for every package of the program, also for the
packages of required modules; as in Go, the `replace` and `exclude` lines of other modules are ignored. The
`gocpp.mod` of a required module is read for its module line and its errors, and for the settings it will
give to its own packages. The loader follows the main module's `require` lines only, so they must name every
module that a program imports, directly or not (the indirect ones are marked `// indirect`): `gocpp get`
keeps them there for you, from the requirements of the modules that it adds ([Version selection](#version-selection)).

### `replace` with a local folder

```text
require github.com/huuhait/mathx v0.3.0
replace github.com/huuhait/mathx => ../mathx            // relative to the main module's folder
replace example.com/vecs v1.0.0 => /home/me/src/vecs     // absolute
```

A folder is absolute or starts with `./` or `../` (`.` and `..` too; the Windows spellings `.\`,
`C:\...` are read, too, but write `/` in a `replace` path so that the file works on every system:
`..\x` is not a path on Linux). The folder must exist. If it has a `gocpp.mod` file, that file must declare
the replaced module path: `module github.com/huuhait/mathx is replaced by ../other, but the gocpp.mod
there declares its path as example.com/other`. A folder **without** `gocpp.mod` is accepted: a git
clone of a plain C or C++ library is the module of the `replace` line, with its headers and any `.gp`
packages in it (the usual way to work on a library locally). The folder is checked when a package imports the
module, so a stale `replace` does not break a build that does not use it. `gocpp list` (step 6.12) will
check them all, with `load.ListModules`.

A `replace` line with a version replaces only that version of the module; one without a version
replaces every version, and the more specific line wins. A replacement that is another module with a
version (`replace a => b v1.0.1`) is downloaded as `b@v1.0.1` (with its own line in `gocpp.sum`) and its files
are the module `a`; `gocpp mod vendor` can't vendor such a replacement yet.

A package of a replaced module has the replaced module as its module, with the version of the
`require` line.

### `vendor/`

If the main module has the file `vendor/modules.txt`, it is **vendored**: every required module is
taken from `vendor/<module path>/` (its files, as `gocpp mod vendor` copied them; a `gocpp.mod` in the
folder is not used), also a replaced one, and nothing is read from the replacement folders, so a vendored project builds without the
other checkouts. A `vendor` folder **without** `modules.txt` is not vendoring: C and C++ projects often
keep third-party sources in a folder of that name, and gocpp leaves it alone.

`vendor/modules.txt` has the format of Go's:

```text
# example.com/vecs v1.0.0 => ../vecs
## explicit
example.com/vecs
# github.com/huuhait/mathx v0.3.0
## explicit
github.com/huuhait/mathx
github.com/huuhait/mathx/geom
```

`# path version` starts a module, `=> dir` records its `replace`, `## explicit` marks a module that
`gocpp.mod` requires itself, and the other lines list the vendored packages. gocpp checks that the file
agrees with `gocpp.mod`, as the go command does, and reports the differences once, for example:

```text
vendor/modules.txt: inconsistent vendoring in /src/app:
	github.com/huuhait/mathx@v0.3.0: is explicitly required in gocpp.mod, but not marked as explicit in vendor/modules.txt
	example.com/vecs@v1.0.0: is replaced in gocpp.mod, but not marked as replaced in vendor/modules.txt

	To ignore the vendor folder, delete vendor/modules.txt.
```

A folder in `vendor/` that is loaded by its path, as the language server loads every package (`gocpp check
./vendor/example.com/lib/sub`), is the package `example.com/lib/sub` of the vendored module, when
vendoring is on and consistent, not `example.com/app/vendor/example.com/lib/sub`.

Every import of a required module then fails with a short message that points to those errors. The
check runs when a package first imports a required module. `gocpp mod vendor` writes the folder
([below](#gocpp-mod-vendor)).

## Downloads

A required module with no `replace` line and no `vendor/modules.txt` is downloaded when a package imports
it (job 6.1c, [ADR-023](../adr/023-module-downloads.md)). The modules are **plain repositories on a git
host**, as C and C++ libraries are: no gocpp file is needed in them (if there is a `gocpp.mod`, its `module`
line must name the module). What happens for `require github.com/ocornut/imgui v1.91.5`:

1. **Where from.** The module path says the repository: `github.com/owner/repo`, `gitlab.com/owner/repo`
   and `codeberg.org/owner/repo` (the first three elements), anything after them is a folder of the module in
   the repository (its tags are `folder/vX.Y.Z`, as in Go); `host/group/sub/repo.git/folder` names a
   repository with more path elements (GitLab subgroups). A last element `/v2` (up to any number) is a major
   version suffix, as in Go: the version must be `v2.x.y`. A path with no `/vN` accepts any major version (C
   libraries have `v3.4.0` and no `/v3` path). The version is the tag `vX.Y.Z` (also tried as `X.Y.Z`, many C
   libraries tag with no `v`), or a **pseudo-version** `v0.0.0-yyyymmddhhmmss-commit12` (`vN.0.0-...` for a path that ends in `/vN`), the commit that
   starts with those 12 hex digits (for libraries with no tags, such as `github.com/nothings/stb`). The
   archive URLs are `https://github.com/o/r/archive/refs/tags/<tag>.tar.gz` (and `.../archive/<commit>.tar.gz`),
   `https://gitlab.com/o/r/-/archive/<tag>/r-<tag>.tar.gz` and `https://codeberg.org/o/r/archive/<tag>.tar.gz`.
2. **Download** over HTTPS (TLS is always verified; plain `http://` is refused, also as a redirect) of the
   archive into a temporary file, with a User-Agent `gocpp/<version> (modfetch)`, at most 5 redirects, a
   limit of 512 MiB for the archive and 10 minutes for the transfer, and 3 tries for the answers 429, 502,
   503 and 504. A 404 or 410 tries the next URL or source; any other answer is an error. Credentials
   (`GOCPP_MODAUTH`) go to their own host only, not to the host of a redirect.
3. **Extract** the `tar.gz` or `zip` (the format is found from the first bytes, not from the URL) with the
   [checks below](#security-model). The archive must be one folder (`<repo>-<ref>/`), which is stripped.
4. **Hash** the files with Go's `h1:` directory hash, as `go.sum` has it: the SHA-256 of the lines
   `<sha256 of the file>  <module>@<version>/<name>` for every file, sorted by name, in base64. File modes,
   times and empty folders do not count. A module with a `gocpp.mod` has a second hash, of that file alone.
5. **Check and record** the hash in `gocpp.sum` ([below](#gocppsum)).
6. **Cache**: the files become read-only (`0444`, `0555` for an executable, folders `0555`; setuid, setgid and
   sticky bits are never set) and the folder is moved into the module cache in one rename.

The loader then uses the cache folder as the module folder. It prints `gocpp: downloading github.com/x/y
v1.2.3` on standard error when it downloads, and nothing when the module is in the cache. The language
server never downloads and never writes `gocpp.sum` (`modfetch.SetOffline`, which nests for several servers in one process): a module that is not in the cache is an error there, and the message says to run `gocpp mod download`. `gocpp check` and `gocpp build` may download; `load.ListModules` looks in the cache only (`modfetch.LookupDir`): it never downloads and never writes `gocpp.sum`.

### Commands

```text
gocpp get [-u[=patch]] [-t] [module[@query] ...]   change requirements, download, record hashes
gocpp mod download [module ...]      download what gocpp.mod requires
gocpp mod verify                     hash the cache (or vendor/) again and compare with gocpp.sum
gocpp mod vendor [-v]                write vendor/ for builds without a network
gocpp mod graph                      print the requirement graph (as go mod graph)
gocpp mod why [-m] module ...        print the chain of requirements that needs a module
```

`gocpp get github.com/ocornut/imgui` asks the repository for its tags with git's smart HTTP
(`<repo>.git/info/refs?service=git-upload-pack`, which every git host serves, with no token and no API) and
picks the version from the **query** after `@`:

| Query | Version |
|---|---|
| none, `upgrade` | `upgrade` for a module that `gocpp.mod` requires (the highest release at or above the version in use; from a pseudo-version never to a release whose commit is older), `latest` for a new one |
| `latest` | the highest release tag; a prerelease only if there is no release; no version tags at all: the head of the default branch as a pseudo-version. A version of a major version that the module path has no `/vN` for (`v2.0.0+incompatible`) is taken only if there is no other release: name it to get it. A retracted version is never taken (the retract lines of the module's highest version decide) |
| `patch` | the highest patch release of the version in use (error for a module that is not required yet) |
| `v1.2.3` | that version, with no listing (also a pseudo-version); a version below the one in use is a **downgrade**. `v2.0.0` and above of a path with no `/vN` is written `v2.0.0+incompatible` |
| `v1`, `v1.2` | the highest tag of that line |
| `<v1.5.0`, `<=v1.5.0`, `>v1.5.0`, `>=v1.5.0` | the highest version that satisfies `<` or `<=`; the lowest that satisfies `>` or `>=` |
| a branch or tag name, a commit (7 to 40 hex digits) | the pseudo-version of that commit; its time is the commit time that the archive's files have |
| `none` | remove the module, and downgrade or remove the modules that need it |

`gocpp get` works on **modules**: the argument is a module path (a package path is not resolved to its module), and a
version excluded by an `exclude` line is never chosen. It computes the new build list by minimal version selection
([below](#version-selection)), downloads the modules that were added or changed, and writes `gocpp.mod`
and `gocpp.sum` **together**, only when every argument worked: `gocpp.sum` first, then `gocpp.mod`, each by one
rename, under a lock of the module cache so that two `gocpp get` commands in one project run one after the other.
`gocpp.mod` gets one `require` line for every module of the build list: the modules that you named carry no
comment, the others `// indirect`; lines that exist are changed in place (comments, unknown statements and `cxx`
blocks stay as they are), and lines of modules that left the build list are removed. With no module and no `-u` it is
`gocpp mod download`. It prints a line for every module of the build list that changed, as `go get` does:
`gocpp: added github.com/x/y v1.2.3`, `gocpp: upgraded github.com/x/y v1.2.0 => v1.2.3`, `gocpp: downgraded ...`,
`gocpp: removed ...`, then `gocpp: warning:` lines (a named version that its module retracted) and `gocpp: note:`
lines (a named module that a `replace` line replaces: the download is not used). It does not touch `vendor/` (it says so
when there is one: run `gocpp mod vendor`). A `gocpp.mod` or `gocpp.sum` that is a symbolic link is not replaced (gocpp
writes by renaming a new file over the old one): edit the file it points to. `gocpp get` writes `gocpp.mod` in go.mod style
(`gocppmod.Format`): one statement per line, one space between the words of a line, a tab for the lines of a block, several blank
lines become one; comments and statements it does not know stay, but the whitespace of a hand-written file may change. `-d` is not
accepted. A temporary file (`.gocpp.sum-<n>`, `.gocpp.mod-<n>`, `.gocpp-<n>`) that a killed run left in the project folder is removed by
the next write once it is a day old. With downloads off (`GOCPP_OFFLINE=1`) a
query that needs a list of versions fails with a message that says so; naming a version that is in the cache works.

The flags: `-u` also upgrades the modules that the named modules need (every module of the build list when none is
named) to their latest versions, and what those need; `-u=patch` does the same to the latest patch releases; `-t` is
accepted for scripts written for `go get` and changes nothing (requirements are per module, not per package). The
flags may follow the modules.

### Version selection

The versions of the modules that a build uses are chosen as the go command does (minimal version selection,
[ADR-070](../adr/070-version-selection-and-gocpp-get.md)): the requirements of the main module's `gocpp.mod` and of the
module file of every module that they name, again and again, and for each module path the **highest** version that
any of them requires. The module file of a dependency is its `gocpp.mod`, else its `go.mod`, else (a C library) none.
With a module proxy the module file is the proxy's `.mod` (a `go.mod`); with a git host it is the file in the
archive. Details that follow Go and are tested against it (`go list -m all` and `go mod graph` on random graphs):

- **The main module** is selected whatever other modules require of it.
- **`replace`** changes where a module's files and requirements come from (a local folder, or another module version), not the version in the build list; a replacement folder or
  module must declare the replaced path (or its own).
- **`exclude`** (main module only): a requirement on an excluded version counts as nothing, and `gocpp get` never chooses it. `exclude lib v2.0.0` (a path with no `/v2`) also excludes `v2.0.0+incompatible`, the name that `gocpp get` and the list of versions use; Go rejects the first spelling, gocpp reads it leniently.
- **`retract`** (in the module file of a later version of a module) does not change the selection of the build list;
  `gocpp get` skips retracted versions for `latest`, `upgrade` and `patch` and as the version that a downgrade falls back
  to (Go's `CheckAllowed`), and warns when you name one. The retract lines are read from the module file of the highest
  version, which is not a module of the build: its hashes are not written to `gocpp.sum`, and `gocpp get mod@v1.2.3` never
  downloads that version's archive. A module proxy serves the `.mod` file alone, which is read; a git host has the file
  only in the archive, so the warning is given only if that version is already in the module cache.
- **Several `require` lines** for one module in one file (go.mod files have them) count as one, at the highest version.
- **A cycle** is fine. **A path with `/v2`** is a module of its own (`example.com/a` and `example.com/a/v2` are both
  in the build list). **`+incompatible`** is a version of the same module: `v2.0.0+incompatible` equals `v2.0.0` for
  comparison, and sorts after `v1.9.0`.
- **Pseudo-versions** sort by their time inside the same base version, and below the release that follows them.
- **A downgrade** (`get x@v1.1.0`, `get x@none`) is Go's `go get` (`mvs.Edit`): every module of the build list is kept
  at its version unless the named versions need otherwise; a module that needs more than a named module allows is
  replaced by its previous version, again and again, or removed when it has none, and a lower version may need a higher
  version of another module, which then goes up.
- **The graph is not pruned**: Go reads a pruned graph for a main module that says `go 1.17` or more, gocpp always reads
  the full graph (as Go does for `go 1.16`).
- **`+incompatible`**: a version `v2.0.0+incompatible` of a path with no `/v2` is a candidate of `latest`, `upgrade` and `patch` when
  the version in use is one, or when the highest compatible release has no module file of its own (most C and C++
  libraries); a repository whose v1 releases have a `go.mod` stays on v1: name the version to cross.
- **A module path with no dot** in its first element can't be downloaded: it needs a `replace` with a local folder (an error
  says so).
- The graph is read in parallel (8 module files at a time), and each module file once.
- **The module cache must be private to the user**: the first download of a module is trusted (no checksum database yet), and
  the hash sidecars that every later use is compared with live in the cache, so a cache that others can write to can be changed
  with its sidecars. The default folder (in the user's cache folder) is private; review `gocpp.sum` diffs as you review `go.sum`'s.

`gocpp mod graph` prints the graph (`module requirement` pairs, the main module with no version, in the order of `go mod
graph`); `gocpp mod why module ...` prints the shortest chain of requirements from the main module to each module. Neither
writes `gocpp.sum` (a module file that it does not list is accepted, but not recorded).

The files that belong to a **module zip** (for `CreateZip`, the hash and the cache): every regular file below the module
root except version control folders (`.git`, `.hg`, `.svn`, `.bzr`), `vendor/` made by `gocpp mod vendor` (it has a
`modules.txt`), and every folder that has a `gocpp.mod` or a `go.mod` (a nested module, with everything below it: the go
command knows `go.mod` only). Symbolic links and devices are errors.

#### `gocpp mod vendor`

Copies every required module, replaced (the local folder, without its own `vendor/`) or downloaded (the
files of the cache, **all** of them: a C++ library needs its headers and sources, not only `.gp`
packages), into `vendor/<module path>/`, and writes `vendor/modules.txt` in Go's format. The list of a
module has its `.gp` packages (a folder with a `.gp` file that is not a `_test.gp` file, outside `testdata`,
`vendor`, `.x` and `_x` folders and outside folders that have a `gocpp.mod`); a C-only module has none.
Afterwards the copy of each downloaded module is hashed again against the cache, and the written
`modules.txt` is checked against `gocpp.mod` (`gocppmod.CheckVendor`). The new folder is made next to
`vendor/` and swapped in, so a failure leaves the old one. A `vendor/` folder with no `modules.txt` (not made
by gocpp) is never replaced unless it is empty. `-mod=vendor` is automatic when `modules.txt` exists. A local `replace` folder is copied as it is (without its `.git` and its own `vendor/`): files in it that are not meant to be shared, such as `.env`, are copied too. A `.vendor-tmp-*` folder that a killed `mod vendor` left is removed by the next run when it is a day old.

### `gocpp.sum`

Next to `gocpp.mod` of the main module, in Go's format and with Go's hashes (the same as `go.sum` and `go mod download` give for the same zip): one line for each module version,
`module version h1:<base64>`, for a module that has a `gocpp.mod`, `module version/gocpp.mod h1:<base64>`, and for a module version whose module file was read from a `go.mod` (in the archive, or from a module proxy), `module version/go.mod h1:<base64>`. `gocpp get` records the `go.mod` line of every module version of the requirement graph that it read, as `go.sum` does.
The lines are sorted. Another gocpp that adds lines at the same time loses none (a lock, and the file is read
again before it is written). A module that gocpp.sum lists is checked on every download and on every use of the
cache (against the hash file that the cache keeps next to the module).

- **First use is trusted.** A module that `gocpp.sum` does not list is downloaded and its hash is added.
  `gocpp.sum` belongs in version control: from then on every machine checks against it. A build that records a hash says so on standard error (`gocpp: recorded github.com/x/y@v1.2.3 in gocpp.sum`). Use `GOCPP_MODFLAGS=-mod=readonly` in CI, so that a build never trusts a module that nobody reviewed.
- **A mismatch is a security error**, never a warning:

  ```text
  SECURITY ERROR
  github.com/x/y v1.2.3: checksum mismatch
  	downloaded: h1:...
  	gocpp.sum:  h1:...

  This download does not match the checksum that gocpp.sum recorded the first time. ...
  Nothing was added to the module cache.
  ```

  The module is not put into the cache, gocpp.sum is not changed, and the build stops. A cache that does not
  match is reported in the same way and is not used.
- **`GOCPP_MODFLAGS=-mod=readonly`** never writes `gocpp.sum`: a module that it does not list is an error
  (`missing entry for ... run "gocpp mod download ..."`) before anything is downloaded. The commands
  `gocpp get`, `gocpp mod download` and `gocpp mod vendor` always record, since the user asked for it.
- There is no checksum database (Go's `sum.golang.org`): the first download is trusted. That is a known limit
  of the design (see the ADR); a later job can add one.

### The module cache

```text
$GOCPP_MODCACHE   (default: <user cache folder>/gocpp/mod, $XDG_CACHE_HOME/gocpp/mod on Linux)
  github.com/ocornut/imgui@v1.91.5/     the files, read-only
  github.com/ocornut/imgui@v1.91.5~h1   their hash, and the hash of gocpp.mod
  github.com/!acme/!tiny@v1.0.0/        upper-case letters are written !lower-case, as in Go's cache
  lock/                                 lock files, one for each module version
```

A module folder appears all at once: it is extracted into a hidden folder next to its final place, hashed,
made read-only and renamed, so a gocpp that sees the folder can use it without a lock. Two gocpp processes that
download the same module wait for each other with a file lock (the second finds the cache filled and does not
download); if the file system has no file locks (NFS without lockd, FUSE, 9p, SMB: flock fails with ENOLCK, EOPNOTSUPP or ENOSYS) gocpp goes on without it and both download, and the rename has one winner
whose hash the other checks. A failed or stopped download leaves nothing but hidden `.tmp-*` and `.dl-*`
files, which the next download in the same folder removes when they are a day old. A cache folder that is
read-only (a shared cache on a read-only mount) works for modules that are in it; a download needs a writable
one. Files are read-only on purpose; `modfetch.CleanModCache(dir)` removes a whole cache (it makes the
folders writable first); the command `gocpp clean -modcache` (step 6.12) will call it.

### Environment variables

| Variable | Meaning |
|---|---|
| `GOCPP_MODCACHE` | The module cache folder (an absolute path), or `off` for none (then a module must be vendored or replaced). Default: `gocpp/mod` in the user cache folder; with neither `$XDG_CACHE_HOME` nor `$HOME` there is none |
| `GOCPP_OFFLINE=1` | Never use the network: a module that is not in the cache (or vendored) is an error that says so. The spellings are the same for module downloads and for the Windows toolchain download: `1`, `true`, `yes` and `on` turn it on; empty, `0`, `false`, `no` and `off` turn it off (any case, spaces around the word are ignored); any other word is on too, so that a typo never goes online |
| `GOCPP_MODFLAGS` | `-mod=readonly` (or `readonly`): do not write `gocpp.sum`; `-mod=mod`: the default |
| `GOCPP_MODPROXY` | Where to download from, in order, like `GOPROXY`: URLs of mirrors, `file:///folder` (a folder in the layout of Go's proxy: `<module>/@v/list`, `.info`, `.mod`, `.zip`), `direct`, `off`. `,` goes on after "not found" only, `\|` after any error. A mirror serves `<url>/<module path>/@v/<version>.tar.gz` (or `.zip`) and the list of versions `<url>/<module path>/@v/list`. Default: `direct` |
| `GOCPP_MODURL` | Rules for direct downloads, separated by spaces: `[module-prefix=]value`. A value without `{` is a base URL that replaces `https://<host>` for the built-in layouts (a GitHub Enterprise or a mirror with the same URLs); a value with `{...}` is a template for the archive URL with the fields `{path} {root} {host} {repo} {name} {subdir} {version} {ref}` (`{ref}` is the tag or the commit). The first rule whose prefix matches is used. This is also how a module on another host is downloaded |
| `GOCPP_MODAUTH` | Credentials, separated by spaces: `host=token` (a bearer token) or `host=user:password` (basic), for the exact host (with its port if the URL has one). Never sent anywhere else, not after a redirect |
| `GOCPP_MODCACERT` | A PEM file of extra certificate authorities (a company CA); TLS is always verified |
| `GOCPP_MODTIMEOUT` | The longest a download may take (a Go duration; default `10m`) |

`HTTPS_PROXY` and `NO_PROXY` apply, as for every Go program. A variable that is malformed is an error, not
ignored, and an error never shows a credential or a URL query.

### Security model

Downloaded code is **data** until the user builds it. gocpp never runs anything from a module: no script, no
hook, no recipe step. (The `cxx` recipe of job 6.1d only lists sources, include folders and flags, and the flags
go through the flag allow-list of [Phase 04 §2.9](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md);
the `gocpp.mod` of a module is untrusted input, with the 16 MiB cap.) What the download defends against:

- **A hostile archive** (an `ExtractError`, nothing is kept). Rejected: an absolute name; `..` in a name; a
  backslash, a NUL or another control character, or a name that is not UTF-8; a name or a path that is too
  long or too deep (1024 bytes, 255 for an element, 64 elements); an element called `.git`, also in the forms that Windows and macOS treat as `.git` (`.git.`, `.git `, `GIT~1`, `.git::$INDEX_ALLOCATION`, a `.git` with zero-width characters in it); combining marks (U+0300 to U+036F: the same name in the composed and the decomposed form is one file on macOS, so such a name is refused rather than extracted differently there); names that mean something special on Windows or in NTFS (a `:` in a name, a name that ends in a dot or a space, and the device names `CON PRN AUX NUL COM1-9 LPT1-9 CONIN$ CONOUT$` with or without an extension), refused on **every** system so that a module has the same files, and the same hash, everywhere (as Go's `module.CheckFilePath` does); a library that has such a file name can't be downloaded: vendor a fixed copy of it; **every symbolic
  link and hard link** (a link may point out of the tree, and a safe subset is not worth the risk: a
  library that has links needs a fork or a vendor copy), devices, FIFOs and other entry types; the same path
  twice, two paths that differ only in case or in format characters, or in the long s and the Kelvin sign (they would clash on Windows and macOS), a file where another entry needs a
  folder; more than one top-level entry; a file at the top level. Limits: 512 MiB downloaded, 2 GiB of
  files, 256 MiB for a file, 200 000 entries. Every tar header counts against the entry limit (a repeated folder header, a `./` entry and a pax header too), the decompressed stream is cut at the sizes of the files plus a small allowance for each header, and the data that headers carry (pax records) is bounded by that allowance, so a small download can't make gocpp work for long or fill memory; the sizes in a header are not trusted. The zip central directory is read in full before its entry count is checked, which the 512 MiB limit of the download bounds. Ctrl-C cancels an extraction and a hash (the second Ctrl-C ends the process). Files are created with `O_EXCL` in a folder that nothing else uses.
  The archive is read to its end, so a download that was cut off fails, also when only the gzip checksum is
  missing.
- **A changed module** (a moved tag, a compromised host or mirror): the hash in `gocpp.sum`.
- **A man in the middle**: TLS with the system roots (plus `GOCPP_MODCACERT`), minimum TLS 1.2, `https` only.
- **Leaked credentials**: sent to the configured host only, never after a redirect to another host.
- **A broken cache**: a module folder is complete or absent. A use of the cache compares the hash file that the cache keeps next to the module with `gocpp.sum` (as Go does); it does not hash the files again, so a file that someone changes in the cache is found by `gocpp mod verify`, not by a build.

What it does **not** defend against: a module that is malicious from its first download (the hash was
trusted at first use; there is no checksum database yet), and the code of a library that you then compile and
run: building a library runs its code in your program, as in every language. Read what you add.

### Not done yet

The checksum database; vendoring a `replace` that points to another
remote module; symbolic links in modules; package-level `gocpp get -u` and `-t` (everything is per module); the
requirements of a module that a proxy serves in a `gocpp.mod` (a proxy serves its `go.mod`); `gocpp mod edit`; two required
modules where one is a folder of the other (`github.com/o/r` and `github.com/o/r/sub`: `gocpp mod vendor` fails
on the files that both have); `gocpp mod tidy`
(`gocpp.sum` keeps the lines of versions that are not required any more); `gocpp clean -modcache` (the function
is `modfetch.CleanModCache`, the command is step 6.12).

## Known limits: the language server

The language server loads each package on its own, with the package's folder as the current folder, so
for it the "main module" is the module of the package that it is checking. Two things follow, to be fixed
in Phase 11 (TODO.md):

- A package of a replaced module that imports a module that only the application's `gocpp.mod`
  replaces is reported as `module ... is required, but is not available` in the editor, while
  `gocpp check` finds it. The language server has to pass the main module's folder to the loader.
- In a vendored project, the language server loads the vendored packages by their folders under
  `vendor/`. The loader now gives them the import path of the vendored module (see above), and a trial
  of the cache with such a project showed no error of its own, but no test of the server covers it.
- Module headers: a header of the package's own module resolves in the editor (go to definition opens the
  file); a header of a required module has the first limit above. The server needs no setting for it: it
  uses the loader, with the files of its snapshot, and the `-iquote` folders reach its probes through the flags of
  the package.

## Module headers

A C or C++ header is imported by the full path of the module that has it, like a package. A quoted
import whose **first path element has a dot** (a domain, as in Go's module paths) and that ends in a
header extension (`.h .hpp .hh .hxx .h++ .inl`) is a **module header**:

```go
import (
	"github.com/huuhait/game/native/physics.h"   // native/physics.h in this project's own module
	"github.com/ocornut/imgui/imgui.h"           // imgui.h in a required module (package imgui)
	"./local.h"                                  // next to the .gp file: not a module header
	"native/x.hpp"                               // next to the .gp file too: no dot in "native"
)
```

- **Which module.** Of the package's own module (`module github.com/huuhait/game` in `gocpp.mod`) and
  the modules that the main module requires, the one with the longest module path that is a prefix of the
  import path names the module, as for packages ([Import path to folder](#import-path-to-folder)); the
  rest of the path is the file in the module's folder. The own module needs no `require`
  (`github.com/huuhait/game/native/physics.h` is `native/physics.h` in the module root, from any
  package of the module). A required module is found like a package's module: `vendor/` if the project
  is vendored, else its `replace` folder, else the download hook (a stub for now: `gocpp can't download
  modules yet`). A required module needs no `gocpp.mod` file: a plain C or C++ repository works
  as a `replace` folder, a vendored module or a downloaded one; if the replacement folder has a
  `gocpp.mod`, its `module` line must match.
- **The name** of the header package is the file name, as for every header (`imgui.h` is `imgui`,
  `imgui_impl_glfw.h` is `imgui_impl_glfw`, `my-lib.h` is `my_lib`), and `rl "github.com/.../raylib.h"`
  renames it.
- **Not a module header:** `"./x.h"`, `"../x.h"` and any path whose first element has no dot
  (`"native/x.h"`, `"x.h"`) stay headers of the package folder, whatever `gocpp.mod` says. A header
  in a folder with a dot in its name is written `"./third.party/x.h"`. System headers (`<vector>`) and
  namespaces (`::std::filesystem`) are not affected either.
- **The C++ include** has the absolute name of the file (`#include "/src/game/native/physics.h"`), so
  the compiler finds exactly that file and a header of the same name in the package folder or in another
  module can't take its place (`gocpp emit` shows that absolute name, so the C++ of two machines differs there). The module's folder is also an `-iquote` folder of the compiles and of the
  probes of the package, so the headers of a library can include each other by their names in the
  library with quotes (`"imconfig.h"` next to it, `"backends/x.h"` or `"imgui.h"` from a sub-folder; the package folder is searched before the module's folder, so a file of the package with the same name as a root-relative include of the library is found first). It
  is `-iquote`, not `-I`, on purpose: a file of the module named like a system header (`string.h`,
  `time.h`, `assert.h`) must not hide `<cstring>` or `<string.h>`. The headers that a library wants found
  as `<x.h>` need the `include` folders of its recipe ([C and C++ libraries](#c-and-c-libraries-cxx-recipes), `-I` or `-isystem`). The module-wide
  `cflags`, `ldflags` and `pkg-config` lines of a dependency's `gocpp.mod` are not applied (the allow-list rule for the
  flags of a dependency is an open point); the flags of a library come from its `cxx` recipe, which is explicit.
  `//gocpp:cflags` of the package still works.
- **`extern "C"`.** A module header is decided like a local one: a `.h` header that has no
  `__cplusplus` guard and compiles as C is included inside `extern "C" { }`; `//gocpp:extern-c
  github.com/ocornut/imgui/imgui.h` and `//gocpp:no-extern-c` name it by the import path as written.
- **Errors**, at the import:

  | Import | Message |
  |---|---|
  | no module has the path | `no required module provides header example.net/x/y.h; add the module that has it to gocpp.mod with a require line, check the path for a typo, or for a header in a folder of the package write "./example.net/x/y.h"` (`...: gocpp.mod file not found in DIR or any parent directory` outside every module; a `replace` without `require` says what to add) |
  | file not in the own module | `header example.com/app/native/x.h is not in module example.com/app (/src/app/native/x.h)` |
  | file not in a required module | `module example.org/lib@v1.0.0 found (/src/lib), but does not contain header example.org/lib/x.h` |
  | module not available | `header example.org/lib/x.h: module example.org/lib@v1.0.0 is required, but is not available: gocpp can't download modules yet (replace it with a local folder: ..., or vendor it)` |
  | the path is a folder | `header example.org/lib/x.h (/src/lib/x.h) is a folder, not a file` |
  | a bad path element | `invalid header path "example.org/lib/../../etc/passwd.h": path element ".." is not allowed (a module header is inside its module)`; also an empty element (`a//b.h`), `.`, an element with `:` (a drive letter or a stream name), one that ends in a dot or a space |
  | `\` in the path | `invalid header path "...": use / in header paths` |

  The path is checked as text before any file is looked at, so a header outside its module (`..`, an
  absolute path after the module path) is rejected. gocpp follows symbolic links, and a link in the module can lead out of it
  (`native/leak.h -> /outside/file.h` is accepted and read by the compiler): that is the user's own choice for
  the own, replaced and vendored modules. Links inside downloaded modules are rejected when the archive is
  extracted (step 6.1c). The paths of a `cxx` recipe are stricter: a link that leaves the module is an error
  for every kind of module.
- **Limits.** A folder with its own `gocpp.mod` inside the own or a required module is accepted for
  headers (the module is chosen by the path prefix only; packages reject such a folder). The same file
  named by two spellings (two import paths, or a symbolic link) is included twice (include guards or
  `#pragma once` decide). Windows reserved device names (`con.h`, `aux.h`) are not rejected.
- **Single package for now.** A program is still one package until step 6.2: the headers work from the
  main package, and the loader resolves them from every package.

## C and C++ libraries (`cxx` recipes)

A C or C++ library is a module like any other (a git clone behind a `replace` line, a vendored folder, a
downloaded archive), and its headers are imported by their full path ([Module headers](#module-headers)).
A header declares; something has to **define**. For a library that is more than headers, `gocpp.mod`
holds a small **recipe**: which C and C++ files to compile, where the headers are, which macros to
define, which system libraries to link. No CMake, no shell, no script: a recipe is a list of facts.
([ADR-024](../adr/024-cxx-recipes.md); Phase 06 [§2.11](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#211-c-and-c-libraries-from-the-internet-your-decision-2026-10-13).)

In a program of several packages, any package may import the headers of a library: the program
builds each library **once** (the union over all its packages, also those the main package does not
import), the library's public flags go to the compiles of every package that imports its headers, and
the link flags of all packages are added in the order of the packages, those that import the others
first ([cpp-mapping.md](cpp-mapping.md#files)).

```text
// gocpp.mod of a project that uses Dear ImGui with GLFW and OpenGL
module github.com/huuhait/mygame

require github.com/ocornut/imgui v1.91.5

cxx github.com/ocornut/imgui {
	src      imgui.cpp imgui_draw.cpp imgui_tables.cpp imgui_widgets.cpp
	src      backends/imgui_impl_glfw.cpp backends/imgui_impl_opengl3.cpp
	include  . backends                       // <imgui.h>, "imgui_impl_glfw.h"
	define   IMGUI_DISABLE_OBSOLETE_FUNCTIONS // in the library and in the user's code
	if linux {
		pkg-config glfw3
		link GL
	}
	if windows {
		link glfw3dll opengl32 gdi32 dwmapi // MinGW's import library for glfw3.dll, and system libraries
	}
}
```

```go
import (
	"github.com/ocornut/imgui/imgui.h"
	"github.com/ocornut/imgui/backends/imgui_impl_glfw.h"
)
```

### Where the recipe comes from

1. **The project's own `gocpp.mod`**, a block `cxx <module path> { ... }` named by the module path. It works for
   any module the project requires, replaced, vendored or downloaded, and for the project's own module
   (`cxx github.com/huuhait/mygame { src native/*.c }` builds the C files of a `native/` folder that `.gp`
   code imports as `"github.com/huuhait/mygame/native/x.h"`).
2. **The library's own `gocpp.mod`**, in the module's folder, a block named by **its own** module path. A library author
   can ship the recipe, and then a project needs nothing but `require`.
3. **None**: the headers are found and used as before (`-iquote` the module's folder), and nothing is
   built. A header import of a module with no recipe whose symbols are not defined fails to link; the message
   then says `module X has no cxx recipe, so its C and C++ files are not built`.

The project's block **replaces** the library's one entirely. They are never merged: a project that
wants one more define copies the library's block and adds it. Two blocks for the same module in one file
are an error.

A recipe is read only for a module that a package of the program imports a header (or a package) from.
A broken recipe of a module the program does not use is not looked at. A `cxx` block in the project's
`gocpp.mod` that names a module which is not the project's own, required or replaced there gets a **warning** (not an error:
the file may be shared), since a typo in the module path would otherwise show only as a link error.

### The statements

Each line of the block is one statement: a verb and its arguments, split like the arguments of a `//gocpp:`
directive (white space separates; single, double and back quotes keep white space; a backslash
makes the next character ordinary; `//` starts a comment). The statements of an `if` block apply only to
the targets for which its condition holds ([ADR-129](../adr/129-a-recipe-condition-is-an-if-block-over-build-constraints.md)):

```text
cxx github.com/ocornut/imgui {
	src imgui.cpp imgui_draw.cpp
	if windows {
		link glfw3dll opengl32
	} else if linux || darwin {
		pkg-config glfw3
		if linux && arm64 {
			define GL_ES
		}
	} else {
		define NO_WINDOW_SYSTEM
	}
}
```

- **The condition** is a `//go:build` constraint with the same reader and the same tags as the files of a package:
  the operating system (`linux`, `windows`, `darwin`, ...), the architecture (`amd64`, `arm64`, ...), `unix`,
  `gocpp`, the release tags `go1.N` and the tags of `-tags`; `!`, `&&`, `||` and parentheses
  (`if !windows {`, `if windows && amd64 {`, `if (linux || darwin) && arm64 {`). A tag that is not known is not
  satisfied, as in a `//go:build` line (`if widnows {` is never taken, and is not an error), but the loader
  **warns** about a tag that is no system, no architecture, not `unix`, `gocpp`, `gc`, `gccgo`, `cgo`, `go1.N` and
  not one of the `-tags`, with a guess (`warning: ... the tag "widnows" ... did you mean windows?`). `android` also
  satisfies `linux`, as in Go. The target is the one of the build (`-os`, `-arch`, `-tags`), not the machine's.
- **The shape** is Go's: `if condition {` with the `{` at the end of the same line, the statements on the lines
  after it, and a line that is only `}` to close. `} else {` and `} else if condition {` continue the block, on the
  line of the `}`; the first branch whose condition holds is taken. A line has one statement or one brace line:
  `if windows { link gdi32 }` and `} link m` are errors. Blocks nest to a depth of 4. A comment (`//`) may follow any
  line, and a brace inside a quoted argument or a comment is only text.
- **The statements** inside a block are the verbs below, the same as outside it. A statement outside every block
  applies to every target. The statements that apply to the target are the recipe: the order is the order of the
  file, and the object cache key follows them (a block that is not taken changes nothing, and the same recipe
  for another target has another key).
- **The old form is gone.** A condition in front of a statement (`windows: link gdi32`, `linux,darwin: link dl`,
  `!windows: define X`) was removed by ADR-129; it is an error that shows the block to write
  (`a condition is written as a block, if windows { ... }, since ADR-129`).

| Statement | Meaning |
|---|---|
| `if condition { ... }` | The statements inside apply only to the targets for which the condition holds; `} else {` and `} else if condition {` continue it (above). |
| `src pattern...` | C and C++ files to compile: `.c`, `.cc`, `.cpp`, `.cxx`. A pattern (`*`, `?`, `[a-z]`) matches inside one folder (`backends/*.cpp`; `**` is not supported); it must match at least one source file, and a file named without a pattern must exist. A name that starts with a dot is matched only by a pattern that starts with a dot. |
| `exclude pattern...` | Leave out of the sources the files that match, whatever line listed them. |
| `include folder...` | `-I folder`, for the compiles of the library **and** of every package that uses it (and its probes): `<x.h>`-style includes work. The module's own folder is always an `-iquote` folder (`.` in `include` makes it `-I`). **Hazard:** `-I` puts the folder before the system's, so a file in it named like a standard header (`stdint.h`, `string.h`, `vector`) hides the system's in every compile with the flag; use a more specific folder (`include include`) when the library root has such a file. gocpp adds a hint when a compile with a recipe folder fails and the folder has such a name. |
| `system-include folder...` | The same with `-isystem`: the compiler's warnings in those headers are not shown. |
| `define NAME[=value]...` | `-D`, for the library **and** its users. Use it for a macro that changes a header: a layout, a feature switch. |
| `private-define NAME[=value]...` | `-D` for the compiles of the library only (`BUILDING_THE_LIB`). |
| `cflags flag...` | Flags of every compile of the library (C and C++); not given to its users. |
| `cxxflags flag...` | Flags of the C++ compiles of the library only. |
| `std c++17` | The C++ standard of the library's C++ files: `c++NN`, `gnu++NN`. The default is the `cxx` line of the library's own `gocpp.mod`, else `c++20`. |
| `cstd gnu11` | The C standard of its C files: `c89`, `c99`, `c11`, `c17`, `c18`, `c23` or `gnu` + the same. The default is `gnu17`. |
| `link name...` | `-lname` at the link (`link m`, `link -lpthread`). |
| `ldflags flag...` | Link flags. |
| `pkg-config package...` | The `--cflags` of the packages go to the library **and** its users, the `--libs` to the link. |

A line that gocpp does not understand is an **error at its line**: `gocpp.mod:12: cxx
github.com/x/y: unknown statement "frobnicate": gocpp does not know it, and ignoring it could build
the wrong library`. So are a wrong number of arguments, a bad macro name, a path that is not allowed, an `if`
without a condition, a condition that is not a build constraint, an `else` without an `if` or after an `else`, a
statement on the line of a `}` or of an `if`, a block that is not an `if`, an `if` that is not closed (the file
reader names the `{` that stays open), more than 4 levels of `if`, more than 1024 conditions, a line longer
than 16 KiB, a NUL or another control character, and more than 4096 statements in a recipe. The other lines are
still checked, and all the errors come at once, at their line and column of the file that has the recipe
(`gocpp.mod:12:3: ...`; the column is that of the word at fault), up to 100 of them.

`define` and `private-define` become `-DNAME=value`, and every flag of `cflags`, `cxxflags`, `ldflags`,
`define`, `private-define` and `pkg-config` goes through the flag allow-list of `//gocpp:cflags`
([Phase 04 §2.9](../../plans/2026-10-13-gocpp/phase-04-cpp-interop/README.md)): `-fplugin=`, `-Xclang`, `-wrapper`,
`-B`, `-specs=`, `--sysroot=`, `@file`, `-fuse-ld=`, `-m32`, `-D_GLIBCXX_DEBUG` and the rest are refused with the
same messages (`GOCPP_CFLAGS_ALLOW` and `GOCPP_LDFLAGS_ALLOW` of the **user's** environment can allow
more). As in a directive, a macro value cannot contain `-` or `@`. In a recipe's link flags `-Wl,-rpath`,
`-Wl,-R`, `-Wl,--just-symbols`, `-Wl,-sectcreate`, `-Wl,-syslibroot`, `-Wl,-framework`, `-Wl,--push-state`, `-Wl,-l`,
`-Wl,--library`, `-Wl,-L` and `-lto_library` are refused too, and so is a library named by a path: `-l:libfoo.a`
is fine, `-l:../../x` and `-l../x` (any `/` or `..` in the name) are not, because the linker looks the name up in
the library folders and the system's.

### Paths stay in the module

Every path of a recipe is relative to the module's folder, written with `/`: no absolute path, no `..`, no
`~` or `$`, no drive letter or `:`, no `\`, no element that ends in a dot or a space. A path in a flag of
`cflags`, `cxxflags` or `ldflags` (`-I`, `-isystem`, `-include`, `-L`, a file for the linker) follows the same
rule and is made absolute from the module's folder. The files and folders that a recipe names are looked up with their symbolic
links resolved, **for every kind of module** (replaced, vendored, downloaded), and a link that leads out of
the module is an error: `a symbolic link leaves the module`. Patterns are bounded: 10000 source files
for a library, 100000 entries in one folder, 200000 folder entries read for the patterns of a library. A source is a
regular file of at most 32 MiB (a named pipe, a device or a huge file is an error at its line, not a hang).

Nothing from a module is executed: no script, no `pkg-config` file from the module's folder (`pkg-config`
runs with package names that cannot name a file: no `/`, not ending in `.pc`, and `PKG_CONFIG_PATH` is the
user's). The library is data that the compiler reads; the compiler and the linker are the only programs
that run, and the allow-list is what keeps their flags from running code.

### What the build does

- The **public** flags (`include`, `system-include`, `define`, the cflags of `pkg-config`) go to the library's compiles and to the compiles and probes of every package that
  imports a header of the module, after the package's own `//gocpp:cflags` and before the `-iquote`
  folders. `gocpp check`, `gocpp emit` and the language server see the same flags (they are in
  `ReadPackageFlags`' result, the one source of flags), so `<x.h>` and the macros work in the editor; they build no library.
- Each source is compiled with `gocpp`'s own driver: `-g -O0` (and the sanitizers with `-asan`), `-std=`, `-w`
  (the warnings of a library are not the user's, and a compile that prints something is never kept in the cache),
  `-iquote <module folder>`, the user's trusted `GOCPP_CFLAGS` (and `GOCPP_CXXFLAGS` for C++), the public flags, the
  library's private flags, `-c -x c` or `-x c++`. C files use the C compiler that goes with the C++ compiler (`GOCPP_CC`). The
  flags of the package that uses the library are **not** part of it.
- The objects are kept in the object cache (`<cache>/gocpp/obj`), a content-addressed cache. The key is the text
  of the source, all the flags (the recipe's, the environment's, `-asan`, the folders), the compiler (path, version, file), the
  module path and version, and the headers the compiler read. So a library is compiled once for
  each (module, version, recipe, compiler, flags) and reused by every project that has the same, and a different
  flag, compiler or source text builds again. `gocpp build -x` shows `# cache hit lib.c <hash>` for a reused
  object and `# not cached: <reason>` when a compile cannot be kept.
- The sources compile at the same time, at most `min(GOMAXPROCS, 64)` of them, together with the generated C++ and the
  runtime. The objects are linked in the order of the libraries and their files, after the program's object, and before
  the runtime; the link flags (`link`, `ldflags`, `pkg-config --libs`) come after all the objects. The link is static.
- A library that does not compile is an **error of the build**, exit status 1, not an internal error:
  ```text
  # github.com/x/lib
  the C or C++ library of module github.com/x/lib@v1.2.0 did not build: src/bad.c did not compile (recipe ./gocpp.mod:9: src src/bad.c)
  /home/me/.cache/gocpp/mod/.../src/bad.c:1:34: error: 'UNDEFINED_NAME' undeclared (first use in this function)
  (gocpp build -x shows the command)
  ```
  It shows the first 40 lines of the compiler's output (`-cpperrors=full`: all). A `src` that does not exist or matches
  nothing is an error at its line before anything is compiled.
- `gocpp mod vendor` copies the whole module, so a vendored library builds the same way (with the recipe of the
  project, or the library's own `gocpp.mod` inside the vendored folder).

### Limits (v0)

- A recipe applies to a module whose header (or package) the program imports. A library that needs the
  **headers of another library** has to have that library imported by the program too; there is no
  `needs` statement yet.
- The default build is the debug build (`-g -O0`) of the rest of the program. Optimization levels of a program come with
  Phase 06's build modes.
- Static link only. Shared libraries and `c-shared` builds come later.
- No shared recipe index and no CMake fallback (TODO Q14, open): a library with no recipe of its own needs a `cxx` block in the project.

## For tools

- `load.Include.File` and `Include.Module` are set for a module header; `Include.IncludeText()` is the
  text after `#include`; `load.IsModuleHeaderPath` is the shape rule; `Package.IncludeDirs` are the
  folders of the modules whose headers a package imports (`build.ReadPackageFlags` makes them `-iquote`).
- `load.Config.ModuleDir` (default: `modfetch.Dir(mainDir, path, version)`, internal/load/moddir.go) is
  the one place that gives the folder of a required module that is neither replaced by a local folder nor
  vendored. The commands use `modfetch.New(cfg)` and its `Get`, `Fetch`, `DownloadAll`, `Verify` and `Vendor`.
- `load.FindModule(dir)` is the module of a folder; `Module.File()` is its parsed `gocpp.mod`
  (`gocppmod.File`, with the syntax tree for tools that edit it).
- `load.ListModules(cfg, dir)` lists the required modules with the folder of each, whether it is
  vendored, the `replace` that applies and why a module is not available. It is the data for a
  `gocpp list -m` style command and never fails.
- `gocppmod.Parse` and `gocppmod.Format` read and write the file; `Format` keeps comments, blank
  lines and every statement it does not know.
- `gocppmod.File.Recipes` finds the `cxx path { }` blocks, and `gocppmod.ParseRecipe` reads the lines of one (the
  statements, their lines and conditions, and the errors). `load.Package.CxxModules` lists the modules whose
  headers a package imports, each with its recipe block (`load.CxxModule`); `build.ReadPackageFlags` turns them into
  `PackageFlags.Libs` (`build.CxxLib`, with `Sources()`), the public compile flags and the link flags.
