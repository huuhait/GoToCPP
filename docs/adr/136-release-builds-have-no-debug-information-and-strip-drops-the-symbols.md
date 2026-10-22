# ADR-136: `-release` builds have no debug information, and `-strip` drops the symbol table

Status: accepted (2026-10-21; job 15.size).

## Context

The Windows ImGui demos built by `gocpp build -os windows` were 63 MB. A Windows hello world was 13 MB, a Linux one
5.6 MB. The cause is the default build, `-O0 -g` ([internal/build/flags.go](../../internal/build/flags.go)): debug
information is most of the file, and a MinGW `.exe` also carries the COFF symbol table (4.6 MB for a hello world,
with long mangled names in the string table). `-release` (`-O2 -flto`) already existed but kept `-g`: the Linux
hello world was still 5.1 MB, and the Windows one 4.9 MB.

Measured on the Windows hello world (13.3 MB): 6.7 MB of `.debug_*` sections, 4.6 MB of symbols, 2.0 MB of
program. The link already used `--gc-sections` with `-ffunction-sections -fdata-sections`, Windows links `-static`
(no DLLs), and Linux links libstdc++ dynamically; none of that was the problem.

## Decision

- The default stays a debug build. Debugging is a feature of gocpp (`gocpp debug`, `break hello.gp:12`, the printers
  of gocpp-gdb.py), and a panic's trace reads Go function names and `file.gp:line` from the symbol table and the
  DWARF line table of the program's own file. A person who runs `gocpp build` and debugs the result must find
  the debug information there. The sizes are documented where users look: `docs/spec/build.md` ("Program size"),
  `docs/guides/windows.md` ("Small programs"), `docs/guides/windows-cross.md` and the READMEs of the demos.
- `-release` is `-O2 -flto -g0`: the compiles and the link both get `-g0`. The link needs it because with
  `-flto` the compiler writes the debug information at the link, from the `-g` that every link line has
  (measured: `-cflags=-g0` alone left 237 KB of `.debug_info` in the Windows hello world, and made a Windows
  `-g1` build identical to `-g0`). The symbol table stays, so a panic prints Go function names, without file and
  line (`?:0`; the trace of an inlined call in an optimized build was already only approximate). This changes
  what `-release` produces, not its code.
- `-strip` is new: the link's `-s` (macOS: `-Wl,-S -Wl,-x`, since ld64 has no `-s`). It drops the symbol table and
  every `.debug_*` section, also the gdb printers (84 KB). A panic then prints `?()` and addresses, as the runtime
  already did for a stripped program (`runtime/README.md`). `-strip` with `-debug`, `-race` or `-asan` is an error:
  their tools need the symbols. `-strip` works with the default build too (585 KB for the Linux hello world).
- The small build is `-release -strip`: Linux hello 224 KB, Windows hello 1.4 MB, `demos/imgui` 2.5 MB,
  `demos/imgui-dx11` 3.3 MB. `-release -cflags=-g -ldflags=-g` is the documented way to keep the debug information of an
  optimized build (later flags win).
- `gocpp debug` has the build flags. `-strip` is an error there (exit 2). `-release` prints one line on standard error
  ("-release builds have no debug information") and goes on, unless the user's flags put the debug information back at
  the compiles and at the link: a `-g` flag (not `-g0`; the last `-g` flag wins) in `-cflags`/`-cxxflags` or `GOCPP_CFLAGS`/
  `GOCPP_CXXFLAGS`, and one in `-ldflags` or `GOCPP_LDFLAGS`. Both are needed because `-release` puts `-g0` on both
  and the link writes the debug information under `-flto`. `-release` used to keep `-g`, and a person may still want an
  optimized program under gdb.
- Both flags are in the key of the up-to-date check (`extraFlags.keyParts`). `-g0` reaches the runtime library
  through the layered flags, whose cache key already follows them.

## Rejected

- **Stripping by default for `-os windows`.** The default build is the build that `gocpp debug` and `gocpp run` use; a
  different default per target would make the Windows build undebuggable for the people who need it most (no Windows
  debugger runs without symbols), and surprise those who copy the same command from the Linux guide.
- **Stripping in `-release`.** A shipped program that panics would print only addresses, and Go's own release
  builds keep their function names in the panic trace. The cost of keeping them is the symbol table: 2.0 MB on
  Windows (3.4 MB against 1.4 MB for a hello world), 150 KB on Linux. Users who want the smallest file add `-strip`.
- **`-gline-tables-only` (`-g1`) for `-release`.** It gives `file.gp:line` in traces, but the trace of optimized,
  inlined code points at the runtime's lines (the panic of an inlined `xs[i]` showed `trace.cpp:47`), and it adds
  1.1 MB to the Linux hello world (1.5 MB against 375 KB) and 0.4 MB to the Windows one (3.9 MB against 3.4 MB).
- **`-Os`.** 154 KB against 224 KB for the Linux hello world, but g++ 15.3 for MinGW fails with an internal compiler
  error in `-Os -flto` (Windows hello and the dx11 demo). It stays a user choice (`-cflags=-Os -ldflags=-Os`).
- **A smaller runtime.** The code of the stripped Windows hello world is 1.0 MB, and libstdc++'s locale facets
  (`time_get`, `money_get`, from the one `std::ostringstream` of `runtime/src/fmt.cpp`) are a part of it. Replacing
  that stream would shrink every Windows program; it is a runtime change of its own and not part of this ADR.

## Consequences

- `-release` programs are not debuggable by symbol and line any more (gdb still has function names from the symbol
  table). The documentation says what to add.
- A panic trace in a `-strip` program shows addresses. `addr2line -e <unstripped build> <pc>` names them when the
  unstripped build has the same flags and source.
- `.debug_gdb_scripts` (84 KB) stays in `-release` builds: it is in the runtime library of every program and not
  debug information in the compiler's sense; `-strip` removes it.
