// phase: 09

// Macros of another package (a lazy assertion, a start-style macro, a wrap macro, a generic macro) and
// the forms of `!` (a call, a must function, main!): the C++ of this program must show them as plain code
// (TestPhase10ExpansionIsInline).
package main

import (
	"fmt"

	"example.com/evidence/util"
)

#util.Trace()
#util.Tally()
func work(i int) int {
	return i*3 + 1
}

func run(n int) (int, error) {
	acc := 1
	for i := 0; i < n; i++ {
		acc = util.Step!(i, acc) ^ work(i)
		#util.Assert(acc >= 0, fmt.Sprint("acc ", acc, " at ", i))
	}
	return #util.Dbg(acc), nil
}

func main!() {
	total := run!(3)
	loaded := util.Load!(4)
	fmt.Println(total, loaded, util.Count)
}
