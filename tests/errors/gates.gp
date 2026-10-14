// phase: 01

// Features of later phases are errors that name the phase (Phase 01
// README §2.9), one per feature per file, at its first use.
package main

import "fmt"

type Point struct{ X, Y int } // ERROR "type declarations are not supported yet \(Phase 03\)" "composite types are not supported yet \(Phase 03\)"

func Map[T any](x T) T { return x } // ERROR "generics are not supported yet \(Phase 07\)"

func main() {
	p := Point{1, 2}
	ch := make(chan int)        // ERROR "channels are not supported yet \(Phase 08\)" "built-in function make is not supported yet \(Phase 03\)"
	go fmt.Println(p.X, Map(1)) // ERROR "go statements are not supported yet \(Phase 08\)"
	_ = ch
	q := Point{3, 4}
	_ = q
}
