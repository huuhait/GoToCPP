// phase: 05

// %w outside Errorf is a bad verb. Go's type switch prints an unnamed []byte through fmtBytes,
// whose default case prints the elements, so each one gets the bad verb; every other type gets it
// as a whole.
package main

import (
	"errors"
	"fmt"
)

type Bytes []byte
type Arr [2]byte

var anyArrays = []any{[2]bool{true}, [0]int{}, [3]int{1, 2, 3}}
var anyArray any = [2]string{"a", "b"}
var anyStruct any = struct{ A [2]int }{[2]int{4, 5}}

func main() {
	fmt.Printf("%w|%.1w|%#w|%5w|%+w\n", []byte("ab"), []byte("ab"), []byte("ab"), []byte("ab"), []byte("ab"))
	fmt.Printf("%w|%#w|%w|%#w\n", []byte(nil), []byte(nil), []byte{}, []byte{})
	fmt.Printf("%w|%#w|%w\n", Bytes("ab"), Bytes("ab"), Arr{1, 2})
	fmt.Printf("%w|%w|%w|%w\n", 5, "str", 2.5, []string{"a", "b"})
	fmt.Printf("%w|%w\n", errors.New("e"), struct{ A int }{1})
	fmt.Println(fmt.Errorf("%w", []byte("x")))
	fmt.Println(anyArrays, anyArray, anyStruct)
}

// Output:
// [%!w(uint8=97) %!w(uint8=98)]|[%!w(uint8=97) %!w(uint8=98)]|[]uint8{%!w(uint8=0x61), %!w(uint8=0x62)}|[%!w(uint8=   97) %!w(uint8=   98)]|[%!w(uint8=97) %!w(uint8=98)]
// []|[]uint8(nil)|[]|[]uint8{}
// %!w(main.Bytes=[97 98])|%!w(main.Bytes=main.Bytes{0x61, 0x62})|%!w(main.Arr=[1 2])
// %!w(int=5)|%!w(string=str)|%!w(float64=2.5)|%!w([]string=[a b])
// %!w(*errors.errorString=&{e})|%!w(struct { A int }={1})
// [%!w(uint8=120)]
// [[true false] [] [1 2 3]] [a b] {[4 5]}
