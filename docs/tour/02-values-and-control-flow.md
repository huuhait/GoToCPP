# Values and control flow

The basic types, constants, `for` loops, `switch` and functions with several results work as in
Go.

<!-- program: 03 id=values go -->
```go
package main

import "fmt"

// Constants with iota: the size classes.
const (
	Small = iota
	Medium
	Large
)

// A function can return several values.
func divmod(a, b int) (int, int) {
	return a / b, a % b
}

// A switch with no tag picks the first true case.
func classify(n int) int {
	switch {
	case n < 10:
		return Small
	case n < 100:
		return Medium
	}
	return Large
}

func main() {
	q, r := divmod(17, 5)
	fmt.Println("17 / 5 =", q, "remainder", r)

	// range over an integer, a Go 1.22 feature.
	for i := range 4 {
		fmt.Printf("%3d has class %d\n", i*40, classify(i*40))
	}

	sum := 0
	for i := 1; i <= 10; i++ {
		if i%2 == 0 {
			continue
		}
		sum += i
	}
	fmt.Println("sum of the odd numbers up to 10:", sum)

	var small uint8 = 250
	small += 10 // wraps at the width of the type
	var ratio float64 = 2.0 / 3.0
	fmt.Printf("%.4f %v %t\n", ratio, small, ratio > 0.5)
}
```

<!-- output -->
```text
17 / 5 = 3 remainder 2
  0 has class 0
 40 has class 1
 80 has class 1
120 has class 2
sum of the odd numbers up to 10: 25
0.6667 4 true
```

## What to notice

- `int` is 64 bits, as in Go on a 64-bit machine. gocpp supports 64-bit targets only; the sizes
  are in the [language spec](../spec/language.md#d15-different-sizes).
- Integer arithmetic wraps at the width of the type, as in Go: `small` is 250, and 250 + 10 is 4
  in a `uint8`. You get Go's results, not C++'s undefined behavior on signed overflow.
- `fmt.Printf` takes Go's verbs and flags (`%3d`, `%.4f`, `%v`, `%t`).
- A `switch` does not fall through to the next case unless you write `fallthrough`.

Next: [Slices, maps, structs and methods](03-data.md).
