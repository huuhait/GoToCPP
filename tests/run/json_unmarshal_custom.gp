// phase: 09
// encoding/json Unmarshaler and TextUnmarshaler: pointer receivers, null handling, fields, map keys and values, errors, RawMessage, embedded types, time.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Temp struct{ Deg float64 }

func (t *Temp) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		f, err := strconv.ParseFloat(strings.TrimSuffix(s, "C"), 64)
		if err != nil {
			return err
		}
		t.Deg = f
		return nil
	}
	return json.Unmarshal(b, &t.Deg)
}

type Rec struct {
	Raw []byte
}

func (r *Rec) UnmarshalJSON(b []byte) error {
	r.Raw = append([]byte(nil), b...)
	return nil
}

type Upper string

func (u *Upper) UnmarshalText(b []byte) error {
	*u = Upper(strings.ToUpper(string(b)))
	return nil
}

type Pair struct{ A, B string }

func (p *Pair) UnmarshalText(b []byte) error {
	a, bb, ok := strings.Cut(string(b), "+")
	if !ok {
		return errors.New("no plus in " + string(b))
	}
	p.A, p.B = a, bb
	return nil
}

type Both struct{ Via string }

func (b *Both) UnmarshalJSON([]byte) error { b.Via = "json"; return nil }
func (b *Both) UnmarshalText([]byte) error { b.Via = "text"; return nil }

type Failing struct{}

func (*Failing) UnmarshalJSON([]byte) error { return errors.New("always fails") }

type Holder struct {
	T   Temp
	TP  *Temp
	R   Rec
	U   Upper
	UP  *Upper
	P   Pair
	B   Both
	TS  []Temp
	TM  map[string]Temp
	Any any
}

type EmbT struct {
	Temp
	Name string
}

func try[T any](label string, in string, init T) {
	v := init
	err := json.Unmarshal([]byte(in), &v)
	fmt.Printf("%s: %+v err=%v\n", label, v, err)
}

