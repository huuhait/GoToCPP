// phase: 09
// lsan: cycle
// encoding/json over unusual types: generics, anonymous structs, recursive types, cycles,
// unsupported kinds (chan, func, complex), interface holders, and how types are named in errors.
// The cycles built on purpose are never freed (D1), hence the lsan line.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var address = regexp.MustCompile(`0x[0-9a-f]{8,}`)

type Box[T any] struct {
	Value T
	Items []T
}

type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

type Tree struct {
	Name     string
	Children []*Tree
	Parent   *Tree `json:"-"`
	Meta     map[string]*Tree
}

type Linked struct {
	Next *Linked
	Self any
}

type WithUnsupported struct {
	Keep   int
	Chan   chan int `json:"-"`
	Skip   func()   `json:"-"`
	hidden chan int
}

type BadChan struct{ C chan int }

type BadFunc struct{ F func() }

type BadComplex struct{ C complex64 }

type BadNested struct{ In struct{ F func() } }

type BadMapValue struct{ M map[string]chan int }

type Anonymous struct {
	Inline struct{ Value int }
	Fields struct {
		A int
		B []struct{ C string }
	}
}

type Iface interface{ Name() string }

type Impl struct{ N string }

func (i Impl) Name() string { return i.N }

type Holders struct {
	Iface   Iface
	Any     any
	Error   error
	Stringy fmt.Stringer
}

type Number int

type Alias = Pair[string, int]

func describe(err error) string {
	if err == nil {
		return "ok"
	}
	var typeErr *json.UnmarshalTypeError
	var unsupported *json.UnsupportedTypeError
	var unsupportedValue *json.UnsupportedValueError
	switch {
	case errors.As(err, &typeErr):
		return fmt.Sprintf("UnmarshalTypeError{Value:%q Type:%v Offset:%d Struct:%q Field:%q}", typeErr.Value, typeErr.Type, typeErr.Offset, typeErr.Struct, typeErr.Field)
	case errors.As(err, &unsupported):
		return fmt.Sprintf("UnsupportedTypeError{%v} %v", unsupported.Type, err)
	case errors.As(err, &unsupportedValue):
		return fmt.Sprintf("UnsupportedValueError{%q} %v", unsupportedValue.Str, err)
	}
	return fmt.Sprintf("%T %v", err, err)
}

func marshal(label string, value any) {
	encoded, err := json.Marshal(value)
	fmt.Printf("%-22s %s | %s\n", label, encoded, describe(err))
}

func unmarshal(label string, input string, target any) {
	err := json.Unmarshal([]byte(input), target)
	fmt.Printf("%-22s %-40s => %s | %s\n", label, input, address.ReplaceAllString(fmt.Sprintf("%+v", target), "0xADDR"), describe(err))
}

