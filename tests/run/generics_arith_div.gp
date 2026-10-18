// phase: 07

// Division and remainder on type arguments: truncation toward zero, the -1 case, and division by zero.
package main

import "fmt"

type Int interface {
	~int8 | ~uint8 | ~int16 | ~int32 | ~int64 | ~int
}

type Num interface {
	Int | ~float32 | ~float64
}

func Div[T Num](a, b T) T { return a / b }
func Rem[T Int](a, b T) T { return a % b }

func Safe[T Num](a, b T) (res T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return Div(a, b), nil
}

func main() {
	fmt.Println(Div(7, 2), Div(-7, 2), Div(7, -2), Div[int8](-128, -1), Div[int64](-9223372036854775808, -1))
	fmt.Println(Rem(7, 3), Rem(-7, 3), Rem(7, -3), Rem[int8](-128, -1), Rem[int64](-9223372036854775808, -1))
	fmt.Println(Div(7.0, 2), Div[float32](1, 3), Div(1.0, 3.0))
	fmt.Println(Div(1.0, 0), Div(-1.0, 0), Div(0.0, 0) != Div(0.0, 0), Div[float32](1, 0))
	fmt.Println(Safe(1, 0))
	fmt.Println(Safe(1.0, 0.0))
	fmt.Println(Safe[int8](5, 2))
	fmt.Println(Safe[uint8](5, 0))
	_, err := Safe[int64](1, 0)
	fmt.Println(err)
	defer func() { fmt.Println("recovered:", recover()) }()
	fmt.Println(Rem[int16](1, 0))
}

// Output:
// 3 -3 -3 -128 -9223372036854775808
// 1 -1 1 0 0
// 3.5 0.33333334 0.3333333333333333
// +Inf -Inf true +Inf
// 0 runtime error: integer divide by zero
// +Inf <nil>
// 2 <nil>
// 0 runtime error: integer divide by zero
// runtime error: integer divide by zero
// recovered: runtime error: integer divide by zero
