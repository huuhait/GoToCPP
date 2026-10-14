// phase: 03

// range over a map: every form (k, k and v, only v, none, with variables that exist), the
// range expression is evaluated once, a key deleted before it is reached is not produced, the
// value is the one when the key is reached, and each iteration has its own variables (Go 1.22).
// gocpp's order is not Go's (D8), so this test only prints sums and sorted results.
package main

import "fmt"

var calls int

func get() map[int]int {
	calls++
	return map[int]int{1: 10, 2: 20, 3: 30}
}

func main() {
	m := map[int]int{1: 10, 2: 20, 3: 30, 4: 40}
	ks, vs := 0, 0
	for k, v := range m {
		ks += k
		vs += v
	}
	fmt.Println("k and v", ks, vs)
	ks = 0
	for k := range m {
		ks += k
	}
	fmt.Println("k", ks)
	vs = 0
	for _, v := range m {
		vs += v
	}
	fmt.Println("v", vs)
	n := 0
	for range m {
		n++
	}
	fmt.Println("none", n)
	var k, v int
	ks, vs = 0, 0
	for k, v = range m {
		ks += k
		vs += v
	}
	fmt.Println("assigned", ks, vs, k*v > 0)

	ks = 0
	for k := range get() {
		ks += k
	}
	fmt.Println("once", ks, calls)

	// Each key deletes the other key of its pair (1 and 2, 3 and 4): one of each pair is seen.
	seen := 0
	for k := range m {
		delete(m, k-1+2*(k%2))
		seen++
	}
	fmt.Println("deleted", seen, len(m))

	// Each key sets the value of the others: a key reached later shows the new value.
	vals := map[string]int{"a": 1, "b": 1, "c": 1}
	total := 0
	for k, v := range vals {
		total += v
		for o := range vals {
			if o != k {
				vals[o] = 10
			}
		}
	}
	fmt.Println("values", total)

	// The range variables are copies: changing them doesn't change the map or the loop.
	c := map[int]int{5: 50, 6: 60}
	sum := 0
	for k, v := range c {
		k += 100
		v = 0
		sum += k + v
	}
	fmt.Println("copies", sum, c[5], c[6], len(c))

	var ptrs []*int
	for k := range map[int]bool{1: true, 2: true, 3: true} {
		ptrs = append(ptrs, &k)
	}
	sum = 0
	for _, p := range ptrs {
		sum += *p
	}
	fmt.Println("per iteration", sum)

	var nilmap map[string]int
	for k, v := range nilmap {
		fmt.Println("never", k, v)
	}
	clear(c)
	for range c {
		fmt.Println("never")
	}
	fmt.Println("done", len(c))
}

// Output:
// k and v 10 100
// k 10
// v 100
// none 4
// assigned 10 100 true
// once 6 1
// deleted 2 2
// values 21
// copies 211 50 60 2
// per iteration 6
// done 0
