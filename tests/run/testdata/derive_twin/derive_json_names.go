// #Derive(JSON) on types and in packages whose names collide with the names the generated code uses (the parameter data, the local type
// gp_plain, the receivers): the generator picks other names, and the methods work. The twin (testdata/derive_twin/derive_json_names.go) writes
// the methods by hand.

package main

import (
	"encoding/json"
	"fmt"
)

type data struct {
	A int `json:"a"`
}

type gp_data struct {
	A int `json:"a"`
}

type gp_plain struct {
	A int `json:"a"`
}

type b struct{ A int }

type v struct{ A int }

type d struct{ A int }

type x struct{ A int }

type bytes struct{ A int }

type Marshal struct{ A int }

// Package-level names like the receivers and the locals that the generator could pick.
var c, t, gp_recv = 1, 2, 3

func Unmarshal() {}

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

func (r_ data) MarshalJSON() ([]byte, error) {
	type gp_plain data
	return json.Marshal(gp_plain(r_))
}

func (r_ *data) UnmarshalJSON(gp_data []byte) error {
	type gp_plain data
	return json.Unmarshal(gp_data, (*gp_plain)(r_))
}

func (r_ gp_data) MarshalJSON() ([]byte, error) {
	type gp_plain gp_data
	return json.Marshal(gp_plain(r_))
}

func (r_ *gp_data) UnmarshalJSON(data []byte) error {
	type gp_plain gp_data
	return json.Unmarshal(data, (*gp_plain)(r_))
}

func (r_ gp_plain) MarshalJSON() ([]byte, error) {
	type gp_plain1 gp_plain
	return json.Marshal(gp_plain1(r_))
}

func (r_ *gp_plain) UnmarshalJSON(data []byte) error {
	type gp_plain1 gp_plain
	return json.Unmarshal(data, (*gp_plain1)(r_))
}

func (r_ b) MarshalJSON() ([]byte, error) {
	type gp_plain b
	return json.Marshal(gp_plain(r_))
}

func (r_ *b) UnmarshalJSON(data []byte) error {
	type gp_plain b
	return json.Unmarshal(data, (*gp_plain)(r_))
}

func (r_ v) MarshalJSON() ([]byte, error) {
	type gp_plain v
	return json.Marshal(gp_plain(r_))
}

func (r_ *v) UnmarshalJSON(data []byte) error {
	type gp_plain v
	return json.Unmarshal(data, (*gp_plain)(r_))
}

func (r_ d) MarshalJSON() ([]byte, error) {
	type gp_plain d
	return json.Marshal(gp_plain(r_))
}

func (r_ *d) UnmarshalJSON(data []byte) error {
	type gp_plain d
	return json.Unmarshal(data, (*gp_plain)(r_))
}

func (r_ x) MarshalJSON() ([]byte, error) {
	type gp_plain x
	return json.Marshal(gp_plain(r_))
}

func (r_ *x) UnmarshalJSON(data []byte) error {
	type gp_plain x
	return json.Unmarshal(data, (*gp_plain)(r_))
}

func (r_ bytes) MarshalJSON() ([]byte, error) {
	type gp_plain bytes
	return json.Marshal(gp_plain(r_))
}

func (r_ *bytes) UnmarshalJSON(data []byte) error {
	type gp_plain bytes
	return json.Unmarshal(data, (*gp_plain)(r_))
}

func (r_ Marshal) MarshalJSON() ([]byte, error) {
	type gp_plain Marshal
	return json.Marshal(gp_plain(r_))
}

func (r_ *Marshal) UnmarshalJSON(data []byte) error {
	type gp_plain Marshal
	return json.Unmarshal(data, (*gp_plain)(r_))
}

func (r_ Holder) MarshalJSON() ([]byte, error) {
	type gp_plain Holder
	return json.Marshal(gp_plain(r_))
}

func (r_ *Holder) UnmarshalJSON(data []byte) error {
	type gp_plain Holder
	return json.Unmarshal(data, (*gp_plain)(r_))
}
