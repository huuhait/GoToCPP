package extra

import (
	"unsafe"

	"example.com/cb/ctable.h"
	"example.com/cb/handlers"
)

// Walk uses the same callback as main does, from another package and another translation unit.
func Walk(n int, t *handlers.Tally) int {
	return int(ctable.ctab_walk(C.int(n), handlers.Visit, unsafe.Pointer(t)))
}
