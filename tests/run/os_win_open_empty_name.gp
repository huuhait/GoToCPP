// phase: 09

//go:build windows

package main

import (
	"fmt"
	"os"
)

func main() {
	_, err := os.Open("")
	fmt.Println(err)
	_, err = os.Create("")
	fmt.Println(err)
	_, err = os.Stat("")
	fmt.Println(err)
	_, err = os.ReadFile("")
	fmt.Println(err)
	fmt.Println(os.Remove(""))
	fmt.Println(os.Mkdir("", 0o755))
	fmt.Println(os.Chdir(""))
	_, err = os.ReadDir("")
	fmt.Println(err)
}

// Output:
// open : The system cannot find the file specified.
// open : The system cannot find the file specified.
// Stat : The system cannot find the path specified.
// open : The system cannot find the file specified.
// remove : The system cannot find the path specified.
// mkdir : The system cannot find the path specified.
// chdir : The filename, directory name, or volume label syntax is incorrect.
// open : The system cannot find the file specified.
