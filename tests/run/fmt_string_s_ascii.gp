// phase: 05

// Go's %s of strings (ascii) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%s]\n", "")
	fmt.Printf("[%+s]\n", "")
	fmt.Printf("[%-s]\n", "")
	fmt.Printf("[% s]\n", "")
	fmt.Printf("[%#s]\n", "")
	fmt.Printf("[%10s]\n", "")
	fmt.Printf("[%-10s]\n", "")
	fmt.Printf("[%010s]\n", "")
	fmt.Printf("[%.3s]\n", "")
	fmt.Printf("[%.0s]\n", "")
	fmt.Printf("[%10.3s]\n", "")
	fmt.Printf("[%-10.3s]\n", "")
	fmt.Printf("[%#10s]\n", "")
	fmt.Printf("[%#.2s]\n", "")
	fmt.Printf("[%+#s]\n", "")
	fmt.Printf("[%s]\n", "hello")
	fmt.Printf("[%+s]\n", "hello")
	fmt.Printf("[%-s]\n", "hello")
	fmt.Printf("[% s]\n", "hello")
	fmt.Printf("[%#s]\n", "hello")
	fmt.Printf("[%10s]\n", "hello")
	fmt.Printf("[%-10s]\n", "hello")
	fmt.Printf("[%010s]\n", "hello")
	fmt.Printf("[%.3s]\n", "hello")
	fmt.Printf("[%.0s]\n", "hello")
	fmt.Printf("[%10.3s]\n", "hello")
	fmt.Printf("[%-10.3s]\n", "hello")
	fmt.Printf("[%#10s]\n", "hello")
	fmt.Printf("[%#.2s]\n", "hello")
	fmt.Printf("[%+#s]\n", "hello")
	fmt.Printf("[%s]\n", "hello, world")
	fmt.Printf("[%+s]\n", "hello, world")
	fmt.Printf("[%-s]\n", "hello, world")
	fmt.Printf("[% s]\n", "hello, world")
	fmt.Printf("[%#s]\n", "hello, world")
	fmt.Printf("[%10s]\n", "hello, world")
	fmt.Printf("[%-10s]\n", "hello, world")
	fmt.Printf("[%010s]\n", "hello, world")
	fmt.Printf("[%.3s]\n", "hello, world")
	fmt.Printf("[%.0s]\n", "hello, world")
	fmt.Printf("[%10.3s]\n", "hello, world")
	fmt.Printf("[%-10.3s]\n", "hello, world")
	fmt.Printf("[%#10s]\n", "hello, world")
	fmt.Printf("[%#.2s]\n", "hello, world")
	fmt.Printf("[%+#s]\n", "hello, world")
	fmt.Printf("[%s]\n", "a b\tc")
	fmt.Printf("[%+s]\n", "a b\tc")
	fmt.Printf("[%-s]\n", "a b\tc")
	fmt.Printf("[% s]\n", "a b\tc")
	fmt.Printf("[%#s]\n", "a b\tc")
	fmt.Printf("[%10s]\n", "a b\tc")
	fmt.Printf("[%-10s]\n", "a b\tc")
	fmt.Printf("[%010s]\n", "a b\tc")
	fmt.Printf("[%.3s]\n", "a b\tc")
	fmt.Printf("[%.0s]\n", "a b\tc")
	fmt.Printf("[%10.3s]\n", "a b\tc")
	fmt.Printf("[%-10.3s]\n", "a b\tc")
	fmt.Printf("[%#10s]\n", "a b\tc")
	fmt.Printf("[%#.2s]\n", "a b\tc")
	fmt.Printf("[%+#s]\n", "a b\tc")
	fmt.Printf("[%s]\n", "line1\nline2")
	fmt.Printf("[%+s]\n", "line1\nline2")
	fmt.Printf("[%-s]\n", "line1\nline2")
	fmt.Printf("[% s]\n", "line1\nline2")
	fmt.Printf("[%#s]\n", "line1\nline2")
	fmt.Printf("[%10s]\n", "line1\nline2")
	fmt.Printf("[%-10s]\n", "line1\nline2")
	fmt.Printf("[%010s]\n", "line1\nline2")
	fmt.Printf("[%.3s]\n", "line1\nline2")
	fmt.Printf("[%.0s]\n", "line1\nline2")
	fmt.Printf("[%10.3s]\n", "line1\nline2")
	fmt.Printf("[%-10.3s]\n", "line1\nline2")
	fmt.Printf("[%#10s]\n", "line1\nline2")
	fmt.Printf("[%#.2s]\n", "line1\nline2")
	fmt.Printf("[%+#s]\n", "line1\nline2")
	fmt.Printf("[%s]\n", "quote\"d")
	fmt.Printf("[%+s]\n", "quote\"d")
	fmt.Printf("[%-s]\n", "quote\"d")
	fmt.Printf("[% s]\n", "quote\"d")
	fmt.Printf("[%#s]\n", "quote\"d")
	fmt.Printf("[%10s]\n", "quote\"d")
	fmt.Printf("[%-10s]\n", "quote\"d")
	fmt.Printf("[%010s]\n", "quote\"d")
	fmt.Printf("[%.3s]\n", "quote\"d")
	fmt.Printf("[%.0s]\n", "quote\"d")
	fmt.Printf("[%10.3s]\n", "quote\"d")
	fmt.Printf("[%-10.3s]\n", "quote\"d")
	fmt.Printf("[%#10s]\n", "quote\"d")
	fmt.Printf("[%#.2s]\n", "quote\"d")
	fmt.Printf("[%+#s]\n", "quote\"d")
	fmt.Printf("[%s]\n", "back\\slash")
	fmt.Printf("[%+s]\n", "back\\slash")
	fmt.Printf("[%-s]\n", "back\\slash")
	fmt.Printf("[% s]\n", "back\\slash")
	fmt.Printf("[%#s]\n", "back\\slash")
	fmt.Printf("[%10s]\n", "back\\slash")
	fmt.Printf("[%-10s]\n", "back\\slash")
	fmt.Printf("[%010s]\n", "back\\slash")
	fmt.Printf("[%.3s]\n", "back\\slash")
	fmt.Printf("[%.0s]\n", "back\\slash")
	fmt.Printf("[%10.3s]\n", "back\\slash")
	fmt.Printf("[%-10.3s]\n", "back\\slash")
	fmt.Printf("[%#10s]\n", "back\\slash")
	fmt.Printf("[%#.2s]\n", "back\\slash")
	fmt.Printf("[%+#s]\n", "back\\slash")
	fmt.Printf("[%s]\n", "`tick`")
	fmt.Printf("[%+s]\n", "`tick`")
	fmt.Printf("[%-s]\n", "`tick`")
	fmt.Printf("[% s]\n", "`tick`")
	fmt.Printf("[%#s]\n", "`tick`")
	fmt.Printf("[%10s]\n", "`tick`")
	fmt.Printf("[%-10s]\n", "`tick`")
	fmt.Printf("[%010s]\n", "`tick`")
	fmt.Printf("[%.3s]\n", "`tick`")
	fmt.Printf("[%.0s]\n", "`tick`")
	fmt.Printf("[%10.3s]\n", "`tick`")
	fmt.Printf("[%-10.3s]\n", "`tick`")
	fmt.Printf("[%#10s]\n", "`tick`")
	fmt.Printf("[%#.2s]\n", "`tick`")
	fmt.Printf("[%+#s]\n", "`tick`")
}

