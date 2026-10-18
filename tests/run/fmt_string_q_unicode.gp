// phase: 05

// Go's %q of strings (unicode) with flags, width and precision (runes, not bytes).
package main

import "fmt"

func main() {
	fmt.Printf("[%q]\n", "héllo")
	fmt.Printf("[%+q]\n", "héllo")
	fmt.Printf("[%-q]\n", "héllo")
	fmt.Printf("[% q]\n", "héllo")
	fmt.Printf("[%#q]\n", "héllo")
	fmt.Printf("[%10q]\n", "héllo")
	fmt.Printf("[%-10q]\n", "héllo")
	fmt.Printf("[%010q]\n", "héllo")
	fmt.Printf("[%.3q]\n", "héllo")
	fmt.Printf("[%.0q]\n", "héllo")
	fmt.Printf("[%10.3q]\n", "héllo")
	fmt.Printf("[%-10.3q]\n", "héllo")
	fmt.Printf("[%#10q]\n", "héllo")
	fmt.Printf("[%#.2q]\n", "héllo")
	fmt.Printf("[%+#q]\n", "héllo")
	fmt.Printf("[%q]\n", "日本語")
	fmt.Printf("[%+q]\n", "日本語")
	fmt.Printf("[%-q]\n", "日本語")
	fmt.Printf("[% q]\n", "日本語")
	fmt.Printf("[%#q]\n", "日本語")
	fmt.Printf("[%10q]\n", "日本語")
	fmt.Printf("[%-10q]\n", "日本語")
	fmt.Printf("[%010q]\n", "日本語")
	fmt.Printf("[%.3q]\n", "日本語")
	fmt.Printf("[%.0q]\n", "日本語")
	fmt.Printf("[%10.3q]\n", "日本語")
	fmt.Printf("[%-10.3q]\n", "日本語")
	fmt.Printf("[%#10q]\n", "日本語")
	fmt.Printf("[%#.2q]\n", "日本語")
	fmt.Printf("[%+#q]\n", "日本語")
	fmt.Printf("[%q]\n", "😀 smile")
	fmt.Printf("[%+q]\n", "😀 smile")
	fmt.Printf("[%-q]\n", "😀 smile")
	fmt.Printf("[% q]\n", "😀 smile")
	fmt.Printf("[%#q]\n", "😀 smile")
	fmt.Printf("[%10q]\n", "😀 smile")
	fmt.Printf("[%-10q]\n", "😀 smile")
	fmt.Printf("[%010q]\n", "😀 smile")
	fmt.Printf("[%.3q]\n", "😀 smile")
	fmt.Printf("[%.0q]\n", "😀 smile")
	fmt.Printf("[%10.3q]\n", "😀 smile")
	fmt.Printf("[%-10.3q]\n", "😀 smile")
	fmt.Printf("[%#10q]\n", "😀 smile")
	fmt.Printf("[%#.2q]\n", "😀 smile")
	fmt.Printf("[%+#q]\n", "😀 smile")
	fmt.Printf("[%q]\n", "\u00a0nbsp")
	fmt.Printf("[%+q]\n", "\u00a0nbsp")
	fmt.Printf("[%-q]\n", "\u00a0nbsp")
	fmt.Printf("[% q]\n", "\u00a0nbsp")
	fmt.Printf("[%#q]\n", "\u00a0nbsp")
	fmt.Printf("[%10q]\n", "\u00a0nbsp")
	fmt.Printf("[%-10q]\n", "\u00a0nbsp")
	fmt.Printf("[%010q]\n", "\u00a0nbsp")
	fmt.Printf("[%.3q]\n", "\u00a0nbsp")
	fmt.Printf("[%.0q]\n", "\u00a0nbsp")
	fmt.Printf("[%10.3q]\n", "\u00a0nbsp")
	fmt.Printf("[%-10.3q]\n", "\u00a0nbsp")
	fmt.Printf("[%#10q]\n", "\u00a0nbsp")
	fmt.Printf("[%#.2q]\n", "\u00a0nbsp")
	fmt.Printf("[%+#q]\n", "\u00a0nbsp")
	fmt.Printf("[%q]\n", "zero\u200bwidth")
	fmt.Printf("[%+q]\n", "zero\u200bwidth")
	fmt.Printf("[%-q]\n", "zero\u200bwidth")
	fmt.Printf("[% q]\n", "zero\u200bwidth")
	fmt.Printf("[%#q]\n", "zero\u200bwidth")
	fmt.Printf("[%10q]\n", "zero\u200bwidth")
	fmt.Printf("[%-10q]\n", "zero\u200bwidth")
	fmt.Printf("[%010q]\n", "zero\u200bwidth")
	fmt.Printf("[%.3q]\n", "zero\u200bwidth")
	fmt.Printf("[%.0q]\n", "zero\u200bwidth")
	fmt.Printf("[%10.3q]\n", "zero\u200bwidth")
	fmt.Printf("[%-10.3q]\n", "zero\u200bwidth")
	fmt.Printf("[%#10q]\n", "zero\u200bwidth")
	fmt.Printf("[%#.2q]\n", "zero\u200bwidth")
	fmt.Printf("[%+#q]\n", "zero\u200bwidth")
}

// Output:
// ["héllo"]
// ["h\u00e9llo"]
// ["héllo"]
// ["héllo"]
// [`héllo`]
// [   "héllo"]
// ["héllo"   ]
// [000"héllo"]
// ["hél"]
// [""]
// [     "hél"]
// ["hél"     ]
// [   `héllo`]
// [`hé`]
// [`héllo`]
// ["日本語"]
// ["\u65e5\u672c\u8a9e"]
// ["日本語"]
// ["日本語"]
// [`日本語`]
// [     "日本語"]
// ["日本語"     ]
// [00000"日本語"]
// ["日本語"]
// [""]
// [     "日本語"]
// ["日本語"     ]
// [     `日本語`]
// [`日本`]
// [`日本語`]
// ["😀 smile"]
// ["\U0001f600 smile"]
// ["😀 smile"]
// ["😀 smile"]
// [`😀 smile`]
// [ "😀 smile"]
// ["😀 smile" ]
// [0"😀 smile"]
// ["😀 s"]
// [""]
// [     "😀 s"]
// ["😀 s"     ]
// [ `😀 smile`]
// [`😀 `]
// [`😀 smile`]
// ["\u00a0nbsp"]
// ["\u00a0nbsp"]
// ["\u00a0nbsp"]
// ["\u00a0nbsp"]
// [` nbsp`]
// ["\u00a0nbsp"]
// ["\u00a0nbsp"]
// ["\u00a0nbsp"]
// ["\u00a0nb"]
// [""]
// ["\u00a0nb"]
// ["\u00a0nb"]
// [   ` nbsp`]
// [` n`]
// [` nbsp`]
// ["zero\u200bwidth"]
// ["zero\u200bwidth"]
// ["zero\u200bwidth"]
// ["zero\u200bwidth"]
// [`zero​width`]
// ["zero\u200bwidth"]
// ["zero\u200bwidth"]
// ["zero\u200bwidth"]
// ["zer"]
// [""]
// [     "zer"]
// ["zer"     ]
// [`zero​width`]
// [`ze`]
// [`zero​width`]
