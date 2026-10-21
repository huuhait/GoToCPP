# Using LLVM tools with gocpp

gocpp compiles to C++, so the tools of the C++ world work on your program. There are two ways to use LLVM's:
load a plugin into the compiler that builds your program, or take the LLVM IR of your program out and run
`opt` on it. Both are your choice, made with your own flags: a package can never ask for them.

## Load a plugin into the build

Pass the plugin in the trusted flags (see [spec/build.md](../spec/build.md)): `-cxxflags` of `gocpp build`, or
`GOCPP_CXXFLAGS` in the environment. They reach every C++ compile of the program, gocpp's runtime too.

```
# clang: an LLVM pass plugin (new pass manager)
GOCPP_CXX=clang++ gocpp build -cxxflags=-fpass-plugin=$PWD/MyPass.so .

# g++ or clang: a front-end plugin, with an argument for it
gocpp build -cxxflags="-fplugin=$PWD/count.so -fplugin-arg-count-verbose=1" .

# the same through the environment
GOCPP_CXXFLAGS=-fpass-plugin=$PWD/MyPass.so gocpp build .
```

A plugin with a folder in its path may be relative to the folder you run gocpp in; gocpp makes it absolute. The plugin
must be built with the same compiler (and LLVM or GCC version) that gocpp uses: `gocpp env GOCPPCXX GOCPPCXXVERSION`
tells which. The file is part of the cache keys by its content, so rebuilding the plugin compiles the program again, and
`gocpp build` does not say "up to date" over a new plugin.

Your plugin sees every translation unit: the generated C++, your `.c` and `.cpp` files, and the runtime. A pass
that wants only your code can look at the file name (the generated files are in `gen/<hash>/gpkg/`, and `#line`
directives point back to the `.gp` lines). Plugins do not run in the probes that read headers for `gocpp check` and
the editor.

A `//gocpp:cflags -fplugin=...` line in a package is an error (the allow-list refuses it); `GOCPP_CFLAGS_ALLOW` can
allow it, which is you trusting that package with your compiler.

## Take the IR out

```
GOCPP_CXX=clang++ gocpp build -emit-llvm .        # text: myprog.llvm/**/*.ll
GOCPP_CXX=clang++ gocpp build -emit-llvm=bc .     # bitcode: *.bc
opt -load-pass-plugin=$PWD/MyPass.so -passes=my-pass -S myprog.llvm/gen/gpkg/example.com/m/main.gp.ll -o -
```

`-emit-llvm` needs clang: with g++ gocpp stops with an error that says so. The IR is made with the same flags as the
object (so `-O2`, `-cxxflags`, and `-fpass-plugin` are in it), as `-S -emit-llvm`. The folder `<program>.llvm` is
emptied by the next build with the flag; gocpp only empties a folder that it made (it has a `.gocpp-llvm` file).

## Look at the generated C++ in an editor

A debug build (the default) writes `compile_commands.json` into the module folder by itself, so clangd understands the
generated C++ and your own C++ files (and a copy into the folder of the generated C++, which `gocpp run` and
`gocpp test` write too). It never replaces a `compile_commands.json` that gocpp did not write (a hand-made one, CMake's or Bear's, a symbolic link): it says so, and `-compdb` overwrites (a symbolic link is replaced by a file; write the file behind it with `gocpp export compdb -o path`). `-compdb=false` turns the automatic write off and leaves an old file in place. A `-release` build writes it with `gocpp build -release -compdb`; `-compdb=false` turns the
automatic write off; `gocpp export compdb` writes it without building. `gocpp export cmake` makes a CMake project
of the program.

What clangd shows, and what it needs:

- Open the generated files from the folder that `gocpp build -x` prints after `# gocpp emit >` (the folder of
  the generated C++ in gocpp's cache, with its own copy of `compile_commands.json`): clangd finds the file's
  command there. The module's `.c` and `.cpp` files find theirs in the module's file. A `pkg.hpp` has no entry
  of its own; clangd takes the command of a neighbor, as for any header.
- The file names the compiler by its full path. With clang++ clangd needs nothing more. With g++, or a clangd
  that cannot find the standard headers (a clangd of a Nix shell that is not the wrapped one in gocpp's dev shell),
  start clangd with `--query-driver=<path of the compiler>` (`gocpp export compdb -h` says the same).
- The generated code has `#line` directives that point at your `.gp` lines (debuggers use them). clangd ignores
  them for its messages: an error that it shows is at a line of the generated C++.
- `clangd --check=FILE` opens one file in batch mode. It counts every refusal of its refactorings
  (`tweak: ExtractFunction ==> FAIL ...`) as an error and exits with 3 for them, so for gocpp's own test
  (`tests/clangd_test.go`, `TestClangdOpensGeneratedCode`) a file opens cleanly when the only errors are those
  refusals. That test runs the check over the generated code of thirteen programs with g++ and with clang++.
