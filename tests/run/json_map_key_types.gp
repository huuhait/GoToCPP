// phase: 09
// encoding/json maps: every permitted key kind in both directions, key sorting, TextMarshaler
// and TextUnmarshaler keys, named key types, unsupported key types, nil and nested maps.

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

var address = regexp.MustCompile(`0x[0-9a-f]{8,}`)

type Name string

type Level int8

type Upper string

func (u Upper) MarshalText() ([]byte, error) {
	return []byte(strings.ToUpper(string(u))), nil
}

func (u *Upper) UnmarshalText(text []byte) error {
	*u = Upper(strings.ToLower(string(text)))
	return nil
}

type Point struct{ X, Y int }

func (p Point) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%d,%d", p.X, p.Y)), nil
}

func (p *Point) UnmarshalText(text []byte) error {
	_, err := fmt.Sscanf(string(text), "%d,%d", &p.X, &p.Y)
	return err
}

type FailKey string

func (FailKey) MarshalText() ([]byte, error) {
	return nil, fmt.Errorf("no key")
}

type PointerKey struct{ N int }

func (p *PointerKey) MarshalText() ([]byte, error) {
	return []byte("ptr"), nil
}

type BothKey string

func (BothKey) MarshalText() ([]byte, error) {
	return []byte("text"), nil
}

func (BothKey) MarshalJSON() ([]byte, error) {
	return []byte(`"json"`), nil
}

func marshal(label string, value any) {
	encoded, err := json.Marshal(value)
	fmt.Printf("%-18s %s err=%v\n", label, encoded, err)
}

func unmarshal[T any](label string, input string) {
	var value T
	err := json.Unmarshal([]byte(input), &value)
	fmt.Printf("%-18s %-34s => %s err=%v\n", label, input, address.ReplaceAllString(fmt.Sprint(value), "0xADDR"), err)
}

