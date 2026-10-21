// phase: 09
// encoding/json corners of the reflection it depends on: pointers to slices, arrays and maps,
// embedded pointers to unexported types, RawMessage in every position, the string option on
// pointers and containers, field names that differ only by case, and interfaces embedded in structs.

package main

import (
	"encoding/json"
	"fmt"
	"regexp"
)

var address = regexp.MustCompile(`0x[0-9a-f]{8,}`)

type hiddenBase struct{ Public int }

type HiddenEmbed struct {
	*hiddenBase
	Other int
}

type ValueEmbed struct {
	hiddenBase
	Other int
}

type Named interface{ Name() string }

type Person struct{ First string }

func (p Person) Name() string { return p.First }

type EmbedsInterface struct {
	Named
	Age int
}

type EmbedsError struct {
	error
	Code int
}

type Containers struct {
	PtrSlice   *[]int
	PtrArray   *[2]string
	PtrMap     *map[string]int
	SliceOfPtr []*int
	ArrayOfPtr [2]*int
	MapOfPtr   map[string]*[]int
	PtrPtr     **[]string
	Deep       [][]map[string][]int
}

type Raws struct {
	Value   json.RawMessage
	Pointer *json.RawMessage
	Slice   []json.RawMessage
	Array   [2]json.RawMessage
	Map     map[string]json.RawMessage
	Any     any
}

type StringOpt struct {
	PtrInt  *int            `json:",string"`
	PtrBool *bool           `json:",string"`
	PtrStr  *string         `json:",string"`
	Ints    []int           `json:",string"`
	Map     map[string]int  `json:",string"`
	Struct  struct{ N int } `json:",string"`
	Iface   any             `json:",string"`
	Uint    uint16          `json:",string"`
	Float32 float32         `json:",string"`
	Named   MyInt           `json:",string"`
}

type MyInt int

type CaseTwins struct {
	Lower int `json:"x"`
	Upper int `json:"X"`
	Mixed int `json:"xY"`
	Other int `json:"XY"`
}

type Embedded struct{ Value int }

type Outer struct {
	Embedded
	Value string
}

type PointerToPointer struct {
	Inner **Embedded
}

func show(label string, value any, err error) {
	fmt.Printf("%-24s %s err=%v\n", label, address.ReplaceAllString(fmt.Sprintf("%+v", value), "0xADDR"), err)
}

func marshal(label string, value any) {
	encoded, err := json.Marshal(value)
	fmt.Printf("%-24s %s err=%v\n", label, encoded, err)
}

