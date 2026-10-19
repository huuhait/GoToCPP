// phase: 14

// C.wstr and C.GoStringW on a target where wchar_t is 32 bits (step 14.4, ADR-082): the text is UTF-32, the helpers
// work the same as on Windows, where wchar_t is 16 bits and the text is UTF-16.
package main

import (
	"fmt"
	"unsafe"

	"<wchar.h>"
)

func main() {
	// Length in wchar_t units: a character beyond U+FFFF is one unit where wchar_t is 32 bits, two where it is 16.
	units := int(wchar.wcslen(C.wstr("héllo 世\U0001F642")))
	want := 8
	if unsafe.Sizeof(C.wchar_t(0)) == 2 {
		want = 9
	}
	fmt.Println("wcslen:", units == want)

	// A round trip through a wide buffer.
	var buf [32]C.wchar_t
	wchar.wcscpy(&buf[0], C.wstr("round trip é世\U0001F642"))
	fmt.Println(C.GoStringW(&buf[0]))
	fmt.Println(C.GoStringWN(&buf[0], 5))

	// A lone surrogate (WTF-8) and an invalid byte.
	var bad [8]C.wchar_t
	wchar.wcscpy(&bad[0], C.wstr("a\xffb"))
	fmt.Printf("%q\n", C.GoStringW(&bad[0]))
	wchar.wcscpy(&bad[0], C.wstr("x\xed\xa0\x80y"))
	fmt.Printf("%q\n", C.GoStringW(&bad[0]))

	// nil is "", and a NUL inside ends the C string.
	fmt.Printf("%q %d\n", C.GoStringW(nil), int(wchar.wcslen(C.wstr("ab\x00cd"))))

	// A holder in a variable, and holders through the `...` of a variadic C function (the emitter writes
	// .gp_ptr() there: a class cannot go through `...`).
	w := C.wstr("kept")
	fmt.Println(int(wchar.wcslen(w)))
	var out [32]C.wchar_t
	n := wchar.swprintf(&out[0], 32, C.wstr("%ls-%d-%ls"), C.wstr("ab\u00e9"), C.int(5), w)
	fmt.Println(int(n), C.GoStringW(&out[0]))

	// C.WString is a malloc'ed copy for a pointer that C keeps; C.free frees it.
	kept := C.WString("kept \u4e16")
	fmt.Println(int(wchar.wcslen(kept)), C.GoStringW(kept))
	C.free(unsafe.Pointer(kept))
}

// Output:
// wcslen: true
// round trip é世🙂
// round
// "a�b"
// "x\xed\xa0\x80y"
// "" 2
// 4
// 10 abé-5-kept
// 6 kept 世
