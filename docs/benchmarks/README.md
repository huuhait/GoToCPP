# Benchmarks

The benchmark suite measures gocpp against real Go and against hand-written C++20 on the same
programs, tracks the numbers per commit and fails a change that makes gocpp more than 5 % slower.
The latest generated table is [runtime.md](runtime.md). The goals it is measured against are those of
Phase 15 (plans/2026-10-13-gocpp/phase-15-hardening-release/README.md): the geometric mean of the CPU benchmarks at or below Go's,
and no benchmark more than 2 times slower than Go.

## What is measured

`bench/programs/<category>/<name>.gp` is one program. It is written once in Go syntax, so the same file
is built by gocpp as `main.gp` and by the Go toolchain as `main.go`: one source of truth, no
drift between the two. The few things that differ (the problem size, `math.Sqrt` against
`std::sqrt`) live in a generated `shim` file (`bench/shim/*.tmpl`). The header of a program says
what it is:

```go
// bench: nbody          the name (the file name)
// category: cpu         cpu, data, dispatch, runtime or generics (the folder name)
// size: 5000000         the problem size of the full run (the constant benchN)
// smoke: 20000          the problem size of the smoke run
// twin: yes             bench/cpp/nbody.cpp exists
// needs: sync           optional: what gocpp does not have yet (see below)
// about: one line       what it measures
```

| Category | Programs |
|---|---|
| cpu | binarytrees, collatz, crc32, fannkuch, fasta, fib, fnv, knucleotide, mandelbrot, math_funcs, matmul, nbody, nqueens, quicksort, revcomp, sha256, sieve, spectralnorm |
| data | map_int, map_string, slice_append, slice_copy, list_alloc, tree_alloc, struct_copy, string_concat, byte_process, strconv_roundtrip, unicode_classify, strings_ops\*, bytes_buffer\*, slices_sort\* |
| dispatch | iface_call, type_switch, embedding, closures, defer_loop, method_value |
| runtime | goroutine_spawn, chan_pingpong_buf, chan_pingpong_unbuf, select_loop, alloc_churn, fmt_sprintf, fmt_println, panic_recover, mutex_counter, waitgroup_fanout, time_now, reflect_fields |
| generics | generic_sum_instances, generic_minmax, generic_map_filter, generic_stack\* |

\* does not build yet, see below.

`bench/cpp/<name>.cpp` is the C++ twin of a compute kernel (idiomatic C++20, not a transliteration: `std::vector`,
`std::unordered_map`, `std::sort`-free where the Go program has a hand-written sort, and so on), built with
`-O2` and the size in `-DBENCH_N`. 32 of the programs have a twin. The twin is the reference for "what does the
abstraction cost"; Go is the reference for the Phase 15 goals.

A program with a `needs:` line is skipped (in all implementations, with the reason in the table "Does not build yet" of
runtime.md) while gocpp's build says the thing is "not available yet" or "not supported yet"; it starts to be measured
when gocpp can build it, with no change to the suite. Without a `needs:` line the same build failure is a failure of the
run. Today (dev at the time of step 15.1) every program builds: `strings_ops`, `bytes_buffer`, `generic_stack` and
`slices_sort` carried a `needs:` line until `strings`, `bytes`, generic types and `slices.Sort` landed. The packages the suite uses:
`fmt`, `math`, `strconv`, `unicode`, `strings`, `bytes`, `slices`, `sync`, `time`, `reflect`, `errors`. Still missing for
the programs of the Phase 15 list: regex-redux (`regexp`) and a JSON-ish parser (`encoding/json`).

**Flavors.** gocpp builds in three flavors, and the suite reports each against Go:

| Flavor | What it is | Implementation names |
|---|---|---|
| `default` | `gocpp build` without flags: today -O0 -g (what users get) | `gocpp-g++:default`, `gocpp-clang++:default` |
| `O2` | `GOCPP_CXXFLAGS="-O2 -g0"`: optimized, no LTO (`-cxxflags` changes the flags) | `gocpp-g++:O2`, `gocpp-clang++:O2` |
| `release` | `gocpp build -release` (-O2 and LTO, Phase 06 step 6.11); skipped, with the reason in the table, when `gocpp build -help` does not list `-release` (older gocpp) | `gocpp-g++:release`, `gocpp-clang++:release` |

