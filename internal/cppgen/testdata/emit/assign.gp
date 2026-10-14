// Assignments: parallel assignment (the whole right side first), x op= y for every
// operator and type class (through the operators of ops.go: gp::div, gp::shl, the
// small-integer cast), ++ and --, and _.
package main

import "fmt"

var calls = 0

func next() int {
	calls++
	return calls
}

func pair() (int, string) {
	return 7, "seven"
}

func main() {
	a, b := 1, 2
	a, b = b, a
	fmt.Println(a, b)
	i, j, k := 1, 2, 3
	i, j, k = k, i, j
	fmt.Println(i, j, k)
	i, j = i+j, i-j // both use the old i and j
	fmt.Println(i, j)
	a, b = 10, 20 // constants need no temporaries
	fmt.Println(a, b)
	a, _ = next(), next() // _ is evaluated too, in order
	fmt.Println(a, calls)
	var s string
	a, s = pair()
	fmt.Println(a, s)
	_, s = pair()
	_, _ = pair()
	_ = next()
	fmt.Println(s, calls)

	// Op-assignments on int.
	x := 100
	x += 5
	x -= 3
	x *= 2
	x /= 7
	x %= 10
	fmt.Println(x)
	x = -7
	x /= 2 // truncates toward zero
	fmt.Println(x)
	x = -7
	x %= 3
	fmt.Println(x)
	x = 6
	x &= 3
	x |= 8
	x ^= 1
	x &^= 8
	fmt.Println(x)
	x <<= 3
	fmt.Println(x)
	x >>= 1
	fmt.Println(x)
	n := 70
	x <<= n // a count of the width or more gives 0
	fmt.Println(x)
	x = -1
	x >>= n
	fmt.Println(x)
	x = 1
	n = 62
	x <<= n
	fmt.Println(x)
	x = -9223372036854775808
	d := -1
	x /= d // MIN / -1 is MIN
	fmt.Println(x)
	x = -9223372036854775808
	x %= d
	fmt.Println(x)
	x = 9223372036854775807
	x++ // wraps
	fmt.Println(x)
	x--
	fmt.Println(x)

	// The small integer types wrap like Go's.
	var u8 uint8 = 100
	u8 += 200
	fmt.Println(u8)
	u8 -= 50
	fmt.Println(u8)
	u8 *= 3
	fmt.Println(u8)
	u8 = 255
	u8++
	fmt.Println(u8)
	u8--
	fmt.Println(u8)
	u8 <<= 4
	fmt.Println(u8)
	var cnt uint8 = 9
	u8 = 1
	u8 <<= cnt
	fmt.Println(u8)
	var i8 int8 = 127
	i8++
	fmt.Println(i8)
	i8 = -128
	var m1 int8 = -1
	i8 /= m1
	fmt.Println(i8)
	i8 = 100
	i8 += 100
	fmt.Println(i8)
	i8 = -128
	i8 >>= 3
	fmt.Println(i8)
	var u16 uint16 = 65535
	u16 += 2
	fmt.Println(u16)
	u16 = 3
	u16 -= 4
	fmt.Println(u16)
	var i16 int16 = -32768
	i16--
	fmt.Println(i16)
	i16 = 300
	i16 *= 300
	fmt.Println(i16)
	var u32 uint32 = 1
	u32 -= 2
	fmt.Println(u32)
	var u64 uint64 = 5
	u64 -= 6
	fmt.Println(u64)
	u64 /= 3
	fmt.Println(u64)
	var r rune = 'a'
	r += 2
	fmt.Println(r)

	// Floats and strings.
	f := 1.5
	f += 2
	f *= 3
	f /= 4
	f -= 0.125
	fmt.Println(f)
	f++
	fmt.Println(f)
	zero := 0.0
	f /= zero
	fmt.Println(f)
	var f32 float32 = 0.1
	f32 += 0.2
	fmt.Println(f32)
	str := "go"
	str += "cpp"
	str += str
	fmt.Println(str)
	ok := true
	ok = !ok
	fmt.Println(ok)
}
