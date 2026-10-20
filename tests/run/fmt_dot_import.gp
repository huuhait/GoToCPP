// phase: 09

// A dot import of fmt: Sprintf(...), Fprintf(&buf, ...), Errorf, FormatString of a value that
// implements State (the argument converts to the interface), and the functions as func values.
package main

import (
	"bytes"
	. "fmt"
	"os"
)

type st struct {
	wid  int
	flag string
}

func (s st) Write(b []byte) (int, error) { return len(b), nil }
func (s st) Width() (int, bool)          { return s.wid, s.wid > 0 }
func (s st) Precision() (int, bool)      { return 0, false }
func (s st) Flag(c int) bool {
	for _, f := range s.flag {
		if int(f) == c {
			return true
		}
	}
	return false
}

type Pt struct{ X, Y int }

func (p Pt) String() string  { return Sprintf("(%d,%d)", p.X, p.Y) }
func (p Pt) Format(f State, c rune) {
	Fprintf(f, "<%s>", FormatString(f, c))
}

func main() {
	var buf bytes.Buffer
	Fprint(&buf, 123456, " ", "x")
	Fprintf(&buf, "|%d|%5.1f|%v", 7, 2.25, []string{"a"})
	Fprintln(&buf, "!")
	Print(buf.String())
	err := Errorf("wrap: %w", os.ErrNotExist)
	Println(err, Sprint(1, 2, "a", "b"), Sprintln("x", 3))
	Println(FormatString(st{wid: 8, flag: "-+"}, 'd'), FormatString(st{}, 's'))
	Printf("%v %+8.3q %-#x\n", Pt{1, 2}, Pt{3, 4}, Pt{5, 6})
	f := Sprintf
	g := Println
	h := Errorf
	w := Fprintf
	a := Append
	g(f("%03d", 5), h("e%d", 1), len(a(nil, 1, 2)))
	w(os.Stdout, "%s\n", "via value")
	b := Appendf([]byte("x="), "%d", 1)
	b = Appendln(b, "end")
	Print(string(b))
}

// Output:
// 123456 x|7|  2.2|[a]!
// wrap: file does not exist 1 2ab x 3
//
// %+-8d %s
// <%v> <%+8.3q> <%-#x>
// 005 e1 3
// via value
// x=1end
