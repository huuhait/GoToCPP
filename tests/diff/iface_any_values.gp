// phase: 05

// The dynamic types of constants and variables converted to any: any(1) is an int, any(1.5) a
// float64, a rune constant an int32, an untyped nil the nil interface (step 5.4).
package main

import "fmt"

func kind(x any) string {
	switch x.(type) {
	case nil:
		return "nil"
	case int:
		return "int"
	case int8:
		return "int8"
	case int16:
		return "int16"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case uint:
		return "uint"
	case uint8:
		return "uint8"
	case uint16:
		return "uint16"
	case uint32:
		return "uint32"
	case uint64:
		return "uint64"
	case uintptr:
		return "uintptr"
	case float32:
		return "float32"
	case float64:
		return "float64"
	case complex64:
		return "complex64"
	case complex128:
		return "complex128"
	case bool:
		return "bool"
	case string:
		return "string"
	}
	return "other"
}

const typed int8 = 5
const big = 1 << 40
const f = 2.5
const r = 'x'

func main() {
	var i int = 1
	var i8 int8 = 1
	var u uint = 1
	var b byte = 1
	var ru rune = 'a'
	fmt.Println(kind(1), kind(1.5), kind('a'), kind("s"), kind(true), kind(nil), kind(2i))
	fmt.Println(kind(i), kind(i8), kind(u), kind(b), kind(ru), kind(typed), kind(big), kind(f), kind(r))
	fmt.Println(kind(int16(1)), kind(int32(1)), kind(int64(1)), kind(uint16(1)), kind(uint32(1)), kind(uint64(1)), kind(uintptr(1)))
	fmt.Println(kind(float32(1)), kind(complex64(1)), kind(complex128(1)), kind(any(1)), kind(any(nil)))
	var x any = 1
	var y any = int64(1)
	var z any = uint8(1)
	_, ok1 := x.(int64)
	_, ok2 := y.(int)
	_, ok3 := z.(byte)
	_, ok4 := z.(int)
	fmt.Println(ok1, ok2, ok3, ok4)
}
