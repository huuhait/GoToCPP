// phase: 04

// A pointer to a C type is a map key (a gp::cptr compares and hashes by address, as Go's pointer
// does): two pointers are one key when they point at one object, whoever made them; the map holds
// its keys, so they live on after the variables are gone. A set of pointers is map[*T]bool.
package main

import (
	"fmt"

	"./umem.hpp"
)

func main() {
	m := map[*umem.upoint]int{}
	q := &umem.upoint{1, 2}
	r := &umem.upoint{1, 2} // equal value, another object: another key
	m[q] = 5
	m[r] = 6
	m[nil] = 7
	alias := q
	m[alias]++
	fmt.Println(len(m), m[q], m[r], m[nil], m[alias])
	delete(m, q)
	_, ok := m[q]
	fmt.Println(len(m), ok, m[r])

	// The keys outlive the variables that made them.
	seen := map[*umem.unode]bool{}
	for i := 1; i <= 3; i++ {
		seen[&umem.unode{v: C.int(i * 10)}] = true
	}
	sum := 0
	for k := range seen {
		sum += int(k.v)
	}
	fmt.Println(len(seen), sum)

	// Pointers from C: one key per address; C walks the list, gocpp indexes it.
	list := umem.build_list(3)
	index := map[*umem.unode]int{}
	i := 0
	for p := list; p != nil; p = p.next {
		index[p] = i
		i++
	}
	second := list.next
	fmt.Println(len(index), index[second], index[list.next.next], index[nil])
	var q2 *umem.unode = second
	fmt.Println(index[q2], q2 == second)
	umem.free_list(list)
}

// Output:
// 3 6 6 7 6
// 2 false 6
// 3 60
// 3 1 2 0
// 1 true