`-flavors default,O2,release` picks them (the default for `make bench`; `make bench-smoke`, which `make ci` runs, uses `O2` only: the default flavor is covered by the test suites and `BENCHFLAGS="-flavors default,O2,release"` adds it; `bench-check` O2).
Only the optimized flavors are compared with the optimized C++ twins. The other implementations are `go` and the twins
`cpp-g++`, `cpp-clang++` (-O2, built with the same compilers).

Also measured: peak resident memory, CPU time, stripped and unstripped binary sizes, and with `-compile`
the build times of generated programs (hello, 20 packages, 100 generic instances, one file of 1000 lines): cold
(empty caches: for gocpp that includes building the runtime), warm (runtime or standard library cached, one
literal in `main` changed) and no-op, for gocpp in the three flavors and for Go.

Not in the suite yet, because the packages do not exist: regex-redux (`regexp`) and a JSON-ish parser
(`encoding/json`) of the Phase 15 list, `bufio`/`io`/`os` file throughput. They are added as programs with a
`needs:` line the day the package lands.

## How it measures

`go run ./tools/gobench` (the commands below) builds every selected program with every implementation, then runs
**rounds**: one round runs each implementation once, in a fixed order, so a change of the machine's load during a
program hits all of its implementations alike. The first round (`-warmup`, default 1) is dropped; the next
`-reps` (default 10) rounds are measured: wall time and user+system CPU time of the child from the kernel's accounting, and its peak
resident set (`getrusage`'s `ru_maxrss` is the larger of the child's peak and the runner's own high-water mark at the exec, so it is used only
when it is above that mark; otherwise the `VmHWM` of `/proc/<pid>/status`, sampled every 2 ms, is used, and a run shorter than
20 ms, which ends before the sampler sees its real peak, has no figure: `-`). Statistics are the **median**, the **MAD** (median absolute deviation, robust against one slow run) and
the minimum. The table shows `median ±MAD%`.

- **Outputs are compared.** Every program prints a result that depends on all of its work; the runner hashes stdout
  and compares the hash of every implementation with Go's. A difference is a `MISMATCH`: loud, and the run exits 2
  whatever else is allowed. A binary whose output changes between runs is the same. A program that exits with a status
  other than 0, or runs longer than `-timeout` (default 120 s), is a `run-failed`.
- **Pinning.** With 4 idle cores (95 % idle over 0.4 s, `-pin auto`) the runs go through `taskset` on them; on a busy
  machine they do not, and the report says so.
