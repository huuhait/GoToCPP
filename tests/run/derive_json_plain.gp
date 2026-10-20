// phase: 09

// #Derive(JSON) (step 9.11, ADR-102): MarshalJSON and UnmarshalJSON that call encoding/json on a copy of the struct type that has no methods,
// so the output and the input are exactly encoding/json's. Every derived type here has a twin with the same fields and no #Derive; the
// program prints both and whether they agree. The twin of this file (testdata/derive_twin/derive_json_plain.go) writes the generated methods by hand.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

#Derive(JSON)
type Cfg struct {
	Name    string            `json:"name"`
	Port    int               `json:"port,omitempty"`
	Debug   bool              `json:"debug"`
	Tags    []string          `json:"tags,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Ratio   float64           `json:"ratio"`
	Skip    string            `json:"-"`
	Quoted  int64             `json:"quoted,string"`
	When    time.Time         `json:"when"`
	Raw     json.RawMessage   `json:"raw,omitempty"`
	Any     any               `json:"any"`
	Ptr     *int              `json:"ptr"`
	private int
}

type CfgPlain struct {
	Name    string            `json:"name"`
	Port    int               `json:"port,omitempty"`
	Debug   bool              `json:"debug"`
	Tags    []string          `json:"tags,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Ratio   float64           `json:"ratio"`
	Skip    string            `json:"-"`
	Quoted  int64             `json:"quoted,string"`
	When    time.Time         `json:"when"`
	Raw     json.RawMessage   `json:"raw,omitempty"`
	Any     any               `json:"any"`
	Ptr     *int              `json:"ptr"`
	private int
}

type Base struct {
	ID   int    `json:"id"`
	Name string `json:"base_name"`
}

type Extra struct {
	Note string `json:"note,omitempty"`
	ID   int    `json:"extra_id"`
}

#Derive(JSON)
type Node struct {
	Base
	*Extra
	Label string  `json:"label"`
	Kids  []Node  `json:"kids,omitempty"`
	Up    *Node   `json:"up,omitempty"`
	Pair  Pair[string, int] `json:"pair"`
}

type NodePlain struct {
	Base
	*Extra
	Label string        `json:"label"`
	Kids  []NodePlain   `json:"kids,omitempty"`
	Up    *NodePlain    `json:"up,omitempty"`
	Pair  PairPlain[string, int] `json:"pair"`
}

#Derive(JSON)
type Pair[K comparable, V any] struct {
	Key K `json:"k"`
	Val V `json:"v"`
}

type PairPlain[K comparable, V any] struct {
	Key K `json:"k"`
	Val V `json:"v"`
}

#Derive(JSON)
type Html struct {
	S string `json:"s"`
}

#Derive(JSON)
type Empty struct{}

func try(label string, got, want []byte, gerr, werr error) {
	fmt.Printf("%s: %s", label, got)
	if gerr != nil {
		fmt.Printf(" err=%v", gerr)
	}
	same := string(got) == string(want) && fmt.Sprint(gerr) == fmt.Sprint(werr)
	fmt.Printf(" | same as plain: %v\n", same)
}

