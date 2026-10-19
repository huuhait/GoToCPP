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
	if _, err := get(n); err != nil {
		return 0, err
	}
	n++
	if n < 3 {
		goto L
	}
	return n, nil
}

func ifLabel() (int, error) {
	i := 0
M:
	c, err := gb(i, i < 2)
	if err != nil {
		return 0, err
	}
	if c {
		i++
		goto M
	}
	return i, nil
}

func ifInitLabel() (int, error) {
	i := 0
N:
	v, err := get(i)
	if err != nil {
		return 0, err
	}
	if v < 2 {
		i++
		goto N
	} else {
		w, err := get(v + 10)
		if err != nil {
			return 0, err
		}
		if w > 0 {
			return w, nil
		}
	}
	return i, nil
}

func switchLabel() (int, error) {
	i := 0
S:
	t, err := get(i)
	if err != nil {
		return 0, err
	}
	switch {
	case t == 0 || t == 1:
		i++
		goto S
	default:
		c, err := get(50)
		if err != nil {
			return 0, err
		}
		if t == c {
			return 50, nil
		}
		return i, nil
	}
}

func rangeLabel() (int, error) {
	i := 0
R:
	a, err := get(i)
	if err != nil {
		return 0, err
	}
	for k := range []int{a, 7} {
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
		x, err := get(i)
		if err != nil {
			return 0, err
		}
		i++
		if x < 2 {
			goto B
		}
	}
	return i, nil
}

func firstStmt() (x int, err error) {
F:
	t, err := get(x + 1)
	if err != nil {
		return 0, err
	}
	x = t
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
		x, err := get(n)
		if err != nil {
			return 0, err
		}
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
	x, err := get(k)
	if err != nil {
		return 0, err
	}
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
