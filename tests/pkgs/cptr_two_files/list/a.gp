package list

import "./umem.hpp"

// Handle holds a list of C nodes; the nodes are made here, in this file's unit.
type Handle struct {
	head *umem.unode
	n    int
}

func New() *Handle { return &Handle{} }

func (h *Handle) Push(v int) {
	n := &umem.unode{v: C.int(v)}
	n.next = h.head // the owner of the old head is kept alive by n
	h.head = n
	h.n++
}
