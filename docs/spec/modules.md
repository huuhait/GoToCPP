# Modules and `gocpp.mod`

A gocpp project is a **module**: a folder tree with a `gocpp.mod` file at its top. The file
names the module, says which other modules it needs and where they are on disk, and holds the
settings for the whole module (the C++ standard, compiler flags, `pkg-config` libraries). This
document says what the file looks like, how gocpp finds the module of a folder, and how an import
path becomes a folder.

| | |
|---|---|
| Status | v0, written with [Phase 06 step 6.1](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#3-steps): the file, the module root, `require`, `replace` and `vendor/` with **local folders only**. Building more than one package is steps 6.2 to 6.7 |
| Source | [ADR-020](../adr/020-modules-gocpp-mod.md), [Phase 06 §2.1](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#21-the-module-file-gocppmod), [Phase 01 §2.6](../../plans/2026-10-13-gocpp/phase-01-frontend/README.md#26-package-loading-and-the-importer) |
| Code | `internal/load` (`modules.go`, `resolve.go`) and `internal/load/gocppmod` (the file format) |
| See also | [language.md](language.md) ([D11](language.md#d11-gocpp-directives-instead-of-cgo-lines)), [interop.md](interop.md) |

**Not here yet:** downloading modules (`gocpp get`, a module cache, `gocpp.sum`, version selection;
Phase 15), and C and C++ libraries as modules with a build recipe
([Phase 06 §2.11](../../plans/2026-10-13-gocpp/phase-06-packages-build/README.md#211-c-and-c-libraries-from-the-internet-your-decision-2026-10-13)).
A module that is required but is neither replaced by a local folder nor vendored is an error that
says so. Nothing in this step touches the network.

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
inside opens a block of its own, and a line that is only `}` or `)` closes it, so
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
newer gocpp versions. A block that is not closed before the end of the file is an error at the last line,
as in Go, and its lines are not read as statements.

**Errors** read like the go command's: `gocpp.mod:3: usage: require module/path v1.2.3`, with the
column after the line when the statement is inside a block (`gocpp.mod:5:2: ...`). All the
statements that are wrong are reported at once, and the valid ones are still used: a mistake in a
`require` line does not hide the module path. The messages include `usage: ...` for a wrong number of
arguments, `repeated module statement`, `invalid module path "x/../y"`, `invalid module version
"v1": must be of the form v1.2.3`, `replacement module without version must be directory path
(rooted or starting with . or ..)`, `multiple replacements for x (the first is at line 3)`,
`mod files must use // comments (not /* */ comments)` and `syntax error (unterminated block ...)`.

`require` of one module twice is allowed with the same version, and an error with two versions:
gocpp has no version selection yet, so keep one.

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
Only its `require`, `replace` and `vendor/` count, for every package of the program, also for the
packages of required modules: there is no version selection yet, and, as in Go, the `replace` lines
of other modules are ignored. The `gocpp.mod` of a required module is read for its module line and
its errors, and for the settings it will give to its own packages; its `require` lines are not
followed. So a program must `require` every module that it imports, directly or not (mark the
indirect ones `// indirect`).

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
version (`replace a => b v1.0.1`) is read, but cannot be used until the module downloads exist: gocpp can not
download it yet. The error says so.

A package of a replaced module has the replaced module as its module, with the version of the
`require` line.

### `vendor/`

If the main module has the file `vendor/modules.txt`, it is **vendored**: every required module is
taken from `vendor/<module path>/` (its packages, without the module's `gocpp.mod`), also a replaced
one, and nothing is read from the replacement folders, so a vendored project builds without the
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
check runs when a package first imports a required module. The `gocpp mod vendor` command that writes
the folder comes with the module downloads.

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
  as `<x.h>` need the `include` folders of its recipe (step 6.1d, which can use `-I` or `-isystem`). The flags of the module's own
  `gocpp.mod` (`cflags`, `pkg-config`, the `include` folders of a `cxx` recipe) are not applied yet:
  recipes come with step 6.1d, and the allow-list rule for the flags of a dependency is an open point.
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
  extracted (step 6.1c).
- **Limits.** A folder with its own `gocpp.mod` inside the own or a required module is accepted for
  headers (the module is chosen by the path prefix only; packages reject such a folder). The same file
  named by two spellings (two import paths, or a symbolic link) is included twice (include guards or
  `#pragma once` decide). Windows reserved device names (`con.h`, `aux.h`) are not rejected.
- **Single package for now.** A program is still one package until step 6.2: the headers work from the
  main package, and the loader resolves them from every package.

## For tools

- `load.Include.File` and `Include.Module` are set for a module header; `Include.IncludeText()` is the
  text after `#include`; `load.IsModuleHeaderPath` is the shape rule; `Package.IncludeDirs` are the
  folders of the modules whose headers a package imports (`build.ReadPackageFlags` makes them `-iquote`).
- `load.Config.ModuleDir` (default: `moduleDir`, internal/load/moddir.go) is the one place that gives the
  folder of a required module that is neither replaced by a local folder nor vendored; the module
  downloads (step 6.1c) provide it.
- `load.FindModule(dir)` is the module of a folder; `Module.File()` is its parsed `gocpp.mod`
  (`gocppmod.File`, with the syntax tree for tools that edit it).
- `load.ListModules(cfg, dir)` lists the required modules with the folder of each, whether it is
  vendored, the `replace` that applies and why a module is not available. It is the data for a
  `gocpp list -m` style command and never fails.
- `gocppmod.Parse` and `gocppmod.Format` read and write the file; `Format` keeps comments, blank
  lines and every statement it does not know.
