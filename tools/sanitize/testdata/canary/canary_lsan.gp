// phase: 04
// canary: lsan

// A canary for the LeakSanitizer job and the leak audit (step 15.11, tools/sanitize): this program
// is wrong on purpose, and has no "// lsan: cycle" line, so leak checking is on. The sanitizer job
// must report "detected memory leaks" for it.
package main

import (
	"fmt"

	"./canary_lsan.h"
)

func main() {
	fmt.Println(int(canary_lsan.leak_block()))
}
