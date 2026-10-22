// phase: 12
// opaque: reject
// windows: yes

// A program whose only HRESULT function is a method, called with the address of a pointer variable for a
// void** parameter (a call that the probe cannot type as a whole): the declaration of the typedef is
// asked for from the method's result, so the call is an error all the same.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	chain := hresult.new_swap_chain()
	var surface *hresult.Surface
	err := chain.GetBuffer(1, &surface)
	fmt.Println(err)
	fmt.Println(chain.GetBuffer(0, &surface), surface.value)
}

// Output:
// HRESULT 0x80070057 (E_INVALIDARG)
// <nil> 7
