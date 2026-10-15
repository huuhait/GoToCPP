// fmt calls with the values of Phase 03 (step 3.15): slices, arrays, maps, structs, pointers
// and named types as arguments of Print, Println and Printf; Sprint, Sprintln and Sprintf,
// which return a gp::string (also from a call with several results, through std::apply);
// widths, precisions and flags; the type tags for int64, uint64 and uintptr (gp::fmtshim::typed,
// also in a deferred call); a nil pointer; gc's copies of fmt's arguments that are not
// addressable (a map read of an array or a pointer, a struct literal) before a later call; the
// builtins min and max; println of a pointer.
package main

import "fmt"

type pt struct{ x, y int }

type Celsius float64

func (c Celsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

type ids []int64

var ma = map[string][2]int{"k": {1, 2}}
var mp = map[string]*pt{"k": {3, 4}}
var n int

func change() int {
	ma["k"] = [2]int{5, 6}
	mp["k"] = nil
	n++
	return n
}

func two() (int64, []string) { return 7, []string{"a"} }

func main() {
	s := []int{1, 2}
	a := [2]string{"a", "b"}
	m := map[string]int{"b": 2, "a": 1}
	p := &pt{1, 2}
	var np *pt
	fmt.Println(s, a, m, p, *p, np, (*int)(nil), []int(nil))
	fmt.Println(Celsius(21.5), ids{1}, []Celsius{1})
	var i64 int64 = 5
	var u64 uint64 = 6
	var up uintptr = 7
	fmt.Printf("%s %d %v %s\n", i64, []uint64{u64}, map[string]uintptr{"k": up}, &[1]int64{8})
	fmt.Printf("[%5d][%-6.2f][%+v][%08.3f][%q][% d]\n", 42, 3.14159, *p, -1.5, "q", 7)
	str := fmt.Sprint("x", 1, 2, s) + fmt.Sprintln(a) + fmt.Sprintf("%v|%d", m, i64)
	fmt.Print(str, "\n")
	fmt.Println(two())
	t := fmt.Sprint(two())
	fmt.Println(t, len(fmt.Sprintln(two())))
	fmt.Println(ma["k"], mp["k"], pt{n, 0}, n, change())
	defer fmt.Printf("deferred %s %v\n", i64, []int64{i64})
	i64 = 50
	x, y := 3, -4
	f := 2.5
	fmt.Println(min(x, y), max(x, y, 10), min(f, 1), max("a", a[1]), min(Celsius(f), 3))
	println(np, p != nil)
}
