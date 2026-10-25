// phase: 05

// Package variables that are large arrays of strings or pointers: clang++ can't build such an object
// at compile time (a constant evaluation ends after 1 M steps), so they are not constinit, and a
// small array of records still is. The values come out as in Go.
package main

import "fmt"

type Rec struct {
	Name string
	Tags []string
	M    map[string]int
}

var bigS [1 << 21]string
var bigP [1 << 20]*int
var bigR [1 << 12]Rec
var bigC [100]complex128
var grid [1 << 10][1 << 10]string

func main() {
	bigS[5] = "five"
	bigS[len(bigS)-1] = "last"
	x := 3
	bigP[9] = &x
	bigR[7].Name = "seven"
	bigC[3] = 1 + 2i
	grid[1000][1000] = "g"
	n := 0
	for _, s := range bigS {
		if s != "" {
			n++
		}
	}
	nilp := 0
	for _, p := range bigP {
		if p == nil {
			nilp++
		}
	}
	fmt.Println(bigS[5], bigS[6] == "", bigS[len(bigS)-1], n, *bigP[9], nilp, bigR[7].Name, len(bigR), bigC[3], grid[1000][1000], grid[0][0] == "")
}

// Output:
// five true last 2 3 1048575 seven 4096 (1+2i) g true
