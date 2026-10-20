# ADR-110: The Go language version moves once a year, and the v1 promise names what it covers

Status: proposed (2026-10-20). Post-1.0 policy. Refines [ADR-012](012-go-subset-and-extensions.md) (gocpp is Go 1.25 plus a short list of extensions) and gives the text of the v1.0 compatibility promise ([Phase 15 §2.7](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#27-v10-compatibility-promise), step 15.16) something to be exact about. It supersedes nothing.

## Context

**How the Go version is fixed today.** One language version, pinned in six places: the checker runs `go/types` with `GoVersion: "go1.25"` ([language.md](../spec/language.md), Phase 01 §2.7); the parser fork `internal/syntax` is a fork of go1.25's `go/scanner` and `go/parser` (a fork of 1.26 needs one more copied package, `go/internal/scannerhooks`, `internal/syntax/FORK.md`); the standard library is ported from the tarball pinned in `tools/portstd/PINS` (go1.25.14, [porting.md](../guides/porting.md)); the conformance suite is Go's `test/` folder at that version (1,941 of 2,131 applicable tests pass, 91.1%, [go-test.md](../conformance/go-test.md)); `go.mod` says `go 1.25.0`; and the dev shell's Go is 1.25.14, which holds staticcheck at v0.7.0 because v0.8 needs Go 1.26 ([CONTRIBUTING.md](../CONTRIBUTING.md)). A `go 1.25` line in a `gocpp.mod` is read and written back unchanged ([modules.md](../spec/modules.md)): the language version is the compiler's, not the module's.

**Where Go is.** On 2026-10-15 the latest stable Go releases were 1.27.2 and 1.26.9 ([upstream/gc-shift-count-issue.md](../../plans/2026-10-13-gocpp/upstream/gc-shift-count-issue.md)). Go supports its two newest releases, so the pinned 1.25 is already outside Go's own support window (the dev shell's Go is 1.25.14). Nothing in the repository has surveyed what 1.26 and 1.27 changed in the language (Go 1.26's release notes list a change to `new` taking an expression operand and to self-referential generic constraints; this was not verified here, it is the first thing the survey below would check).

**What the promise says now.** "Valid gocpp 1.x programs keep compiling and behaving the same across 1.x releases. Not promised: the exact generated C++ and the runtime ABI; generated code must be regenerated; `cpp-lib` headers are versioned." That is all. It does not say which behaviors count.

**What has moved inside the project's own extensions.** Macros were amended once already ([ADR-014](014-macros.md), amended by [ADR-075](075-user-macros-are-generic-functions.md): constants instead of `Site`, generic instances instead of a separate mechanism); the gate paragraph of [ADR-037](037-must-rewrite.md) and the `Phase10Syntax` clause of [ADR-058](058-builtin-macros-and-embed-in-the-loader.md) were superseded by [ADR-095](095-must-and-macros-are-always-available.md). Planned additions: `err := #body()`, `!` on comma-ok results, procedural macros ([TODO.md](../../plans/2026-10-13-gocpp/TODO.md), [language.md](../spec/language.md)). The documentation site already labels several surfaces "(experimental)": goroutines, the standard library, must functions and typed interop (`docs/site/nav.txt`).

## Options

1. **Stay on 1.25** until a reason appears. Cheapest, but the checker, the ports and the conformance baseline age, and a Go 1.26 program that uses a new feature is a gate error.
2. **Follow Go closely**: move to each new release within weeks. Highest cost: a re-port, a re-baseline of the conformance suite and a parser re-fork twice a year.
3. **One move a year, in a minor release** (the proposal below). The cost is paid once and is predictable; a program written for the previous version keeps compiling (Go's language promise makes newer versions accept it).
4. **A language version per module** (honor the `go` line of `gocpp.mod`, as Go does). Needs the lowering to support both semantics where they differ (loop variables, `min`/`max`, range-over-func, generic aliases). Not needed while only the newest semantics exist; a `go` line newer than the compiler supports should be an error (as in Go's toolchain line check).

## Decision (proposed)

**Version policy.** gocpp supports exactly one Go language version, G, at a time. G changes at most once a year, only in a minor release (1.y), to the newest Go release that has had at least two patch releases. A move is one step with a checklist: a written survey of the language and library changes (what the checker needs, what the ports need, which new syntax needs a parser patch or a re-fork); the re-port with `tools/portstd -pin`; the re-baseline of the conformance suite with the passing list unable to shrink; the dev shell and staticcheck pins; the changelog line. `gocpp.mod`'s `go` line stays ignored for semantics, but a line newer than G is an error ("this module needs Go 1.x; this gocpp supports 1.y"). The first move (to 1.26 or 1.27) is a candidate for 1.1.

**What the v1.0 promise covers** (the proposal for the text of step 15.16):

- Covered: the syntax and meaning of the language as written in [language.md](../spec/language.md), including `!`, macros, the built-in macros and the derives `String`, `Equal`, `Clone` and (once step 9.11 merges; [language.md](../spec/language.md) says "JSON is planned" until then) `JSON`; the formats of `gocpp.mod` and `gocpp.sum`; the flags, commands and exit codes of `build`, `run`, `test`, `check`, `fmt`, `vet`, `get`; the API and behavior of the ported standard packages at version G; the rule that typed interop never rejects a program that opaque interop plus the C++ compiler accepts.
- Not covered: the generated C++ and the runtime ABI (as now); the text of diagnostics (tests key on them, but wording may improve; the `gocpp explain` topics are covered by name); `GOCPP_*` developer variables (except that `-interop=typed|opaque|auto` and `GOCPP_INTEROP` are covered as user settings once step 12.11 merges: their names and values stay, the default may change by ADR-108); speed and memory (they may go down; leaks of reference cycles may be fixed); anything the spec marks as implementation-defined (map order, the layout of a panic trace); what depends on a C or C++ header's own version; surfaces still labeled experimental when 1.0 ships (the list must be explicit in the release notes and the labels removed or kept on purpose).
- A divergence from Go that is a bug (the failing tests of the conformance suite) may be fixed in a 1.x release; the changelog says so, because a program that relied on the bug changes.
- **Extensions are additive in 1.x**: no new keyword (`macro` and `lazy` are keywords in two places only, [D18](../spec/language.md#d18-macro-and-lazy-are-keywords-in-two-places), and `C` is a reserved name, [D19](../spec/language.md#d19-c-is-a-reserved-name); that is the whole list; a new form is contextual or starts with `#`), no change in the meaning of an existing valid form. `err := #body()` and `!` on comma-ok are additive. Procedural macros run code at compile time and need their own ADR about trust and sandboxing.

## Trigger

- Step 15.16 (the text of the promise is written then; this ADR is accepted, amended or replaced on it).
- The next Go release after 1.27, or the first user program that needs a language feature newer than 1.25 (the survey starts).
- A bug fix that changes the behavior of a valid program, which asks whether the promise's third bullet is enough.

## Consequences

- Good: users know what they can rely on; the cost of tracking Go is paid in one planned step a year instead of by accident; Go's own stability promise makes older programs keep working.
- Bad: gocpp lags Go by a few months by design; a program that wants a feature of the newest Go waits for the yearly move.
- Uncertain: the size of the first move (the survey is the first task); whether a language version per module (option 4) is wanted by users who depend on old modules.

## Evidence

[language.md](../spec/language.md), [ADR-012](012-go-subset-and-extensions.md), [Phase 15 §2.7](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#27-v10-compatibility-promise), `tools/portstd/PINS`, `internal/syntax/FORK.md`, [go-test.md](../conformance/go-test.md), [CONTRIBUTING.md](../CONTRIBUTING.md), [modules.md](../spec/modules.md), upstream/gc-shift-count-issue.md, the ADRs named above.

## See

[Phase 15 §3](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#3-steps) (15.16, 15.17), [Phase 10 §2.2](../../plans/2026-10-13-gocpp/phase-10-must-and-macros/README.md).