// Output:
// []
// []
// []
// []
// []
// [          ]
// [          ]
// [0000000000]
// []
// []
// [          ]
// [          ]
// [          ]
// []
// []
// [hello]
// [hello]
// [hello]
// [hello]
// [hello]
// [     hello]
// [hello     ]
// [00000hello]
// [hel]
// []
// [       hel]
// [hel       ]
// [     hello]
// [he]
// [hello]
// [hello, world]
// [hello, world]
// [hello, world]
// [hello, world]
// [hello, world]
// [hello, world]
// [hello, world]
// [hello, world]
// [hel]
// []
// [       hel]
// [hel       ]
// [hello, world]
// [he]
// [hello, world]
// [a b	c]
// [a b	c]
// [a b	c]
// [a b	c]
// [a b	c]
// [     a b	c]
// [a b	c     ]
// [00000a b	c]
// [a b]
// []
// [       a b]
// [a b       ]
// [     a b	c]
// [a ]
// [a b	c]
// [line1
// line2]
// [line1
// line2]
// [line1
// line2]
// [line1
// line2]
// [line1
// line2]
// [line1
// line2]
// [line1
// line2]
// [line1
// line2]
// [lin]
// []
// [       lin]
// [lin       ]
// [line1
// line2]
// [li]
// [line1
// line2]
// [quote"d]
// [quote"d]
// [quote"d]
// [quote"d]
// [quote"d]
// [   quote"d]
// [quote"d   ]
// [000quote"d]
// [quo]
// []
// [       quo]
// [quo       ]
// [   quote"d]
// [qu]
// [quote"d]
// [back\slash]
// [back\slash]
// [back\slash]
// [back\slash]
// [back\slash]
// [back\slash]
// [back\slash]
// [back\slash]
// [bac]
// []
// [       bac]
// [bac       ]
// [back\slash]
// [ba]
// [back\slash]
// [`tick`]
// [`tick`]
// [`tick`]
// [`tick`]
// [`tick`]
// [    `tick`]
// [`tick`    ]
// [0000`tick`]
// [`ti]
// []
// [       `ti]
// [`ti       ]
// [    `tick`]
// [`t]
// [`tick`]
