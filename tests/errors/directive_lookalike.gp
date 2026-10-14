// phase: 01
//gocpp:link m
//gocpp:Link pthread // ERROR "unknown //gocpp: directive \x22Link\x22; did you mean //gocpp:link\?"
// gocpp:pkg-config sdl2 // WARNING "\x22// gocpp:pkg-config\x22 is an ordinary comment, not a directive: write //gocpp:pkg-config, with no space after //"
//  GOCPP:link z // WARNING "\x22//  GOCPP:link\x22 is an ordinary comment, not a directive: write //gocpp:link, with no space after //"
// gocpp: a sentence about gocpp, not a directive

// Comments that look like broken directives get a warning, an ordinary
// comment that mentions gocpp gets none.
package main

func main() {}
