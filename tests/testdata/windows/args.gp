// phase: 06

// The header uses the runtime's own gp::args (os.Args comes with Phase 09).
//
//gocpp:cflags -I../../../runtime/include
package main

import (
	"./cmdline.hpp"
)

func main() {
	cmdline.print_args()
}
