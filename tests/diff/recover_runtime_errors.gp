// phase: 05

package main

import "fmt"

type T struct{ n int }

func try(name string, f func()) {
	defer func() {
		fmt.Printf("%s: %v\n", name, recover())
	}()
	f()
}

var zero = 0
var neg = -1
var big = 1 << 62

func main() {
	a := []int{1, 2, 3}
	s := "hello"
	arr := [4]int{1, 2, 3, 4}
	var m map[string]int
	var p *T
	var fn func()
	try("index", func() { _ = a[zero+5] })
	try("index negative", func() { _ = a[neg] })
	try("string index", func() { _ = s[zero+9] })
	try("array index", func() { _ = arr[zero+4] })
	try("slice high", func() { _ = a[:zero+10] })
	try("slice low", func() { _ = a[zero+4:] })
	try("slice order", func() { i := zero + 2; _ = a[i:1] })
	try("slice3", func() { _ = a[0:2:zero+7] })
	try("string slice", func() { _ = s[zero+3 : zero+9] })
	try("nil map store", func() { m["a"] = 1 })
	try("divide", func() { _ = 10 / zero })
	try("modulo", func() { _ = 10 % zero })
	try("nil pointer", func() { _ = p.n })
	try("nil pointer store", func() { p.n = 1 })
	try("nil func", func() { fn() })
	try("make len", func() { _ = make([]int, neg) })
	try("make cap", func() { _ = make([]int, 2, zero+1) })
	try("make huge", func() { _ = make([]int, big) })
	try("shift", func() { _ = 1 << neg })
	try("array conversion", func() { _ = [4]int(a) })
	try("custom", func() { panic(fmt.Sprintf("custom %d", 7)) })
}
