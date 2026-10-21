// phase: 09
// encoding/json struct fields: omitempty and omitzero for every kind, the string option, name
// resolution of embedded structs (depth, tags, ambiguity), unexported fields, "-" tags, and
// case-insensitive matching on decode.

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type Empty struct{}

type Pair struct{ A, B int }

type Zeroer struct{ Value int }

func (z Zeroer) IsZero() bool { return z.Value == 42 }

type PtrZeroer struct{ Value int }

func (z *PtrZeroer) IsZero() bool { return z.Value == 42 }

type OmitAll struct {
	Bool      bool            `json:"bool,omitempty"`
	Int       int             `json:"int,omitempty"`
	Uint      uint8           `json:"uint,omitempty"`
	Float     float64         `json:"float,omitempty"`
	String    string          `json:"string,omitempty"`
	Slice     []int           `json:"slice,omitempty"`
	SliceFull []int           `json:"sliceFull,omitempty"`
	Map       map[string]int  `json:"map,omitempty"`
	MapEmpty  map[string]int  `json:"mapEmpty,omitempty"`
	Pointer   *int            `json:"pointer,omitempty"`
	Interface any             `json:"interface,omitempty"`
	Array     [2]int          `json:"array,omitempty"`
	Array0    [0]int          `json:"array0,omitempty"`
	Struct    Pair            `json:"struct,omitempty"`
	StructPtr *Pair           `json:"structPtr,omitempty"`
	Raw       json.RawMessage `json:"raw,omitempty"`
	Number    json.Number     `json:"number,omitempty"`
	Bytes     []byte          `json:"bytes,omitempty"`
	Time      time.Time       `json:"time,omitempty"`
}

type ZeroAll struct {
	Bool       bool           `json:"bool,omitzero"`
	Int        int            `json:"int,omitzero"`
	Float      float64        `json:"float,omitzero"`
	NegZero    float64        `json:"negzero,omitzero"`
	String     string         `json:"string,omitzero"`
	Slice      []int          `json:"slice,omitzero"`
	SliceEmpty []int          `json:"sliceEmpty,omitzero"`
	Map        map[string]int `json:"map,omitzero"`
	MapEmpty   map[string]int `json:"mapEmpty,omitzero"`
	Pointer    *int           `json:"pointer,omitzero"`
	Interface  any            `json:"interface,omitzero"`
	Array      [2]int         `json:"array,omitzero"`
	Struct     Pair           `json:"struct,omitzero"`
	StructPtr  *Pair          `json:"structPtr,omitzero"`
	Time       time.Time      `json:"time,omitzero"`
	Zeroer     Zeroer         `json:"zeroer,omitzero"`
	ZeroerPtr  *Zeroer        `json:"zeroerPtr,omitzero"`
	PtrZeroer  PtrZeroer      `json:"ptrZeroer,omitzero"`
	Both       []int          `json:"both,omitempty,omitzero"`
	Order      []int          `json:"order,omitzero,omitempty"`
}

func show(label string, value any) {
	encoded, err := json.Marshal(value)
	fmt.Printf("%s: %s %v\n", label, encoded, err)
}

type Base struct {
	ID     int
	Name   string
	Shared string
}

type Mixin struct {
	Name   string
	Extra  int
	Shared string
}

type Deep struct {
	Shared string
	Level  int `json:"deep"`
}

type Wrapper struct {
	Deep
}

type TaggedEmbed struct {
	Base `json:"base"`
	Top  int
}

type PointerEmbed struct {
	*Base
	Other int
}

type unexportedBase struct {
	Visible int
	hidden  int
}

type UnexportedEmbed struct {
	unexportedBase
	Own int
}

type SameDepthUntagged struct{ Name string }

type SameDepthTagged struct {
	Name string `json:"Name"`
}

type SameDepth struct {
	SameDepthUntagged
	SameDepthTagged
}

type SameDepthRenamed struct {
	SameDepthUntagged
	Other struct {
		Name string `json:"Name"`
	}
}

type Conflicts struct {
	Base
	Mixin
}

type DepthWins struct {
	Base
	Wrapper
	Name string
}

