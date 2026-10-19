# Must functions

Go code is full of `if err != nil { return ..., err }`. gocpp has an opt-in shorthand: a `!` on a
function or on one call returns the error for you. It is sugar, rewritten into the early returns you
would write by hand, so it costs nothing at run time. Code without `!` is plain Go.

> **Experimental today.** The `!` syntax is phase 10. Run this lesson with
> `GOCPP_GATE_PHASE=10 gocpp run .`. Without the switch `gocpp check` refuses the program
> (below). The program is not valid Go: `!` is gocpp-only syntax. `GOCPP_GATE_PHASE=10 gocpp expand .`
> prints the plain Go that it stands for ([guide](../guides/expand.md)).

<!-- program: 10 id=must exit=1 -->
```go
package main

import (
	"fmt"
	"strconv"
)

type Config struct{ Port, Workers int }

// parse is a must function: the errors of strconv.Atoi are returned for you,
// and the callers see func parse(port, workers string) (Config, error).
func parse!(port, workers string) Config {
	p := strconv.Atoi(port)
	w := strconv.Atoi(workers)
	if p < 1024 {
		// One value more than the declared results: fail on purpose.
		return Config{}, fmt.Errorf("port %d is below 1024", p)
	}
	return Config{p, w}
}

// describe is an ordinary function that uses a must call: parse!(...) gives
// the Config, or returns the error from describe.
func describe(port, workers string) (string, error) {
	cfg := parse!(port, workers)
	return fmt.Sprintf("%d workers on port %d", cfg.Workers, cfg.Port), nil
}

// main! prints an error to standard error and exits with status 1.
func main!() {
	for _, in := range [][2]string{{"8080", "4"}, {"80", "4"}, {"8080", "x"}} {
		s, err := describe(in[0], in[1])
		fmt.Println(s, "|", err)
	}
	cfg := parse("22", "1")
	fmt.Println("not reached", cfg)
}
```

<!-- output -->
```text
4 workers on port 8080 | <nil>
 | port 80 is below 1024
 | strconv.Atoi: parsing "x": invalid syntax
```

Without the switch the compiler refuses the program:

<!-- rejected: must -->
```text
./main.gp:12:11: must functions are not supported yet (Phase 10)
```

The last line of `main` fails: `parse("22", "1")` returns an error that `main!` does not take, so
the program prints `NAME: port 22 is below 1024` on standard error (`NAME` is the name of the program) and exits with status 1.

## What to notice

- `func parse!(...) Config` returns `Config` and an `error`. Inside it, every call whose error you
  drop (`p := strconv.Atoi(port)`) returns that error, with the other results zero. A `return` with
  one value more than declared fails on purpose.
- `parse!(...)` at a call site is the call-level form: it works on any function whose last result is
  `error`, also in the middle of an expression (`strconv.Atoi!(s) * 2`).
- `if!(x)`, `return!(x)` and `for!(x)` are still Go: after a keyword `!` is "not".
- The rules, the error messages and what happens in a test or a closure are in the
  [spec](../spec/language.md#must-functions-and-must-calls) and the
  [`!` design record](../adr/013-must-errors.md).

Next: [Macros](12-macros.md).
