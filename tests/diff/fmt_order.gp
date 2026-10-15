// phase: 03

// Go's order of evaluation for fmt's arguments (gc's copies, walk/order.go OCONVIFACE): a value
// that is not addressable is copied into a temporary in its place among the calls unless gc
// converts it by value (an 8-byte scalar, a string, a slice): so a map read of an array, a
// struct, a pointer or a map prints the old value, while a variable (a slice, an array, a map,
// a pointer) is read after the calls of its statement and prints what they made of it.
package main

import "fmt"

type Pt struct{ x, y int }

var s = []int{1, 2}
var arr = [3]int{1, 2, 3}
var m = map[string]int{"a": 1, "b": 2}
var p = &Pt{1, 2}
var ma = map[string][2]int{"k": {1, 2}}
var mp = map[string]*Pt{"k": {1, 2}}
var mm = map[string]map[string]int{"k": {"a": 1}}
var ms = map[string][]int{"k": {1, 2}}
var mi = map[string]int{"k": 1}
var mb = map[string]bool{"k": false}
var str = "hi"
var n int

func grow() int {
	s = append(s, 3)
	s[0] = 9
	return len(s)
}

func del() int {
	delete(m, "a")
	m["c"] = 3
	return len(m)
}

func setArr() int {
	arr[0] = 100
	return 0
}

func move() int {
	p = &Pt{7, 8}
	return 1
}

func change() string {
	ma["k"] = [2]int{10, 20}
	mp["k"] = &Pt{30, 40}
	mm["k"] = map[string]int{"z": 9}
	ms["k"] = []int{5}
	mi["k"] = 50
	mb["k"] = true
	str = "ho"
	n++
	return "changed"
}

func main() {
	fmt.Println(s, grow(), s)
	fmt.Println(m, del(), m)
	fmt.Println(arr, setArr(), arr, arr[0])
	fmt.Println(p, move(), p, *p)
	fmt.Println(ma["k"], mp["k"].x, mm["k"], ms["k"], mi["k"], mb["k"], str[0], change())
	fmt.Println(ma["k"], *mp["k"], mm["k"], ms["k"], mi["k"], mb["k"], str[0])
	fmt.Println(ma["k"], mm["k"], ms["k"], mi["k"], mb["k"], n == 1, Pt{n, 0}, [1]int{n}, [2]int{n, n}, change())
	fmt.Println(fmt.Sprint(s, arr, m), fmt.Sprintln(grow(), s), fmt.Sprintf("%v %d", arr, setArr()))
	fmt.Printf("%v %v %d\n", ma["k"], mi["k"], func2())
	fmt.Println(mp["k"] == mp["k"], change(), mp["k"] != nil)
}

func func2() int {
	ma["k"] = [2]int{-1, -2}
	mi["k"] = -5
	return 2
}
