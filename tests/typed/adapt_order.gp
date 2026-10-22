// phase: 12
// opaque: reject

// What the callee stored through an adapted &x is in x as soon as the call has returned, so the rest of
// the expression sees it: in a condition, in the next call of the same expression and in a loop
// condition. The arguments of the call are read before the call, as in Go.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var stored int64
	if adapt.store_five(&stored) == 0 && stored > 0 {
		fmt.Println("condition sees", stored)
	} else {
		fmt.Println("condition is stale", stored)
	}

	var chained int64
	fmt.Println(adapt.bump(&chained) == 1 && adapt.bump(&chained) == 2 && chained == 2)

	var counter int64
	guard := 0
	for adapt.next_value(&counter) < 3 && counter < 10 {
		guard++
		if guard > 20 {
			fmt.Println("the loop does not end")
			break
		}
		fmt.Println("loop", counter)
	}
	fmt.Println("after", counter)

	var doubled int64 = 4
	result := adapt.bump_by(&doubled, int(doubled))
	fmt.Println(result, doubled)
}

// Output:
// condition sees 5
// true
// loop 1
// loop 2
// after 3
// 8 8
