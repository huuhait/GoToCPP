# Isolated reference-count experiment

Phase 15.4, base `919f1a820006b407c03f463821968d49470cefcb`. This directory is
standalone research, not linked to the compiler/runtime, and introduces no default switch,
ABI/ownership change, or production API. ADR-004 remains the production design.

## Frozen protocol (before timing)

Compare the available GCC and Clang, C++20, `-O2 -pthread`, no LTO. Both arms live in the
same executable, use the same libstdc++, create/join a thread **before either arm**, and
assert glibc's `__libc_single_threaded == 0` throughout. The candidate never substitutes
non-atomic counting. The baseline is `std::shared_ptr`, **not the full `gp::ptr` runtime**:
this isolates ownership primitives without giving the candidate credit for omitting runtime
checks, the native registry, or bounded release.

Three kernels, fixed before collecting samples:

1. Owner-only copy/drop: 5,000,000 copies of the backing-allocation owner, reading a live
   payload through the separately retained pointer. Relevant to a string/slice's owner slot.
2. Aliased pointer copy/drop: 5,000,000 copies of an interior-member alias, reading that member.
3. Allocation/lifetime/weak: 500,000 single-block constructions, member aliases, weak creation,
   successful lock, destruction at last strong release, failed lock, then last weak release.

Each kernel warms both arms for 10,000 iterations, then takes four samples per arm in
AB, BA, AB, BA order. No adaptive repetitions or tuning after results. Use process CPU
nanoseconds, retain **all** samples, and summarize medians/ranges and paired ratios, not best
samples. The machine is shared: serialization by the shared performance lock is not CPU or
memory isolation. Small differences are inconclusive, not wins. No application speedup claim.

GCC/Clang opaque inline-assembly barriers expose copied handles/allocated pointers equally;
checksums and destruction counts are checked outside the timer. The lifetime loop also checks
expired/failed locks within the timed work. These barriers and repeated hot tiny allocations
are microbenchmark artifacts. Inspect emitted assembly to establish that allocations and
atomic copy/drop operations survive, without tuning the implementation to it.

Storage probes use an identical stateless recording allocator and `allocate_shared` versus
candidate `allocate`, outside timing. Record value/header/alias/weak sizes and alignments,
allocation count/requested bytes, glibc `malloc_usable_size`, and totals **including the
allocation**. The allocator's usable bytes exclude allocator metadata, fragmentation and RSS;
the requested-minus-value number includes control/padding, not necessarily a standalone
control-block `sizeof`. Both coallocations retain dead value storage while a weak survives.
The string/slice layouts are illustrative structs, not emitted/runtime layout changes.

## Supported subset and proof boundary

`rc.hpp` puts two atomic 32-bit counts and virtual destruction/deallocation dispatch in the
allocation itself. An allocation-only `owner` is one word. An interior `ptr<T>` must still
store both the owner and the pointee, so it is two words, not a universal 8-byte pointer.
Aliases must refer to that allocation (including a null alias); arbitrary out-of-block
addresses are caller errors. A non-null alias without an owner is explicitly rejected.

