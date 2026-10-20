// phase: 09

// #Derive(JSON), nesting (step 9.11 review): a value of a derived type is encoded and decoded as the struct it stands for, directly, as a struct
// without methods is. The methods MarshalJSON and UnmarshalJSON, which write the value to bytes and read it again, would cost the square of the
// depth of the nesting (each level would scan the whole of the document under it twice): the first part of this program checks that nested derived
// values cost what plain structs cost, whatever the compiler and the mode (the ratio of the times is checked, not the times). The rest checks what
// the direct route must keep: Decoder.DisallowUnknownFields, UseNumber and SetEscapeHTML reach the derived value as they reach a plain struct, the
// methods called by hand work, and a type that embeds a derived one keeps the methods that go with it (the twin writes these by hand).

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

#Derive(JSON)
type DN struct {
	In  *DN    `json:"in,omitempty"`
	S   string `json:"s,omitempty"`
	Any any    `json:"any,omitempty"`
}

type PN struct {
	In  *PN    `json:"in,omitempty"`
	S   string `json:"s,omitempty"`
	Any any    `json:"any,omitempty"`
}

#Derive(JSON)
type Inner struct {
	A int    `json:"a"`
	B string `json:"b,omitempty"`
}

#Derive(JSON)
type Item struct {
	A int    `json:"a"`
	B string `json:"b,omitempty"`
}

// Outer has the methods of Inner (a quirk of Go that the derive keeps): only Inner is encoded and decoded.
type Outer struct {
	Inner
	C int `json:"c"`
}

// Own embeds Inner and has methods of its own: they are the ones called.
type Own struct {
	Inner
	C int `json:"c"`
}

func (o *Own) UnmarshalJSON(b []byte) error {
	o.C = 99
	return json.Unmarshal(b, &o.Inner)
}

func (o Own) MarshalJSON() ([]byte, error) { return []byte(`{"own":true}`), nil }

// Holder has fields of derived types: by value, by pointer, in a slice, in a map and in an interface.
type Holder struct {
	V Item            `json:"v"`
	P *Item           `json:"p"`
	L []Item          `json:"l"`
	M map[string]Item `json:"m"`
	I any              `json:"i"`
}

// Addressability: a field whose type has a pointer-receiver MarshalJSON is marshaled through the pointer when the struct is reached through a pointer
// or a slice, and by value when it is not addressable (a map value, the content of an interface, the argument itself): as for a plain struct.
type PR struct{ X int }

func (p *PR) MarshalJSON() ([]byte, error) { return []byte(`"pr"`), nil }

#Derive(JSON)
type AD struct {
	A PR `json:"a"`
}

func nested(depth, size int) string {
	return strings.Repeat(`{"in":`, depth) + `{"s":"` + strings.Repeat("x", size) + `"}` + strings.Repeat("}", depth)
}

// within reports whether f took at most 20 times as long as the same work on the plain type, plus a margin for the noise of a busy machine.
func within(plain, derived func()) bool {
	t := time.Now()
	plain()
	tp := time.Since(t)
	t = time.Now()
	derived()
	td := time.Since(t)
	return td <= 20*tp+500*time.Millisecond
}

