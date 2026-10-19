// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Printf("%q\n", os.Getenv("GPOS_UNSET_VAR"))
	fmt.Println(os.Setenv("GPOS_A", "alpha"))
	fmt.Printf("%q\n", os.Getenv("GPOS_A"))
	fmt.Println(os.Setenv("GPOS_A", "beta"))
	fmt.Printf("%q\n", os.Getenv("GPOS_A"))
	fmt.Println(os.Setenv("GPOS_A", ""))
	v, ok := os.LookupEnv("GPOS_A")
	fmt.Printf("%q %v\n", v, ok)
	fmt.Println(os.Unsetenv("GPOS_A"))
	v, ok = os.LookupEnv("GPOS_A")
	fmt.Printf("%q %v\n", v, ok)
	fmt.Println(os.Unsetenv("GPOS_A"))
	fmt.Println(os.Unsetenv("GPOS_NEVER"))
	fmt.Println(os.Setenv("GPOS_B", "with=equals and spaces"))
	fmt.Printf("%q\n", os.Getenv("GPOS_B"))
	fmt.Println(os.Setenv("GPOS_\u00e9", "unicode"))
	fmt.Printf("%q\n", os.Getenv("GPOS_\u00e9"))
	fmt.Printf("%q\n", os.Getenv(""))
}

// Output:
// ""
// <nil>
// "alpha"
// <nil>
// "beta"
// <nil>
// "" true
// <nil>
// "" false
// <nil>
// <nil>
// <nil>
// "with=equals and spaces"
// <nil>
// "unicode"
// ""
