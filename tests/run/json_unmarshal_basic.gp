// phase: 09
// encoding/json Unmarshal into the basic kinds: scalars, strings, slices, arrays, maps, pointers, interfaces, existing values, null.

package main

import (
	"encoding/json"
	"fmt"
	"regexp"
)

type Point struct{ X, Y int }

type Rec struct {
	Name  string
	Tags  []string
	Attrs map[string]int
	P     *Point
	Any   any
	Arr   [3]int
}

type MyInt int
type MyStr string

var addr = regexp.MustCompile(`0x[0-9a-f]{8,}`)

func dump(v any) string { return addr.ReplaceAllString(fmt.Sprintf("%#v", v), "0xADDR") }

func try[T any](label string, in string, init T) {
	v := init
	err := json.Unmarshal([]byte(in), &v)
	if err != nil {
		fmt.Printf("%s: %s error: %v\n", label, dump(v), err)
		return
	}
	fmt.Printf("%s: %s\n", label, dump(v))
}

func main() {
	try("bool true", `true`, false)
	try("bool false", `false`, true)
	try("bool null keeps", `null`, true)
	try("int", `42`, 0)
	try("int negative", `-17`, 0)
	try("int8 ok", `127`, int8(0))
	try("int8 overflow", `128`, int8(0))
	try("int8 underflow", `-129`, int8(0))
	try("uint8 ok", `255`, uint8(0))
	try("uint8 overflow", `256`, uint8(0))
	try("uint negative", `-1`, uint(0))
	try("int64 min", `-9223372036854775808`, int64(0))
	try("int64 overflow", `9223372036854775808`, int64(0))
	try("uint64 max", `18446744073709551615`, uint64(0))
	try("uint64 overflow", `18446744073709551616`, uint64(0))
	try("int from float", `1.5`, 0)
	try("int from exp", `1e2`, 0)
	try("int from string", `"1"`, 0)
	try("float", `3.25`, 0.0)
	try("float exp", `1.5e3`, 0.0)
	try("float neg exp", `-2.5E-3`, 0.0)
	try("float from int", `7`, 0.0)
	try("float32", `0.1`, float32(0))
	try("float32 overflow", `1e40`, float32(0))
	try("float64 overflow", `1e400`, 0.0)
	try("float big", `123456789012345678901234567890`, 0.0)
	try("string", `"hello"`, "")
	try("string null keeps", `null`, "keep")
	try("string from number", `1`, "")
	try("string empty", `""`, "x")
	try("named int", `5`, MyInt(0))
	try("named string", `"s"`, MyStr(""))
	try("slice", `[1,2,3]`, []int(nil))
	try("slice empty", `[]`, []int(nil))
	try("slice null", `null`, []int{1})
	try("slice overwrite", `[9]`, []int{1, 2, 3})
	try("slice of slice", `[[1],[2,3],[]]`, [][]int(nil))
	try("slice wrong type", `{"a":1}`, []int(nil))
	try("slice elem wrong type", `[1,"x",3]`, []int(nil))
	try("array exact", `[1,2,3]`, [3]int{})
	try("array short", `[1]`, [3]int{7, 8, 9})
	try("array long", `[1,2,3,4,5]`, [3]int{})
	try("bytes", `"aGVsbG8="`, []byte(nil))
	try("bytes bad", `"!!"`, []byte(nil))
	try("bytes from array", `[104,105]`, []byte(nil))
	try("bytes empty", `""`, []byte{1})
	try("map", `{"a":1,"b":2}`, map[string]int(nil))
	try("map merge", `{"b":2,"c":3}`, map[string]int{"a": 1, "b": 0})
	try("map null", `null`, map[string]int{"a": 1})
	try("map int keys", `{"1":"a","20":"b","-3":"c"}`, map[int]string(nil))
	try("map bad int key", `{"x":"a"}`, map[int]string(nil))
	try("map uint8 key overflow", `{"300":"a"}`, map[uint8]string(nil))
	try("map named key", `{"k":1}`, map[MyStr]int(nil))
	try("map of slices", `{"a":[1,2],"b":[]}`, map[string][]int(nil))
	try("map of maps", `{"a":{"b":1}}`, map[string]map[string]int(nil))
	try("map value mismatch", `{"a":"x"}`, map[string]int(nil))
	try("pointer", `5`, (*int)(nil))
	try("pointer null", `null`, new(int))
	try("pointer to pointer", `6`, (**int)(nil))
	try("struct", `{"X":1,"Y":2}`, Point{})
	try("struct partial", `{"Y":5}`, Point{1, 2})
	try("struct null", `null`, Point{1, 2})
	try("struct from array", `[1,2]`, Point{})
	try("struct empty", `{}`, Point{3, 4})
	try("full record", `{"Name":"n","Tags":["a","b"],"Attrs":{"k":1},"P":{"X":1,"Y":2},"Any":[1,"x",null,true,{"z":1.5}],"Arr":[1,2,3]}`, Rec{})
	try("record nulls", `{"Name":null,"Tags":null,"Attrs":null,"P":null,"Any":null}`, Rec{Name: "keep", Tags: []string{"k"}, P: &Point{}, Any: 1})
	try("record update", `{"P":{"Y":9}}`, Rec{P: &Point{X: 1, Y: 2}})
	try("interface number", `1`, any(nil))
	try("interface string", `"s"`, any(nil))
	try("interface bool", `true`, any(nil))
	try("interface null", `null`, any(1))
	try("interface array", `[1,"a",null]`, any(nil))
	try("interface object", `{"b":[1,2],"a":{"c":null}}`, any(nil))
	try("interface holding pointer", `{"X":4}`, any(&Point{1, 2}))
	try("interface holding non-pointer", `{"X":4}`, any(Point{1, 2}))
	try("empty interface slice", `[1,2]`, []any(nil))
	try("map any", `{"n":1,"s":"x","a":[true],"o":{}}`, map[string]any(nil))
	try("whitespace", " \t\r\n[ 1 , 2 ]\n ", []int(nil))
	try("big nesting", `[[[[[[[[[[1]]]]]]]]]]`, any(nil))
}

