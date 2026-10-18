// phase: 05

// Go's %v of strings (unicode) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%v]\n", "héllo")
	fmt.Printf("[%+v]\n", "héllo")
	fmt.Printf("[%-v]\n", "héllo")
	fmt.Printf("[% v]\n", "héllo")
	fmt.Printf("[%#v]\n", "héllo")
	fmt.Printf("[%10v]\n", "héllo")
	fmt.Printf("[%-10v]\n", "héllo")
	fmt.Printf("[%010v]\n", "héllo")
	fmt.Printf("[%.3v]\n", "héllo")
	fmt.Printf("[%.0v]\n", "héllo")
	fmt.Printf("[%10.3v]\n", "héllo")
	fmt.Printf("[%-10.3v]\n", "héllo")
	fmt.Printf("[%#10v]\n", "héllo")
	fmt.Printf("[%#.2v]\n", "héllo")
	fmt.Printf("[%+#v]\n", "héllo")
	fmt.Printf("[%v]\n", "日本語")
	fmt.Printf("[%+v]\n", "日本語")
	fmt.Printf("[%-v]\n", "日本語")
	fmt.Printf("[% v]\n", "日本語")
	fmt.Printf("[%#v]\n", "日本語")
	fmt.Printf("[%10v]\n", "日本語")
	fmt.Printf("[%-10v]\n", "日本語")
	fmt.Printf("[%010v]\n", "日本語")
	fmt.Printf("[%.3v]\n", "日本語")
	fmt.Printf("[%.0v]\n", "日本語")
	fmt.Printf("[%10.3v]\n", "日本語")
	fmt.Printf("[%-10.3v]\n", "日本語")
	fmt.Printf("[%#10v]\n", "日本語")
	fmt.Printf("[%#.2v]\n", "日本語")
	fmt.Printf("[%+#v]\n", "日本語")
	fmt.Printf("[%v]\n", "😀 smile")
	fmt.Printf("[%+v]\n", "😀 smile")
	fmt.Printf("[%-v]\n", "😀 smile")
	fmt.Printf("[% v]\n", "😀 smile")
	fmt.Printf("[%#v]\n", "😀 smile")
	fmt.Printf("[%10v]\n", "😀 smile")
	fmt.Printf("[%-10v]\n", "😀 smile")
	fmt.Printf("[%010v]\n", "😀 smile")
	fmt.Printf("[%.3v]\n", "😀 smile")
	fmt.Printf("[%.0v]\n", "😀 smile")
	fmt.Printf("[%10.3v]\n", "😀 smile")
	fmt.Printf("[%-10.3v]\n", "😀 smile")
	fmt.Printf("[%#10v]\n", "😀 smile")
	fmt.Printf("[%#.2v]\n", "😀 smile")
	fmt.Printf("[%+#v]\n", "😀 smile")
	fmt.Printf("[%v]\n", "\u00a0nbsp")
	fmt.Printf("[%+v]\n", "\u00a0nbsp")
	fmt.Printf("[%-v]\n", "\u00a0nbsp")
	fmt.Printf("[% v]\n", "\u00a0nbsp")
	fmt.Printf("[%#v]\n", "\u00a0nbsp")
	fmt.Printf("[%10v]\n", "\u00a0nbsp")
	fmt.Printf("[%-10v]\n", "\u00a0nbsp")
	fmt.Printf("[%010v]\n", "\u00a0nbsp")
	fmt.Printf("[%.3v]\n", "\u00a0nbsp")
	fmt.Printf("[%.0v]\n", "\u00a0nbsp")
	fmt.Printf("[%10.3v]\n", "\u00a0nbsp")
	fmt.Printf("[%-10.3v]\n", "\u00a0nbsp")
	fmt.Printf("[%#10v]\n", "\u00a0nbsp")
	fmt.Printf("[%#.2v]\n", "\u00a0nbsp")
	fmt.Printf("[%+#v]\n", "\u00a0nbsp")
	fmt.Printf("[%v]\n", "zero\u200bwidth")
	fmt.Printf("[%+v]\n", "zero\u200bwidth")
	fmt.Printf("[%-v]\n", "zero\u200bwidth")
	fmt.Printf("[% v]\n", "zero\u200bwidth")
	fmt.Printf("[%#v]\n", "zero\u200bwidth")
	fmt.Printf("[%10v]\n", "zero\u200bwidth")
	fmt.Printf("[%-10v]\n", "zero\u200bwidth")
	fmt.Printf("[%010v]\n", "zero\u200bwidth")
	fmt.Printf("[%.3v]\n", "zero\u200bwidth")
	fmt.Printf("[%.0v]\n", "zero\u200bwidth")
	fmt.Printf("[%10.3v]\n", "zero\u200bwidth")
	fmt.Printf("[%-10.3v]\n", "zero\u200bwidth")
	fmt.Printf("[%#10v]\n", "zero\u200bwidth")
	fmt.Printf("[%#.2v]\n", "zero\u200bwidth")
	fmt.Printf("[%+#v]\n", "zero\u200bwidth")
}

// Output:
// [héllo]
// [héllo]
// [héllo]
// [héllo]
// ["héllo"]
// [     héllo]
// [héllo     ]
// [00000héllo]
// [hél]
// []
// [       hél]
// [hél       ]
// [   "héllo"]
// ["hé"]
// ["héllo"]
// [日本語]
// [日本語]
// [日本語]
// [日本語]
// ["日本語"]
// [       日本語]
// [日本語       ]
// [0000000日本語]
// [日本語]
// []
// [       日本語]
// [日本語       ]
// [     "日本語"]
// ["日本"]
// ["日本語"]
// [😀 smile]
// [😀 smile]
// [😀 smile]
// [😀 smile]
// ["😀 smile"]
// [   😀 smile]
// [😀 smile   ]
// [000😀 smile]
// [😀 s]
// []
// [       😀 s]
// [😀 s       ]
// [ "😀 smile"]
// ["😀 "]
// ["😀 smile"]
// [ nbsp]
// [ nbsp]
// [ nbsp]
// [ nbsp]
// ["\u00a0nbsp"]
// [      nbsp]
// [ nbsp     ]
// [00000 nbsp]
// [ nb]
// []
// [        nb]
// [ nb       ]
// ["\u00a0nbsp"]
// ["\u00a0n"]
// ["\u00a0nbsp"]
// [zero​width]
// [zero​width]
// [zero​width]
// [zero​width]
// ["zero\u200bwidth"]
// [zero​width]
// [zero​width]
// [zero​width]
// [zer]
// []
// [       zer]
// [zer       ]
// ["zero\u200bwidth"]
// ["ze"]
// ["zero\u200bwidth"]
