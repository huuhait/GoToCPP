// phase: 05

// Go's %v of byte slices: %s %q %x %X treat them as strings, %v and %d as lists of numbers.
package main

import "fmt"

func main() {
	fmt.Printf("[%v]\n", []byte(nil))
	fmt.Printf("[%+v]\n", []byte(nil))
	fmt.Printf("[%-v]\n", []byte(nil))
	fmt.Printf("[% v]\n", []byte(nil))
	fmt.Printf("[%#v]\n", []byte(nil))
	fmt.Printf("[%10v]\n", []byte(nil))
	fmt.Printf("[%-10v]\n", []byte(nil))
	fmt.Printf("[%.3v]\n", []byte(nil))
	fmt.Printf("[%10.2v]\n", []byte(nil))
	fmt.Printf("[%# v]\n", []byte(nil))
	fmt.Printf("[%#10v]\n", []byte(nil))
	fmt.Printf("[%020v]\n", []byte(nil))
	fmt.Printf("[%v]\n", []byte{})
	fmt.Printf("[%+v]\n", []byte{})
	fmt.Printf("[%-v]\n", []byte{})
	fmt.Printf("[% v]\n", []byte{})
	fmt.Printf("[%#v]\n", []byte{})
	fmt.Printf("[%10v]\n", []byte{})
	fmt.Printf("[%-10v]\n", []byte{})
	fmt.Printf("[%.3v]\n", []byte{})
	fmt.Printf("[%10.2v]\n", []byte{})
	fmt.Printf("[%# v]\n", []byte{})
	fmt.Printf("[%#10v]\n", []byte{})
	fmt.Printf("[%020v]\n", []byte{})
	fmt.Printf("[%v]\n", []byte("hello"))
	fmt.Printf("[%+v]\n", []byte("hello"))
	fmt.Printf("[%-v]\n", []byte("hello"))
	fmt.Printf("[% v]\n", []byte("hello"))
	fmt.Printf("[%#v]\n", []byte("hello"))
	fmt.Printf("[%10v]\n", []byte("hello"))
	fmt.Printf("[%-10v]\n", []byte("hello"))
	fmt.Printf("[%.3v]\n", []byte("hello"))
	fmt.Printf("[%10.2v]\n", []byte("hello"))
	fmt.Printf("[%# v]\n", []byte("hello"))
	fmt.Printf("[%#10v]\n", []byte("hello"))
	fmt.Printf("[%020v]\n", []byte("hello"))
	fmt.Printf("[%v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%+v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[% v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-10v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%.3v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10.2v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%# v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#10v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%020v]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%v]\n", []byte("日本語"))
	fmt.Printf("[%+v]\n", []byte("日本語"))
	fmt.Printf("[%-v]\n", []byte("日本語"))
	fmt.Printf("[% v]\n", []byte("日本語"))
	fmt.Printf("[%#v]\n", []byte("日本語"))
	fmt.Printf("[%10v]\n", []byte("日本語"))
	fmt.Printf("[%-10v]\n", []byte("日本語"))
	fmt.Printf("[%.3v]\n", []byte("日本語"))
	fmt.Printf("[%10.2v]\n", []byte("日本語"))
	fmt.Printf("[%# v]\n", []byte("日本語"))
	fmt.Printf("[%#10v]\n", []byte("日本語"))
	fmt.Printf("[%020v]\n", []byte("日本語"))
	fmt.Printf("[%v]\n", []byte("a b"))
	fmt.Printf("[%+v]\n", []byte("a b"))
	fmt.Printf("[%-v]\n", []byte("a b"))
	fmt.Printf("[% v]\n", []byte("a b"))
	fmt.Printf("[%#v]\n", []byte("a b"))
	fmt.Printf("[%10v]\n", []byte("a b"))
	fmt.Printf("[%-10v]\n", []byte("a b"))
	fmt.Printf("[%.3v]\n", []byte("a b"))
	fmt.Printf("[%10.2v]\n", []byte("a b"))
	fmt.Printf("[%# v]\n", []byte("a b"))
	fmt.Printf("[%#10v]\n", []byte("a b"))
	fmt.Printf("[%020v]\n", []byte("a b"))
	fmt.Printf("[%v]\n", []byte("tab\t"))
	fmt.Printf("[%+v]\n", []byte("tab\t"))
	fmt.Printf("[%-v]\n", []byte("tab\t"))
	fmt.Printf("[% v]\n", []byte("tab\t"))
	fmt.Printf("[%#v]\n", []byte("tab\t"))
	fmt.Printf("[%10v]\n", []byte("tab\t"))
	fmt.Printf("[%-10v]\n", []byte("tab\t"))
	fmt.Printf("[%.3v]\n", []byte("tab\t"))
	fmt.Printf("[%10.2v]\n", []byte("tab\t"))
	fmt.Printf("[%# v]\n", []byte("tab\t"))
	fmt.Printf("[%#10v]\n", []byte("tab\t"))
	fmt.Printf("[%020v]\n", []byte("tab\t"))
}

