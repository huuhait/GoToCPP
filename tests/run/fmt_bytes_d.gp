// phase: 05

// Go's %d of byte slices: %s %q %x %X treat them as strings, %v and %d as lists of numbers.
package main

import "fmt"

func main() {
	fmt.Printf("[%d]\n", []byte(nil))
	fmt.Printf("[%+d]\n", []byte(nil))
	fmt.Printf("[%-d]\n", []byte(nil))
	fmt.Printf("[% d]\n", []byte(nil))
	fmt.Printf("[%#d]\n", []byte(nil))
	fmt.Printf("[%10d]\n", []byte(nil))
	fmt.Printf("[%-10d]\n", []byte(nil))
	fmt.Printf("[%.3d]\n", []byte(nil))
	fmt.Printf("[%10.2d]\n", []byte(nil))
	fmt.Printf("[%# d]\n", []byte(nil))
	fmt.Printf("[%#10d]\n", []byte(nil))
	fmt.Printf("[%020d]\n", []byte(nil))
	fmt.Printf("[%d]\n", []byte{})
	fmt.Printf("[%+d]\n", []byte{})
	fmt.Printf("[%-d]\n", []byte{})
	fmt.Printf("[% d]\n", []byte{})
	fmt.Printf("[%#d]\n", []byte{})
	fmt.Printf("[%10d]\n", []byte{})
	fmt.Printf("[%-10d]\n", []byte{})
	fmt.Printf("[%.3d]\n", []byte{})
	fmt.Printf("[%10.2d]\n", []byte{})
	fmt.Printf("[%# d]\n", []byte{})
	fmt.Printf("[%#10d]\n", []byte{})
	fmt.Printf("[%020d]\n", []byte{})
	fmt.Printf("[%d]\n", []byte("hello"))
	fmt.Printf("[%+d]\n", []byte("hello"))
	fmt.Printf("[%-d]\n", []byte("hello"))
	fmt.Printf("[% d]\n", []byte("hello"))
	fmt.Printf("[%#d]\n", []byte("hello"))
	fmt.Printf("[%10d]\n", []byte("hello"))
	fmt.Printf("[%-10d]\n", []byte("hello"))
	fmt.Printf("[%.3d]\n", []byte("hello"))
	fmt.Printf("[%10.2d]\n", []byte("hello"))
	fmt.Printf("[%# d]\n", []byte("hello"))
	fmt.Printf("[%#10d]\n", []byte("hello"))
	fmt.Printf("[%020d]\n", []byte("hello"))
	fmt.Printf("[%d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%+d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[% d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%-10d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%.3d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%10.2d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%# d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%#10d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%020d]\n", []byte{0, 1, 2, 254, 255})
	fmt.Printf("[%d]\n", []byte("日本語"))
	fmt.Printf("[%+d]\n", []byte("日本語"))
	fmt.Printf("[%-d]\n", []byte("日本語"))
	fmt.Printf("[% d]\n", []byte("日本語"))
	fmt.Printf("[%#d]\n", []byte("日本語"))
	fmt.Printf("[%10d]\n", []byte("日本語"))
	fmt.Printf("[%-10d]\n", []byte("日本語"))
	fmt.Printf("[%.3d]\n", []byte("日本語"))
	fmt.Printf("[%10.2d]\n", []byte("日本語"))
	fmt.Printf("[%# d]\n", []byte("日本語"))
	fmt.Printf("[%#10d]\n", []byte("日本語"))
	fmt.Printf("[%020d]\n", []byte("日本語"))
	fmt.Printf("[%d]\n", []byte("a b"))
	fmt.Printf("[%+d]\n", []byte("a b"))
	fmt.Printf("[%-d]\n", []byte("a b"))
	fmt.Printf("[% d]\n", []byte("a b"))
	fmt.Printf("[%#d]\n", []byte("a b"))
	fmt.Printf("[%10d]\n", []byte("a b"))
	fmt.Printf("[%-10d]\n", []byte("a b"))
	fmt.Printf("[%.3d]\n", []byte("a b"))
	fmt.Printf("[%10.2d]\n", []byte("a b"))
	fmt.Printf("[%# d]\n", []byte("a b"))
	fmt.Printf("[%#10d]\n", []byte("a b"))
	fmt.Printf("[%020d]\n", []byte("a b"))
	fmt.Printf("[%d]\n", []byte("tab\t"))
	fmt.Printf("[%+d]\n", []byte("tab\t"))
	fmt.Printf("[%-d]\n", []byte("tab\t"))
	fmt.Printf("[% d]\n", []byte("tab\t"))
	fmt.Printf("[%#d]\n", []byte("tab\t"))
	fmt.Printf("[%10d]\n", []byte("tab\t"))
	fmt.Printf("[%-10d]\n", []byte("tab\t"))
	fmt.Printf("[%.3d]\n", []byte("tab\t"))
	fmt.Printf("[%10.2d]\n", []byte("tab\t"))
	fmt.Printf("[%# d]\n", []byte("tab\t"))
	fmt.Printf("[%#10d]\n", []byte("tab\t"))
	fmt.Printf("[%020d]\n", []byte("tab\t"))
}

