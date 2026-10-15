// phase: 04

package main

import (
	"<cerrno>"
	"<chrono>"
	"<cstdio>"
	"<ctime>"
	"<filesystem>"
	"<iostream>"
	"<string>"
	"<sys/stat.h>"
	"<vector>"
	fs "::std::filesystem"

	"./cnames.h"
)

type Bool bool

func pair() (int, Bool) { return 1, true }

func main() {
	cstdio.printf("%d %s %.1f %c\n", 5, "x", 2.5, 'y') // ::printf, C literals
	ctime.time(nil)                                    // ::time(nullptr)
	n, e := cstdio.EOF, cerrno.errno                   // macros: unqualified
	var v std.vector[int]                              // a namespace
	var p fs.path                                      // a namespace rename
	var w cnames.vec3                                  // the namespace cnames, not the C-level vec3
	k := cnames.color_value(cnames.Color.Red)          // C-level names, an enum class's value
	_ = std.chrono.steady_clock.now()                  // a static member function of a type
	m := std.string.npos                               // a static member of a type
	std.cout.flush()                                   // a member of an object
	var st stat.struct_stat                            // cgo's tag names
	var md cnames.enum_mode = cnames.MODE_B
	var num cnames.union_number
	var pt cnames.struct_point
	x := cnames.WHITE
	a, b := pair()
	_, _, _, _, _, _, _, _, _, _, _, _, _, _ = n, e, v, p, w, k, m, st, md, num, pt, x, a, b
}
