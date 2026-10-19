# Closures and defer

Functions are values. A function literal captures the variables it uses, and `defer` runs a call
when the function returns, last in first out.

<!-- program: 05 id=closures go -->
```go
package main

import "fmt"

// counter returns a function that remembers n between calls.
func counter() func() int {
	n := 0
	return func() int {
		n++
		return n
	}
}

// safeDivide turns a run-time panic into an error result: a deferred
// closure can change the named results.
func safeDivide(a, b int) (result int, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			result, ok = 0, false
		}
	}()
	return a / b, true
}

func main() {
	next := counter()
	fmt.Println(next(), next(), next())

	// Go 1.22: each iteration has its own copy of the loop variable.
	var funcs []func() int
	for i := 0; i < 3; i++ {
		funcs = append(funcs, func() int { return i * 10 })
	}
	for _, f := range funcs {
		fmt.Print(f(), " ")
	}
	fmt.Println()

	// Deferred calls run in reverse order, with the arguments evaluated at the defer.
	for i := 0; i < 3; i++ {
		defer fmt.Println("deferred", i)
	}

	fmt.Println(safeDivide(7, 2))
	fmt.Println(safeDivide(7, 0))
}
```

<!-- output -->
```text
1 2 3
0 10 20 
3 true
0 false
deferred 2
deferred 1
deferred 0
```

## What to notice

- `recover` works as in Go. It returns an `any`, so the lesson needs interfaces (phase 05, which
  is finished). A panic is a C++ exception inside the generated code, and `recover` catches it.
- A captured variable is shared by the closure and the function that made it, as in Go: `n` of
  `counter` lives as long as the returned function does.
- `fmt.Println(safeDivide(7, 2))` passes both results of a call to a variadic function, which Go
  allows too.

Next: [Interfaces and errors](05-interfaces-errors.md).
