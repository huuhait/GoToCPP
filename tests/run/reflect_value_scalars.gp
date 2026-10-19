// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type MyInt int16
type MyStr string
type MyF float32

func main() {
	vals := []any{true, int(-1), int8(-8), int16(-16), int32(-32), int64(-64), uint(1), uint8(8), uint16(16), uint32(32), uint64(64), uintptr(99),
		float32(1.25), 6.5, complex64(1 + 2i), 3 + 4i, "str", MyInt(-7), MyStr("named"), MyF(2.5)}
	for _, x := range vals {
		v := reflect.ValueOf(x)
		switch v.Kind() {
		case reflect.Bool:
			fmt.Println(v.Kind(), v.Bool())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			fmt.Println(v.Kind(), v.Int(), v.CanInt(), v.CanUint(), v.CanFloat())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			fmt.Println(v.Kind(), v.Uint(), v.CanInt(), v.CanUint())
		case reflect.Float32, reflect.Float64:
			fmt.Println(v.Kind(), v.Float(), v.CanFloat())
		case reflect.Complex64, reflect.Complex128:
			fmt.Println(v.Kind(), v.Complex(), real(v.Complex()), imag(v.Complex()), v.CanComplex())
		case reflect.String:
			fmt.Println(v.Kind(), v.String(), v.Len(), v.Type())
		}
	}
}

// Output:
// bool true
// int -1 true false false
// int8 -8 true false false
// int16 -16 true false false
// int32 -32 true false false
// int64 -64 true false false
// uint 1 false true
// uint8 8 false true
// uint16 16 false true
// uint32 32 false true
// uint64 64 false true
// uintptr 99 false true
// float32 1.25 true
// float64 6.5 true
// complex64 (1+2i) 1 2 true
// complex128 (3+4i) 3 4 true
// string str 3 string
// int16 -7 true false false
// string named 5 main.MyStr
// float32 2.5 true
