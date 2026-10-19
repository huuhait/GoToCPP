// phase: 08

// A must call in the channel operation of a select case (step 10.13): the call would run before the select
// picks a case, so it must be written before the select. The body of a case and default are fine
// (tests/run/must_ctx_select_body.gp). The error comes from the lowering, so it needs a file without other errors.

package main

func two(a int) (int, error) { return a * 2, nil }

func ch() (chan int, error) { return make(chan int, 1), nil }

func sel() error {
	c := make(chan int, 1)
	select {
	case c <- two!(1): // ERROR "! in the channel operation of a select case is not supported: evaluate it before the select"
	case v := <-c:
		_ = v
	}
	select {
	case v := <-ch!(): // ERROR "! in the channel operation of a select case is not supported: evaluate it before the select"
		_ = v
	default:
		_ = two!(3)
	}
	return nil
}

func main() { _ = sel() }
