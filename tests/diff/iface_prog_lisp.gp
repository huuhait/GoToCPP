// phase: 05

// A tiny Lisp evaluator on interface{} values, recursive types, func types in interfaces and recovered panics.
package main

import "fmt"

type Value interface{}
type List struct {
	Head Value
	Tail *List
}
type Env map[string]Value
type Func func(args ...Value) Value

func (l *List) Len() int {
	n := 0
	for ; l != nil; l = l.Tail {
		n++
	}
	return n
}
func (l *List) String() string {
	s := "("
	for p := l; p != nil; p = p.Tail {
		if p != l {
			s += " "
		}
		s += fmt.Sprint(p.Head)
	}
	return s + ")"
}
func cons(h Value, t *List) *List { return &List{h, t} }

func eval(e Value, env Env) Value {
	switch x := e.(type) {
	case int, string, bool:
		if s, ok := x.(string); ok {
			if v, ok := env[s]; ok {
				return v
			}
		}
		return x
	case *List:
		if x == nil {
			return nil
		}
		op := eval(x.Head, env)
		var args []Value
		for p := x.Tail; p != nil; p = p.Tail {
			args = append(args, eval(p.Head, env))
		}
		switch f := op.(type) {
		case Func:
			return f(args...)
		case func(...Value) Value:
			return f(args...)
		}
		panic(fmt.Sprintf("not a function: %v (%T)", op, op))
	}
	panic(fmt.Sprintf("cannot eval %T", e))
}

func main() {
	env := Env{
		"+": Func(func(a ...Value) Value {
			s := 0
			for _, v := range a {
				s += v.(int)
			}
			return s
		}),
		"list": func(a ...Value) Value {
			var l *List
			for i := len(a) - 1; i >= 0; i-- {
				l = cons(a[i], l)
			}
			return l
		},
		"len": Func(func(a ...Value) Value { return a[0].(*List).Len() }),
		"x":   40,
	}
	prog := cons("+", cons("x", cons(2, cons(cons("len", cons(cons("list", cons(1, cons(2, nil))), nil)), nil))))
	fmt.Println(prog, eval(prog, env))
	for _, bad := range []Value{cons(1, nil), 3.5, cons("nope", nil), []int{1}} {
		func() {
			defer func() { fmt.Println("error:", recover()) }()
			fmt.Println(eval(bad, env))
		}()
	}
	var v Value = env
	if e, ok := v.(Env); ok {
		fmt.Println(len(e), e["x"])
	}
	_, isMap := v.(map[string]Value)
	fmt.Println(isMap)
}
