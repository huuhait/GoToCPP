# Calling gocpp from C and C++

`gocpp build -buildmode=...` makes a library of your gocpp code. Three modes, for three kinds of consumers
(the rules are in [docs/spec/build.md](../spec/build.md#libraries--buildmode)).

## From C: c-archive and c-shared

`greet.gp`:

```go
package main

//export Add
func Add(a, b int32) int32 { return a + b }

//export Greet
func Greet(name string) string { return "hello, " + name }

//export Sum
func Sum(xs []int64) (int64, bool) {
	var s int64
	for _, x := range xs {
		s += x
	}
	return s, len(xs) > 0
}

func main() {} // required, as in cgo; it never runs
```

```sh
gocpp build -buildmode=c-archive -o libgreet.a .     # libgreet.a + libgreet.h
gocpp build -buildmode=c-shared  -o libgreet.so .    # libgreet.so + libgreet.h
```

`consumer/main.c` (in a folder of its own: a C file next to `greet.gp` is part of the package):

```c
#include <stdio.h>
#include <stdlib.h>
#include "libgreet.h"

int main(void) {
    printf("%d\n", Add(2, 3));
    GoString name = {"bob", 3};
    GoString r = Greet(name);
    printf("%.*s\n", (int)r.n, r.p);
    free((void *)r.p);                      /* results are malloc copies */
    long long xs[3] = {1, 2, 3};
    struct Sum_return s = Sum((GoSlice){xs, 3, 3});
    printf("%lld %d\n", s.r0, s.r1);
    return 0;
}
```

```sh
cc -I. consumer/main.c libgreet.a -lstdc++ -pthread -lm -ldl       # c-archive
cc -I. consumer/main.c -L. -lgreet -Wl,-rpath,.                    # c-shared
```

An archive built with `-asan` needs the sanitizer at the consumer's link too: add `-fsanitize=address,undefined`.

Things to know:

- Only types that C can hold cross the boundary (`int`s, floats, `bool`, pointers, `string`, slices of numbers). The build
  says what is wrong at the `.gp` line. Strings and slices are **copied**: arguments for the call, results into `malloc`
  memory that you free.
- The runtime starts at the first call, from any thread; threads that C creates become goroutines on their own. Package
  variables and `init` run once, before the first exported function body.
- The library takes the process's environment at that first call (of an exported function, or of
  `gocpp_<name>::init()` in a cpp-lib): `os.Environ`, `os.Getenv` and the `$GODEBUG` settings start from what it is
  then. Go's c-archive reads it when the program is loaded, so a host that calls `setenv` before its first call sees
  its value in a gocpp library and not in Go's.
- An exported function may be called from a C `__attribute__((constructor))` or a C++ static initializer of the host,
  which run before the archive's own static initialization: the call works as from `main` (the type descriptors and
  method tables are constants or made at their first use, and so is the runtime's state; a package variable of a C++
  class is built by the package initialization, ADR-093). Go's c-archive hangs in such a call.
- A panic that leaves an exported function prints Go's message and ends the process with status 2.
- When the host's `main` returns or it calls `exit()`, the library's goroutines that still run go on until
  the process ends, as with Go's libraries: no package variable is destroyed under them.
- A shared library hides everything but the exports; two gocpp libraries can be loaded with `dlopen` into one
  process (each has its own runtime and globals) and unloaded with `dlclose`. A library that has a goroutine
  that has not ended (running or blocked) stays loaded: `dlclose` returns 0 and does nothing, as Go's c-shared
  libraries are never unloaded. When the goroutines have ended, the next call of an exported function lets go, and
  `dlclose` unloads. Ending the process does not wait for goroutines. The reference is only given back by a
  later call of an exported function: if the last call returns while a goroutine of an earlier call is still
  finishing, the library stays mapped after `dlclose` until the process ends (safe, but a leak).
- A library that C calls can call C: `import "./helper.h"` works as in a program, and the C files next to the `.gp` file
  are in the archive.

## From C++: cpp-lib

```sh
gocpp build -buildmode=cpp-lib -o dist ./geo ./util
```

`dist/` holds `include/` (the package headers, gocpp's runtime headers and `geo.hpp`...), `lib/libX.a` and the CMake and
pkg-config files. In a CMake project:

```cmake
cmake_minimum_required(VERSION 3.20)
project(app CXX)
find_package(util CONFIG REQUIRED)         # -DCMAKE_PREFIX_PATH=/path/to/dist
add_executable(app main.cpp)
target_link_libraries(app PRIVATE gocpp::util)
```

```cpp
#include <util.hpp>

int main() {
    gocpp_util::init();                    // runs package initialization once
    auto p = geo::Origin().Add(geo::Point{1, 2});
    auto s = util::Shout(gp::string("hi"));
}
```

The API is the native one: `gp::string`, `gp::slice<T>`, structs with their methods. Use the same compiler family as the
library (g++ with g++, clang++ with clang++). Without CMake, `pkg-config --cflags --libs util` (with
`PKG_CONFIG_PATH=dist/lib/pkgconfig`) gives the flags.
