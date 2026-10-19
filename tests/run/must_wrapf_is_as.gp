// phase: 10

package main

import (
	"errors"
	"fmt"
)

type CodeErr struct{ Code int }

func (e *CodeErr) Error() string { return fmt.Sprint("code ", e.Code) }

var ErrBase = errors.New("base")

func fail(kind int) error {
	switch kind {
	case 1:
		return ErrBase
	case 2:
		return &CodeErr{42}
	}
	return nil
}

func run!(kind int) (err error) {
	defer errors.Wrapf(&err, "run %d", kind)
	fail(kind)
	return nil
}

func main() {
	for kind := 0; kind < 3; kind++ {
		err := run(kind)
		var ce *CodeErr
		fmt.Println(kind, err, errors.Is(err, ErrBase), errors.As(err, &ce))
		if ce != nil {
			fmt.Println("code", ce.Code)
		}
	}
}

// Output:
// 0 <nil> false false
// 1 run 1: base true false
// 2 run 2: code 42 false true
// code 42
