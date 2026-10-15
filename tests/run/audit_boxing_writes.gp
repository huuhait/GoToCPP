// phase: 03

// Audit case: the boxing rule counts every change of a captured variable (Phase 03 README §2.2):
// field and element writes, ++ and op-assign, not only x = .... The closure and the function see
// one variable.
package main

import "fmt"

type P struct {
	x, y int
	arr  [2]int
	m    map[string]int
}

func main() {
	p := P{m: map[string]int{}}
	setX := func(v int) { p.x = v }
	incArr := func() { p.arr[1]++ }
	addY := func(d int) { p.y += d }
	show := func() string { return fmt.Sprint(p.x, p.y, p.arr) }
	setX(4)
	incArr()
	incArr()
	addY(10)
	addY(-3)
	fmt.Println(show(), p.x, p.y, p.arr)

	p.x = 40 // the function's write, seen by the closure
	p.arr[0] = 9
	fmt.Println(show())

	n := 0
	inc := func() { n++ }
	dbl := func() { n *= 2 }
	inc()
	dbl()
	inc()
	n <<= 1
	fmt.Println(n)

	arr := [3]int{1, 2, 3}
	swap := func(i, j int) { arr[i], arr[j] = arr[j], arr[i] }
	swap(0, 2)
	fmt.Println(arr)

	// A variable that a closure only reads is copied (not boxed): it sees the value it had then
	// only if nothing changes it later; here nothing does.
	k := 7
	get := func() int { return k * 2 }
	fmt.Println(get())

	// The map is a reference either way.
	put := func(key string) { p.m[key]++ }
	put("a")
	put("a")
	fmt.Println(p.m)
}

// Output:
// 4 7 [0 2] 4 7 [0 2]
// 40 7 [9 2]
// 6
// [3 2 1]
// 14
// map[a:2]
