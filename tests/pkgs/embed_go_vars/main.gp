// phase: 10

// //go:embed on variables of type string and []byte (Phase 10 step 10.8): text, empty,
// binary and unicode files, patterns with wildcards and quotes, groups of variables, defined types, a
// package below the module root, and the contents in package-level initializers.

package main

import (
	_ "embed"
	"fmt"

	"example.com/embedvars/res"
)

//go:embed hello.txt
var hello string

//go:embed hello.txt
var helloBytes []byte

//go:embed empty.txt
var empty string

//go:embed empty.txt
var emptyBytes []byte

//go:embed tricky.bin
var tricky string

//go:embed tricky.bin
var trickyBytes []byte

//go:embed unicode.txt
var uni string

//go:embed assets/a.txt
var sub string

//go:embed assets/deep/*.txt
var deep string

//go:embed assets/noise.bin
var noise []byte

//go:embed "hel*.txt"
var quoted string

//go:embed hello.txt hel*.txt
//go:embed hello.*
var many string

type Name string
type Blob []byte

//go:embed hello.txt
var named Name

//go:embed unicode.txt
var blob Blob

var (
	//go:embed hello.txt
	grouped1 string
	plain    = 3
	//go:embed assets/a.txt
	grouped2 []byte
)

// A package-level initializer sees the embedded value.
var length = len(hello) + len(grouped2)

func sum(b []byte) (s uint32) {
	for _, c := range b {
		s = s*31 + uint32(c)
	}
	return s
}

func main() {
	fmt.Printf("%q\n", hello)
	fmt.Println(len(helloBytes), string(helloBytes) == hello)
	fmt.Println(len(empty), len(emptyBytes), empty == "", emptyBytes == nil)
	fmt.Printf("%q %d %v\n", tricky, len(tricky), string(trickyBytes) == tricky)
	fmt.Printf("%x\n", trickyBytes)
	fmt.Printf("%q %d\n", uni, len(uni))
	fmt.Printf("%q %q\n", sub, deep)
	fmt.Println(len(noise), sum(noise))
	fmt.Println(quoted == hello, many == hello, named, len(blob))
	fmt.Println(grouped1 == hello, string(grouped2) == sub, plain, length)
	fmt.Printf("%q %q\n", res.X, res.XB)
	b := helloBytes
	b[0] = 'J' // each []byte is its own copy
	fmt.Println(string(helloBytes[:5]), hello[:5])
}

// Output:
// "hello, embed\nsecond line\n"
// 25 true
// 0 0 true false
// "\x00\x01\x02a\x01f\x7f\xff\xfe\r\n??/\\n\"quote\"\x00" 24 true
// 0001026101667ffffe0d0a3f3f2f5c6e2271756f74652200
// "héllo 世界 😀\n" 19
// "in assets\n" "deeper\n"
// 70000 2383028053
// true true hello, embed
// second line
//  19
// true true 3 35
// "in res\n" "in res\n"
// Jello hello
