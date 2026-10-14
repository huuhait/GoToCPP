// phase: 01
//gocpp:pkgconfig sdl2 // ERROR "unknown //gocpp: directive \x22pkgconfig\x22; did you mean //gocpp:pkg-config\?"
//gocpp:lnk m // ERROR "unknown //gocpp: directive \x22lnk\x22; did you mean //gocpp:link\?"
//gocpp:frobnicate // ERROR "unknown //gocpp: directive \x22frobnicate\x22$"
//gocpp:extern-c ./a.h ./b.h // ERROR "usage: //gocpp:extern-c header"
//gocpp:link // ERROR "usage: //gocpp:link library\.\.\."
//gocpp: link m // ERROR "missing directive name after //gocpp:; did you mean //gocpp:link\?"
//gocpp:link -lm // ERROR "invalid library name \x22-lm\x22: write m, without -l"
//gocpp:link m pthread // a directive with a comment: no error
// gocpp:link m // WARNING "\x22// gocpp:link\x22 is an ordinary comment, not a directive: write //gocpp:link, with no space after //"

// The //gocpp: directives (Phase 01 README §2.4): unknown names get a "did
// you mean", wrong arguments get the usage, and a comment that looks like
// a broken directive gets a warning. Directive errors are load errors, so
// nothing is type-checked.
package main

func main() {}
