# ADR-107: The standard library beyond tier 2 goes by blockers, not by popularity

Status: proposed (2026-10-20). Post-1.0 direction. Continues [ADR-018](018-stdlib-ported-from-go.md) and the tiers of [Phase 09 §2.6](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#26-package-tiers); the package named `net/http` is "never" in [feature-matrix.md](../spec/feature-matrix.md) and stays so here.

## Context

Tier 1 and 2 are Go's own source copied by `tools/portstd` ([ADR-034](034-portstd-generic-arch-and-rewritten-constraints.md)). The cost so far, from `stdlib/PORTED.txt`, [porting-blockers.md](../guides/porting-blockers.md) and the step reports: a package without assembly, `linkname` or missing runtime support is one line in `ports.txt` plus a gate row, a stub-generator entry and a Go-test fixture (step 9.11 ported about twenty such packages, `encoding/*`, `container/*`, `hash/*`, `log`, `path/filepath`, `math/rand`, `text/tabwriter`, in one step, ADR-102); a package with reference cycles needs patch rules (`regexp/syntax` 15, [ADR-083](083-sort-regexp-ports-and-the-test-matcher.md)); a package that uses `internal/reflectlite` needs natives (`errors`: 5 patches, 3 natives); a package that touches the machine needs natives (`math` 13 and bit-exact results, [ADR-071](071-math-bit-exact.md); `os` and `syscall`, [ADR-067](067-os-on-posix-natives-and-go-errors.md), [ADR-076](076-os-on-windows-over-a-generic-syscall.md)). Step 9.11 recorded what it did not port: `crypto/*` (Go's FIPS tree), `os/exec`, `hash/maphash`, `hash/adler32`, `hash/crc64`, `text/template`.

The remaining packages, sized from the go1.25.14 sources (lines of non-test Go in the package folder, without sub-packages; `asm` = assembly files):

| Package | Lines | Blocker class |
|---|--:|---|
| `compress/flate` 3.2k, `gzip` 0.5k, `zlib` 0.4k, `bzip2` 0.9k, `lzw` 0.6k | 5.6k | none seen: pure Go, no `linkname` |
| `archive/tar` 3.1k, `archive/zip` 2.3k | 5.4k | need `compress`, `os`, `io/fs`, `time` (exist) and, for `tar`, `os/user`, `syscall` (`Stat_t` in `stat_unix.go`) and `internal/godebug` (check what exists) |
| `encoding/csv` 0.7k, `encoding/xml` 4.4k, `encoding/gob` 6.1k | 11.2k | reflect-heavy; they use none of the reflect functions missing today |
| `net/url` 1.4k, `net/netip` 1.7k, `mime` 1.4k | 4.5k | `net/url` imports `net/netip`, which imports `unique`; `unique` is not ported and needs `weak`, `runtime.AddCleanup`, `internal/sync` (HashTrieMap) and a `linkname` to `runtime_rand`: it is a prerequisite (a port with natives, or a patch of `netip`). `net/url` also has one `linkname` file. `net` needs the same |
| `hash/maphash` | small | not pure Go: under `purego` it imports `crypto/rand` and `reflect`, otherwise it links to `runtime.memhash` and `runtime.rand`; tied to the crypto wave. `hash/adler32` and `hash/crc64` are pure |
| `math/big` | 10.1k + 12 asm | `purego` fallbacks exist; 2 files with `linkname` or cgo |
| `text/template` 2.9k + `parse` 2.5k, `html/template` 4.9k | 10.3k | needs `reflect` `Method`/`Call` ([ADR-106](106-reflect-grows-method-and-call-for-types-the-program-already-has.md)) |
| `os/exec` 1.9k, `os/signal` 0.7k + 1 asm | 2.6k | needs process and signal natives; `os.StartProcess` is "not built" ([feature-matrix.md](../spec/feature-matrix.md)) |
| `net` | 18.9k (21 files with `linkname` or cgo) | sockets, resolver, a poller; `internal/poll` is a stub |
| `crypto/*` | `sha256` is 74 lines over `crypto/internal/fips140/sha256`; 47 internal packages | Go's FIPS tree, security-critical, assembly or `purego` |
| `crypto/tls` 14.5k, `net/http` 29.1k (plus a vendored `x/net`) | 43.6k | needs `net`, `crypto`, `compress`, `mime`, `html` first |

