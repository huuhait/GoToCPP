// phase: 09
// encoding/json: every JSON value kind decoded into every kind of Go destination: the result,
// the partial value left behind and the UnmarshalTypeError fields (Value, Type, Offset, Struct, Field).

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

var address = regexp.MustCompile(`0x[0-9a-f]{8,}`)

type Inner struct {
	N int
}

type Outer struct {
	Inner
	Name  string
	Items []Inner
	Ptr   *Inner
}

type Celsius float64

type Flags uint8

type Names []string

type Lookup map[string]int

type Named string

var inputs = []string{
	`null`, `true`, `false`, `0`, `-1`, `127`, `128`, `255`, `256`, `1.5`, `1e2`, `-0`, `1e400`,
	`"text"`, `"12"`, `""`, `[]`, `[1,2,3]`, `["a","b"]`, `[1,"x",true]`, `{}`, `{"N":7}`, `{"a":1}`,
	`{"Name":1}`, `{"Items":[{"N":1},{"N":"x"}]}`, `{"Ptr":{"N":"x"},"Name":"after"}`,
}

func describe(err error) string {
	if err == nil {
		return "ok"
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf("type{Value=%q Type=%v Offset=%d Struct=%q Field=%q}", typeErr.Value, typeErr.Type, typeErr.Offset, typeErr.Struct, typeErr.Field)
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Sprintf("syntax{%d %q}", syntax.Offset, syntax.Error())
	}
	return fmt.Sprintf("%T %q", err, err.Error())
}

func decodeInto[T any](label string, seed T) {
	fmt.Println("==", label)
	for _, input := range inputs {
		value := seed
		err := json.Unmarshal([]byte(input), &value)
		fmt.Printf("%-34s => %s | %s\n", input, address.ReplaceAllString(fmt.Sprintf("%#v", value), "0xADDR"), describe(err))
	}
}

func main() {
	decodeInto("bool", false)
	decodeInto("int", 5)
	decodeInto("int8", int8(5))
	decodeInto("int64", int64(5))
	decodeInto("uint8", uint8(5))
	decodeInto("uint64", uint64(5))
	decodeInto("uintptr", uintptr(5))
	decodeInto("float32", float32(5))
	decodeInto("float64", float64(5))
	decodeInto("string", "seed")
	decodeInto("[]int", []int{9, 9})
	decodeInto("[]string", []string{"seed"})
	decodeInto("[2]int", [2]int{9, 9})
	decodeInto("[4]int", [4]int{9, 9, 9, 9})
	decodeInto("[]byte", []byte("seed"))
	decodeInto("[3]byte", [3]byte{1, 2, 3})
	decodeInto("map[string]int", map[string]int{"seed": 1})
	decodeInto("map[string]any", map[string]any{"seed": 1})
	decodeInto("struct Inner", Inner{N: 9})
	decodeInto("struct Outer", Outer{Name: "seed"})
	decodeInto("*int", new(int))
	decodeInto("*Inner nil", (*Inner)(nil))
	decodeInto("**int", func() **int { p := new(int); return &p }())
	decodeInto("any", any("seed"))
	decodeInto("any nil", any(nil))
	decodeInto("[]any", []any{"seed"})
	decodeInto("Celsius", Celsius(5))
	decodeInto("Flags", Flags(5))
	decodeInto("Names", Names{"seed"})
	decodeInto("Lookup", Lookup{"seed": 1})
	decodeInto("Named", Named("seed"))
	decodeInto("json.Number", json.Number("5"))
	decodeInto("json.RawMessage", json.RawMessage("seed"))
	decodeInto("struct{}", struct{}{})
	decodeInto("[]struct", []struct{ A int }{{A: 9}})
	decodeInto("[0]int", [0]int{})
	decodeInto("map[int]string", map[int]string{5: "seed"})
}

