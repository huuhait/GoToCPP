package b

import "fmt"

// the same struct type as a.Pair's result, in another package
func Take(p struct {
	X int
	Y string
}) {
	fmt.Println("b.Take", p.X, p.Y)
}

func Show(i interface{ M() int }) { fmt.Println("b.Show", i.M()) }

type unexp interface{ m() int }

type T2 struct{}

func (T2) m() int { return 2 }

func CallM(u unexp) int { return u.m() }

func MkT2() unexp { return T2{} }

func Hidden() struct{ x int } { return struct{ x int }{8} }

func ShowHidden(h struct{ x int }) { fmt.Println("b.hidden", h.x) }
