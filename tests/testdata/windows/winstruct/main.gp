// phase: 14

// A package of gocpp, C and C++ files for Windows (step 14.4, ADR-082): the Win32 defaults reach the
// header, the probes and the package's own files; packed structs, bit-fields, UTF-16 text in a struct,
// and the WINAPI callback type work through the C API.
package main

import (
	"fmt"
	"unsafe"

	"./winstruct.h"
)

// ERROR_LEVEL is a macro of the header: a gocpp name can still be called ERROR.
const ERROR = 5

type Entry struct{ ID int }

// add is a binop: a WINAPI function pointer, which a plain gocpp function becomes.
func add(a, b C.int) C.int { return a + b }

func main() {
	fmt.Println("add_stdcall:", int(winstruct.add_stdcall(2, 3)))
	fmt.Println("apply with a gocpp callback:", int(winstruct.apply(add, 40, 2)))

	fmt.Println("packed size:", int(winstruct.packed_size()), unsafe.Sizeof(winstruct.Packed{}))
	var p winstruct.Packed
	p.a, p.b, p.c = 1, 0x01020304, 5
	fmt.Println("packed fields:", int(p.a), int(p.b), int(p.c))

	f := winstruct.make_flags(5, 17, 0xABCDEF)
	fmt.Println("bit-fields:", int(f.kind), int(f.level), int(f.id), unsafe.Sizeof(f))
	f.level = 31
	fmt.Println("bit-field store:", int(f.kind), int(f.level), int(f.id))

	var e winstruct.Entry
	winstruct.fill_entry(&e, 7, C.wstr("héllo 世界"))
	fmt.Println("entry:", int(e.id), C.GoStringW(&e.name[0]), e.enabled != 0)

	fmt.Println("clamp3 (C++ with std::min and std::max):", int(winstruct.clamp3(0, 15, 10)), int(winstruct.clamp3(0, -4, 10)))
	fmt.Println("macro:", int(winstruct.ERROR_LEVEL), ERROR, Entry{ID: 3}.ID)
}
