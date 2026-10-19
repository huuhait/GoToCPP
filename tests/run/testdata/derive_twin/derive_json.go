package main

import (
	"encoding/json"
	"fmt"
)

type Inner struct {
	A int    `json:"a"`
	B string `json:"b,omitempty"`
}

type Event struct {
	ID      int               `json:"id"`
	Name    string            `json:"name"`
	Tags    []string          `json:"tags"`
	Attrs   map[string]int    `json:"attrs,omitempty"`
	Inner   Inner             `json:"inner"`
	Ptr     *Inner            `json:"ptr"`
	Skip    string            `json:"-"`
	Quoted  int64             `json:"quoted,string"`
	Ratio   float64           `json:"ratio"`
	Flag    bool              `json:"flag,omitempty"`
	Raw     []byte            `json:"raw"`
	Nested  map[string][]int  `json:"nested"`
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
