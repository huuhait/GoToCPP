# What generics cost to compile

Generic code compiles like the same code written out by hand, because that is what it is: gocpp lowers
each **instance** (a generic function or type with its list of type arguments) to ordinary C++ written with
those types, not to a C++ template ([ADR-030](../adr/030-generic-instances.md),
[cpp-mapping.md](../spec/cpp-mapping.md#generics-steps-71-to-73)). So the C++ compiler does no template
deduction or instantiation for your generics, and the cost has three parts:

1. gocpp's own work (check and lower: milliseconds for a program of a few thousand lines);
2. the C++ compiler on the text of each instance, which is the cost of that function written by hand;
3. for generic **types**, the text of every instance (struct, methods, descriptor) is in `pkg.hpp`, so each
   translation unit of the package parses it again, and a hand-written program has it in one unit only.

This page has the measurements of step 7.12 and the cheap ways to bring the numbers down. Nothing here needs
a change to gocpp.

## How it was measured

`tools/genericsgen/ctime_gen.py` writes three kinds of programs (`ctime.py` builds and times them). **F**: N generic
functions with a loop, a closure, a map and a slice each, called with K of 10 types (N x K instances). **T**: N
generic types with three methods each, K instances each, every instance used and printed with `%T`. **Nest**: a
chain of 8 to 12 generic types that each hold the next by value, instantiated with types nested 1 to 8 deep. Each
generic program has a **hand** twin (the same code with one concrete function or type per instance), which is
what a C++ programmer would write.

The numbers are CPU seconds (user + system, the minimum of 3 runs) of one `gocpp build` with the object cache
off and the runtime already built, so they include gocpp, every C++ compile and the link. The floor, a program
that prints one line, is 0.7 s with g++ and 0.5 s with clang++ at `-O0`; 1.4 s at `-O2`; 5.0 s and 5.7 s with
`-release`. g++ 15.3 and clang++ 21.1, on a shared 192-core machine at load 100 to 200, so expect +/- 15%.

## The numbers

`-O0 -g` (the default), CPU seconds:

| Program | Instances | g++ generic | g++ hand | clang++ generic | clang++ hand |
|---|---|---|---|---|---|
| 20 functions x 10 types | 200 | 2.9 | 2.8 | 1.4 | 1.7 |
| 100 functions x 5 types | 500 | 4.2 | 4.5 | 2.2 | 2.1 |
| 400 functions x 5 types | 2000 | 17.4 | 17.5 | 8.4 | 8.3 |
| 10 types x 5 (3 methods) | 50 | 4.9 | 5.7 | 3.0 | 3.0 |
| 20 types x 10 (3 methods) | 200 | 28.2 | 18.2 | 14.8 | 10.4 |
| chain of 8 types, 8 depths | 104 | 8.9 | | 4.6 | |
| chain of 12 types, 8 depths | 152 | 12.1 | | 7.5 | |

`-O2` and `-release` (`-O2 -flto`), CPU seconds:

| Program | g++ -O2 | hand | g++ release | hand | clang++ -O2 | hand | clang++ release | hand |
|---|---|---|---|---|---|---|---|---|
| 20 functions x 10 types | 6.4 | 6.2 | 9.2 | 8.9 | 8.5 | 8.2 | 22.6 | 22.6 |
| 100 functions x 5 types | 8.2 | 8.4 | 10.2 | 10.3 | 22.5 | 19.6 | 58.3 | 53.9 |
| 20 types x 10 (3 methods) | 79.2 | 47.6 | 56.2 | 47.5 | 70.1 | 37.0 | 58.0 | 40.3 |
| chain of 8 types, 8 depths | 18.2 | | 17.8 | | 23.0 | | 15.2 | |

What the tables say:

- **Generic functions cost what hand-written ones cost**, 4 to 11 ms of CPU for each small instance at `-O0`
  (200, 500 and 2000 instances are on three lines): the time grows in a straight line with the instances, and
  the generic and the hand-written programs are within the noise of each other, at every optimization level and
  with both compilers. There is no template instantiation to pay for.
- **Generic types cost 30 to 55% more than the same types written out** at `-O0` (the T rows: +55% and +42% in
  the first run above, +33% with g++ and +39% with clang++ in an independent re-measurement of the 20 x 10
  program, 24.1 s against 18.2 s and 14.4 s against 10.4 s; the loaded machine is the noise), and the
  difference is the header: the text of every instance is in `pkg.hpp` (about 5 KB of C++ for a type with three
  methods: 986 KB for the 200 instances, against 92 KB of structs and 419 KB of method bodies in the one unit
  of the hand-written program), and every translation unit of the package includes it: the unit of each `.gp`
  file, `pkg_desc.cpp` and `pkg_init.cpp`. Parsing that header alone takes 9.7 s of CPU with g++ and 3.8 s with
  clang++ (measured with an empty unit that includes only `pkg.hpp`), so `pkg_desc.cpp` and `pkg_init.cpp`, which
  hold almost nothing else, cost 7.4 + 6.8 s and 3.7 + 3.8 s. The header is paid once for each unit, so a package
  of many `.gp` files pays it many times.
- **Nesting costs nothing by itself**: a type nested 8 deep is one more instance with a longer name, and the
  chain of embedded types is a few seconds.
- `-release` is dominated by LTO: its floor is 5 s, and clang++'s ThinLTO of 100 functions x 5 types is 58 s,
  against 10 s with g++.

## A bug that the nesting numbers found

The first run of the chain program took 75 s of CPU, almost all in gocpp's own check of the layout of the structs
(`gocppSizes.structLayout`): each level asked for the layout of the field below it three times, so a chain of
structs that hold each other by value cost 3^depth. Depth 12 took minutes, depth 15 hours. It is fixed (the layout
of a struct is computed in one pass; `TestSizesNestedStructs` has a chain 40 deep), and the chain row above is
the result: `gocpp emit` of the 8 x 8 chain went from 19 s to 0.1 s.

## Cheap ways to compile less

None of these needs work in gocpp.

1. **Keep the object cache on** (the default). A change to one generic function recompiles the units that
   instantiate it and nothing else; `gocpp build` of an unchanged program only links. The measurements above
   turn it off (`GOCPP_NOOBJCACHE=1`) to show the cost of a cold build.
2. **Put the uses of a generic type in few files.** Each `.gp` file of a package is a translation unit that
   includes `pkg.hpp`, with the text of every generic type instance the package names. A package of 40 small
   files pays the header 40 times; the same code in 8 files pays it 8 times. Generic *functions* do not have
   this cost: an instance is written in the unit that asks for it (and twice, in two units that both ask).
3. **Do not print a deep instance in many places.** `%T`, `%v` and `fmt.Println` of a type need its full
   descriptor, which names the descriptors of all its parts; one `Println` of each nested type in a loop of
   generic helpers is cheaper than one in every helper.
4. **Debug builds are the cheap ones.** `-O2` costs 2 to 3 times the CPU of `-O0 -g` with g++ and 5 to 10 times
   with clang++ (which inlines the runtime's templates harder), and `-release` (`-O2 -flto`) 2 to 3 times and 16
   to 26 times; develop with `gocpp run` and build with `-release` when you ship.
5. **The runtime header is precompiled already** (a PCH of `rt.hpp` for every flag set, built once into the user
   cache), which is why the floor is 0.5 s. The same idea would help `pkg.hpp`: a precompiled `pkg.hpp` for
   the packages with many generic type instances would remove the header cost of the T rows (the two units that
   hold only the header are 14 s of the 41 s of CPU of the 20 x 10 program with g++, 7.5 s of 19 s with clang++).
   It needs work in the build driver (a second PCH, chained on the runtime's), which is why it is only listed here.
6. **`extern template` does not apply**: the instances are ordinary functions and structs, not templates.
   Their equivalent would be to write each instance of a generic type once, in one `.cpp`, and only declare
   it in `pkg.hpp` (the struct has to stay in the header because users hold it by value), so that the method
   bodies are compiled once and not parsed by every unit. Today the bodies are in the header under
   macro guards (`_POST`) and are `inline`. That is a change to `internal/lower/insttypes.go`, not a switch.

7. **An external compiler cache is optional.** The existing compiler command accepts
   `CXX='ccache g++'` or `GOCPP_CXX='sccache clang++'`; there is no extra `GOCPP_CCACHE` switch.
   Keep gocpp's own cache on for normal builds. When comparing external cold/warm builds, bypass
   the internal object cache and up-to-date check and inspect the external tool's hit/miss counters:
   a shorter second build can simply be runtime/PCH reuse. Changing work folders and debug paths
   can prevent external hits, and PCH support is tool-specific. The [build specification](../spec/build.md#compiler-launchers-and-external-caches)
   gives the configuration, measurement recipe and compiler-identity limits. No external-cache
   speedup is included in the tables above.

## The concrete next step (not done)

The independent review of step 7.12 measured where the header time goes in the 20 types x 10 program: `pkg.hpp` is
978 KB, 42% of it the method bodies of the instances (the `_POST` blocks), 31% repeated long `#line` paths, and
`pkg_desc.cpp` and `pkg_init.cpp` each parse the whole header: 14 of the 41 CPU seconds with g++, 7.5 of 19 with
clang++. Two changes in `internal/lower` would bring the T rows from +33 to 39% to an expected +5 to 10%:

1. **Emit the method bodies of an instance once, as ordinary (non-`inline`) definitions in the existing
   `pkg_desc.cpp` unit, and only declare them in `pkg.hpp`.** Bodies of at most three statements stay inline in the
   header, so that `-O2` without LTO still inlines the small accessors.
2. **Drop a `#line N` directive that repeats the one before it** (a few percent of the header's size and parse time).

## What to expect for a big program

A program with 2000 generic function instances builds in 17 s (g++) or 8 s (clang++) of CPU at `-O0`, in one
translation unit when it is one file: the wall time is about the same, because one unit is one process. Split
into several files the units compile in parallel. A program with 200 instances of generic types is 28 s (g++) or
15 s (clang++) of CPU, in 3 units that run at the same time. The `slices`, `maps` and `cmp` functions are
instances like yours, in the unit that asks for them: `slices.Sort` for one element type is one instance of Go's
sort, and using it for 30 element types is 30 instances.