The 9.6 priority low is the same problem as `net`: "`Close` does not wake a blocked `Read`/`Write` on a pipe, FIFO or tty" (Go's poller wakes them with `ErrClosed`), so a sockets package needs the wake mechanism (an eventfd or self-pipe polled with the descriptor) that `os` still lacks ([TODO.md](../../plans/2026-10-13-gocpp/TODO.md)). With one thread per goroutine, blocking system calls are fine; deadlines and `Close` while blocked are not.

C and C++ libraries are modules ([ADR-023](023-module-downloads.md), Q14), which is how gocpp reaches HTTP, TLS and databases.

## Options

1. **Port by popularity** (HTTP and TLS first). Starts the biggest, riskiest work first, on top of a poller that does not exist.
2. **Port by blocker class**, cheapest first, with a rule for what is not ported.
3. **Wrap everything with C/C++ libraries** (zlib, OpenSSL, libcurl). Fastest to start, but pulls external libraries and their licenses into the standard library, differs in behavior and error values from Go, and the Windows story is worse.
4. **Port nothing more**; the standard library stays what v1.0 ships.

## Decision (proposed)

Option 2, in waves, each wave a step with Go's own tests as the acceptance bar (the `tests/gotest` fixture of ADR-102, 90% of the selected test functions as for tier 1):

- **Wave 1 (pure Go):** `compress/*`, `archive/*`, `encoding/csv`, `encoding/xml`, `encoding/gob`, `mime`, `hash/adler32` and `hash/crc64`, `math/big` on `purego`, and `unique` first (a port with natives, or a patch of `net/netip`), then `net/netip` and `net/url`; `hash/maphash` waits for Wave 4. Expect one step; the work is fixtures and cycle patches (run the leak audit, [ADR-090](090-nightly-sanitizers-and-a-bounded-leak-policy.md)).
- **Wave 2 (reflect stage 1, ADR-106):** `text/template`, `html/template`, `testing.F`, `testing/quick`.
- **Wave 3 (system layer):** `os/exec`, `os/signal`, then a design for the wake-on-close poller (which also fixes the 9.6 low), then `net` sockets with deadlines.
- **Wave 4 (decide with a spike):** the small crypto set (`sha256`, `sha512`, `hmac`, `aes`, `cipher`, `rand`): port Go's FIPS-tree code with `purego`, or wrap a C library as a module. The test is: which one passes Go's known-answer tests on Linux and Windows with less code.
- **Not ported, as a rule:** `net/http`, `crypto/tls` and anything above 20k lines that depends on `net`: a module that wraps a C++ HTTP library through interop (the "wrapper package" of the plan), published outside the standard library. `plugin`, `go/*` (until self-hosting is chosen, which is not proposed here) and `runtime/*` internals stay out.

A port is accepted when: Go's tests of the package run in a copy of the port and print what `go test` prints, nothing is skipped for a reason other than the documented ones, and the sanitizer jobs are clean (cycles are either patched or listed with a ceiling).

## Trigger

- Demand: after 1.0, count what users ask for (issues, the `skip:not-planned-package` tests of the conformance run: 7 today) and reorder inside a wave, never across the blocker order.
- Wave 2 waits for ADR-106's measurement; Wave 3 for the poller design; Wave 4 for the spike.

## Consequences

- Good: the cost of each step is known from its class; the riskiest pieces (poller, crypto) come last and with a spike; HTTP stays a library problem.
- Bad: no `net/http` in the standard library means Go programs that import it do not build as they are (the feature matrix lists `net/http` as "never"); a program with a TLS server needs a module.
- Uncertain: line counts are a proxy for cost; the "none seen" in Wave 1 is a result of reading import lists and `go:linkname`/assembly, not of a port, and transitive imports were missed once (`unique` behind `net/netip`), so each wave starts with `portstd -list` on its roots. Test fixtures of big packages (the `xml`, `gob` ones) may need the same kind of patches as `encoding/json` had (shims for `math/big`, `image`).

## Evidence

`stdlib/PORTED.txt`, [porting-blockers.md](../guides/porting-blockers.md), [porting.md](../guides/porting.md), ADR-102 (step 9.11, "Not ported" list and the fixtures), line counts from the go1.25.14 tree (`find <pkg> -maxdepth 1 -name '*.go' ! -name '*_test.go'`), the 9.6 lows in [TODO.md](../../plans/2026-10-13-gocpp/TODO.md), [go-test.md](../conformance/go-test.md).

## See

[Phase 09 §2.6](../../plans/2026-10-13-gocpp/phase-09-stdlib-testing/README.md#26-package-tiers), [feature-matrix.md](../spec/feature-matrix.md).
