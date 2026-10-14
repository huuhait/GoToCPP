// phase: 02

// fmt.Printf with the Phase 02 verbs on every basic type, and fmt's error
// forms: a verb of the wrong type, a missing argument, an extra argument
// and a % at the end.
package main

import "fmt"

func main() {
	var i8 int8 = -5
	var u8 uint8 = 250
	var i16 int16 = -300
	var u16 uint16 = 60000
	var i32 int32 = -70000
	var u32 uint32 = 4000000000
	var u64 uint64 = 18446744073709551615
	var f32 float32 = 1.25
	i := 42
	f := 3.14159
	s := "str"
	b := true
	fmt.Printf("%d %d %d %d %d %d %d %d\n", i8, u8, i16, u16, i32, u32, u64, i)
	fmt.Printf("%v %v %v %v %v %v %v %v\n", i8, u8, i16, u16, i32, u32, u64, i)
	fmt.Printf("%f %f %v %v\n", f32, f, f32, f)
	fmt.Printf("%s %v %t %v\n", s, s, b, b)
	fmt.Printf("[%d] [%s] [%t] [%f]\n", s, i, i, i)
	fmt.Printf("[%s] [%d] [%t]\n", f, b, s)
	fmt.Printf("%d %d\n", 1)
	fmt.Printf("%d\n", 1, 2)
	fmt.Printf("%d\n", 1, "two", true)
	fmt.Printf("percent at the end %")
	fmt.Println()
	fmt.Printf("%%d %v%%\n", 7)
	fmt.Printf("")
	fmt.Printf("%v|%v|%v\n", "", 0, false)
	fmt.Printf("%s\n", "é and 日本")
	fmt.Printf("%v %v\n", 1e6, 1e-7)
	fmt.Printf("%f %f\n", float32(0.1), 1e-10)
}
