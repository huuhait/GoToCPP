// phase: 05

// Embedded structs and pointers: the field name is the type name (%+v); promoted fields; embedded
// Stringers take over the whole struct.
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
type Base struct{ ID int }

type Named struct {
	Base
	Name string
}

type Deep struct {
	*Named
	Level int
}

type Inner struct{ V int }

type outer struct {
	Inner
	W int
}

type Pub struct{ A int }

type holder struct {
	Pub
	x int
}

type Shown struct{ N int }

func (s Shown) String() string { return "shown" }

type Wrap struct {
	Shown
	Extra int
}

type Ptr struct{ N int }

func (p *Ptr) String() string { return "ptr-string" }

type WrapPtr struct {
	*Ptr
	Z int
}

func main() {
	n := Named{Base{1}, "ann"}
	fmt.Printf("%v %+v %#v\n", n, n, n)
	d := Deep{&n, 3}
	fmt.Printf("%v\n", norm(fmt.Sprintf("%v", d)))
	fmt.Printf("%+v\n", norm(fmt.Sprintf("%+v", d)))
	fmt.Println(n.ID, n.Base.ID, d.Name, d.ID)
	fmt.Printf("%v %+v %#v\n", outer{Inner{1}, 2}, outer{Inner{1}, 2}, outer{Inner{1}, 2})
	fmt.Printf("%v %+v %#v\n", holder{Pub{1}, 2}, holder{Pub{1}, 2}, holder{Pub{1}, 2})
	w := Wrap{Shown{1}, 2}
	fmt.Printf("%v %+v %#v %s %d\n", w, w, w, w, w)
	wp := WrapPtr{&Ptr{1}, 2}
	fmt.Printf("%v %+v %s\n", wp, wp, wp)
	fmt.Printf("%v %s\n", &w, &w)
}

// Output:
// {{1} ann} {Base:{ID:1} Name:ann} main.Named{Base:main.Base{ID:1}, Name:"ann"}
// {0xP 3}
// {Named:0xP Level:3}
// 1 1 ann 1
// {{1} 2} {Inner:{V:1} W:2} main.outer{Inner:main.Inner{V:1}, W:2}
// {{1} 2} {Pub:{A:1} x:2} main.holder{Pub:main.Pub{A:1}, x:2}
// shown shown main.Wrap{Shown:main.Shown{N:1}, Extra:2} shown {{1} 2}
// ptr-string ptr-string ptr-string
// shown shown
