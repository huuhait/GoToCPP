package main

import (
	"fmt"

	"example.com/macros/check"
	. "example.com/macros/dot"
	"example.com/macros/util"
	u "example.com/macros/util"
)

func work(n int) int {
	if f := util.Trace("main.work"); f != nil {
		defer f()
	}
	util.Assert("main.gp", 21, "main.work", "n < 10", n < 10, func() string { return fmt.Sprint("n=", n) })
	return util.Dbg("n * 2", n*2)
}

func (p *point) move(dx int) (x, y int) {
	util.VM("main.(*point).move", func() {
		x, y = func() (x, y int) {
			if f := util.Trace("main.(*point).move"); f != nil {
				defer f()
			}
			p.x += dx
			return p.x, p.y
		}()
	})
	return
}

type point struct{ x, y int }

func main() {
	fmt.Println(work(3), work(30))
	p := &point{1, 2}
	fmt.Println(p.move(5))

	fmt.Println(util.Describe(func() util.Result { return util.Result{Name: "r", OK: true} }, func() util.Level { return 7 }))
	fmt.Println(u.Describe(func() util.Result { return util.Result{Name: "s"} }, func() util.Level { return util.Level(1) + 1 }))

	fmt.Println(util.Dbg("\"text\"", "text"), util.Dbg("1.5", 1.5), util.Dbg("[]int{1, 2}", []int{1, 2}))

	check.Positive("main.gp", 46, "main.main", -4)
	fmt.Println(check.Both("main.gp", 47, "main.main", 2, func() int { return 3 }))
	fmt.Println(check.Both("main.gp", 48, "main.main", -1, func() int { return 1 }))

	fmt.Println(Twice(21), util.Internal(4), util.Internal(-1))
	fmt.Println("util reported", util.Calls())
}