The initial strong count is 1 and the initial weak count is 1 (an implicit weak anchor).
Copies increment relaxed. Release decrements acquire/release; the thread changing strong
1 to 0 destroys the value exactly once, **then** drops the implicit weak anchor. A weak lock
uses an acquire/release CAS (matching libstdc++'s success ordering) only while strong is nonzero. It either increments before last release,
thereby keeping the value alive, or observes 0 and cannot resurrect it. Weak references keep
the control allocation valid across that CAS. The last weak frees the allocation. Construction
failure frees storage and propagates the exception. Allocation failure also propagates.

Different handle objects may copy/drop the same allocation concurrently. Concurrent access
that mutates the **same handle**, or unsynchronized pointee access, remains a data race.
Weak equality includes owning-block identity and interior address even after expiration;
no address-based allocation recovery is attempted. Counts are bounded below 2^32 simultaneous
references (overflow terminates); tests do not approach that bound. Destructors and allocator
copy/deallocation must not throw. Lock is lock-free only where the platform's atomic type is.
The state-machine argument plus bounded tests is not a formal proof or a concurrency campaign.

Not implemented: foreign/shared-owner bridges, native lifetime registry, arrays and slice
allocation, static/foreign owners, bounded-depth recursive destruction, arbitrary deleters,
full allocator propagation, incomplete/void pointee API, atomic handle load/store, Go nil
panics/weak hashing, unsafe conversion, runtime/debugger ABI, generated layout tables,
Windows/arm64 portability validation or any production integration. There is **no** drop-in
compatibility claim. A future adoption job needs those obligations and new matched evidence.

## Reproduction

From an already-created private output directory, run **one heavy command at a time** using
this project's required wrapper (do not run this as part of broad tests):

```sh
OUT=/absolute/private/output
CACHE=/absolute/private/nix-cache
mkdir -p "$OUT" "$CACHE"
timeout 420s env XDG_CACHE_HOME="$CACHE" GOCPP_MODCACHE=off GOCPP_OFFLINE=1 \
  GOCPP_TEST_TIMEOUT=6m bash /path/to/prescribed/testslot.sh \
  nix develop --offline --no-write-lock-file path:/absolute/checkout -c \
  bash /absolute/checkout/tools/rcexperiment/run.sh check g++ "$OUT/gcc-check"
# Repeat check with clang++; then bench, separately, for each compiler:
timeout 420s env XDG_CACHE_HOME="$CACHE" GOCPP_MODCACHE=off GOCPP_OFFLINE=1 \
  GOCPP_TEST_TIMEOUT=6m bash /path/to/prescribed/testslot.sh \
  nix develop --offline --no-write-lock-file path:/absolute/checkout -c \
  bash /absolute/checkout/tools/rcexperiment/run.sh bench g++ "$OUT/gcc-bench" \
  /absolute/path/to/shared/perf.lock
```

Checks comprise only this directory's correctness executable (normal, ASan+UBSan+LSan and
TSan), with 120-second compile and 60-second execution limits. A recognized TSan startup
failure is reported unsupported, never passing. Any other compile/test/instrumentation error
fails. Timing waits at most 120 seconds for the existing shared performance lock and runs at
most 60 seconds. Output retains identities/hashes, build logs, statuses, stderr, assembly and
raw samples. No downloads, system modifications, full CI, plan edits or cache sharing.

## Results (2026-10-21 UTC)

**Recommendation: keep this isolated prototype/evidence, defer any production replacement.**
The two hot copy/drop kernels improve on both compilers and owner-only storage is 8 bytes
smaller in this subset. That is not a production acceptance result: Clang's lifetime kernel
is effectively a tie, interior-pointer storage is unchanged, and the production integration
obligations above remain open. No ADR, generated/runtime API, layout table, ownership policy,
plan acceptance box, or production default is changed. No project-wide speedup is inferred.

Host: Linux x86-64, AMD Ryzen Threadripper PRO 9995WX, 192 logical CPUs, glibc 2.42.
Compilers: GCC 15.3.0 and Clang 21.1.8, both using the development shell's libstdc++.
Not pinned to a CPU; frequency scaling and other machine activity were not controlled.
The observed within-run ranges are not confidence intervals or a multi-machine noise bound.

### CPU nanoseconds per iteration

Medians, with observed min–max in parentheses; paired ratio is the median of each sample's
candidate/baseline ratio. All four samples per arm, in their actual order and with raw integer
CPU nanoseconds/checksums/destruction counts, are in [results/gcc.csv](results/gcc.csv) and
[results/clang.csv](results/clang.csv). No sample was discarded.

