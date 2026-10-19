// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Name string
type Bytes []byte
type Runes []rune
type IDs []int
type Arr [3]int

func main() {
	st, bt, rt := reflect.TypeOf(""), reflect.TypeOf([]byte{}), reflect.TypeOf([]rune{})
	fmt.Println(reflect.ValueOf(65).Convert(st), reflect.ValueOf('é').Convert(st), reflect.ValueOf(int64(-1)).Convert(st).Len(), reflect.ValueOf(uint32(0x1F600)).Convert(st))
	fmt.Println(reflect.ValueOf("héllo").Convert(bt).Interface(), reflect.ValueOf("héllo").Convert(rt).Interface())
	fmt.Println(reflect.ValueOf([]byte("abc")).Convert(st), reflect.ValueOf([]rune("日本")).Convert(st))
	fmt.Println(reflect.ValueOf(Name("n")).Convert(st).Type(), reflect.ValueOf("s").Convert(reflect.TypeOf(Name(""))).Type())
	fmt.Println(reflect.ValueOf("x").Convert(reflect.TypeOf(Bytes{})).Type(), reflect.ValueOf(Bytes("yz")).Convert(st), reflect.ValueOf("ab").Convert(reflect.TypeOf(Runes{})).Type())
	s := []int{1, 2, 3, 4}
	a := reflect.ValueOf(s).Convert(reflect.TypeOf(Arr{}))
	fmt.Println(a.Interface(), a.Type())
	ids := reflect.ValueOf(s).Convert(reflect.TypeOf(IDs{}))
	fmt.Println(ids.Type(), ids.Interface())
	ap := reflect.ValueOf(s).Convert(reflect.TypeOf(&Arr{}))
	fmt.Println(ap.Elem().Interface(), ap.Type())
	ap.Elem().Index(0).SetInt(100)
	fmt.Println(s)
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf([]int{1}).Convert(reflect.TypeOf(Arr{}))
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf("s").Convert(reflect.TypeOf(1))
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(1.5).Convert(st)
	}()
	fmt.Println(reflect.ValueOf([]int{1}).CanConvert(reflect.TypeOf(Arr{})), reflect.ValueOf(s).CanConvert(reflect.TypeOf(Arr{})))
}

// Output:
// A é 3 😀
// [104 195 169 108 108 111] [104 233 108 108 111]
// abc 日本
// string main.Name
// main.Bytes yz main.Runes
// [1 2 3] main.Arr
// main.IDs [1 2 3 4]
// [1 2 3] *main.Arr
// [100 2 3 4]
// recovered: reflect: cannot convert slice with length 1 to array with length 3
// recovered: reflect.Value.Convert: value of type string cannot be converted to type int
// recovered: reflect.Value.Convert: value of type float64 cannot be converted to type string
// false true
