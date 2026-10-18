// phase: 07

// A constraint with both strings and byte slices: len, indexing, slicing and conversions work on both.
package main

import "fmt"

type Bytes interface{ ~string | ~[]byte }

type Path string
type Data []byte

func Len[T Bytes](s T) int { return len(s) }

func At[T Bytes](s T, i int) byte { return s[i] }

func Str[T Bytes](s T) string { return string(s) }

func Sub[T Bytes](s T, from, to int) T { return s[from:to] }

func Count[T Bytes](s T, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}

func HasPrefix[T Bytes](s, p T) bool {
	return len(s) >= len(p) && string(s[:len(p)]) == string(p)
}

func Upper[T Bytes](s T) []byte {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'a' <= c && c <= 'z' {
			c -= 32
		}
		out[i] = c
	}
	return out
}

func main() {
	fmt.Println(Len("héllo"), Len([]byte{1, 2}), Len(Path("/a/b")), Len(Data("xy")), Len[string](""))
	fmt.Println(At("abc", 1), At([]byte("xyz"), 2), At(Path("/p"), 0))
	fmt.Println(Str([]byte("hi")), Str("s"), Str(Path("/q")), Str(Data{65, 66}))
	fmt.Println(Sub("hello", 1, 3), Sub([]byte("hello"), 1, 3), Sub(Path("/a/b"), 2, 4))
	fmt.Printf("%T %T %T\n", Sub("x", 0, 1), Sub(Path("x"), 0, 1), Sub(Data("x"), 0, 1))
	fmt.Println(Count("banana", 'a'), Count([]byte("banana"), 'n'), Count(Path("//"), '/'))
	fmt.Println(HasPrefix("golang", "go"), HasPrefix([]byte("golang"), []byte("xx")), HasPrefix(Path("/usr/bin"), Path("/usr")))
	fmt.Println(string(Upper("shout")), string(Upper([]byte("quiet"))))
}

// Output:
// 6 2 4 2 0
// 98 122 47
// hi s /q AB
// el [101 108] /b
// string main.Path main.Data
// 3 2 2
// true false true
// SHOUT QUIET
