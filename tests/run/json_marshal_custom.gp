// phase: 09
// encoding/json Marshaler and TextMarshaler: value and pointer receivers, nil pointers, map keys, errors from marshalers, invalid output, compaction, time and big values.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Celsius float64

func (c Celsius) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"celsius": %.1f}`, float64(c))), nil
}

type PtrM struct{ V int }

func (p *PtrM) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"ptr%d"`, p.V)), nil
}

type Text struct{ S string }

func (t Text) MarshalText() ([]byte, error) { return []byte("text:" + t.S), nil }

type TextPtr struct{ S string }

func (t *TextPtr) MarshalText() ([]byte, error) { return []byte("tp:" + t.S), nil }

type Both struct{}

func (Both) MarshalJSON() ([]byte, error) { return []byte(`"json"`), nil }
func (Both) MarshalText() ([]byte, error) { return []byte("text"), nil }

type Bad struct{}

func (Bad) MarshalJSON() ([]byte, error) { return []byte(`{bad`), nil }

type Fail struct{}

func (Fail) MarshalJSON() ([]byte, error) { return nil, errors.New("boom") }

type FailText struct{}

func (FailText) MarshalText() ([]byte, error) { return nil, errors.New("text boom") }

type Empty struct{}

func (Empty) MarshalJSON() ([]byte, error) { return nil, nil }

type Spaces struct{}

func (Spaces) MarshalJSON() ([]byte, error) { return []byte(" [ 1 , 2 , { \"a\" : \"<\" } ]\n"), nil }

type Key struct{ A, B int }

func (k Key) MarshalText() ([]byte, error) { return []byte(fmt.Sprintf("%d-%d", k.A, k.B)), nil }

type Wrapper struct {
	C  Celsius
	P  *PtrM
	PV PtrM
	T  Text
	TP *TextPtr
	O  Both
}

type Panicker struct{}

func (Panicker) MarshalJSON() ([]byte, error) { panic("marshal panic") }

type NilRecv struct{ x int }

func (n *NilRecv) MarshalJSON() ([]byte, error) {
	if n == nil {
		return []byte(`"nil receiver"`), nil
	}
	return []byte(`"set"`), nil
}

type Iface interface{ MarshalJSON() ([]byte, error) }

func show(label string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		var me *json.MarshalerError
		if errors.As(err, &me) {
			fmt.Printf("  MarshalerError type=%v source=%q unwrap=%v\n", me.Type, me.Err, errors.Unwrap(err))
		}
		return
	}
	fmt.Printf("%s: %s\n", label, b)
}

func main() {
	show("celsius", Celsius(21.55))
	show("celsius slice", []Celsius{1, 2})
	show("celsius map", map[string]Celsius{"x": 3})
	show("ptrm value (not addressable)", PtrM{1})
	show("ptrm pointer", &PtrM{2})
	show("ptrm in slice (addressable)", []PtrM{{3}, {4}})
	show("ptrm in array value", [1]PtrM{{5}})
	pa := [1]PtrM{{6}}
	show("ptrm in array ptr", &pa)
	show("ptrm nil pointer", (*PtrM)(nil))
	show("text", Text{"a"})
	show("text ptr value", TextPtr{"b"})
	show("text ptr", &TextPtr{"c"})
	show("both prefers json", Both{})
	show("bad output", Bad{})
	show("fail", Fail{})
	show("fail text", FailText{})
	show("empty output", Empty{})
	show("spaces compacted", Spaces{})
	show("wrapper", Wrapper{C: 1, P: &PtrM{7}, PV: PtrM{8}, T: Text{"t"}, TP: &TextPtr{"u"}})
	show("wrapper zero", Wrapper{})
	show("wrapper pointer", &Wrapper{C: 2, PV: PtrM{9}})
	show("text map key", map[Key]int{{2, 1}: 1, {1, 2}: 2})
	show("text key fails", map[FailText]int{{}: 1})
	show("nil receiver pointer", (*NilRecv)(nil))
	show("nil receiver in struct", struct{ N *NilRecv }{})
	show("set receiver", &NilRecv{})
	var i Iface
	show("nil interface", i)
	i = Celsius(5)
	show("interface holding marshaler", i)
	show("struct with iface", struct{ I Iface }{Celsius(6)})
	show("time", time.Date(2024, 2, 29, 13, 14, 15, 123456789, time.UTC))
	show("time offset", time.Date(2024, 2, 29, 13, 14, 15, 0, time.FixedZone("x", 5400)))
	show("duration", time.Second*90)
	show("error value", errors.New("x"))
	show("raw message", json.RawMessage(`{"a": [1, 2]}`))
	show("raw message ptr", &json.RawMessage{})
	show("raw message nil", json.RawMessage(nil))
	show("raw in struct", struct{ R json.RawMessage }{json.RawMessage(` "x" `)})
	show("raw bad", json.RawMessage(`{`))
	show("number", json.Number("12.5"))
	show("number bad", json.Number("abc"))
	show("number empty", json.Number(""))
	show("number in map", map[string]json.Number{"n": "1e3"})
}

