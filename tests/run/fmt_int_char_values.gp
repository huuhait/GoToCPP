// phase: 05

// Go's %c of integers: code points valid and invalid (surrogates, beyond U+10FFFF, negative),
// with flags.
package main

import "fmt"

func main() {
	fmt.Printf("[%c]\n", 65)
	fmt.Printf("[%+c]\n", 65)
	fmt.Printf("[%-c]\n", 65)
	fmt.Printf("[%#c]\n", 65)
	fmt.Printf("[%8c]\n", 65)
	fmt.Printf("[%-8c]\n", 65)
	fmt.Printf("[%08c]\n", 65)
	fmt.Printf("[%.3c]\n", 65)
	fmt.Printf("[%#8c]\n", 65)
	fmt.Printf("[%+#c]\n", 65)
	fmt.Printf("[%-#8c]\n", 65)
	fmt.Printf("[% c]\n", 65)
	fmt.Printf("[%3.8c]\n", 65)
	fmt.Printf("[%c]\n", 0x263A)
	fmt.Printf("[%+c]\n", 0x263A)
	fmt.Printf("[%-c]\n", 0x263A)
	fmt.Printf("[%#c]\n", 0x263A)
	fmt.Printf("[%8c]\n", 0x263A)
	fmt.Printf("[%-8c]\n", 0x263A)
	fmt.Printf("[%08c]\n", 0x263A)
	fmt.Printf("[%.3c]\n", 0x263A)
	fmt.Printf("[%#8c]\n", 0x263A)
	fmt.Printf("[%+#c]\n", 0x263A)
	fmt.Printf("[%-#8c]\n", 0x263A)
	fmt.Printf("[% c]\n", 0x263A)
	fmt.Printf("[%3.8c]\n", 0x263A)
	fmt.Printf("[%c]\n", 0x1F600)
	fmt.Printf("[%+c]\n", 0x1F600)
	fmt.Printf("[%-c]\n", 0x1F600)
	fmt.Printf("[%#c]\n", 0x1F600)
	fmt.Printf("[%8c]\n", 0x1F600)
	fmt.Printf("[%-8c]\n", 0x1F600)
	fmt.Printf("[%08c]\n", 0x1F600)
	fmt.Printf("[%.3c]\n", 0x1F600)
	fmt.Printf("[%#8c]\n", 0x1F600)
	fmt.Printf("[%+#c]\n", 0x1F600)
	fmt.Printf("[%-#8c]\n", 0x1F600)
	fmt.Printf("[% c]\n", 0x1F600)
	fmt.Printf("[%3.8c]\n", 0x1F600)
	fmt.Printf("[%c]\n", -1)
	fmt.Printf("[%+c]\n", -1)
	fmt.Printf("[%-c]\n", -1)
	fmt.Printf("[%#c]\n", -1)
	fmt.Printf("[%8c]\n", -1)
	fmt.Printf("[%-8c]\n", -1)
	fmt.Printf("[%08c]\n", -1)
	fmt.Printf("[%.3c]\n", -1)
	fmt.Printf("[%#8c]\n", -1)
	fmt.Printf("[%+#c]\n", -1)
	fmt.Printf("[%-#8c]\n", -1)
	fmt.Printf("[% c]\n", -1)
	fmt.Printf("[%3.8c]\n", -1)
	fmt.Printf("[%c]\n", 0x10FFFF)
	fmt.Printf("[%+c]\n", 0x10FFFF)
	fmt.Printf("[%-c]\n", 0x10FFFF)
	fmt.Printf("[%#c]\n", 0x10FFFF)
	fmt.Printf("[%8c]\n", 0x10FFFF)
	fmt.Printf("[%-8c]\n", 0x10FFFF)
	fmt.Printf("[%08c]\n", 0x10FFFF)
	fmt.Printf("[%.3c]\n", 0x10FFFF)
	fmt.Printf("[%#8c]\n", 0x10FFFF)
	fmt.Printf("[%+#c]\n", 0x10FFFF)
	fmt.Printf("[%-#8c]\n", 0x10FFFF)
	fmt.Printf("[% c]\n", 0x10FFFF)
	fmt.Printf("[%3.8c]\n", 0x10FFFF)
	fmt.Printf("[%c]\n", 0x110000)
	fmt.Printf("[%+c]\n", 0x110000)
	fmt.Printf("[%-c]\n", 0x110000)
	fmt.Printf("[%#c]\n", 0x110000)
	fmt.Printf("[%8c]\n", 0x110000)
	fmt.Printf("[%-8c]\n", 0x110000)
	fmt.Printf("[%08c]\n", 0x110000)
	fmt.Printf("[%.3c]\n", 0x110000)
	fmt.Printf("[%#8c]\n", 0x110000)
	fmt.Printf("[%+#c]\n", 0x110000)
	fmt.Printf("[%-#8c]\n", 0x110000)
	fmt.Printf("[% c]\n", 0x110000)
	fmt.Printf("[%3.8c]\n", 0x110000)
	fmt.Printf("[%c]\n", 0xD800)
	fmt.Printf("[%+c]\n", 0xD800)
	fmt.Printf("[%-c]\n", 0xD800)
	fmt.Printf("[%#c]\n", 0xD800)
	fmt.Printf("[%8c]\n", 0xD800)
	fmt.Printf("[%-8c]\n", 0xD800)
	fmt.Printf("[%08c]\n", 0xD800)
	fmt.Printf("[%.3c]\n", 0xD800)
	fmt.Printf("[%#8c]\n", 0xD800)
	fmt.Printf("[%+#c]\n", 0xD800)
	fmt.Printf("[%-#8c]\n", 0xD800)
	fmt.Printf("[% c]\n", 0xD800)
	fmt.Printf("[%3.8c]\n", 0xD800)
	fmt.Printf("[%c]\n", 0xFFFD)
	fmt.Printf("[%+c]\n", 0xFFFD)
	fmt.Printf("[%-c]\n", 0xFFFD)
	fmt.Printf("[%#c]\n", 0xFFFD)
	fmt.Printf("[%8c]\n", 0xFFFD)
	fmt.Printf("[%-8c]\n", 0xFFFD)
	fmt.Printf("[%08c]\n", 0xFFFD)
	fmt.Printf("[%.3c]\n", 0xFFFD)
	fmt.Printf("[%#8c]\n", 0xFFFD)
	fmt.Printf("[%+#c]\n", 0xFFFD)
	fmt.Printf("[%-#8c]\n", 0xFFFD)
	fmt.Printf("[% c]\n", 0xFFFD)
	fmt.Printf("[%3.8c]\n", 0xFFFD)
	fmt.Printf("[%c]\n", '\'')
	fmt.Printf("[%+c]\n", '\'')
	fmt.Printf("[%-c]\n", '\'')
	fmt.Printf("[%#c]\n", '\'')
	fmt.Printf("[%8c]\n", '\'')
	fmt.Printf("[%-8c]\n", '\'')
	fmt.Printf("[%08c]\n", '\'')
	fmt.Printf("[%.3c]\n", '\'')
	fmt.Printf("[%#8c]\n", '\'')
	fmt.Printf("[%+#c]\n", '\'')
	fmt.Printf("[%-#8c]\n", '\'')
	fmt.Printf("[% c]\n", '\'')
	fmt.Printf("[%3.8c]\n", '\'')
	fmt.Printf("[%c]\n", '"')
	fmt.Printf("[%+c]\n", '"')
	fmt.Printf("[%-c]\n", '"')
	fmt.Printf("[%#c]\n", '"')
	fmt.Printf("[%8c]\n", '"')
	fmt.Printf("[%-8c]\n", '"')
	fmt.Printf("[%08c]\n", '"')
	fmt.Printf("[%.3c]\n", '"')
	fmt.Printf("[%#8c]\n", '"')
	fmt.Printf("[%+#c]\n", '"')
	fmt.Printf("[%-#8c]\n", '"')
	fmt.Printf("[% c]\n", '"')
	fmt.Printf("[%3.8c]\n", '"')
	fmt.Printf("[%c]\n", '\\')
	fmt.Printf("[%+c]\n", '\\')
	fmt.Printf("[%-c]\n", '\\')
	fmt.Printf("[%#c]\n", '\\')
	fmt.Printf("[%8c]\n", '\\')
	fmt.Printf("[%-8c]\n", '\\')
	fmt.Printf("[%08c]\n", '\\')
	fmt.Printf("[%.3c]\n", '\\')
	fmt.Printf("[%#8c]\n", '\\')
	fmt.Printf("[%+#c]\n", '\\')
	fmt.Printf("[%-#8c]\n", '\\')
	fmt.Printf("[% c]\n", '\\')
	fmt.Printf("[%3.8c]\n", '\\')
	fmt.Printf("[%c]\n", 0x200B)
	fmt.Printf("[%+c]\n", 0x200B)
	fmt.Printf("[%-c]\n", 0x200B)
	fmt.Printf("[%#c]\n", 0x200B)
	fmt.Printf("[%8c]\n", 0x200B)
	fmt.Printf("[%-8c]\n", 0x200B)
	fmt.Printf("[%08c]\n", 0x200B)
	fmt.Printf("[%.3c]\n", 0x200B)
	fmt.Printf("[%#8c]\n", 0x200B)
	fmt.Printf("[%+#c]\n", 0x200B)
	fmt.Printf("[%-#8c]\n", 0x200B)
	fmt.Printf("[% c]\n", 0x200B)
	fmt.Printf("[%3.8c]\n", 0x200B)
}

