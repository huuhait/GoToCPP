// phase: 02

// Calls inside operators run left to right: arithmetic, comparisons,
// string concatenation, nested calls, conversions, unary operators, and
// operators mixed with locals and constants.
package main

import "fmt"

func tr(name string, v int) int {
	fmt.Print(name, " ")
	return v
}

func trf(name string, v float64) float64 {
	fmt.Print(name, " ")
	return v
}

func trs(name string) string {
	fmt.Print(name, " ")
	return name
}

func main() {
	x := tr("a", 1) + tr("b", 2)*tr("c", 3)
	fmt.Println(x)
	y := tr("a", 10) - tr("b", 4) - tr("c", 3)
	fmt.Println(y)
	z := (tr("a", 1) + tr("b", 2)) * (tr("c", 3) + tr("d", 4))
	fmt.Println(z)
	fmt.Println(tr("lhs", 1) < tr("rhs", 2))
	fmt.Println(trs("left") == trs("right"))
	s := trs("s1") + "-" + trs("s2")
	fmt.Println(s)
	f := trf("f1", 1.5) * trf("f2", 2)
	fmt.Println(f)
	u := uint8(tr("u1", 200)) + uint8(tr("u2", 100))
	fmt.Println(u)
	n := -tr("neg", 5) + ^tr("not", 0)
	fmt.Println(n)
	k := 3
	m := k*tr("m1", 2) + k*tr("m2", 4)
	fmt.Println(m)
	sh := tr("v", 1) << uint(tr("count", 4))
	fmt.Println(sh)
	q := tr("dividend", 17) / tr("divisor", 5) % tr("mod", 2)
	fmt.Println(q)
	b := tr("a", 1) == 1 && tr("b", 2) == 2 || tr("c", 3) == 0
	fmt.Println(b)
	w := tr("w1", 1) | tr("w2", 2) ^ tr("w3", 4)&tr("w4", 6)
	fmt.Println(w)
	c := len(trs("len1")) + len(trs("len22"))
	fmt.Println(c)
	nested := tr("outer", tr("inner1", 1)+tr("inner2", 2))
	fmt.Println(nested)
}
