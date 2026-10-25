# gocpp · TODO

**Legend:** `[ ]` todo · `[~]` doing · `[x]` done
**Last updated:** 2026-10-22 (every open line audited against dev 81c0d4e1; 06.lows merged: [AUDIT-2026-10-22.md](AUDIT-2026-10-22.md))

Keep this list short: only what is next. The full step lists live in each phase's `STATE.md`.
Every item is built through the manager → coder → reviewer loop in [WORKFLOW.md](WORKFLOW.md).
Phase 16 has its own list in [phase-16-dual-engine/STATE.md](phase-16-dual-engine/STATE.md).

---

## Now: jobs in progress (2026-10-22)

- [~] Phase 16 (own clone `/home/huuhait/GoToCPP-Gopkg`, branch `p16`; the old phases stay untouched): step 16.1's spike runs as 16.1a (vendor and build bdwgc 8.2.12), 16.1b (dry run of Go's standard library) and 16.1c (GC-engine walking skeleton), started 2026-10-24. 16.1b is merged (`gostd/dryrun/go1.25.14`, `tools/gostddryrun`); 16.1a and 16.1c are in review round 2. Details in [phase-16-dual-engine/STATE.md](phase-16-dual-engine/STATE.md).
- [~] 11.14: guarded-check implementation integrated after clean review and manager focused checks; measure the final build and continue the remaining typing gaps. Every-row gopls acceptance remains open.
- [~] 12.14: safe PCH/prefetch implementation integrated after clean review and manager checks. 12.14b (ADR-121) merged 2026-10-21: cold typed overhead 0.47-1.56 s; the < 0.5 s criterion was reworded (user accepted ADR-121 2026-10-21). Smaller preambles were withdrawn; cold first-run performance stays separate from a run that retains a precompiled header.
- [~] 15.12: CI/release workflows integrated after clean review (`.github/workflows/`); static checks only, never run on GitHub (exit criterion open).
- [ ] Next: remaining 12.13 editor features and real external-cache measurements (ccache/sccache are not installed here).

Recently closed (kept for the record, details in each STATE.md):

