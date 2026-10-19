// phase: 05

// The debugger support must survive bad memory and be usable by default: a local that holds
// garbage (a frame before its declarations, a string with a negative length, an interface
// with a damaged type descriptor) prints as <unreadable: ...> and the other locals still
// print; boxed variables show their Go type; step stays in the .gp file (the skip rules are
// on when the script loads); Go expressions (acc.n, *acc, len(grid)) work in gp print; a
// deferred call's backtrace has no C++ helper frames. The session is robust.gdb.
package main

import "fmt"

type Acc struct{ n int }

func (a *Acc) Add(d int) {
	a.n += d // @add
}

func deferred() {
	defer func() {
		fmt.Println("deferred") // @deferred
	}()
}

func main() {
	count := 20
	inc := func() {
		count++ // @inc
	}
	inc()
	var err error
	show := func() { _ = err }
	show()
	acc := &Acc{}
	acc.Add(2)
	q := new(int)
	*q = 7
	fl := new(float64)
	*fl = 2.5
	r := []int{1, 2, 3}
	grow := func() { r = append(r, 4) }
	grow()
	arr := [3]int{4, 5, 6}
	fp := &arr
	grid := make([][]int, 300)
	for i := range grid {
		grid[i] = make([]int, 300)
	}
	rates := map[float64]string{100: "a", -2.5: "b", 10.5: "c", 9.5: "d"}
	msg := "hello"
	bad := [4]byte{0xff, 0xfe, 0x80, 0}
	fake := [8]int{}
	var anyv any = 5
	fmt.Println(count, err, acc.n, len(grid), rates, msg, bad, fake, anyv, *q, *fl, r, fp) // @locals
	acc.Add(3)
	fmt.Println("total: " + fmt.Sprint(count))
	deferred()
}
