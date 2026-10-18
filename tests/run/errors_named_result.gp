// phase: 05

// Idiomatic error returns: (T, error), a deferred function that changes a named error result, early returns.

package main

import (
	"errors"
	"fmt"
)

var ErrNeg = errors.New("negative")

type RangeErr struct{ lo, hi, got int }

func (e RangeErr) Error() string { return fmt.Sprintf("%d not in [%d, %d]", e.got, e.lo, e.hi) }

func check(n int) (int, error) {
	if n < 0 {
		return 0, ErrNeg
	}
	if n > 10 {
		return 0, RangeErr{0, 10, n}
	}
	return n * 2, nil
}

func process(n int) (res int, err error) {
	defer func() {
		if err != nil {
			err = annotated{err}
		}
	}()
	return check(n)
}

type annotated struct{ err error }

func (a annotated) Error() string { return "process: " + a.err.Error() }
func (a annotated) Unwrap() error { return a.err }

func main() {
	for _, n := range []int{3, -1, 11} {
		v, err := process(n)
		if err != nil {
			var re RangeErr
			fmt.Println("error:", err.Error(), errors.Is(err, ErrNeg), errors.As(err, &re), re.got)
			continue
		}
		fmt.Println("value", v)
	}
}

// Output:
// value 6
// error: process: negative true false 0
// error: process: 11 not in [0, 10] false true 11
