// phase: 05

// The values that the gdb printers show (runtime/debug/gocpp-gdb.py): every kind of Go value,
// printed at the stop in show and in zero. values.gdb has the expected text.
package main

import (
	"errors"
	"fmt"
)

type Point struct {
	X, Y int
}

type Inner struct {
	A int8
	B uint16
}

type Outer struct {
	Inner
	Name  string
	new   int
	class float64
	Ptr   *Point
	Any   any
}

type Celsius float64
type IDs []int
type Color int
type Counts map[string]int

const (
	Red Color = iota
	Green
)

type Node struct {
	Val  int
	Next *Node
}

type Shape interface {
	Area() int
}

func (p Point) Area() int { return p.X * p.Y }

type Sq struct{ S int }

func (s *Sq) Area() int { return s.S * s.S }

type Stringer interface{ String() string }

func (c Color) String() string { return "color" }

func add(a, b int) int { return a + b }

func zero() {
	var s string
	var sl []int
	var m map[string]int
	var p *Point
	var a any
	var e error
	var f func()
	var pt Point
	var arr [2]int
	var o Outer
	var sh Shape
	fmt.Println(s, sl, m, p, a, e, f == nil, pt, arr, o, sh) // @zero
}

func show(n int, s string) {
	i8, u8, i16, u16, i32, u32, i64, u64 := int8(-5), uint8(200), int16(-300), uint16(60000), int32(-70000), uint32(4000000000), int64(-1<<40), uint64(1<<63)
	f32, f64 := float32(0.1), 2.25
	b, r, by := true, 'x', byte('y')
	c64, c128 := complex64(complex(1, 2)), complex(3.5, -4)
	empty, hello, uni, esc := "", "hello", "héllo, 世界", "a\"b\\c\n\t"
	long := "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"
	var nilSlice []int
	emptySlice := []int{}
	ints := []int{1, 2, 3}
	strs := []string{"a", "bb"}
	nested := [][]int{{1}, {2, 3}}
	pts := []Point{{1, 2}, {3, 4}}
	bs := []byte("hi")
	sub := ints[1:2]
	arr := [3]int{7, 8, 9}
	arr2 := [2][2]int{{1, 2}, {3, 4}}
	var nilMap map[string]int
	emptyMap := map[string]int{}
	m1 := map[string]int{"b": 2, "a": 1}
	m2 := map[int]string{3: "c", 1: "a"}
	m3 := map[string][]int{"x": {1, 2}}
	m4 := map[string]Point{"p": {5, 6}}
	var nilPtr *Point
	pi := new(int)
	*pi = 42
	pp := &Point{7, 8}
	list := &Node{1, &Node{2, &Node{3, nil}}}
	cyc := &Node{Val: 9}
	cyc.Next = cyc
	var nilAny any
	anyInt, anyStr, anyPt, anyPtr := any(42), any("s"), any(Point{1, 2}), any(pp)
	anyF, anyB := any(1.5), any(true)
	err := errors.New("boom")
	var sh Shape = Point{2, 3}
	var sh2 Shape = &Sq{4}
	var st Stringer = Red
	var nilFn func(int) int
	fn := add
	k := 10
	clo := func(x int) int { return x + k }
	temp := Celsius(36.6)
	ids := IDs{4, 5}
	col := Green
	cnt := Counts{"z": 26}
	out := Outer{Inner{-1, 2}, "out", 3, 4.5, pp, 5}
	anon := struct {
		A int
		B string
	}{1, "x"}
	fmt.Println(n, s, i8, u8, i16, u16, i32, u32, i64, u64, f32, f64, b, r, by, c64, c128)
	fmt.Println(empty, hello, uni, esc, long, nilSlice, emptySlice, ints, strs, nested, pts, bs, sub, arr, arr2)
	fmt.Println(nilMap, emptyMap, m1, m2, m3, m4, nilPtr, pi, pp, list, cyc, nilAny, anyInt, anyStr, anyPt, anyPtr, anyF, anyB, err)
	fmt.Println(sh, sh2, st, nilFn == nil, fn(1, 2), clo(1), temp, ids, col, cnt, out, anon) // @show
}

func main() {
	zero()
	show(1, "one")
}