func main() {
	fmt.Println("== encode")
	marshal("string", map[string]int{"b": 2, "a": 1, "": 0, "B": 3, "é": 4, "éa": 5, "z": 6, "a b": 7})
	marshal("int", map[int]string{10: "ten", 2: "two", -1: "neg", 0: "zero", 100: "hundred"})
	marshal("int8", map[int8]bool{-128: true, 127: false})
	marshal("int64", map[int64]int{math.MinInt64: 1, math.MaxInt64: 2})
	marshal("uint", map[uint]int{3: 1, 20: 2, 100: 3})
	marshal("uint64", map[uint64]int{math.MaxUint64: 1, 0: 2})
	marshal("uint8", map[uint8]int{255: 1, 0: 2, 9: 3})
	marshal("uintptr", map[uintptr]int{7: 1})
	marshal("Name", map[Name]int{"x": 1, "w": 2})
	marshal("Level", map[Level]string{-3: "a", 3: "b", 10: "c"})
	marshal("Upper", map[Upper]int{"b": 1, "a": 2, "c": 3})
	marshal("Point", map[Point]int{{2, 1}: 1, {1, 2}: 2, {1, 10}: 3})
	marshal("PointerKey", map[PointerKey]int{{1}: 1})
	marshal("BothKey", map[BothKey]int{"a": 1})
	marshal("nil map", map[string]int(nil))
	marshal("empty map", map[string]int{})
	marshal("nil in map", map[string]*int{"a": nil})
	marshal("nested", map[string]map[int][]string{"x": {2: {"a"}, 1: nil}, "w": nil})
	marshal("any values", map[string]any{"i": 1, "f": 1.5, "s": "s", "n": nil, "l": []any{1, "a"}, "m": map[string]any{"z": 1, "a": 2}})
	marshal("keys needing escape", map[string]int{"<": 1, "\"": 2, "\\": 3, "\n": 4, "\u2028": 5, "\xff": 6})
	marshal("FailKey", map[FailKey]int{"a": 1})
	marshal("float key", map[float64]int{1.5: 1})
	marshal("bool key", map[bool]int{true: 1})
	marshal("array key", map[[2]int]int{{1, 2}: 1})
	marshal("struct key", map[struct{ A int }]int{{1}: 1})
	marshal("interface key", map[any]int{"a": 1})
	marshal("ptr key", map[*int]int{nil: 1})
	marshal("complex key", map[complex128]int{1: 1})
	marshal("slice value", map[string][]int{"a": nil, "b": {}, "c": {1}})

	fmt.Println("== decode")
	unmarshal[map[string]int]("string", `{"a":1,"b":2,"a":3}`)
	unmarshal[map[int]string]("int", `{"10":"a","-2":"b","0":"c"}`)
	unmarshal[map[int]string]("int bad", `{"1.5":"a"}`)
	unmarshal[map[int]string]("int bad2", `{"x":"a"}`)
	unmarshal[map[int]string]("int plus", `{"+1":"a"}`)
	unmarshal[map[int]string]("int space", `{" 1":"a"}`)
	unmarshal[map[int]string]("int zero pad", `{"01":"a"}`)
	unmarshal[map[int8]string]("int8 ovf", `{"128":"a"}`)
	unmarshal[map[int8]string]("int8 ok", `{"127":"a","-128":"b"}`)
	unmarshal[map[uint8]string]("uint8 ovf", `{"256":"a"}`)
	unmarshal[map[uint8]string]("uint8 neg", `{"-1":"a"}`)
	unmarshal[map[uint64]string]("uint64", `{"18446744073709551615":"a"}`)
	unmarshal[map[int64]string]("int64 ovf", `{"9223372036854775808":"a"}`)
	unmarshal[map[Name]int]("Name", `{"a":1}`)
	unmarshal[map[Level]int]("Level", `{"5":1,"500":2}`)
	unmarshal[map[Upper]int]("Upper", `{"ABC":1,"Def":2}`)
	unmarshal[map[Point]int]("Point", `{"1,2":3,"x":4}`)
	unmarshal[map[bool]int]("bool", `{"true":1}`)
	unmarshal[map[float64]int]("float", `{"1.5":1}`)
	unmarshal[map[[2]int]int]("array", `{"1":1}`)
	unmarshal[map[any]int]("any key", `{"a":1}`)
	unmarshal[map[string]int]("null", `null`)
	unmarshal[map[string]int]("wrong value", `{"a":"x","b":2}`)
	unmarshal[map[string]*int]("ptr values", `{"a":null,"b":3}`)
	unmarshal[map[string][]int]("slice values", `{"a":null,"b":[],"c":[1,2]}`)
	unmarshal[map[string]map[string]int]("nested", `{"a":{"b":1},"c":null,"d":{}}`)
	unmarshal[map[string]any]("any", `{"a":[1,{"b":null}],"c":1e2}`)

	fmt.Println("== decode into existing")
	existing := map[string]int{"keep": 1, "replace": 2}
	err := json.Unmarshal([]byte(`{"replace":3,"new":4}`), &existing)
	fmt.Println(existing, err)
	err = json.Unmarshal([]byte(`null`), &existing)
	fmt.Println(existing == nil, err)
	nestedExisting := map[string]map[string]int{"a": {"x": 1}}
	err = json.Unmarshal([]byte(`{"a":{"y":2},"b":{"z":3}}`), &nestedExisting)
	fmt.Println(nestedExisting, err)
	pointers := map[string]*Point{"p": {X: 1, Y: 2}}
	err = json.Unmarshal([]byte(`{"p":{"X":9}}`), &pointers)
	fmt.Println(*pointers["p"], err)
	structs := map[string]Point{"p": {X: 1, Y: 2}}
	err = json.Unmarshal([]byte(`{"p":{"X":9}}`), &structs)
	fmt.Println(structs, err)

	fmt.Println("== round trip")
	original := map[Point][]Upper{{1, 2}: {"a", "B"}, {0, 0}: nil, {3, 4}: {}}
	encoded, _ := json.Marshal(original)
	var back map[Point][]Upper
	err = json.Unmarshal(encoded, &back)
	reencoded, _ := json.Marshal(back)
	fmt.Printf("%s %v %s\n", encoded, err, reencoded)
}

