# ADR-044: The user's compiler flags, LLVM tools, and files for other tools

Status: accepted (2026-10-18; point 5 amended 2026-10-19: a debug build writes `compile_commands.json` by itself; amended 2026-10-22: what `-compdb=path` and `gocpp export compdb -o path` may name), builds on [ADR-036](036-action-graph-and-up-to-date-check.md)

## Context

Steps 6.11 and 6.12. The user wants both ways of using LLVM tools: plugins loaded into the C++ build, and the
program's LLVM IR for `opt`. Phase 04 made the flags of `//gocpp:` lines go through an allow-list that refuses
`-fplugin`. Editors need a compilation database, and people who cannot use gocpp in their build need a CMake file.

## Decision

1. **The user's flags are layered into the trusted flags of the build** (`build.EnvFlags.Layered`): the mode's
   (`-race`, `-release`), then `GOCPP_CFLAGS`/`GOCPP_CXXFLAGS`/`GOCPP_LDFLAGS`, then `-cflags`/`-cxxflags`/`-ldflags`.
   Everything that already read the environment's flags (every compile, the runtime's, the probes) gets them with no
   change in its code. In a compile the trusted flags come before the package's, as `GOCPP_CFLAGS` always did
   (interop spec), and gocpp's language flags last; at the link they come after the package's.
2. **Plugins are only loaded by these flags.** The allow-list stays as it is. A plugin file's content hash is added to
   the object cache key (`PluginSalt`) and to the runtime's key; its size and time are in the up-to-date key.
   Relative plugin paths are made absolute (the compiles run in a work folder). The header probes drop plugin flags
   (`DropPluginFlags`): they run in the editor, count as compiles to a plugin, and have their own cache.
3. **`-emit-llvm` writes IR with a second compiler run** of the object's compile with `-c` turned into `-S -emit-llvm`
   (or `-c -emit-llvm`), after the object compile. Clang only. The runtime has no IR. The output folder
   `<program>.llvm` is emptied only if it carries the marker `.gocpp-llvm`. A second run costs compile time but
   keeps the object cache and the IR independent, and works with a plugin that changes the IR.
4. **The plan.** A build can record every compile and the link it would run (`build.Plan`), from the same functions
   that build the argument lists (not from a copy of them), also under `-n` where nothing runs. The compilation
   database and the CMake export are made from the plan. `export` runs the build as `-n`: the generated files are
   written, no compiler compiles.
