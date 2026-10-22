// phase: 05

// The address of a C/C++ local stored in a package variable keeps the object alive, as Go's pointer
// keeps its variable (docs/spec/interop.md, "&x of a C/C++ value"): `gp = &n`, `gp = &t.field`,
// `list = append(list, &p)` in a loop, `m[k] = &n`, a field of a package variable, a parameter, a
// pointer variable that is then stored, and a local that a deferred closure reads. Each local is
// boxed and the stored gp::cptr owns the box (the C++ destructor runs when the pointer is dropped, not
// at the end of the block); a plain `&n` that only goes to a call stays the raw C++ address.
package main

import (
	"fmt"

	"./cvar_addr_package_var.hpp"
)

var gint *C.int
var gtracked *cav.Tracked
var gpair *cav.Pair
var gfield *C.int
var gpairs []*cav.Pair
var gmap = map[string]*C.int{}
type Holder struct{ p *C.int }

var gholder Holder
var gthrough *C.int
var gmember *C.int
var gnested *C.int
var gelem *C.int
var ggo *C.int
var galias *C.int
var gaddrs []*C.int

func setInt() {
	var n C.int = 5
	gint = &n
}

func setTracked() {
	t := cav.Tracked{}
	t.inc()
	gtracked = &t
}

func setPair() {
	p := cav.Pair{a: 1, b: 2}
	gpair = &p
	gfield = &p.b
}

func appendPairs() {
	for i := 0; i < 3; i++ {
		p := cav.Pair{a: C.int(i), b: C.int(i * 10)}
		gpairs = append(gpairs, &p)
	}
}

func setMap() {
	var n C.int = 7
	gmap["n"] = &n
}

func setHolder(x C.int) {
	gholder.p = &x
}

func setThrough() {
	var n C.int = 9
	p := &n
	gthrough = p
}

type Gen struct{ in cav.Inner }

// Only the address of a member is stored: nothing else keeps the local alive.
func setMember() {
	p := cav.Pair{a: 30, b: 31}
	gmember = &p.b
}

func setNested() {
	r := cav.Rec{}
	r.in.b = 41
	gnested = &r.in.b
}

func setElem() {
	r := cav.Rec{}
	r.arr[2] = 52
	gelem = &r.arr[2]
}

func setGoStruct() {
	o := Gen{}
	o.in.a = 61
	ggo = &o.in.a
}

func setAliased() {
	var x C.int = 71
	q := &gholder
	q.p = &x
}

func setAppended() {
	for i := 0; i < 2; i++ {
		r := cav.Rec{}
		r.in.a = C.int(80 + i)
		gaddrs = append(gaddrs, &r.in.a)
	}
}

func clobber(n int) int {
	var buf [64]int
	for i := range buf {
		buf[i] = n + i
	}
	return buf[n%64]
}

func withDefer() {
	var n C.int = 11
	defer func() { fmt.Println("deferred sees", n) }()
	gint = &n
}

func main() {
	setInt()
	setTracked()
	setPair()
	appendPairs()
	setMap()
	setHolder(8)
	setThrough()
	fmt.Println(*gint, gtracked.n, gpair.a, gpair.b, *gfield)
	for _, p := range gpairs {
		fmt.Println(p.a, p.b)
	}
	fmt.Println(*gmap["n"], *gholder.p, *gthrough)
	gtracked.inc()
	*gfield = 20
	fmt.Println(gtracked.n, gpair.b)
	fmt.Println(cav.alive())
	gtracked = nil
	fmt.Println(cav.alive())
	setMember()
	setNested()
	setElem()
	setGoStruct()
	setAliased()
	setAppended()
	clobber(5)
	fmt.Println(*gmember, *gnested, *gelem, *ggo, *gholder.p)
	fmt.Println(*gaddrs[0], *gaddrs[1])
	withDefer()
	fmt.Println(*gint)
}

// Output:
// 5 1 1 2 2
// 0 0
// 1 10
// 2 20
// 7 8 9
// 2 20
// 1
// 0
// 31 41 52 61 71
// 80 81
// deferred sees 11
// 11
