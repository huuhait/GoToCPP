// phase: 04

// :: or . after a C++ name, from the kind probe: a static member of a class
// (std.string.npos, std.chrono.steady_clock.now, std.numeric_limits[int].max) takes ::, a
// member of an object (std.cout.good) takes .; namespace renames (fs, chrono) are their
// namespaces, and a namespace inside a namespace (std.chrono) is one too.
package main

import (
	"fmt"

	"<chrono>"
	"<filesystem>"
	"<iostream>"
	"<limits>"
	"<string>"
	chrono "::std::chrono"
	fs "::std::filesystem"
)

func main() {
	fmt.Println(int(std.string.npos) == -1, bool(std.cout.good()))
	std.cout.flush()
	start := std.chrono.steady_clock.now()
	_ = chrono.steady_clock.now()
	var p fs.path
	var d chrono.seconds
	_, _, _ = start, p, d
	fmt.Println(int(std.numeric_limits[int].max()) == 1<<63-1, int(std.numeric_limits[int8].digits))
	fmt.Println(int(fs.path.preferred_separator), bool(std.chrono.steady_clock.is_steady))
}

// Output:
// true true
// true 7
// 47 true
