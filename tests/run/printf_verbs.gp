// phase: 02

// fmt.Printf with the Phase 02 verbs %v %d %s %t %f and %%, for every
// basic type, and fmt.Print's spacing rules.
package main

import "fmt"

func main() {
	var i8 int8 = -8
	var u16 uint16 = 65535
	var i32 int32 = -2147483648
	var u64 uint64 = 18446744073709551615
	f32 := float32(1.1)
	f64 := 2.0 / 3.0
	fmt.Printf("%d %d %d %d\n", i8, u16, i32, u64)
	fmt.Printf("%v %v %v %v\n", i8, u16, i32, u64)
	fmt.Printf("%f %f %f %f\n", f32, f64, 1e10, -0.000001)
	fmt.Printf("%v %v %v %v\n", f32, f64, 1e21, 123456789.0)
	fmt.Printf("%s|%v|%s\n", "str", "v", "")
	fmt.Printf("%t %v %t\n", true, false, 1 < 2)
	fmt.Printf("100%% done, %d%%\n", 50)
	fmt.Printf("no verbs\n")
	fmt.Printf("%d+%d=%d\n", 2, 3, 2+3)
	fmt.Printf("%v %v\n", 'a', byte('a'))
	fmt.Printf("%d", 1)
	fmt.Printf("\n")

	fmt.Print("a", "b", 1, 2, "c", 3.5, true, "\n")
	fmt.Print(1, 2, 3, "\n")
	fmt.Print("x", 1, "\n")
	fmt.Println("a", "b", 1, 2, true, 3.5)
	fmt.Println()
	fmt.Print()
	fmt.Println("end")
}

// Output:
// -8 65535 -2147483648 18446744073709551615
// -8 65535 -2147483648 18446744073709551615
// 1.100000 0.666667 10000000000.000000 -0.000001
// 1.1 0.6666666666666666 1e+21 1.23456789e+08
// str|v|
// true false true
// 100% done, 50%
// no verbs
// 2+3=5
// 97 97
// 1
// ab1 2c3.5 true
// 1 2 3
// x1
// a b 1 2 true 3.5
//
// end
