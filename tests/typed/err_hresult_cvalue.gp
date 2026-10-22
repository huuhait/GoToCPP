// phase: 12
// error: (?s)\.gp:\d+:\d+: cannot use hresult\.nt_result\(1\) \(value of type C\.long\) as error value in assignment\n\tonly a call of a function that returns an HRESULT is an error

// Only the call of a function that returns the typedef HRESULT is an error: a function that returns
// another typedef of long (NTSTATUS) is a number, and a number does not go into an error variable.
package main

import (
	"fmt"

	"./hresult.hpp"
)

func main() {
	hr := hresult.status_of(2)
	hr = hresult.nt_result(1)
	fmt.Println(hr)
}
