// phase: 06

// Callbacks to C (step 4.11) that are functions of another package: main names handlers.Visit,
// handlers.OnOpen and handlers.OnData where C takes a function pointer, in a call and in a keyed
// literal of a C struct (callbackValue accepts pkg.F, not only a name of the package). The C
// file is the main package's own; handlers has only Go types in the signatures it exports. Package
// extra passes the same callback from another translation unit (the trampoline is one template
// instance for the program), through the header named by its module path.
package main

import (
	"fmt"
	"unsafe"

	"./ctable.h"
	"example.com/cb/extra"
	"example.com/cb/handlers"
)

func main() {
	t := &handlers.Tally{}
	fmt.Println(ctable.ctab_walk(5, handlers.Visit, unsafe.Pointer(t)), t.Bytes, ctable.ctab_cleanups())
	handlers.Report(t)
	fmt.Println(extra.Walk(3, t), t.Bytes)
	h := ctable.ctab_handlers{on_open: handlers.OnOpen, on_data: handlers.OnData, user: unsafe.Pointer(t)}
	ctable.ctab_set_handlers(&h)
	ctable.ctab_open("db")
	r1 := ctable.ctab_data(C.cstr("abcdef"), 3)
	r2 := ctable.ctab_data(C.cstr("xyz"), 2)
	fmt.Println(r1, r2, t.Names)
}

// Output:
// 30 10 1
// bytes 10
// 5 13
// open db
// 1 2 [abc xy]
