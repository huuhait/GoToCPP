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

`gocpp build -compdb` writes `compile_commands.json` into the module folder, so clangd understands the generated C++
and your own C++ files; `gocpp export compdb` writes it without building. `gocpp export cmake` makes a CMake project
of the program.
