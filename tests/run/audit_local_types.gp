// phase: 03

// Audit case: types declared inside functions (Phase 03 README §2.4) are moved to namespace
// scope with unique names: the same name in two functions, a local type that refers to itself,
// one used as a map key, printed with fmt (%v, %+v), compared, and a local type that shadows a
// package type.
package main

import "fmt"

type item struct{ name string }

func first() any0 {
	type item struct{ a, b int }
	type pair [2]item
	p := pair{{1, 2}, {3, 4}}
	return any0{fmt.Sprintf("%v %+v", p, p[1])}
}

type any0 struct{ s string }

func second() string {
	type item struct{ name, kind string }
	type node struct {
		val  int
		next *node
	}
	var head *node
	for i := 3; i > 0; i-- {
		head = &node{i, head}
	}
	sum := 0
	for n := head; n != nil; n = n.next {
		sum += n.val
	}
	m := map[item]int{{"a", "x"}: 1}
	m[item{"a", "x"}]++
	m[item{"b", "y"}] = 5
	return fmt.Sprint(sum, " ", len(m), " ", m[item{"a", "x"}], " ", item{"z", "w"} == item{"z", "w"})
}

func main() {
	fmt.Println(first().s)
	fmt.Println(second())
	type celsius float64
	type temps []celsius
	t := temps{20.5, 21}
	fmt.Println(t, len(t), item{"pkg"})
	for i := 0; i < 2; i++ {
		type counter struct{ n int }
		c := counter{i}
		c.n += 10
		fmt.Printf("%+v ", c)
	}
	fmt.Println()
}

// Output:
// [{1 2} {3 4}] {a:3 b:4}
// 6 2 2 true
// [20.5 21] 2 {pkg}
// {n:10} {n:11}
