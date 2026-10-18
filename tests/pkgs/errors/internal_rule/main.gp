// phase: 06

// Go's internal rule: lib/internal/secret can be imported only by the packages under lib
// (lib itself does, and it is fine).
package main

import (
	"example.com/e1/lib"
	"example.com/e1/lib/internal/secret" // ERROR "use of internal package example.com/e1/lib/internal/secret not allowed"
)

func main() {
	_ = lib.L()
	_ = secret.S()
}
