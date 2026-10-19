// phase: 07

// Compound assignment and increment on type parameters, also through pointers, indexes and fields.
package main

import "fmt"

type Int interface {
	~int8 | ~uint8 | ~int16 | ~uint32 | ~int64 | ~uint64
}

type Acc struct{ a, b int8 }

func All[T Int](x, y T) []T {
	var out []T
	z := x
	z += y
	out = append(out, z)
	z = x
	z -= y
	out = append(out, z)
	z = x
	z *= y
	out = append(out, z)
	z = x
	z /= y
	out = append(out, z)
	z = x
	z %= y
	out = append(out, z)
	z = x
	z &= y
	out = append(out, z)
	z = x
	z |= y
	out = append(out, z)
	z = x
	z ^= y
	out = append(out, z)
	z = x
	z &^= y
	out = append(out, z)
	z = x
	z <<= 3
	out = append(out, z)
	z = x
	z >>= 2
	out = append(out, z)
	z = x
	z++
	z++
	z--
	out = append(out, z)
	return out
}

func ThroughPtr[T Int](p *T, d T) {
	*p += d
	*p *= 2
	(*p)++
}

func InSlice[S ~[]T, T Int](s S, i int, d T) {
	s[i] += d
	s[i]++
	s[i] <<= 1
}

func ViaMap[K comparable, T Int](m map[K]T, k K, d T) {
	m[k] += d
	m[k]--
}

func main() {
	fmt.Println(All[int8](100, 30))
	fmt.Println(All[uint8](200, 30))
	fmt.Println(All[int16](-30000, 7))
	fmt.Println(All[uint32](4000000000, 5))
	fmt.Println(All[int64](-1<<62, 3))
	fmt.Println(All[uint64](1<<63, 9))
	x := int8(60)
	ThroughPtr(&x, 20)
	u := uint8(130)
	ThroughPtr(&u, 5)
	s := []uint8{250, 1}
	InSlice(s, 0, 10)
	InSlice(s, 1, 100)
	m := map[string]int8{"a": 127}
	ViaMap(m, "a", 5)
	ViaMap(m, "new", -128)
	acc := Acc{120, 7}
	ThroughPtr(&acc.a, 10)
	ThroughPtr(&acc.b, 100)
	fmt.Println(x, u, s, m, acc)
	var c uint8 = 250
	n := 0
	for ; c >= 250 || c < 5; c++ {
		n++
	}
	fmt.Println(c, n)
}

// Output:
// [-126 70 -72 3 10 4 126 122 96 32 25 101]
// [230 170 112 6 20 8 222 214 192 64 50 201]
// [-29993 -30007 -13392 -4285 -5 0 -29993 -29993 -30000 22144 -7500 -29999]
// [4000000005 3999999995 2820130816 800000000 0 0 4000000005 4000000005 4000000000 1935228928 1000000000 4000000001]
// [-4611686018427387901 -4611686018427387907 4611686018427387904 -1537228672809129301 -1 0 -4611686018427387901 -4611686018427387901 -4611686018427387904 0 -1152921504606846976 -4611686018427387903]
// [9223372036854775817 9223372036854775799 9223372036854775808 1024819115206086200 8 0 9223372036854775817 9223372036854775817 9223372036854775808 0 2305843009213693952 9223372036854775809]
// -95 15 [10 204] map[a:-125 new:127] {5 -41}
// 5 11
