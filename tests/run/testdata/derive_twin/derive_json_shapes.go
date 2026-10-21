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

type Ordered struct {
	Zulu string `json:"zulu"`
	Inner
	Alpha string `json:"alpha"`
	*Mixin
	Last int `json:"last"`
}

type Conflicts struct {
	Left
	Right
	Own int `json:"own"`
}

type Shadow struct {
	Inner
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

type WithHidden struct {
	hidden
	Shown int `json:"shown"`
}

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

type Single struct {
	Value float64 `json:"value"`
}

// ---- strings ----

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

type Keys struct {
	Strings map[string]int            `json:"strings"`
	Ints    map[int]string            `json:"ints"`
	Uints   map[uint8]bool            `json:"uints"`
	Named   map[Name]int              `json:"named"`
	Texts   map[Level]string          `json:"texts"`
	Unicode map[string]int            `json:"unicode"`
	Nested  map[string]map[string]int `json:"nested"`
}

type Bad struct {
	Keys map[bool]int `json:"keys"`
}

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

type Tree struct {
	Value    int     `json:"value"`
	Children []*Tree `json:"children,omitempty"`
}

// ---- types that cannot be encoded ----

type Unsupported struct {
	Work func() `json:"work"`
}

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
