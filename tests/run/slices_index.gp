// phase: 03

// s[i] reads and writes the element in the slice's array: plain stores, op=, ++ and -- (with
// Go's wrapping for the small integer types), on numbers, bools, strings and pointers, also
// through a slice that shares the array, and parallel assignments (a swap).
package main

import "fmt"

func main() {
	s := []int{10, 20, 30}
	t := s
	s[0] = 1
	t[1] += 5
	s[2]++
	s[2] *= 3
	fmt.Println(s[0], s[1], s[2], t[0], t[1], t[2])
	s[0], s[2] = s[2], s[0]
	fmt.Println(s[0], s[1], s[2])

	b := []byte{250, 5}
	b[0] += 10
	b[1] -= 6
	b[0]++
	fmt.Println(b[0], b[1])
	i8 := []int8{127, -128}
	i8[0]++
	i8[1]--
	fmt.Println(i8[0], i8[1])
	u16 := []uint16{1}
	u16[0] <<= 15
	u16[0] *= 2
	fmt.Println(u16[0])

	strs := []string{"go", "cpp"}
	strs[0] += strs[1]
	strs[1] = strs[0][1:3]
	fmt.Println(strs[0], strs[1], len(strs[0]))

	flags := make([]bool, 3)
	flags[1] = true
	flags[2] = !flags[1] || flags[0]
	fmt.Println(flags[0], flags[1], flags[2])

	x, y := 1, 2
	ps := []*int{&x, &y}
	*ps[0] = 5
	ps[1] = ps[0]
	*ps[1] += 10
	fmt.Println(x, y, ps[0] == ps[1])

	var idx uint8 = 1
	var i64 int64 = 2
	s[idx] = 100
	s[i64] = 200
	fmt.Println(s[idx], s[i64], s[1.0], s['\x00'])
	for i := range s {
		s[i] = s[i] / 10
	}
	fmt.Println(s[0], s[1], s[2])
}

// Output:
// 1 25 93 1 25 93
// 93 25 1
// 5 255
// -128 127
// 0
// gocpp oc 5
// false true false
// 15 2 true
// 100 200 100 93
// 9 10 20
