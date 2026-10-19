# math

Ported by `tools/portstd` from go1.25.14's `src/math` (BSD-3-Clause, see `../LICENSE`). Do not edit the `.gp` files by hand: the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. A change goes into a rule of `tools/portstd/patches/math.rules` (the file is created for the first rule; every rule has a reason), or into the tool.

| | |
|---|---|
| Source | go1.25.14, `go1.25.14.src.tar.gz` (SHA-256 of the tarball `9e83f44f5fc297378861b4e16cc6aa114be8add7993fb3ceb2c512380aa4d582`) |
| Systems (GOOS) | linux |
| Architecture (GOARCH) | `generic`: no architecture-specific file, so no assembly variant |
| Build tags | `purego` |
| Regenerate | `go run ./tools/portstd math` |

## Files taken

- `abs.gp`, from `abs.go`
- `acosh.gp`, from `acosh.go`
- `asin.gp`, from `asin.go`
- `asinh.gp`, from `asinh.go`
- `atan.gp`, from `atan.go`
- `atan2.gp`, from `atan2.go`
- `atanh.gp`, from `atanh.go`
- `bits.gp`, from `bits.go`
- `cbrt.gp`, from `cbrt.go`
- `const.gp`, from `const.go`
- `copysign.gp`, from `copysign.go`
- `dim.gp`, from `dim.go`
- `dim_noasm.gp`, from `dim_noasm.go`
- `erf.gp`, from `erf.go`
- `erfinv.gp`, from `erfinv.go`
- `exp.gp`, from `exp.go`
- `exp2_noasm.gp`, from `exp2_noasm.go`
- `exp_noasm.gp`, from `exp_noasm.go`
- `expm1.gp`, from `expm1.go`
- `floor.gp`, from `floor.go`
- `floor_noasm.gp`, from `floor_noasm.go`
- `fma.gp`, from `fma.go`
- `frexp.gp`, from `frexp.go`
- `gamma.gp`, from `gamma.go`
- `hypot.gp`, from `hypot.go`
- `hypot_noasm.gp`, from `hypot_noasm.go`
- `j0.gp`, from `j0.go`
- `j1.gp`, from `j1.go`
- `jn.gp`, from `jn.go`
- `ldexp.gp`, from `ldexp.go`
- `lgamma.gp`, from `lgamma.go`
- `log.gp`, from `log.go`
- `log10.gp`, from `log10.go`
- `log1p.gp`, from `log1p.go`
- `log_stub.gp`, from `log_stub.go`
- `logb.gp`, from `logb.go`
- `mod.gp`, from `mod.go`
- `modf.gp`, from `modf.go`
- `modf_noasm.gp`, from `modf_noasm.go`
- `nextafter.gp`, from `nextafter.go`
- `pow.gp`, from `pow.go`
- `pow10.gp`, from `pow10.go`
- `remainder.gp`, from `remainder.go`
- `signbit.gp`, from `signbit.go`
- `sin.gp`, from `sin.go`
- `sincos.gp`, from `sincos.go`
- `sinh.gp`, from `sinh.go`
- `sqrt.gp`, from `sqrt.go`
- `stubs.gp`, from `stubs.go`
- `tan.gp`, from `tan.go`
- `tanh.gp`, from `tanh.go`
- `trig_reduce.gp`, from `trig_reduce.go`
- `unsafe.gp`, from `unsafe.go`

## Files not taken

- `acos_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `acosh_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `arith_s390x.go`: the file name excludes GOOS=linux GOARCH=generic
- `asin_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `asinh_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `atan2_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `atan_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `atanh_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `cbrt_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `cosh_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `dim_amd64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `dim_arm64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `dim_asm.go`: //go:build amd64 || arm64 || loong64 || riscv64 || s390x is false
- `dim_loong64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `dim_riscv64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `dim_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `erf_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `erfc_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `exp2_asm.go`: //go:build arm64 || loong64 is false
- `exp_amd64.go`: the file name excludes GOOS=linux GOARCH=generic
- `exp_amd64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `exp_arm64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `exp_asm.go`: //go:build amd64 || arm64 || loong64 || s390x is false
- `exp_loong64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `exp_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `expm1_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `floor_386.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `floor_amd64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `floor_arm64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `floor_asm.go`: //go:build 386 || amd64 || arm64 || loong64 || ppc64 || ppc64le || riscv64 || s390x || wasm is false
- `floor_loong64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `floor_ppc64x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `floor_riscv64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `floor_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `floor_wasm.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `hypot_386.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `hypot_amd64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `hypot_asm.go`: //go:build 386 || amd64 is false
- `log10_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `log1p_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `log_amd64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `log_asm.go`: //go:build amd64 || s390x is false
- `log_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `modf_arm64.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `modf_asm.go`: //go:build arm64 || ppc64 || ppc64le is false
- `modf_ppc64x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `pow_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `sin_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `sinh_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `stubs_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `tan_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- `tanh_s390x.s`: assembly: gocpp has no Go assembly, the pure-Go variant is taken
- 5 `_test.go` files: Go's tests of the package are run by step 9.10, not copied here

