// phase: 07

// append and copy with a type parameter constrained by ~string | ~[]byte: the call's signature
// mentions the type argument, which may be a defined type (Raw, Text) in the instance.
package main

import "fmt"

type Raw []byte
type Text string
type Name string

func Bytes[T ~string | ~[]byte](t T) []byte {
	return append([]byte(nil), t...)
}

func Append[T ~string | ~[]byte](dst []byte, t T) []byte {
	return append(dst, t...)
}

func Copy[T ~string | ~[]byte](dst []byte, t T) int {
	return copy(dst, t)
}

func Join[S ~[]byte, T ~string | ~[]byte](dst S, ts ...T) S {
	for _, t := range ts {
		dst = append(dst, t...)
		dst = append(dst, '|')
	}
	return dst
}

func main() {
	fmt.Println(Bytes("abc"), Bytes(Text("é")), Bytes(Raw{1, 2}), Bytes([]byte{3}), Bytes(Name("")))
	fmt.Println(string(Append([]byte("x="), Text("yz"))), string(Append(nil, Raw("raw"))), string(Append([]byte{}, "plain")))
	buf := make([]byte, 4)
	fmt.Println(Copy(buf, Text("hello")), string(buf), Copy(buf, Raw("ab")), string(buf), Copy(buf, []byte("xyzzy!")), string(buf))
	fmt.Printf("%q %q\n", Join(Raw("<"), "a", "b"), Join([]byte(nil), Text("t"), "u"))
	fmt.Printf("%T %T\n", Join(Raw(nil), Raw("z")), Join([]byte(nil), Name("n")))
}

// Output:
// [97 98 99] [195 169] [1 2] [3] []
// x=yz raw plain
// 4 xyzz 2 xyzz 4 xyzz
// "<a|b|" "t|u|"
// main.Raw []uint8
