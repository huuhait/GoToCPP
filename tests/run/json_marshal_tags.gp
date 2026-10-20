// phase: 09
// encoding/json struct tags: names, omitempty, omitzero, string, "-", embedded structs and pointers, shadowing, unexported fields.

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type T1 struct {
	A int    `json:"a"`
	B string `json:"bee"`
	C bool   `json:",omitempty"`
	D int    `json:"-"`
	E int    `json:"-,"`
	F int    `json:",string"`
	G string `json:"g,omitempty"`
	H []int  `json:"h,omitempty"`
	I map[string]int `json:"i,omitempty"`
	J *int   `json:"j,omitempty"`
	K float64 `json:"k,omitempty"`
	L any    `json:"l,omitempty"`
	M [0]int `json:"m,omitempty"`
	N struct{} `json:"n,omitempty"`
}

type OZ struct {
	T  time.Time  `json:"t,omitzero"`
	U  time.Time  `json:"u,omitempty"`
	V  int        `json:"v,omitzero"`
	W  *int       `json:"w,omitzero"`
	X  []int      `json:"x,omitzero"`
	Y  []int      `json:"y,omitzero,omitempty"`
	Z  [2]int     `json:"z,omitzero"`
	ZZ struct{ A int } `json:"zz,omitzero"`
}

type Str struct {
	I  int     `json:",string"`
	U  uint8   `json:",string"`
	F  float64 `json:",string"`
	B  bool    `json:",string"`
	S  string  `json:",string"`
	P  *int    `json:",string"`
	SL []int   `json:",string"`
}

type Base struct {
	ID   int
	Name string
}

type Embedded struct {
	Base
	Extra string
}

type EmbeddedPtr struct {
	*Base
	Extra string
}

type Tagged struct {
	Base `json:"base"`
	X    int
}

type Shadow struct {
	Base
	Name string
}

type A struct{ Name string }
type B struct{ Name string }
type Conflict struct {
	A
	B
	Other int
}

type TagWins struct {
	A
	B `json:"B"`
}

type unexp struct {
	Hidden int
}

type WithUnexported struct {
	unexp
	Shown int
	x     int
}

type Exp struct{ V int }
type wrap struct {
	Exp
	W int
}

type BadTag struct {
	A int `json:"a b"`
	B int `json:"c\\d"`
	C int `json:"héllo"`
	D int `json:"<>&"`
	E int `json:"é,omitempty"`
	F int `json:"quo\"te"`
}

type Multi struct {
	A int `json:"a" xml:"b"`
	B int `xml:"b" json:"bb"`
	C int `json:"c,omitempty,string"`
}

func show(label string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		return
	}
	fmt.Printf("%s: %s\n", label, b)
}

func main() {
	show("t1 zero", T1{})
	n := 3
	show("t1 full", T1{A: 1, B: "b", C: true, D: 4, E: 5, F: 6, G: "g", H: []int{1}, I: map[string]int{"k": 1}, J: &n, K: 1.5, L: 1})
	show("omitzero zero", OZ{})
	show("omitzero set", OZ{T: time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC), V: 1, W: &n, X: []int{}, Y: []int{}, Z: [2]int{0, 1}, ZZ: struct{ A int }{1}})
	show("string opt zero", Str{})
	v := 7
	show("string opt", Str{I: 1, U: 2, F: 3.5, B: true, S: "q\"x", P: &v, SL: []int{1}})
	show("embedded", Embedded{Base{1, "n"}, "e"})
	show("embedded ptr", EmbeddedPtr{&Base{2, "m"}, "e"})
	show("embedded ptr nil", EmbeddedPtr{nil, "e"})
	show("tagged embedded", Tagged{Base{3, "t"}, 4})
	show("shadow", Shadow{Base{1, "base"}, "outer"})
	show("conflict", Conflict{A{"a"}, B{"b"}, 9})
	show("tag wins", TagWins{A{"a"}, B{"b"}})
	show("unexported embedded", WithUnexported{unexp{1}, 2, 3})
	show("unexported struct embedding exported", wrap{Exp{5}, 6})
	show("bad tags", BadTag{1, 2, 3, 4, 5, 6})
	show("multi", Multi{1, 2, 0})
	show("multi nonzero", Multi{1, 2, 3})
}

// Output:
// t1 zero: {"a":0,"bee":"","-":0,"F":"0","n":{}}
// t1 full: {"a":1,"bee":"b","C":true,"-":5,"F":"6","g":"g","h":[1],"i":{"k":1},"j":3,"k":1.5,"l":1,"n":{}}
// omitzero zero: {"u":"0001-01-01T00:00:00Z"}
// omitzero set: {"t":"2020-01-02T03:04:05.000000006Z","u":"0001-01-01T00:00:00Z","v":1,"w":3,"x":[],"z":[0,1],"zz":{"A":1}}
// string opt zero: {"I":"0","U":"0","F":"0","B":"false","S":"\"\"","P":null,"SL":null}
// string opt: {"I":"1","U":"2","F":"3.5","B":"true","S":"\"q\\\"x\"","P":"7","SL":[1]}
// embedded: {"ID":1,"Name":"n","Extra":"e"}
// embedded ptr: {"ID":2,"Name":"m","Extra":"e"}
// embedded ptr nil: {"Extra":"e"}
// tagged embedded: {"base":{"ID":3,"Name":"t"},"X":4}
// shadow: {"ID":1,"Name":"outer"}
// conflict: {"Other":9}
// tag wins: {"Name":"a","B":{"Name":"b"}}
// unexported embedded: {"Hidden":1,"Shown":2}
// unexported struct embedding exported: {"V":5,"W":6}
// bad tags: {"a b":1,"B":2,"héllo":3,"\u003c\u003e\u0026":4,"é":5,"F":6}
// multi: {"a":1,"bb":2}
// multi nonzero: {"a":1,"bb":2,"c":"3"}
