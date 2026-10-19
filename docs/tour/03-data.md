# Slices, maps, structs and methods

Arrays, slices, maps and structs have Go's types and Go's rules. Pointers too, with one
difference that you do not see in the code: memory is reference counted, so a pointer keeps its
object alive.

<!-- program: 03 id=data go -->
```go
package main

import "fmt"

// A struct with a method that changes it (pointer receiver)
// and one that does not (value receiver).
type Counter struct {
	Name  string
	Count int
}

func (c *Counter) Add(n int) { c.Count += n }

func (c Counter) String() string {
	return fmt.Sprintf("%s=%d", c.Name, c.Count)
}

func main() {
	// A slice grows with append; len and cap are as in Go.
	var xs []int
	for i := 1; i <= 5; i++ {
		xs = append(xs, i*i)
	}
	fmt.Println(xs, len(xs), xs[1:3])

	// A map. Looking up a missing key gives the zero value.
	stock := map[string]int{"apple": 3, "pear": 0}
	stock["plum"] = 7
	delete(stock, "pear")
	n, ok := stock["pear"]
	fmt.Println(len(stock), stock["apple"], n, ok)

	// Structs are values; a pointer shares one.
	a := Counter{Name: "a"}
	b := a // a copy
	p := &a
	p.Add(5)
	b.Add(1)
	fmt.Println(a, b, *p)

	// Arrays are values too.
	grid := [2][3]int{{1, 2, 3}, {4, 5, 6}}
	copyOfGrid := grid
	copyOfGrid[0][0] = 99
	fmt.Println(grid[0][0], copyOfGrid[0][0])
}
```

<!-- output -->
```text
[1 4 9 16 25] 5 [4 9]
2 3 0 false
a=5 a=1 a=5
1 99
```

## What to notice

- `fmt` finds the `String` method of `Counter` and uses it for `Println(a, b, *p)`.
- `p := &a` makes a `*Counter` that shares `a`. In C++ it is a `gp::ptr<Counter>`, a reference
  counting pointer, and `a`, whose address is taken, lives in a box on the heap that the pointer
  shares. You write the same code as in Go; the [spec](../spec/language.md#d1-reference-counting-instead-of-a-garbage-collector)
  lists what reference counting changes (for example, a cycle of pointers is never freed).
- Map order is random, as in Go: do not print a map by ranging over it if the order matters
  (`fmt.Println(m)` sorts the keys, like Go).

Next: [Closures and defer](04-closures-and-defer.md).
