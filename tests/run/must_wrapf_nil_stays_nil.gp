// phase: 10

package main

import (
	"errors"
	"fmt"
)

func f(fail bool) (err error) {
	defer errors.Wrapf(&err, "context")
	if fail {
		return errors.New("inner")
	}
	return nil
}

func main() {
	err := f(false)
	fmt.Println(err == nil, err)
	err = f(true)
	fmt.Println(err == nil, err)
}

// Output:
// true <nil>
// false context: inner