// Output:
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[]]
// [[104 101 108 108 111]]
// [[+104 +101 +108 +108 +111]]
// [[104 101 108 108 111]]
// [[ 104  101  108  108  111]]
// [[104 101 108 108 111]]
// [[       104        101        108        108        111]]
// [[104        101        108        108        111       ]]
// [[104 101 108 108 111]]
// [[       104        101        108        108        111]]
// [[ 104  101  108  108  111]]
// [[       104        101        108        108        111]]
// [[00000000000000000104 00000000000000000101 00000000000000000108 00000000000000000108 00000000000000000111]]
// [[0 1 2 254 255]]
// [[+0 +1 +2 +254 +255]]
// [[0 1 2 254 255]]
// [[ 0  1  2  254  255]]
// [[0 1 2 254 255]]
// [[         0          1          2        254        255]]
// [[0          1          2          254        255       ]]
// [[000 001 002 254 255]]
// [[        00         01         02        254        255]]
// [[ 0  1  2  254  255]]
// [[         0          1          2        254        255]]
// [[00000000000000000000 00000000000000000001 00000000000000000002 00000000000000000254 00000000000000000255]]
// [[230 151 165 230 156 172 232 170 158]]
// [[+230 +151 +165 +230 +156 +172 +232 +170 +158]]
// [[230 151 165 230 156 172 232 170 158]]
// [[ 230  151  165  230  156  172  232  170  158]]
// [[230 151 165 230 156 172 232 170 158]]
// [[       230        151        165        230        156        172        232        170        158]]
// [[230        151        165        230        156        172        232        170        158       ]]
// [[230 151 165 230 156 172 232 170 158]]
// [[       230        151        165        230        156        172        232        170        158]]
// [[ 230  151  165  230  156  172  232  170  158]]
// [[       230        151        165        230        156        172        232        170        158]]
// [[00000000000000000230 00000000000000000151 00000000000000000165 00000000000000000230 00000000000000000156 00000000000000000172 00000000000000000232 00000000000000000170 00000000000000000158]]
// [[97 32 98]]
// [[+97 +32 +98]]
// [[97 32 98]]
// [[ 97  32  98]]
// [[97 32 98]]
// [[        97         32         98]]
// [[97         32         98        ]]
// [[097 032 098]]
// [[        97         32         98]]
// [[ 97  32  98]]
// [[        97         32         98]]
// [[00000000000000000097 00000000000000000032 00000000000000000098]]
// [[116 97 98 9]]
// [[+116 +97 +98 +9]]
// [[116 97 98 9]]
// [[ 116  97  98  9]]
// [[116 97 98 9]]
// [[       116         97         98          9]]
// [[116        97         98         9         ]]
// [[116 097 098 009]]
// [[       116         97         98         09]]
// [[ 116  97  98  9]]
// [[       116         97         98          9]]
// [[00000000000000000116 00000000000000000097 00000000000000000098 00000000000000000009]]
