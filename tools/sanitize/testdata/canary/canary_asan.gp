// phase: 04
// canary: asan

// A canary for the AddressSanitizer job (step 15.11, tools/sanitize): this program is wrong on
// purpose. The sanitizer job must report a heap-buffer-overflow for it; if the run is silent the
// job is blind and fails.
package main

import (
	"fmt"

	"./canary_asan.h"
)

func main() {
	fmt.Println(int(canary_asan.overflow_heap(4)))
}
