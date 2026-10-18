// phase: 05

// errors.Join: text, nil filtering, the Unwrap() []error method.

package main

import (
	"errors"
	"fmt"
)

func main() {
	a, b, c := errors.New("a"), errors.New("b"), errors.New("c")
	fmt.Println(errors.Join() == nil, errors.Join(nil) == nil, errors.Join(nil, nil) == nil)
	j := errors.Join(a, b, c)
	fmt.Printf("%q\n", j.Error())
	fmt.Println(errors.Join(a).Error(), errors.Join(nil, a, nil).Error())
	fmt.Printf("%q\n", errors.Join(nil, a, nil, b).Error())
	u, ok := j.(interface{ Unwrap() []error })
	fmt.Println(ok, len(u.Unwrap()), u.Unwrap()[0] == a, u.Unwrap()[2] == c)
	_, single := j.(interface{ Unwrap() error })
	fmt.Println(single, errors.Unwrap(j) == nil)
	x := errors.Join(a, b)
	y := errors.Join(a, b)
	fmt.Println(x == y, x == x, x.Error() == y.Error())
	fmt.Println(errors.Is(j, a), errors.Is(j, b), errors.Is(j, c), errors.Is(j, errors.New("a")))
	fmt.Println(errors.Join(errors.New("")).Error() == "", len(errors.Join(errors.New(""), errors.New("")).Error()))
}

// Output:
// true true true
// "a\nb\nc"
// a a
// "a\nb"
// true 3 true true
// false true
// false true true
// true true true false
// true 1
