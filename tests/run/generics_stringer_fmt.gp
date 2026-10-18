// phase: 07

// A type argument with a String or Error method is printed through it by fmt, in generic code too.
package main

import (
	"errors"
	"fmt"
)

type Color int

func (c Color) String() string { return [...]string{"red", "green"}[c] }

type Secret string

func (Secret) String() string { return "***" }

type Pt struct{ X, Y int }

func (p *Pt) String() string { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

type E struct{}

func (E) Error() string { return "E!" }

func Print[T any](v T) {
	fmt.Println(v)
	fmt.Printf("%v %s %d\n", v, v, v)
}

func Sprint[T any](vs ...T) string { return fmt.Sprint(vs) }

func List[T any](vs []T) string {
	out := ""
	for _, v := range vs {
		out += fmt.Sprintf("<%v>", v)
	}
	return out
}

func main() {
	Print(Color(1))
	Print(Secret("pw"))
	Print(&Pt{1, 2})
	Print(Pt{3, 4})
	Print(E{})
	Print(errors.New("e"))
	fmt.Println(Sprint(Color(0), Color(1)), Sprint(Secret("a")), Sprint(&Pt{5, 6}))
	fmt.Println(List([]Color{0, 1}), List([]Secret{"x"}), List([]*Pt{{7, 8}}), List([]error{E{}, nil}), List([]any{Color(1), 2, "s", nil}))
}

// Output:
// green
// green green 1
// ***
// *** *** %!d(main.Secret=pw)
// (1,2)
// (1,2) (1,2) &{1 2}
// {3 4}
// {3 4} {%!s(int=3) %!s(int=4)} {3 4}
// E!
// E! E! {}
// e
// e e &{%!d(string=e)}
// [red green] [***] [(5,6)]
// <red><green> <***> <(7,8)> <E!><<nil>> <green><2><s><<nil>>
