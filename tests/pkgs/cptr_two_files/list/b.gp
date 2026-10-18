package list

import "./umem.hpp"

// Sum is in another file, so in another translation unit: it reads the members back as pointers
// (the registry of the objects that gocpp owns is one for the program).
func (h *Handle) Sum() (s int) {
	for p := h.head; p != nil; p = p.next {
		s += int(p.v)
	}
	return
}

func (h *Handle) Drop() { h.head = h.head.next; h.n-- }
