// phase: 09
// encoding/json custom marshalers: value and pointer receivers, addressability, precedence of
// Marshaler over TextMarshaler, promotion through embedding, nil receivers, invalid output,
// errors from the method, and the exact bytes an UnmarshalJSON receives.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

type ValueJSON struct{ N int }

func (v ValueJSON) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"value":%d}`, v.N)), nil
}

type PointerJSON struct{ N int }

func (p *PointerJSON) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"pointer":%d}`, p.N)), nil
}

type ValueText struct{ N int }

func (v ValueText) MarshalText() ([]byte, error) { return []byte(fmt.Sprintf("text-%d", v.N)), nil }

type PointerText struct{ N int }

func (p *PointerText) MarshalText() ([]byte, error) { return []byte(fmt.Sprintf("ptext-%d", p.N)), nil }

type Both struct{ N int }

func (Both) MarshalJSON() ([]byte, error) { return []byte(`"from json"`), nil }

func (Both) MarshalText() ([]byte, error) { return []byte("from text"), nil }

type Invalid struct{}

func (Invalid) MarshalJSON() ([]byte, error) { return []byte(`{"a":`), nil }

type Spaced struct{}

func (Spaced) MarshalJSON() ([]byte, error) { return []byte(" { \"a\" : [ 1 , 2 ] }\n"), nil }

type Empty struct{}

func (Empty) MarshalJSON() ([]byte, error) { return nil, nil }

type HTMLish struct{}

func (HTMLish) MarshalJSON() ([]byte, error) { return []byte(`"<b>&\u2028"`), nil }

// Separators returns raw U+2028 and U+2029 (as bytes, not as escapes), plus HTML characters.
type Separators struct{}

func (Separators) MarshalJSON() ([]byte, error) {
	return []byte("\"a\xe2\x80\xa8b\xe2\x80\xa9c<d>&e\""), nil
}

type Failing struct{}

var errFailing = errors.New("failing marshaler")

func (Failing) MarshalJSON() ([]byte, error) { return nil, errFailing }

type FailingText struct{}

func (FailingText) MarshalText() ([]byte, error) { return nil, errFailing }

type NilSafe struct{ N int }

func (n *NilSafe) MarshalJSON() ([]byte, error) {
	if n == nil {
		return []byte(`"nil receiver"`), nil
	}
	return []byte(`"set"`), nil
}

type Embeds struct {
	ValueJSON
	Extra int
}

type EmbedsPointer struct {
	*PointerJSON
	Extra int
}

type EmbedsBoth struct {
	ValueJSON
	ValueText
}

type Fields struct {
	V  ValueJSON
	P  PointerJSON
	PP *PointerJSON
	T  ValueText
	PT PointerText
	B  Both
	N  *NilSafe
	I  any
}

type Recorder struct{ Seen []string }

func (r *Recorder) UnmarshalJSON(data []byte) error {
	r.Seen = append(r.Seen, string(data))
	return nil
}

type TextTarget struct{ Seen []string }

func (t *TextTarget) UnmarshalText(text []byte) error {
	t.Seen = append(t.Seen, string(text))
	return nil
}

type FailingDecode struct{}

func (*FailingDecode) UnmarshalJSON([]byte) error { return errFailing }

func marshal(label string, value any) {
	encoded, err := json.Marshal(value)
	var marshalerErr *json.MarshalerError
	detail := ""
	if errors.As(err, &marshalerErr) {
		detail = fmt.Sprintf(" [MarshalerError type=%v unwrap=%v]", marshalerErr.Type, errors.Unwrap(err))
	}
	fmt.Printf("%-22s %s err=%v%s\n", label, encoded, err, detail)
}

