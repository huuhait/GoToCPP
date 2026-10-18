// phase: 05

// GoString is used for %#v only (and its result is printed as it is); Stringer is ignored by %#v;
// a type with both; inside composite values.
package main

import "fmt"

// norm replaces every hexadecimal address (0x...) with 0xP: addresses differ from run to run.
func norm(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '0' && i+1 < len(s) && s[i+1] == 'x' {
			j := i + 2
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'f') {
				j++
			}
			if j > i+2 {
				out = append(out, "0xP"...)
				i = j - 1
				continue
			}
		}
		out = append(out, s[i])
	}
	return string(out)
}
type G struct{ n int }

func (g G) GoString() string { return fmt.Sprintf("G(%d)", g.n) }

type GS struct{ n int }

func (g GS) GoString() string { return "GS!" }
func (g GS) String() string   { return "str" }

type PG struct{}

func (*PG) GoString() string { return "PG!" }

type Plain struct{ A int }

type Holder struct {
	G  G
	g  G
	GS GS
	P  *PG
}

func main() {
	fmt.Printf("%#v %v %+v %s\n", G{1}, G{1}, G{1}, G{1})
	fmt.Printf("%#v %v %s %#v\n", GS{1}, GS{1}, GS{1}, &GS{2})
	fmt.Printf("%#v %#v %v\n", &PG{}, PG{}, PG{})
	fmt.Printf("%#v\n", []G{{1}, {2}})
	fmt.Printf("%#v\n", map[string]G{"k": {3}})
	fmt.Printf("%#v\n", [2]GS{})
	fmt.Println(norm(fmt.Sprintf("%#v", Holder{G{1}, G{2}, GS{3}, nil})))
	fmt.Printf("%#v %#v %#v\n", Plain{1}, &Plain{2}, []Plain{{3}})
	fmt.Printf("%10v|%-10v|%#10v|%x\n", G{1}, G{1}, G{1}, G{1})
	var a any = G{9}
	fmt.Printf("%#v %v\n", a, a)
}

// Output:
// G(1) {1} {n:1} {%!s(int=1)}
// GS! str str GS!
// PG! main.PG{} {}
// []main.G{G(1), G(2)}
// map[string]main.G{"k":G(3)}
// [2]main.GS{GS!, GS!}
// main.Holder{G:G(1), g:main.G{n:2}, GS:GS!, P:PG!}
// main.Plain{A:1} &main.Plain{A:2} []main.Plain{main.Plain{A:3}}
// {         1}|{1         }|      G(1)|{1}
// G(9) {9}