// Output:
// == encode
// string             {"":0,"B":3,"a":1,"a b":7,"b":2,"z":6,"é":4,"éa":5} err=<nil>
// int                {"-1":"neg","0":"zero","10":"ten","100":"hundred","2":"two"} err=<nil>
// int8               {"-128":true,"127":false} err=<nil>
// int64              {"-9223372036854775808":1,"9223372036854775807":2} err=<nil>
// uint               {"100":3,"20":2,"3":1} err=<nil>
// uint64             {"0":2,"18446744073709551615":1} err=<nil>
// uint8              {"0":2,"255":1,"9":3} err=<nil>
// uintptr            {"7":1} err=<nil>
// Name               {"w":2,"x":1} err=<nil>
// Level              {"-3":"a","10":"c","3":"b"} err=<nil>
// Upper              {"a":2,"b":1,"c":3} err=<nil>
// Point              {"1,10":3,"1,2":2,"2,1":1} err=<nil>
// PointerKey          err=json: unsupported type: map[main.PointerKey]int
// BothKey            {"a":1} err=<nil>
// nil map            null err=<nil>
// empty map          {} err=<nil>
// nil in map         {"a":null} err=<nil>
// nested             {"w":null,"x":{"1":null,"2":["a"]}} err=<nil>
// any values         {"f":1.5,"i":1,"l":[1,"a"],"m":{"a":2,"z":1},"n":null,"s":"s"} err=<nil>
// keys needing escape {"\n":4,"\"":2,"\u003c":1,"\\":3,"\u2028":5,"\ufffd":6} err=<nil>
// FailKey            {"a":1} err=<nil>
// float key           err=json: unsupported type: map[float64]int
// bool key            err=json: unsupported type: map[bool]int
// array key           err=json: unsupported type: map[[2]int]int
// struct key          err=json: unsupported type: map[struct { A int }]int
// interface key       err=json: unsupported type: map[interface {}]int
// ptr key             err=json: unsupported type: map[*int]int
// complex key         err=json: unsupported type: map[complex128]int
// slice value        {"a":null,"b":[],"c":[1]} err=<nil>
// == decode
// string             {"a":1,"b":2,"a":3}                => map[a:3 b:2] err=<nil>
// int                {"10":"a","-2":"b","0":"c"}        => map[-2:b 0:c 10:a] err=<nil>
// int bad            {"1.5":"a"}                        => map[] err=json: cannot unmarshal number 1.5 into Go value of type int
// int bad2           {"x":"a"}                          => map[] err=json: cannot unmarshal number x into Go value of type int
// int plus           {"+1":"a"}                         => map[1:a] err=<nil>
// int space          {" 1":"a"}                         => map[] err=json: cannot unmarshal number  1 into Go value of type int
// int zero pad       {"01":"a"}                         => map[1:a] err=<nil>
// int8 ovf           {"128":"a"}                        => map[] err=json: cannot unmarshal number 128 into Go value of type int8
// int8 ok            {"127":"a","-128":"b"}             => map[-128:b 127:a] err=<nil>
// uint8 ovf          {"256":"a"}                        => map[] err=json: cannot unmarshal number 256 into Go value of type uint8
// uint8 neg          {"-1":"a"}                         => map[] err=json: cannot unmarshal number -1 into Go value of type uint8
// uint64             {"18446744073709551615":"a"}       => map[18446744073709551615:a] err=<nil>
// int64 ovf          {"9223372036854775808":"a"}        => map[] err=json: cannot unmarshal number 9223372036854775808 into Go value of type int64
// Name               {"a":1}                            => map[a:1] err=<nil>
// Level              {"5":1,"500":2}                    => map[5:1] err=json: cannot unmarshal number 500 into Go value of type main.Level
// Upper              {"ABC":1,"Def":2}                  => map[abc:1 def:2] err=<nil>
// Point              {"1,2":3,"x":4}                    => map[{1 2}:3] err=expected integer
// bool               {"true":1}                         => map[] err=json: cannot unmarshal object into Go value of type map[bool]int
// float              {"1.5":1}                          => map[] err=json: cannot unmarshal object into Go value of type map[float64]int
// array              {"1":1}                            => map[] err=json: cannot unmarshal object into Go value of type map[[2]int]int
// any key            {"a":1}                            => map[] err=json: cannot unmarshal object into Go value of type map[interface {}]int
// null               null                               => map[] err=<nil>
// wrong value        {"a":"x","b":2}                    => map[a:0 b:2] err=json: cannot unmarshal string into Go value of type int
// ptr values         {"a":null,"b":3}                   => map[a:<nil> b:0xADDR] err=<nil>
// slice values       {"a":null,"b":[],"c":[1,2]}        => map[a:[] b:[] c:[1 2]] err=<nil>
// nested             {"a":{"b":1},"c":null,"d":{}}      => map[a:map[b:1] c:map[] d:map[]] err=<nil>
// any                {"a":[1,{"b":null}],"c":1e2}       => map[a:[1 map[b:<nil>]] c:100] err=<nil>
// == decode into existing
// map[keep:1 new:4 replace:3] <nil>
// true <nil>
// map[a:map[y:2] b:map[z:3]] <nil>
// {0 0} json: cannot unmarshal object into Go value of type *main.Point
// map[p:{0 0}] json: cannot unmarshal object into Go value of type main.Point
// == round trip
// {"0,0":null,"1,2":["A","B"],"3,4":[]} <nil> {"0,0":null,"1,2":["A","B"],"3,4":[]}
