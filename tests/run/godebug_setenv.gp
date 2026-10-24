// phase: 09

// A $GODEBUG setting follows os.Setenv and os.Unsetenv (math/rand's randseednop: rand.Seed is a no-op
// unless randseednop=0; the last name=value of the variable wins): internal/godebug keeps the value
// it read until the program sets $GODEBUG again, as Go's cache does.
package main

import (
	"fmt"
	"math/rand"
	"os"
)

func seeded() bool {
	rand.Seed(42)
	a := rand.Int63()
	rand.Seed(42)
	b := rand.Int63()
	return a == b
}

func main() {
	os.Unsetenv("GODEBUG")
	fmt.Println("default:", seeded())
	os.Setenv("GODEBUG", "randseednop=0")
	fmt.Println("randseednop=0:", seeded())
	os.Setenv("GODEBUG", "randseednop=0,randseednop=1")
	fmt.Println("last wins (1):", seeded())
	os.Setenv("GODEBUG", "x=1,randseednop=0")
	fmt.Println("randseednop=0 again:", seeded())
}

// Output:
// default: false
// randseednop=0: true
// last wins (1): false
// randseednop=0 again: true
