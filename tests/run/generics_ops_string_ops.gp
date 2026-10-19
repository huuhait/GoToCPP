// phase: 07

// Strings as type parameters: concatenation, comparison, indexing, slicing, range, conversion to and from bytes.
package main

import "fmt"

type Str interface{ ~string }

type Path string
type Word string

func (w Word) Shout() string { return string(w) + "!" }

func Join[S Str](sep S, parts ...S) S {
	var out S
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func HasPrefix[S Str](s, p S) bool { return len(s) >= len(p) && s[:len(p)] == p }
func Last[S Str](s S) byte        { return s[len(s)-1] }
func Rev[S Str](s S) S {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return S(r)
}
func Upper[S Str](s S) S {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return S(b)
}
func Count[S Str](s S, c rune) (n int) {
	for _, r := range s {
		if r == c {
			n++
		}
	}
	return
}
func Less[S Str](a, b S) bool { return a < b }
func Pad[S Str](s S, n int) S {
	for len(s) < n {
		s = "." + s
	}
	return s
}

func main() {
	fmt.Println(Join(",", "a", "b", "c"), Join(Path("/"), "usr", "bin"), Join(Word(" ")), Join("-", "solo"))
	fmt.Println(HasPrefix("golang", "go"), HasPrefix(Path("/a"), "/b"), HasPrefix(Word(""), ""), HasPrefix("g", "go"))
	fmt.Println(Last("héllo"), Last(Path("x/y")), Rev("héllo, 世界"), Rev(Word("abc")), Upper("héllo"), Upper(Path("/usr/Bin")))
	fmt.Println(Count("banana", 'a'), Count(Word("日本日"), '日'), Less("a", "b"), Less(Path("b"), Path("a")), Less("", ""))
	fmt.Println(Pad("x", 4), Pad(Word("abc"), 2), Pad(Path(""), 3))
	fmt.Println(Word.Shout(Rev(Word("olleh"))), Rev(Word("abc")).Shout())
	fmt.Printf("%T %T %q %v\n", Rev(Word("a")), Join(Path(""), "a"), Upper(Word("q")), Join("", "a", "b") == "ab")
}

// Output:
// a,b,c usr/bin  solo
// true false true false
// 111 121 界世 ,olléh cba HéLLO /USR/BIN
// 3 2 true false false
// ...x abc ...
// hello! cba!
// main.Word main.Path "Q" true
