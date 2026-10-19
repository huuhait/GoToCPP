// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	p, err := os.Executable()
	fmt.Println(err, len(p) > 1, p[0] == '/')
	fi, err := os.Stat(p)
	fmt.Println(err, fi.Mode().IsRegular(), fi.Mode().Perm()&0o100 != 0)
	h, err := os.Hostname()
	fmt.Println(err, len(h) > 0)
	wd, err := os.Getwd()
	fmt.Println(err, len(wd) > 0 && wd[0] == '/')
}

// Output:
// <nil> true true
// <nil> true true
// <nil> true
// <nil> true
