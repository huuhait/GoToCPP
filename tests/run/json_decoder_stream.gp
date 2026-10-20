// phase: 09
// encoding/json Decoder: several values in a stream, Token API, More, Buffered, InputOffset, Decode of array elements one by one, readers that return one byte at a time.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Msg struct {
	Name string
	Text string
}

type oneByte struct{ r io.Reader }

func (o oneByte) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return o.r.Read(p[:1])
}

func tokens(label, in string, useNumber bool) {
	dec := json.NewDecoder(strings.NewReader(in))
	if useNumber {
		dec.UseNumber()
	}
	fmt.Printf("--- %s\n", label)
	for {
		t, err := dec.Token()
		if err != nil {
			fmt.Printf("  err: %v (EOF=%v) offset=%d\n", err, err == io.EOF, dec.InputOffset())
			return
		}
		fmt.Printf("  %T %v more=%v offset=%d\n", t, t, dec.More(), dec.InputOffset())
	}
}

func main() {
	const stream = `
		{"Name": "Ed", "Text": "Knock knock."}
		{"Name": "Sam", "Text": "Who's there?"}
		{"Name": "Ed", "Text": "Go fmt."}
		{"Name": "Sam", "Text": "Go fmt who?"}
		{"Name": "Ed", "Text": "Go fmt yourself!"}
	`
	dec := json.NewDecoder(strings.NewReader(stream))
	for {
		var m Msg
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			fmt.Println("error:", err)
			break
		}
		fmt.Printf("%s: %s (offset %d)\n", m.Name, m.Text, dec.InputOffset())
	}

	// Mixed values and one byte reads.
	dec = json.NewDecoder(oneByte{strings.NewReader(`1 "two" [3] {"four":4} null true 5.5e1 "é"`)})
	for i := 0; ; i++ {
		var v any
		err := dec.Decode(&v)
		if err != nil {
			fmt.Println("end:", err)
			break
		}
		fmt.Printf("%d: %T %v\n", i, v, v)
	}

	// Values without whitespace between them.
	dec = json.NewDecoder(strings.NewReader(`[1][2]{"a":1}{"b":2}"x""y"12 34`))
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			fmt.Println("end:", err)
			break
		}
		fmt.Println(v)
	}

	// Streaming an array element by element.
	dec = json.NewDecoder(strings.NewReader(`[ {"Name":"a","Text":"1"}, {"Name":"b","Text":"2"}, {"Name":"c","Text":"3"} ]`))
	t, err := dec.Token()
	fmt.Printf("%T: %v %v\n", t, t, err)
	for dec.More() {
		var m Msg
		if err := dec.Decode(&m); err != nil {
			fmt.Println(err)
			break
		}
		fmt.Printf("  %v\n", m)
	}
	t, err = dec.Token()
	fmt.Printf("%T: %v %v\n", t, t, err)
	t, err = dec.Token()
	fmt.Printf("%v %v\n", t, err)

	tokens("object", `{"a": [1, 2.5, "x", true, false, null], "b": {"c": {}}, "d": []}`, false)
	tokens("use number", `[1, 2.5, 1e3]`, true)
	tokens("scalar", `"just a string"`, false)
	tokens("several", `1 2 [3]`, false)
	tokens("bad", `{"a" 1}`, false)
	tokens("bad close", `[1}`, false)
	tokens("trailing comma", `[1,]`, false)
	tokens("truncated", `{"a":`, false)
	tokens("key not string", `{1:2}`, false)
	tokens("empty", ``, false)
	tokens("missing colon", `{"a","b"}`, false)
	tokens("leading comma", `[,1]`, false)
	tokens("colon in array", `[1:2]`, false)
	tokens("comma after open object", `{,}`, false)

	// Buffered returns what the decoder read ahead but did not use.
	dec = json.NewDecoder(strings.NewReader(`{"a":1} trailing data here`))
	var v map[string]int
	fmt.Println(dec.Decode(&v), v)
	rest, _ := io.ReadAll(dec.Buffered())
	fmt.Printf("buffered: %q\n", rest)

	// Decoding into a struct after tokens.
	dec = json.NewDecoder(strings.NewReader(`{"k": {"Name":"n","Text":"t"}, "z": 1}`))
	dec.Token()
	key, _ := dec.Token()
	var m Msg
	fmt.Println(key, dec.Decode(&m), m)
	key, _ = dec.Token()
	var z int
	fmt.Println(key, dec.Decode(&z), z)
	t, err = dec.Token()
	fmt.Println(t, err)

	// Decode errors do not stop the stream when the syntax is fine.
	dec = json.NewDecoder(strings.NewReader(`{"Name": 1} {"Name": "ok"}`))
	for i := 0; i < 3; i++ {
		var m Msg
		err := dec.Decode(&m)
		var ute *json.UnmarshalTypeError
		fmt.Printf("%d: %+v %v %v\n", i, m, err, errors.As(err, &ute))
	}

	// A syntax error is sticky.
	dec = json.NewDecoder(strings.NewReader(`[1,2 3] [4]`))
	var arr []int
	err1 := dec.Decode(&arr)
	err2 := dec.Decode(&arr)
	fmt.Println(err1, "|", err2, err1 == err2)

	// Large value split over reads.
	var big bytes.Buffer
	big.WriteString("[")
	for i := 0; i < 5000; i++ {
		if i > 0 {
			big.WriteString(",")
		}
		fmt.Fprintf(&big, `{"Name":"n%d","Text":"%s"}`, i, strings.Repeat("x", i%50))
	}
	big.WriteString("]")
	var msgs []Msg
	fmt.Println(json.NewDecoder(oneByte{&big}).Decode(&msgs), len(msgs), msgs[4999].Name, len(msgs[4999].Text))
}