func main() {
	fmt.Println("== generics")
	marshal("Box[int]", Box[int]{Value: 1, Items: []int{2, 3}})
	marshal("Box[string]", &Box[string]{Value: "s"})
	marshal("Box[Box[int]]", Box[Box[int]]{Value: Box[int]{Value: 1}})
	marshal("Pair", Pair[string, int]{"k", 1})
	marshal("Alias", Alias{"a", 2})
	marshal("Box[chan]", Box[chan int]{})
	marshal("Box[func]", []Box[func()]{{}})
	marshal("Pair[int,..] as map", map[string]Pair[int, []string]{"x": {1, []string{"y"}}})
	var box Box[int]
	unmarshal("Box[int] ok", `{"Value":5,"Items":[1,2]}`, &box)
	unmarshal("Box[int] bad", `{"Value":"x"}`, &box)
	unmarshal("Box[int] bad item", `{"Items":[1,"x"]}`, &box)
	var nested Box[Pair[string, float32]]
	unmarshal("nested ok", `{"Value":{"Key":"k","Value":1.5}}`, &nested)
	unmarshal("nested bad", `{"Value":{"Key":1}}`, &nested)
	unmarshal("nested bad2", `{"Items":[{"Value":"x"}]}`, &nested)
	var pair Pair[int, bool]
	unmarshal("pair bad", `{"Key":1.5}`, &pair)
	unmarshal("pair bad2", `[]`, &pair)

	fmt.Println("== anonymous and nested types in errors")
	var anonymous Anonymous
	unmarshal("anon ok", `{"Inline":{"Value":1},"Fields":{"A":2,"B":[{"C":"c"}]}}`, &anonymous)
	unmarshal("anon bad", `{"Fields":{"A":"x"}}`, &anonymous)
	unmarshal("anon bad2", `{"Fields":{"B":[{"C":1}]}}`, &anonymous)
	unmarshal("anon bad3", `{"Fields":[]}`, &anonymous)
	unmarshal("anon bad4", `{"Inline":"x"}`, &anonymous)
	unmarshal("anon struct", `{"x":"y"}`, &struct{ X int }{})
	unmarshal("anon slice", `{}`, &[]struct{ A int }{})
	unmarshal("anon map", `[]`, &map[string]struct{}{})
	unmarshal("duration", `"1s"`, new(time.Duration))
	unmarshal("time bad", `1`, new(time.Time))
	unmarshal("number", `"1"`, new(Number))
	unmarshal("json.Number", `[]`, new(json.Number))
	unmarshal("raw", `1`, new(json.RawMessage))
	unmarshal("array", `{}`, new([3]string))
	unmarshal("array of arrays", `[[1],["x"]]`, new([2][1]int))
	unmarshal("chan", `1`, new(chan int))
	unmarshal("func", `1`, new(func()))
	unmarshal("complex", `1`, new(complex128))
	unmarshal("uintptr", `-1`, new(uintptr))
	unmarshal("map chan", `{"a":1}`, new(map[string]chan int))
	unmarshal("struct with chan", `{"Keep":1,"Chan":2}`, &WithUnsupported{})
	unmarshal("bad chan field", `{"C":1}`, &BadChan{})
	unmarshal("bad chan null", `{"C":null}`, &BadChan{})
	unmarshal("bad chan absent", `{}`, &BadChan{})
	unmarshal("bad func field", `{"F":1}`, &BadFunc{})

	fmt.Println("== unsupported kinds on encode")
	marshal("chan", make(chan int))
	marshal("func", func() {})
	marshal("complex", complex(1, 2))
	marshal("bad chan struct", BadChan{})
	marshal("bad func struct", BadFunc{})
	marshal("bad complex struct", BadComplex{})
	marshal("bad nested", BadNested{})
	marshal("bad map value", BadMapValue{M: map[string]chan int{"a": nil}})
	marshal("nil chan in iface", []any{chan int(nil)})
	marshal("unsupported skipped", WithUnsupported{Keep: 1})
	marshal("unsafe ptr", map[string]any{"a": uintptr(5)})
	marshal("chan in slice", []chan int{nil})
	marshal("func in map", map[string]func(){"f": nil})

	fmt.Println("== recursion and cycles")
	root := &Tree{Name: "root"}
	child := &Tree{Name: "child", Parent: root, Meta: map[string]*Tree{"self": nil}}
	root.Children = []*Tree{child, nil}
	marshal("tree", root)
	var decoded Tree
	unmarshal("tree decode", `{"Name":"a","Children":[{"Name":"b","Children":[{"Name":"c"}]},null],"Meta":{"x":{"Name":"y"}}}`, &decoded)
	fmt.Println(decoded.Children[0].Children[0].Name, decoded.Children[1] == nil, decoded.Meta["x"].Name)
	cycle := &Linked{}
	cycle.Next = cycle
	marshal("pointer cycle", cycle)
	slice := make([]any, 1)
	slice[0] = slice
	marshal("slice cycle", slice)
	mapCycle := map[string]any{}
	mapCycle["m"] = mapCycle
	marshal("map cycle", mapCycle)
	selfAny := &Linked{}
	selfAny.Self = selfAny
	marshal("any cycle", selfAny)
	shared := &Linked{}
	marshal("shared not cycle", []*Linked{shared, shared, {Next: shared}})
	long := &Linked{}
	tail := long
	for depth := 0; depth < 900; depth++ {
		tail.Next = &Linked{}
		tail = tail.Next
	}
	encoded, err := json.Marshal(long)
	fmt.Println(len(encoded), err)

	fmt.Println("== interface holders")
	marshal("holders nil", Holders{})
	marshal("holders set", Holders{Iface: Impl{"n"}, Any: &Impl{"m"}, Error: errors.New("e"), Stringy: time.Second})
	marshal("holders ptr nil", Holders{Any: (*Impl)(nil), Iface: (*Impl)(nil)})
	marshal("any numbers", []any{int8(-1), uint16(2), float32(0.1), 'x', byte(1), uint64(1 << 63), Number(4), time.Duration(5)})
	marshal("any named", []any{Number(1), []Number{2}, map[Number]Number{3: 4}})
	marshal("ptr chain", func() any { value := 5; pointer := &value; return &pointer }())
	var holders Holders
	unmarshal("holders decode", `{"Any":{"a":[1,"b"]}}`, &holders)
	unmarshal("holders iface", `{"Iface":{"N":"x"}}`, &holders)
	unmarshal("holders error", `{"Error":"x"}`, &holders)
	unmarshal("holders null", `{"Iface":null,"Any":null}`, &holders)
	var iface Iface
	unmarshal("iface direct", `{"N":"x"}`, &iface)
	unmarshal("iface non-ptr", `1`, iface)
	unmarshal("not pointer", `1`, 5)
	unmarshal("nil pointer", `1`, (*int)(nil))
	unmarshal("nil", `1`, nil)
	unmarshal("map not pointer", `{"a":1}`, map[string]int{})
	var untyped map[string]int
	unmarshal("nil map by value", `{"a":1}`, untyped)
}

