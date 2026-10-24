# v0.1 release-candidate preparation

**Unreleased; not approved for tagging or publication.** This is the preparation record for
Phase 09 step 9.13a, based on `0ceb8f49` (2026-10-20). It is not a release announcement or a
compatibility promise. Step 9.13, milestone M6 and the outstanding acceptance criteria remain
open. The version is still `dev`, the completed-phase marker is still `05`, and Phase 09 APIs
require `GOCPP_GATE_PHASE=09`. Neither marker is advanced by writing these notes.

## Draft changelog

This describes the candidate source tree, not features already delivered in a published v0.1.

- The Go-shaped library includes strings, bytes, Unicode, number conversion, sorting,
  collections, regular expressions, I/O, flags, time, context and synchronization. Most of the
  portable code is ported from **Go 1.25.14**. `os` uses Go-derived code over native Linux and
  Windows file operations. The [checked inventory](stdlib.md) records 51 public and 20 internal
  package directories, including shims and the `embed` stub; those counts are not counts of
  complete Go packages.
- `fmt` is intentionally a **native print/scan engine**, not a port of Go's `fmt` through
  reflection. The reviewed 9.7a work fixes scanning, live operands and standard-stream ownership
  and close ordering. It preserves C/C++ value formatting; it does not promise every Go
  allocation, error-type or concurrent-use behavior. See [ADR-100](../adr/100-fmt-stays-native-decision-9-7-a.md).
- `encoding/json` and several tier-2 packages are present, with descriptor-based reflection and
  `#Derive(JSON)`. Presence and selected upstream passes do not close the separate JSON corpus
  round-trip acceptance criterion.
