// #Derive(JSON) in a file whose only use of encoding/json is the generated code, with the package imported under another name: the
// import is not reported as unused, and the methods are written with the file's name for the package (ADR-102). The twin
// (testdata/derive_twin/derive_json_import_only.go) writes the methods by hand.

package main

import (
	"fmt"
	j "encoding/json"
)

type Cfg struct {
	Name string `json:"name"`
	Port int    `json:"port,omitempty"`
}

type Pair[K comparable, V any] struct {
	Key K `json:"k"`
	Val V `json:"v"`
}

type Base struct{ ID int }

type Emb struct {
	Base
	N string
}

func main() {
	b, err := Cfg{"q", 0}.MarshalJSON()
	fmt.Println(string(b), err)
	var c Cfg
	fmt.Println(c.UnmarshalJSON([]byte(`{"NAME":"z","port":9}`)), c)
	fmt.Println(c.UnmarshalJSON([]byte(`{"name":5}`)))
	p := Pair[string, int]{"a", 1}
	b, err = p.MarshalJSON()
	fmt.Println(string(b), err)
	var q Pair[string, []int]
	fmt.Println(q.UnmarshalJSON([]byte(`{"k":"z","v":[1,2]}`)), q)
	b, _ = Emb{Base{3}, "n"}.MarshalJSON()
	fmt.Println(string(b))
}

func (c Cfg) MarshalJSON() ([]byte, error) {
	type gp_plain Cfg
	return j.Marshal(gp_plain(c))
}

func (c *Cfg) UnmarshalJSON(data []byte) error {
	type gp_plain Cfg
	return j.Unmarshal(data, (*gp_plain)(c))
}

func (p Pair[K, V]) MarshalJSON() ([]byte, error) {
	type gp_plain Pair[K, V]
	return j.Marshal(gp_plain(p))
}

func (p *Pair[K, V]) UnmarshalJSON(data []byte) error {
	type gp_plain Pair[K, V]
	return j.Unmarshal(data, (*gp_plain)(p))
}

func (e Emb) MarshalJSON() ([]byte, error) {
	type gp_plain Emb
	return j.Marshal(gp_plain(e))
}

func (e *Emb) UnmarshalJSON(data []byte) error {
	type gp_plain Emb
	return j.Unmarshal(data, (*gp_plain)(e))
}