- **Machine information** is in every result: CPU, cores, memory, OS, versions of Go, g++, clang++ and gocpp, the load
  average before and after, the pinning and `gomaxprocs` (the parallelism the programs get, which is also Go's garbage
  collector's: a pinned run and an unpinned one, or runs on 4 and on 150 cpus, are different experiments). Two
  machines' numbers are never compared by the gate.
- **Environment.** The benchmarked programs run without `GOGC`, `GOMAXPROCS`, `GODEBUG`, `GOMEMLIMIT`, `GOEXPERIMENT`,
  `MALLOC_*`, `LD_PRELOAD` and `GOCPP_*` from the shell of the runner (and gocpp builds without `GOCPP_*` other than the gate
  phase), so every implementation runs with its defaults. An interrupted run (SIGINT, SIGTERM) removes its temporary work folder.
- **Load.** A run whose 1-minute load average is over `-max-load` (default: an eighth of the cores, at least 4: 16 on
  the 128-core development machine) is called noisy in the report. The authoritative numbers come from a quiet machine.
- Processes are children in their own process group, killed on a timeout and (Linux) with the runner (`Pdeathsig`).

## Commands

| Command | What it does |
|---|---|
| `make bench-smoke` | tiny sizes, one run: builds and runs everything on every implementation (O2 flavor; about a minute) and compares the outputs. **Part of `make ci`.** The times mean nothing |
| `make bench` | the full run with `-compile`; writes `bench/results/` (gitignored), [runtime.md](runtime.md) and a line of `bench/history/<host>-<cores>c.jsonl`. An hour or more |
| `make bench-check` | the regression gate, below. Not part of `make ci`: timing needs a quiet, fixed machine |
| `make bench-baseline` | measure and write `bench/baseline.json`. Deliberate: say why in the commit message |
| `make bench-history` | `go run ./tools/benchhist`: the history table |
| `go run ./tools/gobench list` / `validate` | list the programs; check the headers and twins (also a unit test) |
| `go run ./tools/gobench report -in FILE` | write the markdown of a JSON result |
| `go run ./tools/gobench merge -base bench/baseline.json -add FILE` | put the programs of a partial run into the baseline (for a program added or resized after the last full run) |

`BENCHFLAGS` passes flags: `make bench BENCHFLAGS="-match '^cpu/' -reps 5"`, `-flavors O2`, `-impls go,gocpp-g++:O2`, `-jobs`, `-timeout`,
`-pin 4,5,6,7`, `-cxxflags`, `-keep -work DIR` (keep binaries, for perf and objdump). `-match` is a regular expression
over `name`, `category` and `category/name`.

## The regression gate

`make bench-check` re-measures the suite (same repetitions as the baseline's) and compares each gocpp
benchmark's wall time with `bench/baseline.json`. It fails (exit 1) only when all of this holds:

1. the **median** is more than 5 % over the baseline's,
2. the **fastest run** is more than 5 % over the baseline's fastest run (a busy machine slows some runs, not the best
   one: if only the median moved, the answer is "inconclusive"),
3. the difference of the medians is more than twice the larger MAD,
4. it **reproduces**: a suspected regression is measured again with the same binaries (`-retries`, default 2), and one
   clean re-measurement clears it.

Before comparing, both sides are corrected for **machine drift**: the geometric mean over all benchmarks of
(time of Go and of the C++ twins now) / (their time in the baseline). gocpp does not influence those, so a machine that
is 10 % slower today does not make gocpp look 10 % slower. `-no-drift` turns it off.

The gate is **inconclusive** (exit 0, said loudly) when the load average before the run was over the limit, when the
baseline was measured on another CPU or core count, when one of the two runs was pinned and the other not, when the
parallelism (`gomaxprocs`) differs, when the baseline itself was measured on a busy machine (`-trust-baseline` compares anyway),
or when nothing could be compared. `-require-quiet` turns those into
failures, for a CI machine that is supposed to be quiet. A benchmark that fails to build or run, or has a `MISMATCH`, fails
the check on its own.

The intended setup (Phase 15 §2.1) is a fixed benchmark machine with the same CPU and no other load, where the baseline is
written once per release and `make bench-check` runs for every merge. The tools make no assumption beyond that; shared CI
runners are too noisy for a 5 % gate.

## History per commit

`make bench` appends one JSON line to `bench/history/<host>-<cores>c.jsonl`: commit, subject, date, machine, load,
and the median wall time of every `category/program/implementation`. The file is committed with the change it measures, so
`git log` of the history file is the performance log. `go run ./tools/benchhist` prints a table (one row per commit: the
geometric means of gocpp over Go and over C++ per group, the change from the row before, `[other machine]` when the machine
changed so rows are never compared across machines); `-bench nbody` shows one benchmark per commit, `-group cpu`
restricts the summary, `-last N` the rows.

## What the first run found (2026-10-19, noisy)

The first complete run (docs/benchmarks/runtime.md; `bench/baseline.json` is the same run, `commit` is the dev commit it was
made on top of, `dirty` says the step's own files were not committed yet) was made on the shared development machine (AMD
Threadripper PRO 9995WX, 192 logical cores, shared with about ten other agents, the project capped at 150 cores) at a 1-minute
load average of 67 to 240 (`uptime` at the start: 78; the limit for a quiet machine is 24), 5 runs per implementation,
**not pinned** (`gomaxprocs` 192: Go's garbage collector had the whole machine), dev with `-release` and the prebuilt
runtime library of steps 6.8 and 6.11. Four builds failed at random in the run (see "Builds that needed a second try" below),
and those four programs were measured again in a second, equally unpinned run and merged (`gobench merge`). The medians of
single programs move by 10 to 50 % between runs under that load (the MAD in the tables says how much). Treat every figure as an
order of magnitude, and every ratio between implementations of one program as better than the times (the implementations
alternate within a round). The numbers that matter for Phase 15 are measured on the quiet, pinned benchmark machine at the end
of the phase.

| gocpp over Go, geometric mean | cpu (18) | data (14) | dispatch (6) | runtime (12) | generics (4) | all (54) | all without fib, generic_minmax (52) |
|---|--:|--:|--:|--:|--:|--:|--:|
| `default` build (-O0 -g), g++ / clang++ | 4.9 / 5.4 | 8.3 / 7.8 | 68 / 63 | 7.2 / 7.6 | 7.9 / 7.9 | 8.5 / 8.7 | 9.1 / 9.3 |
| `O2` build, g++ / clang++ | 0.91 / 0.91 | 1.17 / 1.27 | 6.4 / 7.2 | 2.4 / 2.3 | 0.86 / 0.77 | 1.49 / 1.52 | 1.61 / 1.61 |
| `release` build (-O2, LTO), g++ / clang++ | 0.83 / 0.82 | 1.17 / 1.09 | 5.7 / 6.0 | 2.2 / 2.3 | 0.71 / 0.74 | 1.38 / 1.37 | 1.48 / 1.45 |
| `O2` over the C++ twin (32 programs), g++ / clang++ | 1.11 / 1.09 | 1.06 / 1.14 | 3.0 / 3.5 | 1.37 / 1.20 | 1.90 / 1.85 | 1.25 / 1.26 | 1.27 / 1.28 |
| `release` over the C++ twin, g++ / clang++ | 1.01 / 1.00 | 1.17 / 1.01 | 2.6 / 2.8 | 1.45 / 1.23 | 1.45 / 1.78 | 1.18 / 1.15 | 1.20 / 1.16 |

The cpu row without `fib` (the C++ optimizer unrolls and inlines the recursion, `fib` is 3 times faster than Go) is 0.98 / 0.92
for O2 and 0.88 / 0.83 for release; the last column also drops `generic_minmax` (its loop folds away, 0.17 of Go). Quote the
headline with these rows next to it. So the default build, which is what users get today, is about 9 times slower than Go on this
suite (up to 300 times on call-heavy code, because nothing is inlined at -O0); the optimized builds are at Go's speed or a bit
faster on the CPU kernels (the Phase 15 goal) and about 1.5 times Go over everything; the programs with a twin cost 1.2 times
the hand-written C++ (the kernels are at 1.00 with `release`). LTO (`release`) gains about 7 % over plain -O2 here, within the
noise of this run.

Two caveats that favor Go in the wall-time ratios of this machine: Go's garbage collector runs in parallel on all the cpus it
is given, gocpp's reference counting is on the thread that does the work, so allocation-heavy programs look better for Go in
wall time than they cost it: `binarytrees` takes 1.27 s of wall time and **55.8 s of cpu time** in Go (gocpp: 1.18 s and
1.16 s). The JSON file has the cpu time of every run (`cpu_ns`); the tool pins a run on a quiet machine to 4 idle cpus (`-pin auto`) for
this reason. And the goroutine programs (`goroutine_spawn`, `waitgroup_fanout`, `mutex_counter`, `chan_pingpong_*`, `select_loop`)
compare OS threads with Go's user-space goroutines: that is the 2 to 3 times gap below. It is a gap against the Phase 15 goal
"nothing more than 2 times slower than Go", not a point in gocpp's favor and not something this suite excuses.

What the run says about the worst gaps (O2 flavor, g++; causes from stack samples taken with gdb, there is no `perf` here, from
the generated C++, and from the independent review of this step):

| Gap | Where it comes from | Planned step |
|---|---|---|
| **vs Go**: `panic_recover` 38 times slower (2.3 microseconds per panic and recover against 0.06; `release` 25) | a `panic` is a C++ `throw`. In the review's profile, 56 % of the time is `_Unwind_RaiseException` (phases 1 and 2, `uw_frame_state_for`, `execute_cfa_program`), 26 % is `panic_error`'s constructor, which captures a backtrace on every panic (`panic_error::panic_error` -> `gp::os::stack` -> `backtrace(3)` -> `_Unwind_Backtrace`), and 20 % is the `__cxa_rethrow` in `defer_frame::gp_adopt` (a second unwind) | capture the stack lazily (only when the panic reaches the top and is printed), avoid the rethrow in the defer frame |
| **vs Go**: `embedding` 22 times, `defer_loop` 8, `type_switch` 6.7, `iface_call` 5.8, `closures` 3.6, `method_value` 2.9, `bytes_buffer` 14 (`release` 9), `reflect_fields` 8 | call overhead, not allocation. Methods with a pointer receiver are emitted as `static void T::m(gp::ptr<T> b)`: the pointer is taken **by value**, so every call copies a `shared_ptr` (and for an embedded receiver builds a `gp::field_ptr`, the aliasing constructor, first); a `shared_ptr` copy and release costs 4.2 ns on one thread and 14.6 ns once any thread exists, because libstdc++ then switches to atomic operations (it checks `__libc_single_threaded`; the Go runtime has no such cost, and Go inlines the method). The samples agree: `embedding` is `mid::add`, the generated thunks (`gp_thunk_ptop_get`, `gp_thunk_pbase_add`, ...) and the loop; `iface_call` is `icall` plus the thunk (`gp_thunk_psquare_area`) and is the worst gap against the C++ twin as well (6.9 times: the twin's virtual call is a plain indirect call); `defer_loop` is `defer_frame::gp_run` and `gp_release`. `bytes.Buffer` pays the same per call plus `b->buf = gp::sub(b->buf, 0, l+n)` on every byte it writes (a slice header built and assigned with its reference-count traffic; samples: `tryGrowByReslice` and the `gp::slice` helpers) | `const gp::ptr<T>&` or `this` receivers and escape analysis (15.2, 15.3), devirtualization (15.6) |
| **vs Go**: `goroutine_spawn` 2.7, `mutex_counter` 2.7, `select_loop` 2.7, `chan_pingpong_buf` 2.6, `waitgroup_fanout` 2.2, `chan_pingpong_unbuf` 2.1 | goroutines are OS threads (ADR-026): 90 % of the samples of `goroutine_spawn` are `futex` waits in `pthread_cond_wait`, every hand-off is a kernel wake-up. The C++ twin of `chan_pingpong_unbuf` (two threads, one binary semaphore each, a futex pair) is 2.8 times faster than gocpp's channel, so part of the cost is the runtime's channel on top of the thread switch. Programs with fewer, longer-lived threads (`mutex_counter`) still lose 2.7 times, to the cost of the lock path | by design the scheduler stays OS threads, but the gap against the goal is real: channel and mutex fast paths (spinning before sleeping), see Phase 15 |
| **vs Go**: `strconv_roundtrip` 2.2, `unicode_classify` 2.1, `strings_ops` 1.9, `time_now` 1.9 (the ported packages) | the same by-value pointer and slice-header traffic per call | the optimizations of 15.2 to 15.4; the ports themselves stay as they are |
| **vs C++**: `generic_map_filter` 5.5 times slower than the twin (`release` 3.2) | every `make([]T, 0, n)` and `append` growth is a zero-filled array behind a `shared_ptr` (a `memset` and two allocations per slice), where the twin reserves a `std::vector` | slice allocation without the zero fill when the slice is fully overwritten (15.3, 15.6) |
| **vs C++**: `binarytrees` 2.7, `tree_alloc` 2.3, `list_alloc` 1.9 (2.4 with `release`), `slice_append` 1.8 | `shared_ptr` nodes (a control block per node and a reference-count round trip for every child pointer copy and destruction) against `unique_ptr`; against Go `binarytrees` is faster (0.93) and `tree_alloc` is 1.8 times slower | intrusive refcount (15.4), escape analysis (15.2) |

Not a gap: maps (`map_int` 0.40, `map_string` 1.03, `knucleotide` 0.98 of Go's time), `fmt` (`fmt_sprintf` 0.58, `fmt_println`
0.93), allocation churn (0.46), `math_funcs` (0.87) and generic functions (`generic_minmax` 0.17, `generic_sum_instances` 0.92;
`generic_map_filter` 1.8 and `generic_stack` 2.0 of Go).

Compile times (the compile section of runtime.md; seconds, with the prebuilt runtime of step 6.8 in the gocpp numbers): cold
(empty caches, the runtime or standard library built) the default build is at Go's (hello 2.6 against 2.3, 20 packages 4.3 against
4.4), O2 is 1.0 to 1.5 times Go, `release` 1.0 to 2.1 times (LTO), `generics100` (100 generic instances) 2 to 7.5 times; warm
(one literal in `main` changed) 2.5 to 6 times Go in the default build, 6 to 10 in O2 and 16 to 40 in `release`, because a gocpp
rebuild runs the C++ compiler and, for `release`, the LTO link on the changed unit, which is much slower than Go's compile and link
of a small change (`generics100` 16 to 60 times); no-op builds (nothing changed) are faster than Go's.

**Builds that failed at random in the first run (not reproduced, cause not found).** The runner builds `-jobs 4` programs at a
time in one cache directory (as `make -j` or the language server would): 4 of the 324 gocpp program builds of the first run
failed and succeeded when repeated alone: `internal error: generated C++ did not compile`, `did not link`, and
`gocpp build: stat .../gocpp-clang++-release: no such file or directory` (twice; the program of that build is named after the
folder, so this is most likely the program file in the gocpp work folder, which was gone when gocpp moved it; the path is elided here and a build folder of the runner with that name is not excluded). The run is gone, and
nothing says that the four failed at the same time. Since then, 3,840 builds of small programs (both compilers, the default
build, `-release` and `-asan`, cold and warm caches, 96 at a time), 1,062 builds of the benchmark programs (`gobench run
-smoke`, 48 at a time) and 1,000 builds of `tools/buildstress` (a tiny, trimmed object cache, caches replaced while builds ran)
did not fail once, so the cache, the runtime library, the precompiled header and the folders of generated files are not
shown to be the cause. The most likely cause is a removal of the work folders in `TMPDIR` from outside gocpp (a cleaner of the
temporary folder, a script that ends the compilers): `tools/buildstress -wipe-every` makes the same messages with the build of
that day (27 of 150 builds failed). The two `stat` failures are not explained by a removal at a random time, which hits a
compile or a link (seconds) far more often than the tens of milliseconds between the end of the link and the move of the
program; they point to something that is correlated with the end of the link. Nothing in `tools/gobench` removes a build
folder between two phases (each program and implementation has its own folder, and `-build-retries` builds it again one
after the other). gocpp now makes the build again when its work folder is removed, also in that time between the link and the
move of a program (docs/spec/build.md, "A work folder that is removed"), and the runner still tries a failed gocpp build once
more (`-build-retries`, default 1) and names it in the report ("Builds that needed a second try").

These are findings only: nothing was optimized in step 15.1.

## Files

| Path | What |
|---|---|
| `bench/programs/` | the `.gp` programs (also valid `.go`) |
| `bench/cpp/` | the C++ twins |
| `bench/shim/` | the per-implementation size/`sqrt` shims |
| `bench/baseline.json` | the baseline of the gate (schema: `tools/gobench/benchdata/data.go`) |
| `bench/history/` | one `.jsonl` per machine |
| `bench/results/` | the runs (gitignored) |
| `tools/gobench/` | the runner; `benchdata/` has the result types, statistics and the gate decision, with no I/O |
| `tools/benchhist/` | the history table |
| `docs/benchmarks/runtime.md` | the generated table of the last committed run |

## Adding a benchmark

Write `bench/programs/<category>/<name>.gp` with the header, using `benchN` for the size and `sqrt` (from the shim) for
square roots; print a result that depends on all the work (a checksum), identical in Go. Optionally add the twin
`bench/cpp/<name>.cpp` (`BENCH_N`, same output byte for byte), and set `twin: yes`. `go run ./tools/gobench validate`
checks the headers; `go run ./tools/gobench run -smoke -match '^name$'` builds and runs it everywhere. Choose
`size` so that Go takes about 0.3 to 2 seconds.

## Phase 10 features

The cost of `!`, macros and `#Derive` against code written by hand is measured by `go run ./tools/phase10bench` on the programs of
`bench/phase10` (they use syntax that Go cannot build, so they are not part of the suite above): [must-macros.md](must-macros.md) and
[derive.md](derive.md).