- `gocpp test` runs `_test.gp` tests, subtests, examples, benchmarks and fuzz seed corpora, with
  Go-style output. It is not the full Go testing tool: see [testing differences](testing.md#what-differs-from-go).
- Step 9.10 adds reproducible conversion manifests and counting for Go's own tests in shipped
  packages, plus the deferred generic/range/channel suites. It also fixes generic-instance
  captured variables, `T.Setenv`, the `os` stat hook and `time` once initialization.
- Existing packaging provides `go install`, GoReleaser archive generation and a Nix package
  with wrapped compilers. Review of this preparation caught a local module-zip installation
  failure: the probe source basename `aux.go` is reserved on Windows. Its content-preserving
  rename to `auxiliary.go` makes the focused offline local-proxy install pass. This preparation
  also adds distribution notices; it does not create a tag, upload an archive, or install
  anything for a user. See [installation](packaging.md).

## Limits to read before trying it

The [standard-library differences](stdlib.md#what-differs-from-go) are part of these notes:

- Memory uses **automatic reference counting**, not tracing GC. Cycles leak unless broken;
  `runtime.GC` does nothing and finalizers do not close files. Close resources explicitly.
- A goroutine is an **OS thread**, normally with a fixed 8 MiB stack, not a lightweight Go
  scheduler task with a growing stack. `GOMAXPROCS` does not limit running threads. Do not
  assume Go-scale goroutine counts or deep recursion are safe.
- Missing production packages include `crypto/*`, `os/exec`, `os/signal`, `net`, `net/http`,
  `encoding/csv`, `compress/gzip`, `archive/zip`, `text/template` and `time/tzdata`.
  `embed.FS` is not implemented. Test-only copies are not application packages.
- Reflection is a subset: no dynamic `Value.Call`, `Value.Method`, `MakeFunc`, `StructOf` or
  `FuncOf`. `unsafe` sizes/layouts are C++/gocpp layouts, not Go's ABI.
- `os.Root` and process creation are absent; long-path `RemoveAll` and huge zero-size slices
  have known bugs. Linux pollable-file deadlines exist but were excluded from the recorded
  upstream selection. Windows deadlines remain unsupported; Windows `time.Local` is UTC and
  named zones need `ZONEINFO`.
- Compiler and platform requirements are in [packaging](packaging.md#platforms-and-requirements).
  Linux amd64 evidence is not evidence for Linux arm64 or Windows. Generated C++ and runtime
  artifacts should be regenerated together with the matching compiler; no v1 ABI promise is
  made here.

## Phase 09 acceptance evidence

The table distinguishes **recorded evidence** from checks run on a future release commit.
The [phase state](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/STATE.md) owns the actual
checkboxes; this document does not change them. No full acceptance sweep was run for 9.13a.

| Exit criterion | Evidence available at the preparation baseline | Remaining gate |
|---|---|---|
| All tier-1 packages, at least 90% of selected Go test functions under the counting rule | Recent 9.10 record on Linux amd64 with g++ (updated 2026-10-23, when `TestRemoveAllLongPath` began to pass: [stdlib-tests.md](stdlib-tests.md#results)): **25 packages**, 1769 total functions, 1661 selected, 26 divergences removed from the denominator, **1607/1635 = 98.3% selected**, **1607/1769 = 90.8% overall**. It includes 20 of 24 tier-1 packages plus five others. `os` is **164/179 = 91.6% selected**, **164/278 = 59.0% overall**. | **Open.** `fmt`, `sync`, `sync/atomic`, `testing` have no manifest in this count; other tests exist but do not establish this precise upstream-test criterion for all tier 1. No new all-package or cross-platform run. |
| Deferred generic, rangefunc and channel suites | Recent 9.10 record: `slices`/`maps`/`cmp`/`iter` **145/150** selected, with 3 allocation divergences and 2 zero-size-slice failures; rangegen **800/800 units**; `test/chan` **18/19**, with `select2.go` skipped for runtime internals. | Phase state marks the documented selection accepted; it is **not a claim of zero exclusions or failures**. Retain the reasons and the selected/all distinction. |
| `wc` and `grep-lite` match Go | Historical 2026-10-19 flag-port record: identical output for **14 and 12 argument vectors**, respectively. | Marked accepted in phase state, but no rerun on the candidate commit. Preserve that scope rather than claim all inputs. |
| `gocpp test` output matches `go test` | Historical 9.9 fixture comparison for pass/fail/benchmark output in `tests/gotest` ([ADR-056](../adr/056-gocpp-test-and-the-testing-package.md)). | Marked accepted in phase state; not revalidated by the 9.13a docs checks. Missing testing features still apply. |
| `fmt` differential coverage of at least 100 cases | Phase state records **210 `fmt_*` run tests** against Go. Recent 9.7a integration additionally records ten run fixtures on GCC and Clang, ten real-Go comparisons without skips, and focused close/ownership checks. Six corrected fixtures have source-branch real-Windows evidence. | Existing accepted coverage, not a new run of all 210; no full upstream `fmt` sweep and no separately verified Windows fmtdiff interruption run. |
| JSON corpus structs round-trip like Go | Historical 9.11 record: 16 differential programs (984 output lines), 86 tests, 13 examples and 2 fuzz seeds. The later 9.10 manifest records **101/103** passing, two dropped; these are different selections, not additive counts. | **Open.** The specific corpus round-trip criterion remains unchecked. Do not turn package presence, historical tests, or the 98.1% selection result into acceptance. |

The [recorded results and exclusion reasons](stdlib-tests.md#results), package manifests and
[Phase 09 state](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/STATE.md) are the sources for
these numbers. Inventory/link/sample checks validate these documents and the installation
path only; they do not refresh the numerical results above.

## Platform and packaging gates

| Surface | Available evidence | Still required before offering it as a validated RC |
|---|---|---|
| Linux amd64 | Main development/test host; historical Nix install checks from 15.13. In 9.13a review, the offline local-proxy install first failed on the reserved basename `aux.go`; after its rename, the retake passed, including version and sample execution outside the source tree. | Review the exact candidate commit, build/archive metadata, notices and checksums; run the agreed final checks. A local module-proxy install is not a fresh release archive or Nix install check. |
| Linux arm64 | GoReleaser cross-build configuration; Nix package evaluated in 15.13; 15.12 wrote the arm64 CI job and the dev shell's `aarch64-linux` evaluates (workflow checked statically only; a snapshot arm64 binary printed its version under `qemu-aarch64` user emulation, not on arm64 hardware). | Native arm64 execution evidence is absent until the first hosted run. Cross-compilation/evaluation is not platform conformance; either obtain evidence or explicitly leave this target unvalidated. |
| Windows amd64 | Real Windows 11 evidence for focused runtime/interop work; 14.6 records `TestWindowsDebug` **9/9**, typed `<windows.h>` run and native typed check. | Full Phase 14 exit criteria and the [manual Windows release checklist](windows-testing.md#the-manual-checklist-for-a-windows-release) remain open. No Windows run in 9.13a; no lldb/lldb-dap or actual VS Code debugging session is claimed. |
| Binary distributions | Existing `.goreleaser.yaml`: linux amd64/arm64 tarballs, windows amd64 zip, `CGO_ENABLED=0`, version stamping, SHA-256, draft release. `.github/workflows/release.yml` (15.12) builds them on a tag and verifies each archive on its platform; it has never run, and a local snapshot's archives were checked on Linux amd64 and the Windows zip's `gocpp.exe` on the real PC. | No RC archives or public installation/tag are created by this job. Validate the final generated contents and stamp; select the offered targets explicitly. |
| Licenses | Root BSD-3-Clause license and `THIRD_PARTY_NOTICES.txt` for Go-derived code, Go tools, fdlibm/Sun and Cephes notices. Config includes both in archives and the Nix output. | Review actual output, linked dependencies and optional bundled toolchains at distribution time. No compiler or external C/C++ library is bundled by this configuration. |

## Release-owner checklist (not an approval)

- [ ] Resolve the open tier-1 and JSON criteria with evidence, or obtain an explicit plan/scope
  decision from the user. Do not silently waive them or mark full 9.13 complete.
- [ ] Choose the exact candidate commit, version/tag spelling, supported/preview platform list
  and phase-gate policy. The current default stays phase 05; setting the environment is opt-in.
- [ ] Have the user perform/review the agreed final verification. Record commit, compiler,
  platform, command, passes, skips and failures. Historical runs cannot stand in for this.
- [ ] Review the full release note/install path, known gaps, archive contents, notices and
  checksums. Keep `LICENSE` and `THIRD_PARTY_NOTICES.txt` with redistributed binaries.
- [ ] Reconcile README/install claims with the approved artifact availability and status.
  `@latest` is mutable and is not a candidate pin; no example version is a published release.
- [ ] Obtain **explicit user approval** before creating/pushing a tag, creating even a draft
  remote release, uploading binaries, or publishing packages. A local check or clean review
  is not that approval.

Performance claims, a v1 compatibility promise, all-platform conformance and completion of
other open phases are outside this preparation. Do not expand a documentation job into a
full test, benchmark or sanitizer sweep to manufacture a release-ready label.
