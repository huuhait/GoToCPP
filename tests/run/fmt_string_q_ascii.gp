// phase: 05

// Go's %q of strings (ascii) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%q]\n", "")
	fmt.Printf("[%+q]\n", "")
	fmt.Printf("[%-q]\n", "")
	fmt.Printf("[% q]\n", "")
	fmt.Printf("[%#q]\n", "")
	fmt.Printf("[%10q]\n", "")
	fmt.Printf("[%-10q]\n", "")
	fmt.Printf("[%010q]\n", "")
	fmt.Printf("[%.3q]\n", "")
	fmt.Printf("[%.0q]\n", "")
	fmt.Printf("[%10.3q]\n", "")
	fmt.Printf("[%-10.3q]\n", "")
	fmt.Printf("[%#10q]\n", "")
	fmt.Printf("[%#.2q]\n", "")
	fmt.Printf("[%+#q]\n", "")
	fmt.Printf("[%q]\n", "hello")
	fmt.Printf("[%+q]\n", "hello")
	fmt.Printf("[%-q]\n", "hello")
	fmt.Printf("[% q]\n", "hello")
	fmt.Printf("[%#q]\n", "hello")
	fmt.Printf("[%10q]\n", "hello")
	fmt.Printf("[%-10q]\n", "hello")
	fmt.Printf("[%010q]\n", "hello")
	fmt.Printf("[%.3q]\n", "hello")
	fmt.Printf("[%.0q]\n", "hello")
	fmt.Printf("[%10.3q]\n", "hello")
	fmt.Printf("[%-10.3q]\n", "hello")
	fmt.Printf("[%#10q]\n", "hello")
	fmt.Printf("[%#.2q]\n", "hello")
	fmt.Printf("[%+#q]\n", "hello")
	fmt.Printf("[%q]\n", "hello, world")
	fmt.Printf("[%+q]\n", "hello, world")
	fmt.Printf("[%-q]\n", "hello, world")
	fmt.Printf("[% q]\n", "hello, world")
	fmt.Printf("[%#q]\n", "hello, world")
	fmt.Printf("[%10q]\n", "hello, world")
	fmt.Printf("[%-10q]\n", "hello, world")
	fmt.Printf("[%010q]\n", "hello, world")
	fmt.Printf("[%.3q]\n", "hello, world")
	fmt.Printf("[%.0q]\n", "hello, world")
	fmt.Printf("[%10.3q]\n", "hello, world")
	fmt.Printf("[%-10.3q]\n", "hello, world")
	fmt.Printf("[%#10q]\n", "hello, world")
	fmt.Printf("[%#.2q]\n", "hello, world")
	fmt.Printf("[%+#q]\n", "hello, world")
	fmt.Printf("[%q]\n", "a b\tc")
	fmt.Printf("[%+q]\n", "a b\tc")
	fmt.Printf("[%-q]\n", "a b\tc")
	fmt.Printf("[% q]\n", "a b\tc")
	fmt.Printf("[%#q]\n", "a b\tc")
	fmt.Printf("[%10q]\n", "a b\tc")
	fmt.Printf("[%-10q]\n", "a b\tc")
	fmt.Printf("[%010q]\n", "a b\tc")
	fmt.Printf("[%.3q]\n", "a b\tc")
	fmt.Printf("[%.0q]\n", "a b\tc")
	fmt.Printf("[%10.3q]\n", "a b\tc")
	fmt.Printf("[%-10.3q]\n", "a b\tc")
	fmt.Printf("[%#10q]\n", "a b\tc")
	fmt.Printf("[%#.2q]\n", "a b\tc")
	fmt.Printf("[%+#q]\n", "a b\tc")
	fmt.Printf("[%q]\n", "line1\nline2")
	fmt.Printf("[%+q]\n", "line1\nline2")
	fmt.Printf("[%-q]\n", "line1\nline2")
	fmt.Printf("[% q]\n", "line1\nline2")
	fmt.Printf("[%#q]\n", "line1\nline2")
	fmt.Printf("[%10q]\n", "line1\nline2")
	fmt.Printf("[%-10q]\n", "line1\nline2")
	fmt.Printf("[%010q]\n", "line1\nline2")
	fmt.Printf("[%.3q]\n", "line1\nline2")
	fmt.Printf("[%.0q]\n", "line1\nline2")
	fmt.Printf("[%10.3q]\n", "line1\nline2")
	fmt.Printf("[%-10.3q]\n", "line1\nline2")
	fmt.Printf("[%#10q]\n", "line1\nline2")
	fmt.Printf("[%#.2q]\n", "line1\nline2")
	fmt.Printf("[%+#q]\n", "line1\nline2")
	fmt.Printf("[%q]\n", "quote\"d")
	fmt.Printf("[%+q]\n", "quote\"d")
	fmt.Printf("[%-q]\n", "quote\"d")
	fmt.Printf("[% q]\n", "quote\"d")
	fmt.Printf("[%#q]\n", "quote\"d")
	fmt.Printf("[%10q]\n", "quote\"d")
	fmt.Printf("[%-10q]\n", "quote\"d")
	fmt.Printf("[%010q]\n", "quote\"d")
	fmt.Printf("[%.3q]\n", "quote\"d")
	fmt.Printf("[%.0q]\n", "quote\"d")
	fmt.Printf("[%10.3q]\n", "quote\"d")
	fmt.Printf("[%-10.3q]\n", "quote\"d")
	fmt.Printf("[%#10q]\n", "quote\"d")
	fmt.Printf("[%#.2q]\n", "quote\"d")
	fmt.Printf("[%+#q]\n", "quote\"d")
	fmt.Printf("[%q]\n", "back\\slash")
	fmt.Printf("[%+q]\n", "back\\slash")
	fmt.Printf("[%-q]\n", "back\\slash")
	fmt.Printf("[% q]\n", "back\\slash")
	fmt.Printf("[%#q]\n", "back\\slash")
	fmt.Printf("[%10q]\n", "back\\slash")
	fmt.Printf("[%-10q]\n", "back\\slash")
	fmt.Printf("[%010q]\n", "back\\slash")
	fmt.Printf("[%.3q]\n", "back\\slash")
	fmt.Printf("[%.0q]\n", "back\\slash")
	fmt.Printf("[%10.3q]\n", "back\\slash")
	fmt.Printf("[%-10.3q]\n", "back\\slash")
	fmt.Printf("[%#10q]\n", "back\\slash")
	fmt.Printf("[%#.2q]\n", "back\\slash")
	fmt.Printf("[%+#q]\n", "back\\slash")
	fmt.Printf("[%q]\n", "`tick`")
	fmt.Printf("[%+q]\n", "`tick`")
	fmt.Printf("[%-q]\n", "`tick`")
	fmt.Printf("[% q]\n", "`tick`")
	fmt.Printf("[%#q]\n", "`tick`")
	fmt.Printf("[%10q]\n", "`tick`")
	fmt.Printf("[%-10q]\n", "`tick`")
	fmt.Printf("[%010q]\n", "`tick`")
	fmt.Printf("[%.3q]\n", "`tick`")
	fmt.Printf("[%.0q]\n", "`tick`")
	fmt.Printf("[%10.3q]\n", "`tick`")
	fmt.Printf("[%-10.3q]\n", "`tick`")
	fmt.Printf("[%#10q]\n", "`tick`")
	fmt.Printf("[%#.2q]\n", "`tick`")
	fmt.Printf("[%+#q]\n", "`tick`")
}

