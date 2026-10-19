# time

Ported by hand from go1.25.14's `src/time` (`format.go`, `format_rfc3339.go`, `sleep.go`, `tick.go`, `time.go`,
`zoneinfo.go`, `zoneinfo_read.go`, `zoneinfo_unix.go`, `sys_unix.go`; BSD-3-Clause, see `../LICENSE`) in Phase 08 step 8.8
(ADR-041). Phase 09's `tools/portstd` will take over the port; these are the changes it must keep.

- **No generics** (Phase 07): `atoi`, `isDigit`, `parseNanoseconds`, `leadingInt` and `parseRFC3339` of Go are generic over
  `[]byte | string`; here they take a `string`. `parseStrictRFC3339(b string)` is called with `string(data)` by
  `UnmarshalJSON` and `UnmarshalText`.
- **No `unsafe`, `linkname`, `syscall`, `runtime`, `sync`, `internal/*`, `math/bits`**: the `//go:linkname` lines and the legacy
  `legacyTimeTimeAbs`, `legacyAbsClock`, `legacyAbsDate` routines are removed; `bits.Mul32` is `mul32`; `stringslite.Clone`
  is dropped (a string is a value); `bytealg.IndexByte` is a loop; the synctest bubble (`runtimeIsBubbled`) and
  `GODEBUG asynctimerchan` are removed (Go 1.23's behavior only); the `time/tzdata` hook
  (`registerLoadFromEmbeddedTZData`) and the `$GOROOT/lib/time/zoneinfo.zip` source and the AIX workaround are removed.
- **Natives** (`//gocpp:native`, ADR-027, `runtime/include/gocpp/time.hpp`): `now`, `runtimeNano`, `sleep`, `startChan`,
  `startFunc`, `stopTimer`, `resetTimer`, `trySend` (timers, `native.gp` and `sleep.gp`), `callOnce` (the three `sync.Once`
  of `zoneinfo.go`), `getenv`, and the file functions of `sys.gp` (`openFile`, `readFd`, `closeFd`, `seekFd`, `errnoText`;
  Go's `sys_unix.go` calls `syscall`). `errENOENT` is `syscall.ENOENT`.
- **Timers** (`sleep.gp`, `tick.gp`): `Timer` is `{C, r, initTimer}` and `Ticker` `{C, r, initTicker}`, where `r` is the channel
  object that carries the timer record (Go's runtime allocates a bigger struct and converts with `unsafe`). `newTimer`,
  `stopTimer` and `resetTimer` of Go's runtime are `startChan` or `startFunc`, `stopTimer` and `resetTimer` here. `sendTime` takes
  the channel, not an `any`, and sends with `trySend` (Go: a `select` with `default`). `Sleep(d)` calls the native `sleep(int64(d))`.
- **Zone sources**: `platformZoneSources` is Go's list without the NixOS patch's store path (`/usr/share/zoneinfo/`,
  `/usr/share/lib/zoneinfo/`, `/usr/lib/locale/TZ/`, `/etc/zoneinfo`); `zoneinfo_windows.gp` is a stub until Phase 14
  (UTC; `ZONEINFO` still works). `sys.gp` has no build tag (the natives are standard C++).