// Output:
// [A]
// [A]
// [A]
// [A]
// [       A]
// [A       ]
// [0000000A]
// [A]
// [       A]
// [A]
// [A       ]
// [A]
// [  A]
// [☺]
// [☺]
// [☺]
// [☺]
// [       ☺]
// [☺       ]
// [0000000☺]
// [☺]
// [       ☺]
// [☺]
// [☺       ]
// [☺]
// [  ☺]
// [😀]
// [😀]
// [😀]
// [😀]
// [       😀]
// [😀       ]
// [0000000😀]
// [😀]
// [       😀]
// [😀]
// [😀       ]
// [😀]
// [  😀]
// [�]
// [�]
// [�]
// [�]
// [       �]
// [�       ]
// [0000000�]
// [�]
// [       �]
// [�]
// [�       ]
// [�]
// [  �]
// [􏿿]
// [􏿿]
// [􏿿]
// [􏿿]
// [       􏿿]
// [􏿿       ]
// [0000000􏿿]
// [􏿿]
// [       􏿿]
// [􏿿]
// [􏿿       ]
// [􏿿]
// [  􏿿]
// [�]
// [�]
// [�]
// [�]
// [       �]
// [�       ]
// [0000000�]
// [�]
// [       �]
// [�]
// [�       ]
// [�]
// [  �]
// [�]
// [�]
// [�]
// [�]
// [       �]
// [�       ]
// [0000000�]
// [�]
// [       �]
// [�]
// [�       ]
// [�]
// [  �]
// [�]
// [�]
// [�]
// [�]
// [       �]
// [�       ]
// [0000000�]
// [�]
// [       �]
// [�]
// [�       ]
// [�]
// [  �]
// [']
// [']
// [']
// [']
// [       ']
// ['       ]
// [0000000']
// [']
// [       ']
// [']
// ['       ]
// [']
// [  ']
// ["]
// ["]
// ["]
// ["]
// [       "]
// ["       ]
// [0000000"]
// ["]
// [       "]
// ["]
// ["       ]
// ["]
// [  "]
// [\]
// [\]
// [\]
// [\]
// [       \]
// [\       ]
// [0000000\]
// [\]
// [       \]
// [\]
// [\       ]
// [\]
// [  \]
// [​]
// [​]
// [​]
// [​]
// [       ​]
// [​       ]
// [0000000​]
// [​]
// [       ​]
// [​]
// [​       ]
// [​]
// [  ​]
