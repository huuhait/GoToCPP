// phase: 05

// Go's %s of strings (unicode) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%s]\n", "héllo")
	fmt.Printf("[%+s]\n", "héllo")
	fmt.Printf("[%-s]\n", "héllo")
	fmt.Printf("[% s]\n", "héllo")
	fmt.Printf("[%#s]\n", "héllo")
	fmt.Printf("[%10s]\n", "héllo")
	fmt.Printf("[%-10s]\n", "héllo")
	fmt.Printf("[%010s]\n", "héllo")
	fmt.Printf("[%.3s]\n", "héllo")
	fmt.Printf("[%.0s]\n", "héllo")
	fmt.Printf("[%10.3s]\n", "héllo")
	fmt.Printf("[%-10.3s]\n", "héllo")
	fmt.Printf("[%#10s]\n", "héllo")
	fmt.Printf("[%#.2s]\n", "héllo")
	fmt.Printf("[%+#s]\n", "héllo")
	fmt.Printf("[%s]\n", "日本語")
	fmt.Printf("[%+s]\n", "日本語")
	fmt.Printf("[%-s]\n", "日本語")
	fmt.Printf("[% s]\n", "日本語")
	fmt.Printf("[%#s]\n", "日本語")
	fmt.Printf("[%10s]\n", "日本語")
	fmt.Printf("[%-10s]\n", "日本語")
	fmt.Printf("[%010s]\n", "日本語")
	fmt.Printf("[%.3s]\n", "日本語")
	fmt.Printf("[%.0s]\n", "日本語")
	fmt.Printf("[%10.3s]\n", "日本語")
	fmt.Printf("[%-10.3s]\n", "日本語")
	fmt.Printf("[%#10s]\n", "日本語")
	fmt.Printf("[%#.2s]\n", "日本語")
	fmt.Printf("[%+#s]\n", "日本語")
	fmt.Printf("[%s]\n", "😀 smile")
	fmt.Printf("[%+s]\n", "😀 smile")
	fmt.Printf("[%-s]\n", "😀 smile")
	fmt.Printf("[% s]\n", "😀 smile")
	fmt.Printf("[%#s]\n", "😀 smile")
	fmt.Printf("[%10s]\n", "😀 smile")
	fmt.Printf("[%-10s]\n", "😀 smile")
	fmt.Printf("[%010s]\n", "😀 smile")
	fmt.Printf("[%.3s]\n", "😀 smile")
	fmt.Printf("[%.0s]\n", "😀 smile")
	fmt.Printf("[%10.3s]\n", "😀 smile")
	fmt.Printf("[%-10.3s]\n", "😀 smile")
	fmt.Printf("[%#10s]\n", "😀 smile")
	fmt.Printf("[%#.2s]\n", "😀 smile")
	fmt.Printf("[%+#s]\n", "😀 smile")
	fmt.Printf("[%s]\n", "\u00a0nbsp")
	fmt.Printf("[%+s]\n", "\u00a0nbsp")
	fmt.Printf("[%-s]\n", "\u00a0nbsp")
	fmt.Printf("[% s]\n", "\u00a0nbsp")
	fmt.Printf("[%#s]\n", "\u00a0nbsp")
	fmt.Printf("[%10s]\n", "\u00a0nbsp")
	fmt.Printf("[%-10s]\n", "\u00a0nbsp")
	fmt.Printf("[%010s]\n", "\u00a0nbsp")
	fmt.Printf("[%.3s]\n", "\u00a0nbsp")
	fmt.Printf("[%.0s]\n", "\u00a0nbsp")
	fmt.Printf("[%10.3s]\n", "\u00a0nbsp")
	fmt.Printf("[%-10.3s]\n", "\u00a0nbsp")
	fmt.Printf("[%#10s]\n", "\u00a0nbsp")
	fmt.Printf("[%#.2s]\n", "\u00a0nbsp")
	fmt.Printf("[%+#s]\n", "\u00a0nbsp")
	fmt.Printf("[%s]\n", "zero\u200bwidth")
	fmt.Printf("[%+s]\n", "zero\u200bwidth")
	fmt.Printf("[%-s]\n", "zero\u200bwidth")
	fmt.Printf("[% s]\n", "zero\u200bwidth")
	fmt.Printf("[%#s]\n", "zero\u200bwidth")
	fmt.Printf("[%10s]\n", "zero\u200bwidth")
	fmt.Printf("[%-10s]\n", "zero\u200bwidth")
	fmt.Printf("[%010s]\n", "zero\u200bwidth")
	fmt.Printf("[%.3s]\n", "zero\u200bwidth")
	fmt.Printf("[%.0s]\n", "zero\u200bwidth")
	fmt.Printf("[%10.3s]\n", "zero\u200bwidth")
	fmt.Printf("[%-10.3s]\n", "zero\u200bwidth")
	fmt.Printf("[%#10s]\n", "zero\u200bwidth")
	fmt.Printf("[%#.2s]\n", "zero\u200bwidth")
	fmt.Printf("[%+#s]\n", "zero\u200bwidth")
}

// Output:
// [héllo]
// [héllo]
// [héllo]
// [héllo]
// [héllo]
// [     héllo]
// [héllo     ]
// [00000héllo]
// [hél]
// []
// [       hél]
// [hél       ]
// [     héllo]
// [hé]
// [héllo]
// [日本語]
// [日本語]
// [日本語]
// [日本語]
// [日本語]
// [       日本語]
// [日本語       ]
// [0000000日本語]
// [日本語]
// []
// [       日本語]
// [日本語       ]
// [       日本語]
// [日本]
// [日本語]
// [😀 smile]
// [😀 smile]
// [😀 smile]
// [😀 smile]
// [😀 smile]
// [   😀 smile]
// [😀 smile   ]
// [000😀 smile]
// [😀 s]
// []
// [       😀 s]
// [😀 s       ]
// [   😀 smile]
// [😀 ]
// [😀 smile]
// [ nbsp]
// [ nbsp]
// [ nbsp]
// [ nbsp]
// [ nbsp]
// [      nbsp]
// [ nbsp     ]
// [00000 nbsp]
// [ nb]
// []
// [        nb]
// [ nb       ]
// [      nbsp]
// [ n]
// [ nbsp]
// [zero​width]
// [zero​width]
// [zero​width]
// [zero​width]
// [zero​width]
// [zero​width]
// [zero​width]
// [zero​width]
// [zer]
// []
// [       zer]
// [zer       ]
// [zero​width]
// [ze]
// [zero​width]
