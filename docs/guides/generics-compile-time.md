# What generics cost to compile

Generic code compiles like the same code written out by hand, because that is what it is: gocpp lowers
each **instance** (a generic function or type with its list of type arguments) to ordinary C++ written with
those types, not to a C++ template ([ADR-030](../adr/030-generic-instances.md),
[cpp-mapping.md](../spec/cpp-mapping.md#generics-steps-71-to-73)). So the C++ compiler does no template
deduction or instantiation for your generics, and the cost has three parts:

1. gocpp's own work (check and lower: milliseconds for a program of a few thousand lines);
2. the C++ compiler on the text of each instance, which is the cost of that function written by hand;
3. for generic **types**, the struct, descriptor and small methods of every instance are in `pkg.hpp`, so
   each translation unit parses them again. Larger method bodies are now defined once in a descriptor
   translation unit (15.7b; see [the current measurement](#bounded-body-placement-157b)).

The first tables are the historical measurements of step 7.12, before body placement changed. The
15.7b section reports a separate, current baseline/candidate comparison; do not mix the two machines'
numbers. The ordinary build advice below does not need configuration changes.

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
   bodies are compiled once and not parsed by every unit. The bounded implementation below moves
   larger bodies; the small inline bodies still have macro guards (`_POST`). There is no user switch.

7. **An external compiler cache is optional.** The existing compiler command accepts
   `CXX='ccache g++'` or `GOCPP_CXX='sccache clang++'`; there is no extra `GOCPP_CCACHE` switch.
   Keep gocpp's own cache on for normal builds. When comparing external cold/warm builds, bypass
   the internal object cache and up-to-date check and inspect the external tool's hit/miss counters:
   a shorter second build can simply be runtime/PCH reuse. Changing work folders and debug paths
   can prevent external hits, and PCH support is tool-specific. The [build specification](../spec/build.md#compiler-launchers-and-external-caches)
   gives the configuration, measurement recipe and compiler-identity limits. No external-cache
   speedup is included in the tables above.

## Bounded body placement (15.7b)

Implemented and independently reviewed on 2026-10-20, from baseline `1c7081e9`. Larger generic-type methods are ordinary,
non-`inline` definitions in an existing `pkg_desc.cpp`; their member declarations stay in `pkg.hpp`.
The types, names, namespaces, inline descriptors, method adapters and generic-function helper definitions
are unchanged. No C++ templates, modules, new translation-unit kinds or cache mechanisms are
introduced; an otherwise absent `pkg_desc.cpp` is emitted when needed.

The inline policy keeps at most **three lowered statements**, counting receiver copies, temporaries,
individual declarations, control statements, loop initializers and nested bodies, not just Go source
lines. A closure or a call spelled `append`, `make` or `new` makes the method out of line even when its
source is short. Simple getters/setters, small call wrappers and tiny scalar loops therefore remain
visible to `-O2` without LTO, but the T workload's two-line `Add` is compiled once. An initial policy that
rejected all control flow regressed an eight-iteration scalar loop on clang++; the optimized check
caught that and the bounded recursive count replaced it. This is still a heuristic, not a guarantee
of runtime neutrality for larger methods. The original rule of three Go statements would leave every
T method inline.

After the shared-type dependency closure is complete, the first using package in import order emits
the body, in the **declaring package's namespace**. Its header already has the complete instance and
type arguments. Defining every body in the declaring package would fail for `Box[importer.Local]` or
require an import cycle. This choice does not change type identity or ownership, and another source
file or package that uses the same instance does not emit another body. Nested instances and promoted
method forwarders keep the existing ordering and inline adapters. There is no new `#line`
deduplication: the moved body keeps its source positions, and the printer emits the required
directives in the new context. Relocated bodies also receive the using package's macro protection,
including names absent from the header.

### Current matched compile check

The unchanged `ctime_gen.py t 20 10` workload and its handwritten twin, g++ 15.3.0 and clang++ 21.1.8,
Linux x86-64, `-O0 -g`, `-p 4`. Each compiler starts with a private empty runtime/PCH cache; an explicitly
recorded hello build warms it, then all measured builds reuse that runtime/PCH. Program object caching
and the up-to-date check are disabled (`GOCPP_NOOBJCACHE=1 GOCPP_NOUPTODATE=1`); module access is offline.
OS page caches are not flushed, so this is not a cold-machine measurement. The same input paths,
compiler command and flags are used by the baseline and candidate binaries.
Compiler invocations are timed by the same transparent wrapper in both. Runs are serialized under the
shared performance lock, but not CPU-pinned; load averages during the final builds were about 2.0–2.4.

For each compiler the actual order is baseline generic, baseline hand, candidate generic, candidate
hand, then the reverse order. All runs succeeded and printed `600`; no samples were excluded.
CPU seconds (user + system), **both observations**, not the minimum:

| Compiler | Generic baseline | Generic candidate | Hand baseline | Hand candidate |
|---|---|---|---|---|
| g++ | 25.022, 24.644 | 23.645, 23.469 | 10.379, 10.426 | 10.581, 10.530 |
| clang++ | 14.717, 14.363 | 14.230, 13.956 | 7.464, 7.699 | 7.250, 7.302 |

Generic wall seconds were 12.997, 12.695 → 12.240, 12.228 (g++) and 6.737, 6.608 → 6.464, 6.386
(clang++). The handwritten control's generated files are byte-identical between baseline and candidate,
yet its mean CPU time increased about 1.5% with g++ and decreased about 4% with clang++. Generic mean
CPU decreased about 5% and 3%, respectively: **whole-build speedup remains inconclusive at this sample
size/noise**, not the previously estimated +5–10% cost over hand-written code. The candidate still
costs more than twice the handwritten twin with g++, and about twice with clang++.

The deterministic improvement is less repeated method text: 200 `Add` definitions move out of the
header; the 400 `Len`/`First` definitions stay inline. Header bytes fall from 1,052,956 to 903,338 with
g++, and 1,053,756 to 904,138 with clang++ (about 14%). Source paths contribute to these absolute sizes.
`pkg_desc.cpp` grows from 697 bytes to about 149,147 bytes. Per-unit CPU seconds for the generic program:

| Compiler / unit | Baseline pair | Candidate pair |
|---|---|---|
| g++ source | 12.193, 11.896 | 11.462, 11.432 |
| g++ descriptor | 5.817, 5.748 | 5.700, 5.702 |
| g++ init | 5.811, 5.794 | 5.203, 5.168 |
| clang++ source | 5.992, 5.863 | 5.731, 5.660 |
| clang++ descriptor | 3.762, 3.635 | 3.653, 3.598 |
| clang++ init | 3.765, 3.639 | 3.431, 3.367 |

### Optimized non-LTO check

A small `Counter[uint64]` program was built with `-cxxflags=-O2` and no LTO, with both compilers.
Twenty million getter/setter iterations and four million calls of an eight-iteration arithmetic
method were each run in five alternating baseline/candidate pairs. All outputs matched Go.
An initial policy that outlined every loop regressed the arithmetic case on clang++ from a median
19.879 ms to 35.543 ms (about 79%); that prototype and its raw samples are retained separately, not
mixed into the final comparison. Counting loop initializers and bodies under the small-body budget
keeps that method inline and removes this observed regression. The final medians (MAD), milliseconds:

| Compiler / case | Baseline | Candidate |
|---|---|---|
| g++ getters/setters | 87.074 (0.281) | 86.786 (0.358) |
| g++ small arithmetic loop | 35.612 (0.101) | 35.650 (0.251) |
| clang++ getters/setters | 92.708 (0.209) | 92.178 (0.070) |
| clang++ small arithmetic loop | 19.787 (0.155) | 19.787 (0.074) |

These are narrow checks that the retained small methods still optimize, not proof that every
out-of-line method is runtime-neutral. The cross-package fixture also compiles and runs at `-O2`
with larger out-of-line methods, pointer/value receivers, promoted adapters and a closure.

The compile and runtime tables above use the final candidate (SHA-256 beginning `5ed00b17`) against
the unchanged baseline (`12d6a56d`). Earlier prototype runs are retained with the 15.7b handback;
none are substituted for the final measurements or excluded from their own revision's samples.

The historical 7.12 estimate also proposed suppressing repeated `#line` directives. This job does not
implement that separate change, package PCH, shared-instance naming, modules or an external cache.
Full 15.7 remains open; these measurements are not a project-wide performance acceptance.

## What to expect for a big program

These historical 7.12 numbers illustrate scale, not current build-time budgets after 15.7b.
A program with 2000 generic function instances builds in 17 s (g++) or 8 s (clang++) of CPU at `-O0`, in one
translation unit when it is one file: the wall time is about the same, because one unit is one process. Split
into several files the units compile in parallel. A program with 200 instances of generic types is 28 s (g++) or
15 s (clang++) of CPU, in 3 units that run at the same time. The `slices`, `maps` and `cmp` functions are
instances like yours, in the unit that asks for them: `slices.Sort` for one element type is one instance of Go's
sort, and using it for 30 element types is 30 instances.
