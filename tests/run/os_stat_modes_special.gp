// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	for _, p := range []string{"/", "/dev/null", "/dev/zero", "/proc/self", "/proc/self/exe", "/dev/stdin", "/tmp"} {
		fi, err := os.Stat(p)
		if err != nil {
			fmt.Println(p, "error")
			continue
		}
		m := fi.Mode()
		fmt.Println(p, m.IsDir(), m.IsRegular(), m&os.ModeDevice != 0, m&os.ModeCharDevice != 0, m&os.ModeNamedPipe != 0, m&os.ModeSymlink != 0)
	}
	for _, p := range []string{"/proc/self", "/proc/self/exe", "/dev/stdin"} {
		fi, err := os.Lstat(p)
		fmt.Println(p, fi.Mode()&os.ModeSymlink != 0, err)
	}
	fi, _ := os.Stat("/tmp")
	fmt.Println(fi.Mode()&os.ModeSticky != 0)
}

// Output:
// / true false false false false false
// /dev/null false false true true false false
// /dev/zero false false true true false false
// /proc/self true false false false false false
// /proc/self/exe false true false false false false
// /dev/stdin false false true true false false
// /tmp true false false false false false
// /proc/self true <nil>
// /proc/self/exe true <nil>
// /dev/stdin true <nil>
// true
