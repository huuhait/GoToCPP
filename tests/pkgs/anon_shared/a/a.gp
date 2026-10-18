package a

import "fmt"

type Named struct{ V int }

func Pair() struct {
	X int
	Y string
} {
	return struct {
		X int
		Y string
	}{1, "a"}
}

func Wrap(n Named) struct{ N Named } { return struct{ N Named }{n} }

func Ifc() interface{ M() int } { return impl{5} }

type impl struct{ n int }

func (i impl) M() int { return i.n }

func Hidden() struct{ x int } { return struct{ x int }{7} }

func ShowHidden(h struct{ x int }) { fmt.Println("a.hidden", h.x) }

type unexp interface{ m() int }

type T1 struct{}

func (T1) m() int { return 1 }

func CallM(u unexp) int { return u.m() }

func MkT1() unexp { return T1{} }
