// phase: 05

// #Derive(String) on a type whose constants are declared in several files of the package, with
// values that two constants share (Extra here and Late in z_modes.gp): the first constant in the
// order of the files (a_modes.gp, main.gp, z_modes.gp) names the value, as in Go's stringer tool.
// The twin writes String by hand.

package main

import "fmt"

#Derive(String)
type Mode uint8

const (
	Execute   Mode = 4
	Extra     Mode = 8
	ReadAlias      = Read
	Both           = Read | Write
)

func main() {
	fmt.Println(Read, Write, Execute, ReadAlias, Both, Everything)
	fmt.Println(Mode(0), Mode(6), Mode(255))
	fmt.Println(Late, AliasOfWrite, Mode(3).String())
	fmt.Printf("%v %d %s|%6s|\n", Execute, Execute, Both, Write)
}

// Output:
// Read Write Execute Read Both Everything
// Mode(0) Mode(6) Mode(255)
// Extra Write Both
// Execute 4 Both| Write|
