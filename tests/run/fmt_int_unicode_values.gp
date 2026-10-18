// phase: 05

// Go's %U of integers: code points valid and invalid (surrogates, beyond U+10FFFF, negative),
// with flags.
package main

import "fmt"

func main() {
	fmt.Printf("[%U]\n", 65)
	fmt.Printf("[%+U]\n", 65)
	fmt.Printf("[%-U]\n", 65)
	fmt.Printf("[%#U]\n", 65)
	fmt.Printf("[%8U]\n", 65)
	fmt.Printf("[%-8U]\n", 65)
	fmt.Printf("[%08U]\n", 65)
	fmt.Printf("[%.3U]\n", 65)
	fmt.Printf("[%#8U]\n", 65)
	fmt.Printf("[%+#U]\n", 65)
	fmt.Printf("[%-#8U]\n", 65)
	fmt.Printf("[% U]\n", 65)
	fmt.Printf("[%3.8U]\n", 65)
	fmt.Printf("[%U]\n", 0x263A)
	fmt.Printf("[%+U]\n", 0x263A)
	fmt.Printf("[%-U]\n", 0x263A)
	fmt.Printf("[%#U]\n", 0x263A)
	fmt.Printf("[%8U]\n", 0x263A)
	fmt.Printf("[%-8U]\n", 0x263A)
	fmt.Printf("[%08U]\n", 0x263A)
	fmt.Printf("[%.3U]\n", 0x263A)
	fmt.Printf("[%#8U]\n", 0x263A)
	fmt.Printf("[%+#U]\n", 0x263A)
	fmt.Printf("[%-#8U]\n", 0x263A)
	fmt.Printf("[% U]\n", 0x263A)
	fmt.Printf("[%3.8U]\n", 0x263A)
	fmt.Printf("[%U]\n", 0x1F600)
	fmt.Printf("[%+U]\n", 0x1F600)
	fmt.Printf("[%-U]\n", 0x1F600)
	fmt.Printf("[%#U]\n", 0x1F600)
	fmt.Printf("[%8U]\n", 0x1F600)
	fmt.Printf("[%-8U]\n", 0x1F600)
	fmt.Printf("[%08U]\n", 0x1F600)
	fmt.Printf("[%.3U]\n", 0x1F600)
	fmt.Printf("[%#8U]\n", 0x1F600)
	fmt.Printf("[%+#U]\n", 0x1F600)
	fmt.Printf("[%-#8U]\n", 0x1F600)
	fmt.Printf("[% U]\n", 0x1F600)
	fmt.Printf("[%3.8U]\n", 0x1F600)
	fmt.Printf("[%U]\n", 0)
	fmt.Printf("[%+U]\n", 0)
	fmt.Printf("[%-U]\n", 0)
	fmt.Printf("[%#U]\n", 0)
	fmt.Printf("[%8U]\n", 0)
	fmt.Printf("[%-8U]\n", 0)
	fmt.Printf("[%08U]\n", 0)
	fmt.Printf("[%.3U]\n", 0)
	fmt.Printf("[%#8U]\n", 0)
	fmt.Printf("[%+#U]\n", 0)
	fmt.Printf("[%-#8U]\n", 0)
	fmt.Printf("[% U]\n", 0)
	fmt.Printf("[%3.8U]\n", 0)
	fmt.Printf("[%U]\n", 10)
	fmt.Printf("[%+U]\n", 10)
	fmt.Printf("[%-U]\n", 10)
	fmt.Printf("[%#U]\n", 10)
	fmt.Printf("[%8U]\n", 10)
	fmt.Printf("[%-8U]\n", 10)
	fmt.Printf("[%08U]\n", 10)
	fmt.Printf("[%.3U]\n", 10)
	fmt.Printf("[%#8U]\n", 10)
	fmt.Printf("[%+#U]\n", 10)
	fmt.Printf("[%-#8U]\n", 10)
	fmt.Printf("[% U]\n", 10)
	fmt.Printf("[%3.8U]\n", 10)
	fmt.Printf("[%U]\n", -1)
	fmt.Printf("[%+U]\n", -1)
	fmt.Printf("[%-U]\n", -1)
	fmt.Printf("[%#U]\n", -1)
	fmt.Printf("[%8U]\n", -1)
	fmt.Printf("[%-8U]\n", -1)
	fmt.Printf("[%08U]\n", -1)
	fmt.Printf("[%.3U]\n", -1)
	fmt.Printf("[%#8U]\n", -1)
	fmt.Printf("[%+#U]\n", -1)
	fmt.Printf("[%-#8U]\n", -1)
	fmt.Printf("[% U]\n", -1)
	fmt.Printf("[%3.8U]\n", -1)
	fmt.Printf("[%U]\n", 0x10FFFF)
	fmt.Printf("[%+U]\n", 0x10FFFF)
	fmt.Printf("[%-U]\n", 0x10FFFF)
	fmt.Printf("[%#U]\n", 0x10FFFF)
	fmt.Printf("[%8U]\n", 0x10FFFF)
	fmt.Printf("[%-8U]\n", 0x10FFFF)
	fmt.Printf("[%08U]\n", 0x10FFFF)
	fmt.Printf("[%.3U]\n", 0x10FFFF)
	fmt.Printf("[%#8U]\n", 0x10FFFF)
	fmt.Printf("[%+#U]\n", 0x10FFFF)
	fmt.Printf("[%-#8U]\n", 0x10FFFF)
	fmt.Printf("[% U]\n", 0x10FFFF)
	fmt.Printf("[%3.8U]\n", 0x10FFFF)
	fmt.Printf("[%U]\n", 0x110000)
	fmt.Printf("[%+U]\n", 0x110000)
	fmt.Printf("[%-U]\n", 0x110000)
	fmt.Printf("[%#U]\n", 0x110000)
	fmt.Printf("[%8U]\n", 0x110000)
	fmt.Printf("[%-8U]\n", 0x110000)
	fmt.Printf("[%08U]\n", 0x110000)
	fmt.Printf("[%.3U]\n", 0x110000)
	fmt.Printf("[%#8U]\n", 0x110000)
	fmt.Printf("[%+#U]\n", 0x110000)
	fmt.Printf("[%-#8U]\n", 0x110000)
	fmt.Printf("[% U]\n", 0x110000)
	fmt.Printf("[%3.8U]\n", 0x110000)
	fmt.Printf("[%U]\n", 0xD800)
	fmt.Printf("[%+U]\n", 0xD800)
	fmt.Printf("[%-U]\n", 0xD800)
	fmt.Printf("[%#U]\n", 0xD800)
	fmt.Printf("[%8U]\n", 0xD800)
	fmt.Printf("[%-8U]\n", 0xD800)
	fmt.Printf("[%08U]\n", 0xD800)
	fmt.Printf("[%.3U]\n", 0xD800)
	fmt.Printf("[%#8U]\n", 0xD800)
	fmt.Printf("[%+#U]\n", 0xD800)
	fmt.Printf("[%-#8U]\n", 0xD800)
	fmt.Printf("[% U]\n", 0xD800)
	fmt.Printf("[%3.8U]\n", 0xD800)
	fmt.Printf("[%U]\n", 127)
	fmt.Printf("[%+U]\n", 127)
	fmt.Printf("[%-U]\n", 127)
	fmt.Printf("[%#U]\n", 127)
	fmt.Printf("[%8U]\n", 127)
	fmt.Printf("[%-8U]\n", 127)
	fmt.Printf("[%08U]\n", 127)
	fmt.Printf("[%.3U]\n", 127)
	fmt.Printf("[%#8U]\n", 127)
	fmt.Printf("[%+#U]\n", 127)
	fmt.Printf("[%-#8U]\n", 127)
	fmt.Printf("[% U]\n", 127)
	fmt.Printf("[%3.8U]\n", 127)
	fmt.Printf("[%U]\n", 0xFFFD)
	fmt.Printf("[%+U]\n", 0xFFFD)
	fmt.Printf("[%-U]\n", 0xFFFD)
	fmt.Printf("[%#U]\n", 0xFFFD)
	fmt.Printf("[%8U]\n", 0xFFFD)
	fmt.Printf("[%-8U]\n", 0xFFFD)
	fmt.Printf("[%08U]\n", 0xFFFD)
	fmt.Printf("[%.3U]\n", 0xFFFD)
	fmt.Printf("[%#8U]\n", 0xFFFD)
	fmt.Printf("[%+#U]\n", 0xFFFD)
	fmt.Printf("[%-#8U]\n", 0xFFFD)
	fmt.Printf("[% U]\n", 0xFFFD)
	fmt.Printf("[%3.8U]\n", 0xFFFD)
	fmt.Printf("[%U]\n", '\'')
	fmt.Printf("[%+U]\n", '\'')
	fmt.Printf("[%-U]\n", '\'')
	fmt.Printf("[%#U]\n", '\'')
	fmt.Printf("[%8U]\n", '\'')
	fmt.Printf("[%-8U]\n", '\'')
	fmt.Printf("[%08U]\n", '\'')
	fmt.Printf("[%.3U]\n", '\'')
	fmt.Printf("[%#8U]\n", '\'')
	fmt.Printf("[%+#U]\n", '\'')
	fmt.Printf("[%-#8U]\n", '\'')
	fmt.Printf("[% U]\n", '\'')
	fmt.Printf("[%3.8U]\n", '\'')
	fmt.Printf("[%U]\n", '"')
	fmt.Printf("[%+U]\n", '"')
	fmt.Printf("[%-U]\n", '"')
	fmt.Printf("[%#U]\n", '"')
	fmt.Printf("[%8U]\n", '"')
	fmt.Printf("[%-8U]\n", '"')
	fmt.Printf("[%08U]\n", '"')
	fmt.Printf("[%.3U]\n", '"')
	fmt.Printf("[%#8U]\n", '"')
	fmt.Printf("[%+#U]\n", '"')
	fmt.Printf("[%-#8U]\n", '"')
	fmt.Printf("[% U]\n", '"')
	fmt.Printf("[%3.8U]\n", '"')
	fmt.Printf("[%U]\n", '\\')
	fmt.Printf("[%+U]\n", '\\')
	fmt.Printf("[%-U]\n", '\\')
	fmt.Printf("[%#U]\n", '\\')
	fmt.Printf("[%8U]\n", '\\')
	fmt.Printf("[%-8U]\n", '\\')
	fmt.Printf("[%08U]\n", '\\')
	fmt.Printf("[%.3U]\n", '\\')
	fmt.Printf("[%#8U]\n", '\\')
	fmt.Printf("[%+#U]\n", '\\')
	fmt.Printf("[%-#8U]\n", '\\')
	fmt.Printf("[% U]\n", '\\')
	fmt.Printf("[%3.8U]\n", '\\')
	fmt.Printf("[%U]\n", 7)
	fmt.Printf("[%+U]\n", 7)
	fmt.Printf("[%-U]\n", 7)
	fmt.Printf("[%#U]\n", 7)
	fmt.Printf("[%8U]\n", 7)
	fmt.Printf("[%-8U]\n", 7)
	fmt.Printf("[%08U]\n", 7)
	fmt.Printf("[%.3U]\n", 7)
	fmt.Printf("[%#8U]\n", 7)
	fmt.Printf("[%+#U]\n", 7)
	fmt.Printf("[%-#8U]\n", 7)
	fmt.Printf("[% U]\n", 7)
	fmt.Printf("[%3.8U]\n", 7)
	fmt.Printf("[%U]\n", 0x80)
	fmt.Printf("[%+U]\n", 0x80)
	fmt.Printf("[%-U]\n", 0x80)
	fmt.Printf("[%#U]\n", 0x80)
	fmt.Printf("[%8U]\n", 0x80)
	fmt.Printf("[%-8U]\n", 0x80)
	fmt.Printf("[%08U]\n", 0x80)
	fmt.Printf("[%.3U]\n", 0x80)
	fmt.Printf("[%#8U]\n", 0x80)
	fmt.Printf("[%+#U]\n", 0x80)
	fmt.Printf("[%-#8U]\n", 0x80)
	fmt.Printf("[% U]\n", 0x80)
	fmt.Printf("[%3.8U]\n", 0x80)
	fmt.Printf("[%U]\n", 0x200B)
	fmt.Printf("[%+U]\n", 0x200B)
	fmt.Printf("[%-U]\n", 0x200B)
	fmt.Printf("[%#U]\n", 0x200B)
	fmt.Printf("[%8U]\n", 0x200B)
	fmt.Printf("[%-8U]\n", 0x200B)
	fmt.Printf("[%08U]\n", 0x200B)
	fmt.Printf("[%.3U]\n", 0x200B)
	fmt.Printf("[%#8U]\n", 0x200B)
	fmt.Printf("[%+#U]\n", 0x200B)
	fmt.Printf("[%-#8U]\n", 0x200B)
	fmt.Printf("[% U]\n", 0x200B)
	fmt.Printf("[%3.8U]\n", 0x200B)
}

