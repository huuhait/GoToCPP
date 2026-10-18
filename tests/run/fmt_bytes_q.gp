// phase: 05

// Go's %q of byte slices: %s %q %x %X treat them as strings, %v and %d as lists of numbers.
package main

import "fmt"

func main() {
	fmt.Printf("[%q]\n", []byte(nil))
	fmt.Printf("[%+q]\n", []byte(nil))
	fmt.Printf("[%-q]\n", []byte(nil))
	fmt.Printf("[% q]\n", []byte(nil))
	fmt.Printf("[%#q]\n", []byte(nil))
	fmt.Printf("[%10q]\n", []byte(nil))
	fmt.Printf("[%-10q]\n", []byte(nil))
	fmt.Printf("[%.3q]\n", []byte(nil))
	fmt.Printf("[%10.2q]\n", []byte(nil))
	fmt.Printf("[%# q]\n", []byte(nil))
	fmt.Printf("[%#10q]\n", []byte(nil))
	fmt.Printf("[%020q]\n", []byte(nil))
	fmt.Printf("[%q]\n", []byte{})
	fmt.Printf("[%+q]\n", []byte{})
	fmt.Printf("[%-q]\n", []byte{})
	fmt.Printf("[% q]\n", []byte{})
	fmt.Printf("[%#q]\n", []byte{})
	fmt.Printf("[%10q]\n", []byte{})
	fmt.Printf("[%-10q]\n", []byte{})
	fmt.Printf("[%.3q]\n", []byte{})
	fmt.Printf("[%10.2q]\n", []byte{})
	fmt.Printf("[%# q]\n", []byte{})
	fmt.Printf("[%#10q]\n", []byte{})
	fmt.Printf("[%020q]\n", []byte{})
	fmt.Printf("[%q]\n", []byte("hello"))
	fmt.Printf("[%+q]\n", []byte("hello"))
	fmt.Printf("[%-q]\n", []byte("hello"))
	fmt.Printf("[% q]\n", []byte("hello"))
	fmt.Printf("[%#q]\n", []byte("hello"))
	fmt.Printf("[%10q]\n", []byte("hello"))
	fmt.Printf("[%-10q]\n", []byte("hello"))
	fmt.Printf("[%.3q]\n", []byte("hello"))
	fmt.Printf("[%10.2q]\n", []byte("hello"))
	fmt.Printf("[%# q]\n", []byte("hello"))
	fmt.Printf("[%#10q]\n", []byte("hello"))
	fmt.Printf("[%020q]\n", []byte("hello"))
	fmt.Printf("[%q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%+q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[% q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-10q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%.3q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10.2q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%# q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#10q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%020q]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%q]\n", []byte("日本語"))
	fmt.Printf("[%+q]\n", []byte("日本語"))
	fmt.Printf("[%-q]\n", []byte("日本語"))
	fmt.Printf("[% q]\n", []byte("日本語"))
	fmt.Printf("[%#q]\n", []byte("日本語"))
	fmt.Printf("[%10q]\n", []byte("日本語"))
	fmt.Printf("[%-10q]\n", []byte("日本語"))
	fmt.Printf("[%.3q]\n", []byte("日本語"))
	fmt.Printf("[%10.2q]\n", []byte("日本語"))
	fmt.Printf("[%# q]\n", []byte("日本語"))
	fmt.Printf("[%#10q]\n", []byte("日本語"))
	fmt.Printf("[%020q]\n", []byte("日本語"))
	fmt.Printf("[%q]\n", []byte("a b"))
	fmt.Printf("[%+q]\n", []byte("a b"))
	fmt.Printf("[%-q]\n", []byte("a b"))
	fmt.Printf("[% q]\n", []byte("a b"))
	fmt.Printf("[%#q]\n", []byte("a b"))
	fmt.Printf("[%10q]\n", []byte("a b"))
	fmt.Printf("[%-10q]\n", []byte("a b"))
	fmt.Printf("[%.3q]\n", []byte("a b"))
	fmt.Printf("[%10.2q]\n", []byte("a b"))
	fmt.Printf("[%# q]\n", []byte("a b"))
	fmt.Printf("[%#10q]\n", []byte("a b"))
	fmt.Printf("[%020q]\n", []byte("a b"))
	fmt.Printf("[%q]\n", []byte("tab\t"))
	fmt.Printf("[%+q]\n", []byte("tab\t"))
	fmt.Printf("[%-q]\n", []byte("tab\t"))
	fmt.Printf("[% q]\n", []byte("tab\t"))
	fmt.Printf("[%#q]\n", []byte("tab\t"))
	fmt.Printf("[%10q]\n", []byte("tab\t"))
	fmt.Printf("[%-10q]\n", []byte("tab\t"))
	fmt.Printf("[%.3q]\n", []byte("tab\t"))
	fmt.Printf("[%10.2q]\n", []byte("tab\t"))
	fmt.Printf("[%# q]\n", []byte("tab\t"))
	fmt.Printf("[%#10q]\n", []byte("tab\t"))
	fmt.Printf("[%020q]\n", []byte("tab\t"))
}

// Output:
// [""]
// [""]
// [""]
// [""]
// [``]
// [        ""]
// [""        ]
// [""]
// [        ""]
// [``]
// [        ``]
// [000000000000000000""]
// [""]
// [""]
// [""]
// [""]
// [``]
// [        ""]
// [""        ]
// [""]
// [        ""]
// [``]
// [        ``]
// [000000000000000000""]
// ["hello"]
// ["hello"]
// ["hello"]
// ["hello"]
// [`hello`]
// [   "hello"]
// ["hello"   ]
// ["hel"]
// [      "he"]
// [`hello`]
// [   `hello`]
// [0000000000000"hello"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02"]
// ["\x00\x01"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02\xfe\xff"]
// ["\x00\x01\x02\xfe\xff"]
// ["日本語"]
// ["\u65e5\u672c\u8a9e"]
// ["日本語"]
// ["日本語"]
// [`日本語`]
// [     "日本語"]
// ["日本語"     ]
// ["日本語"]
// [      "日本"]
// [`日本語`]
// [     `日本語`]
// [000000000000000"日本語"]
// ["a b"]
// ["a b"]
// ["a b"]
// ["a b"]
// [`a b`]
// [     "a b"]
// ["a b"     ]
// ["a b"]
// [      "a "]
// [`a b`]
// [     `a b`]
// [000000000000000"a b"]
// ["tab\t"]
// ["tab\t"]
// ["tab\t"]
// ["tab\t"]
// [`tab	`]
// [   "tab\t"]
// ["tab\t"   ]
// ["tab"]
// [      "ta"]
// [`tab	`]
// [    `tab	`]
// [0000000000000"tab\t"]
