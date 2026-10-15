// phase: 03

// Package variables of every Phase 03 type: slices, arrays, maps, structs (named and
// anonymous), pointers (to a package variable, to an element of one, to a new block), named
// types, func values (a func literal, a named function, a method value and a method
// expression) and strings; their zero values (var x T); one call for two variables (var a, b =
// f()), a blank variable whose initializer still runs, and initializers that use variables
// declared after them. Writes through the pointers change the variables, and fmt prints
// the composite values as Go does.
package main

import "fmt"

type point struct {
	x, y int
}

func (p point) sum() int    { return p.x + p.y }
func (p *point) move(d int) { p.x += d }

type celsius float64
type names []string
type op func(int, int) int

var (
	sl                     = []int{1, 2, 3}
	arr                    = [3]int{4, 5, 6}
	words                  = [2]string{"a", "b"}
	table                  = map[string]int{"one": 1, "two": 2}
	pt                     = point{1, 2}
	ppt                    = &point{3, 4}
	pnew                   = new(int)
	temp           celsius = 36.6
	people                 = names{"ann", "bob"}
	square                 = func(x int) int { return x * factor }
	factor                 = 3
	greet                  = "hello" + suffix()
	anon                   = struct{ a, b int }{7, 8}
	keyed                  = [5]int{2: 9, 4: 1}
	nested                 = [][]int{{1}, {2, 3}}
	toVar                  = &factor
	toElem                 = &arr[1]
	toField                = &pt.y
	sumOf                  = pt.sum
	moveP                  = (*point).move
	add            op      = func(a, b int) int { return a + b }
	named                  = twice
	sub                    = sl[1:]
	first, second          = pair()
	_                      = note("blank variable")
	count                  = len(table) + len(sl)
	val, ok                = table["two"]
	missing, found         = table["three"]
)

var (
	zeroSl   []int
	zeroArr  [4]int
	zeroMap  map[string]bool
	zeroPt   point
	zeroPtr  *point
	zeroFn   func()
	zeroStr  string
	zeroTemp celsius
)

func suffix() string { return note(" world") }

func note(s string) string {
	fmt.Println("note:", s)
	return s
}

func pair() (int, string) { return 10, "ten" }

func twice(n int) int { return 2 * n }

func main() {
	fmt.Println(len(sl), sl[2], arr[0], words[1], table["one"], pt.x, ppt.y, *pnew, temp, people[1])
	fmt.Println(square(2), greet, anon.b, keyed[2], keyed[4], nested[1][1], *toVar, *toElem, *toField)
	fmt.Println(sumOf(), add(1, 2), named(21), len(sub), sub[0], first, second, count, val, ok, missing, found)
	fmt.Println(zeroSl == nil, zeroArr[3], zeroMap == nil, zeroPt.x, zeroPtr == nil, zeroFn == nil, zeroStr == "", zeroTemp)

	factor = 4
	arr[1] = 50
	pt.y = 20
	*pnew = 7
	moveP(ppt, 10)
	fmt.Println(square(2), *toVar, *toElem, *toField, *pnew, ppt.x, sumOf())
	*toVar = 5
	fmt.Println(factor, square(1))
	fmt.Println(sl, arr, words, table, pt, ppt, people, anon, keyed, nested, sub)
	fmt.Println(zeroSl, zeroArr, zeroMap, zeroPt, zeroPtr, zeroStr == "", zeroTemp)
}

// Output:
// note:  world
// note: blank variable
// 3 3 4 b 1 1 4 0 36.6 bob
// 6 hello world 8 9 1 3 3 5 2
// 3 3 42 2 2 10 ten 5 2 true 0 false
// true 0 true 0 true true true 0
// 8 4 50 20 7 13 3
// 5 5
// [1 2 3] [4 50 6] [a b] map[one:1 two:2] {1 20} &{13 4} [ann bob] {7 8} [0 0 9 0 1] [[1] [2 3]] [2 3]
// [] [0 0 0 0] map[] {0 0} <nil> true 0
