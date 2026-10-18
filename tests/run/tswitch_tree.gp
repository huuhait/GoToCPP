// phase: 05
// ExitCode: 2
// Stderr: ^panic: bad node

// A recursive evaluator of a tree of interface values with a type switch (step 5.5).
package main

import "fmt"

type Node any

type Add struct{ L, R Node }
type Mul struct{ L, R Node }
type Neg struct{ X Node }
type Lit int
type Name string

func eval(n Node, env map[Name]int) int {
	switch n := n.(type) {
	case Lit:
		return int(n)
	case Name:
		return env[n]
	case Add:
		return eval(n.L, env) + eval(n.R, env)
	case Mul:
		return eval(n.L, env) * eval(n.R, env)
	case *Neg:
		return -eval(n.X, env)
	case nil:
		return 0
	}
	panic("bad node")
}

func show(n Node) string {
	switch n := n.(type) {
	case Lit:
		return fmt.Sprint(int(n))
	case Name:
		return string(n)
	case Add:
		return "(" + show(n.L) + " + " + show(n.R) + ")"
	case Mul:
		return show(n.L) + " * " + show(n.R)
	case *Neg:
		return "-" + show(n.X)
	}
	return "?"
}

func main() {
	tree := Add{Mul{Lit(2), Name("x")}, &Neg{Add{Lit(1), nil}}}
	env := map[Name]int{"x": 10}
	fmt.Println(show(tree), "=", eval(tree, env))
	fmt.Println(eval(nil, env))
	defer fmt.Println("deferred")
	fmt.Println(eval(Lit(3), env), eval(struct{}{}, env))
}

// Output:
// (2 * x + -(1 + ?)) = 19
// 0
// deferred
