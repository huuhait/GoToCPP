// phase: 05

// Macros across packages (step 10.9): #util.Assert, a macro of another package, on top of
// functions and inside code; lazy parameters of util's own types; generic macros instantiated in
// main; a macro of check that uses macros of util; an import with another name and a dot import.
// #file() is the path from the module root and #func() has the import path of the package.

package main

import (
	"fmt"

	"example.com/macros/check"
	. "example.com/macros/dot"
	"example.com/macros/util"
	u "example.com/macros/util"
)

#util.Trace()
func work(n int) int {
	#util.Assert(n < 10, fmt.Sprint("n=", n))
	return #util.Dbg(n * 2)
}

#util.VM()
#util.Trace()
func (p *point) move(dx int) (x, y int) {
	p.x += dx
	return p.x, p.y
}

type point struct{ x, y int }

func main() {
	fmt.Println(work(3), work(30))
	p := &point{1, 2}
	fmt.Println(p.move(5))

	fmt.Println(#util.Describe(util.Result{Name: "r", OK: true}, 7))
	fmt.Println(#u.Describe(util.Result{Name: "s"}, util.Level(1)+1))

	// Generic: instantiated here, with util's private helpers named in the code.
	fmt.Println(#util.Dbg("text"), #util.Dbg(1.5), #util.Dbg([]int{1, 2}))

	// Macros that use macros of util.
	#check.Positive(-4)
	fmt.Println(#check.Both(2, 3))
	fmt.Println(#check.Both(-1, 1))

	fmt.Println(#Twice(21), util.Internal(4), util.Internal(-1))
	fmt.Println("util reported", util.Calls())
}

// Output:
// enter main.work
// dbg n * 2 = 6
// exit main.work
// enter main.work
// assert main.gp:21 in main.work: n < 10: n=30
// dbg n * 2 = 60
// exit main.work
// 6 60
// vm main.(*point).move
// enter main.(*point).move
// exit main.(*point).move
// 6 2
// r:true:7
// s:false:2
// dbg "text" = text
// dbg 1.5 = 1.5
// dbg []int{1, 2} = [1 2]
// text 1.5 [1 2]
// assert main.gp:46 in main.main: v > 0: v must be positive
// dbg v + w() = 5
// 5
// assert main.gp:48 in main.main: v > 0: v must be positive
// dbg v + w() = 0
// 0
// dbg n = 4
// assert util/util.gp:58 in example.com/macros/util.Internal: n > 0: n is positive
// dbg n = -1
// 42 5 0
// util reported 20