func main() {
	fmt.Println("== embedded unexported")
	var hidden HiddenEmbed
	err := json.Unmarshal([]byte(`{"Public":1,"Other":2}`), &hidden)
	show("pointer to unexported", hidden.Other, err)
	err = json.Unmarshal([]byte(`{"Other":2}`), &hidden)
	show("not touching it", hidden.Other, err)
	var value ValueEmbed
	err = json.Unmarshal([]byte(`{"Public":1,"Other":2}`), &value)
	show("value unexported", value, err)
	marshal("hidden nil", HiddenEmbed{Other: 1})
	marshal("hidden set", HiddenEmbed{&hiddenBase{5}, 1})
	marshal("value unexported", ValueEmbed{hiddenBase{5}, 1})

	fmt.Println("== embedded interface")
	marshal("embeds iface nil", EmbedsInterface{Age: 3})
	marshal("embeds iface", EmbedsInterface{Person{"p"}, 3})
	marshal("embeds error", EmbedsError{fmt.Errorf("x"), 1})
	var embeds EmbedsInterface
	err = json.Unmarshal([]byte(`{"Named":{"First":"x"},"Age":1}`), &embeds)
	show("decode iface field", embeds, err)
	err = json.Unmarshal([]byte(`{"Age":1}`), &embeds)
	show("decode age only", embeds, err)

	fmt.Println("== containers")
	var containers Containers
	err = json.Unmarshal([]byte(`{"PtrSlice":[1,2],"PtrArray":["a","b","c"],"PtrMap":{"k":1},"SliceOfPtr":[1,null,3],"ArrayOfPtr":[null,5],"MapOfPtr":{"a":[1],"b":null},"PtrPtr":["x"],"Deep":[[{"k":[1,2]},{}],[]]}`), &containers)
	fmt.Println(*containers.PtrSlice, *containers.PtrArray, *containers.PtrMap, len(containers.SliceOfPtr), *containers.SliceOfPtr[0], containers.SliceOfPtr[1] == nil, *containers.SliceOfPtr[2],
		containers.ArrayOfPtr[0] == nil, *containers.ArrayOfPtr[1], *containers.MapOfPtr["a"], containers.MapOfPtr["b"] == nil, **containers.PtrPtr, containers.Deep, err)
	encoded, err := json.Marshal(containers)
	fmt.Println(string(encoded), err)
	err = json.Unmarshal([]byte(`{"PtrSlice":null,"PtrMap":null,"PtrArray":null}`), &containers)
	fmt.Println(containers.PtrSlice == nil, containers.PtrMap == nil, containers.PtrArray == nil, err)
	err = json.Unmarshal([]byte(`{"PtrSlice":"x","SliceOfPtr":["y"],"Deep":[[{"k":["z"]}]]}`), &containers)
	show("errors", "", err)

	fmt.Println("== RawMessage")
	raws := Raws{Value: json.RawMessage(` {"a" : 1} `), Slice: []json.RawMessage{json.RawMessage(`[1, 2]`), nil}, Array: [2]json.RawMessage{json.RawMessage(`"x"`)}, Map: map[string]json.RawMessage{"k": json.RawMessage(`1.50`), "n": nil}}
	marshal("raws", raws)
	marshal("raws pointer", &raws)
	pointer := json.RawMessage(`true`)
	marshal("ptr set", Raws{Pointer: &pointer})
	marshal("ptr nil", Raws{})
	marshal("any raw", []any{json.RawMessage(`[ 1 ]`), &pointer, json.RawMessage(nil)})
	marshal("invalid raw", Raws{Value: json.RawMessage(`{"a":`)})
	marshal("invalid raw 2", Raws{Value: json.RawMessage(`nope`)})
	marshal("html raw", Raws{Value: json.RawMessage(`"<&>"`)})
	var decoded Raws
	err = json.Unmarshal([]byte(`{"Value": {"x" : [1, 2 ]},"Pointer": null,"Slice":[ 1 ,"a",null],"Array":[ true ],"Map":{"k": 2.0 , "n":null},"Any":{"k" : 1}}`), &decoded)
	fmt.Printf("%q %v %q %q %q %v %v\n", decoded.Value, decoded.Pointer == nil, decoded.Slice, decoded.Array, decoded.Map, decoded.Any, err)
	err = json.Unmarshal([]byte(`{"Pointer": 5}`), &decoded)
	fmt.Printf("%q %v\n", *decoded.Pointer, err)
	err = json.Unmarshal([]byte(`{"Value": null}`), &decoded)
	fmt.Printf("%q %v\n", decoded.Value, err)
	reuse := json.RawMessage(make([]byte, 0, 64))
	err = json.Unmarshal([]byte(`[1,2,3]`), &reuse)
	fmt.Printf("%s %v\n", reuse, err)
	var rawNil *json.RawMessage
	err = json.Unmarshal([]byte(`"x"`), rawNil)
	fmt.Println(err)

	fmt.Println("== string option on pointers and containers")
	number, truth, text := 5, true, "s"
	marshal("string opt", StringOpt{PtrInt: &number, PtrBool: &truth, PtrStr: &text, Ints: []int{1}, Map: map[string]int{"a": 1}, Iface: 3, Uint: 7, Float32: 1.5, Named: 9})
	marshal("string opt zero", StringOpt{})
	var option StringOpt
	err = json.Unmarshal([]byte(`{"PtrInt":"6","PtrBool":"false","PtrStr":"\"q\"","Ints":[1,2],"Uint":"8","Float32":"2.5","Named":"11","Iface":"x"}`), &option)
	fmt.Println(*option.PtrInt, *option.PtrBool, *option.PtrStr, option.Ints, option.Uint, option.Float32, option.Named, option.Iface, err)
	for _, input := range []string{`{"PtrInt":6}`, `{"PtrInt":null}`, `{"PtrInt":"null"}`, `{"Ints":["1"]}`, `{"Struct":{"N":"1"}}`, `{"Named":5}`, `{"Uint":"-1"}`, `{"PtrStr":"abc"}`, `{"PtrBool":"1"}`} {
		var attempt StringOpt
		err := json.Unmarshal([]byte(input), &attempt)
		fmt.Printf("%-24s ptrint=%v named=%v err=%v\n", input, attempt.PtrInt != nil, attempt.Named, err)
	}

	fmt.Println("== names that differ by case")
	var twins CaseTwins
	err = json.Unmarshal([]byte(`{"x":1,"X":2,"xy":3,"XY":4,"xY":5,"Xy":6}`), &twins)
	show("twins", twins, err)
	twins = CaseTwins{}
	err = json.Unmarshal([]byte(`{"x":1}`), &twins)
	show("only x", twins, err)
	twins = CaseTwins{}
	err = json.Unmarshal([]byte(`{"X":1}`), &twins)
	show("only X", twins, err)
	twins = CaseTwins{}
	err = json.Unmarshal([]byte(`{"xy":1}`), &twins)
	show("only xy", twins, err)
	marshal("twins", CaseTwins{1, 2, 3, 4})

	fmt.Println("== shadowing by embedding")
	marshal("outer", Outer{Embedded{1}, "outer"})
	var outer Outer
	err = json.Unmarshal([]byte(`{"Value":"v"}`), &outer)
	show("outer decode", outer, err)
	err = json.Unmarshal([]byte(`{"Value":5}`), &outer)
	show("outer decode number", outer, err)
	var chain PointerToPointer
	err = json.Unmarshal([]byte(`{"Inner":{"Value":4}}`), &chain)
	fmt.Println((**chain.Inner).Value, err)
	err = json.Unmarshal([]byte(`{"Inner":null}`), &chain)
	fmt.Println(chain.Inner == nil, err)
}

