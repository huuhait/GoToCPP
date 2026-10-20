// phase: 09
// encoding/json Unmarshal into structs: tags, case-insensitive matching, embedded structs and pointers, unknown fields, ",string", UseNumber, DisallowUnknownFields, duplicate keys.

package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var addr = regexp.MustCompile(`0x[0-9a-f]{8,}`)

func dump(v any) string { return addr.ReplaceAllString(fmt.Sprintf("%+v", v), "0xADDR") }

type Base struct {
	ID   int
	Name string
}

type Emb struct {
	Base
	Extra string
}

type EmbPtr struct {
	*Base
	Extra string
}

type Tagged struct {
	A int    `json:"alpha"`
	B string `json:"beta,omitempty"`
	C int    `json:"-"`
	D int    `json:"-,"`
	E int    `json:",string"`
	F bool   `json:",string"`
	G string `json:",string"`
	H float64 `json:",string"`
}

type Cases struct {
	FooBar int
	Foo_Bar int
	ΣΩ int
	Ǆ int
	K int `json:"k"`
	Kelvin int `json:"K"`
}

type Dup struct {
	A int `json:"x"`
	B int `json:"x"`
	C int
}

type Un struct {
	Exported   int
	unexported int
}

type Wrap struct {
	Inner struct {
		V []struct{ N int }
	}
	Ptr *struct{ Q string }
}

type U1 struct{ V int }
type U2 struct{ V int }
type Amb struct {
	U1
	U2
	W int
}

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
	try("tagged", `{"alpha":1,"beta":"b","C":3,"-":4,"E":"5","F":"true","G":"\"s\"","H":"1.5"}`, Tagged{})
	try("tagged wrong names", `{"A":1,"B":"b","C":3,"D":4}`, Tagged{})
	try("tagged string bad", `{"E":5}`, Tagged{})
	try("tagged string empty", `{"E":""}`, Tagged{})
	try("tagged string null", `{"E":null}`, Tagged{E: 4})
	try("tagged string nested quote", `{"G":"\"x"}`, Tagged{})
	try("case insensitive", `{"foobar":1,"FOO_BAR":2,"σω":3,"ǆ":4,"k":5}`, Cases{})
	try("case upper", `{"FOOBAR":1,"foo_bar":2,"ΣΩ":3,"ǅ":4,"K":5}`, Cases{})
	try("dup tags", `{"x":1,"C":2}`, Dup{})
	try("unexported", `{"Exported":1,"unexported":2}`, Un{})
	try("embedded", `{"ID":1,"Name":"n","Extra":"e"}`, Emb{})
	try("embedded ptr", `{"ID":1,"Name":"n","Extra":"e"}`, EmbPtr{})
	try("embedded ptr only extra", `{"Extra":"e"}`, EmbPtr{})
	try("ambiguous", `{"V":1,"W":2}`, Amb{})
	try("nested", `{"Inner":{"V":[{"N":1},{"N":2}]},"Ptr":{"Q":"q"}}`, Wrap{})
	try("unknown fields ignored", `{"zzz":{"a":[1,2,{"b":null}]},"ID":5}`, Base{})
	try("duplicate keys", `{"ID":1,"ID":2,"Name":"a","name":"b"}`, Base{})
	try("null into non-pointer", `{"ID":null,"Name":null}`, Base{ID: 3, Name: "x"})
	try("wrong type continues", `{"ID":"x","Name":"ok"}`, Base{})
	try("wrong type nested", `{"Inner":{"V":[{"N":"x"},{"N":2}]}}`, Wrap{})
	try("slice of structs", `[{"ID":1},{"ID":2,"Name":"b"}]`, []Base(nil))
	try("map of structs", `{"a":{"ID":1},"b":{"Name":"z"}}`, map[string]Base(nil))
	try("map of pointers", `{"a":{"ID":1},"b":null}`, map[string]*Base(nil))
	try("slice of pointers", `[{"ID":1},null]`, []*Base(nil))
	try("array of structs", `[{"ID":1}]`, [2]Base{})
	try("anonymous struct", `{"A":1,"B":[true]}`, struct {
		A int
		B []bool
	}{})
	try("empty key", `{"":5}`, map[string]int(nil))
	try("unicode keys", `{"\u00e9":1,"é":2,"\ud83d\ude00":3}`, map[string]int(nil))

	// Decoder options.
	dec := json.NewDecoder(strings.NewReader(`{"ID":1,"Nope":2}`))
	dec.DisallowUnknownFields()
	var b Base
	err := dec.Decode(&b)
	fmt.Printf("disallow unknown: %+v %v\n", b, err)

	dec = json.NewDecoder(strings.NewReader(`{"Inner":{"V":[{"N":1,"M":2}]}}`))
	dec.DisallowUnknownFields()
	var w Wrap
	err = dec.Decode(&w)
	fmt.Printf("disallow unknown nested: %+v %v\n", w, err)

	dec = json.NewDecoder(strings.NewReader(`{"a":1,"b":2.5,"c":[3,"x"],"d":12345678901234567890}`))
	dec.UseNumber()
	var m map[string]any
	err = dec.Decode(&m)
	fmt.Printf("use number: %v %v\n", m, err)
	for _, k := range []string{"a", "b", "d"} {
		n := m[k].(json.Number)
		i, ierr := n.Int64()
		f, ferr := n.Float64()
		fmt.Printf("  %s: %q Int64=%d,%v Float64=%v,%v\n", k, n.String(), i, ierr, f, ferr)
	}

	var n json.Number
	err = json.Unmarshal([]byte(`123`), &n)
	fmt.Println(n, err)
	err = json.Unmarshal([]byte(`"456"`), &n)
	fmt.Println(n, err)
	err = json.Unmarshal([]byte(`"abc"`), &n)
	fmt.Println(n, err)
	err = json.Unmarshal([]byte(`1.5e3`), &n)
	fmt.Println(n, err)
	var nn struct{ N json.Number }
	err = json.Unmarshal([]byte(`{"N":-0.0}`), &nn)
	fmt.Println(nn.N, err)
}

