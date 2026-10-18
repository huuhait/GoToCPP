// phase: 05

// Go's %q of integers: code points valid and invalid (surrogates, beyond U+10FFFF, negative),
// with flags.
package main

import "fmt"

func main() {
	fmt.Printf("[%q]\n", 65)
	fmt.Printf("[%+q]\n", 65)
	fmt.Printf("[%-q]\n", 65)
	fmt.Printf("[%#q]\n", 65)
	fmt.Printf("[%8q]\n", 65)
	fmt.Printf("[%-8q]\n", 65)
	fmt.Printf("[%08q]\n", 65)
	fmt.Printf("[%.3q]\n", 65)
	fmt.Printf("[%#8q]\n", 65)
	fmt.Printf("[%+#q]\n", 65)
	fmt.Printf("[%-#8q]\n", 65)
	fmt.Printf("[% q]\n", 65)
	fmt.Printf("[%3.8q]\n", 65)
	fmt.Printf("[%q]\n", 0x263A)
	fmt.Printf("[%+q]\n", 0x263A)
	fmt.Printf("[%-q]\n", 0x263A)
	fmt.Printf("[%#q]\n", 0x263A)
	fmt.Printf("[%8q]\n", 0x263A)
	fmt.Printf("[%-8q]\n", 0x263A)
	fmt.Printf("[%08q]\n", 0x263A)
	fmt.Printf("[%.3q]\n", 0x263A)
	fmt.Printf("[%#8q]\n", 0x263A)
	fmt.Printf("[%+#q]\n", 0x263A)
	fmt.Printf("[%-#8q]\n", 0x263A)
	fmt.Printf("[% q]\n", 0x263A)
	fmt.Printf("[%3.8q]\n", 0x263A)
	fmt.Printf("[%q]\n", 0x1F600)
	fmt.Printf("[%+q]\n", 0x1F600)
	fmt.Printf("[%-q]\n", 0x1F600)
	fmt.Printf("[%#q]\n", 0x1F600)
	fmt.Printf("[%8q]\n", 0x1F600)
	fmt.Printf("[%-8q]\n", 0x1F600)
	fmt.Printf("[%08q]\n", 0x1F600)
	fmt.Printf("[%.3q]\n", 0x1F600)
	fmt.Printf("[%#8q]\n", 0x1F600)
	fmt.Printf("[%+#q]\n", 0x1F600)
	fmt.Printf("[%-#8q]\n", 0x1F600)
	fmt.Printf("[% q]\n", 0x1F600)
	fmt.Printf("[%3.8q]\n", 0x1F600)
	fmt.Printf("[%q]\n", 0)
	fmt.Printf("[%+q]\n", 0)
	fmt.Printf("[%-q]\n", 0)
	fmt.Printf("[%#q]\n", 0)
	fmt.Printf("[%8q]\n", 0)
	fmt.Printf("[%-8q]\n", 0)
	fmt.Printf("[%08q]\n", 0)
	fmt.Printf("[%.3q]\n", 0)
	fmt.Printf("[%#8q]\n", 0)
	fmt.Printf("[%+#q]\n", 0)
	fmt.Printf("[%-#8q]\n", 0)
	fmt.Printf("[% q]\n", 0)
	fmt.Printf("[%3.8q]\n", 0)
	fmt.Printf("[%q]\n", 10)
	fmt.Printf("[%+q]\n", 10)
	fmt.Printf("[%-q]\n", 10)
	fmt.Printf("[%#q]\n", 10)
	fmt.Printf("[%8q]\n", 10)
	fmt.Printf("[%-8q]\n", 10)
	fmt.Printf("[%08q]\n", 10)
	fmt.Printf("[%.3q]\n", 10)
	fmt.Printf("[%#8q]\n", 10)
	fmt.Printf("[%+#q]\n", 10)
	fmt.Printf("[%-#8q]\n", 10)
	fmt.Printf("[% q]\n", 10)
	fmt.Printf("[%3.8q]\n", 10)
	fmt.Printf("[%q]\n", -1)
	fmt.Printf("[%+q]\n", -1)
	fmt.Printf("[%-q]\n", -1)
	fmt.Printf("[%#q]\n", -1)
	fmt.Printf("[%8q]\n", -1)
	fmt.Printf("[%-8q]\n", -1)
	fmt.Printf("[%08q]\n", -1)
	fmt.Printf("[%.3q]\n", -1)
	fmt.Printf("[%#8q]\n", -1)
	fmt.Printf("[%+#q]\n", -1)
	fmt.Printf("[%-#8q]\n", -1)
	fmt.Printf("[% q]\n", -1)
	fmt.Printf("[%3.8q]\n", -1)
	fmt.Printf("[%q]\n", 0x10FFFF)
	fmt.Printf("[%+q]\n", 0x10FFFF)
	fmt.Printf("[%-q]\n", 0x10FFFF)
	fmt.Printf("[%#q]\n", 0x10FFFF)
	fmt.Printf("[%8q]\n", 0x10FFFF)
	fmt.Printf("[%-8q]\n", 0x10FFFF)
	fmt.Printf("[%08q]\n", 0x10FFFF)
	fmt.Printf("[%.3q]\n", 0x10FFFF)
	fmt.Printf("[%#8q]\n", 0x10FFFF)
	fmt.Printf("[%+#q]\n", 0x10FFFF)
	fmt.Printf("[%-#8q]\n", 0x10FFFF)
	fmt.Printf("[% q]\n", 0x10FFFF)
	fmt.Printf("[%3.8q]\n", 0x10FFFF)
	fmt.Printf("[%q]\n", 0x110000)
	fmt.Printf("[%+q]\n", 0x110000)
	fmt.Printf("[%-q]\n", 0x110000)
	fmt.Printf("[%#q]\n", 0x110000)
	fmt.Printf("[%8q]\n", 0x110000)
	fmt.Printf("[%-8q]\n", 0x110000)
	fmt.Printf("[%08q]\n", 0x110000)
	fmt.Printf("[%.3q]\n", 0x110000)
	fmt.Printf("[%#8q]\n", 0x110000)
	fmt.Printf("[%+#q]\n", 0x110000)
	fmt.Printf("[%-#8q]\n", 0x110000)
	fmt.Printf("[% q]\n", 0x110000)
	fmt.Printf("[%3.8q]\n", 0x110000)
	fmt.Printf("[%q]\n", 0xD800)
	fmt.Printf("[%+q]\n", 0xD800)
	fmt.Printf("[%-q]\n", 0xD800)
	fmt.Printf("[%#q]\n", 0xD800)
	fmt.Printf("[%8q]\n", 0xD800)
	fmt.Printf("[%-8q]\n", 0xD800)
	fmt.Printf("[%08q]\n", 0xD800)
	fmt.Printf("[%.3q]\n", 0xD800)
	fmt.Printf("[%#8q]\n", 0xD800)
	fmt.Printf("[%+#q]\n", 0xD800)
	fmt.Printf("[%-#8q]\n", 0xD800)
	fmt.Printf("[% q]\n", 0xD800)
	fmt.Printf("[%3.8q]\n", 0xD800)
	fmt.Printf("[%q]\n", 127)
	fmt.Printf("[%+q]\n", 127)
	fmt.Printf("[%-q]\n", 127)
	fmt.Printf("[%#q]\n", 127)
	fmt.Printf("[%8q]\n", 127)
	fmt.Printf("[%-8q]\n", 127)
	fmt.Printf("[%08q]\n", 127)
	fmt.Printf("[%.3q]\n", 127)
	fmt.Printf("[%#8q]\n", 127)
	fmt.Printf("[%+#q]\n", 127)
	fmt.Printf("[%-#8q]\n", 127)
	fmt.Printf("[% q]\n", 127)
	fmt.Printf("[%3.8q]\n", 127)
	fmt.Printf("[%q]\n", 0xFFFD)
	fmt.Printf("[%+q]\n", 0xFFFD)
	fmt.Printf("[%-q]\n", 0xFFFD)
	fmt.Printf("[%#q]\n", 0xFFFD)
	fmt.Printf("[%8q]\n", 0xFFFD)
	fmt.Printf("[%-8q]\n", 0xFFFD)
	fmt.Printf("[%08q]\n", 0xFFFD)
	fmt.Printf("[%.3q]\n", 0xFFFD)
	fmt.Printf("[%#8q]\n", 0xFFFD)
	fmt.Printf("[%+#q]\n", 0xFFFD)
	fmt.Printf("[%-#8q]\n", 0xFFFD)
	fmt.Printf("[% q]\n", 0xFFFD)
	fmt.Printf("[%3.8q]\n", 0xFFFD)
	fmt.Printf("[%q]\n", '\'')
	fmt.Printf("[%+q]\n", '\'')
	fmt.Printf("[%-q]\n", '\'')
	fmt.Printf("[%#q]\n", '\'')
	fmt.Printf("[%8q]\n", '\'')
	fmt.Printf("[%-8q]\n", '\'')
	fmt.Printf("[%08q]\n", '\'')
	fmt.Printf("[%.3q]\n", '\'')
	fmt.Printf("[%#8q]\n", '\'')
	fmt.Printf("[%+#q]\n", '\'')
	fmt.Printf("[%-#8q]\n", '\'')
	fmt.Printf("[% q]\n", '\'')
	fmt.Printf("[%3.8q]\n", '\'')
	fmt.Printf("[%q]\n", '"')
	fmt.Printf("[%+q]\n", '"')
	fmt.Printf("[%-q]\n", '"')
	fmt.Printf("[%#q]\n", '"')
	fmt.Printf("[%8q]\n", '"')
	fmt.Printf("[%-8q]\n", '"')
	fmt.Printf("[%08q]\n", '"')
	fmt.Printf("[%.3q]\n", '"')
	fmt.Printf("[%#8q]\n", '"')
	fmt.Printf("[%+#q]\n", '"')
	fmt.Printf("[%-#8q]\n", '"')
	fmt.Printf("[% q]\n", '"')
	fmt.Printf("[%3.8q]\n", '"')
	fmt.Printf("[%q]\n", '\\')
	fmt.Printf("[%+q]\n", '\\')
	fmt.Printf("[%-q]\n", '\\')
	fmt.Printf("[%#q]\n", '\\')
	fmt.Printf("[%8q]\n", '\\')
	fmt.Printf("[%-8q]\n", '\\')
	fmt.Printf("[%08q]\n", '\\')
	fmt.Printf("[%.3q]\n", '\\')
	fmt.Printf("[%#8q]\n", '\\')
	fmt.Printf("[%+#q]\n", '\\')
	fmt.Printf("[%-#8q]\n", '\\')
	fmt.Printf("[% q]\n", '\\')
	fmt.Printf("[%3.8q]\n", '\\')
	fmt.Printf("[%q]\n", 7)
	fmt.Printf("[%+q]\n", 7)
	fmt.Printf("[%-q]\n", 7)
	fmt.Printf("[%#q]\n", 7)
	fmt.Printf("[%8q]\n", 7)
	fmt.Printf("[%-8q]\n", 7)
	fmt.Printf("[%08q]\n", 7)
	fmt.Printf("[%.3q]\n", 7)
	fmt.Printf("[%#8q]\n", 7)
	fmt.Printf("[%+#q]\n", 7)
	fmt.Printf("[%-#8q]\n", 7)
	fmt.Printf("[% q]\n", 7)
	fmt.Printf("[%3.8q]\n", 7)
	fmt.Printf("[%q]\n", 0x80)
	fmt.Printf("[%+q]\n", 0x80)
	fmt.Printf("[%-q]\n", 0x80)
	fmt.Printf("[%#q]\n", 0x80)
	fmt.Printf("[%8q]\n", 0x80)
	fmt.Printf("[%-8q]\n", 0x80)
	fmt.Printf("[%08q]\n", 0x80)
	fmt.Printf("[%.3q]\n", 0x80)
	fmt.Printf("[%#8q]\n", 0x80)
	fmt.Printf("[%+#q]\n", 0x80)
	fmt.Printf("[%-#8q]\n", 0x80)
	fmt.Printf("[% q]\n", 0x80)
	fmt.Printf("[%3.8q]\n", 0x80)
	fmt.Printf("[%q]\n", 0x200B)
	fmt.Printf("[%+q]\n", 0x200B)
	fmt.Printf("[%-q]\n", 0x200B)
	fmt.Printf("[%#q]\n", 0x200B)
	fmt.Printf("[%8q]\n", 0x200B)
	fmt.Printf("[%-8q]\n", 0x200B)
	fmt.Printf("[%08q]\n", 0x200B)
	fmt.Printf("[%.3q]\n", 0x200B)
	fmt.Printf("[%#8q]\n", 0x200B)
	fmt.Printf("[%+#q]\n", 0x200B)
	fmt.Printf("[%-#8q]\n", 0x200B)
	fmt.Printf("[% q]\n", 0x200B)
	fmt.Printf("[%3.8q]\n", 0x200B)
}

