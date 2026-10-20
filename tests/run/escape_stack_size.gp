// phase: 08

// Escape analysis (step 15.2, ADR-114): a C++ stack does not grow, and a recursive function has a
// frame for every object that it keeps on the stack. A big object stays in a box on the heap (the
// objects of the recursion below are 80 KB: on the stack 50 levels would need 8 MB), and the
// small ones that stay are limited in a function. The output is what real Go prints.
package main

import "fmt"

type Buf struct {
	data [10000]int
	n    int
}

type Small struct{ a, b, c int }

func fill(b *Buf, v int) {
	for i := range b.data {
		b.data[i] = v
	}
	b.n = v
}

func touch(s *Small, v int) {
	s.a += v
	s.b -= v
	s.c ^= v
}

func rec(d int) int {
	if d == 0 {
		return 0
	}
	var b Buf
	fill(&b, d)
	p := &Buf{}
	fill(p, d+1)
	return rec(d-1) + b.data[5] + p.data[7]
}

func recSmall(d int) int {
	if d == 0 {
		return 0
	}
	var s Small
	touch(&s, d)
	p := &Small{a: d}
	touch(p, 2)
	return recSmall(d-1) + s.a + s.b + s.c + p.a
}

// A local closure must not unbox a large capture either: 120 frames would use over 9 MB.
func recCaptured(d int) int {
	if d == 0 {
		return 0
	}
	var b Buf
	set := func() { b.data[5] = d }
	set()
	return recCaptured(d-1) + b.data[5]
}

type Mid struct{ data [200]int }

func fillMid(m *Mid, v int) { m.data[0] += v }

// Eleven objects of 1600 bytes: the first ten stay on the stack, the last is in a box.
func budget() int {
	var m1, m2, m3, m4, m5, m6, m7, m8, m9, m10, m11 Mid
	fillMid(&m1, 1)
	fillMid(&m2, 2)
	fillMid(&m3, 3)
	fillMid(&m4, 4)
	fillMid(&m5, 5)
	fillMid(&m6, 6)
	fillMid(&m7, 7)
	fillMid(&m8, 8)
	fillMid(&m9, 9)
	fillMid(&m10, 10)
	fillMid(&m11, 11)
	return m1.data[0] + m2.data[0] + m3.data[0] + m4.data[0] + m5.data[0] + m6.data[0] +
		m7.data[0] + m8.data[0] + m9.data[0] + m10.data[0] + m11.data[0]
}

func main() {
	fmt.Println(rec(50))
	fmt.Println(rec(100))
	fmt.Println(recSmall(2000))
	fmt.Println(recCaptured(120))
	fmt.Println(budget())
}

// Output:
// 2600
// 10200
// 4006000
// 7260
// 66
