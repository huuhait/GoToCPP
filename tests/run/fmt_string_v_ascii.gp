// phase: 05

// Go's %v of strings (ascii) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%v]\n", "")
	fmt.Printf("[%+v]\n", "")
	fmt.Printf("[%-v]\n", "")
	fmt.Printf("[% v]\n", "")
	fmt.Printf("[%#v]\n", "")
	fmt.Printf("[%10v]\n", "")
	fmt.Printf("[%-10v]\n", "")
	fmt.Printf("[%010v]\n", "")
	fmt.Printf("[%.3v]\n", "")
	fmt.Printf("[%.0v]\n", "")
	fmt.Printf("[%10.3v]\n", "")
	fmt.Printf("[%-10.3v]\n", "")
	fmt.Printf("[%#10v]\n", "")
	fmt.Printf("[%#.2v]\n", "")
	fmt.Printf("[%+#v]\n", "")
	fmt.Printf("[%v]\n", "hello")
	fmt.Printf("[%+v]\n", "hello")
	fmt.Printf("[%-v]\n", "hello")
	fmt.Printf("[% v]\n", "hello")
	fmt.Printf("[%#v]\n", "hello")
	fmt.Printf("[%10v]\n", "hello")
	fmt.Printf("[%-10v]\n", "hello")
	fmt.Printf("[%010v]\n", "hello")
	fmt.Printf("[%.3v]\n", "hello")
	fmt.Printf("[%.0v]\n", "hello")
	fmt.Printf("[%10.3v]\n", "hello")
	fmt.Printf("[%-10.3v]\n", "hello")
	fmt.Printf("[%#10v]\n", "hello")
	fmt.Printf("[%#.2v]\n", "hello")
	fmt.Printf("[%+#v]\n", "hello")
	fmt.Printf("[%v]\n", "hello, world")
	fmt.Printf("[%+v]\n", "hello, world")
	fmt.Printf("[%-v]\n", "hello, world")
	fmt.Printf("[% v]\n", "hello, world")
	fmt.Printf("[%#v]\n", "hello, world")
	fmt.Printf("[%10v]\n", "hello, world")
	fmt.Printf("[%-10v]\n", "hello, world")
	fmt.Printf("[%010v]\n", "hello, world")
	fmt.Printf("[%.3v]\n", "hello, world")
	fmt.Printf("[%.0v]\n", "hello, world")
	fmt.Printf("[%10.3v]\n", "hello, world")
	fmt.Printf("[%-10.3v]\n", "hello, world")
	fmt.Printf("[%#10v]\n", "hello, world")
	fmt.Printf("[%#.2v]\n", "hello, world")
	fmt.Printf("[%+#v]\n", "hello, world")
	fmt.Printf("[%v]\n", "a b\tc")
	fmt.Printf("[%+v]\n", "a b\tc")
	fmt.Printf("[%-v]\n", "a b\tc")
	fmt.Printf("[% v]\n", "a b\tc")
	fmt.Printf("[%#v]\n", "a b\tc")
	fmt.Printf("[%10v]\n", "a b\tc")
	fmt.Printf("[%-10v]\n", "a b\tc")
	fmt.Printf("[%010v]\n", "a b\tc")
	fmt.Printf("[%.3v]\n", "a b\tc")
	fmt.Printf("[%.0v]\n", "a b\tc")
	fmt.Printf("[%10.3v]\n", "a b\tc")
	fmt.Printf("[%-10.3v]\n", "a b\tc")
	fmt.Printf("[%#10v]\n", "a b\tc")
	fmt.Printf("[%#.2v]\n", "a b\tc")
	fmt.Printf("[%+#v]\n", "a b\tc")
	fmt.Printf("[%v]\n", "line1\nline2")
	fmt.Printf("[%+v]\n", "line1\nline2")
	fmt.Printf("[%-v]\n", "line1\nline2")
	fmt.Printf("[% v]\n", "line1\nline2")
	fmt.Printf("[%#v]\n", "line1\nline2")
	fmt.Printf("[%10v]\n", "line1\nline2")
	fmt.Printf("[%-10v]\n", "line1\nline2")
	fmt.Printf("[%010v]\n", "line1\nline2")
	fmt.Printf("[%.3v]\n", "line1\nline2")
	fmt.Printf("[%.0v]\n", "line1\nline2")
	fmt.Printf("[%10.3v]\n", "line1\nline2")
	fmt.Printf("[%-10.3v]\n", "line1\nline2")
	fmt.Printf("[%#10v]\n", "line1\nline2")
	fmt.Printf("[%#.2v]\n", "line1\nline2")
	fmt.Printf("[%+#v]\n", "line1\nline2")
	fmt.Printf("[%v]\n", "quote\"d")
	fmt.Printf("[%+v]\n", "quote\"d")
	fmt.Printf("[%-v]\n", "quote\"d")
	fmt.Printf("[% v]\n", "quote\"d")
	fmt.Printf("[%#v]\n", "quote\"d")
	fmt.Printf("[%10v]\n", "quote\"d")
	fmt.Printf("[%-10v]\n", "quote\"d")
	fmt.Printf("[%010v]\n", "quote\"d")
	fmt.Printf("[%.3v]\n", "quote\"d")
	fmt.Printf("[%.0v]\n", "quote\"d")
	fmt.Printf("[%10.3v]\n", "quote\"d")
	fmt.Printf("[%-10.3v]\n", "quote\"d")
	fmt.Printf("[%#10v]\n", "quote\"d")
	fmt.Printf("[%#.2v]\n", "quote\"d")
	fmt.Printf("[%+#v]\n", "quote\"d")
	fmt.Printf("[%v]\n", "back\\slash")
	fmt.Printf("[%+v]\n", "back\\slash")
	fmt.Printf("[%-v]\n", "back\\slash")
	fmt.Printf("[% v]\n", "back\\slash")
	fmt.Printf("[%#v]\n", "back\\slash")
	fmt.Printf("[%10v]\n", "back\\slash")
	fmt.Printf("[%-10v]\n", "back\\slash")
	fmt.Printf("[%010v]\n", "back\\slash")
	fmt.Printf("[%.3v]\n", "back\\slash")
	fmt.Printf("[%.0v]\n", "back\\slash")
	fmt.Printf("[%10.3v]\n", "back\\slash")
	fmt.Printf("[%-10.3v]\n", "back\\slash")
	fmt.Printf("[%#10v]\n", "back\\slash")
	fmt.Printf("[%#.2v]\n", "back\\slash")
	fmt.Printf("[%+#v]\n", "back\\slash")
	fmt.Printf("[%v]\n", "`tick`")
	fmt.Printf("[%+v]\n", "`tick`")
	fmt.Printf("[%-v]\n", "`tick`")
	fmt.Printf("[% v]\n", "`tick`")
	fmt.Printf("[%#v]\n", "`tick`")
	fmt.Printf("[%10v]\n", "`tick`")
	fmt.Printf("[%-10v]\n", "`tick`")
	fmt.Printf("[%010v]\n", "`tick`")
	fmt.Printf("[%.3v]\n", "`tick`")
	fmt.Printf("[%.0v]\n", "`tick`")
	fmt.Printf("[%10.3v]\n", "`tick`")
	fmt.Printf("[%-10.3v]\n", "`tick`")
	fmt.Printf("[%#10v]\n", "`tick`")
	fmt.Printf("[%#.2v]\n", "`tick`")
	fmt.Printf("[%+#v]\n", "`tick`")
}