// Output:
// [U+0041]
// [U+0041]
// [U+0041]
// [U+0041 'A']
// [  U+0041]
// [U+0041  ]
// [  U+0041]
// [U+0041]
// [U+0041 'A']
// [U+0041 'A']
// [U+0041 'A']
// [U+0041]
// [U+00000041]
// [U+263A]
// [U+263A]
// [U+263A]
// [U+263A '☺']
// [  U+263A]
// [U+263A  ]
// [  U+263A]
// [U+263A]
// [U+263A '☺']
// [U+263A '☺']
// [U+263A '☺']
// [U+263A]
// [U+0000263A]
// [U+1F600]
// [U+1F600]
// [U+1F600]
// [U+1F600 '😀']
// [ U+1F600]
// [U+1F600 ]
// [ U+1F600]
// [U+1F600]
// [U+1F600 '😀']
// [U+1F600 '😀']
// [U+1F600 '😀']
// [U+1F600]
// [U+0001F600]
// [U+0000]
// [U+0000]
// [U+0000]
// [U+0000]
// [  U+0000]
// [U+0000  ]
// [  U+0000]
// [U+0000]
// [  U+0000]
// [U+0000]
// [U+0000  ]
// [U+0000]
// [U+00000000]
// [U+000A]
// [U+000A]
// [U+000A]
// [U+000A]
// [  U+000A]
// [U+000A  ]
// [  U+000A]
// [U+000A]
// [  U+000A]
// [U+000A]
// [U+000A  ]
// [U+000A]
// [U+0000000A]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+FFFFFFFFFFFFFFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+10FFFF]
// [U+0010FFFF]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+110000]
// [U+00110000]
// [U+D800]
// [U+D800]
// [U+D800]
// [U+D800]
// [  U+D800]
// [U+D800  ]
// [  U+D800]
// [U+D800]
// [  U+D800]
// [U+D800]
// [U+D800  ]
// [U+D800]
// [U+0000D800]
// [U+007F]
// [U+007F]
// [U+007F]
// [U+007F]
// [  U+007F]
// [U+007F  ]
// [  U+007F]
// [U+007F]
// [  U+007F]
// [U+007F]
// [U+007F  ]
// [U+007F]
// [U+0000007F]
// [U+FFFD]
// [U+FFFD]
// [U+FFFD]
// [U+FFFD '�']
// [  U+FFFD]
// [U+FFFD  ]
// [  U+FFFD]
// [U+FFFD]
// [U+FFFD '�']
// [U+FFFD '�']
// [U+FFFD '�']
// [U+FFFD]
// [U+0000FFFD]
// [U+0027]
// [U+0027]
// [U+0027]
// [U+0027 ''']
// [  U+0027]
// [U+0027  ]
// [  U+0027]
// [U+0027]
// [U+0027 ''']
// [U+0027 ''']
// [U+0027 ''']
// [U+0027]
// [U+00000027]
// [U+0022]
// [U+0022]
// [U+0022]
// [U+0022 '"']
// [  U+0022]
// [U+0022  ]
// [  U+0022]
// [U+0022]
// [U+0022 '"']
// [U+0022 '"']
// [U+0022 '"']
// [U+0022]
// [U+00000022]
// [U+005C]
// [U+005C]
// [U+005C]
// [U+005C '\']
// [  U+005C]
// [U+005C  ]
// [  U+005C]
// [U+005C]
// [U+005C '\']
// [U+005C '\']
// [U+005C '\']
// [U+005C]
// [U+0000005C]
// [U+0007]
// [U+0007]
// [U+0007]
// [U+0007]
// [  U+0007]
// [U+0007  ]
// [  U+0007]
// [U+0007]
// [  U+0007]
// [U+0007]
// [U+0007  ]
// [U+0007]
// [U+00000007]
// [U+0080]
// [U+0080]
// [U+0080]
// [U+0080]
// [  U+0080]
// [U+0080  ]
// [  U+0080]
// [U+0080]
// [  U+0080]
// [U+0080]
// [U+0080  ]
// [U+0080]
// [U+00000080]
// [U+200B]
// [U+200B]
// [U+200B]
// [U+200B]
// [  U+200B]
// [U+200B  ]
// [  U+200B]
// [U+200B]
// [  U+200B]
// [U+200B]
// [U+200B  ]
// [U+200B]
// [U+0000200B]
