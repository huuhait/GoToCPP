// phase: 05
package main

import "fmt"

type Scalar int

func (s *Scalar) Read() int           { return int(*s) }
func bindScalar(p *Scalar) func() int { return (*p).Read }

func scalarClosure() func() int {
	p := new(Scalar)
	*p = 7
	return bindScalar(p)
}

type Text string

func (s *Text) Read() int         { return len(*s) }
func bindText(p *Text) func() int { return (*p).Read }

func textClosure() func() int {
	p := new(Text)
	*p = "before"
	return bindText(p)
}

type Values []int

func (s *Values) Read() int           { return (*s)[0] }
func bindValues(p *Values) func() int { return (*p).Read }

func valuesClosure() func() int {
	p := new(Values)
	*p = Values{9}
	return bindValues(p)
}

type Table map[int]int

func (s *Table) Read() int          { return (*s)[1] }
func bindTable(p *Table) func() int { return (*p).Read }

func tableClosure() func() int {
	p := new(Table)
	*p = Table{1: 11}
	return bindTable(p)
}

func main() {
	scalar := scalarClosure()
	text := textClosure()
	values := valuesClosure()
	table := tableClosure()
	fmt.Println(scalar(), text(), values(), table())
}

// Output:
// 7 6 9 11
