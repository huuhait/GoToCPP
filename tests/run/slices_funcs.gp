// phase: 03

// Slices as parameters and results (the callee sees the caller's array, but its own header),
// package variables (a literal is set in the package's initialization, nil is the zero value),
// several results, and a slice variable whose address is taken (boxed).
package main

import "fmt"

var primes = []int{2, 3, 5, 7}
var none []string
var also []int = nil
var words = []string{"a", "b"}

func double(s []int) {
	for i := range s {
		s[i] *= 2
	}
	s = append(s, 100) // the caller's slice keeps its length
	s[0] = -1
}

func split(s []int, n int) ([]int, []int) {
	return s[:n], s[n:]
}

func evens(n int) (out []int) {
	for i := range n {
		if i%2 == 0 {
			out = append(out, i)
		}
	}
	return
}

func grow(p *[]int) {
	*p = append(*p, len(*p))
}

func main() {
	s := []int{1, 2, 3}
	double(s)
	fmt.Println(len(s), s[0], s[1], s[2])
	double(s[:1:1])
	fmt.Println(s[0])
	a, b := split(primes, 1)
	fmt.Println(len(a), len(b), a[0], b[0])
	b[0] = 30
	fmt.Println(primes[1])
	e := evens(7)
	fmt.Println(len(e), e[3])
	fmt.Println(none == nil, also == nil, len(words), words[1])
	none = append(none, "x")
	fmt.Println(len(none), none[0])
	var v []int
	grow(&v)
	grow(&v)
	p := &v
	(*p)[0] = 9
	fmt.Println(len(v), v[0], v[1])
	x, y := split(nil, 0)
	fmt.Println(x == nil, y == nil)
}

// Output:
// 3 2 4 6
// 4
// 1 3 2 3
// 30
// 4 6
// true true 2 b
// 1 x
// 2 9 1
// true true
