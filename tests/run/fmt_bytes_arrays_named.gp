// phase: 05

// Byte arrays and named byte types: %s %x %q %v work on them as on slices; %T and %#v name the
// types.
package main

import "fmt"

type B []byte

type Hash [4]byte

type Octet byte

func main() {
	h := Hash{1, 2, 255, 16}
	fmt.Printf("%v %x %X %s|%q %d\n", h, h, h, Hash{72, 105, 33, 33}, Hash{72, 105, 0, 1}, h)
	fmt.Printf("%v %s %x %q\n", B("abc"), B("abc"), B("abc"), B("abc"))
	fmt.Printf("%v %c %d %q\n", Octet(65), Octet(65), Octet(65), Octet(65))
	fmt.Printf("%v %s\n", []Octet{72, 105}, []Octet{72, 105})
	fmt.Printf("%T %T %T %T\n", []byte(nil), []uint8{}, h, B(nil))
	fmt.Printf("%#v %#v %#v %#v\n", []byte("ab"), []uint8(nil), h, B("x"))
	fmt.Printf("%#v %#v\n", [2][]byte{[]byte("a"), nil}, struct{ P []byte }{[]byte("x")})
	var a any = []byte("xyz")
	fmt.Printf("%s %v %x %T\n", a, a, a, a)
	fmt.Println([]byte("ab"), [2]byte{1, 2}, [][]byte{[]byte("a"), []byte("b")})
	fmt.Print([]byte("ab"), "\n")
}

// Output:
// [1 2 255 16] 0102ff10 0102FF10 Hi!!|"Hi\x00\x01" [1 2 255 16]
// [97 98 99] abc 616263 "abc"
// 65 A 65 'A'
// [72 105] Hi
// []uint8 []uint8 main.Hash main.B
// []byte{0x61, 0x62} []byte(nil) main.Hash{0x1, 0x2, 0xff, 0x10} main.B{0x78}
// [2][]uint8{[]uint8{0x61}, []uint8(nil)} struct { P []uint8 }{P:[]uint8{0x78}}
// xyz [120 121 122] 78797a []uint8
// [97 98] [1 2] [[97] [98]]
// [97 98]
