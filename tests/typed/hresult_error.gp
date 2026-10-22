// phase: 12
// opaque: reject
// windows: yes

// A call of a function whose result type is the typedef HRESULT is an error in the program: nil for
// every code that is not negative (S_OK, S_FALSE and the other success codes), else an error that
// keeps the code and says it. The function and the virtual method are called as they are.
package main

import (
	"errors"
	"fmt"

	"./hresult.hpp"
)

// hresulter is what a program asks of the error to get the code: its Go type is not nameable.
type hresulter interface{ HResult() int32 }

func codeOf(err error) int32 {
	var h hresulter
	if errors.As(err, &h) {
		return h.HResult()
	}
	return 0
}

func status(kind int) error {
	err := hresult.status_of(kind)
	if err != nil {
		return err
	}
	return nil
}

func direct(kind int) error {
	return hresult.status_of(kind)
}

func main() {
	for kind := 0; kind <= 5; kind++ {
		err := hresult.status_of(kind)
		fmt.Println(kind, err == nil, err)
	}

	// A function returns it, with or without a check first.
	fmt.Println(status(2), direct(3), direct(4))

	// The error carries the code: the same code is the same error.
	err := direct(4)
	fmt.Printf("%x\n", uint32(codeOf(err)))
	fmt.Println(err == direct(4), err == direct(2), errors.Is(err, direct(4)))
	fmt.Println(codeOf(direct(0)))

	// An if with an init statement, an out-parameter that is still written, a method through the vtable.
	var value C.int
	if err := hresult.create(&value, -3); err != nil {
		fmt.Println("create failed:", err, value)
	}
	if err := hresult.create(&value, 9); err == nil {
		fmt.Println("create ok:", value)
	}
	chain := hresult.new_swap_chain()
	if err := chain.ResizeBuffers(0, 5); err != nil {
		fmt.Println("resize failed:", err)
	}
	fmt.Println(chain.ResizeBuffers(4, 5) == nil, chain.Present(1, 1) == nil, chain.Present(9, 0))
	var surface *hresult.Surface
	fmt.Println(chain.GetBuffer(0, &surface), hresult.surface_value(surface))
	fmt.Println(chain.GetBuffer(1, &surface))
}

// Output:
// 0 true <nil>
// 1 true <nil>
// 2 false HRESULT 0x80004005 (E_FAIL)
// 3 true <nil>
// 4 false HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)
// 5 false HRESULT 0xFFFFFFFB
// HRESULT 0x80004005 (E_FAIL) <nil> HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)
// 887a0005
// true false true
// 0
// create failed: HRESULT 0x80070057 (E_INVALIDARG) -3
// create ok: 9
// resize failed: HRESULT 0x80070057 (E_INVALIDARG)
// true true HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)
// <nil> 7
// HRESULT 0x80070057 (E_INVALIDARG)
