// phase: 03

// What fmt can't do in Phase 03 (README §2.10): the verbs %v %+v %d %s %q %t %f with flags,
// width and precision work; %T and %w come in Phase 05, the # flag, * and argument indexes and
// the other verbs of Go's fmt in Phase 09 (each feature is reported once per file), and so does
// a format that is not a constant. A nil argument is a nil interface value (Phase 05).
package main

import "fmt"

func main() {
	x := []int{1}
	fmt.Printf("%5.2f %-8v %+d %q %08.3f % d\n", 1.5, x, 2, "s", 2.5, 3)
	fmt.Printf("%#v\n", x) // ERROR "the # flag, \* and argument indexes in format strings are not supported yet \(Phase 09\)"
	fmt.Printf("%T\n", x)  // ERROR "the format verb %T is not supported yet \(Phase 05\)"
	s := fmt.Sprintf("%x", 255) // ERROR "the format verb %x is not supported yet \(Phase 09\)"
	fmt.Println(s, fmt.Sprintf("%*d", 5, 1), fmt.Sprintf("%[1]d", 1))
	fmt.Println(nil) // ERROR "any values are not supported yet \(Phase 05\)"
	f := "%d"
	fmt.Printf(f, 1) // ERROR "format strings that are not constants are not supported yet \(Phase 09\)"
}