// Output:
// Ed: Knock knock. (offset 41)
// Sam: Who's there? (offset 83)
// Ed: Go fmt. (offset 119)
// Sam: Go fmt who? (offset 160)
// Ed: Go fmt yourself! (offset 205)
// 0: float64 1
// 1: string two
// 2: []interface {} [3]
// 3: map[string]interface {} map[four:4]
// 4: <nil> <nil>
// 5: bool true
// 6: float64 55
// 7: string é
// end: EOF
// [1]
// [2]
// map[a:1]
// map[b:2]
// x
// y
// 12
// 34
// end: EOF
// json.Delim: [ <nil>
//   {a 1}
//   {b 2}
//   {c 3}
// json.Delim: ] <nil>
// <nil> EOF
// --- object
//   json.Delim { more=true offset=1
//   string a more=true offset=4
//   json.Delim [ more=true offset=7
//   float64 1 more=true offset=8
//   float64 2.5 more=true offset=13
//   string x more=true offset=18
//   bool true more=true offset=24
//   bool false more=true offset=31
//   <nil> <nil> more=false offset=37
//   json.Delim ] more=true offset=38
//   string b more=true offset=43
//   json.Delim { more=true offset=46
//   string c more=true offset=49
//   json.Delim { more=false offset=52
//   json.Delim } more=false offset=53
//   json.Delim } more=true offset=54
//   string d more=true offset=59
//   json.Delim [ more=false offset=62
//   json.Delim ] more=false offset=63
//   json.Delim } more=false offset=64
//   err: EOF (EOF=true) offset=64
// --- use number
//   json.Delim [ more=true offset=1
//   json.Number 1 more=true offset=2
//   json.Number 2.5 more=true offset=7
//   json.Number 1e3 more=false offset=12
//   json.Delim ] more=false offset=13
//   err: EOF (EOF=true) offset=13
// --- scalar
//   string just a string more=false offset=15
//   err: EOF (EOF=true) offset=15
// --- several
//   float64 1 more=true offset=2
//   float64 2 more=true offset=4
//   json.Delim [ more=true offset=5
//   float64 3 more=false offset=6
//   json.Delim ] more=false offset=7
//   err: EOF (EOF=true) offset=7
// --- bad
//   json.Delim { more=true offset=1
//   string a more=true offset=5
//   err: invalid character '1' after object key (EOF=false) offset=5
// --- bad close
//   json.Delim [ more=true offset=1
//   float64 1 more=false offset=2
//   err: invalid character '}' after array element (EOF=false) offset=2
// --- trailing comma
//   json.Delim [ more=true offset=1
//   float64 1 more=true offset=2
//   err: invalid character ']' looking for beginning of value (EOF=false) offset=3
// --- truncated
//   json.Delim { more=true offset=1
//   string a more=true offset=4
//   err: EOF (EOF=true) offset=5
// --- key not string
//   json.Delim { more=true offset=1
//   err: invalid character '1' (EOF=false) offset=1
// --- empty
//   err: EOF (EOF=true) offset=0
// --- missing colon
//   json.Delim { more=true offset=1
//   string a more=true offset=4
//   err: invalid character ',' after object key (EOF=false) offset=4
// --- leading comma
//   json.Delim [ more=true offset=1
//   err: invalid character ',' looking for beginning of value (EOF=false) offset=1
// --- colon in array
//   json.Delim [ more=true offset=1
//   float64 1 more=true offset=2
//   err: invalid character ':' after array element (EOF=false) offset=2
// --- comma after open object
//   json.Delim { more=true offset=1
//   err: invalid character ',' (EOF=false) offset=1
// <nil> map[a:1]
// buffered: " trailing data here"
// k <nil> {n t}
// z <nil> 1
// } <nil>
// 0: {Name: Text:} json: cannot unmarshal number into Go struct field Msg.Name of type string true
// 1: {Name:ok Text:} <nil> false
// 2: {Name: Text:} EOF false
// invalid character '3' after array element | invalid character '3' after array element true
// <nil> 5000 n4999 49
