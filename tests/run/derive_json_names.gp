// phase: 09

// #Derive(JSON) on types and in packages whose names collide with the names the generated code uses (the parameter data, the local type
// gp_plain, the receivers): the generator picks other names, and the methods work. The twin (testdata/derive_twin/derive_json_names.go) writes
// the methods by hand.

package main

import (
	"encoding/json"
	"fmt"
)

#Derive(JSON)
type data struct {
	A int `json:"a"`
}

#Derive(JSON)
type gp_data struct {
	A int `json:"a"`
}

#Derive(JSON)
type gp_plain struct {
	A int `json:"a"`
}

#Derive(JSON)
type b struct{ A int }

#Derive(JSON)
type v struct{ A int }

#Derive(JSON)
type d struct{ A int }

#Derive(JSON)
type x struct{ A int }

#Derive(JSON)
type bytes struct{ A int }

#Derive(JSON)
type Marshal struct{ A int }

// Package-level names like the receivers and the locals that the generator could pick.
var c, t, gp_recv = 1, 2, 3

func Unmarshal() {}

#Derive(JSON)
type Holder struct {
	data   int
	Data   data      `json:"data"`
	Plain  gp_plain  `json:"plain"`
	Inner  gp_data   `json:"inner"`
	B      b         `json:"b"`
	Bytes  []bytes   `json:"bytes"`
	M      *Marshal  `json:"m"`
}

func main() {
	h := Holder{Data: data{1}, Plain: gp_plain{2}, Inner: gp_data{3}, B: b{4}, Bytes: []bytes{{5}}, M: &Marshal{6}}
	out, err := json.Marshal(h)
	fmt.Println(string(out), err)
	var back Holder
	fmt.Println(json.Unmarshal(out, &back), back.Data, back.Plain, back.Inner, back.B, back.Bytes, *back.M)
	for _, in := range []any{data{1}, gp_data{2}, gp_plain{3}, b{4}, v{5}, d{6}, x{7}, bytes{8}, Marshal{9}} {
		o, err := json.Marshal(&in)
		fmt.Println(string(o), err)
	}
	var dd data
	fmt.Println(json.Unmarshal([]byte(`{"a":42}`), &dd), dd)
	var gd gp_data
	fmt.Println(dd.UnmarshalJSON([]byte(`{"a":7}`)), json.Unmarshal([]byte(`{"a":8}`), &gd), dd, gd)
	fmt.Println(c, t, gp_recv)
}

// Output:
// {"data":{"a":1},"plain":{"a":2},"inner":{"a":3},"b":{"A":4},"bytes":[{"A":5}],"m":{"A":6}} <nil>
// <nil> {1} {2} {3} {4} [{5}] {6}
// {"a":1} <nil>
// {"a":2} <nil>
// {"a":3} <nil>
// {"A":4} <nil>
// {"A":5} <nil>
// {"A":6} <nil>
// {"A":7} <nil>
// {"A":8} <nil>
// {"A":9} <nil>
// <nil> {42}
// <nil> <nil> {7} {8}
// 1 2 3
