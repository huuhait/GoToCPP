// phase: 01

// Features of later phases are errors that name the phase (Phase 01
// README §2.9), one per feature per file, at its first use. The features
// come from Phase 05 and later, so the test keeps working while the phases
// before them are finished.
package main

import "fmt"

type Shape interface{ Area() int } // ERROR "interfaces are not supported yet \(Phase 05\)"

func Map[T any](x T) T { return x } // ERROR "generics are not supported yet \(Phase 07\)"

func main() {
	ch := make(chan int)   // ERROR "channels are not supported yet \(Phase 08\)"
	go fmt.Println(Map(1)) // ERROR "go statements are not supported yet \(Phase 08\)"
	_ = ch
	var s Shape
	_ = s
	done := make(chan bool)
	_ = done
}
