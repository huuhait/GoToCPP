# The internal packages (step 9.2)

The packages that the ported standard library imports and that only it may import, built for step 9.4 (ADR-045). Go's
source is go1.25.14 (the version `tools/portstd` pins); each file keeps Go's license header. A file that is Go's is
copied as it is, except for what this table says.

| Package | From Go's | Differs |
|---|---|---|
| `internal/abi` | `src/internal/abi/escape.go` | Only `NoEscape` and `Escape` (the identity: gocpp has no escape analysis). Type layouts are gocpp's descriptors. |
| `internal/bytealg` | `src/internal/bytealg/*.go` | The assembly routines (`IndexByte`, `Count`, `Equal`, `Compare`, `Index`, `LastIndexByte`, `MakeNoZero`, and the `String` forms) are `//gocpp:native` functions in `runtime/src/bytealg.cpp` and `gocpp/bytealg.hpp`; `MaxLen` is 63, `MaxBruteForce` 64, `Cutover` Go's amd64 formula (Go's amd64 values with AVX2; arm64 has 32, 16 and `4+n>>4`, which gocpp does not copy: the natives do not depend on them); the Rabin-Karp functions and `HashStr` are Go's generic code, with a helper `sameBytes` where Go converts a `[]byte` to `string` to compare (here that would copy). `MakeNoZero` allocates without zeroing, as Go does, and panics like `make` for a negative length. |
| `internal/byteorder` | `src/internal/byteorder/byteorder.go` | None. |
| `internal/cpu` | `src/internal/cpu/cpu.go` | A stub: every feature variable is false, so ported packages take their portable branch (`CacheLinePadSize` is 64, 128 on arm64). |
| `internal/godebug` | `src/internal/godebug/godebug.go` | `Setting` reads `$GODEBUG` at each call (a native `env`); no table of known settings and defaults, no metrics, no runtime hook. |
| `internal/goarch`, `internal/goos` | `src/internal/goarch`, `src/internal/goos` | `amd64` and `arm64`, `linux` and `windows` only (the targets of ADR-021); the generated `z*` files are written by hand in the same shape. |
| `internal/itoa`, `internal/oserror` | the same | None. |
| `internal/race` | `src/internal/race/norace.go` | Without `ReadObjectPC` and `WriteObjectPC` (they take an `*abi.Type`). |
| `internal/stringslite` | `src/internal/stringslite/strings.go` | None (it imports `internal/bytealg`). |
| `internal/testlog` | `src/internal/testlog` | The logger is a plain variable (`atomic.Pointer[Interface]` and `sync.Mutex` are not built yet), and the `//go:linkname` hook of the runtime is gone. |
| `runtime/cgo` | `src/runtime/cgo/handle.go` | The `sync.Map` is a native table behind a mutex (`gocpp/shims.hpp`); `Handle` values count up from 1. |