func main() {
	n := 7
	now := time.Date(2024, 2, 29, 13, 14, 15, 500, time.UTC)
	cfgs := []Cfg{
		{},
		{Name: "a<b>&c", Port: 8080, Debug: true, Tags: []string{"x", "y"}, Env: map[string]string{"B": "2", "A": "1"}, Ratio: 0.1, Skip: "s", Quoted: 5, When: now, Raw: json.RawMessage(`{ "k" : [1, 2] }`), Any: map[string]any{"z": 1, "a": []any{true, nil}}, Ptr: &n},
		{Ratio: 1e21, Quoted: -3, Any: 1.5},
		{Name: "\u2028\xff\x00", Ratio: 123456789.125},
	}
	for i, c := range cfgs {
		p := CfgPlain(c)
		got, gerr := json.Marshal(c)
		want, werr := json.Marshal(p)
		try(fmt.Sprint("marshal cfg ", i), got, want, gerr, werr)
		got, gerr = json.Marshal(&c)
		try(fmt.Sprint("marshal &cfg ", i), got, want, gerr, werr)
		got, gerr = c.MarshalJSON()
		try(fmt.Sprint("method cfg ", i), got, want, gerr, werr)
		got, gerr = json.MarshalIndent(c, ">", "  ")
		want, werr = json.MarshalIndent(p, ">", "  ")
		try(fmt.Sprint("indent cfg ", i), got, want, gerr, werr)
	}

	// Slices, maps and pointers of a derived type.
	got, gerr := json.Marshal([]Cfg{cfgs[0], cfgs[2]})
	want, werr := json.Marshal([]CfgPlain{CfgPlain(cfgs[0]), CfgPlain(cfgs[2])})
	try("slice", got, want, gerr, werr)
	got, gerr = json.Marshal(map[string]*Cfg{"b": &cfgs[2], "a": nil})
	want, werr = json.Marshal(map[string]*CfgPlain{"b": (*CfgPlain)(&cfgs[2]), "a": nil})
	try("map of pointers", got, want, gerr, werr)

	// Embedded structs, an embedded pointer, nesting and a generic type.
	tree := Node{Base: Base{1, "root"}, Extra: &Extra{"hello", 9}, Label: "r", Pair: Pair[string, int]{"k", 1},
		Kids: []Node{{Base: Base{2, "kid"}, Label: "c", Pair: Pair[string, int]{"kk", 2}}}}
	treeP := NodePlain{Base: Base{1, "root"}, Extra: &Extra{"hello", 9}, Label: "r", Pair: PairPlain[string, int]{"k", 1},
		Kids: []NodePlain{{Base: Base{2, "kid"}, Label: "c", Pair: PairPlain[string, int]{"kk", 2}}}}
	got, gerr = json.Marshal(tree)
	want, werr = json.Marshal(treeP)
	try("tree", got, want, gerr, werr)
	var back Node
	var backP NodePlain
	err1 := json.Unmarshal(got, &back)
	err2 := json.Unmarshal(got, &backP)
	b1, _ := json.Marshal(back)
	b2, _ := json.Marshal(backP)
	fmt.Println("tree round trip:", err1, err2, string(b1) == string(got), string(b2) == string(got), back.Base.ID, back.Extra.Note, len(back.Kids), back.Kids[0].Name, back.Pair)

	pr := Pair[string, []int]{"p", []int{1, 2}}
	got, gerr = json.Marshal(pr)
	want, werr = json.Marshal(PairPlain[string, []int]{"p", []int{1, 2}})
	try("generic", got, want, gerr, werr)
	var pb Pair[string, []int]
	fmt.Println("generic unmarshal:", json.Unmarshal([]byte(`{"k":"z","v":[3,4]}`), &pb), pb)

	// Unmarshal: the same values and the same kinds of errors as the plain type.
	inputs := []string{
		`{}`, `null`, `{"name":"n","port":1,"debug":true,"tags":["a"],"env":{"k":"v"},"ratio":2.5,"quoted":"12","when":"2020-01-02T03:04:05Z","raw":{"x":1},"any":[1,"a",null],"ptr":4}`,
		`{"NAME":"case","PORT":2,"Debug":true}`, `{"unknown":1,"name":"after"}`,
		`{"name":5}`, `{"port":"x"}`, `{"quoted":5}`, `{"quoted":"x"}`, `{"when":"yesterday"}`, `[1]`, `{"name":"x"`, `{"name" 1}`, ``, `tru`,
		`{"tags":null,"env":null,"ptr":null,"any":null}`, `{"raw":null}`,
	}
	for _, in := range inputs {
		var c Cfg
		var p CfgPlain
		e1 := json.Unmarshal([]byte(in), &c)
		e2 := json.Unmarshal([]byte(in), &p)
		b1, _ := json.Marshal(c)
		b2, _ := json.Marshal(p)
		kind := func(e error) string {
			var se *json.SyntaxError
			var te *json.UnmarshalTypeError
			switch {
			case e == nil:
				return "ok"
			case errors.As(e, &se):
				return "syntax"
			case errors.As(e, &te):
				return "type " + te.Value
			}
			return fmt.Sprintf("%T", e)
		}
		fmt.Printf("unmarshal %-32q -> %s | plain %s | same values: %v | msg: %v\n", in, kind(e1), kind(e2), string(b1) == string(b2), e1)
	}

	// The escaping of HTML is encoding/json's default.
	got, gerr = json.Marshal(Html{"<a href=\"x\">&</a>"})
	want, werr = json.Marshal(struct {
		S string `json:"s"`
	}{"<a href=\"x\">&</a>"})
	try("html", got, want, gerr, werr)
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.Encode(Html{"a&b"})
	enc.Encode([]Html{{"c"}})
	fmt.Print(sb.String())
	got, _ = json.Marshal(Empty{})
	fmt.Println(string(got))

	// The methods can be called directly.
	var h Html
	fmt.Println(h.UnmarshalJSON([]byte(`{"s":"direct"}`)), h.S)
	d, err := h.MarshalJSON()
	fmt.Println(string(d), err)
	var hp *Html = &Html{"ptr"}
	d, err = hp.MarshalJSON()
	fmt.Println(string(d), err)

	// Decoder: a stream of derived values.
	dec := json.NewDecoder(strings.NewReader(`{"s":"1"} {"s":"2"} [{"s":"3"}]`))
	var h1, h2 Html
	var hs []Html
	fmt.Println(dec.Decode(&h1), dec.Decode(&h2), dec.Decode(&hs), h1, h2, hs, dec.More())
}