// Output:
// [""]
// [""]
// [""]
// [""]
// [``]
// [        ""]
// [""        ]
// [00000000""]
// [""]
// [""]
// [        ""]
// [""        ]
// [        ``]
// [``]
// [``]
// ["hello"]
// ["hello"]
// ["hello"]
// ["hello"]
// [`hello`]
// [   "hello"]
// ["hello"   ]
// [000"hello"]
// ["hel"]
// [""]
// [     "hel"]
// ["hel"     ]
// [   `hello`]
// [`he`]
// [`hello`]
// ["hello, world"]
// ["hello, world"]
// ["hello, world"]
// ["hello, world"]
// [`hello, world`]
// ["hello, world"]
// ["hello, world"]
// ["hello, world"]
// ["hel"]
// [""]
// [     "hel"]
// ["hel"     ]
// [`hello, world`]
// [`he`]
// [`hello, world`]
// ["a b\tc"]
// ["a b\tc"]
// ["a b\tc"]
// ["a b\tc"]
// [`a b	c`]
// [  "a b\tc"]
// ["a b\tc"  ]
// [00"a b\tc"]
// ["a b"]
// [""]
// [     "a b"]
// ["a b"     ]
// [   `a b	c`]
// [`a `]
// [`a b	c`]
// ["line1\nline2"]
// ["line1\nline2"]
// ["line1\nline2"]
// ["line1\nline2"]
// ["line1\nline2"]
// ["line1\nline2"]
// ["line1\nline2"]
// ["line1\nline2"]
// ["lin"]
// [""]
// [     "lin"]
// ["lin"     ]
// ["line1\nline2"]
// [`li`]
// ["line1\nline2"]
// ["quote\"d"]
// ["quote\"d"]
// ["quote\"d"]
// ["quote\"d"]
// [`quote"d`]
// ["quote\"d"]
// ["quote\"d"]
// ["quote\"d"]
// ["quo"]
// [""]
// [     "quo"]
// ["quo"     ]
// [ `quote"d`]
// [`qu`]
// [`quote"d`]
// ["back\\slash"]
// ["back\\slash"]
// ["back\\slash"]
// ["back\\slash"]
// [`back\slash`]
// ["back\\slash"]
// ["back\\slash"]
// ["back\\slash"]
// ["bac"]
// [""]
// [     "bac"]
// ["bac"     ]
// [`back\slash`]
// [`ba`]
// [`back\slash`]
// ["`tick`"]
// ["`tick`"]
// ["`tick`"]
// ["`tick`"]
// ["`tick`"]
// [  "`tick`"]
// ["`tick`"  ]
// [00"`tick`"]
// ["`ti"]
// [""]
// [     "`ti"]
// ["`ti"     ]
// [  "`tick`"]
// ["`t"]
// ["`tick`"]