func main() {
	fmt.Println("== receivers")
	marshal("value", ValueJSON{1})
	marshal("value pointer", &ValueJSON{1})
	marshal("pointer method, value", PointerJSON{2})
	marshal("pointer method, pointer", &PointerJSON{2})
	pointerSlice := []PointerJSON{{1}, {2}}
	marshal("pointer method, slice", pointerSlice)
	marshal("pointer method, array", [2]PointerJSON{{1}, {2}})
	marshal("pointer method, map", map[string]PointerJSON{"a": {1}})
	marshal("pointer method, ptr map", map[string]*PointerJSON{"a": {1}})
	marshal("text value", ValueText{3})
	marshal("text pointer method, value", PointerText{4})
	marshal("text pointer method, ptr", &PointerText{4})
	marshal("text pointer method, slice", []PointerText{{4}})
	marshal("both", Both{})
	marshal("both as key", map[string]Both{"k": {}})
	marshal("fields by value", Fields{I: PointerJSON{9}})
	marshal("fields by pointer", &Fields{I: PointerJSON{9}})
	marshal("fields nil ptr", &Fields{PP: nil, N: nil})
	marshal("fields with ptrs", &Fields{PP: &PointerJSON{7}, N: &NilSafe{}, I: &PointerJSON{8}})

	fmt.Println("== nil receivers")
	var nilPointer *NilSafe
	marshal("nil ptr marshaler", nilPointer)
	marshal("nil in slice", []*NilSafe{nil, {}})
	marshal("nil in map", map[string]*NilSafe{"a": nil})
	var nilValueJSON *ValueJSON
	marshal("nil ptr, value method", nilValueJSON)
	marshal("nil in interface", []any{nilValueJSON})
	var nilInterface fmt.Stringer
	marshal("nil interface", nilInterface)

	fmt.Println("== embedding")
	marshal("embeds", Embeds{ValueJSON{1}, 2})
	marshal("embeds ptr", &Embeds{ValueJSON{1}, 2})
	marshal("embeds pointer, set", EmbedsPointer{&PointerJSON{1}, 2})
	marshal("embeds both (ambiguous)", EmbedsBoth{ValueJSON{1}, ValueText{2}})

	fmt.Println("== output handling")
	marshal("invalid", Invalid{})
	marshal("spaced", Spaced{})
	marshal("empty", Empty{})
	marshal("html", HTMLish{})
	marshal("html in slice", []HTMLish{{}})
	marshal("separators", Separators{})
	marshal("separators in map", map[string]Separators{"k": {}})
	for _, escape := range []bool{true, false} {
		var separators bytes.Buffer
		separatorEncoder := json.NewEncoder(&separators)
		separatorEncoder.SetEscapeHTML(escape)
		separatorErr := separatorEncoder.Encode([]any{Separators{}, "x\u2028<"})
		fmt.Printf("separators escapeHTML=%v: %q %v\n", escape, separators.String(), separatorErr)
	}
	marshal("failing", Failing{})
	marshal("failing text", FailingText{})
	marshal("failing in struct", struct{ A Failing }{})
	marshal("failing in slice", []Failing{{}})
	marshal("failing text key", map[FailingTextKey]int{{}: 1})
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode([]any{HTMLish{}, "<>"})
	fmt.Printf("noescape html: %q %v\n", buffer.String(), err)
	buffer.Reset()
	encoder = json.NewEncoder(&buffer)
	encoder.SetIndent(">", "  ")
	err = encoder.Encode(map[string]any{"s": Spaced{}, "v": ValueJSON{3}})
	fmt.Printf("indent: %q %v\n", buffer.String(), err)
	indented, err := json.MarshalIndent([]any{Spaced{}, Both{}}, "", "\t")
	fmt.Printf("%s %v\n", indented, err)

	fmt.Println("== what UnmarshalJSON receives")
	for _, input := range []string{`null`, ` 1 `, `"s"`, `[ 1 , 2 ]`, `{"a" : {"b":null}}`, `true`, "\n\"x\"\n"} {
		var recorder Recorder
		err := json.Unmarshal([]byte(input), &recorder)
		fmt.Printf("%-24q => %q %v\n", input, recorder.Seen, err)
	}
	var recorderPointer *Recorder
	err = json.Unmarshal([]byte(`null`), &recorderPointer)
	fmt.Println(recorderPointer == nil, err)
	err = json.Unmarshal([]byte(`1`), &recorderPointer)
	fmt.Println(recorderPointer.Seen, err)
	var inStruct struct {
		A Recorder
		B *Recorder
		C []Recorder
		D map[string]Recorder
		E map[string]*Recorder
		F [2]Recorder
	}
	err = json.Unmarshal([]byte(`{"A":1,"B":null,"C":[2,null],"D":{"k":3},"E":{"k":null,"j":4},"F":[5]}`), &inStruct)
	fmt.Printf("A=%q B=%v C=%q D=%q E=%d F=%q %v\n", inStruct.A.Seen, inStruct.B, inStruct.C, inStruct.D, len(inStruct.E), inStruct.F, err)

	fmt.Println("== UnmarshalText")
	for _, input := range []string{`"abc"`, `"a\u00e9\n"`, `null`, `1`, `[]`, `{}`, `"\ud800"`, `true`} {
		var target TextTarget
		err := json.Unmarshal([]byte(input), &target)
		fmt.Printf("%-14s => %q %v\n", input, target.Seen, err)
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			fmt.Printf("   UnmarshalTypeError Value=%q Type=%v Offset=%d\n", typeErr.Value, typeErr.Type, typeErr.Offset)
		}
	}
	var textKeys map[string]TextTarget
	err = json.Unmarshal([]byte(`{"a":"x"}`), &textKeys)
	fmt.Println(textKeys, err)

	fmt.Println("== failing UnmarshalJSON")
	var failing FailingDecode
	err = json.Unmarshal([]byte(`1`), &failing)
	fmt.Println(err, err == errFailing)
	var failingField struct{ A, B FailingDecode }
	err = json.Unmarshal([]byte(`{"A":1,"B":2}`), &failingField)
	fmt.Println(err, err == errFailing)
}

