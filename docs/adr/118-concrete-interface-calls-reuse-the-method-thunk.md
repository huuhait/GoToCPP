# ADR-118: Proven-concrete interface calls reuse the method thunk directly

Status: accepted (2026-10-20), after independent review and focused manager checks. Phase 15,
bounded job 15.6a.
Bounds-check work remains separate (15.6b).

## Context

Interface wrappers call `gp::icall`, which loads a method-table entry at runtime. At
`aada8d18`, a local initialized from a known concrete value still uses this path even when
nothing can change the local's dynamic type. The class-hierarchy analysis in ADR-114 proves
purity for borrowing decisions; it does not change dispatch. GCC and Clang at `-O2` leave an
indirect call in the small kernel below.

## Decision

Only a direct concrete-to-interface conversion, or an interface local with one initializer
and no other definition or exposed address, may bypass dispatch. All uses of that local must
be direct method-call receivers: even a comparison or a copy declines the optimization.
This rules out passing the Go interface to an opaque C++ reference parameter without a new
foreign alias analysis. The local proof reuses `singleDef` and `memoryVar`; it does not infer
a type from possible implementations of an interface. Only declared methods of non-generic
named types in the same package qualify.

The interface conversion, allocation, value copy and owning lifetime all remain. The call
uses the **actual interface**, never the initializer again. An immediately invoked lambda
uses the same body and parameter types as the existing method-table thunk (`thunkFunc`):

- Value receivers read the original boxed copy, not the current source variable.
- Pointer receivers use the same borrowed or owning unbox as before. Knowing the concrete
  type does not introduce any additional borrowing or change escape analysis.
- The value-method wrapper for a nil `*T` is unchanged; a typed nil interface still has a
  concrete type. Nil interfaces take the old dynamic path.
- The lambda takes the existing thunk's argument values before unboxing the receiver, so
  argument evaluation cannot invalidate an unowned receiver snapshot.
- `icall_ticket` still handles the exact interface slot, preserving recover-ticket behavior.
- The lambda has the runtime's zero template tag for compiler helpers, so it does not add a
  fictitious Go closure frame to stack traces.

The order pass treats the lambda receiver as an ordinary argument, not a C++ member-call
object. This is necessary: C++ guarantees that a member-call object is evaluated first but
may evaluate ordinary arguments right to left. Explicit receiver and argument calls are
therefore ordered by the existing machinery before the adapter call.

Everything unproved retains dynamic dispatch: assignments or address aliases, interface
copies/conversions, fields/elements, unknown interface results and parameters, promoted
methods, generic types/functions/method bodies, methods in other packages, foreign parts,
method values, tuple arguments, `go`, and `defer`. Some of these can be proved safe with more
work; widening them is not part of this change. No runtime API or interface ABI changes.

## Focused evidence

`TestConcreteInterfaceCall`, `TestDynamicInterfaceCallFallback` and
`TestConcreteInterfaceForeignFallback` check direct and fallback code shapes. The run fixture
`tests/run/devirt_concrete.gp` covers source-value mutation, held pointer ownership, an owning
receiver, receiver/argument side effects, aliases and reassignment, promoted and generic
fallbacks, nil values, asynchronous/deferred calls, and recover. It and the existing
`borrow_nil` and `borrow_ifacebump` match Go under GCC and Clang; the new fixture also passes
both compilers with `-asan` (ASan/UBSan). `TestTraceInterfaceVsGo/pointer` checks the
proven-concrete call's panic stack against Go without inlining, with both compilers.
The fixture checks the recovered runtime-error kind:
Go may choose an ordinary nil panic after its own devirtualization where the existing gocpp
thunk uses its value-method nil-wrapper message. This change preserves that existing thunk.

The kernel is `bench/programs/dispatch/concrete_iface.gp`, with `benchN = 40000000`. Each
iteration performs a dependent unsigned integer update and accumulates its result. This is
a dispatch/inlining microbenchmark, not a whole-project benchmark or a release-performance
claim. Both emitted C++ and disassembly were inspected: baseline GCC retains `call *%rdx`
in the loop; the candidate inlines the integer update. Interface construction is unchanged.

Measurement: Linux x86-64, AMD Ryzen Threadripper PRO 9995WX, CPU 42 affinity, shared host;
GCC 15.3.0, Clang 21.1.8, Go 1.25.14; `gocpp build -cxxflags=-O2`, checks enabled, no LTO.
Seven alternating forward/reverse runs of baseline/candidate/Go, with every output checked.
Reported CPU time is child user + system time; MAD is median absolute deviation.

| Implementation | Baseline median (ms) | Candidate median (ms) | Baseline / candidate MAD (ms) |
|---|---:|---:|---:|
| GCC | 40.904 | 32.836 | 0.039 / 0.046 |
| Clang | 49.239 | 33.120 | 0.440 / 0.320 |
| Go reference | 41.481 | — | 0.800 / — |

Baseline compiler: clean `aada8d18`, SHA-256
`dadae37cc6250d5d6b01f1610e82e38c32d4188afb4a81a84570fcc4ac73e448`.
Candidate compiler: that base plus this uncommitted job, SHA-256
`af64211e265cd1cf8b00a80605529cdd9e48ed561d281ba06d838e3ff1bc50ad`.
The benchmark input is the checked-in source with `const benchN = 40000000` appended;
SHA-256 `e241a6807c19d906e83ce35b8af3e0b69587f001e77e55382d64ddb20b982a8f`.

| Binary | SHA-256 |
|---|---|
| Baseline GCC | `5ec1d401831a5d9871875bdef7b722e89e882b761c643b8db7cd92104c8cc63b` |
| Candidate GCC | `a49f9f50f89343a4c6cdf8879c092d62e217f6c9d9a3ca681b92673ad3f3f2e1` |
| Baseline Clang | `1004726adb1f794122b4d2abe9ead56e34764b544238d4bfce475179505232a0` |
| Candidate Clang | `3027e12655cf9c7add5770f1d11cfad7531ca53718f6b0d8c8076434c8a3b2c9` |
| Go | `47412e01c84cd70a295fb82d42ff5bfefb50c1b51bb0cd2096f63406c0b0a30c` |

No full suite, sanitizer campaign, release/LTO matrix or Windows test was run for this job.
