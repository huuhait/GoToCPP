// phase: 03

// Audit case: runtime types have no plain-named members (Phase 03 README §2.1), so Go methods
// and fields of any name work on types built on them: named slices, maps, strings, pointers and
// funcs with methods called get, len, size, data, reset, swap, begin, end, push_back, owner and
// value, and struct fields of those names.
package main

import "fmt"

type Stack []int

func (s *Stack) push_back(v int) { *s = append(*s, v) }
func (s Stack) len() int         { return len(s) }
func (s Stack) data() []int      { return s }
func (s Stack) begin() int       { return s[0] }
func (s Stack) end() int         { return s[len(s)-1] }
func (s *Stack) reset()          { *s = nil }
func (s Stack) size() int        { return cap(s) - cap(s) + len(s) }
func (s Stack) swap(i, j int)    { s[i], s[j] = s[j], s[i] }

type Dict map[string]int

func (d Dict) get(k string) int   { return d[k] }
func (d Dict) value() int         { return len(d) }
func (d Dict) erase(k string)     { delete(d, k) }
func (d Dict) find(k string) bool { _, ok := d[k]; return ok }

type Name string

func (n Name) data() string { return string(n) + "!" }
func (n Name) size() int    { return len(n) * 2 }
func (n Name) view() Name   { return n[1:] }

type Fn func() int

func (f Fn) get() int  { return f() + 1 }
func (f Fn) call() int { return f() * 10 }

type Holder struct {
	get, len, size, data, owner, reset, value int
	begin, end                                string
}

func (h *Holder) swap() { h.begin, h.end = h.end, h.begin }

func main() {
	var s Stack
	for i := 1; i <= 4; i++ {
		s.push_back(i * i)
	}
	s.swap(0, 3)
	fmt.Println(s.len(), s.data(), s.begin(), s.end(), s.size())
	s.reset()
	fmt.Println(s.len(), s == nil)

	d := Dict{"a": 1, "b": 2}
	d.erase("a")
	fmt.Println(d.get("b"), d.value(), d.find("a"), d.find("b"))

	n := Name("gocpp")
	fmt.Println(n.data(), n.size(), n.view())

	f := Fn(func() int { return 4 })
	fmt.Println(f.get(), f.call())

	h := &Holder{get: 1, len: 2, size: 3, data: 4, owner: 5, reset: 6, value: 7, begin: "b", end: "e"}
	h.swap()
	fmt.Printf("%+v\n", *h)
}

// Output:
// 4 [16 4 9 1] 16 1 4
// 0 true
// 2 1 false true
// gocpp! 10 ocpp
// 5 40
// {get:1 len:2 size:3 data:4 owner:5 reset:6 value:7 begin:e end:b}