// Output:
// tagged: {A:1 B:b C:0 D:4 E:5 F:true G:s H:1.5}
// tagged wrong names: {A:0 B: C:0 D:0 E:0 F:false G: H:0}
// tagged string bad: {A:0 B: C:0 D:0 E:0 F:false G: H:0} error: json: invalid use of ,string struct tag, trying to unmarshal unquoted value into int
// tagged string empty: {A:0 B: C:0 D:0 E:0 F:false G: H:0} error: json: invalid use of ,string struct tag, trying to unmarshal "" into int
// tagged string null: {A:0 B: C:0 D:0 E:4 F:false G: H:0}
// tagged string nested quote: {A:0 B: C:0 D:0 E:0 F:false G: H:0} error: json: invalid use of ,string struct tag, trying to unmarshal "\"x" into string
// case insensitive: {FooBar:1 Foo_Bar:2 ΣΩ:3 Ǆ:4 K:5 Kelvin:0}
// case upper: {FooBar:1 Foo_Bar:2 ΣΩ:3 Ǆ:4 K:0 Kelvin:5}
// dup tags: {A:0 B:0 C:2}
// unexported: {Exported:1 unexported:0}
// embedded: {Base:{ID:1 Name:n} Extra:e}
// embedded ptr: {Base:0xADDR Extra:e}
// embedded ptr only extra: {Base:<nil> Extra:e}
// ambiguous: {U1:{V:0} U2:{V:0} W:2}
// nested: {Inner:{V:[{N:1} {N:2}]} Ptr:0xADDR}
// unknown fields ignored: {ID:5 Name:}
// duplicate keys: {ID:2 Name:b}
// null into non-pointer: {ID:3 Name:x}
// wrong type continues: {ID:0 Name:ok} error: json: cannot unmarshal string into Go struct field Base.ID of type int
// wrong type nested: {Inner:{V:[{N:0} {N:2}]} Ptr:<nil>} error: json: cannot unmarshal string into Go struct field .Inner.V.N of type int
// slice of structs: [{ID:1 Name:} {ID:2 Name:b}]
// map of structs: map[a:{ID:1 Name:} b:{ID:0 Name:z}]
// map of pointers: map[a:0xADDR b:<nil>]
// slice of pointers: [0xADDR <nil>]
// array of structs: [{ID:1 Name:} {ID:0 Name:}]
// anonymous struct: {A:1 B:[true]}
// empty key: map[:5]
// unicode keys: map[é:2 😀:3]
// disallow unknown: {ID:1 Name:} json: unknown field "Nope"
// disallow unknown nested: {Inner:{V:[{N:1}]} Ptr:<nil>} json: unknown field "M"
// use number: map[a:1 b:2.5 c:[3 x] d:12345678901234567890] <nil>
//   a: "1" Int64=1,<nil> Float64=1,<nil>
//   b: "2.5" Int64=0,strconv.ParseInt: parsing "2.5": invalid syntax Float64=2.5,<nil>
//   d: "12345678901234567890" Int64=9223372036854775807,strconv.ParseInt: parsing "12345678901234567890": value out of range Float64=1.2345678901234567e+19,<nil>
// 123 <nil>
// 456 <nil>
// 456 json: invalid number literal, trying to unmarshal "\"abc\"" into Number
// 1.5e3 <nil>
// -0.0 <nil>
