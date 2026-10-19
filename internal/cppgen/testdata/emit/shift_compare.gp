package main

// A shift as the operand of a comparison gets parentheses: clang warns about a << 1 < b when the
// operands are class types (-Woverloaded-shift-op-parentheses), as the operands of a named
// integer type are (found by the 15.8 fuzzer).

type N uint8

func main() {
	var a, b N = 3, 200
	var c, d int32 = 5, 40
	println(a<<1 < b, b>>1 == a, a<<2 > b, c<<2 <= d, (d >> 1) != c)
}