// Output:
// ['A']
// ['A']
// ['A']
// ['A']
// [     'A']
// ['A'     ]
// [00000'A']
// ['A']
// [     'A']
// ['A']
// ['A'     ]
// ['A']
// ['A']
// ['☺']
// ['\u263a']
// ['☺']
// ['☺']
// [     '☺']
// ['☺'     ]
// [00000'☺']
// ['☺']
// [     '☺']
// ['\u263a']
// ['☺'     ]
// ['☺']
// ['☺']
// ['😀']
// ['\U0001f600']
// ['😀']
// ['😀']
// [     '😀']
// ['😀'     ]
// [00000'😀']
// ['😀']
// [     '😀']
// ['\U0001f600']
// ['😀'     ]
// ['😀']
// ['😀']
// ['\x00']
// ['\x00']
// ['\x00']
// ['\x00']
// [  '\x00']
// ['\x00'  ]
// [00'\x00']
// ['\x00']
// [  '\x00']
// ['\x00']
// ['\x00'  ]
// ['\x00']
// ['\x00']
// ['\n']
// ['\n']
// ['\n']
// ['\n']
// [    '\n']
// ['\n'    ]
// [0000'\n']
// ['\n']
// [    '\n']
// ['\n']
// ['\n'    ]
// ['\n']
// ['\n']
// ['�']
// ['\ufffd']
// ['�']
// ['�']
// [     '�']
// ['�'     ]
// [00000'�']
// ['�']
// [     '�']
// ['\ufffd']
// ['�'     ]
// ['�']
// ['�']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['\U0010ffff']
// ['�']
// ['\ufffd']
// ['�']
// ['�']
// [     '�']
// ['�'     ]
// [00000'�']
// ['�']
// [     '�']
// ['\ufffd']
// ['�'     ]
// ['�']
// ['�']
// ['�']
// ['\ufffd']
// ['�']
// ['�']
// [     '�']
// ['�'     ]
// [00000'�']
// ['�']
// [     '�']
// ['\ufffd']
// ['�'     ]
// ['�']
// ['�']
// ['\x7f']
// ['\x7f']
// ['\x7f']
// ['\x7f']
// [  '\x7f']
// ['\x7f'  ]
// [00'\x7f']
// ['\x7f']
// [  '\x7f']
// ['\x7f']
// ['\x7f'  ]
// ['\x7f']
// ['\x7f']
// ['�']
// ['\ufffd']
// ['�']
// ['�']
// [     '�']
// ['�'     ]
// [00000'�']
// ['�']
// [     '�']
// ['\ufffd']
// ['�'     ]
// ['�']
// ['�']
// ['\'']
// ['\'']
// ['\'']
// ['\'']
// [    '\'']
// ['\''    ]
// [0000'\'']
// ['\'']
// [    '\'']
// ['\'']
// ['\''    ]
// ['\'']
// ['\'']
// ['"']
// ['"']
// ['"']
// ['"']
// [     '"']
// ['"'     ]
// [00000'"']
// ['"']
// [     '"']
// ['"']
// ['"'     ]
// ['"']
// ['"']
// ['\\']
// ['\\']
// ['\\']
// ['\\']
// [    '\\']
// ['\\'    ]
// [0000'\\']
// ['\\']
// [    '\\']
// ['\\']
// ['\\'    ]
// ['\\']
// ['\\']
// ['\a']
// ['\a']
// ['\a']
// ['\a']
// [    '\a']
// ['\a'    ]
// [0000'\a']
// ['\a']
// [    '\a']
// ['\a']
// ['\a'    ]
// ['\a']
// ['\a']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u0080']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
// ['\u200b']