// Output:
// celsius: {"celsius":21.6}
// celsius slice: [{"celsius":1.0},{"celsius":2.0}]
// celsius map: {"x":{"celsius":3.0}}
// ptrm value (not addressable): {"V":1}
// ptrm pointer: "ptr2"
// ptrm in slice (addressable): ["ptr3","ptr4"]
// ptrm in array value: [{"V":5}]
// ptrm in array ptr: ["ptr6"]
// ptrm nil pointer: null
// text: "text:a"
// text ptr value: {"S":"b"}
// text ptr: "tp:c"
// both prefers json: "json"
// bad output: error: json: error calling MarshalJSON for type main.Bad: invalid character 'b' looking for beginning of object key string
//   MarshalerError type=main.Bad source="invalid character 'b' looking for beginning of object key string" unwrap=invalid character 'b' looking for beginning of object key string
// fail: error: json: error calling MarshalJSON for type main.Fail: boom
//   MarshalerError type=main.Fail source="boom" unwrap=boom
// fail text: error: json: error calling MarshalText for type main.FailText: text boom
//   MarshalerError type=main.FailText source="text boom" unwrap=text boom
// empty output: error: json: error calling MarshalJSON for type main.Empty: unexpected end of JSON input
//   MarshalerError type=main.Empty source="unexpected end of JSON input" unwrap=unexpected end of JSON input
// spaces compacted: [1,2,{"a":"\u003c"}]
// wrapper: {"C":{"celsius":1.0},"P":"ptr7","PV":{"V":8},"T":"text:t","TP":"tp:u","O":"json"}
// wrapper zero: {"C":{"celsius":0.0},"P":null,"PV":{"V":0},"T":"text:","TP":null,"O":"json"}
// wrapper pointer: {"C":{"celsius":2.0},"P":null,"PV":"ptr9","T":"text:","TP":null,"O":"json"}
// text map key: {"1-2":2,"2-1":1}
// text key fails: error: json: encoding error for type "map[main.FailText]int": "text boom"
// nil receiver pointer: null
// nil receiver in struct: {"N":null}
// set receiver: "set"
// nil interface: null
// interface holding marshaler: {"celsius":5.0}
// struct with iface: {"I":{"celsius":6.0}}
// time: "2024-02-29T13:14:15.123456789Z"
// time offset: "2024-02-29T13:14:15+01:30"
// duration: 90000000000
// error value: {}
// raw message: {"a":[1,2]}
// raw message ptr: error: json: error calling MarshalJSON for type *json.RawMessage: unexpected end of JSON input
//   MarshalerError type=*json.RawMessage source="unexpected end of JSON input" unwrap=unexpected end of JSON input
// raw message nil: null
// raw in struct: {"R":"x"}
// raw bad: error: json: error calling MarshalJSON for type json.RawMessage: unexpected end of JSON input
//   MarshalerError type=json.RawMessage source="unexpected end of JSON input" unwrap=unexpected end of JSON input
// number: 12.5
// number bad: error: json: invalid number literal "abc"
// number empty: 0
// number in map: {"n":1e3}
