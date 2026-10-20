# ADR-109: After 1.0 the pipeline and arm64 come first; macOS needs a machine and lldb parity; a checksum database needs an ecosystem

Status: proposed (2026-10-20). Post-1.0 direction. Builds on [ADR-021](021-platforms-linux-windows.md) (platforms), [ADR-023](023-module-downloads.md) and [ADR-070](070-version-selection-and-gocpp-get.md) (modules and their trust), [ADR-066](066-debugger-support.md) (debugger) and [ADR-089](089-packaging-go-install-goreleaser-nix.md) (packaging). It would amend ADR-021's "macOS is not a target" only if its trigger fires, and then by a new ADR.

## Context

**Platforms.** Linux x86-64 and Windows x86-64 are the targets; macOS is "not a target. The code stays portable (standard C++, one OS layer)" (ADR-021, Q8). Linux arm64 is in the v1.0 plan (step 15.12, open): the Swiss-table maps use a portable group on arm64 that was only compile-checked, with no NEON group (15.5 low in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md)), and Go's arm64 conversions saturate where gocpp copies amd64 ([Phase 15 §2.3](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#23-platforms)). Windows arm64 ("llvm-mingw supports it") and the MSVC ABI (`clang-cl`) are listed as after 1.0. The link driver already knows about macOS in one place (`--gc-sections` is not passed there, [build-link.md](../spec/build-link.md)). The machines the project tests on are the Linux server and one real Windows PC (ssh); no document mentions a Mac or an ARM Windows machine to test on.

**Debugger.** gdb is the full debugger (printers, frame filter, `gp goroutines`, travelling inside every program); LLDB gets "the values (best effort)" ([ADR-066](066-debugger-support.md), [debugging.md](../guides/debugging.md)). Known gaps from the 13.2 review: `frame`, `up`, `finish` and `info locals` still print C++ names and temporaries (a gdb frame decorator covers only `bt`), and a boxed local's type is read from its declaration line (so `x := f()` stays `(*int)`); the emitter could mark boxes. macOS has no gdb that is practical to rely on, so a macOS target depends on LLDB parity.

**Modules and trust.** Modules come from git hosts as tag or commit archives; the hash is Go's `h1:` tree hash in `gocpp.sum`, trust on first use, no checksum database, a mismatch is a hard `SECURITY ERROR`; `GOCPP_MODPROXY` accepts a mirror (a `file://` folder is what is tested); `gocpp mod vendor` and `-mod=readonly` are the strict modes (ADR-023, ADR-070, [modules.md](../guides/modules.md)). ADR-023 did not use Go's proxy because it would serve a made-up `go.mod` (a different tree), and gocpp filters nested `gocpp.mod` directories before hashing (15.14), so its hash can differ from Go's for the same repository. The plan says "a proxy is a later option". No public gocpp module exists yet that this project knows of.

**Hosting.** `dev` is pushed to a GitHub remote since 2026-10-20 (the user's standing order); decision Q9 as recorded in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md) still says "GitHub: not yet; CI is `make ci` on your machine". `.github/workflows/sanitizers-nightly.yml` exists; whether it has run on GitHub is unverified. GoReleaser builds static `CGO_ENABLED=0` archives for linux amd64/arm64 and windows amd64 as a draft release; "the release pipeline (15.12 and 15.16); nothing was published", the Windows host's pinned toolchain download and MinGW in the Nix package are not done (15.13 note in [Phase 15](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md)). BSD-3 requires the license notices to travel with a binary (Phase 15 STATE, 2026-10-14 note).

## Options

*Platforms*: (a) Linux arm64 as in 1.0, then **macOS arm64 and x86-64** as best-effort targets (Apple clang and libc++ replace libstdc++; the OS layer `runtime/src/os/` needs a third implementation of thread stack bounds, the stack-overflow hook, the executable path and the clock; LLDB-only debugging; sanitizer support differs); (b) Windows arm64 with llvm-mingw once an ARM machine exists; (c) the MSVC ABI through `clang-cl`, for C++ libraries built with Visual Studio; (d) none of them until asked.

*Trust*: (e) keep TOFU plus `gocpp.sum` plus vendoring; (f) ask Go's checksum database for modules that Go's proxy also serves, when the tree hashes agree (needs a test of when they do); (g) run a gocpp proxy and checksum database (a service, a trust root, a privacy policy; the protocol is Go's, which gocpp implements without `x/mod`); (h) sign release binaries and `checksums.txt` (independent of modules).

*Pipeline*: (i) enable the workflows on the GitHub repository and update Q9 (a user decision); (j) stay local.

## Decision (proposed)

In this order, each step standing alone:

1. **Confirm Q9 (the repository is on GitHub), then run the pipeline**: the nightly sanitizer workflow, the arm64 job (step 15.12), GoReleaser releases with `checksums.txt`, signatures (option h) and the third-party license notice. Until Q9, `make sanitize-nightly` and `tools/sanitize/nightly.sh` on the author's machine stay the nightly.
2. **Linux arm64** as a tested target (in 1.0), with the NEON group of the maps as a measured follow-up.
3. **macOS** only when there is a maintainer with a Mac and LLDB reaches parity with gdb's `gp` commands for locals, frames and goroutines. Then a new ADR.
4. **Trust:** keep (e) for the first year after 1.0. Test (f) as a cheap experiment (do Go's sumdb and gocpp agree on the hash for a module that has both `go.mod` and `gocpp.mod`?). Build (g) only when public gocpp modules exist in numbers that make first-use attacks a real risk (the trigger below).
5. **Windows arm64, MSVC ABI:** on request, each its own ADR.
6. **Debugger:** finish the 13.2 lows (box markers in the emitted C++ so locals have their Go types, Go names in `frame`/`up`/`finish`) before adding another debugger front end.

## Trigger

- The user confirms Q9 (step 1); a failing arm64 CI or an arm64 user (step 2).
- A Mac user who commits to testing, and a working LLDB `gp` command set (step 3).
- More than a handful of public gocpp modules, or a reported hostile first download (step 4).
- A user library that is only distributed as a Visual Studio build (step 5).

## Consequences

- Good: each step is small and measurable; no platform is promised without a machine to test it; the security posture of modules is stated rather than implied.
- Bad: macOS users wait; trust on first use stays the weakest point for a while (mitigated by `gocpp.sum` in version control, `-mod=readonly` and vendoring).
- Uncertain: whether macOS is wanted by the users (the plan does not know any); what a macOS port of the OS layer costs (nothing in the repository measures it).

## Evidence

[ADR-021](021-platforms-linux-windows.md), [ADR-023](023-module-downloads.md), [ADR-066](066-debugger-support.md), [ADR-089](089-packaging-go-install-goreleaser-nix.md), [Phase 15 README](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md) (§2.3, §2.4, §2.5 and the 15.13/15.14 notes), the TODO.md decisions Q8, Q9 and the 15.5, 13.2 and 15.14 lows, `.github/workflows/sanitizers-nightly.yml`.

## See

[Phase 15 §2.8](../../plans/2026-10-13-gocpp/phase-15-hardening-release/README.md#28-after-10-each-becomes-an-adr-when-chosen), [Phase 13](../../plans/2026-10-13-gocpp/phase-13-tooling/README.md).