// Output:
// []
// []
// []
// []
// [""]
// [          ]
// [          ]
// [0000000000]
// []
// []
// [          ]
// [          ]
// [        ""]
// [""]
// [""]
// [hello]
// [hello]
// [hello]
// [hello]
// ["hello"]
// [     hello]
// [hello     ]
// [00000hello]
// [hel]
// []
// [       hel]
// [hel       ]
// [   "hello"]
// ["he"]
// ["hello"]
// [hello, world]
// [hello, world]
// [hello, world]
// [hello, world]
// ["hello, world"]
// [hello, world]
// [hello, world]
// [hello, world]
// [hel]
// []
// [       hel]
// [hel       ]
// ["hello, world"]
// ["he"]
// ["hello, world"]
// [a b	c]
// [a b	c]
// [a b	c]
// [a b	c]
// ["a b\tc"]
// [     a b	c]
// [a b	c     ]
// [00000a b	c]
// [a b]
// []
// [       a b]
// [a b       ]
// [  "a b\tc"]
// ["a "]
// ["a b\tc"]
// [line1
// line2]
// [line1
// line2]
// [line1
// line2]
// [line1
// line2]
// ["line1\nline2"]
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
// ["line1\nline2"]
// ["li"]
// ["line1\nline2"]
// [quote"d]
// [quote"d]
// [quote"d]
// [quote"d]
// ["quote\"d"]
// [   quote"d]
// [quote"d   ]
// [000quote"d]
// [quo]
// []
// [       quo]
// [quo       ]
// ["quote\"d"]
// ["qu"]
// ["quote\"d"]
// [back\slash]
// [back\slash]
// [back\slash]
// [back\slash]
// ["back\\slash"]
// [back\slash]
// [back\slash]
// [back\slash]
// [bac]
// []
// [       bac]
// [bac       ]
// ["back\\slash"]
// ["ba"]
// ["back\\slash"]
// [`tick`]
// [`tick`]
// [`tick`]
// [`tick`]
// ["`tick`"]
// [    `tick`]
// [`tick`    ]
// [0000`tick`]
// [`ti]
// []
// [       `ti]
// [`ti       ]
// [  "`tick`"]
// ["`t"]
// ["`tick`"]
