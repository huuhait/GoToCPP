// phase: 09

// #Derive(JSON) on the struct shapes that programs use (exit criterion of Phase 10: the output matches
// encoding/json). Every section is one kind of shape: field order with embedded structs, nil and empty
// slices and maps with and without omitempty, pointers, floats and integers at their limits, text
// escaping, omitempty and omitzero, map keys, interface fields, and decoding into values that already
// hold data. The twin (testdata/derive_twin/derive_json_shapes.go) is the same program without #Derive, so
// real Go's encoding/json on the plain structs prints the expected output.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func show(label string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		return
	}
	fmt.Printf("%s: %s\n", label, data)
}

func decode(label string, input string, target any) {
	err := json.Unmarshal([]byte(input), target)
	data, _ := json.Marshal(target)
	fmt.Printf("%s: err=%s value=%s\n", label, describe(err), data)
}

// describe prints an error without the name of the struct type: the decoder of a derived type
// works on a copy of the struct (gp_plain) and names that one in UnmarshalTypeError.Struct and
// in the text of the error (known divergence, ADR-102).
func describe(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fmt.Sprintf(
			"type error (value %q, field %q, offset %d)", typeErr.Value, typeErr.Field, typeErr.Offset,
		)
	}
	return fmt.Sprint(err)
}

// ---- field order and embedding ----

type Inner struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Deep string
}

type Mixin struct {
	Name  string `json:"name"`
	Extra int    `json:"extra"`
	Deep  string
}

type Left struct {
	Clash int
	Tag   string `json:"left_tag"`
}

type Right struct {
	Clash int
	Tag   string `json:"left_tag"`
}

type hidden struct {
	Visible int `json:"visible"`
}

#Derive(JSON)
type Ordered struct {
	Zulu string `json:"zulu"`
	Inner
	Alpha string `json:"alpha"`
	*Mixin
	Last int `json:"last"`
}

#Derive(JSON)
type Conflicts struct {
	Left
	Right
	Own int `json:"own"`
}

