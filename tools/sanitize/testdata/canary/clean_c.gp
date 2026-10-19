// phase: 04

// A control for the canaries (step 15.11): the same kind of program as canary_asan, canary_ubsan
// and canary_lsan, without their errors. Every sanitizer job must leave it silent.
package main

import (
	"fmt"

	"./clean_c.h"
)

func main() {
	fmt.Println(int(clean_c.sum_block(16)))
}
