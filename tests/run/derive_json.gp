// phase: 09

// #Derive(JSON) on a struct (the exit criterion of Phase 10): MarshalJSON and UnmarshalJSON from the struct tags,
// and the output equals what encoding/json prints for the same struct without the line. The twin
// (testdata/derive_twin/derive_json.go) is this program without #Derive(JSON), so real Go's encoding/json gives
// the expected output.

package main

import (
	"encoding/json"
	"fmt"
)

type Inner struct {
	A int    `json:"a"`
	B string `json:"b,omitempty"`
}

#Derive(JSON)
type Event struct {
	ID      int              `json:"id"`
	Name    string           `json:"name"`
	Tags    []string         `json:"tags"`
	Attrs   map[string]int   `json:"attrs,omitempty"`
	Inner   Inner            `json:"inner"`
	Ptr     *Inner           `json:"ptr"`
	Skip    string           `json:"-"`
	Quoted  int64            `json:"quoted,string"`
	Ratio   float64          `json:"ratio"`
	Flag    bool             `json:"flag,omitempty"`
	Raw     []byte           `json:"raw"`
	Nested  map[string][]int `json:"nested"`
	private int
	NoTag   string
}

func main() {
	evs := []Event{
		{ID: 1, Name: "plain", Tags: []string{"a", "b"}, Inner: Inner{A: 1}, Quoted: 42, Ratio: 0.5, Raw: []byte("hi"), NoTag: "n"},
		{ID: 2, Name: "<esc> & \"quotes\"   é", Attrs: map[string]int{"z": 1, "a": 2}, Ptr: &Inner{A: 2, B: "b"}, Skip: "skipped", Flag: true, Ratio: 1e21, Nested: map[string][]int{"k": {1, 2}}},
		{},
	}
	for _, e := range evs {
		b, err := json.Marshal(e)
		fmt.Println(string(b), err)
		var back Event
		err = json.Unmarshal(b, &back)
		fmt.Println(back.ID, back.Name, back.Tags == nil, back.Ptr != nil, back.Quoted, back.Ratio, err)
	}
	var bad Event
	fmt.Println(json.Unmarshal([]byte(`{"id":"x"}`), &bad) != nil)
}

// Output:
// {"id":1,"name":"plain","tags":["a","b"],"inner":{"a":1},"ptr":null,"quoted":"42","ratio":0.5,"raw":"aGk=","nested":null,"NoTag":"n"} <nil>
// 1 plain false false 42 0.5 <nil>
// {"id":2,"name":"\u003cesc\u003e \u0026 \"quotes\" \u2028 é","tags":null,"attrs":{"a":2,"z":1},"inner":{"a":0},"ptr":{"a":2,"b":"b"},"quoted":"0","ratio":1e+21,"flag":true,"raw":null,"nested":{"k":[1,2]},"NoTag":""} <nil>
// 2 <esc> & "quotes"   é true true 0 1e+21 <nil>
// {"id":0,"name":"","tags":null,"inner":{"a":0},"ptr":null,"quoted":"0","ratio":0,"raw":null,"nested":null,"NoTag":""} <nil>
// 0  true false 0 0 <nil>
// true
