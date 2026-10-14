// phase: 03

// Named strings, slices, arrays, maps and pointers (step 3.9): their operations give the named
// type where Go does (s[a:b], append, +), they convert to and from their underlying types (also
// implicitly where Go assigns them), they range like their underlying types, and named types
// declared in functions are types of their own.
package main

import "fmt"

type Name string
type Names []Name
type Row [3]int
type Grid [2]Row
type Ages map[Name]int
type Celsius float64
type Temp Celsius
type Node struct {
	val  int
	next *Node
}
type Link *Node
type Rune int32
type Bytes []byte

func longest(ns Names) Name {
	var best Name
	for _, n := range ns {
		if len(n) > len(best) {
			best = n
		}
	}
	return best
}

func total(r Row) (t int) {
	for _, v := range r {
		t += v
	}
	return
}

func grow(ns []Name) []Name { return append(ns, "zed") }

func main() {
	n := Name("gopher")
	n2 := n[1:4] + "!"
	fmt.Println(n2, len(n2), n2 == "oph!", n > n2, string(n[0]), n[len(n)-1])
	for i, r := range n[:3] {
		fmt.Print(i, ":", r, " ")
	}
	fmt.Println()
	b := []byte(n)
	b[0] = 'G'
	fmt.Println(Name(b), string(Bytes(b)), Name(Rune(0x4e16)), []rune(n)[1])
	ns := Names{"ann", "bob"}
	ns = append(ns, "carol", n)
	ns = grow(ns)
	fmt.Println(len(ns), longest(ns), ns[len(ns)-1], longest(ns[:2]))
	ns2 := ns[1:3:4]
	fmt.Println(len(ns2), cap(ns2), ns2[0])
	copy(ns, Names{"x"})
	fmt.Println(ns[0])
	var nilNames Names
	fmt.Println(nilNames == nil, len(nilNames), longest(nil))
	ms := make(Names, 2, 5)
	ms[1] = "m"
	fmt.Println(len(ms), cap(ms), ms[1] == "m")
	for i := range ms {
		ms[i] += "+"
	}
	fmt.Println(ms[0], ms[1])

	r := Row{1, 2, 3}
	g := Grid{r, {4, 5, 6}}
	g[1][2] = 60
	fmt.Println(total(r), total(g[1]), g[0] == r, g == Grid{{1, 2, 3}, {4, 5, 60}}, len(g))
	for i, row := range g {
		fmt.Println(i, row[0], total(row))
	}
	s := r[1:]
	s[0] = 20
	fmt.Println(r[1], len(s))
	pr := &r
	pr[2] = 30
	fmt.Println(r[2], len(pr), total(*pr))
	for i, v := range pr {
		fmt.Print(i, v, " ")
	}
	fmt.Println()

	ages := Ages{"ann": 30}
	ages["bob"] = 25
	ages["ann"]++
	delete(ages, "zed")
	age, ok := ages["bob"]
	fmt.Println(len(ages), ages["ann"], age, ok)
	var nilAges Ages
	fmt.Println(nilAges == nil, nilAges["x"])
	sum := 0
	for k, v := range ages {
		sum += v + len(k)
	}
	fmt.Println(sum)
	mk := make(Ages)
	mk[n] = 1
	fmt.Println(mk["gopher"])

	c := Celsius(20)
	t := Temp(c) + 0.5
	fmt.Println(t, Celsius(t)*2, float64(t))
	pc := &c
	*pc += 1
	cp := new(Celsius)
	*cp = c * 2
	fmt.Println(c, *cp)

	var l Link = &Node{val: 1}
	l.next = &Node{val: 2}
	var plain *Node = l
	fmt.Println(l.val, l.next.val, plain.val, l != nil, (*Node)(l) == plain)
	l = nil
	fmt.Println(l == nil)

	type Local int
	type Pair struct{ a, b Local }
	pa := Pair{1, 2}
	pa.a += pa.b * 10
	fmt.Println(pa, Local(7)/2)
	fmt.Printf("%v %s %d %v\n", n, n2, Local(3), c)
}

func init() {
	type Local string
	fmt.Println(Local("init") + "!")
}