type TagWins struct {
	Base
	Mixin
	Label string `json:"Name"`
}

type TwoTags struct {
	First  int `json:"x"`
	Second int `json:"x"`
}

type Skips struct {
	Hidden  int `json:"-"`
	Dash    int `json:"-,"`
	private int
	Plain   int
	Comma   int `json:",omitempty"`
	Empty   int `json:""`
	Space   int `json:"a b"`
	Weird   int `json:"x,y"`
	Punct   int `json:"!#$%&()*+-./:;<=>?@[]^_{|}~ "`
	Quote   int `json:"a\"b"`
	Bad     int `json:"a\\b"`
	Unicode int `json:"héllo"`
	Opts    int `json:",string,omitempty"`
	Unknown int `json:"unk,whatever"`
}

type Cases struct {
	Alpha int
	BETA  int
	Gamma int `json:"gAmMa"`
}

func main() {
	fmt.Println("== omitempty")
	show("zero", OmitAll{})
	number := 0
	show("filled", OmitAll{
		Bool: true, Int: 1, Uint: 2, Float: 0.5, String: "s", Slice: []int{}, SliceFull: []int{1},
		Map: map[string]int{}, MapEmpty: map[string]int{}, Pointer: &number, Interface: 0, Array: [2]int{0, 1},
		Struct: Pair{}, StructPtr: &Pair{}, Raw: json.RawMessage("null"), Number: "0", Bytes: []byte{}, Time: time.Unix(0, 0).UTC(),
	})
	show("empty non-nil", OmitAll{Slice: []int{}, Map: map[string]int{}, Interface: []int{}, Bytes: []byte{}})

	fmt.Println("== omitzero")
	show("zero", ZeroAll{})
	zeroNumber := 0
	show("filled", ZeroAll{
		Bool: true, Int: 1, Float: 0.5, NegZero: negativeZero(), String: "s", Slice: []int{}, SliceEmpty: []int{},
		Map: map[string]int{}, MapEmpty: map[string]int{}, Pointer: &zeroNumber, Interface: 0, Array: [2]int{0, 1},
		Struct: Pair{A: 1}, StructPtr: &Pair{}, Time: time.Unix(0, 0).UTC(), Zeroer: Zeroer{Value: 42}, ZeroerPtr: &Zeroer{Value: 42},
		PtrZeroer: PtrZeroer{Value: 42}, Both: []int{}, Order: []int{},
	})
	show("zeroer values", ZeroAll{Zeroer: Zeroer{Value: 1}, ZeroerPtr: &Zeroer{Value: 0}, PtrZeroer: PtrZeroer{Value: 1}})
	show("by pointer", &ZeroAll{PtrZeroer: PtrZeroer{Value: 42}})

	fmt.Println("== embedding")
	show("conflicts", Conflicts{Base: Base{ID: 1, Name: "b", Shared: "s1"}, Mixin: Mixin{Name: "m", Extra: 2, Shared: "s2"}})
	show("depth wins", DepthWins{Base: Base{ID: 1, Name: "b"}, Wrapper: Wrapper{Deep{Shared: "d", Level: 3}}, Name: "top"})
	show("tag wins", TagWins{Base: Base{Name: "b"}, Mixin: Mixin{Name: "m"}, Label: "l"})
	show("same depth, tag wins", SameDepth{SameDepthUntagged{"untagged"}, SameDepthTagged{"tagged"}})
	show("same depth, tag wins ptr", &SameDepth{SameDepthUntagged{"untagged"}, SameDepthTagged{"tagged"}})
	show("same depth, other field", SameDepthRenamed{SameDepthUntagged{"untagged"}, struct {
		Name string `json:"Name"`
	}{"other"}})
	show("tagged embed", TaggedEmbed{Base: Base{ID: 5}, Top: 1})
	show("pointer embed nil", PointerEmbed{Other: 1})
	show("pointer embed", PointerEmbed{Base: &Base{ID: 5, Name: "n"}, Other: 1})
	show("unexported embed", UnexportedEmbed{unexportedBase{Visible: 1, hidden: 2}, 3})
	show("two tags", TwoTags{1, 2})
	show("skips", Skips{Hidden: 1, Dash: 2, private: 3, Plain: 4, Comma: 5, Empty: 6, Space: 7, Weird: 8, Punct: 9, Quote: 10, Bad: 11, Unicode: 12, Opts: 0, Unknown: 14})

	for _, input := range []string{
		`{"ID":1,"Name":"n","Shared":"s","Extra":2}`,
		`{"id":1,"NAME":"n","shared":"s","extra":2}`,
		`{"Name":"n","Shared":"s"}`,
	} {
		var conflicts Conflicts
		err := json.Unmarshal([]byte(input), &conflicts)
		fmt.Printf("conflicts %-48s => %+v %v\n", input, conflicts, err)
	}
	var sameDepth SameDepth
	err := json.Unmarshal([]byte(`{"Name":"decoded"}`), &sameDepth)
	fmt.Printf("same depth decode => %+v %v\n", sameDepth, err)
	for _, input := range []string{`{"ID":1,"Name":"n"}`, `{"base":{"ID":2},"Top":3}`, `{"BASE":{"ID":2}}`} {
		var tagged TaggedEmbed
		err := json.Unmarshal([]byte(input), &tagged)
		fmt.Printf("tagged %-30s => %+v %v\n", input, tagged, err)
	}
	for _, input := range []string{`{"ID":1,"Other":2}`, `{"Other":2}`, `{}`} {
		var pointer PointerEmbed
		err := json.Unmarshal([]byte(input), &pointer)
		fmt.Printf("pointer embed %-22s => base=%+v other=%d %v\n", input, pointer.Base, pointer.Other, err)
	}
	for _, input := range []string{`{"Visible":1,"Own":2}`, `{"hidden":1}`, `{"visible":4}`} {
		var unexported UnexportedEmbed
		err := json.Unmarshal([]byte(input), &unexported)
		fmt.Printf("unexported embed %-26s => %+v %v\n", input, unexported, err)
	}
	var twoTags TwoTags
	err = json.Unmarshal([]byte(`{"x":1}`), &twoTags)
	fmt.Printf("two tags => %+v %v\n", twoTags, err)

	fmt.Println("== case")
	for _, input := range []string{`{"alpha":1}`, `{"ALPHA":1}`, `{"beta":2}`, `{"Beta":2,"BETA":3}`, `{"gamma":4}`, `{"GAMMA":5}`, `{"gAmMa":6}`, `{"Alpha":1,"alpha":2}`} {
		var value Cases
		err := json.Unmarshal([]byte(input), &value)
		fmt.Printf("%-24s => %+v %v\n", input, value, err)
	}

	fmt.Println("== decode skips")
	var skips Skips
	err = json.Unmarshal([]byte(`{"-":1,"Hidden":2,"private":3,"Plain":4,"a b":5,"x":6,"a\"b":7,"héllo":8,"HÉLLO":9,"unk":10}`), &skips)
	fmt.Printf("%+v %v\n", skips, err)
}