// Output:
// == generics
// Box[int]               {"Value":1,"Items":[2,3]} | ok
// Box[string]            {"Value":"s","Items":null} | ok
// Box[Box[int]]          {"Value":{"Value":1,"Items":null},"Items":null} | ok
// Pair                   {"Key":"k","Value":1} | ok
// Alias                  {"Key":"a","Value":2} | ok
// Box[chan]               | UnsupportedTypeError{chan int} json: unsupported type: chan int
// Box[func]               | UnsupportedTypeError{func()} json: unsupported type: func()
// Pair[int,..] as map    {"x":{"Key":1,"Value":["y"]}} | ok
// Box[int] ok            {"Value":5,"Items":[1,2]}                => &{Value:5 Items:[1 2]} | ok
// Box[int] bad           {"Value":"x"}                            => &{Value:5 Items:[1 2]} | UnmarshalTypeError{Value:"string" Type:int Offset:12 Struct:"Box[int]" Field:"Value"}
// Box[int] bad item      {"Items":[1,"x"]}                        => &{Value:5 Items:[1 2]} | UnmarshalTypeError{Value:"string" Type:int Offset:15 Struct:"Box[int]" Field:"Items"}
// nested ok              {"Value":{"Key":"k","Value":1.5}}        => &{Value:{Key:k Value:1.5} Items:[]} | ok
// nested bad             {"Value":{"Key":1}}                      => &{Value:{Key:k Value:1.5} Items:[]} | UnmarshalTypeError{Value:"number" Type:string Offset:17 Struct:"Pair[string,float32]" Field:"Value.Key"}
// nested bad2            {"Items":[{"Value":"x"}]}                => &{Value:{Key:k Value:1.5} Items:[{Key: Value:0}]} | UnmarshalTypeError{Value:"string" Type:float32 Offset:22 Struct:"Pair[string,float32]" Field:"Items.Value"}
// pair bad               {"Key":1.5}                              => &{Key:0 Value:false} | UnmarshalTypeError{Value:"number 1.5" Type:int Offset:10 Struct:"Pair[int,bool]" Field:"Key"}
// pair bad2              []                                       => &{Key:0 Value:false} | UnmarshalTypeError{Value:"array" Type:main.Pair[int,bool] Offset:1 Struct:"" Field:""}
// == anonymous and nested types in errors
// anon ok                {"Inline":{"Value":1},"Fields":{"A":2,"B":[{"C":"c"}]}} => &{Inline:{Value:1} Fields:{A:2 B:[{C:c}]}} | ok
// anon bad               {"Fields":{"A":"x"}}                     => &{Inline:{Value:1} Fields:{A:2 B:[{C:c}]}} | UnmarshalTypeError{Value:"string" Type:int Offset:18 Struct:"" Field:"Fields.A"}
// anon bad2              {"Fields":{"B":[{"C":1}]}}               => &{Inline:{Value:1} Fields:{A:2 B:[{C:c}]}} | UnmarshalTypeError{Value:"number" Type:string Offset:22 Struct:"" Field:"Fields.B.C"}
// anon bad3              {"Fields":[]}                            => &{Inline:{Value:1} Fields:{A:2 B:[{C:c}]}} | UnmarshalTypeError{Value:"array" Type:struct { A int; B []struct { C string } } Offset:11 Struct:"Anonymous" Field:"Fields"}
// anon bad4              {"Inline":"x"}                           => &{Inline:{Value:1} Fields:{A:2 B:[{C:c}]}} | UnmarshalTypeError{Value:"string" Type:struct { Value int } Offset:13 Struct:"Anonymous" Field:"Inline"}
// anon struct            {"x":"y"}                                => &{X:0} | UnmarshalTypeError{Value:"string" Type:int Offset:8 Struct:"" Field:"X"}
// anon slice             {}                                       => &[] | UnmarshalTypeError{Value:"object" Type:[]struct { A int } Offset:1 Struct:"" Field:""}
// anon map               []                                       => &map[] | UnmarshalTypeError{Value:"array" Type:map[string]struct {} Offset:1 Struct:"" Field:""}
// duration               "1s"                                     => 0s | UnmarshalTypeError{Value:"string" Type:time.Duration Offset:4 Struct:"" Field:""}
// time bad               1                                        => 0001-01-01 00:00:00 +0000 UTC | *errors.errorString Time.UnmarshalJSON: input is not a JSON string
// number                 "1"                                      => 0xADDR | UnmarshalTypeError{Value:"string" Type:main.Number Offset:3 Struct:"" Field:""}
// json.Number            []                                       =>  | UnmarshalTypeError{Value:"array" Type:json.Number Offset:1 Struct:"" Field:""}
// raw                    1                                        => &[49] | ok
// array                  {}                                       => &[  ] | UnmarshalTypeError{Value:"object" Type:[3]string Offset:1 Struct:"" Field:""}
// array of arrays        [[1],["x"]]                              => &[[1] [0]] | UnmarshalTypeError{Value:"string" Type:int Offset:9 Struct:"" Field:""}
// chan                   1                                        => 0xADDR | UnmarshalTypeError{Value:"number" Type:chan int Offset:1 Struct:"" Field:""}
// func                   1                                        => 0xADDR | UnmarshalTypeError{Value:"number" Type:func() Offset:1 Struct:"" Field:""}
// complex                1                                        => 0xADDR | UnmarshalTypeError{Value:"number" Type:complex128 Offset:1 Struct:"" Field:""}
// uintptr                -1                                       => 0xADDR | UnmarshalTypeError{Value:"number -1" Type:uintptr Offset:2 Struct:"" Field:""}
// map chan               {"a":1}                                  => &map[a:<nil>] | UnmarshalTypeError{Value:"number" Type:chan int Offset:6 Struct:"" Field:""}
// struct with chan       {"Keep":1,"Chan":2}                      => &{Keep:1 Chan:<nil> Skip:<nil> hidden:<nil>} | ok
// bad chan field         {"C":1}                                  => &{C:<nil>} | UnmarshalTypeError{Value:"number" Type:chan int Offset:6 Struct:"BadChan" Field:"C"}
// bad chan null          {"C":null}                               => &{C:<nil>} | ok
// bad chan absent        {}                                       => &{C:<nil>} | ok
// bad func field         {"F":1}                                  => &{F:<nil>} | UnmarshalTypeError{Value:"number" Type:func() Offset:6 Struct:"BadFunc" Field:"F"}
// == unsupported kinds on encode
// chan                    | UnsupportedTypeError{chan int} json: unsupported type: chan int
// func                    | UnsupportedTypeError{func()} json: unsupported type: func()
// complex                 | UnsupportedTypeError{complex128} json: unsupported type: complex128
// bad chan struct         | UnsupportedTypeError{chan int} json: unsupported type: chan int
// bad func struct         | UnsupportedTypeError{func()} json: unsupported type: func()
// bad complex struct      | UnsupportedTypeError{complex64} json: unsupported type: complex64
// bad nested              | UnsupportedTypeError{func()} json: unsupported type: func()
// bad map value           | UnsupportedTypeError{chan int} json: unsupported type: chan int
// nil chan in iface       | UnsupportedTypeError{chan int} json: unsupported type: chan int
// unsupported skipped    {"Keep":1} | ok
// unsafe ptr             {"a":5} | ok
// chan in slice           | UnsupportedTypeError{chan int} json: unsupported type: chan int
// func in map             | UnsupportedTypeError{func()} json: unsupported type: func()
// == recursion and cycles
// tree                   {"Name":"root","Children":[{"Name":"child","Children":null,"Meta":{"self":null}},null],"Meta":null} | ok
// tree decode            {"Name":"a","Children":[{"Name":"b","Children":[{"Name":"c"}]},null],"Meta":{"x":{"Name":"y"}}} => &{Name:a Children:[0xADDR <nil>] Parent:<nil> Meta:map[x:0xADDR]} | ok
// c true y
// pointer cycle           | UnsupportedValueError{"encountered a cycle via *main.Linked"} json: unsupported value: encountered a cycle via *main.Linked
// slice cycle             | UnsupportedValueError{"encountered a cycle via []interface {}"} json: unsupported value: encountered a cycle via []interface {}
// map cycle               | UnsupportedValueError{"encountered a cycle via map[string]interface {}"} json: unsupported value: encountered a cycle via map[string]interface {}
// any cycle               | UnsupportedValueError{"encountered a cycle via *main.Linked"} json: unsupported value: encountered a cycle via *main.Linked
// shared not cycle       [{"Next":null,"Self":null},{"Next":null,"Self":null},{"Next":{"Next":null,"Self":null},"Self":null}] | ok
// 18925 <nil>
// == interface holders
// holders nil            {"Iface":null,"Any":null,"Error":null,"Stringy":null} | ok
// holders set            {"Iface":{"N":"n"},"Any":{"N":"m"},"Error":{},"Stringy":1000000000} | ok
// holders ptr nil        {"Iface":null,"Any":null,"Error":null,"Stringy":null} | ok
// any numbers            [-1,2,0.1,120,1,9223372036854775808,4,5] | ok
// any named              [1,[2],{"3":4}] | ok
// ptr chain              5 | ok
// holders decode         {"Any":{"a":[1,"b"]}}                    => &{Iface:<nil> Any:map[a:[1 b]] Error:<nil> Stringy:<nil>} | ok
// holders iface          {"Iface":{"N":"x"}}                      => &{Iface:<nil> Any:map[a:[1 b]] Error:<nil> Stringy:<nil>} | UnmarshalTypeError{Value:"object" Type:main.Iface Offset:10 Struct:"Holders" Field:"Iface"}
// holders error          {"Error":"x"}                            => &{Iface:<nil> Any:map[a:[1 b]] Error:<nil> Stringy:<nil>} | UnmarshalTypeError{Value:"string" Type:error Offset:12 Struct:"Holders" Field:"Error"}
// holders null           {"Iface":null,"Any":null}                => &{Iface:<nil> Any:<nil> Error:<nil> Stringy:<nil>} | ok
// iface direct           {"N":"x"}                                => 0xADDR | UnmarshalTypeError{Value:"object" Type:main.Iface Offset:1 Struct:"" Field:""}
// iface non-ptr          1                                        => <nil> | *json.InvalidUnmarshalError json: Unmarshal(nil)
// not pointer            1                                        => 5 | *json.InvalidUnmarshalError json: Unmarshal(non-pointer int)
// nil pointer            1                                        => <nil> | *json.InvalidUnmarshalError json: Unmarshal(nil *int)
// nil                    1                                        => <nil> | *json.InvalidUnmarshalError json: Unmarshal(nil)
// map not pointer        {"a":1}                                  => map[] | *json.InvalidUnmarshalError json: Unmarshal(non-pointer map[string]int)
// nil map by value       {"a":1}                                  => map[] | *json.InvalidUnmarshalError json: Unmarshal(non-pointer map[string]int)
