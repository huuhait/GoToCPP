// phase: 05

// Sentinel errors: package variables compared with ==, in switches and in maps.

package main

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound = errors.New("not found")
	ErrDenied   = errors.New("denied")
	ErrOther    = errors.New("not found")
)

func find(k int) error {
	switch k {
	case 0:
		return nil
	case 1:
		return ErrNotFound
	case 2:
		return ErrDenied
	}
	return ErrOther
}

func name(err error) string {
	switch err {
	case nil:
		return "ok"
	case ErrNotFound:
		return "notfound"
	case ErrDenied:
		return "denied"
	}
	return "other: " + err.Error()
}

func main() {
	for k := 0; k < 4; k++ {
		err := find(k)
		fmt.Println(k, name(err), err == ErrNotFound, err == nil)
	}
	counts := map[error]int{}
	for _, k := range []int{1, 2, 1, 3, 3, 3, 0} {
		counts[find(k)]++
	}
	fmt.Println(len(counts), counts[ErrNotFound], counts[ErrDenied], counts[ErrOther], counts[nil])
	fmt.Println(errors.Is(ErrOther, ErrNotFound), errors.Is(ErrNotFound, ErrNotFound))
}

// Output:
// 0 ok false true
// 1 notfound true false
// 2 denied false false
// 3 other: not found false false
// 4 2 1 3 1
// false true
