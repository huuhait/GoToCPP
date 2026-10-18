// phase: 06

// Constants of another package: untyped and typed, of named types, strings with NUL bytes and Unicode, a constant that is no float64, one that is no int64 (no C++ constant for these two in pkg.hpp: the header's inline constexpr variables are for C++ users, the generated code folds every use).

package main

import (
	"fmt"

	"example.com/k/k"
)

func main() {
	fmt.Println(k.Huge/k.Big > 0, k.Small, k.Pi2, k.F32, k.U8, k.Neg, uint64(k.Mask), len(k.Str), k.Ch, k.Flag, k.Cplx, k.Sh)
	fmt.Println(k.Dur.Millis(), k.Unicode, k.Ratio(), k.Second)
	fmt.Printf("%q\n", k.Str)
}

// Output:
// true 1e-300 6.283185307179586 0.1 200 -128 18446744073709551615 25 120 true (1+2i) 4
// 3000 héllo, 世界 0.5 1000
// "multi\nline \"quoted\" \x00 nul"
