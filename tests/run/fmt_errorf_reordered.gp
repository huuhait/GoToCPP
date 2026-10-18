// phase: 05

// Errorf with %w and argument indexes: the wrapped errors of %[2]w %[1]w are in argument order
// after sorting; a repeated %[1]w counts once; operands that are not errors are skipped by
// Unwrap() []error; %w with width and flags.
package main

import "fmt"

func many(err error) []error {
	u, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func one(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func main() {
	e1, e2, e3 := fmt.Errorf("e1"), fmt.Errorf("e2"), fmt.Errorf("e3")
	for _, f := range []string{"%[2]w %[1]w", "%[1]w %[2]w %[1]w", "%[3]w %[1]w %[2]w", "%w %[1]w", "%[2]w %w", "%[3]w %[3]w", "%w %w %w", "%[1]w|%[3]w"} {
		err := fmt.Errorf(f, e1, e2, e3)
		w := many(err)
		fmt.Printf("%-18s -> %v | %T | %d:", f, err, err, len(w))
		for _, x := range w {
			fmt.Print(" ", x)
		}
		fmt.Println()
	}
	err := fmt.Errorf("%[2]w", e1, e2)
	fmt.Println(err, one(err), many(err) == nil)
	err = fmt.Errorf("%[1]w %[1]w", e1)
	fmt.Println(err, one(err), many(err) == nil, len(many(err)))
	err = fmt.Errorf("%w %w %w", e1, 7, e3)
	fmt.Println(err, len(many(err)))
	err = fmt.Errorf("%5w|%-5w|%+w|%#w", e1, e2, e3, e1)
	fmt.Println(err, len(many(err)))
	err = fmt.Errorf("%[3]w %[1]w", e1, e2, e3, e1)
	fmt.Println(err, len(many(err)), many(err)[0] == e1)
}

// Output:
// %[2]w %[1]w        -> e2 e1 | *fmt.wrapErrors | 2: e1 e2
// %[1]w %[2]w %[1]w  -> e1 e2 e1 | *fmt.wrapErrors | 2: e1 e2
// %[3]w %[1]w %[2]w  -> e3 e1 e2 | *fmt.wrapErrors | 3: e1 e2 e3
// %w %[1]w           -> e1 e1 | *fmt.wrapErrors | 1: e1
// %[2]w %w           -> e2 e3 | *fmt.wrapErrors | 2: e2 e3
// %[3]w %[3]w        -> e3 e3 | *fmt.wrapErrors | 1: e3
// %w %w %w           -> e1 e2 e3 | *fmt.wrapErrors | 3: e1 e2 e3
// %[1]w|%[3]w        -> e1|e3 | *fmt.wrapErrors | 2: e1 e3
// e2 e2 true
// e1 e1 <nil> false 1
// e1 %!w(int=7) e3 2
//    e1|e2   |e3|&%!w(errors.errorString=errors.errorString{s:"e1"}) 4
// e3 e1 2 true
