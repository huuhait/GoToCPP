// #Derive(JSON) (step 9.11, ADR-102): MarshalJSON and UnmarshalJSON that call encoding/json on a copy of the struct type that has no methods,
// so the output and the input are exactly encoding/json's. Every derived type here has a twin with the same fields and no #Derive; the
// program prints both and whether they agree. The twin of this file (testdata/derive_twin/derive_json.go) writes the generated methods by hand.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

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

type Pair[K comparable, V any] struct {
	Key K `json:"k"`
	Val V `json:"v"`
}

type PairPlain[K comparable, V any] struct {
	Key K `json:"k"`
	Val V `json:"v"`
}

type Html struct {
	S string `json:"s"`
}

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

func (c Cfg) MarshalJSON() ([]byte, error) {
	type gp_plain Cfg
	return json.Marshal(gp_plain(c))
}

func (c *Cfg) UnmarshalJSON(data []byte) error {
	type gp_plain Cfg
	return json.Unmarshal(data, (*gp_plain)(c))
}

func (n Node) MarshalJSON() ([]byte, error) {
	type gp_plain Node
	return json.Marshal(gp_plain(n))
}

func (n *Node) UnmarshalJSON(data []byte) error {
	type gp_plain Node
	return json.Unmarshal(data, (*gp_plain)(n))
}

func (p Pair[K, V]) MarshalJSON() ([]byte, error) {
	type gp_plain Pair[K, V]
	return json.Marshal(gp_plain(p))
}

func (p *Pair[K, V]) UnmarshalJSON(data []byte) error {
	type gp_plain Pair[K, V]
	return json.Unmarshal(data, (*gp_plain)(p))
}

func (h Html) MarshalJSON() ([]byte, error) {
	type gp_plain Html
	return json.Marshal(gp_plain(h))
}

func (h *Html) UnmarshalJSON(data []byte) error {
	type gp_plain Html
	return json.Unmarshal(data, (*gp_plain)(h))
}

func (e Empty) MarshalJSON() ([]byte, error) {
	type gp_plain Empty
	return json.Marshal(gp_plain(e))
}

func (e *Empty) UnmarshalJSON(data []byte) error {
	type gp_plain Empty
	return json.Unmarshal(data, (*gp_plain)(e))
}
