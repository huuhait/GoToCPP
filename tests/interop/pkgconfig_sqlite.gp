// phase: 04
// pkg-config: sqlite3

// A header found through //gocpp:pkg-config: the probes and the build get the package's flags
// (check.Package.ProbeFlags, check.Package.Flags), and the program links with -lsqlite3.
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>"
)

func main() {
	var n C.int = sqlite3.sqlite3_libversion_number()
	fmt.Println(int(n) >= 3000000, int(sqlite3.sqlite3_threadsafe()) >= 0)
}

// Output:
// true true
