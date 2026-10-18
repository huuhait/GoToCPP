// phase: 05

// Go's %q of strings (controls) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%q]\n", "\u2028sep")
	fmt.Printf("[%+q]\n", "\u2028sep")
	fmt.Printf("[%-q]\n", "\u2028sep")
	fmt.Printf("[% q]\n", "\u2028sep")
	fmt.Printf("[%#q]\n", "\u2028sep")
	fmt.Printf("[%10q]\n", "\u2028sep")
	fmt.Printf("[%-10q]\n", "\u2028sep")
	fmt.Printf("[%010q]\n", "\u2028sep")
	fmt.Printf("[%.3q]\n", "\u2028sep")
	fmt.Printf("[%.0q]\n", "\u2028sep")
	fmt.Printf("[%10.3q]\n", "\u2028sep")
	fmt.Printf("[%-10.3q]\n", "\u2028sep")
	fmt.Printf("[%#10q]\n", "\u2028sep")
	fmt.Printf("[%#.2q]\n", "\u2028sep")
	fmt.Printf("[%+#q]\n", "\u2028sep")
	fmt.Printf("[%q]\n", "bell\a")
	fmt.Printf("[%+q]\n", "bell\a")
	fmt.Printf("[%-q]\n", "bell\a")
	fmt.Printf("[% q]\n", "bell\a")
	fmt.Printf("[%#q]\n", "bell\a")
	fmt.Printf("[%10q]\n", "bell\a")
	fmt.Printf("[%-10q]\n", "bell\a")
	fmt.Printf("[%010q]\n", "bell\a")
	fmt.Printf("[%.3q]\n", "bell\a")
	fmt.Printf("[%.0q]\n", "bell\a")
	fmt.Printf("[%10.3q]\n", "bell\a")
	fmt.Printf("[%-10.3q]\n", "bell\a")
	fmt.Printf("[%#10q]\n", "bell\a")
	fmt.Printf("[%#.2q]\n", "bell\a")
	fmt.Printf("[%+#q]\n", "bell\a")
	fmt.Printf("[%q]\n", "\x7f")
	fmt.Printf("[%+q]\n", "\x7f")
	fmt.Printf("[%-q]\n", "\x7f")
	fmt.Printf("[% q]\n", "\x7f")
	fmt.Printf("[%#q]\n", "\x7f")
	fmt.Printf("[%10q]\n", "\x7f")
	fmt.Printf("[%-10q]\n", "\x7f")
	fmt.Printf("[%010q]\n", "\x7f")
	fmt.Printf("[%.3q]\n", "\x7f")
	fmt.Printf("[%.0q]\n", "\x7f")
	fmt.Printf("[%10.3q]\n", "\x7f")
	fmt.Printf("[%-10.3q]\n", "\x7f")
	fmt.Printf("[%#10q]\n", "\x7f")
	fmt.Printf("[%#.2q]\n", "\x7f")
	fmt.Printf("[%+#q]\n", "\x7f")
	fmt.Printf("[%q]\n", "\x00nul")
	fmt.Printf("[%+q]\n", "\x00nul")
	fmt.Printf("[%-q]\n", "\x00nul")
	fmt.Printf("[% q]\n", "\x00nul")
	fmt.Printf("[%#q]\n", "\x00nul")
	fmt.Printf("[%10q]\n", "\x00nul")
	fmt.Printf("[%-10q]\n", "\x00nul")
	fmt.Printf("[%010q]\n", "\x00nul")
	fmt.Printf("[%.3q]\n", "\x00nul")
	fmt.Printf("[%.0q]\n", "\x00nul")
	fmt.Printf("[%10.3q]\n", "\x00nul")
	fmt.Printf("[%-10.3q]\n", "\x00nul")
	fmt.Printf("[%#10q]\n", "\x00nul")
	fmt.Printf("[%#.2q]\n", "\x00nul")
	fmt.Printf("[%+#q]\n", "\x00nul")
	fmt.Printf("[%q]\n", "\ufeffbom")
	fmt.Printf("[%+q]\n", "\ufeffbom")
	fmt.Printf("[%-q]\n", "\ufeffbom")
	fmt.Printf("[% q]\n", "\ufeffbom")
	fmt.Printf("[%#q]\n", "\ufeffbom")
	fmt.Printf("[%10q]\n", "\ufeffbom")
	fmt.Printf("[%-10q]\n", "\ufeffbom")
	fmt.Printf("[%010q]\n", "\ufeffbom")
	fmt.Printf("[%.3q]\n", "\ufeffbom")
	fmt.Printf("[%.0q]\n", "\ufeffbom")
	fmt.Printf("[%10.3q]\n", "\ufeffbom")
	fmt.Printf("[%-10.3q]\n", "\ufeffbom")
	fmt.Printf("[%#10q]\n", "\ufeffbom")
	fmt.Printf("[%#.2q]\n", "\ufeffbom")
	fmt.Printf("[%+#q]\n", "\ufeffbom")
	fmt.Printf("[%q]\n", "\x1b[0m")
	fmt.Printf("[%+q]\n", "\x1b[0m")
	fmt.Printf("[%-q]\n", "\x1b[0m")
	fmt.Printf("[% q]\n", "\x1b[0m")
	fmt.Printf("[%#q]\n", "\x1b[0m")
	fmt.Printf("[%10q]\n", "\x1b[0m")
	fmt.Printf("[%-10q]\n", "\x1b[0m")
	fmt.Printf("[%010q]\n", "\x1b[0m")
	fmt.Printf("[%.3q]\n", "\x1b[0m")
	fmt.Printf("[%.0q]\n", "\x1b[0m")
	fmt.Printf("[%10.3q]\n", "\x1b[0m")
	fmt.Printf("[%-10.3q]\n", "\x1b[0m")
	fmt.Printf("[%#10q]\n", "\x1b[0m")
	fmt.Printf("[%#.2q]\n", "\x1b[0m")
	fmt.Printf("[%+#q]\n", "\x1b[0m")
	fmt.Printf("[%q]\n", "\u0085next")
	fmt.Printf("[%+q]\n", "\u0085next")
	fmt.Printf("[%-q]\n", "\u0085next")
	fmt.Printf("[% q]\n", "\u0085next")
	fmt.Printf("[%#q]\n", "\u0085next")
	fmt.Printf("[%10q]\n", "\u0085next")
	fmt.Printf("[%-10q]\n", "\u0085next")
	fmt.Printf("[%010q]\n", "\u0085next")
	fmt.Printf("[%.3q]\n", "\u0085next")
	fmt.Printf("[%.0q]\n", "\u0085next")
	fmt.Printf("[%10.3q]\n", "\u0085next")
	fmt.Printf("[%-10.3q]\n", "\u0085next")
	fmt.Printf("[%#10q]\n", "\u0085next")
	fmt.Printf("[%#.2q]\n", "\u0085next")
	fmt.Printf("[%+#q]\n", "\u0085next")
}

