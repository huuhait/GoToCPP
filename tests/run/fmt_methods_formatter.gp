// phase: 05

// A Format(fmt.State, rune) method takes over every verb: it sees the flags, width and precision
// through the State, and can print with Fprintf, Write and Sprint; Formatter wins over Stringer
// and Error.
package main

import "fmt"

type F struct{ n int }

func (f F) Format(s fmt.State, verb rune) {
	w, wok := s.Width()
	p, pok := s.Precision()
	fmt.Fprintf(s, "F{verb=%c n=%d w=%d/%v p=%d/%v", verb, f.n, w, wok, p, pok)
	for _, c := range "+-# 0" {
		if s.Flag(int(c)) {
			fmt.Fprintf(s, " %c", c)
		}
	}
	fmt.Fprint(s, "}")
}

type G struct{}

func (G) Format(s fmt.State, verb rune) {
	s.Write([]byte("G!"))
	fmt.Fprintln(s, verb)
}

func (G) String() string { return "never" }

type H struct{}

func (H) Format(f fmt.State, c rune) {
	switch c {
	case 'v', 's':
		fmt.Fprint(f, "hv")
	case 'd':
		fmt.Fprint(f, FormatIt(f, c))
	default:
		fmt.Fprintf(f, "%%!%c(H)", c)
	}
}

func FormatIt(f fmt.State, c rune) string { return fmt.FormatString(f, c) }

type PF struct{}

func (*PF) Format(s fmt.State, c rune) { fmt.Fprint(s, "PF") }

func main() {
	for _, f := range []string{"%v", "%+v", "%#v", "%d", "%s", "%x", "%q", "%8.3d", "%-8v", "%+08.2f", "% x", "%#q", "%T", "%p", "%w"} {
		fmt.Printf("%s -> ", f)
		fmt.Printf(f, F{1})
		fmt.Printf("\n")
	}
	fmt.Printf("%v|%d|%s\n", G{}, G{}, G{})
	fmt.Println(F{2}, G{}, H{})
	fmt.Printf("%v %d %x %+10.3d %#-8x|\n", H{}, H{}, H{}, H{}, H{})
	fmt.Printf("%v %v %v\n", &PF{}, PF{}, []*PF{{}})
	fmt.Printf("%v\n", []F{{1}, {2}})
	fmt.Printf("%v\n", map[string]F{"a": {3}})
	fmt.Printf("%v\n", struct{ X F }{F{4}})
	fmt.Printf("%v\n", struct{ x F }{F{5}})
	fmt.Println(fmt.Sprint(F{6}), fmt.Sprintf("%5.1v", F{7}), fmt.Sprintln(F{8}) == "F{verb=v n=8 w=0/false p=0/false}\n")
}

// Output:
// %v -> F{verb=v n=1 w=0/false p=0/false}
// %+v -> F{verb=v n=1 w=0/false p=0/false +}
// %#v -> F{verb=v n=1 w=0/false p=0/false #}
// %d -> F{verb=d n=1 w=0/false p=0/false}
// %s -> F{verb=s n=1 w=0/false p=0/false}
// %x -> F{verb=x n=1 w=0/false p=0/false}
// %q -> F{verb=q n=1 w=0/false p=0/false}
// %8.3d -> F{verb=d n=1 w=8/true p=3/true}
// %-8v -> F{verb=v n=1 w=8/true p=0/false -}
// %+08.2f -> F{verb=f n=1 w=8/true p=2/true + 0}
// % x -> F{verb=x n=1 w=0/false p=0/false  }
// %#q -> F{verb=q n=1 w=0/false p=0/false #}
// %T -> main.F
// %p -> %!p(main.F={1})
// %w -> %!w(main.F={1})
// G!118
// |G!100
// |G!115
//
// F{verb=v n=2 w=0/false p=0/false} G!118
//  hv
// hv %d %!x(H) %+10.3d %!x(H)|
// PF {} [PF]
// [F{verb=v n=1 w=0/false p=0/false} F{verb=v n=2 w=0/false p=0/false}]
// map[a:F{verb=v n=3 w=0/false p=0/false}]
// {F{verb=v n=4 w=0/false p=0/false}}
// {{5}}
// F{verb=v n=6 w=0/false p=0/false} F{verb=v n=7 w=5/true p=1/true} true
