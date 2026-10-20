// phase: 09
// divergence: D24

// filepath.Glob recurses once for each separator of the pattern. Go refuses a pattern with 10000 (ErrBadPattern, CVE-2022-30632) and grows its
// stack for fewer; gocpp's stacks are fixed (D24), so patches/path__filepath.rules makes Glob give the same error when the runtime says that
// the stack has no more room. A pattern of 10001 separators is the error everywhere; one of 200 is fine everywhere; in between the answer
// depends on the stack of the compiler and the mode, but it is one of the two and never a crash.

package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func main() {
	for _, n := range []int{200, 5000, 6400, 9999, 10001, 100000} {
		m, err := filepath.Glob("/*" + strings.Repeat("/", n))
		switch {
		case n == 200:
			fmt.Println(n, "separators:", m, err)
		case n >= 10001:
			fmt.Println(n, "separators:", m, err)
		default:
			fmt.Println(n, "separators: no crash:", err == nil || err == filepath.ErrBadPattern)
		}
	}
	done := make(chan error)
	go func() {
		_, err := filepath.Glob("/*" + strings.Repeat("/", 100000))
		done <- err
	}()
	fmt.Println("in a goroutine:", <-done)
}

// Output:
// 200 separators: [] <nil>
// 5000 separators: no crash: true
// 6400 separators: no crash: true
// 9999 separators: no crash: true
// 10001 separators: [] syntax error in pattern
// 100000 separators: [] syntax error in pattern
// in a goroutine: syntax error in pattern
