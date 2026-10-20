# time

Ported by hand from go1.25.14's `src/time` (`format.go`, `format_rfc3339.go`, `sleep.go`, `tick.go`, `time.go`,
`zoneinfo.go`, `zoneinfo_read.go`, `zoneinfo_unix.go`, `sys_unix.go`; BSD-3-Clause, see `../LICENSE`) in Phase 08 step 8.8
(ADR-041). Phase 09's `tools/portstd` will take over the port; these are the changes it must keep.

- **No generics** (Phase 07): `atoi`, `isDigit`, `parseNanoseconds`, `leadingInt` and `parseRFC3339` of Go are generic over
  `[]byte | string`; here they take a `string`. `parseStrictRFC3339(b string)` is called with `string(data)` by
  `UnmarshalJSON` and `UnmarshalText`.
- **No `unsafe`, `linkname`, `syscall`, `runtime`, `internal/*`, `math/bits`** (`sync` since step 9.10): the `//go:linkname` lines and the legacy
  `legacyTimeTimeAbs`, `legacyAbsClock`, `legacyAbsDate` routines are removed; `bits.Mul32` is `mul32`; `stringslite.Clone`
  is dropped (a string is a value); `bytealg.IndexByte` is a loop; the synctest bubble (`runtimeIsBubbled`) and
  `GODEBUG asynctimerchan` are removed (Go 1.23's behavior only); the `time/tzdata` hook
  (`registerLoadFromEmbeddedTZData`) and the `$GOROOT/lib/time/zoneinfo.zip` source and the AIX workaround are removed; Go's variable
  `loadFromEmbeddedTZData` and the function `gorootZoneSource` stay (nil, and unused by `LoadLocation`), and `interrupt` does nothing, because Go's `export_test.go` names them (step 9.10).
  The three once-only initializations (`localLoc`, the unnamed fixed zones, `ZONEINFO`) are `sync.Once` as in Go; the native `callOnce` that replaced them before 9.10 is still in `time.hpp` and unused.
- **Natives** (`//gocpp:native`, ADR-027, `runtime/include/gocpp/time.hpp`): `now`, `runtimeNano`, `sleep`, `startChan`,
  `startFunc`, `stopTimer`, `resetTimer`, `trySend` (timers, `native.gp` and `sleep.gp`), `getenv`, and the file functions of `sys.gp` (`openFile`, `readFd`, `closeFd`, `seekFd`, `errnoText`;
  Go's `sys_unix.go` calls `syscall`). `errENOENT` is `syscall.ENOENT`.
- **Timers** (`sleep.gp`, `tick.gp`): `Timer` is `{C, r, initTimer}` and `Ticker` `{C, r, initTicker}`, where `r` is the channel
  object that carries the timer record (Go's runtime allocates a bigger struct and converts with `unsafe`). `newTimer`,
  `stopTimer` and `resetTimer` of Go's runtime are `startChan` or `startFunc`, `stopTimer` and `resetTimer` here. `sendTime` takes
  the channel, not an `any`, and sends with `trySend` (Go: a `select` with `default`). `Sleep(d)` calls the native `sleep(int64(d))`.
- **Zone sources**: `platformZoneSources` is Go's list without the NixOS patch's store path (`/usr/share/zoneinfo/`,
  `/usr/share/lib/zoneinfo/`, `/usr/lib/locale/TZ/`, `/etc/zoneinfo`); `zoneinfo_windows.gp` still uses UTC for `Local`,
  with no Windows registry zone discovery (`ZONEINFO` works for named zones). `sys.gp` has no build tag
  (the natives are standard C++).
