// phase: 12
// opaque: reject
// windows: yes

// The places where the HRESULT stays a number: a call whose result the program drops (a statement,
// defer, go), and a call that is converted to a number to get the code itself, so that a success
// code such as DXGI_STATUS_OCCLUDED can be told from S_OK. The other results that look like it are
// not errors: LRESULT, NTSTATUS, BOOL, and a typedef of HRESULT are the numbers that they are.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	chain := hresult.new_swap_chain()

	// The result of a statement is dropped, as in C++.
	chain.Present(1, 0)
	defer chain.Present(1, 0)
	fmt.Println(hresult.present_count())

	// The code, and the success code that is not S_OK.
	code := int32(hresult.status_of(2))
	fmt.Printf("%x\n", uint32(code))
	occluded := int32(chain.Present(1, 1)) == int32(hresult.DXGI_STATUS_OCCLUDED)
	fmt.Println(occluded, chain.Present(1, 1) == nil)
	var wide C.long = C.long(hresult.status_of(1))
	fmt.Println(wide)

	// Not HRESULT by name: numbers.
	fmt.Println(hresult.window_result(2), hresult.nt_result(0), hresult.bool_result(1), hresult.my_result(1))
	var lresult C.long = hresult.window_result(1)
	fmt.Println(lresult != 0)

	// An argument that the probe cannot spell (the address of a pointer variable for a void**
	// parameter) does not make the call an error when the program drops the result, assigns it to
	// _ or asks for the code.
	var surface *hresult.Surface
	chain.GetBuffer(0, &surface)
	code = int32(chain.GetBuffer(1, &surface))
	_ = chain.GetBuffer(0, &surface)
	fmt.Printf("%x %d\n", uint32(code), surface.value)
}

// Output:
// 1
// 80004005
// true true
// 1
// -2147467259 0 1 1
// true
// 80070057 7
