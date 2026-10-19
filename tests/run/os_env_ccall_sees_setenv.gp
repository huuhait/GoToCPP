// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	os.Setenv("GPOS_VISIBLE", "to-all")
	fmt.Println(os.Getenv("GPOS_VISIBLE"))
	os.Unsetenv("GPOS_VISIBLE")
	fmt.Printf("%q\n", os.Getenv("GPOS_VISIBLE"))
	os.Setenv("HOME", "/h")
	h, err := os.UserHomeDir()
	fmt.Println(h, err)
}

// Output:
// to-all
// ""
// /h <nil>
