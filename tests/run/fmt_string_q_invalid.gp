// phase: 05

// Go's %q of strings (invalid) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%q]\n", "\xff")
	fmt.Printf("[%+q]\n", "\xff")
	fmt.Printf("[%-q]\n", "\xff")
	fmt.Printf("[% q]\n", "\xff")
	fmt.Printf("[%#q]\n", "\xff")
	fmt.Printf("[%10q]\n", "\xff")
	fmt.Printf("[%-10q]\n", "\xff")
	fmt.Printf("[%010q]\n", "\xff")
	fmt.Printf("[%.3q]\n", "\xff")
	fmt.Printf("[%.0q]\n", "\xff")
	fmt.Printf("[%10.3q]\n", "\xff")
	fmt.Printf("[%-10.3q]\n", "\xff")
	fmt.Printf("[%#10q]\n", "\xff")
	fmt.Printf("[%#.2q]\n", "\xff")
	fmt.Printf("[%+#q]\n", "\xff")
	fmt.Printf("[%q]\n", "ab\xc3")
	fmt.Printf("[%+q]\n", "ab\xc3")
	fmt.Printf("[%-q]\n", "ab\xc3")
	fmt.Printf("[% q]\n", "ab\xc3")
	fmt.Printf("[%#q]\n", "ab\xc3")
	fmt.Printf("[%10q]\n", "ab\xc3")
	fmt.Printf("[%-10q]\n", "ab\xc3")
	fmt.Printf("[%010q]\n", "ab\xc3")
	fmt.Printf("[%.3q]\n", "ab\xc3")
	fmt.Printf("[%.0q]\n", "ab\xc3")
	fmt.Printf("[%10.3q]\n", "ab\xc3")
	fmt.Printf("[%-10.3q]\n", "ab\xc3")
	fmt.Printf("[%#10q]\n", "ab\xc3")
	fmt.Printf("[%#.2q]\n", "ab\xc3")
	fmt.Printf("[%+#q]\n", "ab\xc3")
	fmt.Printf("[%q]\n", "\xed\xa0\x80")
	fmt.Printf("[%+q]\n", "\xed\xa0\x80")
	fmt.Printf("[%-q]\n", "\xed\xa0\x80")
	fmt.Printf("[% q]\n", "\xed\xa0\x80")
	fmt.Printf("[%#q]\n", "\xed\xa0\x80")
	fmt.Printf("[%10q]\n", "\xed\xa0\x80")
	fmt.Printf("[%-10q]\n", "\xed\xa0\x80")
	fmt.Printf("[%010q]\n", "\xed\xa0\x80")
	fmt.Printf("[%.3q]\n", "\xed\xa0\x80")
	fmt.Printf("[%.0q]\n", "\xed\xa0\x80")
	fmt.Printf("[%10.3q]\n", "\xed\xa0\x80")
	fmt.Printf("[%-10.3q]\n", "\xed\xa0\x80")
	fmt.Printf("[%#10q]\n", "\xed\xa0\x80")
	fmt.Printf("[%#.2q]\n", "\xed\xa0\x80")
	fmt.Printf("[%+#q]\n", "\xed\xa0\x80")
	fmt.Printf("[%q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%+q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%-q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[% q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%#q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%10q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%-10q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%010q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%.3q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%.0q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%10.3q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%-10.3q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%#10q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%#.2q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%+#q]\n", "\xf4\x90\x80\x80")
	fmt.Printf("[%q]\n", "a\xffb")
	fmt.Printf("[%+q]\n", "a\xffb")
	fmt.Printf("[%-q]\n", "a\xffb")
	fmt.Printf("[% q]\n", "a\xffb")
	fmt.Printf("[%#q]\n", "a\xffb")
	fmt.Printf("[%10q]\n", "a\xffb")
	fmt.Printf("[%-10q]\n", "a\xffb")
	fmt.Printf("[%010q]\n", "a\xffb")
	fmt.Printf("[%.3q]\n", "a\xffb")
	fmt.Printf("[%.0q]\n", "a\xffb")
	fmt.Printf("[%10.3q]\n", "a\xffb")
	fmt.Printf("[%-10.3q]\n", "a\xffb")
	fmt.Printf("[%#10q]\n", "a\xffb")
	fmt.Printf("[%#.2q]\n", "a\xffb")
	fmt.Printf("[%+#q]\n", "a\xffb")
}

// Output:
// ["\xff"]
// ["\xff"]
// ["\xff"]
// ["\xff"]
// ["\xff"]
// [    "\xff"]
// ["\xff"    ]
// [0000"\xff"]
// ["\xff"]
// [""]
// [    "\xff"]
// ["\xff"    ]
// [    "\xff"]
// ["\xff"]
// ["\xff"]
// ["ab\xc3"]
// ["ab\xc3"]
// ["ab\xc3"]
// ["ab\xc3"]
// ["ab\xc3"]
// [  "ab\xc3"]
// ["ab\xc3"  ]
// [00"ab\xc3"]
// ["ab\xc3"]
// [""]
// [  "ab\xc3"]
// ["ab\xc3"  ]
// [  "ab\xc3"]
// [`ab`]
// ["ab\xc3"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// [""]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0\x80"]
// ["\xed\xa0"]
// ["\xed\xa0\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90\x80"]
// [""]
// ["\xf4\x90\x80"]
// ["\xf4\x90\x80"]
// ["\xf4\x90\x80\x80"]
// ["\xf4\x90"]
// ["\xf4\x90\x80\x80"]
// ["a\xffb"]
// ["a\xffb"]
// ["a\xffb"]
// ["a\xffb"]
// ["a\xffb"]
// [  "a\xffb"]
// ["a\xffb"  ]
// [00"a\xffb"]
// ["a\xffb"]
// [""]
// [  "a\xffb"]
// ["a\xffb"  ]
// [  "a\xffb"]
// ["a\xff"]
// ["a\xffb"]