#Derive(JSON)
type Shadow struct {
	Inner
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

#Derive(JSON)
type WithHidden struct {
	hidden
	Shown int `json:"shown"`
}

#Derive(JSON)
type Renamed struct {
	Inner `json:"inner"`
	Plain int `json:"-"`
	Dash  int `json:"-,"`
	Comma int `json:",omitempty"`
	Empty int `json:""`
	Odd   int `json:"a b\"c<d>"`
	Uni   int `json:"nome-é"`
}

// ---- nil and empty containers, pointers ----

type Leaf struct {
	N int `json:"n"`
}

#Derive(JSON)
type Containers struct {
	NilSlice     []int             `json:"nil_slice"`
	EmptySlice   []int             `json:"empty_slice"`
	FullSlice    []int             `json:"full_slice"`
	NilMap       map[string]int    `json:"nil_map"`
	EmptyMap     map[string]int    `json:"empty_map"`
	OmitNilSlice []int             `json:"omit_nil_slice,omitempty"`
	OmitEmpty    []int             `json:"omit_empty_slice,omitempty"`
	OmitMap      map[string]int    `json:"omit_map,omitempty"`
	OmitEmptyMap map[string]int    `json:"omit_empty_map,omitempty"`
	NilBytes     []byte            `json:"nil_bytes"`
	EmptyBytes   []byte            `json:"empty_bytes"`
	Bytes        []byte            `json:"bytes"`
	Array        [3]int            `json:"array"`
	ZeroArray    [0]int            `json:"zero_array"`
	OmitZeroArr  [0]int            `json:"omit_zero_array,omitempty"`
	OmitArray    [2]int            `json:"omit_array,omitempty"`
	Nested       [][]string        `json:"nested"`
	MapSlices    map[string][]Leaf `json:"map_slices"`
	Ptr          *int              `json:"ptr"`
	PtrPtr       **int             `json:"ptr_ptr"`
	PtrStruct    *Leaf             `json:"ptr_struct"`
	OmitPtr      *Leaf             `json:"omit_ptr,omitempty"`
	Leaves       []*Leaf           `json:"leaves"`
}

// ---- numbers ----

#Derive(JSON)
type Numbers struct {
	Small   float64 `json:"small"`
	Tiny    float64 `json:"tiny"`
	Huge    float64 `json:"huge"`
	Edge21  float64 `json:"edge21"`
	Edge20  float64 `json:"edge20"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	NegZero float64 `json:"neg_zero"`
	Third   float64 `json:"third"`
	Single  float32 `json:"single"`
	Big32   float32 `json:"big32"`
	I8      int8    `json:"i8"`
	U8      uint8   `json:"u8"`
	I64     int64   `json:"i64"`
	U64     uint64  `json:"u64"`
	Quoted  float64 `json:"quoted,string"`
	QInt    uint64  `json:"qint,string"`
	QBool   bool    `json:"qbool,string"`
	QStr    string  `json:"qstr,string"`
}

#Derive(JSON)
type Single struct {
	Value float64 `json:"value"`
}

// ---- strings ----

#Derive(JSON)
type Texts struct {
	Plain   string            `json:"plain"`
	Accents string            `json:"accents"`
	Emoji   string            `json:"emoji"`
	Html    string            `json:"html"`
	Control string            `json:"control"`
	Lines   string            `json:"lines"`
	Invalid string            `json:"invalid"`
	Quotes  string            `json:"quotes"`
	Del     string            `json:"del"`
	Keys    map[string]string `json:"keys"`
}

// ---- omitempty and omitzero ----

type Marker struct {
	On bool `json:"on"`
}

func (m Marker) IsZero() bool { return !m.On }

#Derive(JSON)
type Omit struct {
	Int       int               `json:"int,omitempty"`
	Float     float64           `json:"float,omitempty"`
	Str       string            `json:"str,omitempty"`
	Bool      bool              `json:"bool,omitempty"`
	Iface     any               `json:"iface,omitempty"`
	Ptr       *int              `json:"ptr,omitempty"`
	Struct    Leaf              `json:"struct,omitempty"`
	ZeroStr   Leaf              `json:"zero_struct,omitzero"`
	Time      time.Time         `json:"time,omitzero"`
	OldTime   time.Time         `json:"old_time,omitempty"`
	Marker    Marker            `json:"marker,omitzero"`
	NilSlice  []int             `json:"nil_slice,omitzero"`
	EmptySl   []int             `json:"empty_slice,omitzero"`
	EmptyMap  map[string]string `json:"empty_map,omitzero"`
	Both      []int             `json:"both,omitempty,omitzero"`
	NegZeroF  float64           `json:"neg_zero_f,omitzero"`
	NegZeroEm float64           `json:"neg_zero_em,omitempty"`
}

// ---- map keys and interface fields ----

type Level int

func (l Level) MarshalText() ([]byte, error) { return []byte(fmt.Sprintf("level-%d", int(l))), nil }

func (l *Level) UnmarshalText(text []byte) error {
	_, err := fmt.Sscanf(string(text), "level-%d", (*int)(l))
	return err
}

type Name string

#Derive(JSON)
type Keys struct {
	Strings map[string]int            `json:"strings"`
	Ints    map[int]string            `json:"ints"`
	Uints   map[uint8]bool            `json:"uints"`
	Named   map[Name]int              `json:"named"`
	Texts   map[Level]string          `json:"texts"`
	Unicode map[string]int            `json:"unicode"`
	Nested  map[string]map[string]int `json:"nested"`
}

#Derive(JSON)
type Bad struct {
	Keys map[bool]int `json:"keys"`
}

#Derive(JSON)
type Dynamic struct {
	Nothing any            `json:"nothing"`
	Number  any            `json:"number"`
	Text    any            `json:"text"`
	List    any            `json:"list"`
	Struct  any            `json:"struct"`
	Pointer any            `json:"pointer"`
	Derived any            `json:"derived"`
	Map     map[string]any `json:"map"`
	Error   error          `json:"error"`
}

#Derive(JSON)
type Tree struct {
	Value    int     `json:"value"`
	Children []*Tree `json:"children,omitempty"`
}

// ---- types that cannot be encoded ----

#Derive(JSON)
type Unsupported struct {
	Work func() `json:"work"`
}

#Derive(JSON)
type Channel struct {
	Events chan int `json:"events"`
}

func main() {
	// Field order: declaration order, embedded fields where they are written, the shallowest and
	// the tagged names win, equal depths cancel each other.
	show("ordered", Ordered{
		Zulu:  "z",
		Inner: Inner{ID: 1, Name: "inner", Deep: "d1"},
		Alpha: "a",
		Mixin: &Mixin{Name: "mixin", Extra: 2, Deep: "d2"},
		Last:  9,
	})
	show("ordered nil embedded", Ordered{Zulu: "z", Last: 1})
	show("conflicts", Conflicts{
		Left:  Left{Clash: 1, Tag: "l"},
		Right: Right{Clash: 2, Tag: "r"},
		Own:   3,
	})
	show("shadow", Shadow{Inner: Inner{ID: 5, Name: "deep", Deep: "x"}, Name: "shallow", ID: "id"})
	show("shadow omitted", Shadow{Inner: Inner{ID: 5}})
	show("hidden", WithHidden{hidden: hidden{Visible: 7}, Shown: 8})
	show("renamed", Renamed{
		Inner: Inner{ID: 1}, Plain: 2, Dash: 3, Comma: 4, Empty: 5, Odd: 6, Uni: 7,
	})
	show("renamed omit", Renamed{})

	// Nil and empty containers.
	show("containers zero", Containers{})
	number := 7
	numberPtr := &number
	show("containers set", Containers{
		EmptySlice:   []int{},
		FullSlice:    []int{1, 2},
		EmptyMap:     map[string]int{},
		OmitEmpty:    []int{},
		OmitMap:      map[string]int{"b": 2, "a": 1},
		OmitEmptyMap: map[string]int{},
		EmptyBytes:   []byte{},
		Bytes:        []byte("any bytes \x00\xff"),
		Array:        [3]int{1, 2, 3},
		OmitArray:    [2]int{},
		Nested:       [][]string{nil, {}, {"x"}},
		MapSlices:    map[string][]Leaf{"k": {{1}, {2}}, "e": nil},
		Ptr:          numberPtr,
		PtrPtr:       &numberPtr,
		PtrStruct:    &Leaf{N: 3},
		OmitPtr:      &Leaf{},
		Leaves:       []*Leaf{nil, {N: 4}},
	})

	// Numbers.
	show("numbers zero", Numbers{})
	show("numbers set", Numbers{
		Small:   0.1,
		Tiny:    1e-7,
		Huge:    1e300,
		Edge21:  1e21,
		Edge20:  1e20,
		Min:     5e-324,
		Max:     math.MaxFloat64,
		NegZero: math.Copysign(0, -1),
		Third:   1.0 / 3.0,
		Single:  0.1,
		Big32:   math.MaxFloat32,
		I8:      -128,
		U8:      255,
		I64:     math.MinInt64,
		U64:     math.MaxUint64,
		Quoted:  2.5,
		QInt:    math.MaxUint64,
		QBool:   true,
		QStr:    "say \"hi\"",
	})
	show("nan", Single{math.NaN()})
	show("inf", Single{math.Inf(1)})
	show("neg inf", Single{math.Inf(-1)})
	values := []float64{1e-6, 1e-7, 123456789, 1e20, 1e21, 12345678901234567890, 0.000001234}
	for _, value := range values {
		show("float", Single{value})
	}

	// Strings.
	show("texts", Texts{
		Plain:   "plain",
		Accents: "héllo 日本",
		Emoji:   "\U0001F600 and \U0001F468‍\U0001F469",
		Html:    "<script>alert('x' & \"y\")</script>",
		Control: "\x00\x01\b\f\n\r\t\x1f",
		Lines:   "a b c",
		Invalid: "bad \xff\xfe utf8 \xc3",
		Quotes:  "\"quoted\" and back\\slash and /slash",
		Del:     "\x7f",
		Keys:    map[string]string{"kéy": "v", "<k>": "&", "\n": " ", "": "empty key"},
	})

	// omitempty and omitzero.
	show("omit zero", Omit{})
	one := 1
	show("omit set", Omit{
		Int:       1,
		Float:     0.5,
		Str:       "s",
		Bool:      true,
		Iface:     0,
		Ptr:       &one,
		Struct:    Leaf{},
		ZeroStr:   Leaf{N: 1},
		Time:      time.Date(2024, 2, 29, 1, 2, 3, 4, time.UTC),
		OldTime:   time.Time{},
		Marker:    Marker{On: true},
		NilSlice:  []int{},
		EmptySl:   []int{},
		EmptyMap:  map[string]string{},
		Both:      []int{},
		NegZeroF:  math.Copysign(0, -1),
		NegZeroEm: math.Copysign(0, -1),
	})

	// Map keys, interface fields and recursion.
	show("keys zero", Keys{})
	show("keys set", Keys{
		Strings: map[string]int{"b": 1, "a": 2, "": 3},
		Ints:    map[int]string{10: "ten", 9: "nine", -1: "minus"},
		Uints:   map[uint8]bool{200: true, 3: false},
		Named:   map[Name]int{"y": 1, "x": 2},
		Texts:   map[Level]string{3: "c", 1: "a", 2: "b"},
		Unicode: map[string]int{"é": 1, "z": 2, "\U0001F600": 3},
		Nested:  map[string]map[string]int{"o": {"b": 1, "a": 2}, "n": nil},
	})
	show("bad keys", Bad{Keys: map[bool]int{true: 1}})
	show("dynamic zero", Dynamic{})
	show("dynamic set", Dynamic{
		Number:  1.5,
		Text:    "t",
		List:    []any{1, "x", nil, []int{1}},
		Struct:  Leaf{N: 1},
		Pointer: &Leaf{N: 2},
		Derived: Tree{Value: 1, Children: []*Tree{{Value: 2}, nil}},
		Map:     map[string]any{"z": nil, "a": []string{"q"}},
		Error:   nil,
	})
	grandchild := &Tree{Value: 3}
	child := &Tree{Value: 2, Children: []*Tree{grandchild}}
	show("tree", Tree{Value: 1, Children: []*Tree{child, {Value: 4}}})
	show("tree slice", []Tree{{Value: 1}, {Value: 2}})
	show("tree map", map[string]Tree{"b": {Value: 2}, "a": {Value: 1}})
	show("unsupported", Unsupported{})
	show("channel", Channel{})

	// Decoding into empty values.
	decode(
		"decode ordered",
		`{"zulu":"z","id":4,"name":"n","alpha":"a","extra":5,"Deep":"d","last":6}`,
		&Ordered{},
	)
	decode("decode shadow", `{"name":"top","id":"s","Deep":"deep"}`, &Shadow{})
	decode("decode hidden", `{"visible":1,"shown":2}`, &WithHidden{})
	decode("decode case", `{"ZULU":"upper","Alpha":"mixed","LAST":3}`, &Ordered{})
	decode("decode duplicate", `{"last":1,"last":2,"zulu":"a","zulu":"b"}`, &Ordered{})
	decode("decode unknown", `{"nope":{"deep":[1,2,{"x":null}]},"last":8}`, &Ordered{})
	decode("decode nulls", `{"zulu":null,"inner":null,"last":null}`, &Ordered{})
	decode("decode containers", `{
		"nil_slice":[1],"empty_slice":[],"nil_map":{"a":1},"bytes":"AQL/",
		"array":[1,2,3,4,5],"ptr":5,"ptr_ptr":6,"ptr_struct":{"n":7},"leaves":[null,{"n":1}]
	}`, &Containers{})
	decode("decode short array", `{"array":[9]}`, &Containers{})
	decode("decode numbers", `{
		"small":1e2,"i8":127,"u8":0,"i64":-9223372036854775808,"u64":18446744073709551615,
		"quoted":"3.5","qint":"7","qbool":"true","qstr":"\"x\""
	}`, &Numbers{})
	decode("decode overflow", `{"i8":128}`, &Numbers{})
	decode("decode negative unsigned", `{"u8":-1}`, &Numbers{})
	decode("decode fraction into int", `{"i64":1.5}`, &Numbers{})
	decode(
		"decode keys",
		`{"strings":{"a":1},"ints":{"5":"five"},"uints":{"7":true},"texts":{"level-4":"d"}}`,
		&Keys{},
	)
	decode("decode bad int key", `{"ints":{"x":"v"}}`, &Keys{})
	decode(
		"decode dynamic",
		`{"nothing":null,"number":1,"text":"t","list":[1,"a",null],"map":{"k":{"n":1}}}`,
		&Dynamic{},
	)
	decode("decode error iface", `{"error":"text"}`, &Dynamic{})
	decode(
		"decode tree",
		`{"value":1,"children":[{"value":2,"children":[{"value":3}]},null]}`,
		&Tree{},
	)
	decode("decode escapes", `{"plain":"é😀\n","invalid":"\ud800 lone"}`, &Texts{})
	decode("decode syntax", `{"plain":"x",}`, &Texts{})
	decode("decode type", `{"plain":5}`, &Texts{})
	decode("decode truncated", `{"plain":"x"`, &Texts{})
	decode("decode array for struct", `[1,2]`, &Texts{})

	// Decoding into values that hold data: slices restart, maps merge, pointers are reused.
	slice := []int{9, 9, 9, 9}
	existing := Containers{
		FullSlice: slice,
		NilMap:    map[string]int{"old": 1, "keep": 2},
		Ptr:       numberPtr,
		PtrStruct: &Leaf{N: 100},
		Array:     [3]int{7, 7, 7},
	}
	input := `{"full_slice":[1,2],"nil_map":{"keep":3,"new":4},"ptr":8,"ptr_struct":{},"array":[1]}`
	err := json.Unmarshal([]byte(input), &existing)
	data, _ := json.Marshal(existing)
	fmt.Printf("prefilled: err=%v value=%s\n", err, data)
	fmt.Println("prefilled: slice shared:", &existing.FullSlice[0] == &slice[0], "tail:", slice)
	fmt.Println("prefilled: pointer shared:", existing.Ptr == numberPtr, number)

	// A stream of values and an encoder with other options.
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", " ")
	encoder.Encode(Texts{Html: "<b>&</b>", Plain: "p"})
	encoder.Encode([]Leaf{{1}, {2}})
	fmt.Print(out.String())
	decoder := json.NewDecoder(strings.NewReader(`{"value":1} {"value":2,"extra":true} [{"value":3}]`))
	decoder.DisallowUnknownFields()
	var first, second Tree
	var many []Tree
	fmt.Println(decoder.Decode(&first), first.Value)
	fmt.Println(decoder.Decode(&second), second.Value)
	fmt.Println(decoder.Decode(&many), len(many))
}

// Output:
// ordered: {"zulu":"z","id":1,"alpha":"a","extra":2,"last":9}
// ordered nil embedded: {"zulu":"z","id":0,"alpha":"","last":1}
// conflicts: {"own":3}
// shadow: {"Deep":"x","name":"shallow","id":"id"}
// shadow omitted: {"Deep":"","name":""}
// hidden: {"visible":7,"shown":8}
// renamed: {"inner":{"id":1,"name":"","Deep":""},"-":3,"Comma":4,"Empty":5,"Odd":6,"nome-é":7}
// renamed omit: {"inner":{"id":0,"name":"","Deep":""},"-":0,"Empty":0,"Odd":0,"nome-é":0}
// containers zero: {"nil_slice":null,"empty_slice":null,"full_slice":null,"nil_map":null,"empty_map":null,"nil_bytes":null,"empty_bytes":null,"bytes":null,"array":[0,0,0],"zero_array":[],"omit_array":[0,0],"nested":null,"map_slices":null,"ptr":null,"ptr_ptr":null,"ptr_struct":null,"leaves":null}
// containers set: {"nil_slice":null,"empty_slice":[],"full_slice":[1,2],"nil_map":null,"empty_map":{},"omit_map":{"a":1,"b":2},"nil_bytes":null,"empty_bytes":"","bytes":"YW55IGJ5dGVzIAD/","array":[1,2,3],"zero_array":[],"omit_array":[0,0],"nested":[null,[],["x"]],"map_slices":{"e":null,"k":[{"n":1},{"n":2}]},"ptr":7,"ptr_ptr":7,"ptr_struct":{"n":3},"omit_ptr":{"n":0},"leaves":[null,{"n":4}]}
// numbers zero: {"small":0,"tiny":0,"huge":0,"edge21":0,"edge20":0,"min":0,"max":0,"neg_zero":0,"third":0,"single":0,"big32":0,"i8":0,"u8":0,"i64":0,"u64":0,"quoted":"0","qint":"0","qbool":"false","qstr":"\"\""}
// numbers set: {"small":0.1,"tiny":1e-7,"huge":1e+300,"edge21":1e+21,"edge20":100000000000000000000,"min":5e-324,"max":1.7976931348623157e+308,"neg_zero":-0,"third":0.3333333333333333,"single":0.1,"big32":3.4028235e+38,"i8":-128,"u8":255,"i64":-9223372036854775808,"u64":18446744073709551615,"quoted":"2.5","qint":"18446744073709551615","qbool":"true","qstr":"\"say \\\"hi\\\"\""}
// nan: error: json: unsupported value: NaN
// inf: error: json: unsupported value: +Inf
// neg inf: error: json: unsupported value: -Inf
// float: {"value":0.000001}
// float: {"value":1e-7}
// float: {"value":123456789}
// float: {"value":100000000000000000000}
// float: {"value":1e+21}
// float: {"value":12345678901234567000}
// float: {"value":0.000001234}
// texts: {"plain":"plain","accents":"héllo 日本","emoji":"😀 and 👨‍👩","html":"\u003cscript\u003ealert('x' \u0026 \"y\")\u003c/script\u003e","control":"\u0000\u0001\b\f\n\r\t\u001f","lines":"a\u2028b\u2029c","invalid":"bad \ufffd\ufffd utf8 \ufffd","quotes":"\"quoted\" and back\\slash and /slash","del":"","keys":{"":"empty key","\n":"\u2028","\u003ck\u003e":"\u0026","kéy":"v"}}
// omit zero: {"struct":{"n":0},"old_time":"0001-01-01T00:00:00Z"}
// omit set: {"int":1,"float":0.5,"str":"s","bool":true,"iface":0,"ptr":1,"struct":{"n":0},"zero_struct":{"n":1},"time":"2024-02-29T01:02:03.000000004Z","old_time":"0001-01-01T00:00:00Z","marker":{"on":true},"nil_slice":[],"empty_slice":[],"empty_map":{}}
// keys zero: {"strings":null,"ints":null,"uints":null,"named":null,"texts":null,"unicode":null,"nested":null}
// keys set: {"strings":{"":3,"a":2,"b":1},"ints":{"-1":"minus","10":"ten","9":"nine"},"uints":{"200":true,"3":false},"named":{"x":2,"y":1},"texts":{"level-1":"a","level-2":"b","level-3":"c"},"unicode":{"z":2,"é":1,"😀":3},"nested":{"n":null,"o":{"a":2,"b":1}}}
// bad keys: error: json: unsupported type: map[bool]int
// dynamic zero: {"nothing":null,"number":null,"text":null,"list":null,"struct":null,"pointer":null,"derived":null,"map":null,"error":null}
// dynamic set: {"nothing":null,"number":1.5,"text":"t","list":[1,"x",null,[1]],"struct":{"n":1},"pointer":{"n":2},"derived":{"value":1,"children":[{"value":2},null]},"map":{"a":["q"],"z":null},"error":null}
// tree: {"value":1,"children":[{"value":2,"children":[{"value":3}]},{"value":4}]}
// tree slice: [{"value":1},{"value":2}]
// tree map: {"a":{"value":1},"b":{"value":2}}
// unsupported: error: json: unsupported type: func()
// channel: error: json: unsupported type: chan int
// decode ordered: err=<nil> value={"zulu":"z","id":4,"alpha":"a","extra":5,"last":6}
// decode shadow: err=<nil> value={"Deep":"deep","name":"top","id":"s"}
// decode hidden: err=<nil> value={"visible":1,"shown":2}
// decode case: err=<nil> value={"zulu":"upper","id":0,"alpha":"mixed","last":3}
// decode duplicate: err=<nil> value={"zulu":"b","id":0,"alpha":"","last":2}
// decode unknown: err=<nil> value={"zulu":"","id":0,"alpha":"","last":8}
// decode nulls: err=<nil> value={"zulu":"","id":0,"alpha":"","last":0}
// decode containers: err=<nil> value={"nil_slice":[1],"empty_slice":[],"full_slice":null,"nil_map":{"a":1},"empty_map":null,"nil_bytes":null,"empty_bytes":null,"bytes":"AQL/","array":[1,2,3],"zero_array":[],"omit_array":[0,0],"nested":null,"map_slices":null,"ptr":5,"ptr_ptr":6,"ptr_struct":{"n":7},"leaves":[null,{"n":1}]}
// decode short array: err=<nil> value={"nil_slice":null,"empty_slice":null,"full_slice":null,"nil_map":null,"empty_map":null,"nil_bytes":null,"empty_bytes":null,"bytes":null,"array":[9,0,0],"zero_array":[],"omit_array":[0,0],"nested":null,"map_slices":null,"ptr":null,"ptr_ptr":null,"ptr_struct":null,"leaves":null}
// decode numbers: err=<nil> value={"small":100,"tiny":0,"huge":0,"edge21":0,"edge20":0,"min":0,"max":0,"neg_zero":0,"third":0,"single":0,"big32":0,"i8":127,"u8":0,"i64":-9223372036854775808,"u64":18446744073709551615,"quoted":"3.5","qint":"7","qbool":"true","qstr":"\"x\""}
// decode overflow: err=type error (value "number 128", field "i8", offset 9) value={"small":0,"tiny":0,"huge":0,"edge21":0,"edge20":0,"min":0,"max":0,"neg_zero":0,"third":0,"single":0,"big32":0,"i8":0,"u8":0,"i64":0,"u64":0,"quoted":"0","qint":"0","qbool":"false","qstr":"\"\""}
// decode negative unsigned: err=type error (value "number -1", field "u8", offset 8) value={"small":0,"tiny":0,"huge":0,"edge21":0,"edge20":0,"min":0,"max":0,"neg_zero":0,"third":0,"single":0,"big32":0,"i8":0,"u8":0,"i64":0,"u64":0,"quoted":"0","qint":"0","qbool":"false","qstr":"\"\""}
// decode fraction into int: err=type error (value "number 1.5", field "i64", offset 10) value={"small":0,"tiny":0,"huge":0,"edge21":0,"edge20":0,"min":0,"max":0,"neg_zero":0,"third":0,"single":0,"big32":0,"i8":0,"u8":0,"i64":0,"u64":0,"quoted":"0","qint":"0","qbool":"false","qstr":"\"\""}
// decode keys: err=<nil> value={"strings":{"a":1},"ints":{"5":"five"},"uints":{"7":true},"named":null,"texts":{"level-4":"d"},"unicode":null,"nested":null}
// decode bad int key: err=type error (value "number x", field "ints", offset 10) value={"strings":null,"ints":{},"uints":null,"named":null,"texts":null,"unicode":null,"nested":null}
// decode dynamic: err=<nil> value={"nothing":null,"number":1,"text":"t","list":[1,"a",null],"struct":null,"pointer":null,"derived":null,"map":{"k":{"n":1}},"error":null}
// decode error iface: err=type error (value "string", field "error", offset 15) value={"nothing":null,"number":null,"text":null,"list":null,"struct":null,"pointer":null,"derived":null,"map":null,"error":null}
// decode tree: err=<nil> value={"value":1,"children":[{"value":2,"children":[{"value":3}]},null]}
// decode escapes: err=<nil> value={"plain":"é😀\n","accents":"","emoji":"","html":"","control":"","lines":"","invalid":"� lone","quotes":"","del":"","keys":null}
// decode syntax: err=invalid character '}' looking for beginning of object key string value={"plain":"","accents":"","emoji":"","html":"","control":"","lines":"","invalid":"","quotes":"","del":"","keys":null}
// decode type: err=type error (value "number", field "plain", offset 10) value={"plain":"","accents":"","emoji":"","html":"","control":"","lines":"","invalid":"","quotes":"","del":"","keys":null}
// decode truncated: err=unexpected end of JSON input value={"plain":"","accents":"","emoji":"","html":"","control":"","lines":"","invalid":"","quotes":"","del":"","keys":null}
// decode array for struct: err=type error (value "array", field "", offset 1) value={"plain":"","accents":"","emoji":"","html":"","control":"","lines":"","invalid":"","quotes":"","del":"","keys":null}
// prefilled: err=<nil> value={"nil_slice":null,"empty_slice":null,"full_slice":[1,2],"nil_map":{"keep":3,"new":4,"old":1},"empty_map":null,"nil_bytes":null,"empty_bytes":null,"bytes":null,"array":[1,0,0],"zero_array":[],"omit_array":[0,0],"nested":null,"map_slices":null,"ptr":8,"ptr_ptr":null,"ptr_struct":{"n":100},"leaves":null}
// prefilled: slice shared: true tail: [1 2 9 9]
// prefilled: pointer shared: true 8
// {
//  "plain": "p",
//  "accents": "",
//  "emoji": "",
//  "html": "<b>&</b>",
//  "control": "",
//  "lines": "",
//  "invalid": "",
//  "quotes": "",
//  "del": "",
//  "keys": null
// }
// [
//  {
//   "n": 1
//  },
//  {
//   "n": 2
//  }
// ]
// <nil> 1
// json: unknown field "extra" 2
// <nil> 1