- [x] 15.2 escape analysis (b3a1219b), 9.12 stdlib inventory, 15.7a/b/c, 15.6a/b, 15.4, 15.3, 15.derive, 13.vetexit, 9.jsonround, 11.14b, 9.13a (no tag or release is authorized), 12.13a-e (12.13d: C/C++ editor budget row and slow-clang test, ADR-126, 3aa700f1; 12.13e: `textDocument/declaration`, ADR-135).
- [x] `TestHeaderIndexAuxiliaryMetadataDoesNotAccumulate` under `-race` (job 12.racefix, merged 2026-10-21): the hang was about 1100 fake-compiler starts times the 1 s race exit sleep, cured by `internal/racetest.ExitChildrenQuickly`. The `nix develop` leftover problem (dead /nix/store/*-source copies, inode exhaustion) is fixed by job 13.devenv (2026-10-21, 606c008d): `scripts/in-devenv` (`make devenv`, docs/CONTRIBUTING.md).
- [x] Five tests that failed or hung on clean dev fixed 2026-10-21 (job dev.testhealth); `TestPCHCutOffIsMadeAgainWithTheSameAnswers` flake fixed (job dev.docs2); index, name, member validation and member completion timeouts wrap `context.DeadlineExceeded`; docs, `emit -annotate` and `explain` texts updated to the escape analysis (ADR-114); `TestWedgedProgram` stand-ins exit when their parent dies.
- [x] Parent-step typed-interop work: 12.default (typed is the default, ADR-131, 1c342d97), 12.args (ADR-130), 12.check (ADR-134), 04.extern (ADR-128), 15.size (ADR-136), 14.hires/14.sleep (ADR-133/132).

At most five active workers, including reviewers; start the next review as its coder releases a slot.
Only necessary focused checks. No `make ci`, `verify.sh` or full sweeps by any worker or
manager. The user owns the final manual broad verification. Review findings are handed
back together after each round; workers report only at completion or a genuine blocker.

## Triage: what the open lines really are (manager estimate, 2026-10-22)

The open list below has about 87 checkbox lines. They are NOT 87 tasks: many lines bundle several
small "lows", and most bundles are old review leftovers that no user has hit. Rough split by what
the lines are (a line counts once):

| Kind | Lines | What it means |
|---|---|---|
| In a running job | about 9 | recover, `&cvar`, anonymous interface field, reflect, sync.Map, paths and offline, HRESULT, LSP speed: they close at merge |
| Real bugs a user can hit | about 16 | 8 MiB goroutine stacks, `const d = x+1`, `sys/select.h`, header ending mid-declaration, `check` and `build` disagree, `make([]struct{}, MaxInt)`, builds flaky when several run in one cache, syscall small differences, `-release` traces without frames |
| Failing or never-run tests and CI | about 9 | clang++/`-asan` conformance campaigns, first GitHub CI run, full `make test-windows` |
| Performance (no wrong answer) | about 8 | timers, string maps, panic from a callee, `iter.Pull`, `-O0` default speed |
| Old review leftovers ("lows" bundles) | about 20 | 6.x, 7.x, 8.x, 10.x, 12.x, 13.x, 14.x, 15.x bundles: mutants, cosmetic output, rare shapes |
| Cosmetic, stale comments, docs | about 15 | wording, stale text, missing run tests |
| Needs the real Windows PC | about 4 | `Chdir` over 260 characters, `time.Local`, `Process.Release`, 14.x suites |

Release exit criteria (Phase 15: v1.0) are separate lines in the phase STATE files and need you:
24 h fuzzing, benchmarks published, CI green on three targets, installable release, docs site live,
the v1.0 promise and changelog. Phase 14 (Windows) exit lines need the real PC.

Working order of the manager: finish the running jobs, then the real bugs, then the failing tests,
then the bundles in the order of how likely a user hits them. Performance and cosmetic lines only
when the rest is done or a user asks.

---

## Historical front-end checklist

- [x] Turn the front-end spike (header packages, namespace rewrite + namespace probe, `FakeImportC`) into `internal/load` + `internal/check` — done 2026-10-19: internal/load and internal/check exist (Phase 01 done: steps 1.9, 1.10)
- [x] After M1: language server v0 (Phase 11 early track: diagnostics, hover, go-to-definition, completion, formatting; about 2 weeks, see STATE timeline) — done 2026-10-14


## Open work by area (audited 2026-10-22 against dev 81c0d4e1)

One line per claim: what is wrong, where, and the smallest repro (a tiny `.gp` program run with the
dev `gocpp`, `GOCPP_GATE_PHASE=15`). `(was Lnn)` is the line of the previous TODO.md
(`git show 81c0d4e1:plans/2026-10-13-gocpp/TODO.md`), where the long original text of a carried
item is. "carried" means: the claim was read against the code, nothing in `git log` has touched it,
but it was not run again. The per-claim evidence is in [AUDIT-2026-10-22.md](AUDIT-2026-10-22.md).
Items inside a running job (see "Now") are not repeated here.

### Runtime

- [ ] `&cvar` still dangles silently where the address is kept by something the compiler cannot see (ADR-139): a call that keeps its argument (`register(&x)`, `cb(&x)`, the yield of a range-over-func, `slices.Insert`/`maps.Copy` with C types), an address a C/C++ function keeps, a raw pointer member of a C struct that leaves, `&sl[1].a`. Over-retention (safe): `&n.next.v`, an unconverted C scalar result (`return lib.fill(&st)`).
- [ ] ADR-139 follow-ups: `gocpp vet` `cpplocal` now reports only what the compiler already boxes (plus `return &x`), so it is informational (the new interop files sit in `knownFindings` per whole file, a per-line expectation is tighter); `mayHoldPointers` on a C struct is decided only by emission (no unit pin); `escTop` and the `top == nil` case in `stores()` are dead defensive code.
- [ ] Goroutine stacks are 8 MiB (Go: up to 1 GB): deep recursion in a goroutine overflows at about 100,000 pointer-recursive frames; io/fs TestCVE202230630 (runs in a test goroutine) and fs.Glob of about 6000 separators in a goroutine. The main goroutine's stack grows to 1 GiB since job dev.stackdepth (RLIMIT_STACK raised at start, ADR-104); goroutines need ADR-104 option 2b.
- [ ] os/exec (when built) should start children with the original RLIMIT_STACK soft limit, as Go restores RLIMIT_NOFILE (origRlimitNofile): gocpp raises it to 1 GiB at start, so a child's glibc threads default to 1 GiB stacks of address space (D7).
- [ ] Types that are zero-size in Go but hold a byte in C++ (`[3]struct{}`, `struct{ z [0]int }`) still allocate in `make`: `make([][3]struct{}, math.MaxInt)` ends in `fatal error: runtime: out of memory` (Go: len MaxInt); needs an empty C++ representation or index-free element access (dev.makeslice review).
- [ ] `*(*[N]struct{})(s)` with N > 64 copies N bytes from the 64-byte `detail::zerobase` (to_array_ptr deref; `[N]struct{}(s)` is guarded since dev.makeslice); ASan has not reported it.
- [ ] reflect zero-size and huge slices: `reflect.MakeSlice(reflect.TypeOf([]struct{}{}), math.MaxInt, math.MaxInt)` and `Value.Grow(math.MaxInt)` say `reflect: array too large` (Go succeeds); `reflect.MakeSlice([]int, MaxInt/4, MaxInt/4)` says the same (Go: `runtime: allocation size out of range`) (runtime/src/reflect.cpp:234).
- [ ] Order: `m[s+"x"] = append(m[s+"x"], f())` reads the key before f (Go after): gocpp's `sameSafe` accepts a string `+`, gc's SameSafeExpr has no OADDSTR, so gc does not use mapAppend there (dev.makeslice review, also on dev).
- [ ] `runtime.Callers(1, pcs)` returns 1 (Go: 3), no `runtime.main`/`runtime.goexit` frames; `ReadMemStats` leaves `Mallocs`, `Frees` and the pause fields at 0 (`TotalAlloc`, `NumGC`, `NumForcedGC` are done, 08.synclows) (was L244).
- [ ] `time`: `NewTimer`+`Stop` costs 668-771 ns against Go's 129 ns (about 5x, pinned to one CPU, load about 37, 08.synclows); `time.Now` 51 against 38 ns (intrusive timer heap not done; needs timed_wait.* and the Windows timer code) (was L239).
- [ ] `RLock`/`RUnlock` through an embedded or field mutex build an aliasing counted pointer (`gp::ptr<RWMutex>(m, &m->mu)`) per call: an atomic refcount increment and decrement on the owner; about 2.5x on `Load` with 8 goroutines for any struct with an embedded mutex, and the speed depends on the object address mod 64 (08.synclows).
- [ ] `atomic.Pointer.Load` takes a spin lock: 46-58 ns on one thread and tens of microseconds at 64 threads; `sizeof(sync.Map)` is 144 bytes (40 before the Range list; 80 without the padding); `Range` of a Map above 4096 entries, or the first one after a change of the key set, is O(n) and allocates about 64 bytes per entry for the call (no worse than the old Map) (08.synclows).
- [ ] The full runtime test programs hang about 0.5% of the time at exit under ASan -O0 under heavy load (the `rt_test::threads_ended()` wait gives up after two minutes); also on dev (08.synclows review).
- [ ] The test helper `remove_all` guard (runtime/src/os/linux/testing_os.cpp `remove_target`) on Windows: 8.3 names, `\\localhost\C$` and relative names (checked as spelled, not joined to the cwd) are not resolved (documented in the helper); the `TMPDIR=/` carve-out is untested as non-root (13.hangs).
- [ ] `-release` traces: other goroutines print `?()` frames, no `created by`, and a panic trace shows `main.main()  ?:0` because ADR-136 made `-release` `-g0`; `GOTRACEBACK` is ignored, `runtime.Stack(buf, true)` shows one goroutine, no `runtime stack:` section (was L267).
- [ ] Goroutine panicking while `main` returns gives exit 2 where Go usually exits 0 (eager OS-thread start, by design); `defer f()` with a func variable costs 19.5 ns against 15.9 ns; a panic from a callee costs 17-22x Go (libgcc unwind floor) (was L244, L222, L278).
- [ ] 8.12 race detector limits: `Found N` can differ from Go's, two unrelated senders on one channel are ordered for TSan, a finished goroutine's race is found only if fewer than 64 others finished, `gocpp test -race` is refused (was L262).
- [ ] 8.13 lows: timer `Reset`/`Stop` race windows survive the tests; `test/chan` `doubleselect`, `sieve2`, `select2` do not run; stress tests need `// phase: 09`; big mode is not run under sanitizers (was L282).
- [ ] 8.11 lows: no `, N minutes` suffix; a goroutine blocked in a child wait or blocking read counts as running; a foreign C++ thread is invisible to the detector; a fatal error in a goroutine prints only that goroutine (was L267).
- [ ] `-race`: a mid-run `runtime.Stack(buf, true)` can hide a real race between the dumper and a goroutine that was in a short sleep when dumped (seen up to ~150 ms sleeps, not with 200 ms or more or a channel wait; both compilers); a panic dump is not affected; hiding the dump from the detector gives false races (tried; docs/guides/race.md).
- [ ] 9.x/8.11 reflect and dump lows: `crash` exits 2 (Go 134); scalar `DeepEqual(ValueOf(1), ValueOf(1))` rows; the fatal-error default prints one goroutine; Windows `tb-*` cases are Linux only; failing-barrier note checked end to end only for the panic path (`Stack(all)` unit-covered); mutants that survive: lazy-init (idx, memo), `fence_removed` (unobservable), `forget_adopted` without the wait, two narrow-window edge orderings, the header `ignore_reads` (no test kills it); goroutine create/finish cost not proved zero (g++ 0.94, clang++ 1.00 paired, `-release` code unchanged).
- [ ] 6.4/6.14 lows: surviving mutants (array variables not lazy, only the first lazy variable ensured); `lazy_boot` pinned by a golden only; a c-shared lib built `-asan` with a filled `std::vector` package var leaks across `dlclose`; `notePkgVar` still says "not constinit" for lazy variables (was L285).
- [ ] 6.10 lows: cgo-parity gaps of ADR-043 "Known gaps"; `internal/libgen.Collect` has no direct unit test; the pin is released by a later exported call (was L245).
- [ ] panic-perf lows: no unit test pins the panicking-goroutine count across the local-panic move; the lazy path is off under ASan; Windows on non-x86-64 keeps the old stack walk (was L278).
- [ ] append-cap lows: gc stack-buffer cases are not copied; `&s[i] == &s[j]` for distinct zero-size elements is false; `reflect.cpp` and `golayout.hpp` disagree on pointer-ful foreign types; huge lengths of empty structs no longer allocate since dev.makeslice; the Go-zero-size types that hold a byte in C++ are their own line above (was L269).
- [ ] 15.5 maps: string maps 84 B/entry at n=1000 (Go 48), range 3.5-4.2 ns/entry, string delete 66 ns (Go 57-60) (was L253).

### Stdlib

- [ ] Packages that are not available (`package ... is not available yet`): `os/exec`, `os/signal`, `os.StartProcess`, `net/*`, `crypto/*`, `text/template`, `html/template`, `encoding/csv`, `compress/*` (Phase 16 plans Go's own copies; `check` repro: import any of them) (was L303, L257, STATE Phase 14 14.3).
- [ ] `reflect.ValueOf(std.string("x"))`, `reflect.TypeOf(r.N)` of a C value, `reflect.ValueOf([]std.string{...})`: `internal error: not supported yet: passing C/C++ values to this function`; a Foreign type's descriptor from a struct field and from `TypeOf` may not intern equal (was L233).
- [ ] `testing`/`gocpp test`: `-race`, `-cover`, `-json`, `-fuzz`, `-shuffle` are refused (`... is not supported yet`); no vet before tests; no goroutine dump on a test timeout; `libLock/libNow/libSleep` natives and `util.gp` not yet replaced by `sync`/`time`/`flag`; `testing/iotest`, `testing/fstest`, `runtime.AddCleanup` missing; `gocpp test file.gp...` unsupported (was L251, L257).
- [ ] 7.10 lows: `iter.Pull` is a thread hand-off (create+stop 15-19 us, 100,000 live Pulls 18-21 s); string-keyed generic ops 2.8-5.7x Go; `maps.Clone` of string/struct maps 2.3-7.5x slower; `testing.AllocsPerRun` returns 0 (was L265).
- [ ] 9.4 lows: ported strconv/regexp/strings are slow at `-O0` (default); regexp 4-12x Go at -O2; `portstd -check` misses a stray hand-written `.gp` and `own` files; `tests/pkgs/regexp_internal` should be replaced by Go's originals run by `gocpp test`; `compress/bzip2` missing for TestRE2Exhaustive; testing-only `-test.*` flag divergences (was L248, L257, L276, L292).
- [ ] 9.11 lows: `io/fs.Glob` overflows a goroutine's 8 MiB stack at -O0 for ~6000 separators (main is fixed); `#Derive(JSON)` is an API shim with documented differences (ADR-102) (was L303).
- [ ] 14.3 lows (Windows, need the PC): `Chdir` over 260 chars, `time.Local` is UTC, `Process.Release` during `Wait`, `os.Stdout.Close()` then `Println` returns nil, `RemoveAll` error texts, no registry/`NewCallback` (was L264).

### Compiler (check, lower, cppgen, generics, interop)

- [ ] Range over an iterator with written C++ type arguments (`range Indexed[std.string](words)`): a Go-typed yield parameter (the `int` index) is a C/C++ value to the build and needs `int(i)`; `check` is silent and the inferred form `Indexed(words)` accepts it (dev.genericgaps review).
- [ ] `var s = std.to_string(5)` at package level: the build refuses it with the positioned limit message, `check` stays silent (about 120 check unit tests use such variables in unbuilt packages; ADR-141).
- [ ] Generic C++ messages (dev.genericcxx review nits): reasons spell `Box[C.std.string]` where the user wrote `std.string`; the package-variable suggestion falls back to parameter names for conversions/literals (`Pair[K, V]` instead of `Pair[int, std.string]`); `F(std.string("x"), 5)` with Box[U] should say that writing the type arguments fixes it.
- [ ] A pointer to a type argument that C++ deduces: `func Ptr[T any](p *T) *T` called as `Ptr(&s)` with `s std.string` is a C++ error (`no matching function ... (std::string*&)`: the parameter is a gocpp pointer, `&s` a raw C++ address) (found by dev.genericcxx; also on dev).
- [ ] An untyped constant for a parameter whose type is a written C++ type argument: `Id[std.string]("xyz")`, `Grow[std.string](words, "z")` are a C++ error (`could not convert ... from 'string' to 'std::string'`); `std.string("xyz")` works (found by dev.genericcxx; also on dev).
- [ ] A generic alias of an unnamed type with a C++ type argument (`type Vec[T any] = []T`, `var b Vec[std.string]`, `b = append(b, x)`) gives `internal error: not supported yet: append of C/C++ values` (found by dev.genericcxx; also on dev).
- [ ] `TestDeclsDeterministic` (internal/typedinterop/probe/decls_more_test.go:13) fails whenever `GOCPP_PROBE_CACHE` is set: the second run comes from the cache (`"cached":true`) and the comparison keeps that field. Repro: `GOCPP_PROBE_CACHE=$(mktemp -d) go test -count=1 -run '^TestDeclsDeterministic$' ./internal/typedinterop/probe` (found by the dev.probecache review).
- [ ] Big arguments that still copy through the C++ stack (ADR-086 §3, D24; dev.deferbig moved the rest of defer/go to the heap): an ordinary call with a foreign sibling (`show(big, foreign)` snapshots `auto gp_t0 = big;`); with a foreign sibling, the snapshots of a generic instance, of a variadic function's fixed parameters and of a large value receiver (`defer b.show(big, foreign)`); without a sibling, the implicit array conversion `arr(u)` for methods, interface methods, func values and literals, generic instances, the `go` forms and aware functions; a struct conversion `gp::convert<S>(v)` of a value over 128 KiB in every call, defer and go. Repro: an 8 MiB `[1<<20]int` argument in each shape ends in `fatal error: stack overflow`.
- [ ] `go func(a [N]int, _ Foreign){...}(big, foreign)` lowers the func literal twice: a dead `auto gp_t0 = gp::func<...>(...)` snapshot, then a second copy for `go_call` (found by the dev.deferbig review; also on dev).
- [ ] Named types over C types (`type Chars [1]C.char`, `type Count C.int`, named slices, maps, pointers and funcs over C types): `check` accepts them, the build stops with a positioned `internal error: not supported yet: named types declared over C/C++ types (T, a later step of Phase 04)` (an unused `type Fn func() C.int` used to build; every real use failed on dev).
- [ ] check/build agreement (dev.checkagree): `const k = lib.f(1)` with gates open is reported only by the build, and opaque `const g = lib.cvar + 1` only by an internal error in the build (also on dev); `const k = C.int(1)` with gates open gives `not supported yet: the identifier k`; `a == b` on arrays with C parts is gated at Phase 12; a constraint that embeds another constraint over `~[]C.int` (`type Emb interface{ CInts }`) still gives `internal error: expression has no type: x[1]` in check.
- [ ] Header hints and probes (dev.checkagree): the union probe does not mirror the headers of the package's own .c/.cpp files and of imported gocpp packages; the NotAName hint can fire on an undefined name that is no typo of anything in scope (`fmt.Println` without the import in a file that imports `./3d.h`); `gocpp fmt` and `gocpp doc` print the raw parser error for `select.X` (no hint); hint errors print after the file's other errors; test gaps: the loader's file-name guard for hint errors, same-offset errors, a comment before the dot, the typo-rule boundaries and the single-term constraint union clause.
- [ ] `x, y := ...` and else-if init shadows: `chain` in internal/lower/control.go closes the shadow blocks as separate `}` lines, each with a duplicate `#line` (cosmetic); named-type-over-C `type Handle geo.handle` is refused by build and accepted by check (was L80, L310).
- [ ] The headline note of `emit -annotate` says "captures by reference" for a closure whose captured variable is a boxed `[n]` capture (internal/lower/whynotes.go:115) (was L32).
- [ ] Stale comments: internal/lower/ops.go:562 "Phase 03 adds string(r)", internal/lower/expr.go:644 "from Phase 03 on", cmd/gocpp/build.go:1016 "when GOCPP_TYPED_INTEROP=1", runtime/include/gocpp/testing.hpp:5-8 lists `flag` among stubs (was L199, L299, L292).
- [ ] Interprocedural escape analysis for opaque locals (4.8): an inner-block C++ local whose address a Go function stores can dangle under defers (docs/spec/interop.md "not tracked") (was L216).
- [ ] Evaluation order D22 inliner corner: `h(gs[j], mi[a[i]])` checks `i` first in gc when `h` is not inlined; gocpp always gives the inlined order (was L196).
- [ ] go/types `type T T.x.y` panics `nil underlying`; gocpp reports `internal error: the type checker failed on this package` at the package clause: report upstream (was L272).
- [ ] 7.x lows carried: `iter.Pull` thread cost; generic-type compile time +33-39% (emit instance method bodies once in `pkg_desc.cpp`); `type WP weak.Pointer[T]` gate error; `reflect` of generic instances; `hasC(a)` dead gate; 400 instances take 40-70 s at -O0 (was L246, L258, L275).
- [ ] 10.x lows carried: wrap macro on a `must` function is rejected (`#W is a wrap macro: it can't be used on top of the must function g! (yet)`); `#func()` numbering of package-level closures; macro name hygiene (error by design); escaping lazy/`#body()` parameters stay boxed; `#M(f())` with a multi-value call; hover/completion nits; 10.11 `expand` overlap error R3-F1 and displaced comments; 10.12 `version.Phase` is still 5 (`internal/version/version.go:53`); `update.sh` drift of `gocpp_macro_name_test.go` (was L250, L263, L284, L290, L296).
- [ ] 12.x typed-interop lows carried: 12.3/12.4 (field offsets, parameter names, friends, `std::basic_string` dump over 32 MB, `T<(2 > 1)>`), 12.5 (silent by design cases, deleted-function text), 12.6 (method values `f := v.size` give a C++ error; keyed literals of arrays of structs), 12.7 (cache not on filelock/fsretry), 12.8 (constexpr calls, `-0.0`, enumerator `::NAME`), 12.9 (promoted operators, struct from another package), 12.10 (bindgen is a read-only view; only clang 21 tested), 12.11 (`gocpp test -interop=typed` without clang exits 1, docs say 3; up-to-date key ignores clang appearing), 12.12 (clang 17-20 untested) (was L259, L266, L277, L288, L294, L295, L297, L299, L310).
- [ ] 13.x vet/diagnostics lows carried: 13.1 (clang prints package types bare; caret on wrong occurrence; raw link errors only get a hint), 13.3 (cycle analyzer exemptions, narrowing covers explicit `C.T(x)` only), 13.3b (blank lazy param `_ lazy int` reported as reserved), 13.5 (reasons for `.gp` lines without C++) (was L235, L241, L247, L289).
- [ ] Phase 01/02 docs: README §2.1/§2.5 and ADR-002 say `gp_as[T]` (now `gp_cgoh.As[T]`); Phase 02 README file table lacks `ops.go`; `docs/CONTRIBUTING.md:61` does not list `tools/stubgen`; `docs/adr/README.md` does not describe dated amendments (was L67, L71, L75, L81).
- [ ] One rule for "is this C" in the gates: `embedsOpaque` (internal/check/gatewalk.go:754) sees only this package's C type names, so a struct embedding `lib.T` (with `type T C.Foo`) misses the promoted-member gate (live only below gate phase 12; `version.Phase` is 5) (was L72).
- [ ] A probe's compiler that exits non-zero with cut output hides the truncation (internal/interop/nsprobe.go:970 "Known limit"): own the pipe and retry for every exit status (was L82).
- [ ] HRESULT (ADR-137): `gocpp vet` has no dropped-HRESULT analyzer; the editor has no HResults hook (silent on 18 of 20 misuse shapes); `check` and `build` disagree for an `error` argument to a C numeric function and for the full generic form `Gen[hresult.NTSTATUS]{...}`.
- [ ] HRESULT (ADR-137): NTSTATUS is not a second row of the result table; an HRESULT through a function pointer or `std::function` stays raw; `%d` and `%#v` of the error show `{}`.
- [ ] Map literals next to a C key (ADR-137): an unused variable inside a skipped value may go unreported (`CheckExpr` reports the first error per skipped value); an elided literal of a C/C++ type is refused; an anonymous Go struct with a C field is refused (elided) and fails in the build (full form).
- [ ] Phase 04: `type D hresult.Surface` is refused at its declaration; named types over C types; a generic instantiated with a Go struct that has a C field says "C/C++ type argument" (`Box[W]`, also on dev).
- [ ] ADR-137 test gaps: `maxRefNodes` (1000) boundary not pinned; `nameAt` tie-break (first match) not pinned; the two-type-argument generic message (`Pair[int, hresult.NTSTATUS]`) not tested; the `maxIRDepth` comment figure (2000-2500 levels) is unmeasured (2000 builds, 2400 timed out at 200 s); `internal/check/hresults.go:175` copies and sorts through `Diagnostics()` where `Any` would do.

### Build, modules, packaging

- [ ] `gocpp build -n` prints 171 lines (the runtime object compiles) instead of the `libgocpprt.a` link line (was L255).
- [ ] 6.2/6.8/6.11 lows carried: `unitName` hash mutant, goldens through `EmitProgram`, hash collision of the unnamed-type hash, a private `gen/<hash>-<pid>-<n>` folder left by a failed build, `--sysroot dir`/`-Wl,-T,script`/`-Wl,-L,dir`/`-Xlinker -Map` not made absolute, -race/-release/-lto matrix on clang++ (only the quote-path `-release` case is covered), 6.8 second round (`build -n` lists runtime objects, mutant coverage notes) (was L227, L249, L255). Forced include through `-I` is done (06.lows).
- [ ] 15.14 lows carried: a local `replace` with `get m@latest` (L1); git-host selection downloads full archives (L4); a hand-edited `require lib v2.0.0` is not canonicalized and remaining L9 mutants (nested case clashes in `CreateZip`, `isFileProxy`, the `!rel.IsZero()` guard, sweep tests, retraction details); a pseudo-version vs release check reads the archive mtime (was L256). N1 (non-canonical `exclude`) and the exclude mutants of `Required`, `Previous`, `CanonicalVersion` are done (06.lows).
- [ ] 15.13 lows: `-X ...Version=` (empty) prints `gocpp  (phase ...)`; `version = "dev-${rev}"` renames the vendor derivation per commit; three packaging survivors; no aarch64 nix build, no `go install ...@latest` run; `scripts/flake-hash` was never run against a real `nix build` and kills only the nix process, not its children (was L280).
- [ ] 15.1 builds flaky when several run at once in one cache (4 of 324 builds failed at random under load, `stat .../gocpp-clang++-release: no such file`, 2026-10-19): NOT reproduced in about 12,700 builds on dev and on 0f55ecf5^ (dev.flakybuild); the window between the link and the move or start is closed for `gocpp build` and `gocpp run`; a compiler or linker ended from outside (SIGKILL, SIGTERM, SIGHUP, SIGINT, also a killed `cc1plus`/`collect2`/`ld`) is reported as such, not as a gocpp bug; the two `stat` failures are not explained by a random removal (the window is milliseconds), the original run is lost and who removed the folders is unknown; the committed benchmark baseline is still a loaded-machine baseline (was L274).
- [ ] Build robustness (dev.flakybuild): `gocpp test`, `gocpp debug` and library modes (`installLibrary`) have no retry when the program or library is gone after the link; `tools/gobench` under a shared /tmp relies on `-build-retries 1`; `tools/buildstress` has no mode that wipes only the program after the link; `gocpp clean -cache` during a build prints `internal error ... bug in gocpp` (documented as failing, but mislabeled); a killed `ld` or `as` under g++ `-flto` was not tried.
- [ ] 15.9 cleanups: turn the `capgrowth` scenario on by default and delete the `cap-growth-size-classes` entry of tools/difftest/known.txt (cap now equals real Go: repro with 33 string appends gives 71); the generator lacks slices/maps/cmp/iter/sort/context scenarios (was L271).
- [ ] 15.15 lows carried: site not published (see user-gated); docs link to `.html` stdlib pages that do not resolve on GitHub; every new `docs/guides/*.md` needs a line in docs/site/nav.txt (was L287).
- [ ] Build-flake lows: a work folder removed between the exe check and the exec is not retried; `g++ -flto` leaves `cc*.ltrans*.o` in TMPDIR; the compile_commands.json "not written by gocpp" notice can appear (was L279).
- [ ] Module loader: symlinks inside a cached module are not re-checked (extraction refuses them: low risk); reserved device names `con.h`, `aux.h` are not rejected as header paths; the LSP loads each package alone with its own module as main module (docs/spec/modules.md "Known limits: the language server") (was L121, L123, L214).
- [ ] C-only flags (`-std=gnu11`) have no place: `//gocpp:cflags` goes to C++ too (was L204).

### LSP

- [ ] LSP focused check, error order (ADR-115 amendment, case 3): an edit that removes the only earlier error of a package lets a go/types follow-on error ("invalid operand/type") of another file come back; the client gets it only at the settle run, 100 ms later.
- [ ] LSP focused check, speed only: a package with any weight-1 instantiation (`List[Pair[K, V]]` in a generic map) declines every focused check after the next full check; a precise version would rebuild mono.go's graph (base edges minus the old body plus the new body) and decline only on a new cycle.
- [ ] docs/benchmarks/lsp.md is from 2026-10-18 (before 11.lspgap5): regenerate it with one `make lsp-bench` on a quiet machine (load under ~16); the 2026-10-23 figures are in ADR-142.
- [ ] 11.11: references/rename on never-opened packages have no budget row (cold 0.6-1.1 s); the 11.8 numbers in docs/guides/editors.md were taken under load (was L218).
- [ ] 14.5 lows carried: no server-side file watcher (client `didChangeWatchedFiles` only), dead UNC share stalls the first request 5 s (was L293).
- [ ] Unexplained, seen once: `TestRandomEdits` seed 77 under `ulimit -n` 28-48 kept `package .../mid is not in module` (was L231).
- [ ] Completion nits: a stray blank line before `AdditionalTextEdits` (internal/lsp/server/completion.go:158); `TestFixDangling` builds `cache.ParsedFile` without `Mode` (was L182).
- [ ] Re-measure the formatting and organize-imports benchmarks on a quiet machine in 11.11's harness (was L183).

### Tooling (debugger, vet, test runner, fuzz, tests infrastructure)

- [ ] lldb shows a variable captured by reference as a C++ reference (the dev.debugtests reference printer is gdb only); a captured C enum or C pointer still prints as `&GREEN @0x...` under gdb.
- [ ] gdb printers: `info locals` and raw gdb `print` still show raw closure structs on clang++ and `n = &6` for a g++ box; exotic Go types (a struct or interface with members, an array length that is a name) print as `func(...)`; a generic type argument that is a type parameter prints the parameter name inside a generic function.
- [ ] 13.2 lows: the stop line, `frame`, `up` and `info locals` still print C++ names and temporaries; a boxed local is unwrapped by reading the `.gp` declaration line; `gp print` has no map indexing or calls; `$gp_blocked()` counts only channel waits; the lldb script got no robustness pass (was L240).
- [ ] Test infrastructure duplication: the compiler-matrix test code exists in 18+ files (rg `GOCPP_REQUIRE_CXX`); two Wine layers and two ssh runners (internal/wine, internal/winssh, runtime/windows_test.go, runtime/winremote_test.go) are not unified (the wall-clock parts of this line were fixed by dev.walltests) (was L77, L187, L189, L190, L205, L219, L252).
- [ ] `clean-tmp` (scripts/clean-tmp): a mount that appears after its check and bind mounts are invisible to `rm --one-file-system`; the "not yours" guard has no test (needs a second user).
- [ ] 15.8 fuzz lows carried: 20-minute campaigns per target not run; `FuzzFormat` gofmt quirk on a comment before the package clause; FuzzGen samples 1 in 40 compiles; killed workers leak `/tmp/gocpp-fuzz-gen-*` (was L272).
- [ ] 15.10 conformance: baseline 1941 of 2131 (91.1%, g++ default only: meets the 90% criterion numerically) needs clang++ and `-asan` campaigns and a re-baseline; `[bug]` clusters (multi-value call as arguments of `append`/`copy`/`delete`/`complex`, `[]MyByte` to string, `Sprintf(f())`), ~20 errorcheck wording differences (was L260).
- [ ] 15.11 first full nightly never run (repo is on GitHub now, workflows exist); 15.12 workflows never run on GitHub (was L286, L41).
- [ ] 14.7/14.8 Windows suites: full `make test-windows` never run; 14.2/14.4 lows need the Windows PC (was L283, L270, L300, L301).

### Docs and tests

- [ ] `docs/conformance/go-test.md` tables are stale: regenerate them in the user's broad run (`go run ./tools/gotest run -out results.json`, then `gotest report -md docs/conformance/go-test.md results.json` and `gotest gate -update -results results.json`); the dated note in it gives 2132 applicable, 1955 in passing.txt (at least 91.7 %).

### Needs your decision (user-gated)

- [ ] Final manual checks only you run: Phase 04 suite and header zoo green in typed mode; VS Code checklist (incl. `!` and macros); `make ci`; `make test-windows` (45-90 min); first GitHub run of ci.yml/sanitizers-nightly.yml; the 1,000,000-program differential campaign (~960 CPU-hours); 24 h fuzzing; the Phase 14 exit criteria; v0.1 release approval (9.13) and the v1.0 promise (15.16).
- [ ] 10.7 error return traces: waiting for your four answers (creation sites, `-release` default, default printing, what a cut does) (phase-10 STATE).
- [ ] testify (Phase 09 §2.7): a drop-in with testify's API first, or wait for the real module? Fuzzing (`testing.F`) and `-cover` into the plan proper? (was L119)
- [ ] C/C++ modules: shared recipe index? opt-in CMake fallback? (was L125). Operator overloading or destructors for gocpp types after v0.1 (was L127). `!` for comma-ok (was L132). `err := #body()` for wrap macros (was L133).
- [ ] Should the gc shift-count bug be filed upstream? Draft: [upstream/gc-shift-count-issue.md](upstream/gc-shift-count-issue.md) (was L186).
- [ ] C-only flags: `//gocpp:conlyflags` or `GOCPP_CONLYFLAGS`, only if users need it (was L204).
- [ ] Docs site hosting: where to publish `site/` (15.15) and a workflow for it (was L287).
- [ ] ADRs 103-110 are `proposed`: accept or drop; ADR-104 only recommends keeping threads (was L298).
- [ ] 15.17/Q9: the repository IS on GitHub (`origin` = github.com/huuhait/GoToCPP); the Q9 text below says "not yet": confirm it can be updated.
- [ ] Remote branch `origin/job/04-4.7` is still on origin (the stray `origin/job/12-12.9` is gone).
- [ ] The license question before v1.0 (runtime exception, like LLVM's) (was L150).

## Closed by the 2026-10-22 audit (found fixed on dev; commit and evidence in AUDIT-2026-10-22.md)

- [x] 9.6: `Close` wakes a blocked `Read`/`Write` on a pipe, FIFO or tty (the line carried a wrong `PRIORITY`) (done: 423d71aa, ADR-067 amended; tests os_poll_close_wakes_read/write, os_poll_close_fifo_tty)
- [x] 9.6: `GOOS=windows gocpp check` of a program that imports `os` (done: 472bc359, 14.3; `gocpp check -os windows` exits 0, `TestCheckTarget/windows/*` passes). `darwin` is not a target (Q8, ADR-021) and was removed.
- [x] 6.13: `TestCheckTarget/windows/*` failed since 9.6 (done: 472bc359)
- [x] "x (C/C++ value) is not an interface" for `x.(T)` on a C value (done: ab24f06b, 4.18)
- [x] `for i, x = range arr` with an existing C-typed `x`: internal error (fixed: repro prints the elements; no single commit found)
- [x] 4.8/5.11: `go fmt.Fprintln(os.Stdout, ...)`, fmt function values `f := fmt.Sprintf`, spread operands changed by a String method (done: 31514e86, 9.7a native-fmt audit)
- [x] 6.4: `gp::lazy<T>` and lazy c-archive/c-shared init (done: adf26aae, ADR-093); Phase 06 is done
- [x] 8: TSan suite green: `GOCPP_RACE_COMPILERS=all go test -run '^TestRace$' ./tests` gives 247 PASS, 0 FAIL (2026-10-22); Phase 08 is done
- [x] 8.11: Goexit from `main` with a goroutine blocked forever hangs: it reports the deadlock like Go (repro)
- [x] 7.3: `%T` of a local type argument prints `main.Stack[main.point·1]` as Go does; `strings.Join` in generic code works; `MethodByName` exists (stdlib/reflect/type.gp:534); `iter.Seq[int]`, a generic function returning an anonymous struct with type-parameter fields, a pointer method call on `*H` in a generic function where `H` has `std.vector` fields, a type declared in a package-level `var` initializer closure: all work (repros g2, g3, g6, g8)
- [x] 7.8: `clang++ link failure ~_Sp_counted_ptr_inplace` for a `recover()`-aware literal that allocates: not reproducible with clang++ 21 at -O0 and `-release`
- [x] 7.10: `examples/generics` keeps its `// pending:` line; 9.4c `examples/grep-lite` pending; 10.10/10.13 `examples/macros` and `derive_json` pending: none is pending any more (`TestPendingFiles` want list is empty, tests/format_test.go:1117; flag 09-flag, `#Derive(JSON)` 9.11 ADR-102)
- [x] 9.9: `runtime/src/testing_regex.cpp` removed and `testdeps.MatchString` calls `regexp` (done: c5f79b9e, 9.4c)
- [x] 9.1: portstd downloads have an `http.Client` timeout (tools/portstd/source.go:261,413)
- [x] 12.1: struct-tag literal `lits.struct_pair_s{1,5}`, comma-ok on a `std::map` index (clear error with a hint), `Result` without a written type (done: 256caa04, 12.3/12.4; ab24f06b)
- [x] 12.13: C/C++ editor budget row (< 2 s, background) and slow-clang test (done: 3aa700f1, ADR-126)
- [x] 12.11: the default flip to auto (done: 1c342d97, ADR-131)
- [x] 15.3: escape analysis for copies out of slice/range values (done: b3a1219b, ADR-114 section 3)
- [x] 15.9: `append` capacity follows gc 1.25 (done: 8a579fb6); `FuzzParse` backslash-CR (done: 4b085e9a)
- [x] 15.14 L3: Windows has a cross-process project lock (done: ab85402c, 5a86d319: internal/modfetch/lock_windows.go `LockFileEx`)
- [x] 6.11: write `compile_commands.json` after every build or opt-in? Decided: debug builds write it, release only with `-compdb` (done: 0cf8ec4e, ADR-044)
- [x] 14.4: typed probes with the Windows target (done: 94c4a6d9, 14.6)
- [x] 12.13c2 header locations in hover and go-to-definition (done: b392dcfe)
- [x] 8.11: the stray files `a`, `a.cpp`, `xdg11/` at the repo root are gone (checked)
- [x] `TestRuntimeObjectsHaveNoStaticInit` (done: 4966364a); `TestEditorFeaturesSweep` (semanticTokens, ac4cb183); `TestInteropUpToDate`; `TestProgramParallel` (not reproducible, 1 run); `TestStdlibStubs`/`TestGateNotesCurrent` (pass/skip on dev)
- [x] 5.4: an interface with a method named like a type compiles and runs (repro prints 3)
- [x] 15.5 arm64 NEON group: not wanted, decided in runtime/include/gocpp/swiss.hpp:150 (SWAR as fast)
- [x] 12.9 stray remote branch `origin/job/12-12.9`: deleted
- [x] 10.11 R3-F2 `expand` prints a half-processed file: the sample is now a parse error and exits 1; ADR number 092 claimed twice: only one `092` exists
- [x] 4.16: same-stem `.h`/`.cpp` automatic C++ guess: decided by Q17 (stays a directive)
- [x] 11.11: the LSP reference project is chosen (three reference projects, b3dc18f0)
- [x] 7.3: `unsafe.Sizeof` of a type parameter's value uses gocpp sizes: documented divergence D15 (docs/spec/language.md:440)
- [x] 7.4-7.9: the C++ concept accepts a pointer method with a different signature / `Get_` for `Get()`: unreachable, go/types rejects both first
- [x] 9.4a: a release (-O2) build mode is needed for speed comparisons: `gocpp build -release` exists (e39ca144); `Fprintln` of 1M lines takes 0.47 s against Go's 0.44 s at `-release` (3.45 s at the default `-O0 -g`)

## Closed earlier (kept for the record)

- [x] FakeImportC false errors (from job 1.11): dropped by `internal/check/falseerrors.go` when the operand is a C/C++ value (job 1.15; Phase 01 README §2.8 "As built"); covered by TestFalseErrors and the corpus in `internal/check/testdata/corpus`
- [x] `docs/spec/cpp-mapping.md` §package initialization (line ~71): `run_main` gets `&gp_init_all` as soon as a package has variables or `init` functions (not only from Phase 06), and the `#line` example shows a directive only where the line is not previous + 1 (from job 2.1) — done 2026-10-19: docs/spec/cpp-mapping.md now says `run_main` gets `&gp_init_all` when a package has variables or init functions; the `#line` example is declared simplified (checked 2026-10-19)
- [x] The gates reject a call through a parenthesized function name, `(g)(1)` and `(fmt.Println)(1)`, as "function values … (Phase 03)" at Phase 02: unparen the callee before the function-value check (from job 2.3); give it to the next job that touches `internal/check/gatewalk.go` — done 2026-10-19: `gatewalk.go` unparens the callee (`ast.Unparen(call.Fun)`, step 1.12)
- [x] `docs/spec/cpp-mapping.md:80` maps `uintptr` to `uintptr_t`; the emitter writes `uint64_t` (job 2.1 decision): fix the doc (from the 2.3 review) — moot: `internal/lower/types.go` maps `uintptr` to `uintptr_t` and the doc is right as written (checked 2026-10-19)
- [x] `gocpp fmt` (an upstream go/printer bug, also in gofmt with Go generics) drops the parentheses in `range (std.vector[int]{7, 8})`, and the result doesn't parse ("expected 1 expression"): keep the parentheses when the range expression is a composite literal (from job 1.15) — fixed 2026-10-14 (internal/format/parens.go)
- [x] For later Phase 08 steps (from the 8.1/8.15 review): 8.11 (deadlock detection) needs a count of blocked goroutines and a registry of the live goroutines: the goroutine record (`gp::detail::goroutine`, `runtime/include/gocpp/goroutine.hpp`) has no "blocked" flag and `gp::waiter` has no park hook yet, so add both there. 8.10 (`Goexit`) must catch Phase 05's `goexit_signal` in `goroutine_main` (`runtime/src/goroutine.cpp`) so the goroutine ends and counts down. 8.2 (`go` codegen): `gp::detail::go_start` takes a `std::unique_ptr<task>` and a stack size; `gp::go(f)` is the template wrapper. Step 8.2 also adds `goroutine.hpp`/`waiter.hpp` to `rt.hpp` and the new standard headers (`<condition_variable>`, `<mutex>`, `<chrono>`) to `internal/lower/cxx_macros.txt` (`go test ./internal/cppgen -run TestMacroList -update`). — stale: 8.2, 8.10 and 8.11 are merged (`is_blocked` flag, `goexit_signal` catch in runtime/src/goroutine.cpp; checked 2026-10-19)
- [x] VS Code TextMate grammar for `.gp`, including `name!(`, `func name!(`, `#Name`, `#body()` and `macro` (Phase 11 step 11.10, job 11.10a, done 2026-10-13)
- [x] Header zoo libraries chosen: C/C++ standard headers + SDL2, raylib, sqlite3, zlib (Phase 04 step 4.18)
- [x] Completion nits from the 11.6 review — done in job 11.6b (2026-10-14)
- [x] TestProbeCache (internal/interop/nsprobe_test.go:672-680) may be flaky under heavy load: the "vec.h just changed" / "is old now" steps assume a probe run ends within nscache.go's 2 s racyWindow; set the mtimes explicitly instead of relying on wall time (seen once in a 3.7b `make ci` at load 28; not reproduced in 8 runs) — fixed 2026-10-14 (job fix-flaky)
- [x] TestHoverOpaqueDisk (internal/lsp/server/opaque_test.go:153, "the second server read no summary from disk") failed once in a loaded `make ci` (job 3.8 review); passed 5 times alone. Likely the disk-cache write racing the second server: make the test wait for the first server's writes (Pipeline.Close) before starting the second (Phase 11) — fixed 2026-10-14 (job fix-flaky)
- [x] Flaky under load (found 2026-10-17 by the object-cache coder, not caused by it): `TestCHeaderOffHotPath` in internal/lsp/cache fails in `make ci` at machine load 30-50 (parallel jobs on the 128-core server); it fails the same way on the commit before the object cache with `go test -count=200 -run TestCHeaderOffHotPath ./internal/lsp/cache`, and passes alone and in small package sets. Look at what it times (a hot-path budget?) and make it load-proof the way internal/interop's TestProbeCache was (job fix-flaky). — fixed by 6bac76a2 (phase 11 steps 11.8/11.9): the test no longer depends on timing (`chAnswersChanged`)
- [x] `runtimeKey` (internal/build/runtime.go) keys the runtime's cached object files on the text of an `@file` response-file argument in the compiler's args or GOCPP_CXXFLAGS, not on its content, so editing the file leaves old runtime objects in use while the program compiles with the new flags (pre-existing; found by the object-cache review 2026-10-17, low severity). Fix: hash the content of any `@file` argument (build.readsDriverFile finds them) or skip the runtime object cache for those flags. — fixed with e39ca144 (6.11/6.12): `runtimeKey` now salts with `PluginSalt`, which hashes the content of response files and `-include`/`-imacros` files
- [x] The runtime package's tests take ~5 min alone at load 21 and passed go test's 10 min default inside a loaded `make ci` (after 3.15b's fmt diff cases); `make test` now passes -timeout 30m. Make runtime/cxx_test.go faster: compile the C++ tests in parallel subtests (t.Parallel per compiler and mode) and share one runtime object build per mode — fixed by 4940a6ed (runtime tests compile in parallel, one runtime object build per mode) and `-timeout 45m` in the Makefile
- [x] tests/run/defer_repanicked.gp (1,001 levels of deferred panics, from 3.13) makes real Go itself crash now and then in runtime.printpanics (2/40 runs at GOAMD64=v1): TestRunVsGo (opt-in, GOCPP_RUN_VS_GO=1) is flaky on it. Lower the depth to where Go is stable (e.g. 200) or skip it in TestRunVsGo with a note; possibly an upstream Go bug worth a look — fixed in job 3.18 (depth 200)
- [x] docs/spec/language.md D25 wording (nit from the last 3.12 review): the slow-key list ("a bool, int8, int16, uint8 or uint16 key, a float, most arrays and structs") leaves out complex keys and maps whose value is over 128 bytes (every key is slow then), and only keys gc keeps in registers are copied (arrays of at most one element, structs of at most 4 fields: a `[3]int32` key prints the new value in Go and gocpp). Suggested: "a key that the map call takes by address (see orderKey), of a type that fits in registers". Give it to the next job that touches language.md
- [x] Back-end fallback messages still name the finished Phase 03: left in internal/lower/ops.go:400 and :465, expr.go:358 and expr.go's node-kind table (~577-582) (fixups-1 did the rest, the Never gate test and the examplePhase comment; give these to job 4.4's next round) — done except two stale comments (internal/lower/ops.go:559 "Phase 03 adds string(r)" and expr.go:581 "from Phase 03 on"; checked 2026-10-19)
- [x] A test run writes the real `~/.cache/gocpp/rt` (the runtime extraction of `gocpp build`) when XDG_CACHE_HOME isn't overridden (found in the 4.1 review; older than 4.1). Tests must never write the user's cache: find the tests (cmd/gocpp, tests/, internal/build) and give them a temporary cache folder, with a check like the LSP's — fixed in job 4.13 (the harness sets XDG_CACHE_HOME)
- [x] `gocpp build -o /dev/null` fails ("open /dev/.null.tmp-…: permission denied"): internal/build MoveFile writes a temp file next to the target; `go build -o /dev/null` works. Write directly when the target isn't a regular file (found in job 4.13)
- [x] The interop error-test harness (tests/harness_test.go interopErrorsTest, othersLineRx) accepts the "(and N more errors in gocpp's code: …)" line silently, so a seed that also triggers a gocpp bug still passes: treat that line as unexpected unless the test opts in (4.17 review nit); also reflow the "Trimming" bullet in docs/spec/interop.md — done (ab24f06b): the line is accepted only with `// others-ok: N`)
- [x] Narrow the rename method family to gopls's use-tied unification (Phase 11, step 11.8 review): gocpp renames every method that has the name and shape of a method of an interface of the module (internal/lsp/server/refs.go `relatedMethods`), gopls only the methods that a use ties to the interface (a value passed, assigned, returned or asserted where the interface is needed). Renaming `Close` or `String` of an interface renames unrelated types of the module; rename now tells the user which declarations it added (family.go). Anonymous interface literals and dynamic assertions are not in the family in either. — done: the default family is use-tied (`relatedByTies`, 69d29fc8 and rounds 3-5, 63084086); `relatedMethods` is kept only for the outside-interface refusal and `-wide-family`; the residual wider-than-gopls cases are in the last line (checked 2026-10-19)
- [x] `FuzzParse` in internal/load/gocppmod fails on the input `"\\\r//"` (a backslash, a CR and `//`): `Format` is not idempotent (found 2026-10-17 by the 6.1d coder while fuzzing; unrelated to recipes). The corpus file was deleted, not committed: re-find it with `go test -fuzz FuzzParse ./internal/load/gocppmod` and fix Format/Parse (the CR handling), then keep the input as a seed. — fixed by 4b085e9a (15.8): `ParseSyntax` rejects a backslash before CR; regression test `TestBackslashBeforeCR`
- [x] (fixed by 5.9/5.6, 2026-10-18) Unhashable map keys (found 2026-10-18 by the 5.4 reviewer): the nil/empty-map text rule (read/delete in a nil map panics with Go's 'hash of unhashable type: []int', no 'runtime error: ' prefix; an empty map the same; other uses 'runtime error: hash of unhashable type []int') covers only keys whose STATIC type is an interface. A struct or array key that CONTAINS an interface holding a []int does not panic on read/delete in a nil map and gives the 'runtime error: hash...' text in an empty one. `check_empty_key`/`iface::gp_key_check` must recurse into the key (repro: review-54/r2B/u1.go cases structany, arrany, SA).
- [x] (fixed by 5.9/5.6, 2026-10-18; `&p` of a *C.T variable vs `**T` and const char* arrays remain documented limits) C pointers through `any` (found by the 5.4 reviewer, both fail on dev too): a `const T*` result becomes `cptr<const T>` so `.(*C.char)` is false; a `T**` is boxed as `cptr<Pt*>` but asserted as raw `Pt**`. Normalise const pointees and nested pointers on both sides, or document the limits (repros review-54/r2B/t/cp1.gp, cp2.gp).
- [x] 5.4 bug (found by the 5.8 reviewers, reproduces on dev): an interface with a method named like a type it uses (`type I interface{ Item() Item }`) does not compile ('internal error: generated C++ did not compile ... changes meaning of Item'): the interface struct in internal/lower/ifaces.go writes the type unqualified; qualify it (`::gpkg_main::Item`) as Fields and Params already do. Add a run test (the late-marking embedding test avoids it). — fixed in code (the repro compiles and prints 3, probably by per-package codegen 6.2/6.3))
- [x] `-O0 -g` is the only build mode (internal/build/flags.go `debugFlags`): a release/-O2 mode is needed before any Go speed comparison (Phase 15); ported utf8/bits are 8-12x slower than Go at -O0. — done: `gocpp build -release` (-O2 -flto) and `-debug` exist (e39ca144, 6.11/6.12); the default stays -O0 by design

## Decisions (answered 2026-10-13)


- [x] **Q1 Memory:** reference counting (ARC), no garbage collector. Cycles are broken with `weak.Pointer` (Phase 03).
- [x] **Q2 `int`:** 64-bit, like Go. `C.int` is there for C APIs.
- [x] **Q3 C/C++ imports:** like Go packages. A header is a package named after its file (`"<raylib.h>"` → `raylib.InitWindow`, `"./physics.h"` → `physics.step`), and C++ namespaces are packages that need no import (`std.vector[int]`). `C` keeps only C's built-in types and helpers (`C.int`, `C.CString`). No inline preamble: a helper goes into a header next to the `.gp` file. (Phase 01 §2.3, Phase 04)
- [x] **Q4 Concurrency:** `go` starts a real OS thread. No goroutine scheduler, now or later. (Phase 08)
- [x] **Q5 Runtime checks:** on, like Go: a bad index, nil pointer or division by zero stops with Go's message and the `.gp` line; `-nochecks` turns off the bounds and nil checks for speed (division and shift checks stay, because C++ has undefined behavior without them). (Phase 02 §2.3, Phase 03 §2.9)
- [x] **Q6 Module path:** `github.com/huuhait/gocpp`.
- [x] **Q7 License:** BSD-3-Clause for now. Look at the runtime license again before v1.0 (see backlog).
- [x] **Q8 Platforms:** Linux and Windows, Linux first; Windows test builds from Phase 06, the full port in Phase 14, both before v1.0. macOS is not a target. (ADR-021)
- [x] **Q9 GitHub:** not yet (2026-10-13). Since then `origin` is github.com/huuhait/GoToCPP and `.github/workflows/` has ci.yml, release.yml and sanitizers-nightly.yml (15.12, never run there); `make ci` stays the local CI. Confirm this reading (open: see "Needs your decision").
- [x] **Q10 `!` (must):** only where you write it. On a function (`func loadConfig!(path string) Config`): every error inside it that you don't take yourself is returned; callers see `(Config, error)`. On one call (`cfg := loadConfig!(path)`) in any function. Without `!`, plain Go. (Phase 10 §2.1)
- [x] **Q11 `!` in `main`:** print `<program>: <error>` and exit 1 (for `func main!()` and for `F!()` in `main`).
- [x] **Q12 Macro style:** `macro Name(args) { ... }`, used as `#Name(...)` inside code or on top of a func, and `#Derive(...)` on a type.
- [x] **Q14 C/C++ libraries from the internet (2026-10-13):** modules like Go's; headers imported by full path (`"github.com/ocornut/imgui/imgui.h"`); `<...>` only for system headers; built from a small `cxx` recipe in `gocpp.mod`, no CMake. (Phase 06 §2.11)
- [x] **Q15 Toolchain like Go's (2026-10-15):** gocpp downloads its pinned llvm-mingw (UCRT, SHA-256 in the binary) when no Windows toolchain is installed, the way `GOTOOLCHAIN` fetches Go; `GOCPP_TOOLCHAIN=auto|local|pinned`. Windows needs only `gocpp.exe`. (Phase 06 §2.10, Phase 14 §2.2, Phase 15 §2.4)
- [x] **Q13 Macros on top of a func:** both styles. Start style (runs at the start; a returned `func()` runs at the end) and wrap style (`#body()`: the macro decides how the function's code runs, e.g. `#VM()`, `#Recover()`, `#InThread()`, `#Once()`). (Phase 10 §2.2)

- [x] **Q16 C/C++ libraries from the internet are built NOW (2026-10-17):** module downloads (tagged archive or commit, hash in `gocpp.sum`, read-only module cache, `gocpp mod vendor`), reading and building the `cxx` recipes, and module-header imports come before Phase 15 as jobs 6.1b (import rule), 6.1c (downloads) and 6.1d (recipes). Phase 06 §2.11 "download comes later" is superseded.
- [x] **Q17 Local headers import like Go packages (2026-10-17):** a quoted import whose first path element has a dot is a module header, and the module can be the project's OWN module: with `module github.com/huuhait/project`, `import "github.com/huuhait/project/native/x.h"` is `native/x.h` in the module root; required modules use the same rule (longest module path prefix wins); `"./x.h"` and `"native/x.h"` stay relative to the package folder. (This also settles the shape of the `.h` + same-stem `.cpp` question: the automatic-C++ guess is still not made; it stays a directive.)

## Questions answered

- [x] Is `C` available in every file? **Decided: yes**, for C's built-in types and helpers; the front-end adds `import "C"` only to files that use it (D2 in Phase 00).
- [x] Default optimization level for `gocpp build`: **`-O2 -g`** (Phase 06 §2.9); re-checked with the Phase 15 benchmarks.
- [x] Namespaces for generated C++: **one flat `gpkg_<escaped import path>` per package** (`gpkg_main`, `gpkg_fmt`), and C/C++ names fully qualified (`::time`) (ADR-010, ADR-019; changed by the audit from nested `gpkg::…`).
- [x] Default interop mode: auto (typed when clang++ is found, else opaque) since 2026-10-21, ADR-131 (supersedes "opaque until typed passes the whole Phase 04 suite", Phase 12 §2.8). Open: header zoo and Phase 04 suite in typed mode are user-only checks.

## Ideas / backlog (not scheduled)

- [ ] A downloaded toolchain for Linux targets too (a pinned clang + lld + libc++, or `zig c++`), so a Linux machine without `g++`/`clang++` also needs only gocpp (follows Q15)
- [ ] Web playground: the transpiler compiled to WebAssembly, showing `.gp` → C++ live
- [ ] `gocpp test -cover` via clang source-based coverage mapped through `#line`
- [ ] Fuzzing in `testing` (`testing.F`)
- [ ] `net/http` package wrapping a C++ HTTP library (Phase 16 plans Go's own `net/http` on its GC engine instead; if gp's standard library gains a `net/http` later, programs move from Go's copy to gp's, Phase 16 §2.3)
- [ ] Phase 16, dual engine: Go modules from gp code, on a second engine with a garbage collector (bdwgc) while gp stays on reference counting. Planned in full on 2026-10-21 and reviewed on 2026-10-22 (your answers are in phase-16-dual-engine/STATE.md); it replaces the earlier idea of an optional tracing GC, since gp itself stays on reference counting; it starts when you decide
- [ ] WebAssembly target through emscripten
- [ ] Self-hosting: compile the gocpp compiler with gocpp
- [ ] Procedural macros: user code generators run at compile time (AST in, AST out), after v1.0
- [ ] `#!` shebang support so `.gp` files can run as scripts (`#!/usr/bin/env -S gocpp run`)
- [~] GitHub Actions running `make ci`: the workflows exist (15.12, a8125cf0) but have never run on GitHub (was: "once the repo is on GitHub, Q9")
- [ ] MSVC ABI on Windows (`clang-cl`) for C++ libraries built with Visual Studio; Windows arm64
- [ ] Inline C++ preamble (cgo style) as a shortcut for tiny helpers
- [ ] Before v1.0: decide whether the runtime gets a license exception (like LLVM's), so programs built with gocpp owe no notice

## Done

- [x] 2026-10-13: Plan written in `plans/2026-10-13-gocpp/` (README, STATE, TODO, phases) and committed with the spikes
- [x] 2026-10-13: Added to the plan: `F!()` must-calls + macros (Phase 10), fast language server (Phase 11); renumbered later phases
- [x] 2026-10-13: Spike: `F!()` via hidden generic helpers on stock `go/types` (types inferred, bad uses rejected)
- [x] 2026-10-13: Spike: macros = functions + hidden call site + lazy args (generic `#Dbg` works; lazy arg evaluated 0 times on success)
- [x] 2026-10-13: Spike: `macro` declarations + on-top macros (`#Trace` enter/exit even on panic, `#Locked` race-clean with 100 concurrent calls)
- [x] 2026-10-13: Measured `go/types` re-check speed with cached deps (`net/http` 28.5k lines ≈ 29 ms)
- [x] 2026-10-13: Spike: stock `go/parser` + `go/types` (`FakeImportC`) type-check interop-heavy `.gp` with 0 false errors
- [x] 2026-10-13: Spike: clang probes for C++ types (`decltype`) and macro values (`gp_val<V>`)
- [x] 2026-10-13: Checked that stock `go/printer` prints gocpp syntax, plus Go's `fmt`/`println` formats and the stdlib porting blockers
- [x] 2026-10-13: Compiled and ran the plan's key C++ idioms on GCC 15 and Clang 21 (identical results)
- [x] 2026-10-13: Spike programs kept in `spikes/2026-10-13/` (with a re-run README)
- [x] 2026-10-13: Full audit of all plans by 5 independent reviewers; verified findings fixed (see STATE log)
- [x] 2026-10-13: Your decisions Q1–Q13 recorded and applied to every phase; new Phase 14 (Windows), hardening is now Phase 15
- [x] 2026-10-13: Spikes for the decisions: header packages + namespace probe (clang++ and g++), must functions (two type checks, 3 runs), wrap macros (`#VM`, `#Recover`, `#InThread`, `#Once`, race-clean)
- [x] 2026-10-13: Phase 00 steps 0.2–0.7, 0.9, 0.10: ADR-001 … ADR-021, the spec docs v0, the repo skeleton, the Nix dev shell, the 19 target programs, the contributing guide (one commit per step on branch `dev`)
- [x] 2026-10-13: Phase 00 steps 0.8 + 0.11: the test harness (six runners, phase gating, both compilers) and `make ci`
- [x] 2026-10-13: Phase 00 job 0.6b (stale docs, `xvfb-run`) and the Phase 00 exit check: **Phase 00 done**
- [x] 2026-10-13: Phase 01 step 1.13: `internal/diag` prints errors like `go build`
- [x] 2026-10-13: Phase 01 step 1.1: `go/scanner` + `go/parser` forked into `internal/syntax` (`FORK.md`, `update.sh`)
- [x] 2026-10-13: Phase 01 step 1.2: parser patch 1 (`std.map`, `node.type()`)
- [x] 2026-10-13: Phase 01 step 1.5: the round-trip test over the Go standard library (6240 files)
- [x] 2026-10-13: Phase 01 step 1.6: the import classifier (headers and namespaces as packages, the reserved `C`)
- [x] 2026-10-13: Phase 01 step 1.7: the `//gocpp:` directive parser
- [x] 2026-10-13: Phase 01 step 1.3: parser patch 2 (`std.array[int, 5]`)
- [x] 2026-10-13: Phase 11 job 11.10a: the VS Code language + TextMate grammar for `.gp`
- [x] 2026-10-13: Phase 01 step 1.16: `gocpp fmt` (gofmt's output, header import groups kept as written)
- [x] 2026-10-13: Phase 01 step 1.4: parser patch 3 (`std.chrono.seconds`, `std.vector[int].iterator`)
- [x] 2026-10-13: Phase 01 step 1.9: the package loader (`load.Load`) and the stdlib stubs

## Audit 2026-10-22

Job todo.audit checked every open line of this file and of the STATE files (phases 00-15) against dev
81c0d4e1: [AUDIT-2026-10-22.md](AUDIT-2026-10-22.md) has one table (item, verdict, evidence). Phase 16
and the running jobs (08.synclows, 06.lows, 11.lspgap5, 12.hresult, dev.walltests) were not audited.

Removed as obsolete (the code or document no longer exists, or the decision changed):

- 9.6 darwin: `GOOS=darwin gocpp check` of programs that import `os`: darwin is not a target (Q8, ADR-021).
- 4.16 review: automatic C++ treatment of a `.h` with a same-stem `.cpp`: decided by Q17, it stays a directive.
- 7.4-7.9 concept laxity (pointer method with another signature, `Get_` for `Get()`): go/types rejects both before the emitter.
- 7.3 `unsafe.Sizeof` of a type parameter's value: documented divergence D15, not a bug.
- 7.8 clang++ link failure for a `recover()`-aware literal with `gp::new_`: not reproducible with clang++ 21 (-O0, -release).
- 8.9/8.10 `Goexit` from main with a blocked goroutine hangs: it reports the deadlock like Go.
- 15.5 arm64 NEON group: decided against (swiss.hpp:150).
- TestProgramParallel g++ internal compiler error with the PCH: not reproducible (one run, passes).
- Runtime TestCXX failure of 3.8 s on 2026-10-14: the log was lost, nothing to act on.
- 10.11 R3-F2 (`expand` prints a half-processed file) and the doubled ADR 092; the merge note of job/09-flag; the `x, y := x+1, 5` scope claim of the 2.10b review (not reproduced, the emitter was rewritten).

Wrong claims fixed:

- 9.6 `PRIORITY: Close does not wake a blocked Read/Write`: fixed on 2026-10-19 (423d71aa); the line kept saying open.
- 12.6 "typed lowering is OFF by default behind GOCPP_TYPED_INTEROP=1": typed is the default since 1c342d97 (ADR-131).
- 6.11 open question "compile_commands.json after every build or opt-in": decided in 0cf8ec4e (debug builds write it).
- Phase 06, 08, 13 status lines: see their STATE files (06 and 08 are done; 13 has all steps merged but its debugger exit criterion currently fails in tests/gdb).
- "Parallel isolated jobs" (old line 40) listed jobs that are merged; replaced by the real list of running jobs under "Now".
- Q9 ("GitHub: not yet") is stale: `origin` is github.com/huuhait/GoToCPP and the workflows exist (never run there).
- The 9.9 line asked to delete `testing_regex.cpp`, the 9.4b line to drop two `@skip iter.go` rules and `sortDirEntries`, the 9.11 line about `examples/macros` pending: all done.

Verdict counts (rows of the table; a row that says "carried" stands for several small claims that were
read against the code but not run again): see the "Counts" table of AUDIT-2026-10-22.md (44 DONE, 181 OPEN, 25 PARTLY, 13 OBSOLETE, 9 IN-PROGRESS).