// Output:
// == embedded unexported
// pointer to unexported    2 err=json: cannot set embedded pointer to unexported struct: main.hiddenBase
// not touching it          2 err=<nil>
// value unexported         {hiddenBase:{Public:1} Other:2} err=<nil>
// hidden nil               {"Other":1} err=<nil>
// hidden set               {"Public":5,"Other":1} err=<nil>
// value unexported         {"Public":5,"Other":1} err=<nil>
// == embedded interface
// embeds iface nil         {"Named":null,"Age":3} err=<nil>
// embeds iface             {"Named":{"First":"p"},"Age":3} err=<nil>
// embeds error             {"Code":1} err=<nil>
// decode iface field       {Named:<nil> Age:1} err=json: cannot unmarshal object into Go struct field EmbedsInterface.Named of type main.Named
// decode age only          {Named:<nil> Age:1} err=<nil>
// == containers
// [1 2] [a b] map[k:1] 3 1 true 3 true 5 [1] true [x] [[map[k:[1 2]] map[]] []] <nil>
// {"PtrSlice":[1,2],"PtrArray":["a","b"],"PtrMap":{"k":1},"SliceOfPtr":[1,null,3],"ArrayOfPtr":[null,5],"MapOfPtr":{"a":[1],"b":null},"PtrPtr":["x"],"Deep":[[{"k":[1,2]},{}],[]]} <nil>
// true true true <nil>
// errors                    err=json: cannot unmarshal string into Go struct field Containers.PtrSlice of type []int
// == RawMessage
// raws                     {"Value":{"a":1},"Pointer":null,"Slice":[[1,2],null],"Array":["x",null],"Map":{"k":1.50,"n":null},"Any":null} err=<nil>
// raws pointer             {"Value":{"a":1},"Pointer":null,"Slice":[[1,2],null],"Array":["x",null],"Map":{"k":1.50,"n":null},"Any":null} err=<nil>
// ptr set                  {"Value":null,"Pointer":true,"Slice":null,"Array":[null,null],"Map":null,"Any":null} err=<nil>
// ptr nil                  {"Value":null,"Pointer":null,"Slice":null,"Array":[null,null],"Map":null,"Any":null} err=<nil>
// any raw                  [[1],true,null] err=<nil>
// invalid raw               err=json: error calling MarshalJSON for type json.RawMessage: unexpected end of JSON input
// invalid raw 2             err=json: error calling MarshalJSON for type json.RawMessage: invalid character 'o' in literal null (expecting 'u')
// html raw                 {"Value":"\u003c\u0026\u003e","Pointer":null,"Slice":null,"Array":[null,null],"Map":null,"Any":null} err=<nil>
// "{\"x\" : [1, 2 ]}" true ["1" "\"a\"" "null"] ["true" ""] map["k":"2.0" "n":"null"] map[k:1] <nil>
// "5" <nil>
// "null" <nil>
// [1,2,3] <nil>
// json: Unmarshal(nil *json.RawMessage)
// == string option on pointers and containers
// string opt               {"PtrInt":"5","PtrBool":"true","PtrStr":"\"s\"","Ints":[1],"Map":{"a":1},"Struct":{"N":0},"Iface":3,"Uint":"7","Float32":"1.5","Named":"9"} err=<nil>
// string opt zero          {"PtrInt":null,"PtrBool":null,"PtrStr":null,"Ints":null,"Map":null,"Struct":{"N":0},"Iface":null,"Uint":"0","Float32":"0","Named":"0"} err=<nil>
// 6 false q [1 2] 8 2.5 11 x <nil>
// {"PtrInt":6}             ptrint=false named=0 err=json: invalid use of ,string struct tag, trying to unmarshal unquoted value into *int
// {"PtrInt":null}          ptrint=false named=0 err=<nil>
// {"PtrInt":"null"}        ptrint=false named=0 err=<nil>
// {"Ints":["1"]}           ptrint=false named=0 err=json: cannot unmarshal string into Go struct field StringOpt.Ints of type int
// {"Struct":{"N":"1"}}     ptrint=false named=0 err=json: cannot unmarshal string into Go struct field .Struct.N of type int
// {"Named":5}              ptrint=false named=0 err=json: invalid use of ,string struct tag, trying to unmarshal unquoted value into main.MyInt
// {"Uint":"-1"}            ptrint=false named=0 err=json: cannot unmarshal number -1 into Go struct field StringOpt.Uint of type uint16
// {"PtrStr":"abc"}         ptrint=false named=0 err=json: invalid use of ,string struct tag, trying to unmarshal "abc" into string
// {"PtrBool":"1"}          ptrint=false named=0 err=json: invalid use of ,string struct tag, trying to unmarshal "1" into bool
// == names that differ by case
// twins                    {Lower:1 Upper:2 Mixed:6 Other:4} err=<nil>
// only x                   {Lower:1 Upper:0 Mixed:0 Other:0} err=<nil>
// only X                   {Lower:0 Upper:1 Mixed:0 Other:0} err=<nil>
// only xy                  {Lower:0 Upper:0 Mixed:1 Other:0} err=<nil>
// twins                    {"x":1,"X":2,"xY":3,"XY":4} err=<nil>
// == shadowing by embedding
// outer                    {"Value":"outer"} err=<nil>
// outer decode             {Embedded:{Value:0} Value:v} err=<nil>
// outer decode number      {Embedded:{Value:0} Value:v} err=json: cannot unmarshal number into Go struct field Outer.Value of type string
// 4 <nil>
// true <nil>
