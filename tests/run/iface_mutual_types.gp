// phase: 05

// Interfaces whose methods mention each other (a visitor and its nodes), a stack of any, handlers
// that return any (step 5.4): the interface structs are declared before they are defined.
package main

import "fmt"

type Stack struct{ items []any }

func (s *Stack) Push(x any) { s.items = append(s.items, x) }
func (s *Stack) Pop() any {
	n := len(s.items)
	x := s.items[n-1]
	s.items = s.items[:n-1]
	return x
}

type Visitor interface {
	VisitNum(n int) string
	VisitAdd(a, b Node) string
}
type Node interface{ Accept(v Visitor) string }
type Num int
type Add struct{ a, b Node }

func (n Num) Accept(v Visitor) string { return v.VisitNum(int(n)) }
func (a Add) Accept(v Visitor) string { return v.VisitAdd(a.a, a.b) }

type printer struct{ depth int }

func (p *printer) VisitNum(n int) string { return fmt.Sprint(n) }
func (p *printer) VisitAdd(a, b Node) string {
	p.depth++
	defer func() { p.depth-- }()
	return "(" + a.Accept(p) + "+" + b.Accept(p) + ")"
}

type Event struct {
	Name    string
	Payload any
}

type Handler func(Event) any

func main() {
	st := &Stack{}
	st.Push(1)
	st.Push("two")
	st.Push(3.0)
	st.Push(nil)
	st.Push([]int{1, 2})
	for len(st.items) > 0 {
		switch v := st.Pop().(type) {
		case nil:
			fmt.Println("nil")
		case []int:
			fmt.Println("slice", len(v))
		case float64:
			fmt.Println("float", v)
		case string:
			fmt.Println("string", v)
		case int:
			fmt.Println("int", v)
		}
	}
	tree := Add{Num(1), Add{Num(2), Num(3)}}
	fmt.Println(tree.Accept(&printer{}))
	handlers := map[string]Handler{
		"echo": func(e Event) any { return e.Payload },
		"len":  func(e Event) any { return len(e.Name) },
	}
	for _, n := range []string{"echo", "len"} {
		r := handlers[n](Event{"hello", 42})
		if i, ok := r.(int); ok {
			fmt.Println(n, i+1)
		}
	}
	var events []Event
	events = append(events, Event{"a", nil}, Event{"b", Num(3)}, Event{Name: "c", Payload: &printer{depth: 2}})
	for _, e := range events {
		switch p := e.Payload.(type) {
		case Node:
			fmt.Println(e.Name, "node", p.Accept(&printer{}))
		case *printer:
			fmt.Println(e.Name, "printer", p.depth)
		case nil:
			fmt.Println(e.Name, "none")
		}
	}
}

// Output:
// slice 2
// nil
// float 3
// string two
// int 1
// (1+(2+3))
// echo 43
// len 6
// a none
// b node 3
// c printer 2
