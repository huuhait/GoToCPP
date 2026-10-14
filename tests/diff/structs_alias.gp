// phase: 03

// Structs are values, as in Go: a copy is independent, a struct in a slice is changed in place
// through an index or a pointer, an array of structs is copied whole, a range copies each
// element into its variable, and a pointer to a field of an element keeps the old array alive
// after an append moves the slice.
package main

import "fmt"

type Acct struct {
	owner string
	bal   int
	tags  [2]string
	hist  []int
}

func deposit(a *Acct, v int) {
	a.bal += v
	a.hist = append(a.hist, v)
}

func rich(a Acct) bool {
	a.bal *= 2 // a copy
	return a.bal > 100
}

func main() {
	a := Acct{owner: "ann", bal: 10}
	b := a
	deposit(&a, 5)
	b.tags[0] = "copy"
	fmt.Println(a.owner, a.bal, len(a.hist), a.tags[0] == "", b.bal, b.tags[0], len(b.hist))

	accts := []Acct{{owner: "x"}, {owner: "y", bal: 60}}
	accts[0].bal = 7
	p := &accts[1]
	deposit(p, 1)
	fmt.Println(accts[0].bal, accts[1].bal, rich(accts[1]), accts[1].bal)
	for _, v := range accts {
		v.bal = 0 // the copy
	}
	for i := range accts {
		accts[i].bal++
	}
	fmt.Println(accts[0].bal, accts[1].bal)

	arr := [2]Acct{{owner: "m"}, {owner: "n"}}
	arr2 := arr
	arr2[0].owner = "changed"
	fmt.Println(arr[0].owner, arr2[0].owner, arr[1].owner)

	pb := &accts[0].bal
	accts = append(accts, Acct{owner: "z"}) // a new array
	*pb = 99
	fmt.Println(accts[0].bal, *pb, len(accts))

	s := a
	s.hist[0] = 42 // the copy shares the slice's array
	fmt.Println(a.hist[0], s.hist[0])
}