// Output:
// marshal cfg 0: {"name":"","debug":false,"ratio":0,"quoted":"0","when":"0001-01-01T00:00:00Z","any":null,"ptr":null} | same as plain: true
// marshal &cfg 0: {"name":"","debug":false,"ratio":0,"quoted":"0","when":"0001-01-01T00:00:00Z","any":null,"ptr":null} | same as plain: true
// method cfg 0: {"name":"","debug":false,"ratio":0,"quoted":"0","when":"0001-01-01T00:00:00Z","any":null,"ptr":null} | same as plain: true
// indent cfg 0: {
// >  "name": "",
// >  "debug": false,
// >  "ratio": 0,
// >  "quoted": "0",
// >  "when": "0001-01-01T00:00:00Z",
// >  "any": null,
// >  "ptr": null
// >} | same as plain: true
// marshal cfg 1: {"name":"a\u003cb\u003e\u0026c","port":8080,"debug":true,"tags":["x","y"],"env":{"A":"1","B":"2"},"ratio":0.1,"quoted":"5","when":"2024-02-29T13:14:15.0000005Z","raw":{"k":[1,2]},"any":{"a":[true,null],"z":1},"ptr":7} | same as plain: true
// marshal &cfg 1: {"name":"a\u003cb\u003e\u0026c","port":8080,"debug":true,"tags":["x","y"],"env":{"A":"1","B":"2"},"ratio":0.1,"quoted":"5","when":"2024-02-29T13:14:15.0000005Z","raw":{"k":[1,2]},"any":{"a":[true,null],"z":1},"ptr":7} | same as plain: true
// method cfg 1: {"name":"a\u003cb\u003e\u0026c","port":8080,"debug":true,"tags":["x","y"],"env":{"A":"1","B":"2"},"ratio":0.1,"quoted":"5","when":"2024-02-29T13:14:15.0000005Z","raw":{"k":[1,2]},"any":{"a":[true,null],"z":1},"ptr":7} | same as plain: true
// indent cfg 1: {
// >  "name": "a\u003cb\u003e\u0026c",
// >  "port": 8080,
// >  "debug": true,
// >  "tags": [
// >    "x",
// >    "y"
// >  ],
// >  "env": {
// >    "A": "1",
// >    "B": "2"
// >  },
// >  "ratio": 0.1,
// >  "quoted": "5",
// >  "when": "2024-02-29T13:14:15.0000005Z",
// >  "raw": {
// >    "k": [
// >      1,
// >      2
// >    ]
// >  },
// >  "any": {
// >    "a": [
// >      true,
// >      null
// >    ],
// >    "z": 1
// >  },
// >  "ptr": 7
// >} | same as plain: true
// marshal cfg 2: {"name":"","debug":false,"ratio":1e+21,"quoted":"-3","when":"0001-01-01T00:00:00Z","any":1.5,"ptr":null} | same as plain: true
// marshal &cfg 2: {"name":"","debug":false,"ratio":1e+21,"quoted":"-3","when":"0001-01-01T00:00:00Z","any":1.5,"ptr":null} | same as plain: true
// method cfg 2: {"name":"","debug":false,"ratio":1e+21,"quoted":"-3","when":"0001-01-01T00:00:00Z","any":1.5,"ptr":null} | same as plain: true
// indent cfg 2: {
// >  "name": "",
// >  "debug": false,
// >  "ratio": 1e+21,
// >  "quoted": "-3",
// >  "when": "0001-01-01T00:00:00Z",
// >  "any": 1.5,
// >  "ptr": null
// >} | same as plain: true
// marshal cfg 3: {"name":"\u2028\ufffd\u0000","debug":false,"ratio":123456789.125,"quoted":"0","when":"0001-01-01T00:00:00Z","any":null,"ptr":null} | same as plain: true
// marshal &cfg 3: {"name":"\u2028\ufffd\u0000","debug":false,"ratio":123456789.125,"quoted":"0","when":"0001-01-01T00:00:00Z","any":null,"ptr":null} | same as plain: true
// method cfg 3: {"name":"\u2028\ufffd\u0000","debug":false,"ratio":123456789.125,"quoted":"0","when":"0001-01-01T00:00:00Z","any":null,"ptr":null} | same as plain: true
// indent cfg 3: {
// >  "name": "\u2028\ufffd\u0000",
// >  "debug": false,
// >  "ratio": 123456789.125,
// >  "quoted": "0",
// >  "when": "0001-01-01T00:00:00Z",
// >  "any": null,
// >  "ptr": null
// >} | same as plain: true
// slice: [{"name":"","debug":false,"ratio":0,"quoted":"0","when":"0001-01-01T00:00:00Z","any":null,"ptr":null},{"name":"","debug":false,"ratio":1e+21,"quoted":"-3","when":"0001-01-01T00:00:00Z","any":1.5,"ptr":null}] | same as plain: true
// map of pointers: {"a":null,"b":{"name":"","debug":false,"ratio":1e+21,"quoted":"-3","when":"0001-01-01T00:00:00Z","any":1.5,"ptr":null}} | same as plain: true
// tree: {"id":1,"base_name":"root","note":"hello","extra_id":9,"label":"r","kids":[{"id":2,"base_name":"kid","label":"c","pair":{"k":"kk","v":2}}],"pair":{"k":"k","v":1}} | same as plain: true
// tree round trip: <nil> <nil> true true 1 hello 1 kid {k 1}
// generic: {"k":"p","v":[1,2]} | same as plain: true
// generic unmarshal: <nil> {z [3 4]}
// unmarshal "{}"                             -> ok | plain ok | same values: true | msg: <nil>
// unmarshal "null"                           -> ok | plain ok | same values: true | msg: <nil>
// unmarshal "{\"name\":\"n\",\"port\":1,\"debug\":true,\"tags\":[\"a\"],\"env\":{\"k\":\"v\"},\"ratio\":2.5,\"quoted\":\"12\",\"when\":\"2020-01-02T03:04:05Z\",\"raw\":{\"x\":1},\"any\":[1,\"a\",null],\"ptr\":4}" -> ok | plain ok | same values: true | msg: <nil>
// unmarshal "{\"NAME\":\"case\",\"PORT\":2,\"Debug\":true}" -> ok | plain ok | same values: true | msg: <nil>
// unmarshal "{\"unknown\":1,\"name\":\"after\"}" -> ok | plain ok | same values: true | msg: <nil>
// unmarshal "{\"name\":5}"                   -> type number | plain type number | same values: true | msg: json: cannot unmarshal number into Go struct field gp_plain.name of type string
// unmarshal "{\"port\":\"x\"}"               -> type string | plain type string | same values: true | msg: json: cannot unmarshal string into Go struct field gp_plain.port of type int
// unmarshal "{\"quoted\":5}"                 -> *errors.errorString | plain *errors.errorString | same values: true | msg: json: invalid use of ,string struct tag, trying to unmarshal unquoted value into int64
// unmarshal "{\"quoted\":\"x\"}"             -> *errors.errorString | plain *errors.errorString | same values: true | msg: json: invalid use of ,string struct tag, trying to unmarshal "x" into int64
// unmarshal "{\"when\":\"yesterday\"}"       -> *time.ParseError | plain *time.ParseError | same values: true | msg: parsing time "yesterday" as "2006-01-02T15:04:05Z07:00": cannot parse "yesterday" as "2006"
// unmarshal "[1]"                            -> type array | plain type array | same values: true | msg: json: cannot unmarshal array into Go value of type main.gp_plain
// unmarshal "{\"name\":\"x\""                -> syntax | plain syntax | same values: true | msg: unexpected end of JSON input
// unmarshal "{\"name\" 1}"                   -> syntax | plain syntax | same values: true | msg: invalid character '1' after object key
// unmarshal ""                               -> syntax | plain syntax | same values: true | msg: unexpected end of JSON input
// unmarshal "tru"                            -> syntax | plain syntax | same values: true | msg: invalid character ' ' in literal true (expecting 'e')
// unmarshal "{\"tags\":null,\"env\":null,\"ptr\":null,\"any\":null}" -> ok | plain ok | same values: true | msg: <nil>
// unmarshal "{\"raw\":null}"                 -> ok | plain ok | same values: true | msg: <nil>
// html: {"s":"\u003ca href=\"x\"\u003e\u0026\u003c/a\u003e"} | same as plain: true
// {"s":"a\u0026b"}
// [{"s":"c"}]
// {}
// <nil> direct
// {"s":"direct"} <nil>
// {"s":"ptr"} <nil>
// <nil> <nil> <nil> {1} {2} [{3}] false
