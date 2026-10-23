// phase: 09

// io/fs.Glob recurses once for each separator of the pattern, and Go refuses a pattern with 10000
// of them (CVE-2022-30630); unlike path/filepath's, the port has no stack guard. With the main
// thread's RLIMIT_STACK raised to 1 GiB at start (the runtime's thread layer) a pattern of 7000
// separators fits, as in Go; with the system's 8 MiB about 6000 ended the program with "fatal
// error: stack overflow". (Each level copies the pattern: the time grows with the square of the
// separators, so the test stays near that limit, and Go's TestCVE202230630 pattern of 10001, which
// Go refuses after 10000 levels, is left to that test: it runs in a goroutine, which keeps 8 MiB.)
package main

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
)

func main() {
	for _, n := range []int{200, 7000} {
		// No name in the folder starts with this, so every level of the recursion has no match
		// (the time doesn't grow with the folder).
		m, err := fs.Glob(os.DirFS("."), "no-such-name-*"+strings.Repeat("/", n))
		fmt.Println(n, "separators:", len(m), err)
	}
}

// Output:
// 200 separators: 0 <nil>
// 7000 separators: 0 <nil>
