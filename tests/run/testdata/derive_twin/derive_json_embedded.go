// #Derive(JSON) with embedded fields (step 9.11 review, round 3). The encoder and the decoder work on the struct that the type stands for, so a method
// that the struct gets by promotion from an embedded field (time.Time, a type with its own MarshalJSON and UnmarshalJSON, a pointer to one) must be
// found there in both directions, as it is by json.Marshal and json.Unmarshal of the struct (the twin writes the generated methods by hand, which
// is what the generated code does). A type that embeds a derived one gets its methods by promotion too and must not use the route of the derived type
// when it has methods of its own, even when the embedded pointer is nil. A hand-written method with the name GoCPPJSONPlainPtr on another type is
// ignored. And the cycle detection of the encoder starts after 1000 levels, as for a plain struct.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Stamp has methods of its own: "s<N>" in both directions.
type Stamp struct{ N int }

func (s Stamp) MarshalJSON() ([]byte, error) { return []byte(`"s` + strconv.Itoa(s.N) + `"`), nil }

func (s *Stamp) UnmarshalJSON(b []byte) error {
	t := strings.Trim(string(b), `"`)
	n, err := strconv.Atoi(strings.TrimPrefix(t, "s"))
	if err != nil || !strings.HasPrefix(t, "s") {
		return errors.New("bad stamp " + string(b))
	}
	s.N = n
	return nil
}

type Ev struct {
	time.Time
	Name string `json:"name"`
}

type WithStamp struct {
	Stamp
	X int `json:"x"`
}

type WithPtr struct {
	*Stamp
	Y int `json:"y"`
}

type Base struct {
	A int    `json:"a"`
	B string `json:"b,omitempty"`
}

// Mid gets the methods of Base by promotion and has none of its own.
type Mid struct {
	*Base
	M int `json:"m"`
}

// PE2 has a MarshalJSON of its own and embeds a pointer to a derived type: the pointer can be nil.
type PE2 struct {
	*Base
	C int `json:"c"`
}

func (PE2) MarshalJSON() ([]byte, error) { return []byte(`"ok2"`), nil }

// PE4 has one too and the nil pointer is in the middle of the chain to Base.
type PE4 struct {
	*Mid
	Z int `json:"z"`
}

func (PE4) MarshalJSON() ([]byte, error) { return []byte(`"ok4"`), nil }

func (p *PE4) UnmarshalJSON(b []byte) error {
	p.Z = len(b)
	return nil
}

// Types that are not derived and have a method with the reserved name: it must be ignored.
type RogueNil struct{ A int }

func (RogueNil) MarshalJSON() ([]byte, error)    { return []byte(`"rogue-nil"`), nil }
func (r *RogueNil) UnmarshalJSON(b []byte) error { r.A = -1; return nil }
func (r *RogueNil) GoCPPJSONPlainPtr() any       { return nil }

type RogueInt struct{ A int }

func (RogueInt) MarshalJSON() ([]byte, error)    { return []byte(`"rogue-int"`), nil }
func (r *RogueInt) UnmarshalJSON(b []byte) error { r.A = -2; return nil }
func (r *RogueInt) GoCPPJSONPlainPtr() any       { return new(int) }

type other struct{ A int }

type RogueStruct struct{ A int }

func (RogueStruct) MarshalJSON() ([]byte, error)    { return []byte(`"rogue-struct"`), nil }
func (r *RogueStruct) UnmarshalJSON(b []byte) error { r.A = -3; return nil }
func (r *RogueStruct) GoCPPJSONPlainPtr() any       { return &other{99} }

type RogueSelf struct{ A int }

func (RogueSelf) MarshalJSON() ([]byte, error)    { return []byte(`"rogue-self"`), nil }
func (r *RogueSelf) UnmarshalJSON(b []byte) error { r.A = -4; return nil }
func (r *RogueSelf) GoCPPJSONPlainPtr() any       { return r }

// Cycle detection: Cnt counts the times the encoder reaches it.
var visits int

type Cnt struct{}

func (Cnt) MarshalJSON() ([]byte, error) { visits++; return []byte("1"), nil }

type CN struct {
	C    Cnt
	Next *CN
}

type PCN struct {
	C    Cnt
	Next *PCN
}

