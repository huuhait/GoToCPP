// phase: 12
// opaque: reject
// windows: yes

// A variable made from the call of a function that returns an HRESULT is an error: Go errors go into
// it, and it goes where an error goes. That holds for a call the probe types and for one it cannot
// (the address of a pointer variable for a void** parameter).
package main

import (
	"errors"
	"fmt"

	"./hresult.hpp"
)

func wrap(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("wrapped: %w", err)
}

func main() {
	err := hresult.status_of(2)
	fmt.Println(err)
	err = errors.New("a Go error")
	fmt.Println(err)
	err = hresult.status_of(0)
	fmt.Println(err == nil)

	chain := hresult.new_swap_chain()
	var surface *hresult.Surface
	second := chain.GetBuffer(1, &surface)
	fmt.Println(wrap(second))
	second = errors.New("another Go error")
	fmt.Println(second)
	second = chain.GetBuffer(0, &surface)
	fmt.Println(wrap(second), surface.value)
}

// Output:
// HRESULT 0x80004005 (E_FAIL)
// a Go error
// true
// wrapped: HRESULT 0x80070057 (E_INVALIDARG)
// another Go error
// <nil> 7