## Patches

- `abs.go`: Abs is native: clearing the sign bit of x is one instruction (std::fabs), and it keeps NaN payloads like Go's.
- `exp_noasm.go`: Exp is Go's amd64 assembly (Shibata's algorithm, with FMA instructions when the CPU has them), not the pure-Go exp: the two differ in the last bit for many inputs.
- `floor.go` (3 places): Floor, Ceil and Trunc are native, directly (not through archFloor and the other stubs, so that no call stays between the program and the instruction): amd64 has them as assembly (SSE4.1 ROUNDSD), and the results are exact, so gp::math::floor, ceil and trunc (ROUNDSD itself when the CPU has it) give the same bits, also for a signaling NaN. The Go code (floor, ceil, trunc) stays for Go's tests.
- `floor.go`: RoundToEven is native: gc turns it into ROUNDSD (an intrinsic with a runtime check for SSE4.1) on amd64, which turns a signaling NaN into a quiet one, as std::nearbyint does; the Go code below returns the signaling NaN as it is, so the bits would differ for that input. The Go version stays for Go's tests.
- `fma.go`: FMA is native: gc turns math.FMA into the VFMADD231SD instruction when the CPU has it (a runtime check), and std::fma is the same exactly rounded operation, so the bits are the same; the software version below stays for Go's tests.
- `log_stub.go`: Log is Go's amd64 assembly, not the pure-Go log: the same algorithm, but the assembly takes the exponent from the bits of x without normalizing a subnormal x, so Log of a subnormal number is wrong in Go on amd64, and gocpp reproduces it (@note above).
- `sqrt.go`: Sqrt is native: gc turns math.Sqrt into the SQRTSD instruction (an intrinsic), whose result is the correctly rounded square root, as std::sqrt's is; the pure-Go sqrt below stays (Go's tests use it).
- `unsafe.go` (5 places): The bit casts are native: `*(*uint64)(unsafe.Pointer(&f))` would make gocpp allocate f on the heap (unsafe.Pointer keeps memory alive, divergence D16), and every other function of the package calls these.

## Native functions

| Function | Go linkname | Native |
|---|---|---|
| `Abs` (`abs.gp`) | (none: no body upstream) | `//gocpp:native gp::math::abs` |
| `archExp` (`exp_noasm.gp`) | (none: no body upstream) | `//gocpp:native gp::math::exp` |
| `Ceil` (`floor.gp`) | (none: no body upstream) | `//gocpp:native gp::math::ceil` |
| `Floor` (`floor.gp`) | (none: no body upstream) | `//gocpp:native gp::math::floor` |
| `RoundToEven` (`floor.gp`) | (none: no body upstream) | `//gocpp:native gp::math::round_to_even` |
| `Trunc` (`floor.gp`) | (none: no body upstream) | `//gocpp:native gp::math::trunc` |
| `FMA` (`fma.gp`) | (none: no body upstream) | `//gocpp:native gp::math::fma` |
| `archLog` (`log_stub.gp`) | (none: no body upstream) | `//gocpp:native gp::math::log` |
| `Sqrt` (`sqrt.gp`) | (none: no body upstream) | `//gocpp:native gp::math::sqrt` |
| `Float32bits` (`unsafe.gp`) | (none: no body upstream) | `//gocpp:native gp::math::float32bits` |
| `Float32frombits` (`unsafe.gp`) | (none: no body upstream) | `//gocpp:native gp::math::float32frombits` |
| `Float64bits` (`unsafe.gp`) | (none: no body upstream) | `//gocpp:native gp::math::float64bits` |
| `Float64frombits` (`unsafe.gp`) | (none: no body upstream) | `//gocpp:native gp::math::float64frombits` |

## Findings

None.

## Known divergences from Go

- The results are Go's on amd64 (GOAMD64=v1): Floor, Ceil, Trunc, Sqrt, Abs, FMA and the Float bit functions are exact (native); Exp and Log run the algorithms of Go's amd64 assembly (runtime/src/math.cpp), including its two quirks: Exp uses the FMA instructions when the CPU has AVX and FMA (as Go does; `GODEBUG=cpu.fma=off` and `cpu.avx=off` select the other path, as in Go), and Log of a subnormal number is wrong in Go on amd64 (Log(5e-324) is -709.08956571282, not -744.44): gocpp has the same value. On arm64 Go runs other code for Exp and Log; gocpp gives the amd64 results there too.
- FMA and the other operations are never fused by the C++ compiler (`-ffp-contract=off`, ADR-006), as in gc on amd64 with GOAMD64=v1 or v2; Go's own use of `math.FMA` is the only fusion.
