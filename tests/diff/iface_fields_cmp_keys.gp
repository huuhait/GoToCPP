// phase: 05

// Interfaces in struct and array fields: comparison, map keys, unhashable and uncomparable values, NaN keys.
package main

import "fmt"

type Key struct {
	A any
	B string
}
type K2 struct {
	E error
	N [2]any
}
type Opaque struct{ f func() }

func main() {
	m := map[Key]int{}
	m[Key{1, "a"}] = 1
	m[Key{int64(1), "a"}] = 2
	m[Key{"s", "a"}] = 3
	m[Key{nil, "a"}] = 4
	m[Key{Key{2, "x"}, "n"}] = 5
	fmt.Println(len(m), m[Key{1, "a"}], m[Key{int64(1), "a"}], m[Key{nil, "a"}], m[Key{Key{2, "x"}, "n"}], m[Key{Key{3, "x"}, "n"}])
	m2 := map[K2]string{}
	e := fmt.Errorf("x")
	m2[K2{e, [2]any{1, "a"}}] = "one"
	m2[K2{nil, [2]any{}}] = "two"
	fmt.Println(m2[K2{e, [2]any{1, "a"}}], m2[K2{nil, [2]any{nil, nil}}], m2[K2{fmt.Errorf("x"), [2]any{1, "a"}}] == "")
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		m[Key{[]int{1}, "bad"}] = 1
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		_ = m[Key{map[string]int{}, "bad"}]
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		a, b := Key{[]int{1}, "x"}, Key{[]int{1}, "x"}
		fmt.Println(a == b)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		a, b := Key{1, "x"}, Key{[]int{1}, "x"}
		fmt.Println(a == b)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		a, b := [2]any{1, []int{}}, [2]any{2, []int{}}
		fmt.Println(a == b)
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		a, b := [2]any{1, []int{}}, [2]any{1, []int{}}
		fmt.Println(a == b)
	}()
	var x, y any = Opaque{}, Opaque{}
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		fmt.Println(x == y)
	}()
	var p, q any = &Opaque{}, &Opaque{}
	fmt.Println(p == q, p == p)
	var i1, i2 any = 1, 1.0
	var i3, i4 any = int8(1), int16(1)
	fmt.Println(i1 == i2, i3 == i4, i1 == 1, i2 == 1.0, i3 == int8(1))
	type E1 struct{}
	var z1, z2 any = E1{}, struct{}{}
	fmt.Println(z1 == z2, z1 == E1{})
	nan := 0.0
	nan = nan / nan
	var n1, n2 any = nan, nan
	fmt.Println(n1 == n2, n1 == n1)
	mm := map[any]int{nan: 1}
	mm[nan] = 2
	fmt.Println(len(mm), mm[nan])
	delete(mm, nan)
	fmt.Println(len(mm))
	var s1, s2 any = "ab", "a" + string(rune('b'))
	fmt.Println(s1 == s2)
	var ptrs = map[any]string{&s1: "ps1", 'x': "rune"}
	ptrs[int32(120)] = "int32"
	ptrs[byte(120)] = "byte"
	fmt.Println(len(ptrs), ptrs['x'], ptrs[byte(120)])
}