| Compiler / kernel | shared_ptr | intrusive | Paired ratio |
|---|---:|---:|---:|
| GCC owner copy/drop | 13.762 (13.760–13.885) | 10.586 (10.586–10.645) | 0.7692 |
| GCC alias copy/drop | 13.762 (13.761–13.763) | 10.585 (10.585–10.587) | 0.7692 |
| GCC lifetime/weak | 59.276 (59.272–59.282) | 51.072 (51.069–51.078) | 0.8616 |
| Clang owner copy/drop | 13.761 (13.759–13.791) | 10.598 (10.585–10.613) | 0.7694 |
| Clang alias copy/drop | 13.815 (13.759–13.963) | 10.850 (10.849–10.897) | 0.7871 |
| Clang lifetime/weak | 59.264 (59.142–59.622) | 58.774 (58.707–58.833) | 0.9927 |

The <1% Clang lifetime difference is **inconclusive**, not an allocation-performance win.
GCC's lifetime result does not establish that other payloads, allocators, thread contention,
weak-heavy caches, recursive release, or application workloads improve. No single-thread
fast-path comparison or contended-performance test was run.

### Storage accounting (both compilers)

| Quantity | shared_ptr baseline | intrusive candidate |
|---|---:|---:|
| Payload bytes / alignment | 16 / 8 | 16 / 8 |
| Owner-only handle bytes / alignment | 16 / 8 | 8 / 8 |
| Aliased-pointer handle bytes / alignment | 16 / 8 | 16 / 8 |
| Weak handle bytes / alignment | 16 / 8 | 16 / 8 |
| Requested coallocation / alignment | 32 / 8 | 32 / 8 |
| Control + padding (requested minus payload) | 16 | 16 |
| glibc usable coallocation | 40 | 40 |
| Allocations / deallocations | 1 / 1 | 1 / 1 |
| Owner-only handle + requested heap | 48 | 40 |
| Alias handle + requested heap | 48 | 48 |
| Alias + weak handles + requested heap | 64 | 64 |
| Alias + weak handles + usable heap | 72 | 72 |
| Illustrative string header + requested heap | 64 | 56 |
| Illustrative slice header + requested heap | 72 | 64 |

The actual candidate control base is 16 bytes/alignment 8; its in-place payload block is
32/alignment 8. The baseline's control/padding is observed through the allocator, not an
assumed standard-library control layout. `gp::weak_ptr` is 24 bytes because it also preserves
an explicit address for Go equality/hash; this benchmark's baseline weak type is the 16-byte
standard primitive, not that wrapper. Nothing here redefines the runtime's weak layout.
For a value retained only by weak handles, both implementations destroy the value promptly
but retain the entire 32 requested / 40 usable heap bytes, not just the 16-byte count header.
Usable heap totals still exclude allocator metadata and fragmentation; RSS was not measured.

### Correctness, commands and retained evidence

All runs used this prefix, private offline caches, one heavy command at a time:

```sh
timeout 420s env \
  XDG_CACHE_HOME=/tmp/claude-1000/rc15.4-evidence/nix-cache \
  GOCPP_MODCACHE=off GOCPP_OFFLINE=1 GOCPP_TEST_TIMEOUT=6m \
  bash /tmp/claude-1000/-home-huuhait-GoToCPP/fa18916c-20de-43da-b276-dbf712f662ca/scratchpad/testslot.sh \
  nix develop --offline --no-write-lock-file path:/home/huuhait/GoToCPP-15.4 -c
```

The command after `-c` was `bash tools/rcexperiment/run.sh MODE COMPILER OUT [LOCK]`
with the script's **absolute** checkout path. Actual combinations below; `OUT` is under
`/tmp/claude-1000/rc15.4-evidence/`, and `LOCK` was
`/home/huuhait/.cache/gocpp-resume.rPhZsx/perf.lock` for both benchmarks.

| Mode | Compiler | OUT | Result |
|---|---|---|---|
| check | g++ | gcc-check | normal, ASan+UBSan+LSan, TSan all pass |
| check | clang++ | clang-check | normal, ASan+UBSan+LSan, TSan all pass |
| bench | g++ | gcc-bench | 24 timed samples, all sinks checked, exit 0 |
| bench | clang++ | clang-bench | 24 timed samples, all sinks checked, exit 0 |
| check | g++ | gcc-final | final-source normal, ASan+UBSan+LSan, TSan all pass |
| check | clang++ | clang-final | final-source normal, ASan+UBSan+LSan, TSan all pass |

