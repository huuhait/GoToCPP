// phase: 05

// Every kind of dynamic type as a map[any] key (step 5.6): the same value of two different
// types is two keys, equal values of one type are one key, the zero values are keys, many keys
// of one kind do not collide into a slow map (the count is checked, not the time).
package main

import "fmt"

type Named int
type Str string
type P struct {
	x, y int
}
type Q struct {
	s string
	p *int
	f float64
	b bool
}
type Arr [3]int
type Nest struct {
	p P
	a Arr
}
type Z struct{}
type E [0]int

func main() {
	m := map[any]string{}
	one := 1
	add := func(k any, v string) { m[k] = v }
	add(1, "int")
	add(int8(1), "int8")
	add(int16(1), "int16")
	add(int32(1), "int32")
	add(int64(1), "int64")
	add(uint(1), "uint")
	add(uint8(1), "uint8")
	add(uint16(1), "uint16")
	add(uint32(1), "uint32")
	add(uint64(1), "uint64")
	add(uintptr(1), "uintptr")
	add(float32(1), "float32")
	add(float64(1), "float64")
	add(complex64(1), "complex64")
	add(complex128(1), "complex128")
	add(true, "bool")
	add("1", "string")
	add(Named(1), "Named")
	add(Str("1"), "Str")
	add(&one, "ptr")
	add(P{1, 1}, "P")
	add(Arr{1, 1, 1}, "Arr")
	add([3]int{1, 1, 1}, "[3]int")
	add([2]int{1, 1}, "[2]int")
	add(Nest{P{1, 1}, Arr{1}}, "Nest")
	add(Z{}, "Z")
	add(E{}, "E")
	add([0]string{}, "[0]string")
	add(struct{}{}, "struct{}")
	add(nil, "nil")
	add('1', "rune")
	add(byte(1), "byte")
	fmt.Println(len(m))
	// rune is int32 and byte is uint8: they replaced int32(1) and uint8(1).
	fmt.Println(m[int32(1)], m[uint8(1)], m[1], m[int64(1)], m[1.0], m[float32(1)])
	fmt.Println(m[Named(1)], m[Str("1")], m["1"], m[P{1, 1}], m[Arr{1, 1, 1}], m[[3]int{1, 1, 1}], m[[2]int{1, 1}])
	fmt.Println(m[Nest{P{1, 1}, Arr{1}}], m[Nest{P{1, 1}, Arr{2}}] == "", m[Z{}], m[E{}], m[[0]string{}], m[struct{}{}], m[nil], m[&one], m[new(int)] == "")

	// Zero values are keys of their own types.
	zeros := map[any]int{}
	for i, z := range []any{0, int8(0), uint(0), 0.0, float32(0), "", false, nil, P{}, Arr{}, Q{}, Nest{}, complex(0, 0), (*int)(nil), Named(0), Str("")} {
		zeros[z] = i
	}
	fmt.Println(len(zeros), zeros[0], zeros[""], zeros[false], zeros[nil], zeros[Q{}], zeros[(*int)(nil)], zeros[0.0], zeros[Str("")])

	// Structs with several field kinds, one differing field is a different key.
	qs := map[any]int{}
	q := Q{"a", &one, 1.5, true}
	qs[q] = 1
	q2 := q
	q2.b = false
	qs[q2] = 2
	q3 := q
	q3.f = 2.5
	qs[q3] = 3
	q4 := q
	q4.p = nil
	qs[q4] = 4
	q5 := q
	qs[q5] += 10
	fmt.Println(len(qs), qs[q], qs[q2], qs[q3], qs[q4])

	// Many keys of every kind: found again, and the count is exact.
	big := map[any]int{}
	const n = 3000
	for i := 0; i < n; i++ {
		big[i] = i
		big[int64(i)] = i
		big[uint16(i)] = i
		big[float64(i)/4] = i
		big[Str(fmt.Sprint(i))] = i
		big[P{i, -i}] = i
		big[Arr{i, i / 2, i / 3}] = i
		big[complex(float64(i), 1)] = i
		big[Named(i*8)] = i
	}
	bad := 0
	for i := 0; i < n; i++ {
		if big[i] != i || big[int64(i)] != i || big[uint16(i)] != i&0xffff && i < 65536 || big[float64(i)/4] != i ||
			big[Str(fmt.Sprint(i))] != i || big[P{i, -i}] != i || big[Arr{i, i / 2, i / 3}] != i || big[complex(float64(i), 1)] != i || big[Named(i*8)] != i {
			bad++
		}
	}
	fmt.Println(len(big), bad)
}

// Output:
// 31
// int32 byte int int64 float64 float32
// Named Str string P Arr [3]int [2]int
// Nest true Z E [0]string struct{} nil ptr true
// 16 0 5 6 7 10 13 3 15
// 4 11 2 3 4
// 27000 0
