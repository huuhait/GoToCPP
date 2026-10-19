// phase: 05

// The go/types errors of a use of a macro of another package, in the words of the macro.

package main

import "example.com/mcross2/util"

func main() {
	_ = util.Plain(1, nil) // ERROR "util.Plain is a macro: write #util.Plain\(...\)"
	f := util.Plain        // ERROR "util.Plain is a macro: write #util.Plain\(...\)"
	_ = f
	_ = #util.Plain("x", "y") // ERROR "(?s)cannot use \x22x\x22 \(untyped string constant\) as int value in argument to #util.Plain.*macro Plain is declared at util/util.gp:5"
	_ = #util.Plain(1, 2)     // ERROR "cannot use 2 \(untyped int constant\) as string value in return statement"
	_ = util.Real(2)
}
