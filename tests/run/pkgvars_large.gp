// phase: 03

// Package variables larger than 128 KiB (D24): they live in static storage, never on the
// stack, and gp_var_init builds a literal in place, element by element and field by field,
// also a literal of constants (clang++ can't evaluate a keyed literal of more than 1 << 20
// elements at compile time), one of a named array type, a struct with a large array field,
// and a literal inside a conversion (block([1 << 21]int{...}), [1 << 21]int(block{...}),
// recCopy(record{...})), so that no 16 MiB temporary goes on the C++ stack. A copy of another
// large variable, &[N]T{...} and the zero value work too.
package main

import "fmt"

type block [1 << 21]int

type record struct {
	cells [1 << 18]int
	name  string
	n     int
}

type recCopy record

var (
	zero      [1 << 21]int
	consts    = [1 << 21]int{1, 2, 3}
	keyed     = [1 << 21]int{5: 1}
	namedK    = block{7: 3}
	recK      = record{cells: [1 << 18]int{5: 1}}
	sparse    = [1 << 21]int{0: next(), 1<<21 - 1: 7}
	copied    = sparse
	named     = block{5: next(), 6: 1}
	rec       = record{name: "rec", n: next(), cells: [1 << 18]int{1: next()}}
	ptr       = &[1 << 20]int{1: next()}
	mixed     = [1 << 20]int{next(), next(), 10: next()}
	toNamed   = block([1 << 21]int{5: next()})
	toUnnamed = [1 << 21]int(block{6: next()})
	toCopy    = recCopy(record{n: next(), cells: [1 << 18]int{2: 4}})
	counter   int
)

func next() int {
	counter++
	return counter * 10
}

func main() {
	zero[5] = 1
	fmt.Println(zero[5], consts[2], keyed[5], keyed[6], namedK[7], recK.cells[5], recK.name == "")
	fmt.Println(sparse[0], sparse[1<<21-1], copied[0], named[5], named[6])
	fmt.Println(rec.name, rec.n, rec.cells[1], ptr[1], mixed[0], mixed[1], mixed[10])
	fmt.Println(toNamed[5], toUnnamed[6], toCopy.n, toCopy.cells[2], counter)
	sparse[0] = -1
	fmt.Println(copied[0], len(named), len(rec.cells))
}

// Output:
// 1 3 1 0 3 1 true
// 10 7 10 20 1
// rec 30 40 50 60 70 80
// 90 100 110 4 11
// 10 2097152 262144
