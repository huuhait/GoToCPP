// phase: 09

//go:build windows

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
// false open /proc/self/stat: The system cannot find the path specified.
// false open /proc/self/status: The system cannot find the path specified.
// 0 open /dev/null: The system cannot find the path specified.
// false open /proc/self/cmdline: The system cannot find the path specified.
