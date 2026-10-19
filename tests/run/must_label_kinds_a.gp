// phase: 05

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

var budget int

// get counts its calls (the program stops after 40) and fails for n > 90.
func get(n int) (int, error) {
	budget++
	if budget > 40 {
		panic("budget")
	}
	fmt.Println("get", n)
	if n > 90 {
		return 0, fmt.Errorf("get %d: %w", n, ErrBad)
	}
	return n, nil
}

func gb(n int, v bool) (bool, error) {
	budget++
	if budget > 40 {
		panic("budget")
	}
	fmt.Println("gb", n, v)
	return v, nil
}

func stmtLabel() (int, error) {
	n := 0
L:
	get!(n)
	n++
	if n < 3 {
		goto L
	}
	return n, nil
}

func ifLabel() (int, error) {
	i := 0
M:
	if gb!(i, i < 2) {
		i++
		goto M
	}
	return i, nil
}

func ifInitLabel() (int, error) {
	i := 0
N:
	if v := get!(i); v < 2 {
		i++
		goto N
	} else if w := get!(v + 10); w > 0 {
		return w, nil
	}
	return i, nil
}

func switchLabel() (int, error) {
	i := 0
S:
	switch get!(i) {
	case 0, 1:
		i++
		goto S
	case get!(50):
		return 50, nil
	default:
		return i, nil
	}
}

func rangeLabel() (int, error) {
	i := 0
R:
	for k := range []int{get!(i), 7} {
		if i < 2 {
			i++
			goto R
		}
		fmt.Println("k", k)
	}
	return i, nil
}

func blockLabel() (int, error) {
	i := 0
B:
	{
		x := get!(i)
		i++
		if x < 2 {
			goto B
		}
	}
	return i, nil
}

func firstStmt() (x int, err error) {
F:
	x = get!(x + 1)
	if x < 3 {
		goto F
	}
	return x, nil
}

func caseClause(k int) (int, error) {
	n := 0
	switch k {
	case 1:
	C:
		x := get!(n)
		n++
		if x < 2 {
			goto C
		}
	case 2:
		n = -1
	}
	return n, nil
}

func forward(k int) (int, error) {
	if k == 0 {
		goto E
	}
	fmt.Println("skipped when k is 0")
E:
	x := get!(k)
	return x, nil
}

func main() {
	fmt.Println(stmtLabel())
	fmt.Println(ifLabel())
	fmt.Println(ifInitLabel())
	fmt.Println(switchLabel())
	fmt.Println(rangeLabel())
	fmt.Println(blockLabel())
	fmt.Println(firstStmt())
	fmt.Println(caseClause(1))
	fmt.Println(caseClause(2))
	fmt.Println(forward(0))
	fmt.Println(forward(5))
	fmt.Println(forward(95))
}

// Output:
// get 0
// get 1
// get 2
// 3 <nil>
// gb 0 true
// gb 1 true
// gb 2 false
// 2 <nil>
// get 0
// get 1
// get 2
// get 12
// 12 <nil>
// get 0
// get 1
// get 2
// get 50
// 2 <nil>
// get 0
// get 1
// get 2
// k 0
// k 1
// 2 <nil>
// get 0
// get 1
// get 2
// 3 <nil>
// get 1
// get 2
// get 3
// 3 <nil>
// get 0
// get 1
// get 2
// 3 <nil>
// -1 <nil>
// get 0
// 0 <nil>
// skipped when k is 0
// get 5
// 5 <nil>
// skipped when k is 0
// get 95
// 0 get 95: bad
