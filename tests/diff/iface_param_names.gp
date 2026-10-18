// phase: 05

// Thunks and wrappers of the interface structs use parameter and local names that no Go name can be
// (gp_self, gp_p, gp_a0): types and parameters named self, p, a0, a1 (and gp_self, gp_p, gp_a0), as
// receivers, parameters and results, with value and pointer methods behind interfaces (step 5.4).
package main

import "fmt"

type self struct{ n int }
type p struct{ n int }
type a0 struct{ n int }
type a1 struct{ n int }
type gp_self struct{ n int }
type gp_p struct{ n int }
type gp_a0 struct{ n int }

type I interface {
	M(self self, p p, a0 a0) int
	N(gp_self gp_self, gp_p gp_p, gp_a0 gp_a0) int
	Q(self, p, a0 int) int
}

type impl struct{ self int }

func (i impl) M(self self, p p, a0 a0) int { return i.self + self.n + p.n + a0.n }
func (i *impl) N(gp_self gp_self, gp_p gp_p, gp_a0 gp_a0) int {
	return gp_self.n + gp_p.n + gp_a0.n + i.self
}
func (i impl) Q(self, p, a0 int) int { return self*100 + p*10 + a0 }

type PI interface{ M() int }
type PJ interface{ N(int, string) int }

// The receiver types are named like the locals of the thunks.
func (x p) M() int                    { return x.n }
func (x a0) N(a int, b string) int    { return x.n + a + len(b) }
func (x *a1) N(a int, b string) int   { return x.n + a }
func (x a1) M() int                   { return x.n }
func (x *self) M() int                { return x.n * 2 }
func (x *gp_p) N(a int, b string) int { return x.n * a }

func main() {
	var i I = &impl{1}
	fmt.Println(i.M(self{1}, p{2}, a0{3}), i.N(gp_self{1}, gp_p{2}, gp_a0{3}), i.Q(1, 2, 3))
	var x any = i
	_, ok := x.(I)
	var v any = impl{5}
	_, ok2 := v.(interface{ Q(int, int, int) int })
	fmt.Println(ok, ok2)
	var s any = self{1}
	var pp any = &p{2}
	switch s.(type) {
	case self:
		fmt.Println("self")
	}
	fmt.Println(pp.(*p).n)

	var pi PI = &p{1}
	var pj PJ = &a0{2}
	var pk PJ = &a1{3}
	var pl PI = a1{4}
	var pm PI = &self{5}
	var pn PJ = &gp_p{6}
	fmt.Println(pi.M(), pj.N(1, "ab"), pk.N(1, "x"), pl.M(), pm.M(), pn.N(2, ""))
}
