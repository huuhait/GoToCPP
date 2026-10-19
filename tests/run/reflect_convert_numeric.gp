// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type MyInt int16
type Celsius float64

func main() {
	types := []reflect.Type{reflect.TypeOf(int8(0)), reflect.TypeOf(int(0)), reflect.TypeOf(uint8(0)), reflect.TypeOf(uint32(0)), reflect.TypeOf(uint64(0)),
		reflect.TypeOf(float32(0)), reflect.TypeOf(float64(0)), reflect.TypeOf(MyInt(0)), reflect.TypeOf(Celsius(0))}
	srcs := []any{int(300), int8(-5), uint8(200), uint64(1 << 40), 3.99, -3.99, float32(2.5), MyInt(77), Celsius(36.6)}
	for _, s := range srcs {
		sv := reflect.ValueOf(s)
		fmt.Printf("%T %v:", s, s)
		for _, t := range types {
			fmt.Printf(" %v", sv.Convert(t).Interface())
		}
		fmt.Println()
	}
	c := reflect.ValueOf(complex64(1 + 2i)).Convert(reflect.TypeOf(complex128(0)))
	fmt.Println(c.Interface(), c.Type(), reflect.ValueOf(3+4i).Convert(reflect.TypeOf(complex64(0))).Interface())
	fmt.Println(reflect.ValueOf(int8(1)).CanConvert(reflect.TypeOf(1.5)), reflect.ValueOf(1).CanConvert(reflect.TypeOf("")), reflect.ValueOf(1).CanConvert(reflect.TypeOf([]int{})))
}

// Output:
// int 300: 44 300 44 300 300 300 300 300 300
// int8 -5: -5 -5 251 4294967291 18446744073709551611 -5 -5 -5 -5
// uint8 200: -56 200 200 200 200 200 200 200 200
// uint64 1099511627776: 0 1099511627776 0 0 1099511627776 1.0995116e+12 1.099511627776e+12 0 1.099511627776e+12
// float64 3.99: 3 3 3 3 3 3.99 3.99 3 3.99
// float64 -3.99: -3 -3 253 4294967293 18446744073709551613 -3.99 -3.99 -3 -3.99
// float32 2.5: 2 2 2 2 2 2.5 2.5 2 2.5
// main.MyInt 77: 77 77 77 77 77 77 77 77 77
// main.Celsius 36.6: 36 36 36 36 36 36.6 36.6 36 36.6
// (1+2i) complex128 (3+4i)
// true true false