// Output:
// == bool
// null                               => false | ok
// true                               => true | ok
// false                              => false | ok
// 0                                  => false | type{Value="number" Type=bool Offset=1 Struct="" Field=""}
// -1                                 => false | type{Value="number" Type=bool Offset=2 Struct="" Field=""}
// 127                                => false | type{Value="number" Type=bool Offset=3 Struct="" Field=""}
// 128                                => false | type{Value="number" Type=bool Offset=3 Struct="" Field=""}
// 255                                => false | type{Value="number" Type=bool Offset=3 Struct="" Field=""}
// 256                                => false | type{Value="number" Type=bool Offset=3 Struct="" Field=""}
// 1.5                                => false | type{Value="number" Type=bool Offset=3 Struct="" Field=""}
// 1e2                                => false | type{Value="number" Type=bool Offset=3 Struct="" Field=""}
// -0                                 => false | type{Value="number" Type=bool Offset=2 Struct="" Field=""}
// 1e400                              => false | type{Value="number" Type=bool Offset=5 Struct="" Field=""}
// "text"                             => false | type{Value="string" Type=bool Offset=6 Struct="" Field=""}
// "12"                               => false | type{Value="string" Type=bool Offset=4 Struct="" Field=""}
// ""                                 => false | type{Value="string" Type=bool Offset=2 Struct="" Field=""}
// []                                 => false | type{Value="array" Type=bool Offset=1 Struct="" Field=""}
// [1,2,3]                            => false | type{Value="array" Type=bool Offset=1 Struct="" Field=""}
// ["a","b"]                          => false | type{Value="array" Type=bool Offset=1 Struct="" Field=""}
// [1,"x",true]                       => false | type{Value="array" Type=bool Offset=1 Struct="" Field=""}
// {}                                 => false | type{Value="object" Type=bool Offset=1 Struct="" Field=""}
// {"N":7}                            => false | type{Value="object" Type=bool Offset=1 Struct="" Field=""}
// {"a":1}                            => false | type{Value="object" Type=bool Offset=1 Struct="" Field=""}
// {"Name":1}                         => false | type{Value="object" Type=bool Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => false | type{Value="object" Type=bool Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => false | type{Value="object" Type=bool Offset=1 Struct="" Field=""}
// == int
// null                               => 5 | ok
// true                               => 5 | type{Value="bool" Type=int Offset=4 Struct="" Field=""}
// false                              => 5 | type{Value="bool" Type=int Offset=5 Struct="" Field=""}
// 0                                  => 0 | ok
// -1                                 => -1 | ok
// 127                                => 127 | ok
// 128                                => 128 | ok
// 255                                => 255 | ok
// 256                                => 256 | ok
// 1.5                                => 5 | type{Value="number 1.5" Type=int Offset=3 Struct="" Field=""}
// 1e2                                => 5 | type{Value="number 1e2" Type=int Offset=3 Struct="" Field=""}
// -0                                 => 0 | ok
// 1e400                              => 5 | type{Value="number 1e400" Type=int Offset=5 Struct="" Field=""}
// "text"                             => 5 | type{Value="string" Type=int Offset=6 Struct="" Field=""}
// "12"                               => 5 | type{Value="string" Type=int Offset=4 Struct="" Field=""}
// ""                                 => 5 | type{Value="string" Type=int Offset=2 Struct="" Field=""}
// []                                 => 5 | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// [1,2,3]                            => 5 | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// ["a","b"]                          => 5 | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 5 | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// {}                                 => 5 | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"N":7}                            => 5 | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"a":1}                            => 5 | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Name":1}                         => 5 | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 5 | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 5 | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// == int8
// null                               => 5 | ok
// true                               => 5 | type{Value="bool" Type=int8 Offset=4 Struct="" Field=""}
// false                              => 5 | type{Value="bool" Type=int8 Offset=5 Struct="" Field=""}
// 0                                  => 0 | ok
// -1                                 => -1 | ok
// 127                                => 127 | ok
// 128                                => 5 | type{Value="number 128" Type=int8 Offset=3 Struct="" Field=""}
// 255                                => 5 | type{Value="number 255" Type=int8 Offset=3 Struct="" Field=""}
// 256                                => 5 | type{Value="number 256" Type=int8 Offset=3 Struct="" Field=""}
// 1.5                                => 5 | type{Value="number 1.5" Type=int8 Offset=3 Struct="" Field=""}
// 1e2                                => 5 | type{Value="number 1e2" Type=int8 Offset=3 Struct="" Field=""}
// -0                                 => 0 | ok
// 1e400                              => 5 | type{Value="number 1e400" Type=int8 Offset=5 Struct="" Field=""}
// "text"                             => 5 | type{Value="string" Type=int8 Offset=6 Struct="" Field=""}
// "12"                               => 5 | type{Value="string" Type=int8 Offset=4 Struct="" Field=""}
// ""                                 => 5 | type{Value="string" Type=int8 Offset=2 Struct="" Field=""}
// []                                 => 5 | type{Value="array" Type=int8 Offset=1 Struct="" Field=""}
// [1,2,3]                            => 5 | type{Value="array" Type=int8 Offset=1 Struct="" Field=""}
// ["a","b"]                          => 5 | type{Value="array" Type=int8 Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 5 | type{Value="array" Type=int8 Offset=1 Struct="" Field=""}
// {}                                 => 5 | type{Value="object" Type=int8 Offset=1 Struct="" Field=""}
// {"N":7}                            => 5 | type{Value="object" Type=int8 Offset=1 Struct="" Field=""}
// {"a":1}                            => 5 | type{Value="object" Type=int8 Offset=1 Struct="" Field=""}
// {"Name":1}                         => 5 | type{Value="object" Type=int8 Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 5 | type{Value="object" Type=int8 Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 5 | type{Value="object" Type=int8 Offset=1 Struct="" Field=""}
// == int64
// null                               => 5 | ok
// true                               => 5 | type{Value="bool" Type=int64 Offset=4 Struct="" Field=""}
// false                              => 5 | type{Value="bool" Type=int64 Offset=5 Struct="" Field=""}
// 0                                  => 0 | ok
// -1                                 => -1 | ok
// 127                                => 127 | ok
// 128                                => 128 | ok
// 255                                => 255 | ok
// 256                                => 256 | ok
// 1.5                                => 5 | type{Value="number 1.5" Type=int64 Offset=3 Struct="" Field=""}
// 1e2                                => 5 | type{Value="number 1e2" Type=int64 Offset=3 Struct="" Field=""}
// -0                                 => 0 | ok
// 1e400                              => 5 | type{Value="number 1e400" Type=int64 Offset=5 Struct="" Field=""}
// "text"                             => 5 | type{Value="string" Type=int64 Offset=6 Struct="" Field=""}
// "12"                               => 5 | type{Value="string" Type=int64 Offset=4 Struct="" Field=""}
// ""                                 => 5 | type{Value="string" Type=int64 Offset=2 Struct="" Field=""}
// []                                 => 5 | type{Value="array" Type=int64 Offset=1 Struct="" Field=""}
// [1,2,3]                            => 5 | type{Value="array" Type=int64 Offset=1 Struct="" Field=""}
// ["a","b"]                          => 5 | type{Value="array" Type=int64 Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 5 | type{Value="array" Type=int64 Offset=1 Struct="" Field=""}
// {}                                 => 5 | type{Value="object" Type=int64 Offset=1 Struct="" Field=""}
// {"N":7}                            => 5 | type{Value="object" Type=int64 Offset=1 Struct="" Field=""}
// {"a":1}                            => 5 | type{Value="object" Type=int64 Offset=1 Struct="" Field=""}
// {"Name":1}                         => 5 | type{Value="object" Type=int64 Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 5 | type{Value="object" Type=int64 Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 5 | type{Value="object" Type=int64 Offset=1 Struct="" Field=""}
// == uint8
// null                               => 0x5 | ok
// true                               => 0x5 | type{Value="bool" Type=uint8 Offset=4 Struct="" Field=""}
// false                              => 0x5 | type{Value="bool" Type=uint8 Offset=5 Struct="" Field=""}
// 0                                  => 0x0 | ok
// -1                                 => 0x5 | type{Value="number -1" Type=uint8 Offset=2 Struct="" Field=""}
// 127                                => 0x7f | ok
// 128                                => 0x80 | ok
// 255                                => 0xff | ok
// 256                                => 0x5 | type{Value="number 256" Type=uint8 Offset=3 Struct="" Field=""}
// 1.5                                => 0x5 | type{Value="number 1.5" Type=uint8 Offset=3 Struct="" Field=""}
// 1e2                                => 0x5 | type{Value="number 1e2" Type=uint8 Offset=3 Struct="" Field=""}
// -0                                 => 0x5 | type{Value="number -0" Type=uint8 Offset=2 Struct="" Field=""}
// 1e400                              => 0x5 | type{Value="number 1e400" Type=uint8 Offset=5 Struct="" Field=""}
// "text"                             => 0x5 | type{Value="string" Type=uint8 Offset=6 Struct="" Field=""}
// "12"                               => 0x5 | type{Value="string" Type=uint8 Offset=4 Struct="" Field=""}
// ""                                 => 0x5 | type{Value="string" Type=uint8 Offset=2 Struct="" Field=""}
// []                                 => 0x5 | type{Value="array" Type=uint8 Offset=1 Struct="" Field=""}
// [1,2,3]                            => 0x5 | type{Value="array" Type=uint8 Offset=1 Struct="" Field=""}
// ["a","b"]                          => 0x5 | type{Value="array" Type=uint8 Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 0x5 | type{Value="array" Type=uint8 Offset=1 Struct="" Field=""}
// {}                                 => 0x5 | type{Value="object" Type=uint8 Offset=1 Struct="" Field=""}
// {"N":7}                            => 0x5 | type{Value="object" Type=uint8 Offset=1 Struct="" Field=""}
// {"a":1}                            => 0x5 | type{Value="object" Type=uint8 Offset=1 Struct="" Field=""}
// {"Name":1}                         => 0x5 | type{Value="object" Type=uint8 Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 0x5 | type{Value="object" Type=uint8 Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 0x5 | type{Value="object" Type=uint8 Offset=1 Struct="" Field=""}
// == uint64
// null                               => 0x5 | ok
// true                               => 0x5 | type{Value="bool" Type=uint64 Offset=4 Struct="" Field=""}
// false                              => 0x5 | type{Value="bool" Type=uint64 Offset=5 Struct="" Field=""}
// 0                                  => 0x0 | ok
// -1                                 => 0x5 | type{Value="number -1" Type=uint64 Offset=2 Struct="" Field=""}
// 127                                => 0x7f | ok
// 128                                => 0x80 | ok
// 255                                => 0xff | ok
// 256                                => 0x100 | ok
// 1.5                                => 0x5 | type{Value="number 1.5" Type=uint64 Offset=3 Struct="" Field=""}
// 1e2                                => 0x5 | type{Value="number 1e2" Type=uint64 Offset=3 Struct="" Field=""}
// -0                                 => 0x5 | type{Value="number -0" Type=uint64 Offset=2 Struct="" Field=""}
// 1e400                              => 0x5 | type{Value="number 1e400" Type=uint64 Offset=5 Struct="" Field=""}
// "text"                             => 0x5 | type{Value="string" Type=uint64 Offset=6 Struct="" Field=""}
// "12"                               => 0x5 | type{Value="string" Type=uint64 Offset=4 Struct="" Field=""}
// ""                                 => 0x5 | type{Value="string" Type=uint64 Offset=2 Struct="" Field=""}
// []                                 => 0x5 | type{Value="array" Type=uint64 Offset=1 Struct="" Field=""}
// [1,2,3]                            => 0x5 | type{Value="array" Type=uint64 Offset=1 Struct="" Field=""}
// ["a","b"]                          => 0x5 | type{Value="array" Type=uint64 Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 0x5 | type{Value="array" Type=uint64 Offset=1 Struct="" Field=""}
// {}                                 => 0x5 | type{Value="object" Type=uint64 Offset=1 Struct="" Field=""}
// {"N":7}                            => 0x5 | type{Value="object" Type=uint64 Offset=1 Struct="" Field=""}
// {"a":1}                            => 0x5 | type{Value="object" Type=uint64 Offset=1 Struct="" Field=""}
// {"Name":1}                         => 0x5 | type{Value="object" Type=uint64 Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 0x5 | type{Value="object" Type=uint64 Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 0x5 | type{Value="object" Type=uint64 Offset=1 Struct="" Field=""}
// == uintptr
// null                               => 0x5 | ok
// true                               => 0x5 | type{Value="bool" Type=uintptr Offset=4 Struct="" Field=""}
// false                              => 0x5 | type{Value="bool" Type=uintptr Offset=5 Struct="" Field=""}
// 0                                  => 0x0 | ok
// -1                                 => 0x5 | type{Value="number -1" Type=uintptr Offset=2 Struct="" Field=""}
// 127                                => 0x7f | ok
// 128                                => 0x80 | ok
// 255                                => 0xff | ok
// 256                                => 0x100 | ok
// 1.5                                => 0x5 | type{Value="number 1.5" Type=uintptr Offset=3 Struct="" Field=""}
// 1e2                                => 0x5 | type{Value="number 1e2" Type=uintptr Offset=3 Struct="" Field=""}
// -0                                 => 0x5 | type{Value="number -0" Type=uintptr Offset=2 Struct="" Field=""}
// 1e400                              => 0x5 | type{Value="number 1e400" Type=uintptr Offset=5 Struct="" Field=""}
// "text"                             => 0x5 | type{Value="string" Type=uintptr Offset=6 Struct="" Field=""}
// "12"                               => 0x5 | type{Value="string" Type=uintptr Offset=4 Struct="" Field=""}
// ""                                 => 0x5 | type{Value="string" Type=uintptr Offset=2 Struct="" Field=""}
// []                                 => 0x5 | type{Value="array" Type=uintptr Offset=1 Struct="" Field=""}
// [1,2,3]                            => 0x5 | type{Value="array" Type=uintptr Offset=1 Struct="" Field=""}
// ["a","b"]                          => 0x5 | type{Value="array" Type=uintptr Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 0x5 | type{Value="array" Type=uintptr Offset=1 Struct="" Field=""}
// {}                                 => 0x5 | type{Value="object" Type=uintptr Offset=1 Struct="" Field=""}
// {"N":7}                            => 0x5 | type{Value="object" Type=uintptr Offset=1 Struct="" Field=""}
// {"a":1}                            => 0x5 | type{Value="object" Type=uintptr Offset=1 Struct="" Field=""}
// {"Name":1}                         => 0x5 | type{Value="object" Type=uintptr Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 0x5 | type{Value="object" Type=uintptr Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 0x5 | type{Value="object" Type=uintptr Offset=1 Struct="" Field=""}
// == float32
// null                               => 5 | ok
// true                               => 5 | type{Value="bool" Type=float32 Offset=4 Struct="" Field=""}
// false                              => 5 | type{Value="bool" Type=float32 Offset=5 Struct="" Field=""}
// 0                                  => 0 | ok
// -1                                 => -1 | ok
// 127                                => 127 | ok
// 128                                => 128 | ok
// 255                                => 255 | ok
// 256                                => 256 | ok
// 1.5                                => 1.5 | ok
// 1e2                                => 100 | ok
// -0                                 => -0 | ok
// 1e400                              => 5 | type{Value="number 1e400" Type=float32 Offset=5 Struct="" Field=""}
// "text"                             => 5 | type{Value="string" Type=float32 Offset=6 Struct="" Field=""}
// "12"                               => 5 | type{Value="string" Type=float32 Offset=4 Struct="" Field=""}
// ""                                 => 5 | type{Value="string" Type=float32 Offset=2 Struct="" Field=""}
// []                                 => 5 | type{Value="array" Type=float32 Offset=1 Struct="" Field=""}
// [1,2,3]                            => 5 | type{Value="array" Type=float32 Offset=1 Struct="" Field=""}
// ["a","b"]                          => 5 | type{Value="array" Type=float32 Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 5 | type{Value="array" Type=float32 Offset=1 Struct="" Field=""}
// {}                                 => 5 | type{Value="object" Type=float32 Offset=1 Struct="" Field=""}
// {"N":7}                            => 5 | type{Value="object" Type=float32 Offset=1 Struct="" Field=""}
// {"a":1}                            => 5 | type{Value="object" Type=float32 Offset=1 Struct="" Field=""}
// {"Name":1}                         => 5 | type{Value="object" Type=float32 Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 5 | type{Value="object" Type=float32 Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 5 | type{Value="object" Type=float32 Offset=1 Struct="" Field=""}
// == float64
// null                               => 5 | ok
// true                               => 5 | type{Value="bool" Type=float64 Offset=4 Struct="" Field=""}
// false                              => 5 | type{Value="bool" Type=float64 Offset=5 Struct="" Field=""}
// 0                                  => 0 | ok
// -1                                 => -1 | ok
// 127                                => 127 | ok
// 128                                => 128 | ok
// 255                                => 255 | ok
// 256                                => 256 | ok
// 1.5                                => 1.5 | ok
// 1e2                                => 100 | ok
// -0                                 => -0 | ok
// 1e400                              => 5 | type{Value="number 1e400" Type=float64 Offset=5 Struct="" Field=""}
// "text"                             => 5 | type{Value="string" Type=float64 Offset=6 Struct="" Field=""}
// "12"                               => 5 | type{Value="string" Type=float64 Offset=4 Struct="" Field=""}
// ""                                 => 5 | type{Value="string" Type=float64 Offset=2 Struct="" Field=""}
// []                                 => 5 | type{Value="array" Type=float64 Offset=1 Struct="" Field=""}
// [1,2,3]                            => 5 | type{Value="array" Type=float64 Offset=1 Struct="" Field=""}
// ["a","b"]                          => 5 | type{Value="array" Type=float64 Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 5 | type{Value="array" Type=float64 Offset=1 Struct="" Field=""}
// {}                                 => 5 | type{Value="object" Type=float64 Offset=1 Struct="" Field=""}
// {"N":7}                            => 5 | type{Value="object" Type=float64 Offset=1 Struct="" Field=""}
// {"a":1}                            => 5 | type{Value="object" Type=float64 Offset=1 Struct="" Field=""}
// {"Name":1}                         => 5 | type{Value="object" Type=float64 Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 5 | type{Value="object" Type=float64 Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 5 | type{Value="object" Type=float64 Offset=1 Struct="" Field=""}
// == string
// null                               => "seed" | ok
// true                               => "seed" | type{Value="bool" Type=string Offset=4 Struct="" Field=""}
// false                              => "seed" | type{Value="bool" Type=string Offset=5 Struct="" Field=""}
// 0                                  => "seed" | type{Value="number" Type=string Offset=1 Struct="" Field=""}
// -1                                 => "seed" | type{Value="number" Type=string Offset=2 Struct="" Field=""}
// 127                                => "seed" | type{Value="number" Type=string Offset=3 Struct="" Field=""}
// 128                                => "seed" | type{Value="number" Type=string Offset=3 Struct="" Field=""}
// 255                                => "seed" | type{Value="number" Type=string Offset=3 Struct="" Field=""}
// 256                                => "seed" | type{Value="number" Type=string Offset=3 Struct="" Field=""}
// 1.5                                => "seed" | type{Value="number" Type=string Offset=3 Struct="" Field=""}
// 1e2                                => "seed" | type{Value="number" Type=string Offset=3 Struct="" Field=""}
// -0                                 => "seed" | type{Value="number" Type=string Offset=2 Struct="" Field=""}
// 1e400                              => "seed" | type{Value="number" Type=string Offset=5 Struct="" Field=""}
// "text"                             => "text" | ok
// "12"                               => "12" | ok
// ""                                 => "" | ok
// []                                 => "seed" | type{Value="array" Type=string Offset=1 Struct="" Field=""}
// [1,2,3]                            => "seed" | type{Value="array" Type=string Offset=1 Struct="" Field=""}
// ["a","b"]                          => "seed" | type{Value="array" Type=string Offset=1 Struct="" Field=""}
// [1,"x",true]                       => "seed" | type{Value="array" Type=string Offset=1 Struct="" Field=""}
// {}                                 => "seed" | type{Value="object" Type=string Offset=1 Struct="" Field=""}
// {"N":7}                            => "seed" | type{Value="object" Type=string Offset=1 Struct="" Field=""}
// {"a":1}                            => "seed" | type{Value="object" Type=string Offset=1 Struct="" Field=""}
// {"Name":1}                         => "seed" | type{Value="object" Type=string Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => "seed" | type{Value="object" Type=string Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => "seed" | type{Value="object" Type=string Offset=1 Struct="" Field=""}
// == []int
// null                               => []int(nil) | ok
// true                               => []int{9, 9} | type{Value="bool" Type=[]int Offset=4 Struct="" Field=""}
// false                              => []int{9, 9} | type{Value="bool" Type=[]int Offset=5 Struct="" Field=""}
// 0                                  => []int{9, 9} | type{Value="number" Type=[]int Offset=1 Struct="" Field=""}
// -1                                 => []int{9, 9} | type{Value="number" Type=[]int Offset=2 Struct="" Field=""}
// 127                                => []int{9, 9} | type{Value="number" Type=[]int Offset=3 Struct="" Field=""}
// 128                                => []int{9, 9} | type{Value="number" Type=[]int Offset=3 Struct="" Field=""}
// 255                                => []int{9, 9} | type{Value="number" Type=[]int Offset=3 Struct="" Field=""}
// 256                                => []int{9, 9} | type{Value="number" Type=[]int Offset=3 Struct="" Field=""}
// 1.5                                => []int{9, 9} | type{Value="number" Type=[]int Offset=3 Struct="" Field=""}
// 1e2                                => []int{9, 9} | type{Value="number" Type=[]int Offset=3 Struct="" Field=""}
// -0                                 => []int{9, 9} | type{Value="number" Type=[]int Offset=2 Struct="" Field=""}
// 1e400                              => []int{9, 9} | type{Value="number" Type=[]int Offset=5 Struct="" Field=""}
// "text"                             => []int{9, 9} | type{Value="string" Type=[]int Offset=6 Struct="" Field=""}
// "12"                               => []int{9, 9} | type{Value="string" Type=[]int Offset=4 Struct="" Field=""}
// ""                                 => []int{9, 9} | type{Value="string" Type=[]int Offset=2 Struct="" Field=""}
// []                                 => []int{} | ok
// [1,2,3]                            => []int{1, 2, 3} | ok
// ["a","b"]                          => []int{1, 2} | type{Value="string" Type=int Offset=4 Struct="" Field=""}
// [1,"x",true]                       => []int{1, 2, 0} | type{Value="string" Type=int Offset=6 Struct="" Field=""}
// {}                                 => []int{1, 2} | type{Value="object" Type=[]int Offset=1 Struct="" Field=""}
// {"N":7}                            => []int{1, 2} | type{Value="object" Type=[]int Offset=1 Struct="" Field=""}
// {"a":1}                            => []int{1, 2} | type{Value="object" Type=[]int Offset=1 Struct="" Field=""}
// {"Name":1}                         => []int{1, 2} | type{Value="object" Type=[]int Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => []int{1, 2} | type{Value="object" Type=[]int Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => []int{1, 2} | type{Value="object" Type=[]int Offset=1 Struct="" Field=""}
// == []string
// null                               => []string(nil) | ok
// true                               => []string{"seed"} | type{Value="bool" Type=[]string Offset=4 Struct="" Field=""}
// false                              => []string{"seed"} | type{Value="bool" Type=[]string Offset=5 Struct="" Field=""}
// 0                                  => []string{"seed"} | type{Value="number" Type=[]string Offset=1 Struct="" Field=""}
// -1                                 => []string{"seed"} | type{Value="number" Type=[]string Offset=2 Struct="" Field=""}
// 127                                => []string{"seed"} | type{Value="number" Type=[]string Offset=3 Struct="" Field=""}
// 128                                => []string{"seed"} | type{Value="number" Type=[]string Offset=3 Struct="" Field=""}
// 255                                => []string{"seed"} | type{Value="number" Type=[]string Offset=3 Struct="" Field=""}
// 256                                => []string{"seed"} | type{Value="number" Type=[]string Offset=3 Struct="" Field=""}
// 1.5                                => []string{"seed"} | type{Value="number" Type=[]string Offset=3 Struct="" Field=""}
// 1e2                                => []string{"seed"} | type{Value="number" Type=[]string Offset=3 Struct="" Field=""}
// -0                                 => []string{"seed"} | type{Value="number" Type=[]string Offset=2 Struct="" Field=""}
// 1e400                              => []string{"seed"} | type{Value="number" Type=[]string Offset=5 Struct="" Field=""}
// "text"                             => []string{"seed"} | type{Value="string" Type=[]string Offset=6 Struct="" Field=""}
// "12"                               => []string{"seed"} | type{Value="string" Type=[]string Offset=4 Struct="" Field=""}
// ""                                 => []string{"seed"} | type{Value="string" Type=[]string Offset=2 Struct="" Field=""}
// []                                 => []string{} | ok
// [1,2,3]                            => []string{"seed", "", ""} | type{Value="number" Type=string Offset=2 Struct="" Field=""}
// ["a","b"]                          => []string{"a", "b"} | ok
// [1,"x",true]                       => []string{"a", "x", ""} | type{Value="number" Type=string Offset=2 Struct="" Field=""}
// {}                                 => []string{"a"} | type{Value="object" Type=[]string Offset=1 Struct="" Field=""}
// {"N":7}                            => []string{"a"} | type{Value="object" Type=[]string Offset=1 Struct="" Field=""}
// {"a":1}                            => []string{"a"} | type{Value="object" Type=[]string Offset=1 Struct="" Field=""}
// {"Name":1}                         => []string{"a"} | type{Value="object" Type=[]string Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => []string{"a"} | type{Value="object" Type=[]string Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => []string{"a"} | type{Value="object" Type=[]string Offset=1 Struct="" Field=""}
// == [2]int
// null                               => [2]int{9, 9} | ok
// true                               => [2]int{9, 9} | type{Value="bool" Type=[2]int Offset=4 Struct="" Field=""}
// false                              => [2]int{9, 9} | type{Value="bool" Type=[2]int Offset=5 Struct="" Field=""}
// 0                                  => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=1 Struct="" Field=""}
// -1                                 => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=2 Struct="" Field=""}
// 127                                => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=3 Struct="" Field=""}
// 128                                => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=3 Struct="" Field=""}
// 255                                => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=3 Struct="" Field=""}
// 256                                => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=3 Struct="" Field=""}
// 1.5                                => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=3 Struct="" Field=""}
// 1e2                                => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=3 Struct="" Field=""}
// -0                                 => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=2 Struct="" Field=""}
// 1e400                              => [2]int{9, 9} | type{Value="number" Type=[2]int Offset=5 Struct="" Field=""}
// "text"                             => [2]int{9, 9} | type{Value="string" Type=[2]int Offset=6 Struct="" Field=""}
// "12"                               => [2]int{9, 9} | type{Value="string" Type=[2]int Offset=4 Struct="" Field=""}
// ""                                 => [2]int{9, 9} | type{Value="string" Type=[2]int Offset=2 Struct="" Field=""}
// []                                 => [2]int{0, 0} | ok
// [1,2,3]                            => [2]int{1, 2} | ok
// ["a","b"]                          => [2]int{9, 9} | type{Value="string" Type=int Offset=4 Struct="" Field=""}
// [1,"x",true]                       => [2]int{1, 9} | type{Value="string" Type=int Offset=6 Struct="" Field=""}
// {}                                 => [2]int{9, 9} | type{Value="object" Type=[2]int Offset=1 Struct="" Field=""}
// {"N":7}                            => [2]int{9, 9} | type{Value="object" Type=[2]int Offset=1 Struct="" Field=""}
// {"a":1}                            => [2]int{9, 9} | type{Value="object" Type=[2]int Offset=1 Struct="" Field=""}
// {"Name":1}                         => [2]int{9, 9} | type{Value="object" Type=[2]int Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => [2]int{9, 9} | type{Value="object" Type=[2]int Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => [2]int{9, 9} | type{Value="object" Type=[2]int Offset=1 Struct="" Field=""}
// == [4]int
// null                               => [4]int{9, 9, 9, 9} | ok
// true                               => [4]int{9, 9, 9, 9} | type{Value="bool" Type=[4]int Offset=4 Struct="" Field=""}
// false                              => [4]int{9, 9, 9, 9} | type{Value="bool" Type=[4]int Offset=5 Struct="" Field=""}
// 0                                  => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=1 Struct="" Field=""}
// -1                                 => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=2 Struct="" Field=""}
// 127                                => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=3 Struct="" Field=""}
// 128                                => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=3 Struct="" Field=""}
// 255                                => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=3 Struct="" Field=""}
// 256                                => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=3 Struct="" Field=""}
// 1.5                                => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=3 Struct="" Field=""}
// 1e2                                => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=3 Struct="" Field=""}
// -0                                 => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=2 Struct="" Field=""}
// 1e400                              => [4]int{9, 9, 9, 9} | type{Value="number" Type=[4]int Offset=5 Struct="" Field=""}
// "text"                             => [4]int{9, 9, 9, 9} | type{Value="string" Type=[4]int Offset=6 Struct="" Field=""}
// "12"                               => [4]int{9, 9, 9, 9} | type{Value="string" Type=[4]int Offset=4 Struct="" Field=""}
// ""                                 => [4]int{9, 9, 9, 9} | type{Value="string" Type=[4]int Offset=2 Struct="" Field=""}
// []                                 => [4]int{0, 0, 0, 0} | ok
// [1,2,3]                            => [4]int{1, 2, 3, 0} | ok
// ["a","b"]                          => [4]int{9, 9, 0, 0} | type{Value="string" Type=int Offset=4 Struct="" Field=""}
// [1,"x",true]                       => [4]int{1, 9, 9, 0} | type{Value="string" Type=int Offset=6 Struct="" Field=""}
// {}                                 => [4]int{9, 9, 9, 9} | type{Value="object" Type=[4]int Offset=1 Struct="" Field=""}
// {"N":7}                            => [4]int{9, 9, 9, 9} | type{Value="object" Type=[4]int Offset=1 Struct="" Field=""}
// {"a":1}                            => [4]int{9, 9, 9, 9} | type{Value="object" Type=[4]int Offset=1 Struct="" Field=""}
// {"Name":1}                         => [4]int{9, 9, 9, 9} | type{Value="object" Type=[4]int Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => [4]int{9, 9, 9, 9} | type{Value="object" Type=[4]int Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => [4]int{9, 9, 9, 9} | type{Value="object" Type=[4]int Offset=1 Struct="" Field=""}
// == []byte
// null                               => []byte(nil) | ok
// true                               => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="bool" Type=[]uint8 Offset=4 Struct="" Field=""}
// false                              => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="bool" Type=[]uint8 Offset=5 Struct="" Field=""}
// 0                                  => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=1 Struct="" Field=""}
// -1                                 => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=2 Struct="" Field=""}
// 127                                => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=3 Struct="" Field=""}
// 128                                => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=3 Struct="" Field=""}
// 255                                => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=3 Struct="" Field=""}
// 256                                => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=3 Struct="" Field=""}
// 1.5                                => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=3 Struct="" Field=""}
// 1e2                                => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=3 Struct="" Field=""}
// -0                                 => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=2 Struct="" Field=""}
// 1e400                              => []byte{0x73, 0x65, 0x65, 0x64} | type{Value="number" Type=[]uint8 Offset=5 Struct="" Field=""}
// "text"                             => []byte{0xb5, 0xec, 0x6d} | ok
// "12"                               => []byte{0x73, 0x65, 0x65, 0x64} | base64.CorruptInputError "illegal base64 data at input byte 0"
// ""                                 => []byte{} | ok
// []                                 => []byte{} | ok
// [1,2,3]                            => []byte{0x1, 0x2, 0x3} | ok
// ["a","b"]                          => []byte{0x1, 0x2} | type{Value="string" Type=uint8 Offset=4 Struct="" Field=""}
// [1,"x",true]                       => []byte{0x1, 0x2, 0x3} | type{Value="string" Type=uint8 Offset=6 Struct="" Field=""}
// {}                                 => []byte{0x1, 0x2, 0x3, 0x64} | type{Value="object" Type=[]uint8 Offset=1 Struct="" Field=""}
// {"N":7}                            => []byte{0x1, 0x2, 0x3, 0x64} | type{Value="object" Type=[]uint8 Offset=1 Struct="" Field=""}
// {"a":1}                            => []byte{0x1, 0x2, 0x3, 0x64} | type{Value="object" Type=[]uint8 Offset=1 Struct="" Field=""}
// {"Name":1}                         => []byte{0x1, 0x2, 0x3, 0x64} | type{Value="object" Type=[]uint8 Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => []byte{0x1, 0x2, 0x3, 0x64} | type{Value="object" Type=[]uint8 Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => []byte{0x1, 0x2, 0x3, 0x64} | type{Value="object" Type=[]uint8 Offset=1 Struct="" Field=""}
// == [3]byte
// null                               => [3]uint8{0x1, 0x2, 0x3} | ok
// true                               => [3]uint8{0x1, 0x2, 0x3} | type{Value="bool" Type=[3]uint8 Offset=4 Struct="" Field=""}
// false                              => [3]uint8{0x1, 0x2, 0x3} | type{Value="bool" Type=[3]uint8 Offset=5 Struct="" Field=""}
// 0                                  => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=1 Struct="" Field=""}
// -1                                 => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=2 Struct="" Field=""}
// 127                                => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=3 Struct="" Field=""}
// 128                                => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=3 Struct="" Field=""}
// 255                                => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=3 Struct="" Field=""}
// 256                                => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=3 Struct="" Field=""}
// 1.5                                => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=3 Struct="" Field=""}
// 1e2                                => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=3 Struct="" Field=""}
// -0                                 => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=2 Struct="" Field=""}
// 1e400                              => [3]uint8{0x1, 0x2, 0x3} | type{Value="number" Type=[3]uint8 Offset=5 Struct="" Field=""}
// "text"                             => [3]uint8{0x1, 0x2, 0x3} | type{Value="string" Type=[3]uint8 Offset=6 Struct="" Field=""}
// "12"                               => [3]uint8{0x1, 0x2, 0x3} | type{Value="string" Type=[3]uint8 Offset=4 Struct="" Field=""}
// ""                                 => [3]uint8{0x1, 0x2, 0x3} | type{Value="string" Type=[3]uint8 Offset=2 Struct="" Field=""}
// []                                 => [3]uint8{0x0, 0x0, 0x0} | ok
// [1,2,3]                            => [3]uint8{0x1, 0x2, 0x3} | ok
// ["a","b"]                          => [3]uint8{0x1, 0x2, 0x0} | type{Value="string" Type=uint8 Offset=4 Struct="" Field=""}
// [1,"x",true]                       => [3]uint8{0x1, 0x2, 0x3} | type{Value="string" Type=uint8 Offset=6 Struct="" Field=""}
// {}                                 => [3]uint8{0x1, 0x2, 0x3} | type{Value="object" Type=[3]uint8 Offset=1 Struct="" Field=""}
// {"N":7}                            => [3]uint8{0x1, 0x2, 0x3} | type{Value="object" Type=[3]uint8 Offset=1 Struct="" Field=""}
// {"a":1}                            => [3]uint8{0x1, 0x2, 0x3} | type{Value="object" Type=[3]uint8 Offset=1 Struct="" Field=""}
// {"Name":1}                         => [3]uint8{0x1, 0x2, 0x3} | type{Value="object" Type=[3]uint8 Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => [3]uint8{0x1, 0x2, 0x3} | type{Value="object" Type=[3]uint8 Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => [3]uint8{0x1, 0x2, 0x3} | type{Value="object" Type=[3]uint8 Offset=1 Struct="" Field=""}
// == map[string]int
// null                               => map[string]int(nil) | ok
// true                               => map[string]int{"seed":1} | type{Value="bool" Type=map[string]int Offset=4 Struct="" Field=""}
// false                              => map[string]int{"seed":1} | type{Value="bool" Type=map[string]int Offset=5 Struct="" Field=""}
// 0                                  => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=1 Struct="" Field=""}
// -1                                 => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=2 Struct="" Field=""}
// 127                                => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=3 Struct="" Field=""}
// 128                                => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=3 Struct="" Field=""}
// 255                                => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=3 Struct="" Field=""}
// 256                                => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=3 Struct="" Field=""}
// 1.5                                => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=3 Struct="" Field=""}
// 1e2                                => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=3 Struct="" Field=""}
// -0                                 => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=2 Struct="" Field=""}
// 1e400                              => map[string]int{"seed":1} | type{Value="number" Type=map[string]int Offset=5 Struct="" Field=""}
// "text"                             => map[string]int{"seed":1} | type{Value="string" Type=map[string]int Offset=6 Struct="" Field=""}
// "12"                               => map[string]int{"seed":1} | type{Value="string" Type=map[string]int Offset=4 Struct="" Field=""}
// ""                                 => map[string]int{"seed":1} | type{Value="string" Type=map[string]int Offset=2 Struct="" Field=""}
// []                                 => map[string]int{"seed":1} | type{Value="array" Type=map[string]int Offset=1 Struct="" Field=""}
// [1,2,3]                            => map[string]int{"seed":1} | type{Value="array" Type=map[string]int Offset=1 Struct="" Field=""}
// ["a","b"]                          => map[string]int{"seed":1} | type{Value="array" Type=map[string]int Offset=1 Struct="" Field=""}
// [1,"x",true]                       => map[string]int{"seed":1} | type{Value="array" Type=map[string]int Offset=1 Struct="" Field=""}
// {}                                 => map[string]int{"seed":1} | ok
// {"N":7}                            => map[string]int{"N":7, "seed":1} | ok
// {"a":1}                            => map[string]int{"N":7, "a":1, "seed":1} | ok
// {"Name":1}                         => map[string]int{"N":7, "Name":1, "a":1, "seed":1} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => map[string]int{"Items":0, "N":7, "Name":1, "a":1, "seed":1} | type{Value="array" Type=int Offset=10 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => map[string]int{"Items":0, "N":7, "Name":0, "Ptr":0, "a":1, "seed":1} | type{Value="object" Type=int Offset=8 Struct="" Field=""}
// == map[string]any
// null                               => map[string]interface {}(nil) | ok
// true                               => map[string]interface {}{"seed":1} | type{Value="bool" Type=map[string]interface {} Offset=4 Struct="" Field=""}
// false                              => map[string]interface {}{"seed":1} | type{Value="bool" Type=map[string]interface {} Offset=5 Struct="" Field=""}
// 0                                  => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=1 Struct="" Field=""}
// -1                                 => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=2 Struct="" Field=""}
// 127                                => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=3 Struct="" Field=""}
// 128                                => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=3 Struct="" Field=""}
// 255                                => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=3 Struct="" Field=""}
// 256                                => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=3 Struct="" Field=""}
// 1.5                                => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=3 Struct="" Field=""}
// 1e2                                => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=3 Struct="" Field=""}
// -0                                 => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=2 Struct="" Field=""}
// 1e400                              => map[string]interface {}{"seed":1} | type{Value="number" Type=map[string]interface {} Offset=5 Struct="" Field=""}
// "text"                             => map[string]interface {}{"seed":1} | type{Value="string" Type=map[string]interface {} Offset=6 Struct="" Field=""}
// "12"                               => map[string]interface {}{"seed":1} | type{Value="string" Type=map[string]interface {} Offset=4 Struct="" Field=""}
// ""                                 => map[string]interface {}{"seed":1} | type{Value="string" Type=map[string]interface {} Offset=2 Struct="" Field=""}
// []                                 => map[string]interface {}{"seed":1} | type{Value="array" Type=map[string]interface {} Offset=1 Struct="" Field=""}
// [1,2,3]                            => map[string]interface {}{"seed":1} | type{Value="array" Type=map[string]interface {} Offset=1 Struct="" Field=""}
// ["a","b"]                          => map[string]interface {}{"seed":1} | type{Value="array" Type=map[string]interface {} Offset=1 Struct="" Field=""}
// [1,"x",true]                       => map[string]interface {}{"seed":1} | type{Value="array" Type=map[string]interface {} Offset=1 Struct="" Field=""}
// {}                                 => map[string]interface {}{"seed":1} | ok
// {"N":7}                            => map[string]interface {}{"N":7, "seed":1} | ok
// {"a":1}                            => map[string]interface {}{"N":7, "a":1, "seed":1} | ok
// {"Name":1}                         => map[string]interface {}{"N":7, "Name":1, "a":1, "seed":1} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => map[string]interface {}{"Items":[]interface {}{map[string]interface {}{"N":1}, map[string]interface {}{"N":"x"}}, "N":7, "Name":1, "a":1, "seed":1} | ok
// {"Ptr":{"N":"x"},"Name":"after"}   => map[string]interface {}{"Items":[]interface {}{map[string]interface {}{"N":1}, map[string]interface {}{"N":"x"}}, "N":7, "Name":"after", "Ptr":map[string]interface {}{"N":"x"}, "a":1, "seed":1} | ok
// == struct Inner
// null                               => main.Inner{N:9} | ok
// true                               => main.Inner{N:9} | type{Value="bool" Type=main.Inner Offset=4 Struct="" Field=""}
// false                              => main.Inner{N:9} | type{Value="bool" Type=main.Inner Offset=5 Struct="" Field=""}
// 0                                  => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=1 Struct="" Field=""}
// -1                                 => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=2 Struct="" Field=""}
// 127                                => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 128                                => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 255                                => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 256                                => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 1.5                                => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 1e2                                => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// -0                                 => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=2 Struct="" Field=""}
// 1e400                              => main.Inner{N:9} | type{Value="number" Type=main.Inner Offset=5 Struct="" Field=""}
// "text"                             => main.Inner{N:9} | type{Value="string" Type=main.Inner Offset=6 Struct="" Field=""}
// "12"                               => main.Inner{N:9} | type{Value="string" Type=main.Inner Offset=4 Struct="" Field=""}
// ""                                 => main.Inner{N:9} | type{Value="string" Type=main.Inner Offset=2 Struct="" Field=""}
// []                                 => main.Inner{N:9} | type{Value="array" Type=main.Inner Offset=1 Struct="" Field=""}
// [1,2,3]                            => main.Inner{N:9} | type{Value="array" Type=main.Inner Offset=1 Struct="" Field=""}
// ["a","b"]                          => main.Inner{N:9} | type{Value="array" Type=main.Inner Offset=1 Struct="" Field=""}
// [1,"x",true]                       => main.Inner{N:9} | type{Value="array" Type=main.Inner Offset=1 Struct="" Field=""}
// {}                                 => main.Inner{N:9} | ok
// {"N":7}                            => main.Inner{N:7} | ok
// {"a":1}                            => main.Inner{N:9} | ok
// {"Name":1}                         => main.Inner{N:9} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => main.Inner{N:9} | ok
// {"Ptr":{"N":"x"},"Name":"after"}   => main.Inner{N:9} | ok
// == struct Outer
// null                               => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | ok
// true                               => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="bool" Type=main.Outer Offset=4 Struct="" Field=""}
// false                              => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="bool" Type=main.Outer Offset=5 Struct="" Field=""}
// 0                                  => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=1 Struct="" Field=""}
// -1                                 => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=2 Struct="" Field=""}
// 127                                => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=3 Struct="" Field=""}
// 128                                => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=3 Struct="" Field=""}
// 255                                => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=3 Struct="" Field=""}
// 256                                => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=3 Struct="" Field=""}
// 1.5                                => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=3 Struct="" Field=""}
// 1e2                                => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=3 Struct="" Field=""}
// -0                                 => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=2 Struct="" Field=""}
// 1e400                              => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=main.Outer Offset=5 Struct="" Field=""}
// "text"                             => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="string" Type=main.Outer Offset=6 Struct="" Field=""}
// "12"                               => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="string" Type=main.Outer Offset=4 Struct="" Field=""}
// ""                                 => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="string" Type=main.Outer Offset=2 Struct="" Field=""}
// []                                 => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="array" Type=main.Outer Offset=1 Struct="" Field=""}
// [1,2,3]                            => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="array" Type=main.Outer Offset=1 Struct="" Field=""}
// ["a","b"]                          => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="array" Type=main.Outer Offset=1 Struct="" Field=""}
// [1,"x",true]                       => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="array" Type=main.Outer Offset=1 Struct="" Field=""}
// {}                                 => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | ok
// {"N":7}                            => main.Outer{Inner:main.Inner{N:7}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | ok
// {"a":1}                            => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | ok
// {"Name":1}                         => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner(nil), Ptr:(*main.Inner)(nil)} | type{Value="number" Type=string Offset=9 Struct="Outer" Field="Name"}
// {"Items":[{"N":1},{"N":"x"}]}      => main.Outer{Inner:main.Inner{N:0}, Name:"seed", Items:[]main.Inner{main.Inner{N:1}, main.Inner{N:0}}, Ptr:(*main.Inner)(nil)} | type{Value="string" Type=int Offset=26 Struct="Inner" Field="Items.N"}
// {"Ptr":{"N":"x"},"Name":"after"}   => main.Outer{Inner:main.Inner{N:0}, Name:"after", Items:[]main.Inner(nil), Ptr:(*main.Inner)(0xADDR)} | type{Value="string" Type=int Offset=15 Struct="Inner" Field="Ptr.N"}
// == *int
// null                               => (*int)(nil) | ok
// true                               => (*int)(0xADDR) | type{Value="bool" Type=int Offset=4 Struct="" Field=""}
// false                              => (*int)(0xADDR) | type{Value="bool" Type=int Offset=5 Struct="" Field=""}
// 0                                  => (*int)(0xADDR) | ok
// -1                                 => (*int)(0xADDR) | ok
// 127                                => (*int)(0xADDR) | ok
// 128                                => (*int)(0xADDR) | ok
// 255                                => (*int)(0xADDR) | ok
// 256                                => (*int)(0xADDR) | ok
// 1.5                                => (*int)(0xADDR) | type{Value="number 1.5" Type=int Offset=3 Struct="" Field=""}
// 1e2                                => (*int)(0xADDR) | type{Value="number 1e2" Type=int Offset=3 Struct="" Field=""}
// -0                                 => (*int)(0xADDR) | ok
// 1e400                              => (*int)(0xADDR) | type{Value="number 1e400" Type=int Offset=5 Struct="" Field=""}
// "text"                             => (*int)(0xADDR) | type{Value="string" Type=int Offset=6 Struct="" Field=""}
// "12"                               => (*int)(0xADDR) | type{Value="string" Type=int Offset=4 Struct="" Field=""}
// ""                                 => (*int)(0xADDR) | type{Value="string" Type=int Offset=2 Struct="" Field=""}
// []                                 => (*int)(0xADDR) | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// [1,2,3]                            => (*int)(0xADDR) | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// ["a","b"]                          => (*int)(0xADDR) | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// [1,"x",true]                       => (*int)(0xADDR) | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// {}                                 => (*int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"N":7}                            => (*int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"a":1}                            => (*int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Name":1}                         => (*int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => (*int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => (*int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// == *Inner nil
// null                               => (*main.Inner)(nil) | ok
// true                               => &main.Inner{N:0} | type{Value="bool" Type=main.Inner Offset=4 Struct="" Field=""}
// false                              => &main.Inner{N:0} | type{Value="bool" Type=main.Inner Offset=5 Struct="" Field=""}
// 0                                  => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=1 Struct="" Field=""}
// -1                                 => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=2 Struct="" Field=""}
// 127                                => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 128                                => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 255                                => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 256                                => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 1.5                                => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// 1e2                                => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=3 Struct="" Field=""}
// -0                                 => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=2 Struct="" Field=""}
// 1e400                              => &main.Inner{N:0} | type{Value="number" Type=main.Inner Offset=5 Struct="" Field=""}
// "text"                             => &main.Inner{N:0} | type{Value="string" Type=main.Inner Offset=6 Struct="" Field=""}
// "12"                               => &main.Inner{N:0} | type{Value="string" Type=main.Inner Offset=4 Struct="" Field=""}
// ""                                 => &main.Inner{N:0} | type{Value="string" Type=main.Inner Offset=2 Struct="" Field=""}
// []                                 => &main.Inner{N:0} | type{Value="array" Type=main.Inner Offset=1 Struct="" Field=""}
// [1,2,3]                            => &main.Inner{N:0} | type{Value="array" Type=main.Inner Offset=1 Struct="" Field=""}
// ["a","b"]                          => &main.Inner{N:0} | type{Value="array" Type=main.Inner Offset=1 Struct="" Field=""}
// [1,"x",true]                       => &main.Inner{N:0} | type{Value="array" Type=main.Inner Offset=1 Struct="" Field=""}
// {}                                 => &main.Inner{N:0} | ok
// {"N":7}                            => &main.Inner{N:7} | ok
// {"a":1}                            => &main.Inner{N:0} | ok
// {"Name":1}                         => &main.Inner{N:0} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => &main.Inner{N:0} | ok
// {"Ptr":{"N":"x"},"Name":"after"}   => &main.Inner{N:0} | ok
// == **int
// null                               => (**int)(nil) | ok
// true                               => (**int)(0xADDR) | type{Value="bool" Type=int Offset=4 Struct="" Field=""}
// false                              => (**int)(0xADDR) | type{Value="bool" Type=int Offset=5 Struct="" Field=""}
// 0                                  => (**int)(0xADDR) | ok
// -1                                 => (**int)(0xADDR) | ok
// 127                                => (**int)(0xADDR) | ok
// 128                                => (**int)(0xADDR) | ok
// 255                                => (**int)(0xADDR) | ok
// 256                                => (**int)(0xADDR) | ok
// 1.5                                => (**int)(0xADDR) | type{Value="number 1.5" Type=int Offset=3 Struct="" Field=""}
// 1e2                                => (**int)(0xADDR) | type{Value="number 1e2" Type=int Offset=3 Struct="" Field=""}
// -0                                 => (**int)(0xADDR) | ok
// 1e400                              => (**int)(0xADDR) | type{Value="number 1e400" Type=int Offset=5 Struct="" Field=""}
// "text"                             => (**int)(0xADDR) | type{Value="string" Type=int Offset=6 Struct="" Field=""}
// "12"                               => (**int)(0xADDR) | type{Value="string" Type=int Offset=4 Struct="" Field=""}
// ""                                 => (**int)(0xADDR) | type{Value="string" Type=int Offset=2 Struct="" Field=""}
// []                                 => (**int)(0xADDR) | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// [1,2,3]                            => (**int)(0xADDR) | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// ["a","b"]                          => (**int)(0xADDR) | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// [1,"x",true]                       => (**int)(0xADDR) | type{Value="array" Type=int Offset=1 Struct="" Field=""}
// {}                                 => (**int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"N":7}                            => (**int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"a":1}                            => (**int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Name":1}                         => (**int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => (**int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => (**int)(0xADDR) | type{Value="object" Type=int Offset=1 Struct="" Field=""}
// == any
// null                               => <nil> | ok
// true                               => true | ok
// false                              => false | ok
// 0                                  => 0 | ok
// -1                                 => -1 | ok
// 127                                => 127 | ok
// 128                                => 128 | ok
// 255                                => 255 | ok
// 256                                => 256 | ok
// 1.5                                => 1.5 | ok
// 1e2                                => 100 | ok
// -0                                 => -0 | ok
// 1e400                              => "seed" | type{Value="number 1e400" Type=float64 Offset=6 Struct="" Field=""}
// "text"                             => "text" | ok
// "12"                               => "12" | ok
// ""                                 => "" | ok
// []                                 => []interface {}{} | ok
// [1,2,3]                            => []interface {}{1, 2, 3} | ok
// ["a","b"]                          => []interface {}{"a", "b"} | ok
// [1,"x",true]                       => []interface {}{1, "x", true} | ok
// {}                                 => map[string]interface {}{} | ok
// {"N":7}                            => map[string]interface {}{"N":7} | ok
// {"a":1}                            => map[string]interface {}{"a":1} | ok
// {"Name":1}                         => map[string]interface {}{"Name":1} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => map[string]interface {}{"Items":[]interface {}{map[string]interface {}{"N":1}, map[string]interface {}{"N":"x"}}} | ok
// {"Ptr":{"N":"x"},"Name":"after"}   => map[string]interface {}{"Name":"after", "Ptr":map[string]interface {}{"N":"x"}} | ok
// == any nil
// null                               => <nil> | ok
// true                               => true | ok
// false                              => false | ok
// 0                                  => 0 | ok
// -1                                 => -1 | ok
// 127                                => 127 | ok
// 128                                => 128 | ok
// 255                                => 255 | ok
// 256                                => 256 | ok
// 1.5                                => 1.5 | ok
// 1e2                                => 100 | ok
// -0                                 => -0 | ok
// 1e400                              => <nil> | type{Value="number 1e400" Type=float64 Offset=6 Struct="" Field=""}
// "text"                             => "text" | ok
// "12"                               => "12" | ok
// ""                                 => "" | ok
// []                                 => []interface {}{} | ok
// [1,2,3]                            => []interface {}{1, 2, 3} | ok
// ["a","b"]                          => []interface {}{"a", "b"} | ok
// [1,"x",true]                       => []interface {}{1, "x", true} | ok
// {}                                 => map[string]interface {}{} | ok
// {"N":7}                            => map[string]interface {}{"N":7} | ok
// {"a":1}                            => map[string]interface {}{"a":1} | ok
// {"Name":1}                         => map[string]interface {}{"Name":1} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => map[string]interface {}{"Items":[]interface {}{map[string]interface {}{"N":1}, map[string]interface {}{"N":"x"}}} | ok
// {"Ptr":{"N":"x"},"Name":"after"}   => map[string]interface {}{"Name":"after", "Ptr":map[string]interface {}{"N":"x"}} | ok
// == []any
// null                               => []interface {}(nil) | ok
// true                               => []interface {}{"seed"} | type{Value="bool" Type=[]interface {} Offset=4 Struct="" Field=""}
// false                              => []interface {}{"seed"} | type{Value="bool" Type=[]interface {} Offset=5 Struct="" Field=""}
// 0                                  => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=1 Struct="" Field=""}
// -1                                 => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=2 Struct="" Field=""}
// 127                                => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=3 Struct="" Field=""}
// 128                                => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=3 Struct="" Field=""}
// 255                                => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=3 Struct="" Field=""}
// 256                                => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=3 Struct="" Field=""}
// 1.5                                => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=3 Struct="" Field=""}
// 1e2                                => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=3 Struct="" Field=""}
// -0                                 => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=2 Struct="" Field=""}
// 1e400                              => []interface {}{"seed"} | type{Value="number" Type=[]interface {} Offset=5 Struct="" Field=""}
// "text"                             => []interface {}{"seed"} | type{Value="string" Type=[]interface {} Offset=6 Struct="" Field=""}
// "12"                               => []interface {}{"seed"} | type{Value="string" Type=[]interface {} Offset=4 Struct="" Field=""}
// ""                                 => []interface {}{"seed"} | type{Value="string" Type=[]interface {} Offset=2 Struct="" Field=""}
// []                                 => []interface {}{} | ok
// [1,2,3]                            => []interface {}{1, 2, 3} | ok
// ["a","b"]                          => []interface {}{"a", "b"} | ok
// [1,"x",true]                       => []interface {}{1, "x", true} | ok
// {}                                 => []interface {}{1} | type{Value="object" Type=[]interface {} Offset=1 Struct="" Field=""}
// {"N":7}                            => []interface {}{1} | type{Value="object" Type=[]interface {} Offset=1 Struct="" Field=""}
// {"a":1}                            => []interface {}{1} | type{Value="object" Type=[]interface {} Offset=1 Struct="" Field=""}
// {"Name":1}                         => []interface {}{1} | type{Value="object" Type=[]interface {} Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => []interface {}{1} | type{Value="object" Type=[]interface {} Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => []interface {}{1} | type{Value="object" Type=[]interface {} Offset=1 Struct="" Field=""}
// == Celsius
// null                               => 5 | ok
// true                               => 5 | type{Value="bool" Type=main.Celsius Offset=4 Struct="" Field=""}
// false                              => 5 | type{Value="bool" Type=main.Celsius Offset=5 Struct="" Field=""}
// 0                                  => 0 | ok
// -1                                 => -1 | ok
// 127                                => 127 | ok
// 128                                => 128 | ok
// 255                                => 255 | ok
// 256                                => 256 | ok
// 1.5                                => 1.5 | ok
// 1e2                                => 100 | ok
// -0                                 => -0 | ok
// 1e400                              => 5 | type{Value="number 1e400" Type=main.Celsius Offset=5 Struct="" Field=""}
// "text"                             => 5 | type{Value="string" Type=main.Celsius Offset=6 Struct="" Field=""}
// "12"                               => 5 | type{Value="string" Type=main.Celsius Offset=4 Struct="" Field=""}
// ""                                 => 5 | type{Value="string" Type=main.Celsius Offset=2 Struct="" Field=""}
// []                                 => 5 | type{Value="array" Type=main.Celsius Offset=1 Struct="" Field=""}
// [1,2,3]                            => 5 | type{Value="array" Type=main.Celsius Offset=1 Struct="" Field=""}
// ["a","b"]                          => 5 | type{Value="array" Type=main.Celsius Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 5 | type{Value="array" Type=main.Celsius Offset=1 Struct="" Field=""}
// {}                                 => 5 | type{Value="object" Type=main.Celsius Offset=1 Struct="" Field=""}
// {"N":7}                            => 5 | type{Value="object" Type=main.Celsius Offset=1 Struct="" Field=""}
// {"a":1}                            => 5 | type{Value="object" Type=main.Celsius Offset=1 Struct="" Field=""}
// {"Name":1}                         => 5 | type{Value="object" Type=main.Celsius Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 5 | type{Value="object" Type=main.Celsius Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 5 | type{Value="object" Type=main.Celsius Offset=1 Struct="" Field=""}
// == Flags
// null                               => 0x5 | ok
// true                               => 0x5 | type{Value="bool" Type=main.Flags Offset=4 Struct="" Field=""}
// false                              => 0x5 | type{Value="bool" Type=main.Flags Offset=5 Struct="" Field=""}
// 0                                  => 0x0 | ok
// -1                                 => 0x5 | type{Value="number -1" Type=main.Flags Offset=2 Struct="" Field=""}
// 127                                => 0x7f | ok
// 128                                => 0x80 | ok
// 255                                => 0xff | ok
// 256                                => 0x5 | type{Value="number 256" Type=main.Flags Offset=3 Struct="" Field=""}
// 1.5                                => 0x5 | type{Value="number 1.5" Type=main.Flags Offset=3 Struct="" Field=""}
// 1e2                                => 0x5 | type{Value="number 1e2" Type=main.Flags Offset=3 Struct="" Field=""}
// -0                                 => 0x5 | type{Value="number -0" Type=main.Flags Offset=2 Struct="" Field=""}
// 1e400                              => 0x5 | type{Value="number 1e400" Type=main.Flags Offset=5 Struct="" Field=""}
// "text"                             => 0x5 | type{Value="string" Type=main.Flags Offset=6 Struct="" Field=""}
// "12"                               => 0x5 | type{Value="string" Type=main.Flags Offset=4 Struct="" Field=""}
// ""                                 => 0x5 | type{Value="string" Type=main.Flags Offset=2 Struct="" Field=""}
// []                                 => 0x5 | type{Value="array" Type=main.Flags Offset=1 Struct="" Field=""}
// [1,2,3]                            => 0x5 | type{Value="array" Type=main.Flags Offset=1 Struct="" Field=""}
// ["a","b"]                          => 0x5 | type{Value="array" Type=main.Flags Offset=1 Struct="" Field=""}
// [1,"x",true]                       => 0x5 | type{Value="array" Type=main.Flags Offset=1 Struct="" Field=""}
// {}                                 => 0x5 | type{Value="object" Type=main.Flags Offset=1 Struct="" Field=""}
// {"N":7}                            => 0x5 | type{Value="object" Type=main.Flags Offset=1 Struct="" Field=""}
// {"a":1}                            => 0x5 | type{Value="object" Type=main.Flags Offset=1 Struct="" Field=""}
// {"Name":1}                         => 0x5 | type{Value="object" Type=main.Flags Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => 0x5 | type{Value="object" Type=main.Flags Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => 0x5 | type{Value="object" Type=main.Flags Offset=1 Struct="" Field=""}
// == Names
// null                               => main.Names(nil) | ok
// true                               => main.Names{"seed"} | type{Value="bool" Type=main.Names Offset=4 Struct="" Field=""}
// false                              => main.Names{"seed"} | type{Value="bool" Type=main.Names Offset=5 Struct="" Field=""}
// 0                                  => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=1 Struct="" Field=""}
// -1                                 => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=2 Struct="" Field=""}
// 127                                => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=3 Struct="" Field=""}
// 128                                => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=3 Struct="" Field=""}
// 255                                => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=3 Struct="" Field=""}
// 256                                => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=3 Struct="" Field=""}
// 1.5                                => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=3 Struct="" Field=""}
// 1e2                                => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=3 Struct="" Field=""}
// -0                                 => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=2 Struct="" Field=""}
// 1e400                              => main.Names{"seed"} | type{Value="number" Type=main.Names Offset=5 Struct="" Field=""}
// "text"                             => main.Names{"seed"} | type{Value="string" Type=main.Names Offset=6 Struct="" Field=""}
// "12"                               => main.Names{"seed"} | type{Value="string" Type=main.Names Offset=4 Struct="" Field=""}
// ""                                 => main.Names{"seed"} | type{Value="string" Type=main.Names Offset=2 Struct="" Field=""}
// []                                 => main.Names{} | ok
// [1,2,3]                            => main.Names{"seed", "", ""} | type{Value="number" Type=string Offset=2 Struct="" Field=""}
// ["a","b"]                          => main.Names{"a", "b"} | ok
// [1,"x",true]                       => main.Names{"a", "x", ""} | type{Value="number" Type=string Offset=2 Struct="" Field=""}
// {}                                 => main.Names{"a"} | type{Value="object" Type=main.Names Offset=1 Struct="" Field=""}
// {"N":7}                            => main.Names{"a"} | type{Value="object" Type=main.Names Offset=1 Struct="" Field=""}
// {"a":1}                            => main.Names{"a"} | type{Value="object" Type=main.Names Offset=1 Struct="" Field=""}
// {"Name":1}                         => main.Names{"a"} | type{Value="object" Type=main.Names Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => main.Names{"a"} | type{Value="object" Type=main.Names Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => main.Names{"a"} | type{Value="object" Type=main.Names Offset=1 Struct="" Field=""}
// == Lookup
// null                               => main.Lookup(nil) | ok
// true                               => main.Lookup{"seed":1} | type{Value="bool" Type=main.Lookup Offset=4 Struct="" Field=""}
// false                              => main.Lookup{"seed":1} | type{Value="bool" Type=main.Lookup Offset=5 Struct="" Field=""}
// 0                                  => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=1 Struct="" Field=""}
// -1                                 => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=2 Struct="" Field=""}
// 127                                => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=3 Struct="" Field=""}
// 128                                => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=3 Struct="" Field=""}
// 255                                => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=3 Struct="" Field=""}
// 256                                => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=3 Struct="" Field=""}
// 1.5                                => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=3 Struct="" Field=""}
// 1e2                                => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=3 Struct="" Field=""}
// -0                                 => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=2 Struct="" Field=""}
// 1e400                              => main.Lookup{"seed":1} | type{Value="number" Type=main.Lookup Offset=5 Struct="" Field=""}
// "text"                             => main.Lookup{"seed":1} | type{Value="string" Type=main.Lookup Offset=6 Struct="" Field=""}
// "12"                               => main.Lookup{"seed":1} | type{Value="string" Type=main.Lookup Offset=4 Struct="" Field=""}
// ""                                 => main.Lookup{"seed":1} | type{Value="string" Type=main.Lookup Offset=2 Struct="" Field=""}
// []                                 => main.Lookup{"seed":1} | type{Value="array" Type=main.Lookup Offset=1 Struct="" Field=""}
// [1,2,3]                            => main.Lookup{"seed":1} | type{Value="array" Type=main.Lookup Offset=1 Struct="" Field=""}
// ["a","b"]                          => main.Lookup{"seed":1} | type{Value="array" Type=main.Lookup Offset=1 Struct="" Field=""}
// [1,"x",true]                       => main.Lookup{"seed":1} | type{Value="array" Type=main.Lookup Offset=1 Struct="" Field=""}
// {}                                 => main.Lookup{"seed":1} | ok
// {"N":7}                            => main.Lookup{"N":7, "seed":1} | ok
// {"a":1}                            => main.Lookup{"N":7, "a":1, "seed":1} | ok
// {"Name":1}                         => main.Lookup{"N":7, "Name":1, "a":1, "seed":1} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => main.Lookup{"Items":0, "N":7, "Name":1, "a":1, "seed":1} | type{Value="array" Type=int Offset=10 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => main.Lookup{"Items":0, "N":7, "Name":0, "Ptr":0, "a":1, "seed":1} | type{Value="object" Type=int Offset=8 Struct="" Field=""}
// == Named
// null                               => "seed" | ok
// true                               => "seed" | type{Value="bool" Type=main.Named Offset=4 Struct="" Field=""}
// false                              => "seed" | type{Value="bool" Type=main.Named Offset=5 Struct="" Field=""}
// 0                                  => "seed" | type{Value="number" Type=main.Named Offset=1 Struct="" Field=""}
// -1                                 => "seed" | type{Value="number" Type=main.Named Offset=2 Struct="" Field=""}
// 127                                => "seed" | type{Value="number" Type=main.Named Offset=3 Struct="" Field=""}
// 128                                => "seed" | type{Value="number" Type=main.Named Offset=3 Struct="" Field=""}
// 255                                => "seed" | type{Value="number" Type=main.Named Offset=3 Struct="" Field=""}
// 256                                => "seed" | type{Value="number" Type=main.Named Offset=3 Struct="" Field=""}
// 1.5                                => "seed" | type{Value="number" Type=main.Named Offset=3 Struct="" Field=""}
// 1e2                                => "seed" | type{Value="number" Type=main.Named Offset=3 Struct="" Field=""}
// -0                                 => "seed" | type{Value="number" Type=main.Named Offset=2 Struct="" Field=""}
// 1e400                              => "seed" | type{Value="number" Type=main.Named Offset=5 Struct="" Field=""}
// "text"                             => "text" | ok
// "12"                               => "12" | ok
// ""                                 => "" | ok
// []                                 => "seed" | type{Value="array" Type=main.Named Offset=1 Struct="" Field=""}
// [1,2,3]                            => "seed" | type{Value="array" Type=main.Named Offset=1 Struct="" Field=""}
// ["a","b"]                          => "seed" | type{Value="array" Type=main.Named Offset=1 Struct="" Field=""}
// [1,"x",true]                       => "seed" | type{Value="array" Type=main.Named Offset=1 Struct="" Field=""}
// {}                                 => "seed" | type{Value="object" Type=main.Named Offset=1 Struct="" Field=""}
// {"N":7}                            => "seed" | type{Value="object" Type=main.Named Offset=1 Struct="" Field=""}
// {"a":1}                            => "seed" | type{Value="object" Type=main.Named Offset=1 Struct="" Field=""}
// {"Name":1}                         => "seed" | type{Value="object" Type=main.Named Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => "seed" | type{Value="object" Type=main.Named Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => "seed" | type{Value="object" Type=main.Named Offset=1 Struct="" Field=""}
// == json.Number
// null                               => "5" | ok
// true                               => "5" | type{Value="bool" Type=json.Number Offset=4 Struct="" Field=""}
// false                              => "5" | type{Value="bool" Type=json.Number Offset=5 Struct="" Field=""}
// 0                                  => "0" | ok
// -1                                 => "-1" | ok
// 127                                => "127" | ok
// 128                                => "128" | ok
// 255                                => "255" | ok
// 256                                => "256" | ok
// 1.5                                => "1.5" | ok
// 1e2                                => "1e2" | ok
// -0                                 => "-0" | ok
// 1e400                              => "1e400" | ok
// "text"                             => "5" | *errors.errorString "json: invalid number literal, trying to unmarshal \"\\\"text\\\"\" into Number"
// "12"                               => "12" | ok
// ""                                 => "5" | *errors.errorString "json: invalid number literal, trying to unmarshal \"\\\"\\\"\" into Number"
// []                                 => "5" | type{Value="array" Type=json.Number Offset=1 Struct="" Field=""}
// [1,2,3]                            => "5" | type{Value="array" Type=json.Number Offset=1 Struct="" Field=""}
// ["a","b"]                          => "5" | type{Value="array" Type=json.Number Offset=1 Struct="" Field=""}
// [1,"x",true]                       => "5" | type{Value="array" Type=json.Number Offset=1 Struct="" Field=""}
// {}                                 => "5" | type{Value="object" Type=json.Number Offset=1 Struct="" Field=""}
// {"N":7}                            => "5" | type{Value="object" Type=json.Number Offset=1 Struct="" Field=""}
// {"a":1}                            => "5" | type{Value="object" Type=json.Number Offset=1 Struct="" Field=""}
// {"Name":1}                         => "5" | type{Value="object" Type=json.Number Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => "5" | type{Value="object" Type=json.Number Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => "5" | type{Value="object" Type=json.Number Offset=1 Struct="" Field=""}
// == json.RawMessage
// null                               => json.RawMessage{0x6e, 0x75, 0x6c, 0x6c} | ok
// true                               => json.RawMessage{0x74, 0x72, 0x75, 0x65} | ok
// false                              => json.RawMessage{0x66, 0x61, 0x6c, 0x73, 0x65} | ok
// 0                                  => json.RawMessage{0x30} | ok
// -1                                 => json.RawMessage{0x2d, 0x31} | ok
// 127                                => json.RawMessage{0x31, 0x32, 0x37} | ok
// 128                                => json.RawMessage{0x31, 0x32, 0x38} | ok
// 255                                => json.RawMessage{0x32, 0x35, 0x35} | ok
// 256                                => json.RawMessage{0x32, 0x35, 0x36} | ok
// 1.5                                => json.RawMessage{0x31, 0x2e, 0x35} | ok
// 1e2                                => json.RawMessage{0x31, 0x65, 0x32} | ok
// -0                                 => json.RawMessage{0x2d, 0x30} | ok
// 1e400                              => json.RawMessage{0x31, 0x65, 0x34, 0x30, 0x30} | ok
// "text"                             => json.RawMessage{0x22, 0x74, 0x65, 0x78, 0x74, 0x22} | ok
// "12"                               => json.RawMessage{0x22, 0x31, 0x32, 0x22} | ok
// ""                                 => json.RawMessage{0x22, 0x22} | ok
// []                                 => json.RawMessage{0x5b, 0x5d} | ok
// [1,2,3]                            => json.RawMessage{0x5b, 0x31, 0x2c, 0x32, 0x2c, 0x33, 0x5d} | ok
// ["a","b"]                          => json.RawMessage{0x5b, 0x22, 0x61, 0x22, 0x2c, 0x22, 0x62, 0x22, 0x5d} | ok
// [1,"x",true]                       => json.RawMessage{0x5b, 0x31, 0x2c, 0x22, 0x78, 0x22, 0x2c, 0x74, 0x72, 0x75, 0x65, 0x5d} | ok
// {}                                 => json.RawMessage{0x7b, 0x7d} | ok
// {"N":7}                            => json.RawMessage{0x7b, 0x22, 0x4e, 0x22, 0x3a, 0x37, 0x7d} | ok
// {"a":1}                            => json.RawMessage{0x7b, 0x22, 0x61, 0x22, 0x3a, 0x31, 0x7d} | ok
// {"Name":1}                         => json.RawMessage{0x7b, 0x22, 0x4e, 0x61, 0x6d, 0x65, 0x22, 0x3a, 0x31, 0x7d} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => json.RawMessage{0x7b, 0x22, 0x49, 0x74, 0x65, 0x6d, 0x73, 0x22, 0x3a, 0x5b, 0x7b, 0x22, 0x4e, 0x22, 0x3a, 0x31, 0x7d, 0x2c, 0x7b, 0x22, 0x4e, 0x22, 0x3a, 0x22, 0x78, 0x22, 0x7d, 0x5d, 0x7d} | ok
// {"Ptr":{"N":"x"},"Name":"after"}   => json.RawMessage{0x7b, 0x22, 0x50, 0x74, 0x72, 0x22, 0x3a, 0x7b, 0x22, 0x4e, 0x22, 0x3a, 0x22, 0x78, 0x22, 0x7d, 0x2c, 0x22, 0x4e, 0x61, 0x6d, 0x65, 0x22, 0x3a, 0x22, 0x61, 0x66, 0x74, 0x65, 0x72, 0x22, 0x7d} | ok
// == struct{}
// null                               => struct {}{} | ok
// true                               => struct {}{} | type{Value="bool" Type=struct {} Offset=4 Struct="" Field=""}
// false                              => struct {}{} | type{Value="bool" Type=struct {} Offset=5 Struct="" Field=""}
// 0                                  => struct {}{} | type{Value="number" Type=struct {} Offset=1 Struct="" Field=""}
// -1                                 => struct {}{} | type{Value="number" Type=struct {} Offset=2 Struct="" Field=""}
// 127                                => struct {}{} | type{Value="number" Type=struct {} Offset=3 Struct="" Field=""}
// 128                                => struct {}{} | type{Value="number" Type=struct {} Offset=3 Struct="" Field=""}
// 255                                => struct {}{} | type{Value="number" Type=struct {} Offset=3 Struct="" Field=""}
// 256                                => struct {}{} | type{Value="number" Type=struct {} Offset=3 Struct="" Field=""}
// 1.5                                => struct {}{} | type{Value="number" Type=struct {} Offset=3 Struct="" Field=""}
// 1e2                                => struct {}{} | type{Value="number" Type=struct {} Offset=3 Struct="" Field=""}
// -0                                 => struct {}{} | type{Value="number" Type=struct {} Offset=2 Struct="" Field=""}
// 1e400                              => struct {}{} | type{Value="number" Type=struct {} Offset=5 Struct="" Field=""}
// "text"                             => struct {}{} | type{Value="string" Type=struct {} Offset=6 Struct="" Field=""}
// "12"                               => struct {}{} | type{Value="string" Type=struct {} Offset=4 Struct="" Field=""}
// ""                                 => struct {}{} | type{Value="string" Type=struct {} Offset=2 Struct="" Field=""}
// []                                 => struct {}{} | type{Value="array" Type=struct {} Offset=1 Struct="" Field=""}
// [1,2,3]                            => struct {}{} | type{Value="array" Type=struct {} Offset=1 Struct="" Field=""}
// ["a","b"]                          => struct {}{} | type{Value="array" Type=struct {} Offset=1 Struct="" Field=""}
// [1,"x",true]                       => struct {}{} | type{Value="array" Type=struct {} Offset=1 Struct="" Field=""}
// {}                                 => struct {}{} | ok
// {"N":7}                            => struct {}{} | ok
// {"a":1}                            => struct {}{} | ok
// {"Name":1}                         => struct {}{} | ok
// {"Items":[{"N":1},{"N":"x"}]}      => struct {}{} | ok
// {"Ptr":{"N":"x"},"Name":"after"}   => struct {}{} | ok
// == []struct
// null                               => []struct { A int }(nil) | ok
// true                               => []struct { A int }{struct { A int }{A:9}} | type{Value="bool" Type=[]struct { A int } Offset=4 Struct="" Field=""}
// false                              => []struct { A int }{struct { A int }{A:9}} | type{Value="bool" Type=[]struct { A int } Offset=5 Struct="" Field=""}
// 0                                  => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=1 Struct="" Field=""}
// -1                                 => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=2 Struct="" Field=""}
// 127                                => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=3 Struct="" Field=""}
// 128                                => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=3 Struct="" Field=""}
// 255                                => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=3 Struct="" Field=""}
// 256                                => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=3 Struct="" Field=""}
// 1.5                                => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=3 Struct="" Field=""}
// 1e2                                => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=3 Struct="" Field=""}
// -0                                 => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=2 Struct="" Field=""}
// 1e400                              => []struct { A int }{struct { A int }{A:9}} | type{Value="number" Type=[]struct { A int } Offset=5 Struct="" Field=""}
// "text"                             => []struct { A int }{struct { A int }{A:9}} | type{Value="string" Type=[]struct { A int } Offset=6 Struct="" Field=""}
// "12"                               => []struct { A int }{struct { A int }{A:9}} | type{Value="string" Type=[]struct { A int } Offset=4 Struct="" Field=""}
// ""                                 => []struct { A int }{struct { A int }{A:9}} | type{Value="string" Type=[]struct { A int } Offset=2 Struct="" Field=""}
// []                                 => []struct { A int }{} | ok
// [1,2,3]                            => []struct { A int }{struct { A int }{A:9}, struct { A int }{A:0}, struct { A int }{A:0}} | type{Value="number" Type=struct { A int } Offset=2 Struct="" Field=""}
// ["a","b"]                          => []struct { A int }{struct { A int }{A:9}, struct { A int }{A:0}} | type{Value="string" Type=struct { A int } Offset=4 Struct="" Field=""}
// [1,"x",true]                       => []struct { A int }{struct { A int }{A:9}, struct { A int }{A:0}, struct { A int }{A:0}} | type{Value="number" Type=struct { A int } Offset=2 Struct="" Field=""}
// {}                                 => []struct { A int }{struct { A int }{A:9}} | type{Value="object" Type=[]struct { A int } Offset=1 Struct="" Field=""}
// {"N":7}                            => []struct { A int }{struct { A int }{A:9}} | type{Value="object" Type=[]struct { A int } Offset=1 Struct="" Field=""}
// {"a":1}                            => []struct { A int }{struct { A int }{A:9}} | type{Value="object" Type=[]struct { A int } Offset=1 Struct="" Field=""}
// {"Name":1}                         => []struct { A int }{struct { A int }{A:9}} | type{Value="object" Type=[]struct { A int } Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => []struct { A int }{struct { A int }{A:9}} | type{Value="object" Type=[]struct { A int } Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => []struct { A int }{struct { A int }{A:9}} | type{Value="object" Type=[]struct { A int } Offset=1 Struct="" Field=""}
// == [0]int
// null                               => [0]int{} | ok
// true                               => [0]int{} | type{Value="bool" Type=[0]int Offset=4 Struct="" Field=""}
// false                              => [0]int{} | type{Value="bool" Type=[0]int Offset=5 Struct="" Field=""}
// 0                                  => [0]int{} | type{Value="number" Type=[0]int Offset=1 Struct="" Field=""}
// -1                                 => [0]int{} | type{Value="number" Type=[0]int Offset=2 Struct="" Field=""}
// 127                                => [0]int{} | type{Value="number" Type=[0]int Offset=3 Struct="" Field=""}
// 128                                => [0]int{} | type{Value="number" Type=[0]int Offset=3 Struct="" Field=""}
// 255                                => [0]int{} | type{Value="number" Type=[0]int Offset=3 Struct="" Field=""}
// 256                                => [0]int{} | type{Value="number" Type=[0]int Offset=3 Struct="" Field=""}
// 1.5                                => [0]int{} | type{Value="number" Type=[0]int Offset=3 Struct="" Field=""}
// 1e2                                => [0]int{} | type{Value="number" Type=[0]int Offset=3 Struct="" Field=""}
// -0                                 => [0]int{} | type{Value="number" Type=[0]int Offset=2 Struct="" Field=""}
// 1e400                              => [0]int{} | type{Value="number" Type=[0]int Offset=5 Struct="" Field=""}
// "text"                             => [0]int{} | type{Value="string" Type=[0]int Offset=6 Struct="" Field=""}
// "12"                               => [0]int{} | type{Value="string" Type=[0]int Offset=4 Struct="" Field=""}
// ""                                 => [0]int{} | type{Value="string" Type=[0]int Offset=2 Struct="" Field=""}
// []                                 => [0]int{} | ok
// [1,2,3]                            => [0]int{} | ok
// ["a","b"]                          => [0]int{} | ok
// [1,"x",true]                       => [0]int{} | ok
// {}                                 => [0]int{} | type{Value="object" Type=[0]int Offset=1 Struct="" Field=""}
// {"N":7}                            => [0]int{} | type{Value="object" Type=[0]int Offset=1 Struct="" Field=""}
// {"a":1}                            => [0]int{} | type{Value="object" Type=[0]int Offset=1 Struct="" Field=""}
// {"Name":1}                         => [0]int{} | type{Value="object" Type=[0]int Offset=1 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => [0]int{} | type{Value="object" Type=[0]int Offset=1 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => [0]int{} | type{Value="object" Type=[0]int Offset=1 Struct="" Field=""}
// == map[int]string
// null                               => map[int]string(nil) | ok
// true                               => map[int]string{5:"seed"} | type{Value="bool" Type=map[int]string Offset=4 Struct="" Field=""}
// false                              => map[int]string{5:"seed"} | type{Value="bool" Type=map[int]string Offset=5 Struct="" Field=""}
// 0                                  => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=1 Struct="" Field=""}
// -1                                 => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=2 Struct="" Field=""}
// 127                                => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=3 Struct="" Field=""}
// 128                                => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=3 Struct="" Field=""}
// 255                                => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=3 Struct="" Field=""}
// 256                                => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=3 Struct="" Field=""}
// 1.5                                => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=3 Struct="" Field=""}
// 1e2                                => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=3 Struct="" Field=""}
// -0                                 => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=2 Struct="" Field=""}
// 1e400                              => map[int]string{5:"seed"} | type{Value="number" Type=map[int]string Offset=5 Struct="" Field=""}
// "text"                             => map[int]string{5:"seed"} | type{Value="string" Type=map[int]string Offset=6 Struct="" Field=""}
// "12"                               => map[int]string{5:"seed"} | type{Value="string" Type=map[int]string Offset=4 Struct="" Field=""}
// ""                                 => map[int]string{5:"seed"} | type{Value="string" Type=map[int]string Offset=2 Struct="" Field=""}
// []                                 => map[int]string{5:"seed"} | type{Value="array" Type=map[int]string Offset=1 Struct="" Field=""}
// [1,2,3]                            => map[int]string{5:"seed"} | type{Value="array" Type=map[int]string Offset=1 Struct="" Field=""}
// ["a","b"]                          => map[int]string{5:"seed"} | type{Value="array" Type=map[int]string Offset=1 Struct="" Field=""}
// [1,"x",true]                       => map[int]string{5:"seed"} | type{Value="array" Type=map[int]string Offset=1 Struct="" Field=""}
// {}                                 => map[int]string{5:"seed"} | ok
// {"N":7}                            => map[int]string{5:"seed"} | type{Value="number" Type=string Offset=6 Struct="" Field=""}
// {"a":1}                            => map[int]string{5:"seed"} | type{Value="number" Type=string Offset=6 Struct="" Field=""}
// {"Name":1}                         => map[int]string{5:"seed"} | type{Value="number" Type=string Offset=9 Struct="" Field=""}
// {"Items":[{"N":1},{"N":"x"}]}      => map[int]string{5:"seed"} | type{Value="array" Type=string Offset=10 Struct="" Field=""}
// {"Ptr":{"N":"x"},"Name":"after"}   => map[int]string{5:"seed"} | type{Value="object" Type=string Offset=8 Struct="" Field=""}
