// phase: 09

// Every Format method of one Printf call gets the same fmt.State (Go: one printer per call), so a saved State compares equal.
package main

import "fmt"

var prev fmt.State

type F int

func (f F) Format(s fmt.State, c rune) {
	fmt.Fprintf(s, "%d:%v", int(f), prev == s)
	prev = s
}

func main() {
	prev = nil
	fmt.Printf("%v %v %v\n", F(1), F(2), []F{3, 4})
}

// Output:
// 1:false 2:true [3:true 4:true]
