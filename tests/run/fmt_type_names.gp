// phase: 05

// %T names the dynamic type as reflect does: predeclared and named types, composite types, funcs,
// channels-free interface types, pointers, anonymous structs and interfaces.
package main

import "fmt"

type Celsius float64

type Pt struct{ X int }

type List []Pt

type Dict map[string]Pt

type Fn func(int) string

type Arr [3]int

type Ptr *Pt

type Iface interface{ M() }

type Gen struct{}

func main() {
	var i Iface
	var e error
	var st fmt.Stringer
	var ptrptr **Pt
	vals := []any{
		1, int8(1), uint(1), uintptr(1), 1.5, float32(1), complex(1, 1), complex64(1), "s", true, 'r', byte(1),
		Celsius(1), Pt{}, &Pt{}, List{}, Dict{}, Arr{}, Fn(nil), Ptr(nil), []int{}, []*Pt{}, [][]string{}, [2][3]bool{},
		map[string]int{}, map[Pt][]Celsius{}, map[any]error{}, func() {}, func(int, string) (bool, error) { return false, nil },
		func(...int) {}, struct{}{}, struct{ A int }{}, struct {
			A int `k:"v"`
			b []string
		}{}, []struct{ X, Y int }{}, ptrptr, i, e, st, &i, &e, []any{}, []error{}, [0]int{}, &[]int{}, &map[string]int{},
		new(int), new(Pt), new([]Pt), struct{ Pt }{}, struct {
			*Pt
			N int
		}{}, []interface{ M() }{}, []interface{ Error() string }{}, []fmt.Stringer{}, func(Iface) {},
		func(a, b int) (int, int) { return b, a },
	}
	n := 3
	var n64 int64 = 4
	var u uint = 5
	var u64 uint64 = 6
	var up uintptr = 7
	fmt.Printf("%T %T %T %T %T %T %T %T %T\n", n, 1, n64, u, u64, up, 'r', byte(1), int32(2))
	fmt.Printf("%T %T %T %T %T\n", 2.5, float32(1), "s", true, complex(1, 2))
	for _, v := range vals {
		fmt.Printf("%T\n", v)
	}
	fmt.Printf("%T %T %T\n", nil, vals, &vals)
}

// Output:
// int int int64 uint uint64 uintptr int32 uint8 int32
// float64 float32 string bool complex128
// int
// int8
// uint
// uintptr
// float64
// float32
// complex128
// complex64
// string
// bool
// int32
// uint8
// main.Celsius
// main.Pt
// *main.Pt
// main.List
// main.Dict
// main.Arr
// main.Fn
// main.Ptr
// []int
// []*main.Pt
// [][]string
// [2][3]bool
// map[string]int
// map[main.Pt][]main.Celsius
// map[interface {}]error
// func()
// func(int, string) (bool, error)
// func(...int)
// struct {}
// struct { A int }
// struct { A int "k:\"v\""; b []string }
// []struct { X int; Y int }
// **main.Pt
// <nil>
// <nil>
// <nil>
// *main.Iface
// *error
// []interface {}
// []error
// [0]int
// *[]int
// *map[string]int
// *int
// *main.Pt
// *[]main.Pt
// struct { main.Pt }
// struct { *main.Pt; N int }
// []interface { M() }
// []interface { Error() string }
// []fmt.Stringer
// func(main.Iface)
// func(int, int) (int, int)
// <nil> []interface {} *[]interface {}