func main() {
	doc := []byte(nested(500, 512<<10))
	var p PN
	var d DN
	fmt.Println("unmarshal 500 deep, 512 KB at the bottom: linear:", within(
		func() { json.Unmarshal(doc, &p) },
		func() { json.Unmarshal(doc, &d) }))
	var bp, bd []byte
	fmt.Println("marshal the same: linear:", within(
		func() { bp, _ = json.Marshal(&p) },
		func() { bd, _ = json.Marshal(&d) }))
	fmt.Println("same document back:", string(bp) == string(doc), string(bd) == string(doc))

	// A deep list, and a document with many siblings.
	var dl *DN
	var pl *PN
	for i := 0; i < 300; i++ {
		dl = &DN{In: dl, S: "s"}
		pl = &PN{In: pl, S: "s"}
	}
	b1, e1 := json.Marshal(dl)
	b2, e2 := json.Marshal(pl)
	fmt.Println("list of 300:", string(b1) == string(b2), e1, e2, len(b1))
	var l1 []DN
	var l2 []PN
	many := "[" + strings.Repeat(`{"s":"a","any":[1,2,{"k":null}]},`, 2000) + `{}]`
	e1, e2 = json.Unmarshal([]byte(many), &l1), json.Unmarshal([]byte(many), &l2)
	b1, _ = json.Marshal(l1)
	b2, _ = json.Marshal(l2)
	fmt.Println("2001 siblings:", len(l1), len(l2), e1, e2, string(b1) == string(b2))

	// Decoder options and Encoder options reach the derived value.
	var out DN
	dec := json.NewDecoder(strings.NewReader(`{"in":{"s":"a","zzz":1}}`))
	dec.DisallowUnknownFields()
	fmt.Println("unknown field:", dec.Decode(&out))
	out = DN{}
	err := json.Unmarshal([]byte(`{"in":{"s":5},"s":"after"}`), &out)
	fmt.Println("mismatch, then on:", err != nil, out.S, out.In != nil)
	dec = json.NewDecoder(strings.NewReader(`{"any":[12345678901234567890,{"n":1.5}]}`))
	dec.UseNumber()
	out = DN{}
	err = dec.Decode(&out)
	fmt.Printf("use number: %v %T\n", err, out.Any.([]any)[0])
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode(Holder{V: Item{1, "<&>"}, P: &Item{2, "<&>"}, L: []Item{{3, "<&>"}}, M: map[string]Item{"k": {4, "<&>"}}, I: Item{5, "<&>"}})
	enc = json.NewEncoder(&buf)
	enc.Encode(Holder{V: Item{1, "<&>"}, P: &Item{2, "<&>"}, L: []Item{{3, "<&>"}}, M: map[string]Item{"k": {4, "<&>"}}, I: Item{5, "<&>"}})
	fmt.Print(buf.String())

	// Fields of every kind, filled by Unmarshal.
	var h Holder
	err = json.Unmarshal([]byte(`{"v":{"a":1},"p":{"a":2,"b":"p"},"l":[{"a":3},{"a":4,"b":"x"}],"m":{"k":{"a":5}},"i":{"a":6}}`), &h)
	fmt.Printf("holder: %v %+v %+v %+v %+v %v\n", err, h.V, *h.P, h.L, h.M, h.I)
	err = json.Unmarshal([]byte(`{"v":{"a":"no"},"p":[1],"l":[{"a":true}]}`), &h)
	fmt.Println("holder errors:", err != nil, strings.HasPrefix(err.Error(), "json: cannot unmarshal string into Go struct field"))
	err = json.Unmarshal([]byte(`{"v":null,"p":null,"l":null,"m":null,"i":null}`), &h)
	fmt.Printf("holder nulls: %v %+v %v %v %v %v\n", err, h.V, h.P, h.L == nil, h.M == nil, h.I)
	var ip *Item
	err = json.Unmarshal([]byte(`null`), &ip)
	fmt.Println("null into pointer:", err, ip == nil)
	err = json.Unmarshal([]byte(`"str"`), &ip)
	fmt.Println("string into pointer:", err != nil, strings.HasPrefix(err.Error(), "json: cannot unmarshal string into Go value of type"))

	// Addressability.
	try1 := func(label string, v any) {
		b, err := json.Marshal(v)
		fmt.Printf("addressability, %s: %s %v\n", label, b, err)
	}
	try1("value", AD{})
	try1("pointer", &AD{})
	try1("slice", []AD{{}})
	try1("map", map[string]AD{"k": {}})
	try1("interface", []any{AD{}})
	try1("pointer in interface", []any{&AD{}})
	try1("array in pointer", &[1]AD{})

	// The methods called by hand.
	var in Inner
	fmt.Println("UnmarshalJSON:", in.UnmarshalJSON([]byte(`{"a":3,"b":"q"}`)), in)
	fmt.Println("UnmarshalJSON, bad:", in.UnmarshalJSON([]byte(`{"a":`)))
	mb, err := Inner{7, "<m>"}.MarshalJSON()
	fmt.Println("MarshalJSON:", string(mb), err)

	// Types that embed a derived one.
	var o Outer
	fmt.Println("outer:", json.Unmarshal([]byte(`{"a":2,"b":"b","c":7}`), &o), o)
	ob, err := json.Marshal(Outer{Inner{1, "x"}, 5})
	fmt.Println("outer:", string(ob), err)
	var w Own
	fmt.Println("own:", json.Unmarshal([]byte(`{"a":2,"c":7}`), &w), w)
	wb, err := json.Marshal(Own{Inner{1, "x"}, 5})
	fmt.Println("own:", string(wb), err)
	var ho struct{ O Own }
	fmt.Println("own in a struct:", json.Unmarshal([]byte(`{"O":{"a":8}}`), &ho), ho)
}

// Output:
// unmarshal 500 deep, 512 KB at the bottom: linear: true
// marshal the same: linear: true
// same document back: true true
// list of 300: true <nil> <nil> 4494
// 2001 siblings: 2001 2001 <nil> <nil> true
// unknown field: json: unknown field "zzz"
// mismatch, then on: true after true
// use number: <nil> json.Number
// {"v":{"a":1,"b":"<&>"},"p":{"a":2,"b":"<&>"},"l":[{"a":3,"b":"<&>"}],"m":{"k":{"a":4,"b":"<&>"}},"i":{"a":5,"b":"<&>"}}
// {"v":{"a":1,"b":"\u003c\u0026\u003e"},"p":{"a":2,"b":"\u003c\u0026\u003e"},"l":[{"a":3,"b":"\u003c\u0026\u003e"}],"m":{"k":{"a":4,"b":"\u003c\u0026\u003e"}},"i":{"a":5,"b":"\u003c\u0026\u003e"}}
// holder: <nil> {A:1 B:} {A:2 B:p} [{A:3 B:} {A:4 B:x}] map[k:{A:5 B:}] map[a:6]
// holder errors: true true
// holder nulls: <nil> {A:1 B:} <nil> true true <nil>
// null into pointer: <nil> true
// string into pointer: true true
// addressability, value: {"a":{"X":0}} <nil>
// addressability, pointer: {"a":"pr"} <nil>
// addressability, slice: [{"a":"pr"}] <nil>
// addressability, map: {"k":{"a":{"X":0}}} <nil>
// addressability, interface: [{"a":{"X":0}}] <nil>
// addressability, pointer in interface: [{"a":"pr"}] <nil>
// addressability, array in pointer: [{"a":"pr"}] <nil>
// UnmarshalJSON: <nil> {3 q}
// UnmarshalJSON, bad: unexpected end of JSON input
// MarshalJSON: {"a":7,"b":"\u003cm\u003e"} <nil>
// outer: <nil> {{2 b} 0}
// outer: {"a":1,"b":"x"} <nil>
// own: <nil> {{2 } 99}
// own: {"own":true} <nil>
// own in a struct: <nil> {{{8 } 99}}
