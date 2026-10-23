// phase: 08

// defer and go with arguments larger than 128 KiB (8 MiB and 64 MiB arrays, a struct with an
// 8 MiB array): the defer or go statement copies them, but to the heap (gp::detail::big_arg),
// not onto the C++ stack, which doesn't grow (an 8 MiB copy overflows the main thread's). The
// copy is what the statement saw: later changes don't reach the deferred call or the goroutine.
package main

import (
	"fmt"
	"sync"
)

type arr8 [1 << 20]int  // 8 MiB
type arr64 [8 << 20]int // 64 MiB

type big struct {
	n int
	a arr8
}

// mid is larger than 128 KiB but fits the stack: an unnamed struct converts to it with
// gp::convert<mid>, a temporary on the C++ stack (D24), then the heap copy.
type mid struct {
	n int
	a [1 << 15]int
}

var a8 arr8
var anon struct {
	n int
	a [1 << 15]int
}
var plain [1 << 20]int // the same type as arr8's underlying one: an implicit conversion
var a64 arr64
var b big
var wg sync.WaitGroup

func f8(tag string, x arr8) {
	fmt.Println(tag, x[1], x[len(x)-1])
}

func f64(tag string, x arr64, n int) {
	fmt.Println(tag, x[1], x[len(x)-1], n)
}

func fbig(x big, tag string) {
	fmt.Println(tag, x.n, x.a[1])
}

func fplain(tag string, x [1 << 20]int) {
	fmt.Println(tag, x[1])
}

func gplain(x [1 << 20]int) {
	defer wg.Done()
	fmt.Println("goplain", x[1])
}

func fmid(tag string, x mid) {
	fmt.Println(tag, x.n, x.a[1])
}

func gmid(x mid) {
	defer wg.Done()
	fmt.Println("gomid", x.n, x.a[1])
}

func (x mid) show(tag string, y mid) {
	fmt.Println(tag, x.n, y.n, y.a[1])
}

func two(x arr8, y arr64, n int) {
	fmt.Println("two", x[1], y[1], n)
}

func g8(x arr8) {
	defer wg.Done()
	fmt.Println("go8", x[1], x[len(x)-1])
}

func g64(x arr64, s string) {
	defer wg.Done()
	fmt.Println("go64", x[1], x[len(x)-1], s)
}

func gbig(x big) {
	defer wg.Done()
	fmt.Println("gobig", x.n, x.a[1])
}

func (p *big) take(x arr8, tag string) {
	fmt.Println(tag, p.n, x[1])
}

func (v big) show(tag string) {
	fmt.Println(tag, v.n, v.a[1])
}

func aware(x arr8) {
	if r := recover(); r != nil {
		fmt.Println("recovered", r)
	}
	fmt.Println("aware", x[1])
}

func awarePlain(x [1 << 20]int) {
	recover()
	fmt.Println("awareplain", x[1])
}

func change() int {
	a8[1] = 100
	return 7
}

func deferPackage() {
	a8[1], a8[len(a8)-1] = 1, 2
	defer f8("pkg8", a8)
	a64[1], a64[len(a64)-1] = 3, 4
	defer f64("pkg64", a64, 5)
	b.n, b.a[1] = 6, 7
	defer fbig(b, "pkgbig")
	defer two(a8, a64, 8)
	plain[1] = 9
	defer f8("convert", plain) // [1 << 20]int to arr8: converted on the heap
	defer fplain("convertplain", a8)
	anon.n, anon.a[1] = 40, 41
	defer fmid("unnamed struct", anon)
	defer mid{n: 42}.show("unnamed struct method", anon)
	fm := fmid
	defer fm("unnamed struct func value", anon)
	a8[1], a64[1], b.n, plain[1], anon.n = 0, 0, 0, 0, 0
}

func deferLocal() {
	var l arr8
	l[1] = 11
	defer f8("local8", l)
	var m arr64
	m[1] = 12
	defer f64("local64", m, 6)
	var s big
	s.n = 13
	defer fbig(s, "localbig")
	p := &l
	defer f8("deref8", *p)
	l[1], m[1], s.n = 0, 0, 0
}

func deferOrder() {
	a8[1] = 1
	defer f8("order", a8)
	defer fmt.Println("change", change())
	defer f64("call", a64, change())
	a8[1] = 2
}

func deferOthers() {
	var l arr8
	l[1] = 20
	fv := f8
	defer fv("funcvalue", l)
	defer func(x arr8) {
		fmt.Println("literal", x[1])
	}(l)
	var s big
	s.n = 21
	defer s.show("method")
	defer s.take(l, "ptrmethod")
	l[1], s.n = 0, 0
}

func deferAware() {
	var l arr8
	l[1] = 22
	defer aware(l)
	defer awarePlain(l)
	l[1] = 0
	panic("boom")
}

func deferLoop() {
	var l arr8
	for i := 0; i < 3; i++ {
		l[1] = i
		defer f8("loop", l)
	}
}

func goCalls() {
	a8[1], a8[len(a8)-1] = 30, 31
	wg.Add(1)
	go g8(a8)
	a8[1] = 0
	wg.Wait()

	a64[1], a64[len(a64)-1] = 32, 33
	wg.Add(1)
	go g64(a64, "s")
	a64[1] = 0
	wg.Wait()

	plain[1] = 38
	wg.Add(1)
	go g8(plain)
	plain[1] = 0
	wg.Wait()

	a8[1] = 39
	wg.Add(1)
	go gplain(a8)
	a8[1] = 0
	wg.Wait()

	anon.n, anon.a[1] = 43, 44
	wg.Add(1)
	go gmid(anon)
	anon.n = 0
	wg.Wait()

	var s big
	s.n, s.a[1] = 34, 35
	wg.Add(1)
	go gbig(s)
	s.n = 0
	wg.Wait()

	var l arr8
	l[1] = 36
	wg.Add(1)
	go func(x arr8) {
		defer wg.Done()
		fmt.Println("golit", x[1])
	}(l)
	l[1] = 0
	wg.Wait()

	var m arr64
	m[1] = 37
	done := make(chan bool)
	go func(x arr64) {
		fmt.Println("golit64", x[1])
		done <- true
	}(m)
	m[1] = 0
	<-done
}

func main() {
	deferPackage()
	deferLocal()
	deferOrder()
	deferOthers()
	deferAware()
	deferLoop()
	goCalls()
}
