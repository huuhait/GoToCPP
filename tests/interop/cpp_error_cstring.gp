// phase: 04

// A gocpp string is no C string: a C function that takes a const char* doesn't take it (only a
// std::string_view parameter does, Phase 04 README §2.6). In opaque mode gocpp doesn't know the
// parameter's type, so it is the C++ compiler's error at the .gp line (step 4.17), with a hint.
// The other way, string(p) of a C string is an error too (C.GoString(p) copies one).
package main

import (
	"<cstdio>"
	"<cstdlib>"
	"<cstring>"
)

func main() {
	s := "hi"
	cstdio.puts(s)        // ERROR "C\+\+: (cannot convert|no viable conversion from) 'string' to 'const char ?\*' \(a gocpp string is no C string: pass C\.cstr\(s\), or C\.CString\(s\) for a copy that C keeps\)"
	_ = cstring.strlen(s) // ERROR "C\+\+: (cannot convert|no viable conversion from) 'string' to 'const char ?\*' \(a gocpp string"
	_ = string(cstdlib.getenv("HOME")) // ERROR "C\+\+: string\(p\) of a C string: use C\.GoString\(p\) \(or C\.GoStringN\(p, n\)\)"
	cstdio.puts(C.cstr(s))
	_ = C.GoString(cstdlib.getenv("HOME"))
}