func show(label string, v any, err error) {
	b, merr := json.Marshal(v)
	fmt.Printf("%s: %s %v | marshal: %s %v\n", label, fmt.Sprint(v), err, b, merr)
}

func try(label string, v any) {
	b, err := json.Marshal(v)
	fmt.Printf("%s: %s %v\n", label, b, err)
}

func main() {
	when := time.Date(2024, 2, 29, 13, 14, 15, 0, time.UTC)

	// time.Time embedded: the methods of time.Time are the type's.
	try("ev", Ev{when, "x"})
	try("&ev", &Ev{when, "x"})
	try("[]ev", []Ev{{when, "x"}})
	try("map ev", map[string]Ev{"k": {when, "x"}})
	try("any ev", []any{Ev{when, "x"}})
	var ev Ev
	err := json.Unmarshal([]byte(`"2023-01-02T03:04:05Z"`), &ev)
	show("unmarshal ev", ev.Time.UTC(), err)
	var evs []Ev
	err = json.Unmarshal([]byte(`["2023-01-02T03:04:05Z","2024-01-02T03:04:05Z"]`), &evs)
	fmt.Println("unmarshal []ev:", len(evs), err, evs[1].Time.Year())
	var evm map[string]*Ev
	err = json.Unmarshal([]byte(`{"k":"2023-01-02T03:04:05Z","n":null}`), &evm)
	fmt.Println("unmarshal map ev:", len(evm), err, evm["k"].Time.Year(), evm["n"] == nil)
	err = json.Unmarshal([]byte(`{"name":"x"}`), &ev)
	fmt.Println("unmarshal ev from an object:", err != nil)

	// A type with methods of its own, embedded by value and by pointer.
	try("withstamp", WithStamp{Stamp{7}, 1})
	var ws WithStamp
	err = json.Unmarshal([]byte(`"s42"`), &ws)
	fmt.Println("unmarshal withstamp:", ws, err)
	err = json.Unmarshal([]byte(`"nope"`), &ws)
	fmt.Println("unmarshal withstamp, bad:", err)
	try("withptr", WithPtr{&Stamp{8}, 2})
	wp := WithPtr{Stamp: &Stamp{}}
	err = json.Unmarshal([]byte(`"s43"`), &wp)
	fmt.Println("unmarshal withptr:", wp.Stamp.N, wp.Y, err)
	var holder struct {
		W  WithStamp
		P  *WithStamp
		L  []WithStamp
		Ev Ev
	}
	err = json.Unmarshal([]byte(`{"W":"s1","P":"s2","L":["s3","s4"],"Ev":"2020-05-06T07:08:09Z"}`), &holder)
	fmt.Println("holder:", holder.W, holder.P, holder.L, holder.Ev.Time.Year(), err)

	// A derived type embedded in a type that is not: the methods are promoted, and only the embedded part is encoded.
	try("mid", Mid{&Base{1, "b"}, 5})
	var mid Mid
	mid.Base = &Base{}
	err = json.Unmarshal([]byte(`{"a":3,"b":"x","m":9}`), &mid)
	fmt.Println("mid:", *mid.Base, mid.M, err)

	// A type with methods of its own that embeds a nil pointer to a derived type (and the nil can be in the middle of the chain).
	try("pe2", PE2{nil, 1})
	try("&pe2", &PE2{nil, 1})
	try("[]pe2", []PE2{{nil, 1}})
	try("map pe2", map[string]PE2{"k": {nil, 1}})
	try("any pe2", []any{PE2{nil, 1}, &PE2{nil, 2}})
	try("pe2 with base", PE2{&Base{1, "b"}, 1})
	try("pe4", PE4{nil, 1})
	try("&pe4", &PE4{nil, 1})
	try("[]pe4", []PE4{{nil, 1}, {&Mid{nil, 1}, 2}, {&Mid{&Base{1, ""}, 2}, 3}})
	var pe4 PE4
	err = json.Unmarshal([]byte(`{"z":1}`), &pe4)
	fmt.Println("pe4:", pe4.Z, pe4.Mid == nil, err)
	var pe4s []PE4
	err = json.Unmarshal([]byte(`[{"z":1},[1,2]]`), &pe4s)
	fmt.Println("[]pe4:", len(pe4s), pe4s[0].Z, pe4s[1].Z, err)

	// Hand-written methods with the reserved name on types that are not derived.
	try("rogue nil", RogueNil{1})
	try("&rogue nil", &RogueNil{1})
	try("[]rogue nil", []RogueNil{{1}})
	try("rogue int", []RogueInt{{1}})
	try("rogue struct", []RogueStruct{{1}})
	try("&rogue struct", &RogueStruct{1})
	try("rogue self", []RogueSelf{{1}})
	var rn RogueNil
	var ri []RogueInt
	var rs map[string]RogueStruct
	var rf *RogueSelf
	e1 := json.Unmarshal([]byte(`{"A":5}`), &rn)
	e2 := json.Unmarshal([]byte(`[{"A":5}]`), &ri)
	e3 := json.Unmarshal([]byte(`{"k":{"A":5}}`), &rs)
	e4 := json.Unmarshal([]byte(`{"A":5}`), &rf)
	fmt.Println("rogues:", rn.A, ri[0].A, rs["k"].A, rf.A, e1, e2, e3, e4)

	// The cycle detection starts after 1000 levels of pointers, as for a plain struct: the number of nodes reached before the error is the same.
	// (Where the stack holds fewer than 1000 levels of either, the error is the one about the depth and the counts do not matter.)
	count := func(v any) (string, int) {
		visits = 0
		_, err := json.Marshal(v)
		msg := "ok"
		var ue *json.UnsupportedValueError
		if errors.As(err, &ue) {
			msg = "cycle"
			if strings.Contains(ue.Error(), "exceeded max depth") {
				msg = "too deep"
			}
		}
		return msg, visits
	}
	leaf := &CN{}
	pleaf := &PCN{}
	cyc := &CN{}
	cyc.Next = cyc
	pcyc := &PCN{}
	pcyc.Next = pcyc
	wide := make([]*CN, 0, 1200)
	pwide := make([]*PCN, 0, 1200)
	for i := 0; i < 1100; i++ {
		wide = append(wide, leaf)
		pwide = append(pwide, pleaf)
	}
	m1, c1 := count(append(wide, cyc))
	m2, c2 := count(append(pwide, pcyc))
	same := m1 == m2 && (m1 != "cycle" || c1 == c2)
	fmt.Println("cycle after 1100 shallow pointers: same as plain:", same, m1 != "ok")
	m1, c1 = count(cyc)
	m2, c2 = count(pcyc)
	same = m1 == m2 && (m1 != "cycle" || c1 == c2)
	fmt.Println("cycle alone: same as plain:", same, m1 != "ok")
	cyc.Next, pcyc.Next = nil, nil // a cycle is never freed: break them
	// A list that is not a cycle, 600 levels deep, is not one.
	var head *CN
	var phead *PCN
	for i := 0; i < 600; i++ {
		head = &CN{Next: head}
		phead = &PCN{Next: phead}
	}
	m1, _ = count(head)
	m2, _ = count(phead)
	fmt.Println("list of 600:", m1, m2)
}

func (g Ev) MarshalJSON() ([]byte, error) {
	type gp_plain Ev
	return json.Marshal(gp_plain(g))
}

func (g *Ev) UnmarshalJSON(data []byte) error {
	type gp_plain Ev
	return json.Unmarshal(data, (*gp_plain)(g))
}

func (g WithStamp) MarshalJSON() ([]byte, error) {
	type gp_plain WithStamp
	return json.Marshal(gp_plain(g))
}

func (g *WithStamp) UnmarshalJSON(data []byte) error {
	type gp_plain WithStamp
	return json.Unmarshal(data, (*gp_plain)(g))
}

func (g WithPtr) MarshalJSON() ([]byte, error) {
	type gp_plain WithPtr
	return json.Marshal(gp_plain(g))
}

func (g *WithPtr) UnmarshalJSON(data []byte) error {
	type gp_plain WithPtr
	return json.Unmarshal(data, (*gp_plain)(g))
}

func (g Base) MarshalJSON() ([]byte, error) {
	type gp_plain Base
	return json.Marshal(gp_plain(g))
}

func (g *Base) UnmarshalJSON(data []byte) error {
	type gp_plain Base
	return json.Unmarshal(data, (*gp_plain)(g))
}