// Output:
// ["\u2028sep"]
// ["\u2028sep"]
// ["\u2028sep"]
// ["\u2028sep"]
// [` sep`]
// ["\u2028sep"]
// ["\u2028sep"]
// ["\u2028sep"]
// ["\u2028se"]
// [""]
// ["\u2028se"]
// ["\u2028se"]
// [    ` sep`]
// [` s`]
// [` sep`]
// ["bell\a"]
// ["bell\a"]
// ["bell\a"]
// ["bell\a"]
// ["bell\a"]
// [  "bell\a"]
// ["bell\a"  ]
// [00"bell\a"]
// ["bel"]
// [""]
// [     "bel"]
// ["bel"     ]
// [  "bell\a"]
// [`be`]
// ["bell\a"]
// ["\x7f"]
// ["\x7f"]
// ["\x7f"]
// ["\x7f"]
// ["\x7f"]
// [    "\x7f"]
// ["\x7f"    ]
// [0000"\x7f"]
// ["\x7f"]
// [""]
// [    "\x7f"]
// ["\x7f"    ]
// [    "\x7f"]
// ["\x7f"]
// ["\x7f"]
// ["\x00nul"]
// ["\x00nul"]
// ["\x00nul"]
// ["\x00nul"]
// ["\x00nul"]
// [ "\x00nul"]
// ["\x00nul" ]
// [0"\x00nul"]
// ["\x00nu"]
// [""]
// [  "\x00nu"]
// ["\x00nu"  ]
// [ "\x00nul"]
// ["\x00n"]
// ["\x00nul"]
// ["\ufeffbom"]
// ["\ufeffbom"]
// ["\ufeffbom"]
// ["\ufeffbom"]
// ["\ufeffbom"]
// ["\ufeffbom"]
// ["\ufeffbom"]
// ["\ufeffbom"]
// ["\ufeffbo"]
// [""]
// ["\ufeffbo"]
// ["\ufeffbo"]
// ["\ufeffbom"]
// ["\ufeffb"]
// ["\ufeffbom"]
// ["\x1b[0m"]
// ["\x1b[0m"]
// ["\x1b[0m"]
// ["\x1b[0m"]
// ["\x1b[0m"]
// [ "\x1b[0m"]
// ["\x1b[0m" ]
// [0"\x1b[0m"]
// ["\x1b[0"]
// [""]
// [  "\x1b[0"]
// ["\x1b[0"  ]
// [ "\x1b[0m"]
// ["\x1b["]
// ["\x1b[0m"]
// ["\u0085next"]
// ["\u0085next"]
// ["\u0085next"]
// ["\u0085next"]
// [`next`]
// ["\u0085next"]
// ["\u0085next"]
// ["\u0085next"]
// ["\u0085ne"]
// [""]
// ["\u0085ne"]
// ["\u0085ne"]
// [   `next`]
// [`n`]
// [`next`]
