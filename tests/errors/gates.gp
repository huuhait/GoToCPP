// phase: 01

// Features of later phases are errors that name the phase (Phase 01
// README §2.9), one per feature per file, at its first use. The features
// come from Phase 08 and later, so the test keeps working while the phases
// before them are finished (the gates of earlier phases are lifted: Phase 05
// step 5.12). Interfaces, errors and the other Phase 05 features are tested
// in the other tests of this folder and in internal/check, which runs the
// gates at every phase.
package main

import "fmt"

func main() {
	ch := make(chan int) // ERROR "channels are not supported yet \(Phase 08\)"
	go fmt.Println(1)    // ERROR "go statements are not supported yet \(Phase 08\)"
	_ = ch
	done := make(chan bool)
	_ = done
}
