// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	os.Setenv("GPOS_C", "x")
	os.Clearenv()
	fmt.Println(len(os.Environ()), os.Getenv("GPOS_C") == "", os.Getenv("PATH") == "")
	_, ok := os.LookupEnv("HOME")
	fmt.Println(ok)
	os.Setenv("GPOS_D", "after")
	fmt.Println(os.Environ())
	os.Unsetenv("GPOS_D")
	fmt.Println(len(os.Environ()))
}

// Output:
// 0 true true
// false
// [GPOS_D=after]
// 0
