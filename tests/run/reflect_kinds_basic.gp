// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	vals := []any{
		true, int(1), int8(2), int16(3), int32(4), int64(5),
		uint(6), uint8(7), uint16(8), uint32(9), uint64(10), uintptr(11),
		float32(1.5), 2.5, complex64(1 + 2i), complex128(3 + 4i), "str",
	}
	for _, v := range vals {
		t := reflect.TypeOf(v)
		fmt.Printf("%-10s kind=%-10v name=%-10s string=%-10s pkg=%q bits=%d\n", t, t.Kind(), t.Name(), t.String(), t.PkgPath(), bitsOf(t))
	}
	var b byte = 1
	var r rune = 'a'
	fmt.Println(reflect.TypeOf(b), reflect.TypeOf(r), reflect.TypeOf(b) == reflect.TypeOf(uint8(0)), reflect.TypeOf(r) == reflect.TypeOf(int32(0)))
}

func bitsOf(t reflect.Type) int {
	switch t.Kind() {
	case reflect.Bool, reflect.String:
		return -1
	}
	return t.Bits()
}

// Output:
// bool       kind=bool       name=bool       string=bool       pkg="" bits=-1
// int        kind=int        name=int        string=int        pkg="" bits=64
// int8       kind=int8       name=int8       string=int8       pkg="" bits=8
// int16      kind=int16      name=int16      string=int16      pkg="" bits=16
// int32      kind=int32      name=int32      string=int32      pkg="" bits=32
// int64      kind=int64      name=int64      string=int64      pkg="" bits=64
// uint       kind=uint       name=uint       string=uint       pkg="" bits=64
// uint8      kind=uint8      name=uint8      string=uint8      pkg="" bits=8
// uint16     kind=uint16     name=uint16     string=uint16     pkg="" bits=16
// uint32     kind=uint32     name=uint32     string=uint32     pkg="" bits=32
// uint64     kind=uint64     name=uint64     string=uint64     pkg="" bits=64
// uintptr    kind=uintptr    name=uintptr    string=uintptr    pkg="" bits=64
// float32    kind=float32    name=float32    string=float32    pkg="" bits=32
// float64    kind=float64    name=float64    string=float64    pkg="" bits=64
// complex64  kind=complex64  name=complex64  string=complex64  pkg="" bits=64
// complex128 kind=complex128 name=complex128 string=complex128 pkg="" bits=128
// string     kind=string     name=string     string=string     pkg="" bits=-1
// uint8 int32 true true
