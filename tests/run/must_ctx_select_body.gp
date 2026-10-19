// phase: 08

// ! in a select statement: in the BODY of a case and of default it is a plain statement (each body runs
// only when its case is taken); in the channel operation of a case it is an error (tests/errors/
// must_select_case.gp). The twin (testdata/must_twin/must_ctx_select_body.go) writes the checks by hand.

package main

import (
	"errors"
	"fmt"
)

var ErrBad = errors.New("bad")

func half(n int) (int, error) {
	fmt.Println("half", n)
	if n%2 != 0 {
		return 0, fmt.Errorf("half %d: %w", n, ErrBad)
	}
	return n / 2, nil
}

// pick takes a value from a (or b), and halves it in the body of the case that was taken.
func pick(a, b chan int) (int, error) {
	select {
	case v := <-a:
		fmt.Println("from a")
		return half!(v), nil
	case v := <-b:
		fmt.Println("from b")
		w := half!(v) + half!(v+2)
		return w, nil
	}
}

// poll has a default body that fails.
func poll(c chan int) (int, error) {
	select {
	case v := <-c:
		return v, nil
	default:
		fmt.Println("empty")
		return half!(3), nil
	}
}

// loop sends 3 values and receives them in a select that halves each one; the error of the
// second one ends the function while the loop runs.
func loop() (sum int, err error) {
	c := make(chan int, 3)
	for _, v := range []int{4, 5, 6} {
		c <- v
	}
	for range 3 {
		select {
		case v := <-c:
			sum += half!(v)
		}
	}
	return sum, nil
}

func main() {
	a, b := make(chan int, 1), make(chan int, 1)
	a <- 8
	fmt.Println(pick(a, b))
	a <- 3
	fmt.Println(pick(a, b))
	b <- 4
	fmt.Println(pick(a, b))
	b <- 5
	fmt.Println(pick(a, b))
	fmt.Println(poll(a))
	a <- 7
	fmt.Println(poll(a))
	fmt.Println(loop())
}

// Output:
// from a
// half 8
// 4 <nil>
// from a
// half 3
// 0 half 3: bad
// from b
// half 4
// half 6
// 5 <nil>
// from b
// half 5
// 0 half 5: bad
// empty
// half 3
// 0 half 3: bad
// 7 <nil>
// half 4
// half 5
// 0 half 5: bad
