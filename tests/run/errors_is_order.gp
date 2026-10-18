// phase: 05

// errors.Is visits the tree in pre-order, depth first: err itself (== first, then its Is method), then its children in order.

package main

import (
	"errors"
	"fmt"
)

type node struct {
	name string
	kids []error
	next error
}

func (n *node) Error() string { return n.name }
func (n *node) Is(target error) bool {
	fmt.Print(n.name, " ")
	return false
}

type single struct{ node }

func (s *single) Unwrap() error { return s.next }

type multi struct{ node }

func (m *multi) Unwrap() []error { return m.kids }

func main() {
	leaf1 := &single{node{name: "leaf1"}}
	leaf2 := &single{node{name: "leaf2"}}
	chain := &single{node{name: "chain", next: leaf2}}
	m1 := &multi{node{name: "m1", kids: []error{leaf1, nil, chain}}}
	root := &single{node{name: "root", next: m1}}
	other := errors.New("not in the tree")
	fmt.Println(errors.Is(root, other))
	fmt.Println(errors.Is(m1, other))
	fmt.Println(errors.Is(leaf1, other))
	fmt.Println(errors.Is(root, leaf2), errors.Is(root, chain))
}

// Output:
// root m1 leaf1 chain leaf2 false
// m1 leaf1 chain leaf2 false
// leaf1 false
// root m1 leaf1 chain root m1 leaf1 true true
