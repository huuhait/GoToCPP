// phase: 04

// A project that was vendored by "gocpp mod vendor" (job 6.1c): gocpp.mod requires example.com/hdr, and
// vendor/modules.txt and vendor/example.com/hdr/ are what the command writes for a downloaded module (all
// of its files, here a README and a header-only library). The header is imported by the full path of its
// module (job 6.1b), and the project builds with no network and no module cache.
package main

import (
	"fmt"

	"example.com/hdr/include/hdr.h"
)

func main() {
	fmt.Println(hdr.clamp(15, 0, 10), hdr.clamp(-3, 0, 10), hdr.sum3(1, 2, 3), hdr.VERSION_MAJOR)
}

// Output:
// 10 0 6 1
