# ADR-101: The interop mode is chosen by a flag, and typed falls back to opaque without clang

Status: accepted (2026-10-20). Builds on [ADR-005](005-interop-header-packages.md) (opaque interop),
[ADR-060](060-typed-interop-probes.md) (the probes), [ADR-079](079-typed-interop-checker-reports-only-what-the-compiler-reports.md)
(the checker), [ADR-087](087-typed-interop-code-generation.md) (the code generation), [ADR-097](097-typed-interop-puts-the-values-of-c-constants-into-the-program-before-go-types.md)
(constants) and [ADR-098](098-promoted-members-of-embedded-cpp-classes-are-decided-by-the-compiler-in-a-second-pass.md)
(promoted members); the contract is [docs/spec/typed-interop.md](../spec/typed-interop.md) §13.

## Context

Steps 12.1 to 12.10 built the typed pipeline piece by piece, each behind the hidden developer switch
`GOCPP_TYPED_INTEROP=1`, and left two things out: the interop checker (12.5) ran only in the hidden command
`gocpp __typed-check`, and nothing said what happens without clang++. A user needs one documented way to choose,
a predictable answer when clang++ is missing, and a decision about the default.

## Decision

1. **One flag, one variable.** `-interop=typed|opaque|auto` on `check`, `build`, `run`, `test`, `emit`, `vet`, `expand`
   (and, because they take the build flags, `debug` and `export`), and `GOCPP_INTEROP` with the same values. The
   flag wins over the variable, which wins over the older `GOCPP_TYPED_INTEROP` (`1` is typed, `0` is opaque; kept as
   an alias and listed in `gocpp env`), which wins over `typedinterop.DefaultMode`. A bad value is a usage error
   (status 2). The rules live in `internal/typedinterop/mode.go` (`ResolveMode`, `Decide`, `Hooks`), not in `cmd/gocpp`,
   so that the language server (12.13) takes the same ones; `internal/lsp` was not touched.
2. **`auto` is typed when clang++ is installed, else opaque, silently.** At `-v` one line says which and why. The
   explicit `typed` without clang++ is an error with status 3 (the status of `gocpp bindgen` without clang++) and a
   message that names the source of the setting and the three ways out (install, `GOCPP_CLANG`, `-interop=opaque`).
   clang++ is looked for only when a package that is not the standard library's imports a header, so a program of Go
   alone pays nothing and `-interop=typed` does not fail it. `auto` stays opaque on a Windows host, where the probes
   do not have the MinGW-w64 target yet.
3. **Typed mode is the whole pipeline, run once.** `check` (and so every command that checks) sets the constants
   and members hooks, and after the type check runs `Analyze` once per package with the options of the code
   generation (`Decls`, `StringToCStr`, the target's data model) and gives the report to the interop checker
   (`icheck.CheckReport`); `build`, `run`, `test` and `emit` give the same report to `lower.Interop.SetTyped`.
   This ties the checker to the code generation (the string-to-`const char*` conversion the generated code does,
   the checker's probes do too), as ADR-087 asked. A package that cannot be probed stays opaque: a note at `-v` (and
   a warning for `typed`).
4. **Opaque is the default for now.** `DefaultMode = Opaque`. The evidence is good on what could be measured: 294
   programs that import headers (`tests/interop`, `tests/zoo`, `tests/run`, `tests/pkgs`, `examples`) build and print the
   same in typed mode with g++ and clang++ (three programs differ, all of them tests of errors that opaque mode
   rejects), and the warm overhead is 0 to 35 ms. It does not yet show: cold probing under 0.5 s for the library-heavy
   examples (0.9 to 2.4 s measured on a lightly loaded machine), clang 17 to 20 (only clang 21 here; step 12.12), the rest of 12.12's
   tests, a Windows host. **To switch:** set `DefaultMode = Auto`; reword the places that say "opaque is the default"
   (`docs/spec/typed-interop.md` §13.1 and §13.6, `docs/spec/interop.md` "What typed interop will add", the guides that name
   `-interop`, the `-interop` help text in `cmd/gocpp/interop.go` and `build.go`, the `GOCPP_INTEROP` default that
   `gocpp env` shows and its two goldens); the suites of `cmd/gocpp` and `tests` already set `GOCPP_INTEROP=opaque` in
   their `TestMain`, so they keep testing the opaque pipeline, and `TestCorpusTyped` the typed one. A dry run of the switch (`DefaultMode = Auto`, the focused tests of `cmd/gocpp` for the interop mode, typed interop, env, help, explain and Windows hints) failed only the two goldens of `gocpp env`.
5. **Cost.** `gocpp check` keeps the compiler detection between runs (`build.SetDetectCache`, as `build` does) when it
   looks for clang++: without it every warm typed check started clang++ twice.
6. **Messages.** The opaque lowering's message for a macro in an array length says how to get typed mode
   ("needs typed interop (-interop=typed, which needs clang++); or write the number"); the notes of the phase-12
   gates in the feature matrix say that typed interop accepts those features. The gate messages themselves
   ("... not supported yet (Phase 12)") are unchanged: tests, `gocpp explain` and the editor key on them.

## Consequences

- `GOCPP_TYPED_INTEROP=1` is no longer a quiet switch: it means typed, and typed without clang++ is an error. The
  tests that wanted the quiet fallback use `GOCPP_INTEROP=auto`.
- The checker's errors now appear in `gocpp check` and `gocpp build` in typed mode, before the C++ compile; with g++ as
  the build compiler they are warnings (ADR-079).
- Flipping the default later is one constant plus the wording; nothing else depends on it.

## Alternatives considered

- *Make `auto` the default now.* The compatibility evidence allows it; the unmeasured parts (older clang versions, cold
  cost on a quiet machine) are what a user with clang 18 would meet first, and the fallback would hide a failure there.
  Left to the manager and the user.
- *`typed` without clang++ falls back too, with a warning.* Rejected: a user who writes `-interop=typed` in a script or
  a CI file wants the early errors; a silent downgrade hides that they are gone.
- *Look for clang++ always.* Rejected: it costs a compiler run for programs that have nothing to probe.
