// phase: 05

// fmt.Errorf formats like Sprintf; the result is an error whose type is *errors.errorString
// without %w, *fmt.wrapError with one %w, *fmt.wrapErrors with several.
package main

import "fmt"

type MyErr struct{ code int }

func (e *MyErr) Error() string { return fmt.Sprintf("my error %d", e.code) }

type ValErr struct{ n int }

func (e ValErr) Error() string { return fmt.Sprintf("val %d", e.n) }

func unwrap(err error) error {
	u, ok := err.(interface{ Unwrap() error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func unwrapMany(err error) []error {
	u, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func main() {
	e1 := fmt.Errorf("plain %d %s", 1, "x")
	fmt.Println(e1, e1.Error())
	fmt.Printf("%T %v %q %s\n", e1, e1, e1, e1)
	base := &MyErr{7}
	e2 := fmt.Errorf("wrapped: %w", base)
	fmt.Println(e2)
	fmt.Printf("%T %v %t %t\n", e2, e2, unwrap(e2) == error(base), unwrap(e1) == nil)
	e3 := fmt.Errorf("two: %w and %w", base, ValErr{2})
	fmt.Println(e3)
	fmt.Printf("%T %d %v\n", e3, len(unwrapMany(e3)), unwrapMany(e3))
	e4 := fmt.Errorf("deep: %w", e2)
	fmt.Println(e4, unwrap(unwrap(e4)) == error(base))
	e5 := fmt.Errorf("value: %w", ValErr{9})
	fmt.Println(e5, unwrap(e5))
	e6 := fmt.Errorf("nil: %w", nil)
	fmt.Printf("%v %T %v\n", e6, e6, unwrap(e6))
	e7 := fmt.Errorf("%w", base)
	fmt.Println(e7, e7.Error() == base.Error())
	e8 := fmt.Errorf("no verb")
	fmt.Println(e8, fmt.Errorf("%d%%", 50))
	fmt.Println(fmt.Errorf("%[2]w %[1]w", ValErr{1}, ValErr{2}), fmt.Errorf("%w %[1]w", base))
	fmt.Println(len(unwrapMany(fmt.Errorf("%[2]w %[1]w", ValErr{1}, ValErr{2}))), len(unwrapMany(fmt.Errorf("%w %[1]w", base))))
}

// Output:
// plain 1 x plain 1 x
// *errors.errorString plain 1 x "plain 1 x" plain 1 x
// wrapped: my error 7
// *fmt.wrapError wrapped: my error 7 true true
// two: my error 7 and val 2
// *fmt.wrapErrors 2 [my error 7 val 2]
// deep: wrapped: my error 7 true
// value: val 9 val 9
// nil: %!w(<nil>) *fmt.wrapError <nil>
// my error 7 true
// no verb 50%
// val 2 val 1 my error 7 my error 7
// 2 1