type FailingTextKey struct{}

func (FailingTextKey) MarshalText() ([]byte, error) { return nil, errFailing }

// Output:
// == receivers
// value                  {"value":1} err=<nil>
// value pointer          {"value":1} err=<nil>
// pointer method, value  {"N":2} err=<nil>
// pointer method, pointer {"pointer":2} err=<nil>
// pointer method, slice  [{"pointer":1},{"pointer":2}] err=<nil>
// pointer method, array  [{"N":1},{"N":2}] err=<nil>
// pointer method, map    {"a":{"N":1}} err=<nil>
// pointer method, ptr map {"a":{"pointer":1}} err=<nil>
// text value             "text-3" err=<nil>
// text pointer method, value {"N":4} err=<nil>
// text pointer method, ptr "ptext-4" err=<nil>
// text pointer method, slice ["ptext-4"] err=<nil>
// both                   "from json" err=<nil>
// both as key            {"k":"from json"} err=<nil>
// fields by value        {"V":{"value":0},"P":{"N":0},"PP":null,"T":"text-0","PT":{"N":0},"B":"from json","N":null,"I":{"N":9}} err=<nil>
// fields by pointer      {"V":{"value":0},"P":{"pointer":0},"PP":null,"T":"text-0","PT":"ptext-0","B":"from json","N":null,"I":{"N":9}} err=<nil>
// fields nil ptr         {"V":{"value":0},"P":{"pointer":0},"PP":null,"T":"text-0","PT":"ptext-0","B":"from json","N":null,"I":null} err=<nil>
// fields with ptrs       {"V":{"value":0},"P":{"pointer":0},"PP":{"pointer":7},"T":"text-0","PT":"ptext-0","B":"from json","N":"set","I":{"pointer":8}} err=<nil>
// == nil receivers
// nil ptr marshaler      null err=<nil>
// nil in slice           [null,"set"] err=<nil>
// nil in map             {"a":null} err=<nil>
// nil ptr, value method  null err=<nil>
// nil in interface       [null] err=<nil>
// nil interface          null err=<nil>
// == embedding
// embeds                 {"value":1} err=<nil>
// embeds ptr             {"value":1} err=<nil>
// embeds pointer, set    {"pointer":1} err=<nil>
// embeds both (ambiguous) {"value":1} err=<nil>
// == output handling
// invalid                 err=json: error calling MarshalJSON for type main.Invalid: unexpected end of JSON input [MarshalerError type=main.Invalid unwrap=unexpected end of JSON input]
// spaced                 {"a":[1,2]} err=<nil>
// empty                   err=json: error calling MarshalJSON for type main.Empty: unexpected end of JSON input [MarshalerError type=main.Empty unwrap=unexpected end of JSON input]
// html                   "\u003cb\u003e\u0026\u2028" err=<nil>
// html in slice          ["\u003cb\u003e\u0026\u2028"] err=<nil>
// separators             "a\u2028b\u2029c\u003cd\u003e\u0026e" err=<nil>
// separators in map      {"k":"a\u2028b\u2029c\u003cd\u003e\u0026e"} err=<nil>
// separators escapeHTML=true: "[\"a\\u2028b\\u2029c\\u003cd\\u003e\\u0026e\",\"x\\u2028\\u003c\"]\n" <nil>
// separators escapeHTML=false: "[\"a\u2028b\u2029c<d>&e\",\"x\\u2028<\"]\n" <nil>
// failing                 err=json: error calling MarshalJSON for type main.Failing: failing marshaler [MarshalerError type=main.Failing unwrap=failing marshaler]
// failing text            err=json: error calling MarshalText for type main.FailingText: failing marshaler [MarshalerError type=main.FailingText unwrap=failing marshaler]
// failing in struct       err=json: error calling MarshalJSON for type main.Failing: failing marshaler [MarshalerError type=main.Failing unwrap=failing marshaler]
// failing in slice        err=json: error calling MarshalJSON for type main.Failing: failing marshaler [MarshalerError type=main.Failing unwrap=failing marshaler]
// failing text key        err=json: encoding error for type "map[main.FailingTextKey]int": "failing marshaler"
// noescape html: "[\"<b>&\\u2028\",\"<>\"]\n" <nil>
// indent: "{\n>  \"s\": {\n>    \"a\": [\n>      1,\n>      2\n>    ]\n>  },\n>  \"v\": {\n>    \"value\": 3\n>  }\n>}\n" <nil>
// [
// 	{
// 		"a": [
// 			1,
// 			2
// 		]
// 	},
// 	"from json"
// ] <nil>
// == what UnmarshalJSON receives
// "null"                   => ["null"] <nil>
// " 1 "                    => ["1"] <nil>
// "\"s\""                  => ["\"s\""] <nil>
// "[ 1 , 2 ]"              => ["[ 1 , 2 ]"] <nil>
// "{\"a\" : {\"b\":null}}" => ["{\"a\" : {\"b\":null}}"] <nil>
// "true"                   => ["true"] <nil>
// "\n\"x\"\n"              => ["\"x\""] <nil>
// true <nil>
// [1] <nil>
// A=["1"] B=<nil> C=[{["2"]} {["null"]}] D=map["k":{["3"]}] E=2 F=[{["5"]} {[]}] <nil>
// == UnmarshalText
// "abc"          => ["abc"] <nil>
// "a\u00e9\n"    => ["aé\n"] <nil>
// null           => [] <nil>
// 1              => [] json: cannot unmarshal number into Go value of type *main.TextTarget
//    UnmarshalTypeError Value="number" Type=*main.TextTarget Offset=1
// []             => [] json: cannot unmarshal array into Go value of type *main.TextTarget
//    UnmarshalTypeError Value="array" Type=*main.TextTarget Offset=1
// {}             => [] json: cannot unmarshal object into Go value of type *main.TextTarget
//    UnmarshalTypeError Value="object" Type=*main.TextTarget Offset=1
// "\ud800"       => ["�"] <nil>
// true           => [] json: cannot unmarshal bool into Go value of type *main.TextTarget
//    UnmarshalTypeError Value="bool" Type=*main.TextTarget Offset=4
// map[a:{[x]}] <nil>
// == failing UnmarshalJSON
// failing marshaler true
// failing marshaler true
