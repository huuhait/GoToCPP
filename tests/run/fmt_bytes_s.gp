// phase: 05

// Go's %s of byte slices: %s %q %x %X treat them as strings, %v and %d as lists of numbers.
package main

import "fmt"

func main() {
	fmt.Printf("[%s]\n", []byte(nil))
	fmt.Printf("[%+s]\n", []byte(nil))
	fmt.Printf("[%-s]\n", []byte(nil))
	fmt.Printf("[% s]\n", []byte(nil))
	fmt.Printf("[%#s]\n", []byte(nil))
	fmt.Printf("[%10s]\n", []byte(nil))
	fmt.Printf("[%-10s]\n", []byte(nil))
	fmt.Printf("[%.3s]\n", []byte(nil))
	fmt.Printf("[%10.2s]\n", []byte(nil))
	fmt.Printf("[%# s]\n", []byte(nil))
	fmt.Printf("[%#10s]\n", []byte(nil))
	fmt.Printf("[%020s]\n", []byte(nil))
	fmt.Printf("[%s]\n", []byte{})
	fmt.Printf("[%+s]\n", []byte{})
	fmt.Printf("[%-s]\n", []byte{})
	fmt.Printf("[% s]\n", []byte{})
	fmt.Printf("[%#s]\n", []byte{})
	fmt.Printf("[%10s]\n", []byte{})
	fmt.Printf("[%-10s]\n", []byte{})
	fmt.Printf("[%.3s]\n", []byte{})
	fmt.Printf("[%10.2s]\n", []byte{})
	fmt.Printf("[%# s]\n", []byte{})
	fmt.Printf("[%#10s]\n", []byte{})
	fmt.Printf("[%020s]\n", []byte{})
	fmt.Printf("[%s]\n", []byte("hello"))
	fmt.Printf("[%+s]\n", []byte("hello"))
	fmt.Printf("[%-s]\n", []byte("hello"))
	fmt.Printf("[% s]\n", []byte("hello"))
	fmt.Printf("[%#s]\n", []byte("hello"))
	fmt.Printf("[%10s]\n", []byte("hello"))
	fmt.Printf("[%-10s]\n", []byte("hello"))
	fmt.Printf("[%.3s]\n", []byte("hello"))
	fmt.Printf("[%10.2s]\n", []byte("hello"))
	fmt.Printf("[%# s]\n", []byte("hello"))
	fmt.Printf("[%#10s]\n", []byte("hello"))
	fmt.Printf("[%020s]\n", []byte("hello"))
	fmt.Printf("[%s]\n", []byte("日本語"))
	fmt.Printf("[%+s]\n", []byte("日本語"))
	fmt.Printf("[%-s]\n", []byte("日本語"))
	fmt.Printf("[% s]\n", []byte("日本語"))
	fmt.Printf("[%#s]\n", []byte("日本語"))
	fmt.Printf("[%10s]\n", []byte("日本語"))
	fmt.Printf("[%-10s]\n", []byte("日本語"))
	fmt.Printf("[%.3s]\n", []byte("日本語"))
	fmt.Printf("[%10.2s]\n", []byte("日本語"))
	fmt.Printf("[%# s]\n", []byte("日本語"))
	fmt.Printf("[%#10s]\n", []byte("日本語"))
	fmt.Printf("[%020s]\n", []byte("日本語"))
	fmt.Printf("[%s]\n", []byte("a b"))
	fmt.Printf("[%+s]\n", []byte("a b"))
	fmt.Printf("[%-s]\n", []byte("a b"))
	fmt.Printf("[% s]\n", []byte("a b"))
	fmt.Printf("[%#s]\n", []byte("a b"))
	fmt.Printf("[%10s]\n", []byte("a b"))
	fmt.Printf("[%-10s]\n", []byte("a b"))
	fmt.Printf("[%.3s]\n", []byte("a b"))
	fmt.Printf("[%10.2s]\n", []byte("a b"))
	fmt.Printf("[%# s]\n", []byte("a b"))
	fmt.Printf("[%#10s]\n", []byte("a b"))
	fmt.Printf("[%020s]\n", []byte("a b"))
	fmt.Printf("[%s]\n", []byte("tab\t"))
	fmt.Printf("[%+s]\n", []byte("tab\t"))
	fmt.Printf("[%-s]\n", []byte("tab\t"))
	fmt.Printf("[% s]\n", []byte("tab\t"))
	fmt.Printf("[%#s]\n", []byte("tab\t"))
	fmt.Printf("[%10s]\n", []byte("tab\t"))
	fmt.Printf("[%-10s]\n", []byte("tab\t"))
	fmt.Printf("[%.3s]\n", []byte("tab\t"))
	fmt.Printf("[%10.2s]\n", []byte("tab\t"))
	fmt.Printf("[%# s]\n", []byte("tab\t"))
	fmt.Printf("[%#10s]\n", []byte("tab\t"))
	fmt.Printf("[%020s]\n", []byte("tab\t"))
}

// Output:
// []
// []
// []
// []
// []
// [          ]
// [          ]
// []
// [          ]
// []
// [          ]
// [00000000000000000000]
// []
// []
// []
// []
// []
// [          ]
// [          ]
// []
// [          ]
// []
// [          ]
// [00000000000000000000]
// [hello]
// [hello]
// [hello]
// [hello]
// [hello]
// [     hello]
// [hello     ]
// [hel]
// [        he]
// [hello]
// [     hello]
// [000000000000000hello]
// [日本語]
// [日本語]
// [日本語]
// [日本語]
// [日本語]
// [       日本語]
// [日本語       ]
// [日本語]
// [        日本]
// [日本語]
// [       日本語]
// [00000000000000000日本語]
// [a b]
// [a b]
// [a b]
// [a b]
// [a b]
// [       a b]
// [a b       ]
// [a b]
// [        a ]
// [a b]
// [       a b]
// [00000000000000000a b]
// [tab	]
// [tab	]
// [tab	]
// [tab	]
// [tab	]
// [      tab	]
// [tab	      ]
// [tab]
// [        ta]
// [tab	]
// [      tab	]
// [0000000000000000tab	]
