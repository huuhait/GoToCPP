// phase: 05

// Promoted methods named like C++ words, macros of the headers the runtime includes, runtime names and
// types (typename, and, NULL, errno, gp, std, string, int64_t, major, ...): the forwarders are escaped the
// way declared methods are.
package main

import "fmt"

type Item struct{ id int }
type Pair struct{ a, b int }
type Base struct{ n int }

func (b Base) Item() Item          { return Item{b.n} }
func (b *Base) SetItem(i Item)     { b.n = i.id }
func (b Base) errno() int          { return 1 }
func (b Base) assert() int         { return 2 }
func (b Base) NULL() int           { return 3 }
func (b Base) stdin() int          { return 4 }
func (b Base) max(x, y int) int    { if x > y { return x }; return y }
func (b Base) abs() int            { return 5 }
func (b Base) main() int           { return 6 }
func (b Base) typename() int       { return 7 }
func (b Base) and() int            { return 8 }
func (b Base) not() int            { return 9 }
func (b Base) xor() int            { return 10 }
func (b Base) default_() int       { return 11 }
func (b Base) explicit() int       { return 12 }
func (b Base) virtual() int        { return 13 }
func (b Base) gp() int             { return 14 }
func (b Base) std() int            { return 15 }
func (b Base) string() int         { return 16 }
func (b Base) Pair() Pair          { return Pair{1, 2} }
func (b Base) int64_t() int        { return 17 }
func (b Base) size_t() int         { return 18 }
func (b Base) this() int           { return 19 }
func (b Base) sizeof() int         { return 20 }
func (b Base) unix() int           { return 21 }
func (b Base) linux() int          { return 22 }
func (b Base) major() int          { return 23 }
func (b Base) EOF() int            { return 24 }
func (b Base) stderr() int         { return 25 }
func (b Base) putchar() int         { return 26 }
func (b Base) Self() Base          { return b }
func (b *Base) PSelf() *Base       { return b }

type Outer struct {
	Base
	Z int
}

func main() {
	o := Outer{Base{3}, 1}
	fmt.Println(o.Item(), o.errno(), o.assert(), o.NULL(), o.stdin(), o.max(3, 4), o.abs(), o.main(), o.typename(), o.and(), o.not(), o.xor())
	fmt.Println(o.default_(), o.explicit(), o.virtual(), o.gp(), o.std(), o.string(), o.Pair(), o.int64_t(), o.size_t(), o.this(), o.sizeof(), o.unix(), o.linux(), o.major(), o.EOF(), o.stderr(), o.putchar())
	o.SetItem(Item{9})
	fmt.Println(o.Self(), o.PSelf() == &o.Base)
}

// Output:
// {3} 1 2 3 4 4 5 6 7 8 9 10
// 11 12 13 14 15 16 {1 2} 17 18 19 20 21 22 23 24 25 26
// {9} true
