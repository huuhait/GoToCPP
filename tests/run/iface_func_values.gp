// phase: 05

// Func values and method values in interfaces (step 5.4).
package main

import "fmt"

type Op func(int, int) int

func (o Op) Apply(a, b int) int { return o(a, b) }

type Applier interface{ Apply(a, b int) int }

func add(a, b int) int { return a + b }

func main() {
	var x any = add
	f := x.(func(int, int) int)
	fmt.Println(f(1, 2))
	var ap Applier = Op(add)
	fmt.Println(ap.Apply(3, 4))
	ops := map[string]any{"add": add, "mul": func(a, b int) int { return a * b }, "neg": func(a int) int { return -a }}
	for _, k := range []string{"add", "mul", "neg"} {
		switch fn := ops[k].(type) {
		case func(int, int) int:
			fmt.Println(k, fn(6, 7))
		case func(int) int:
			fmt.Println(k, fn(6))
		}
	}
	_, isOp := x.(Op)
	fmt.Println(isOp)
	var y any = Op(add)
	_, isFn := y.(func(int, int) int)
	fmt.Println(isFn, y.(Applier).Apply(1, 1))
	counter := 0
	var inc any = func() int { counter++; return counter }
	inc.(func() int)()
	inc.(func() int)()
	fmt.Println(counter)
	fs := map[string]func(int) any{"id": func(x int) any { return x }, "str": func(x int) any { return fmt.Sprint(x) }}
	fmt.Println(fs["id"](3).(int)+1, fs["str"](3).(string)+"!")
	var nilf func()
	var nf any = nilf
	fmt.Println(nf == nil, nf.(func()) == nil)
}

// Output:
// 3
// 7
// add 13
// mul 42
// neg -6
// false
// false 2
// 2
// 4 3!
// false true
