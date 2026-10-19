// phase: 10

// In a must function, a call whose last result is a concrete error type (not exactly error) is
// not handled for you: a nil *MyErr would become a non-nil error. Taking the error yourself,
// or dropping it with _, is fine.
package main

type MyErr struct{}

func (*MyErr) Error() string { return "my" }

func typed() (int, *MyErr) { return 0, nil }

func typedOnly() *MyErr { return nil }

func run!() int {
	a := typed()    // ERROR "assignment mismatch: 1 variable but typed returns 2 values" "the last result of typed is \*MyErr, not error: a must function does not handle it"
	typedOnly()    // ERROR "the last result of typedOnly is \*MyErr, not error"
	b, _ := typed()
	c, err := typed()
	_ = err
	return a + b + c
}
