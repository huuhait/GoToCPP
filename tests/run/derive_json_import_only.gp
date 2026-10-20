// phase: 09

// #Derive(JSON) in a file whose only use of encoding/json is the generated code, with the package imported under another name: the
// import is not reported as unused, and the methods are written with the file's name for the package (ADR-102). The twin
// (testdata/derive_twin/derive_json_import_only.go) writes the methods by hand.

package main

import (
	"fmt"
	j "encoding/json"
)

#Derive(JSON)
type Cfg struct {
	Name string `json:"name"`
	Port int    `json:"port,omitempty"`
}

#Derive(JSON)
type Pair[K comparable, V any] struct {
	Key K `json:"k"`
	Val V `json:"v"`
}

type Base struct{ ID int }

#Derive(JSON)
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

// Output:
// {"name":"q"} <nil>
// <nil> {z 9}
// json: cannot unmarshal number into Go struct field gp_plain.name of type string
// {"k":"a","v":1} <nil>
// <nil> {z [1 2]}
// {"ID":3,"N":"n"}
