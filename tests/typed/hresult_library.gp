// phase: 12
// opaque: reject
// windows: yes

// A variable made from the call of a function that returns an HRESULT is an error wherever an error
// goes: the functions of the Go libraries, panic, type assertions and switches, methods, map keys.
package main

import (
	"errors"
	"fmt"

	"./hresult.hpp"
)

// hresulter is what a program asks of the error to get the code: its Go type is not nameable.
type hresulter interface{ HResult() int32 }

// shout takes the text of the error to a function of the program.
func shout(text string) string { return fmt.Sprintf("<%s>", text) }

func main() {
	e := hresult.status_of(4)
	var h hresulter
	fmt.Printf("%v %x\n", errors.As(e, &h), uint32(h.HResult()))
	fmt.Println(errors.Is(e, hresult.status_of(4)), errors.Is(e, hresult.status_of(2)))
	fmt.Println(errors.Unwrap(e))
	fmt.Println(errors.Join(e, nil))
	wrapped := fmt.Errorf("device: %w", e)
	fmt.Println(wrapped, errors.Is(wrapped, e))
	fmt.Printf("%v|%q\n", e, e)

	// Methods and the text of the error.
	fmt.Println(len(e.Error()), shout(e.Error()))
	fmt.Println(e.Error() == "HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)", "x" == e.Error())

	// Type assertions and switches.
	if x, ok := e.(hresulter); ok {
		fmt.Println("asserted", x.HResult() < 0)
	}
	switch v := e.(type) {
	case hresulter:
		fmt.Println("type switch", v.HResult() < 0)
	default:
		fmt.Println("not an hresulter")
	}
	switch e {
	case nil:
		fmt.Println("nil")
	case hresult.status_of(2):
		fmt.Println("E_FAIL")
	case hresult.status_of(4):
		fmt.Println("the same code")
	}
	switch hresult.status_of(0) {
	case nil:
		fmt.Println("S_OK is nil")
	}

	// A map key: the same code is the same key.
	counts := map[error]int{}
	counts[hresult.status_of(4)]++
	counts[hresult.status_of(4)]++
	counts[hresult.status_of(2)]++
	fmt.Println(len(counts), counts[e])

	// panic carries the error.
	defer func() {
		r := recover()
		err, ok := r.(error)
		fmt.Println("recovered:", ok, err)
	}()
	panic(e)
}

// Output:
// true 887a0005
// true false
// <nil>
// HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)
// device: HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED) true
// HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)|"HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)"
// 46 <HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)>
// true false
// asserted true
// type switch true
// the same code
// S_OK is nil
// 2 2
// recovered: true HRESULT 0x887A0005 (DXGI_ERROR_DEVICE_REMOVED)