The initial GCC command also ran `clang-format -i` on the four C++ files before checking.
Each check executable runs 10 cases for **each** backend, including 32 races with two
workers per backend and 512 lock attempts per worker, plus candidate invalid-owner/weak
identity tests. Final normal/sanitizer executions all exited 0 with zero-byte stderr.
TSan genuinely ran successfully on both toolchains; nothing was classified unsupported.
No full runtime or sanitizer sweep, Windows/arm64 execution, or production acceptance run.
`bash -n tools/rcexperiment/run.sh` also passed.

After initial measurement, weak lock's successful CAS was strengthened from acquire to
acquire/release to exactly match libstdc++'s ordering. This is a fairness correction, not a
performance retune. Both final-source check sets above were rerun. Each final benchmark was
rebuilt using `timeout 120s COMPILER -std=c++20 -pthread -O2 -Wall -Wextra -Werror
/absolute/checkout/tools/rcexperiment/rc_bench.cpp -o OUT/bench` inside the same wrapper/shell,
and `cmp OUT/bench PREVIOUS_BENCH/bench` succeeded for **both** compilers. The final and
measured binaries are byte-identical, so no extra timing samples were collected/selected.
Initial and final check/build/identity logs, statuses, all stderr and benchmark assembly are
retained under the OUT directories above. This makes the pre-correction source hash in the
initial benchmark identity logs explicit rather than silently presenting it as final-source
provenance.

Assembly inspection found retained allocation calls (`_Znwm`) in both lifetime loops,
locked increments/decrements in both copy loops, and locked CAS in both weak-lock paths.
The candidate commonly uses locked add/sub; libstdc++ commonly uses locked add/xadd and
retains its `__libc_single_threaded` branch. Both execute the atomic branch; no unthreaded
baseline was timed. That instruction-shape observation is not an application-level cause
analysis or a promise on another architecture/compiler.

### Snapshot identities

Only eight files are added here: `rc.hpp`, `support.hpp`, `rc_test.cpp`, `rc_bench.cpp`,
`run.sh`, this README, and the two raw CSV files. Base commit is the full hash at the top.
No production file is edited. SHA-256 identities:

```text
c273c02c787b3258938b2f314d3612b5f26753774b5b761c5cebee41116efbb6  rc.hpp
7eb446aad9a6d1a6c94614eb85037679a91b8e40bfddffe08cc2b75d4b710662  support.hpp
49ea17c4ad6ebf0f9fc0e9a58e7452c83e7c028d237fae62f8fb89c5959a3488  rc_test.cpp
4a1aabd7fb3f53a6ba3702457d3791ff92deb22796fe29fa832709dd16c439a3  rc_bench.cpp
e9b2067fa1b21ffde08e2d9923eec2316cc4c38178d62678f07fdfd721af9b3c  run.sh
cb7c87857289d76a90cbe3f74ef3bac022e2e968f1bfc9be5641c0dc098bcac2  results/gcc.csv
6f4918c1f1e3597a1235d84a69eb0de99e8428e3252cab150921d81459669186  results/clang.csv
4fc3712e0a6a2c7ba805fff3fa5d8fec6f9ff624c56771ccda01fc2b20833209  ../../flake.lock
3654f8410908010235fc9b008bf5807d45b3dac9b1774ecf645e85d3468e4552  GCC measured/final binary
79d2a0df28d6a259255e782b923e060d5ba86a4bccd725e6f86ecc2217299ccb  Clang measured/final binary
```

Compiler executables resolved to:

```text
/nix/store/dy6j7267wrgjlr2z748sbx83zj16vs3c-gcc-wrapper-15.3.0/bin/g++
/nix/store/injsp2brg9mjxljavd5idjyms18b5izy-clang-wrapper-21.1.8/bin/clang++
```
