// phase: 05

// The text of the panic for comparing two interface values of an uncomparable dynamic type
// (step 5.6), for every kind, named and unnamed, and when it is not raised: different dynamic
// types, one nil operand, equal pointers, or a prefix of the struct that already differs.
package main

import "fmt"

type S struct{ v []int }
type M map[string]bool
type F func(int) int
type A [2]F
type N struct {
	a int
	b A
}
type Wrap struct{ inner any }
type Ptr *[]int

func try(name string, x, y any) {
	defer func() {
		if r := recover(); r != nil {
			e := r.(error)
			fmt.Println(name, "->", e.Error())
		}
	}()
	fmt.Println(name, x == y, x != y)
}

func main() {
	try("slice", []int{1}, []int{1})
	try("slice of string", []string{}, []string{})
	try("map", map[int]int{}, map[int]int{})
	try("named map", M{}, M{})
	try("func", func() {}, func() {})
	try("named func", F(nil), F(nil))
	try("struct", S{}, S{})
	try("array of func", A{}, A{})
	try("struct with array of func", N{}, N{})
	try("wrapped", Wrap{[]int{}}, Wrap{[]int{}})
	try("wrapped nested", Wrap{Wrap{M{}}}, Wrap{Wrap{M{}}})
	try("wrapped different first", Wrap{1}, Wrap{[]int{}})
	try("pointer to slice", &[]int{}, &[]int{})
	try("named pointer", Ptr(nil), Ptr(nil))
	try("different types", []int{1}, []string{"a"})
	try("slice vs nil", []int{1}, nil)
	try("nil vs map", nil, map[int]int{})
	try("slice vs int", []int{1}, 1)
	try("struct different field first", N{1, A{}}, N{2, A{}})
	try("same pointer", &S{}, &S{})
	try("same func pointer value", &[]F{}, &[]F{})
	var e1, e2 error
	try("nil errors", e1, e2)
}

// Output:
// slice -> runtime error: comparing uncomparable type []int
// slice of string -> runtime error: comparing uncomparable type []string
// map -> runtime error: comparing uncomparable type map[int]int
// named map -> runtime error: comparing uncomparable type main.M
// func -> runtime error: comparing uncomparable type func()
// named func -> runtime error: comparing uncomparable type main.F
// struct -> runtime error: comparing uncomparable type main.S
// array of func -> runtime error: comparing uncomparable type main.A
// struct with array of func -> runtime error: comparing uncomparable type main.N
// wrapped -> runtime error: comparing uncomparable type []int
// wrapped nested -> runtime error: comparing uncomparable type main.M
// wrapped different first false true
// pointer to slice false true
// named pointer true false
// different types false true
// slice vs nil false true
// nil vs map false true
// slice vs int false true
// struct different field first -> runtime error: comparing uncomparable type main.N
// same pointer false true
// same func pointer value false true
// nil errors true false
