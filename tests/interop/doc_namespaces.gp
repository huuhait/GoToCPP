// phase: 04

package main

import (
	"fmt"

	fs "::std::filesystem" // optional: a shorter name
	"<chrono>"
	"<filesystem>"
	"<vector>"
)

func main() {
	v := std.vector[int]{1, 2, 3} // no import needed for std
	var d std.chrono.seconds      // a dotted C++ type name
	p := fs.path("/tmp/x.txt")
	fmt.Println(v.size(), d.count(), p.filename())
}

// Output:
// 3 0 "x.txt"
