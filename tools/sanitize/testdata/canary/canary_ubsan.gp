// phase: 04
// canary: ubsan

// A canary for the UndefinedBehaviorSanitizer job (step 15.11, tools/sanitize): this program is
// wrong on purpose. The sanitizer job must report "shift exponent 40 is too large" for it.
package main

import (
	"fmt"

	"./canary_ubsan.h"
)

func main() {
	fmt.Println(int(canary_ubsan.shift_too_far(1, 40)))
}