// Output:
// [[]]
// [[]]
// [[]]
// [[]]
// [[]byte(nil)]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]byte(nil)]
// [[]byte(nil)]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]byte{}]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]byte{}]
// [[]byte{}]
// [[]]
// [[104 101 108 108 111]]
// [[104 101 108 108 111]]
// [[104 101 108 108 111]]
// [[ 104  101  108  108  111]]
// [[]byte{0x68, 0x65, 0x6c, 0x6c, 0x6f}]
// [[       104        101        108        108        111]]
// [[104        101        108        108        111       ]]
// [[104 101 108 108 111]]
// [[       104        101        108        108        111]]
// [[]byte{ 0x68,  0x65,  0x6c,  0x6c,  0x6f}]
// [[]byte{      0x68,       0x65,       0x6c,       0x6c,       0x6f}]
// [[00000000000000000104 00000000000000000101 00000000000000000108 00000000000000000108 00000000000000000111]]
// [[0 1 2 254 255]]
// [[0 1 2 254 255]]
// [[0 1 2 254 255]]
// [[ 0  1  2  254  255]]
// [[]byte{0x0, 0x1, 0x2, 0xfe, 0xff}]
// [[         0          1          2        254        255]]
// [[0          1          2          254        255       ]]
// [[000 001 002 254 255]]
// [[        00         01         02        254        255]]
// [[]byte{ 0x0,  0x1,  0x2,  0xfe,  0xff}]
// [[]byte{       0x0,        0x1,        0x2,       0xfe,       0xff}]
// [[00000000000000000000 00000000000000000001 00000000000000000002 00000000000000000254 00000000000000000255]]
// [[230 151 165 230 156 172 232 170 158]]
// [[230 151 165 230 156 172 232 170 158]]
// [[230 151 165 230 156 172 232 170 158]]
// [[ 230  151  165  230  156  172  232  170  158]]
// [[]byte{0xe6, 0x97, 0xa5, 0xe6, 0x9c, 0xac, 0xe8, 0xaa, 0x9e}]
// [[       230        151        165        230        156        172        232        170        158]]
// [[230        151        165        230        156        172        232        170        158       ]]
// [[230 151 165 230 156 172 232 170 158]]
// [[       230        151        165        230        156        172        232        170        158]]
// [[]byte{ 0xe6,  0x97,  0xa5,  0xe6,  0x9c,  0xac,  0xe8,  0xaa,  0x9e}]
// [[]byte{      0xe6,       0x97,       0xa5,       0xe6,       0x9c,       0xac,       0xe8,       0xaa,       0x9e}]
// [[00000000000000000230 00000000000000000151 00000000000000000165 00000000000000000230 00000000000000000156 00000000000000000172 00000000000000000232 00000000000000000170 00000000000000000158]]
// [[97 32 98]]
// [[97 32 98]]
// [[97 32 98]]
// [[ 97  32  98]]
// [[]byte{0x61, 0x20, 0x62}]
// [[        97         32         98]]
// [[97         32         98        ]]
// [[097 032 098]]
// [[        97         32         98]]
// [[]byte{ 0x61,  0x20,  0x62}]
// [[]byte{      0x61,       0x20,       0x62}]
// [[00000000000000000097 00000000000000000032 00000000000000000098]]
// [[116 97 98 9]]
// [[116 97 98 9]]
// [[116 97 98 9]]
// [[ 116  97  98  9]]
// [[]byte{0x74, 0x61, 0x62, 0x9}]
// [[       116         97         98          9]]
// [[116        97         98         9         ]]
// [[116 097 098 009]]
// [[       116         97         98         09]]
// [[]byte{ 0x74,  0x61,  0x62,  0x9}]
// [[]byte{      0x74,       0x61,       0x62,        0x9}]
// [[00000000000000000116 00000000000000000097 00000000000000000098 00000000000000000009]]
