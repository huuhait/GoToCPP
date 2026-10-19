# Macros

A macro is a function that runs when the program is compiled and gets the place and the text of
where it is used. It is type-checked where it is declared, not at every use. Macros are used inside
code (`#Assert(...)`), on top of a function (`#Trace()`) and on top of a type (`#Derive(String)`).

> **Experimental today.** Macros are phase 10. Run this lesson with
> `GOCPP_GATE_PHASE=10 gocpp run .`. The program is not valid Go: `macro` and `#Name` are gocpp-only
> syntax. `GOCPP_GATE_PHASE=10 gocpp expand .`
> prints the plain Go that they stand for ([guide](../guides/expand.md)).

<!-- program: 10 id=macros -->
```go
package main

import "fmt"

// Assert stops the program when cond is false. msg is lazy: it is built only
// when the assertion fails, so a passing check costs nothing for its message.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

// Trace prints when a function starts and, when the func() it returns runs, when it ends.
macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

// Derive writes methods next to the type.
#Derive(String)
type Level int

const (
	Debug Level = iota
	Info
	Warn
)

#Trace()
func ratio(a, b int) int {
	#Assert(b != 0, fmt.Sprintf("ratio(%d, %d)", a, b))
	return a / b
}

func main() {
	fmt.Println(Debug, Warn, Level(7))
	fmt.Println("ratio:", ratio(8, 2))

	defer func() { fmt.Println("recovered:", recover()) }()
	ratio(1, 0)
}
```

<!-- output -->
```text
Debug Warn Level(7)
enter main.ratio
exit  main.ratio
ratio: 4
enter main.ratio
exit  main.ratio
recovered: main.gp:31: assertion failed: b != 0: ratio(1, 0)
```

Without the switch the compiler refuses the program:

<!-- rejected: macros -->
```text
./main.gp:7:1: macros are not supported yet (Phase 10)
```

## What to notice

- `#file()`, `#line()`, `#func()` and `#text(x)` describe the place where the macro is used, not
  the macro: the failed assertion names the line of `#Assert` and the text `b != 0` that you wrote.
- `#Trace()` on top of `ratio` runs `Trace` as the first statement of the function, and the
  `func()` it returns when the function ends, also when it panics. The `exit` line is printed
  before the panic reaches `main`.
- `#Derive(String)` wrote `Level.String`. `Equal` and `Clone` can be derived too.
- Wrap-style macros (`#body()`) run a function's code the way the macro wants: in a goroutine, under
  a lock, with `recover`. [examples/macros](../../examples/macros/main.gp) has `#Locked`,
  `#Recover` and `#InThread`.
- The full rules are in the [spec](../spec/language.md#macros): the built-in macros, `#embed`, the wrap-style rules and
  how macros are compiled.

Next: [Where to go next](13-next.md).
