// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Rec struct {
	B   bool
	I   int
	I8  int8
	I16 int16
	I32 int32
	I64 int64
	U   uint
	U8  uint8
	U16 uint16
	U32 uint32
	U64 uint64
	Up  uintptr
	F32 float32
	F64 float64
	C64 complex64
	C   complex128
	S   string
	Sl  []int
	M   map[string]int
	P   *int
	A   [2]string
	St  struct{ X, Y int }
	In  any
	Er  error
}

func main() {
	var r Rec
	v := reflect.ValueOf(&r).Elem()
	v.Field(0).SetBool(true)
	v.Field(1).SetInt(-5)
	v.Field(2).SetInt(127)
	v.Field(3).SetInt(-300)
	v.Field(4).SetInt(1 << 20)
	v.Field(5).SetInt(-1 << 40)
	v.Field(6).SetUint(7)
	v.Field(7).SetUint(255)
	v.Field(8).SetUint(65535)
	v.Field(9).SetUint(1 << 31)
	v.Field(10).SetUint(1 << 63)
	v.Field(11).SetUint(99)
	v.Field(12).SetFloat(1.5)
	v.Field(13).SetFloat(-2.25)
	v.Field(14).SetComplex(1 + 2i)
	v.Field(15).SetComplex(3 - 4i)
	v.Field(16).SetString("hello")
	v.Field(17).Set(reflect.ValueOf([]int{1, 2, 3}))
	v.Field(18).Set(reflect.ValueOf(map[string]int{"a": 1}))
	n := 42
	v.Field(19).Set(reflect.ValueOf(&n))
	v.Field(20).Index(1).SetString("x")
	v.Field(21).Field(1).SetInt(8)
	v.Field(22).Set(reflect.ValueOf("any"))
	v.Field(23).Set(reflect.ValueOf(fmt.Errorf("boom")))
	fmt.Printf("%+v\n", r.St)
	fmt.Println(r.B, r.I, r.I8, r.I16, r.I32, r.I64, r.U, r.U8, r.U16, r.U32, r.U64, r.Up)
	fmt.Println(r.F32, r.F64, r.C64, r.C, r.S, r.Sl, r.M, *r.P, r.A, r.In, r.Er)
	v.Field(2).SetInt(200)
	v.Field(7).SetUint(300)
	fmt.Println(r.I8, r.U8)
}

// Output:
// {X:0 Y:8}
// true -5 127 -300 1048576 -1099511627776 7 255 65535 2147483648 9223372036854775808 99
// 1.5 -2.25 (1+2i) (3-4i) hello [1 2 3] map[a:1] 42 [ x] any boom
// -56 44
