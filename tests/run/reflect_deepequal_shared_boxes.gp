// phase: 09

// DeepEqual of two reflect.Values compares their pointer fields, and Go's conversion to an
// interface shares the box of some values instead of allocating one: a bool and a one-byte
// integer always, a value of 2, 4 or 8 bytes (aligned to its size, without pointers) below 256,
// "" and a slice with a nil array (also as the one field of a struct or the one element of an
// array), and a zero-size value; and a value that Go keeps in the interface word itself (a
// pointer, a map, a func, or a struct or an array of one of them) is its own pointer field (ADR-035
// item 10e). The values are
// computed at run time, so that no row depends on the boxes of constants (which Go shares, and
// gocpp does not).
package main

import (
	"fmt"
	"os"
	"reflect"
)

type small struct{ X int64 }
type pair struct{ A, B int32 }
type half struct{ H int16 }
type bytes2 [2]uint8
type myInt int
type empty struct{}
type strS struct{ S string }
type sliceS struct{ X []int }
type ptrS struct{ P *int }
type ptrPair struct {
	P *int
	X int
}
type mapS struct{ M map[int]int }
type funcA [1]func()
type nested struct {
	E empty
	Z [0]int
}

func eq(name string, a, b any) {
	fmt.Println(name, reflect.DeepEqual(reflect.ValueOf(a), reflect.ValueOf(b)))
}

func main() {
	k := len(os.Args) // 1
	eq("int 1", 1, 1)
	eq("run-time int 1", k, k)
	eq("run-time int 0", k-1, k-1)
	eq("run-time int 255", k+254, k+254)
	eq("run-time int 256", k+255, k+255)
	eq("run-time int -1", k-2, k-2)
	eq("int 1 and 2", k, k+1)
	eq("named int", myInt(k), myInt(k))
	eq("int and named int", k, myInt(k))
	eq("int8 -1", int8(k-2), int8(k-2))
	eq("uint8 200", uint8(k+199), uint8(k+199))
	eq("bool", k == 1, k == 1)
	eq("bool false", k == 2, k == 2)
	eq("int16 200", int16(k+199), int16(k+199))
	eq("int16 300", int16(k+299), int16(k+299))
	eq("int16 -1", int16(k-2), int16(k-2))
	eq("uint32 7", uint32(k+6), uint32(k+6))
	eq("rune 'a'", rune(k+96), rune(k+96))
	eq("rune 'é'", rune(k+232), rune(k+232))
	eq("rune 'ā'", rune(k+256), rune(k+256))
	eq("uintptr 3", uintptr(k+2), uintptr(k+2))
	eq("float64 0", float64(k-1), float64(k-1))
	eq("float64 1", float64(k), float64(k))
	f := float64(k - 1)
	eq("float64 -0", -f, -f)
	eq("float32 0", float32(k-1), float32(k-1))
	eq("complex64 0", complex64(complex(float32(k-1), 0)), complex64(complex(float32(k-1), 0)))
	eq("complex128 0", complex(float64(k-1), 0), complex(float64(k-1), 0))
	s := fmt.Sprint(k)
	eq("empty string", s[:0], s[:0])
	eq("string", s, s)
	var ns []int
	eq("nil slice", ns, ns)
	eq("empty slice", []int{}, []int{})
	eq("struct of int64 1", small{int64(k)}, small{int64(k)})
	eq("struct of int64 300", small{int64(k + 299)}, small{int64(k + 299)})
	eq("struct of two int32", pair{int32(k), 0}, pair{int32(k), 0})
	eq("struct of int16", half{int16(k)}, half{int16(k)})
	eq("array of two bytes", bytes2{uint8(k), 0}, bytes2{uint8(k), 0})
	eq("array of one int64", [1]int64{int64(k)}, [1]int64{int64(k)})
	eq("empty struct", empty{}, empty{})
	eq("zero-size struct", nested{}, nested{})
	eq("zero-length array", [0]string{}, [0]string{})
	eq("empty and nested", empty{}, nested{})
	eq("struct of one empty string", strS{s[:0]}, strS{s[:0]})
	eq("struct of one string", strS{s}, strS{s})
	eq("array of one empty string", [1]string{s[:0]}, [1]string{s[:0]})
	eq("struct of one nil slice", sliceS{ns}, sliceS{ns})
	eq("struct of one non-nil slice", sliceS{[]int{}}, sliceS{[]int{}})
	var np *int
	q := new(int)
	eq("struct of one nil pointer", ptrS{np}, ptrS{np})
	eq("struct of one pointer", ptrS{q}, ptrS{q})
	eq("structs of two pointers", ptrS{q}, ptrS{new(int)})
	eq("struct of a pointer and an int", ptrPair{q, 1}, ptrPair{q, 1})
	eq("array of one pointer", [1]*int{q}, [1]*int{q})
	var nm map[int]int
	eq("struct of one nil map", mapS{nm}, mapS{nm})
	eq("array of one nil func", funcA{}, funcA{})
	var x any = k
	var y any = k
	eq("two interfaces of 1", x, y)
	v := reflect.ValueOf(&x).Elem().Elem()
	w := reflect.ValueOf(&y).Elem().Elem()
	fmt.Println("elem of two interfaces", reflect.DeepEqual(v, w), v.Int(), w.Int())
}

// Output:
// int 1 true
// run-time int 1 true
// run-time int 0 true
// run-time int 255 true
// run-time int 256 false
// run-time int -1 false
// int 1 and 2 false
// named int true
// int and named int false
// int8 -1 true
// uint8 200 true
// bool true
// bool false true
// int16 200 true
// int16 300 false
// int16 -1 false
// uint32 7 true
// rune 'a' true
// rune 'é' true
// rune 'ā' false
// uintptr 3 true
// float64 0 true
// float64 1 false
// float64 -0 false
// float32 0 true
// complex64 0 false
// complex128 0 false
// empty string true
// string false
// nil slice true
// empty slice false
// struct of int64 1 true
// struct of int64 300 false
// struct of two int32 false
// struct of int16 true
// array of two bytes false
// array of one int64 true
// empty struct true
// zero-size struct true
// zero-length array true
// empty and nested false
// struct of one empty string true
// struct of one string false
// array of one empty string true
// struct of one nil slice true
// struct of one non-nil slice false
// struct of one nil pointer true
// struct of one pointer true
// structs of two pointers false
// struct of a pointer and an int false
// array of one pointer true
// struct of one nil map true
// array of one nil func true
// two interfaces of 1 true
// elem of two interfaces true 1 1
