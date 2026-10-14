// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// defer with structs: a struct argument is copied at the defer statement (also one larger
// than 128 KiB, which is passed by reference and stored as a copy); a named struct result
// changed through a pointer by a deferred call is what the function returns; and p.x with a
// nil p panics at the defer statement, so that call is never deferred and the ones before it
// run.
package main

import "fmt"

type Pt struct{ x, y int }

type Huge struct {
	v   int
	big [1 << 15]int
	n   int
}

func show(tag string, p Pt) { fmt.Println(tag, p, p.x+p.y) }

func showHuge(h Huge) { fmt.Println("huge", h.v, h.big[100], h.n) }

func fix(p *Pt) { p.y = 99 }

func move(p *Pt, dx int) { p.x += dx }

func named() (p Pt) {
	defer fix(&p)
	p = Pt{1, 2}
	return
}

func valued() (p Pt) {
	defer move(&p, 10)
	return Pt{3, 4}
}

func copies() {
	s := Pt{1, 2}
	defer show("copy", s)
	ps := &s
	defer show("through a pointer", *ps)
	s.x = 100
	ps.y = 200
	fmt.Println("now", s)
}

func huge() {
	var h Huge
	h.v, h.big[100], h.n = 1, 2, 3
	defer showHuge(h)
	h.v, h.big[100], h.n = 10, 20, 30
	fmt.Println("huge now", h.v, h.big[100], h.n)
}

func field(p *Pt) int { return p.x }

func nilField() {
	defer fmt.Println("deferred before the nil field")
	var p *Pt
	defer fmt.Println("never", p.x)
	fmt.Println("not reached")
}

func main() {
	fmt.Println(named(), valued())
	copies()
	huge()
	nilField()
}

// Output:
// {1 99} {13 4}
// now {100 200}
// through a pointer {1 2} 3
// copy {1 2} 3
// huge now 10 20 30
// huge 1 2 3
// deferred before the nil field
