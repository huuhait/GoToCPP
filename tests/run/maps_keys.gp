// phase: 03

// Key types: bools, small and large integers, floats (+0 and -0 are one key, and a store to an
// equal key keeps the new key; NaN is never found again, so each store adds an entry, which
// delete can't remove but clear does), strings (also from bytes and substrings), and pointers
// (equal when they point to the same variable).
package main

import "fmt"

func main() {
	b := map[bool]string{true: "yes", false: "no"}
	fmt.Println(b[true], b[false], b[1 > 2], len(b))

	i8 := map[int8]int{-128: 1, 127: 2}
	u64 := map[uint64]int{1<<64 - 1: 3, 0: 4}
	fmt.Println(i8[-128], i8[127], i8[0], u64[1<<64-1], u64[0], u64[5])

	zero := 0.0
	negz := -zero
	nan := zero / zero
	f := map[float64]string{}
	f[negz] = "neg"
	f[zero] = "pos"
	for k, v := range f {
		fmt.Println("key", 1/k, v, len(f))
	}
	f[nan] = "nan1"
	f[nan] = "nan2"
	_, found := f[nan]
	fmt.Println(len(f), found, f[nan] == "")
	nans := 0
	for k := range f {
		if k != k {
			nans++
		}
	}
	delete(f, nan)
	fmt.Println(nans, len(f))
	clear(f)
	fmt.Println(len(f))
	f32 := map[float32]int{0.1: 1}
	var x float32 = 0.1
	fmt.Println(f32[x], f32[0.2])

	s := map[string]int{}
	word := "hello world"
	s[word[:5]] = 1
	s[word[6:]] = 2
	bs := []byte("hello")
	fmt.Println(s["hello"], s[string(bs)], s["world"], s[""], len(s))
	s[""] = 3
	fmt.Println(s[""], len(s))

	p, q := new(int), new(int)
	r := p
	pm := map[*int]int{p: 1, q: 2}
	pm[r] += 10
	fmt.Println(pm[p], pm[q], pm[nil], len(pm))
	pm[nil] = 5
	fmt.Println(pm[nil], len(pm))
}

// Output:
// yes no no 2
// 1 2 0 3 4 0
// key +Inf pos 1
// 3 false true
// 2 3
// 0
// 1 0
// 1 1 2 0 2
// 3 3
// 11 2 0 2
// 5 3
