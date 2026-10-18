// phase: 05

// %T and %v of recovered panic values: a failed type assertion gives *runtime.TypeAssertionError,
// an index error runtime.boundsError, a nil map write runtime.plainError, panic(fmt.Errorf) gives
// *fmt.wrapError, custom values print as themselves.
package main

import "fmt"

type MyErr struct{ code int }

func (e MyErr) Error() string { return fmt.Sprint("myerr ", e.code) }

func try(f func()) (r any) {
	defer func() { r = recover() }()
	f()
	return
}

func main() {
	var x any = "s"
	var m map[string]int
	var arr []int
	var ip *int
	zero := 0
	vals := []any{
		try(func() { _ = x.(int) }),
		try(func() { _ = x.(fmt.Stringer) }),
		try(func() { arr[5] = 1 }),
		try(func() { m["a"] = 1 }),
		try(func() { _ = *ip }),
		try(func() { _ = 1 / zero }),
		try(func() { panic("plain") }),
		try(func() { panic(MyErr{3}) }),
		try(func() { panic(fmt.Errorf("wrapped %d", 1)) }),
		try(func() { panic(fmt.Sprintf("%d", 12)) }),
		try(func() { panic(42) }),
		try(func() { panic(1.5) }),
		try(func() { var e error; panic(e) }),
	}
	for i, v := range vals {
		fmt.Printf("%d: %T | %v\n", i, v, v)
	}
	fmt.Printf("%v\n", try(func() {}))
}

// Output:
// 0: *runtime.TypeAssertionError | interface conversion: interface {} is string, not int
// 1: *runtime.TypeAssertionError | interface conversion: string is not fmt.Stringer: missing method String
// 2: runtime.boundsError | runtime error: index out of range [5] with length 0
// 3: runtime.plainError | assignment to entry in nil map
// 4: runtime.errorString | runtime error: invalid memory address or nil pointer dereference
// 5: runtime.errorString | runtime error: integer divide by zero
// 6: string | plain
// 7: main.MyErr | myerr 3
// 8: *errors.errorString | wrapped 1
// 9: string | 12
// 10: int | 42
// 11: float64 | 1.5
// 12: *runtime.PanicNilError | panic called with nil argument
// <nil>
