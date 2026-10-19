// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	b, err := os.ReadFile("/proc/self/stat")
	fmt.Println(len(b) > 10, err)
	b, err = os.ReadFile("/proc/self/status")
	fmt.Println(len(b) > 100, err)
	b, err = os.ReadFile("/dev/null")
	fmt.Println(len(b), err)
	b, err = os.ReadFile("/proc/self/cmdline")
	fmt.Println(len(b) > 0, err)
}

// Output:
// true <nil>
// true <nil>
// 0 <nil>
// true <nil>
