// phase: 03

// int and int64 are the same C++ type in gocpp (and uint, uint64 and uintptr), but fmt names
// the Go type in its error forms: the emitter passes a type tag for int64, uint64 and uintptr,
// also inside slices, arrays, maps and pointers, struct fields and named types, and in the
// arguments of a deferred call.
package main

import "fmt"

type rec struct {
	A int64
	B uint64
	C []uintptr
	D int
}

type big int64

type bigs []uint64

func two() (int64, []uint64) { return 5, []uint64{1, 2} }

func three() (int, uint, uintptr) { return 1, 2, 3 }

func main() {
	var i int = 1
	var i64 int64 = 2
	var u uint = 3
	var u64 uint64 = 4
	var up uintptr = 5
	fmt.Printf("%s %s %s %s %s\n", i, i64, u, u64, up)
	fmt.Printf("%s %s\n", int64(6), uint64(7))
	fmt.Printf("%s %s\n", []int64{1, 2}, [2]uint64{3, 4})
	fmt.Printf("%s|%s\n", map[int64]uintptr{1: 2}, map[string][]int64{"k": {1}})
	fmt.Printf("%s %s %s %v\n", &[1]int64{9}, (*int64)(nil), (*[]uintptr)(nil), &i64 != nil)
	fmt.Printf("%s\n", rec{1, 2, []uintptr{3}, 4})
	fmt.Printf("%s %s %d\n", big(8), bigs{1}, big(9))
	fmt.Println(fmt.Sprint(two()), fmt.Sprintln(three()) == "1 2 3\n")
	fmt.Println(two())
	fmt.Print(three())
	fmt.Println()
	defer fmt.Printf("deferred %s %s %v\n", i64, []uint64{u64}, up)
	defer fmt.Println("deferred", i64, u64)
	i64 = 100
	fmt.Printf("%t %t\n", i64, []int64{})
	fmt.Printf("%q\n", map[uint64]int64{1: 1})
}

// Output:
// %!s(int=1) %!s(int64=2) %!s(uint=3) %!s(uint64=4) %!s(uintptr=5)
// %!s(int64=6) %!s(uint64=7)
// [%!s(int64=1) %!s(int64=2)] [%!s(uint64=3) %!s(uint64=4)]
// map[%!s(int64=1):%!s(uintptr=2)]|map[k:[%!s(int64=1)]]
// &[%!s(int64=9)] %!s(*int64=<nil>) %!s(*[]uintptr=<nil>) true
// {%!s(int64=1) %!s(uint64=2) [%!s(uintptr=3)] %!s(int=4)}
// %!s(main.big=8) [%!s(uint64=1)] 9
// 5 [1 2] true
// 5 [1 2]
// 1 2 3
// %!t(int64=100) []
// map['\x01':'\x01']
// deferred 2 4
// deferred %!s(int64=2) [%!s(uint64=4)] 5