// Output:
// bool true: true
// bool false: false
// bool null keeps: true
// int: 42
// int negative: -17
// int8 ok: 127
// int8 overflow: 0 error: json: cannot unmarshal number 128 into Go value of type int8
// int8 underflow: 0 error: json: cannot unmarshal number -129 into Go value of type int8
// uint8 ok: 0xff
// uint8 overflow: 0x0 error: json: cannot unmarshal number 256 into Go value of type uint8
// uint negative: 0x0 error: json: cannot unmarshal number -1 into Go value of type uint
// int64 min: -9223372036854775808
// int64 overflow: 0 error: json: cannot unmarshal number 9223372036854775808 into Go value of type int64
// uint64 max: 0xADDR
// uint64 overflow: 0x0 error: json: cannot unmarshal number 18446744073709551616 into Go value of type uint64
// int from float: 0 error: json: cannot unmarshal number 1.5 into Go value of type int
// int from exp: 0 error: json: cannot unmarshal number 1e2 into Go value of type int
// int from string: 0 error: json: cannot unmarshal string into Go value of type int
// float: 3.25
// float exp: 1500
// float neg exp: -0.0025
// float from int: 7
// float32: 0.1
// float32 overflow: 0 error: json: cannot unmarshal number 1e40 into Go value of type float32
// float64 overflow: 0 error: json: cannot unmarshal number 1e400 into Go value of type float64
// float big: 1.2345678901234568e+29
// string: "hello"
// string null keeps: "keep"
// string from number: "" error: json: cannot unmarshal number into Go value of type string
// string empty: ""
// named int: 5
// named string: "s"
// slice: []int{1, 2, 3}
// slice empty: []int{}
// slice null: []int(nil)
// slice overwrite: []int{9}
// slice of slice: [][]int{[]int{1}, []int{2, 3}, []int{}}
// slice wrong type: []int(nil) error: json: cannot unmarshal object into Go value of type []int
// slice elem wrong type: []int{1, 0, 3} error: json: cannot unmarshal string into Go value of type int
// array exact: [3]int{1, 2, 3}
// array short: [3]int{1, 0, 0}
// array long: [3]int{1, 2, 3}
// bytes: []byte{0x68, 0x65, 0x6c, 0x6c, 0x6f}
// bytes bad: []byte(nil) error: illegal base64 data at input byte 0
// bytes from array: []byte{0x68, 0x69}
// bytes empty: []byte{}
// map: map[string]int{"a":1, "b":2}
// map merge: map[string]int{"a":1, "b":2, "c":3}
// map null: map[string]int(nil)
// map int keys: map[int]string{-3:"c", 1:"a", 20:"b"}
// map bad int key: map[int]string{} error: json: cannot unmarshal number x into Go value of type int
// map uint8 key overflow: map[uint8]string{} error: json: cannot unmarshal number 300 into Go value of type uint8
// map named key: map[main.MyStr]int{"k":1}
// map of slices: map[string][]int{"a":[]int{1, 2}, "b":[]int{}}
// map of maps: map[string]map[string]int{"a":map[string]int{"b":1}}
// map value mismatch: map[string]int{"a":0} error: json: cannot unmarshal string into Go value of type int
// pointer: (*int)(0xADDR)
// pointer null: (*int)(nil)
// pointer to pointer: (**int)(0xADDR)
// struct: main.Point{X:1, Y:2}
// struct partial: main.Point{X:1, Y:5}
// struct null: main.Point{X:1, Y:2}
// struct from array: main.Point{X:0, Y:0} error: json: cannot unmarshal array into Go value of type main.Point
// struct empty: main.Point{X:3, Y:4}
// full record: main.Rec{Name:"n", Tags:[]string{"a", "b"}, Attrs:map[string]int{"k":1}, P:(*main.Point)(0xADDR), Any:[]interface {}{1, "x", interface {}(nil), true, map[string]interface {}{"z":1.5}}, Arr:[3]int{1, 2, 3}}
// record nulls: main.Rec{Name:"keep", Tags:[]string(nil), Attrs:map[string]int(nil), P:(*main.Point)(nil), Any:interface {}(nil), Arr:[3]int{0, 0, 0}}
// record update: main.Rec{Name:"", Tags:[]string(nil), Attrs:map[string]int(nil), P:(*main.Point)(0xADDR), Any:interface {}(nil), Arr:[3]int{0, 0, 0}}
// interface number: 1
// interface string: "s"
// interface bool: true
// interface null: <nil>
// interface array: []interface {}{1, "a", interface {}(nil)}
// interface object: map[string]interface {}{"a":map[string]interface {}{"c":interface {}(nil)}, "b":[]interface {}{1, 2}}
// interface holding pointer: &main.Point{X:4, Y:2}
// interface holding non-pointer: map[string]interface {}{"X":4}
// empty interface slice: []interface {}{1, 2}
// map any: map[string]interface {}{"a":[]interface {}{true}, "n":1, "o":map[string]interface {}{}, "s":"x"}
// whitespace: []int{1, 2}
// big nesting: []interface {}{[]interface {}{[]interface {}{[]interface {}{[]interface {}{[]interface {}{[]interface {}{[]interface {}{[]interface {}{[]interface {}{1}}}}}}}}}}