5. **`compile_commands.json` is opt-in** (`-compdb`, `gocpp export compdb`), not written by every build: the plan's
   README text said "after each build", but a build that writes into the module folder unasked is a surprise, and a
   file next to `gocpp.mod` is something to commit or ignore by choice. (Open question for the manager.)
   **Amended 2026-10-19 (the user's decision): a debug build writes it by itself.** People debug into the generated
   C++ (ADR-066), and clangd must understand it there, so `gocpp build`, `run`, `test` and `debug` in the default
   debug mode (no flag, `-debug`, `-race`) write it at the end of a successful build, to the places that the bare
   `-compdb` writes, with these rules:
   - A `-release` build writes it only with `-compdb` (a release build is for shipping, not for stepping through).
     `-compdb` and `-compdb=path` work in every mode, for `run`, `test` and `debug` too (they had no `-compdb`).
   - **The off switch is `-compdb=false`**, the form `optFlag` already parsed (`-emit-llvm=false` too): a boolean
     style, as with every Go flag, and no second flag name. Unlike `-emit-llvm=false` it is a value of its own, not
     "not given", so it also wins over the debug default.
   - The copy in the module's folder is written by `gocpp build` for the host only. `run` and `test` build one program
     at a time into folders that they remove or that change with each package (a test run of `./...` would leave the
     file of the last package), and a cross-compile's commands (MinGW, aarch64) are not what clangd needs for the
     host's sources, so it must not replace the host's file. `run`, `test`, `debug` and cross builds write the copy in
     the folder of the generated C++ only, and only when that folder is kept: it is, in the cache's `gen/`, but not
     without a cache, where it is in the work folder that `run` removes (`-work` keeps it).
   - `-n` writes nothing by itself (it writes no program either); `gocpp export` writes only what it was asked for;
     `gocpp check` does not build.
   - A file that cannot be written (a read-only module folder, a full disk) is a warning on standard error (at most
     once a day for a file: a read-only module folder would repeat it for every build) and the build succeeds: the
     file was not asked for. With an explicit `-compdb` it stays an error.
   - **The automatic write only replaces what gocpp wrote** (review of 2026-10-19). The cache keeps the hash of the
     last content written to each path (`cache/compdb/`). A file that is there with other bytes (a hand-made file, the
     database of CMake or Bear, a file somebody edited) is left alone with one line on standard error that says so
     and names `-compdb`; a symbolic link (the usual setup that links the file into a build folder) is never
     replaced. An explicit `-compdb` (and `gocpp export compdb`, with or without `-o`) replaces a regular file, and
     replaces a symbolic link by a regular file: **no write ever goes through a link** (a cloned repository can ship
     `compile_commands.json -> ~/.bashrc`, or a dangling link that would create a file anywhere; the file behind a link
     is untouched, nothing is created at a dangling target). To fill the file behind a link, name it:
     `gocpp export compdb -o path`. Both record the hash, so a later automatic write may replace that file. The copy in
     the folder of the generated files is gocpp's own and is written without this check. `-compdb=false` leaves a
     file that is there as it is. The record is keyed by the path with the links of its folder resolved, so a build
     from a linked folder finds it. Limits: a cleared cache loses the records, and a differing file then looks foreign
     and gets the note until `-compdb` writes it again; the replace and the record are two steps, not atomic across
     builds of different programs that run at the same time (the worst case is a note, or one more write); the records
     are small and are never trimmed.
   - **What a name may be** (amended 2026-10-22, job 13.hangs). The name of `-compdb=path` and `-o path` is looked at
     with `os.Lstat`, like Go's `-o`: a folder, or a name that ends with a slash, gets `compile_commands.json` (a
     folder that does not exist is made, as `go build -o bin/` makes `bin`); a name that does not exist and has no
     slash is the file; a regular file is replaced. A **named pipe, a socket or a device is refused** with a message
     (it hung for ever on the open and ignored SIGTERM, and a device would be replaced by the rename); so is a
     trailing slash on a regular file. A **symbolic link to a folder is refused** unless the name ends with a slash
     (`-o link/`: the person said it is a folder; `os.Stat` used to resolve it silently, and a repository can ship
     such a link): the link is neither written through nor replaced by a file without a word. A link to a file, or a
     dangling one, stays as decided above (replaced by the file). The automatic write is given file names only and
     never resolves a name: a folder or a link with the name `compile_commands.json` is foreign and left alone.
   - **Nothing is written when the generated files are not kept**: without a usable cache folder the build is in a
     work folder that is removed, and the commands name files in it, so neither copy is written (a good file stays).
     An explicit `-compdb` says so in a warning and writes nothing.
   - **The module's copy is for a program in a module** (a folder with `gocpp.mod`), not for `gocpp build main.gp`
     of a loose file, whose folder may be anywhere. With several programs in one module it holds the commands of the
     one built last (the copy in the generated folder is per program).
   - The build stays *stamped* (the up-to-date check of ADR-036 still ends a build that is current). Only the copy in
     the generated folder is an output of the check (a deleted one is written again); the module's copy is no change of
     the package (so it is not in the listing) and not an output, because with several programs it changes with every
     build and the builds would never be current. An up-to-date build does not write the files again: delete the
     generated folder's copy, or run `gocpp build -a` or `gocpp export compdb`. `-compdb=false` is in the key of the
     check, so the build that wrote nothing is not taken for the one that writes. The file is written only when its
     content changes, by a rename, and its content is the same for the same build, so the program, the object cache
     and the generated files are the same as without it.
   - The repository's `.gitignore` ignores `compile_commands.json`; the test harness builds with `-compdb=false`
     in its source folders.
6. **`gocpp export cmake`** copies the generated files and the runtime into the folder and refers to the project's own
   files by absolute path; it checks the compiler family at configure time. Limits are in docs/spec/build.md.
7. **`gocpp list` and `gocpp env`** follow `go list` and `go env`. `env` has a table of variables that a test keeps
   in step with the Go code.

## Consequences

- The cache is correct across a rebuilt plugin; a plugin that depends on files other than itself is not tracked.
- `-race`, `-release`, `-debug` are flags of build and run (the plan listed them); `-buildmode` is step 6.10's.
- `-n` shows the plain compile commands (no precompiled header), which is what a tool wants to run.
