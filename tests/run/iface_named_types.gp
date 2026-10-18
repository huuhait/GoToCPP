// phase: 05

// Named types of every kind in an interface: the dynamic type is the named type, not its
// underlying type (step 5.4).
package main

import "fmt"

type Celsius float64
type Name string
type IDs []int
type Table map[string]int
type Pt [2]int
type Op func(int) int
type Flag bool
type Handle uintptr

func (c Celsius) Kind() string { return "celsius" }
func (n Name) Kind() string    { return "name" }
func (i IDs) Kind() string     { return "ids" }
func (t Table) Kind() string   { return "table" }
func (p Pt) Kind() string      { return "pt" }
func (o Op) Kind() string      { return "op" }
func (f Flag) Kind() string    { return "flag" }
func (h Handle) Kind() string  { return "handle" }

type Kinder interface{ Kind() string }

func what(x any) string {
	switch x.(type) {
	case float64:
		return "float64"
	case string:
		return "string"
	case []int:
		return "[]int"
	case map[string]int:
		return "map[string]int"
	case [2]int:
		return "[2]int"
	case func(int) int:
		return "func(int) int"
	case bool:
		return "bool"
	case uintptr:
		return "uintptr"
	case Kinder:
		return "kinder"
	}
	return "other"
}

func main() {
	vals := []Kinder{Celsius(1), Name("n"), IDs{1}, Table{"a": 1}, Pt{1, 2}, Op(func(x int) int { return x + 1 }), Flag(true), Handle(7)}
	for _, v := range vals {
		fmt.Println(v.Kind(), what(v))
	}
	fmt.Println(what(1.5), what("s"), what([]int{1}), what(map[string]int{}), what([2]int{}), what(func(x int) int { return x }), what(true), what(uintptr(3)))
	var k Kinder = Celsius(36.6)
	c := k.(Celsius)
	fmt.Println(float64(c) == 36.6)
	_, isFloat := any(k).(float64)
	fmt.Println(isFloat)
	op := vals[5].(Op)
	fmt.Println(op(41))
}

// Output:
// celsius kinder
// name kinder
// ids kinder
// table kinder
// pt kinder
// op kinder
// flag kinder
// handle kinder
// float64 string []int map[string]int [2]int func(int) int bool uintptr
// true
// false
// 42