func main() {
	var t Temp
	fmt.Println(json.Unmarshal([]byte(`"21.5C"`), &t), t)
	fmt.Println(json.Unmarshal([]byte(`30`), &t), t)
	fmt.Println(json.Unmarshal([]byte(`null`), &t), t)
	fmt.Println(json.Unmarshal([]byte(`"bad"`), &t), t)

	var h Holder
	err := json.Unmarshal([]byte(`{"T":"1C","TP":2,"R":{"x": [1, 2]},"U":"abc","UP":"def","P":"l+r","B":"x","TS":[3,"4C"],"TM":{"k":5},"Any":{"a":1}}`), &h)
	fmt.Printf("%v\n", err)
	fmt.Printf("T=%v TP=%v R=%s U=%s UP=%s P=%v B=%v TS=%v TM=%v Any=%v\n", h.T, *h.TP, h.R.Raw, h.U, *h.UP, h.P, h.B, h.TS, h.TM, h.Any)

	h = Holder{}
	err = json.Unmarshal([]byte(`{"T":null,"TP":null,"R":null,"U":null,"UP":null,"P":null,"B":null}`), &h)
	fmt.Printf("nulls: err=%v T=%v TP=%v R=%q U=%q UP=%v P=%v B=%v\n", err, h.T, h.TP, h.R.Raw, h.U, h.UP, h.P, h.B)

	h = Holder{}
	err = json.Unmarshal([]byte(`{"P":"nopl"}`), &h)
	fmt.Printf("text error: %v\n", err)
	var ute *json.UnmarshalTypeError
	fmt.Println(errors.As(err, &ute))
	err = json.Unmarshal([]byte(`{"P":5}`), &h)
	fmt.Printf("text wrong type: %v\n", err)
	err = json.Unmarshal([]byte(`{"U":["x"]}`), &h)
	fmt.Printf("text wrong type 2: %v\n", err)

	var f Failing
	fmt.Println(json.Unmarshal([]byte(`{}`), &f))
	var fs []Failing
	fmt.Println(json.Unmarshal([]byte(`[1,2]`), &fs), len(fs))

	var mk map[Upper]int
	fmt.Println(json.Unmarshal([]byte(`{"a":1,"b":2}`), &mk), mk)
	var mp map[Pair]int
	fmt.Println(json.Unmarshal([]byte(`{"a+b":1}`), &mp), mp)
	var mpb map[Pair]int
	fmt.Println(json.Unmarshal([]byte(`{"ab":1}`), &mpb), mpb)

	var e EmbT
	fmt.Println(json.Unmarshal([]byte(`"7C"`), &e), e)
	fmt.Println(json.Unmarshal([]byte(`{"Name":"n"}`), &e), e)

	var tm time.Time
	fmt.Println(json.Unmarshal([]byte(`"2024-02-29T13:14:15.5Z"`), &tm), tm.UTC())
	fmt.Println(json.Unmarshal([]byte(`"2024-02-29"`), &tm))
	fmt.Println(json.Unmarshal([]byte(`5`), &tm))
	var d time.Duration
	fmt.Println(json.Unmarshal([]byte(`1500`), &d), d)

	var raw json.RawMessage
	fmt.Println(json.Unmarshal([]byte(` {"a" : [1, 2 ] } `), &raw), string(raw))
	var rs struct {
		A json.RawMessage
		B *json.RawMessage
		C []json.RawMessage
		D map[string]json.RawMessage
	}
	fmt.Println(json.Unmarshal([]byte(`{"A":null,"B":"x","C":[1,{"y":2},[]],"D":{"k": true}}`), &rs))
	fmt.Printf("A=%s B=%s C=%s D=%s\n", rs.A, *rs.B, rs.C, rs.D)

	var it any = &Temp{}
	fmt.Println(json.Unmarshal([]byte(`"9C"`), it), it.(*Temp).Deg)
	var ip *Temp
	fmt.Println(json.Unmarshal([]byte(`10`), &ip), ip.Deg)
	var ipp **Temp
	fmt.Println(json.Unmarshal([]byte(`11`), &ipp), (*ipp).Deg)

	try("text into string-kind field via ,string", `{"V":"x"}`, struct {
		V Upper `json:",string"`
	}{})
}

// Output:
// <nil> {21.5}
// <nil> {30}
// strconv.ParseFloat: parsing "": invalid syntax {30}
// strconv.ParseFloat: parsing "bad": invalid syntax {30}
// <nil>
// T={1} TP={2} R={"x": [1, 2]} U=ABC UP=DEF P={l r} B={json} TS=[{3} {4}] TM=map[k:{5}] Any=map[a:1]
// nulls: err=strconv.ParseFloat: parsing "": invalid syntax T={0} TP=<nil> R="" U="" UP=<nil> P={ } B={}
// text error: no plus in nopl
// false
// text wrong type: json: cannot unmarshal number into Go struct field Holder.P of type main.Pair
// text wrong type 2: json: cannot unmarshal array into Go struct field Holder.U of type main.Upper
// always fails
// always fails 1
// <nil> map[A:1 B:2]
// <nil> map[{a b}:1]
// no plus in ab map[]
// <nil> {{7} }
// json: cannot unmarshal object into Go value of type float64 {{7} }
// <nil> 2024-02-29 13:14:15.5 +0000 UTC
// parsing time "2024-02-29" as "2006-01-02T15:04:05Z07:00": cannot parse "" as "T"
// Time.UnmarshalJSON: input is not a JSON string
// <nil> 1.5µs
// <nil> {"a" : [1, 2 ] }
// <nil>
// A=null B="x" C=[1 {"y":2} []] D=map[k:true]
// <nil> 9
// <nil> 10
// <nil> 11
// text into string-kind field via ,string: {V:} err=json: invalid use of ,string struct tag, trying to unmarshal "x" into main.Upper