func negativeZero() float64 {
	zero := 0.0
	return -zero * 1
}

// Output:
// == omitempty
// zero: {"array":[0,0],"struct":{"A":0,"B":0},"time":"0001-01-01T00:00:00Z"} <nil>
// filled: {"bool":true,"int":1,"uint":2,"float":0.5,"string":"s","sliceFull":[1],"pointer":0,"interface":0,"array":[0,1],"struct":{"A":0,"B":0},"structPtr":{"A":0,"B":0},"raw":null,"number":0,"time":"1970-01-01T00:00:00Z"} <nil>
// empty non-nil: {"interface":[],"array":[0,0],"struct":{"A":0,"B":0},"time":"0001-01-01T00:00:00Z"} <nil>
// == omitzero
// zero: {"zeroer":{"Value":0},"ptrZeroer":{"Value":0}} <nil>
// filled: {"bool":true,"int":1,"float":0.5,"string":"s","slice":[],"sliceEmpty":[],"map":{},"mapEmpty":{},"pointer":0,"interface":0,"array":[0,1],"struct":{"A":1,"B":0},"structPtr":{"A":0,"B":0},"time":"1970-01-01T00:00:00Z"} <nil>
// zeroer values: {"zeroer":{"Value":1},"zeroerPtr":{"Value":0},"ptrZeroer":{"Value":1}} <nil>
// by pointer: {"zeroer":{"Value":0}} <nil>
// == embedding
// conflicts: {"ID":1,"Extra":2} <nil>
// depth wins: {"ID":1,"Shared":"","deep":3,"Name":"top"} <nil>
// tag wins: {"ID":0,"Extra":0,"Name":"l"} <nil>
// same depth, tag wins: {"Name":"tagged"} <nil>
// same depth, tag wins ptr: {"Name":"tagged"} <nil>
// same depth, other field: {"Name":"untagged","Other":{"Name":"other"}} <nil>
// tagged embed: {"base":{"ID":5,"Name":"","Shared":""},"Top":1} <nil>
// pointer embed nil: {"Other":1} <nil>
// pointer embed: {"ID":5,"Name":"n","Shared":"","Other":1} <nil>
// unexported embed: {"Visible":1,"Own":3} <nil>
// two tags: {} <nil>
// skips: {"-":2,"Plain":4,"Comma":5,"Empty":6,"a b":7,"x":8,"!#$%\u0026()*+-./:;\u003c=\u003e?@[]^_{|}~ ":9,"Quote":10,"Bad":11,"héllo":12,"unk":14} <nil>
// conflicts {"ID":1,"Name":"n","Shared":"s","Extra":2}       => {Base:{ID:1 Name: Shared:} Mixin:{Name: Extra:2 Shared:}} <nil>
// conflicts {"id":1,"NAME":"n","shared":"s","extra":2}       => {Base:{ID:1 Name: Shared:} Mixin:{Name: Extra:2 Shared:}} <nil>
// conflicts {"Name":"n","Shared":"s"}                        => {Base:{ID:0 Name: Shared:} Mixin:{Name: Extra:0 Shared:}} <nil>
// same depth decode => {SameDepthUntagged:{Name:} SameDepthTagged:{Name:decoded}} <nil>
// tagged {"ID":1,"Name":"n"}            => {Base:{ID:0 Name: Shared:} Top:0} <nil>
// tagged {"base":{"ID":2},"Top":3}      => {Base:{ID:2 Name: Shared:} Top:3} <nil>
// tagged {"BASE":{"ID":2}}              => {Base:{ID:2 Name: Shared:} Top:0} <nil>
// pointer embed {"ID":1,"Other":2}     => base=&{ID:1 Name: Shared:} other=2 <nil>
// pointer embed {"Other":2}            => base=<nil> other=2 <nil>
// pointer embed {}                     => base=<nil> other=0 <nil>
// unexported embed {"Visible":1,"Own":2}      => {unexportedBase:{Visible:1 hidden:0} Own:2} <nil>
// unexported embed {"hidden":1}               => {unexportedBase:{Visible:0 hidden:0} Own:0} <nil>
// unexported embed {"visible":4}              => {unexportedBase:{Visible:4 hidden:0} Own:0} <nil>
// two tags => {First:0 Second:0} <nil>
// == case
// {"alpha":1}              => {Alpha:1 BETA:0 Gamma:0} <nil>
// {"ALPHA":1}              => {Alpha:1 BETA:0 Gamma:0} <nil>
// {"beta":2}               => {Alpha:0 BETA:2 Gamma:0} <nil>
// {"Beta":2,"BETA":3}      => {Alpha:0 BETA:3 Gamma:0} <nil>
// {"gamma":4}              => {Alpha:0 BETA:0 Gamma:4} <nil>
// {"GAMMA":5}              => {Alpha:0 BETA:0 Gamma:5} <nil>
// {"gAmMa":6}              => {Alpha:0 BETA:0 Gamma:6} <nil>
// {"Alpha":1,"alpha":2}    => {Alpha:2 BETA:0 Gamma:0} <nil>
// == decode skips
// {Hidden:0 Dash:1 private:0 Plain:4 Comma:0 Empty:0 Space:5 Weird:6 Punct:0 Quote:0 Bad:0 Unicode:9 Opts:0 Unknown:10} <nil>
