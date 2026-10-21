// phase: 09
// encoding/json Decoder and Encoder options: DisallowUnknownFields (nested, embedded, maps,
// case-insensitive), UseNumber, SetIndent, SetEscapeHTML, Buffered, InputOffset, More, Token.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var address = regexp.MustCompile(`0x[0-9a-f]{8,}`)

type Leaf struct{ X int }

type Core struct{ Shared int }

type Config struct {
	Core
	Name  string
	Leaf  Leaf
	Ptr   *Leaf
	List  []Leaf
	Map   map[string]Leaf
	Any   any
	Raw   json.RawMessage
	Plain map[string]int
	hide  int
}

func decodeStrict(input string) {
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.DisallowUnknownFields()
	var config Config
	err := decoder.Decode(&config)
	fmt.Printf("%-46s => %s err=%v\n", input, address.ReplaceAllString(fmt.Sprintf("%+v", config), "0xADDR"), err)
}

func tokens(input string) {
	decoder := json.NewDecoder(strings.NewReader(input))
	fmt.Printf("-- %q\n", input)
	for {
		offsetBefore := decoder.InputOffset()
		token, err := decoder.Token()
		if err != nil {
			var syntax *json.SyntaxError
			if errors.As(err, &syntax) {
				fmt.Printf("  error syntax offset=%d %q (input offset %d)\n", syntax.Offset, err, decoder.InputOffset())
			} else {
				fmt.Printf("  error %T %v (input offset %d)\n", err, err, decoder.InputOffset())
			}
			return
		}
		fmt.Printf("  %-12T %-10v offset %d->%d more=%v\n", token, token, offsetBefore, decoder.InputOffset(), decoder.More())
	}
}

func main() {
	fmt.Println("== DisallowUnknownFields")
	decodeStrict(`{"Name":"a","Shared":1}`)
	decodeStrict(`{"Name":"a","extra":1}`)
	decodeStrict(`{"name":"a","SHARED":2}`)
	decodeStrict(`{"Leaf":{"X":1,"Y":2}}`)
	decodeStrict(`{"Ptr":{"Z":2}}`)
	decodeStrict(`{"List":[{"X":1},{"X":2,"W":3}]}`)
	decodeStrict(`{"Map":{"k":{"X":1,"V":1}}}`)
	decodeStrict(`{"Plain":{"anything":1}}`)
	decodeStrict(`{"Any":{"anything":1}}`)
	decodeStrict(`{"Raw":{"anything":1}}`)
	decodeStrict(`{"hide":1}`)
	decodeStrict(`{"Name":1,"extra":2}`)
	decodeStrict(`{"extra":2,"Name":1}`)
	decodeStrict(`{"Core":{"Shared":1}}`)
	decodeStrict(`[1]`)
	var loose Config
	err := json.Unmarshal([]byte(`{"extra":1,"Name":"n"}`), &loose)
	fmt.Println("plain Unmarshal ignores:", loose.Name, err)
	decoder := json.NewDecoder(strings.NewReader(`{"extra":1} {"Name":"second"} {"Name":"third","extra":3}`))
	decoder.DisallowUnknownFields()
	for {
		var config Config
		err := decoder.Decode(&config)
		fmt.Printf("stream: %q %v\n", config.Name, err)
		if err != nil {
			if err == io.EOF {
				break
			}
			if _, isSyntax := err.(*json.SyntaxError); isSyntax {
				break
			}
		}
	}

	fmt.Println("== UseNumber")
	decoder = json.NewDecoder(strings.NewReader(`{"a":1,"b":[2.50,"x",{"c":3e2}],"d":12345678901234567890} 4 [5]`))
	decoder.UseNumber()
	for {
		var value any
		err := decoder.Decode(&value)
		if err != nil {
			fmt.Println("end:", err)
			break
		}
		fmt.Printf("%v %T\n", value, value)
		if object, ok := value.(map[string]any); ok {
			fmt.Printf("  a is %T, d is %T\n", object["a"], object["d"])
		}
	}
	var typed struct {
		N json.Number
		F float64
		I any
		S []any
	}
	decoder = json.NewDecoder(strings.NewReader(`{"N":1.5,"F":2.5,"I":3,"S":[4]}`))
	decoder.UseNumber()
	err = decoder.Decode(&typed)
	fmt.Printf("%+v %v\n", typed, err)
	for _, input := range []string{`{"N":"1.5"}`, `{"N":"abc"}`, `{"N":true}`, `{"N":null}`, `{"N":[1]}`} {
		var number struct{ N json.Number }
		err := json.Unmarshal([]byte(input), &number)
		fmt.Printf("%-16s => %q %v\n", input, number.N, err)
	}

	fmt.Println("== Encoder options")
	value := map[string]any{"html": "<a href=\"x\">&</a>", "list": []int{1, 2}, "empty": []int{}, "obj": map[string]int{}, "nested": map[string]any{"k": []any{}}}
	for _, escape := range []bool{true, false} {
		for _, indent := range []struct{ prefix, indent string }{{"", ""}, {"", "  "}, {"#", "\t"}, {"# ", ""}} {
			var buffer bytes.Buffer
			encoder := json.NewEncoder(&buffer)
			encoder.SetEscapeHTML(escape)
			encoder.SetIndent(indent.prefix, indent.indent)
			err := encoder.Encode(value)
			err2 := encoder.Encode("second<")
			fmt.Printf("escape=%v indent=%q,%q: %q %v %v\n", escape, indent.prefix, indent.indent, buffer.String(), err, err2)
		}
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", " ")
	_ = encoder.Encode(1)
	encoder.SetIndent("", "")
	_ = encoder.Encode([]int{1})
	encoder.SetIndent("p", "")
	_ = encoder.Encode([]int{1})
	fmt.Printf("%q\n", buffer.String())
	buffer.Reset()
	encoder = json.NewEncoder(&buffer)
	err = encoder.Encode(map[string]any{"bad": make(chan int)})
	fmt.Printf("failed encode wrote %q err=%v\n", buffer.String(), err)
	err = encoder.Encode([]any{1, func() {}})
	fmt.Printf("failed encode wrote %q err=%v\n", buffer.String(), err)

	fmt.Println("== Buffered and InputOffset")
	decoder = json.NewDecoder(strings.NewReader(`{"a":1}   {"b":2}trailing`))
	var first map[string]int
	err = decoder.Decode(&first)
	rest, _ := io.ReadAll(decoder.Buffered())
	fmt.Printf("%v %v rest=%q offset=%d more=%v\n", first, err, rest, decoder.InputOffset(), decoder.More())
	var second map[string]int
	err = decoder.Decode(&second)
	rest, _ = io.ReadAll(decoder.Buffered())
	fmt.Printf("%v %v rest=%q offset=%d more=%v\n", second, err, rest, decoder.InputOffset(), decoder.More())
	var third map[string]int
	err = decoder.Decode(&third)
	fmt.Printf("%v %v offset=%d\n", third, err, decoder.InputOffset())

	fmt.Println("== Token")
	tokens(`{"a":[1,"s",true,null,{"b":-2.5}],"c":{}}`)
	tokens(`[]`)
	tokens(`[[],{}]`)
	tokens(`"just a string"`)
	tokens(`  42  `)
	tokens(`null true`)
	tokens(`{"a" 1}`)
	tokens(`{"a":1,}`)
	tokens(`[1,]`)
	tokens(`[1 2]`)
	tokens(`{1:2}`)
	tokens(`{"a":}`)
	tokens(`]`)
	tokens(`}`)
	tokens(`[1`)
	tokens(`{"a":1`)
	tokens(`{"a"`)
	tokens(`,`)
	tokens(`:`)
	tokens(`{"a":1}}`)
	tokens(`[1]]`)
	tokens(`{"a":1,"a":[2]}`)

	fmt.Println("== Token mixed with Decode")
	decoder = json.NewDecoder(strings.NewReader(`[{"n":1},{"n":2},{"n":3}] {"after":true}`))
	token, err := decoder.Token()
	fmt.Printf("%v %v\n", token, err)
	for decoder.More() {
		var item struct{ N int }
		err := decoder.Decode(&item)
		fmt.Printf("item %+v %v\n", item, err)
	}
	token, err = decoder.Token()
	fmt.Printf("%v %v\n", token, err)
	var after map[string]bool
	err = decoder.Decode(&after)
	fmt.Printf("%v %v\n", after, err)
	decoder = json.NewDecoder(strings.NewReader(`{"k":"v","w":[1,2]}`))
	_, _ = decoder.Token()
	key, _ := decoder.Token()
	var text string
	err = decoder.Decode(&text)
	fmt.Printf("%v %q %v\n", key, text, err)
	key, _ = decoder.Token()
	var numbers []int
	err = decoder.Decode(&numbers)
	fmt.Printf("%v %v %v more=%v\n", key, numbers, err, decoder.More())
	closing, err := decoder.Token()
	fmt.Printf("%v %v\n", closing, err)
	_, err = decoder.Token()
	fmt.Println(err)
}

// Output:
// == DisallowUnknownFields
// {"Name":"a","Shared":1}                        => {Core:{Shared:1} Name:a Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=<nil>
// {"Name":"a","extra":1}                         => {Core:{Shared:0} Name:a Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: unknown field "extra"
// {"name":"a","SHARED":2}                        => {Core:{Shared:2} Name:a Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=<nil>
// {"Leaf":{"X":1,"Y":2}}                         => {Core:{Shared:0} Name: Leaf:{X:1} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: unknown field "Y"
// {"Ptr":{"Z":2}}                                => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:0xADDR List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: unknown field "Z"
// {"List":[{"X":1},{"X":2,"W":3}]}               => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[{X:1} {X:2}] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: unknown field "W"
// {"Map":{"k":{"X":1,"V":1}}}                    => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[k:{X:1}] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: unknown field "V"
// {"Plain":{"anything":1}}                       => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[anything:1] hide:0} err=<nil>
// {"Any":{"anything":1}}                         => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:map[anything:1] Raw:[] Plain:map[] hide:0} err=<nil>
// {"Raw":{"anything":1}}                         => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[123 34 97 110 121 116 104 105 110 103 34 58 49 125] Plain:map[] hide:0} err=<nil>
// {"hide":1}                                     => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: unknown field "hide"
// {"Name":1,"extra":2}                           => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: cannot unmarshal number into Go struct field Config.Name of type string
// {"extra":2,"Name":1}                           => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: unknown field "extra"
// {"Core":{"Shared":1}}                          => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: unknown field "Core"
// [1]                                            => {Core:{Shared:0} Name: Leaf:{X:0} Ptr:<nil> List:[] Map:map[] Any:<nil> Raw:[] Plain:map[] hide:0} err=json: cannot unmarshal array into Go value of type main.Config
// plain Unmarshal ignores: n <nil>
// stream: "" json: unknown field "extra"
// stream: "second" <nil>
// stream: "third" json: unknown field "extra"
// stream: "" EOF
// == UseNumber
// map[a:1 b:[2.50 x map[c:3e2]] d:12345678901234567890] map[string]interface {}
//   a is json.Number, d is json.Number
// 4 json.Number
// [5] []interface {}
// end: EOF
// {N:1.5 F:2.5 I:3 S:[4]} <nil>
// {"N":"1.5"}      => "1.5" <nil>
// {"N":"abc"}      => "" json: invalid number literal, trying to unmarshal "\"abc\"" into Number
// {"N":true}       => "" json: cannot unmarshal bool into Go struct field .N of type json.Number
// {"N":null}       => "" <nil>
// {"N":[1]}        => "" json: cannot unmarshal array into Go struct field .N of type json.Number
// == Encoder options
// escape=true indent="","": "{\"empty\":[],\"html\":\"\\u003ca href=\\\"x\\\"\\u003e\\u0026\\u003c/a\\u003e\",\"list\":[1,2],\"nested\":{\"k\":[]},\"obj\":{}}\n\"second\\u003c\"\n" <nil> <nil>
// escape=true indent="","  ": "{\n  \"empty\": [],\n  \"html\": \"\\u003ca href=\\\"x\\\"\\u003e\\u0026\\u003c/a\\u003e\",\n  \"list\": [\n    1,\n    2\n  ],\n  \"nested\": {\n    \"k\": []\n  },\n  \"obj\": {}\n}\n\"second\\u003c\"\n" <nil> <nil>
// escape=true indent="#","\t": "{\n#\t\"empty\": [],\n#\t\"html\": \"\\u003ca href=\\\"x\\\"\\u003e\\u0026\\u003c/a\\u003e\",\n#\t\"list\": [\n#\t\t1,\n#\t\t2\n#\t],\n#\t\"nested\": {\n#\t\t\"k\": []\n#\t},\n#\t\"obj\": {}\n#}\n\"second\\u003c\"\n" <nil> <nil>
// escape=true indent="# ","": "{\n# \"empty\": [],\n# \"html\": \"\\u003ca href=\\\"x\\\"\\u003e\\u0026\\u003c/a\\u003e\",\n# \"list\": [\n# 1,\n# 2\n# ],\n# \"nested\": {\n# \"k\": []\n# },\n# \"obj\": {}\n# }\n\"second\\u003c\"\n" <nil> <nil>
// escape=false indent="","": "{\"empty\":[],\"html\":\"<a href=\\\"x\\\">&</a>\",\"list\":[1,2],\"nested\":{\"k\":[]},\"obj\":{}}\n\"second<\"\n" <nil> <nil>
// escape=false indent="","  ": "{\n  \"empty\": [],\n  \"html\": \"<a href=\\\"x\\\">&</a>\",\n  \"list\": [\n    1,\n    2\n  ],\n  \"nested\": {\n    \"k\": []\n  },\n  \"obj\": {}\n}\n\"second<\"\n" <nil> <nil>
// escape=false indent="#","\t": "{\n#\t\"empty\": [],\n#\t\"html\": \"<a href=\\\"x\\\">&</a>\",\n#\t\"list\": [\n#\t\t1,\n#\t\t2\n#\t],\n#\t\"nested\": {\n#\t\t\"k\": []\n#\t},\n#\t\"obj\": {}\n#}\n\"second<\"\n" <nil> <nil>
// escape=false indent="# ","": "{\n# \"empty\": [],\n# \"html\": \"<a href=\\\"x\\\">&</a>\",\n# \"list\": [\n# 1,\n# 2\n# ],\n# \"nested\": {\n# \"k\": []\n# },\n# \"obj\": {}\n# }\n\"second<\"\n" <nil> <nil>
// "1\n[1]\n[\np1\np]\n"
// failed encode wrote "" err=json: unsupported type: chan int
// failed encode wrote "" err=json: unsupported type: func()
// == Buffered and InputOffset
// map[a:1] <nil> rest="   {\"b\":2}trailing" offset=7 more=true
// map[b:2] <nil> rest="trailing" offset=17 more=true
// map[] invalid character 'a' in literal true (expecting 'u') offset=17
// == Token
// -- "{\"a\":[1,\"s\",true,null,{\"b\":-2.5}],\"c\":{}}"
//   json.Delim   {          offset 0->1 more=true
//   string       a          offset 1->4 more=true
//   json.Delim   [          offset 4->6 more=true
//   float64      1          offset 6->7 more=true
//   string       s          offset 7->11 more=true
//   bool         true       offset 11->16 more=true
//   <nil>        <nil>      offset 16->21 more=true
//   json.Delim   {          offset 21->23 more=true
//   string       b          offset 23->26 more=true
//   float64      -2.5       offset 26->31 more=false
//   json.Delim   }          offset 31->32 more=false
//   json.Delim   ]          offset 32->33 more=true
//   string       c          offset 33->37 more=true
//   json.Delim   {          offset 37->39 more=false
//   json.Delim   }          offset 39->40 more=false
//   json.Delim   }          offset 40->41 more=false
//   error *errors.errorString EOF (input offset 41)
// -- "[]"
//   json.Delim   [          offset 0->1 more=false
//   json.Delim   ]          offset 1->2 more=false
//   error *errors.errorString EOF (input offset 2)
// -- "[[],{}]"
//   json.Delim   [          offset 0->1 more=true
//   json.Delim   [          offset 1->2 more=false
//   json.Delim   ]          offset 2->3 more=true
//   json.Delim   {          offset 3->5 more=false
//   json.Delim   }          offset 5->6 more=false
//   json.Delim   ]          offset 6->7 more=false
//   error *errors.errorString EOF (input offset 7)
// -- "\"just a string\""
//   string       just a string offset 0->15 more=false
//   error *errors.errorString EOF (input offset 15)
// -- "  42  "
//   float64      42         offset 0->4 more=false
//   error *errors.errorString EOF (input offset 4)
// -- "null true"
//   <nil>        <nil>      offset 0->4 more=true
//   bool         true       offset 5->9 more=false
//   error *errors.errorString EOF (input offset 9)
// -- "{\"a\" 1}"
//   json.Delim   {          offset 0->1 more=true
//   string       a          offset 1->4 more=true
//   error syntax offset=5 "invalid character '1' after object key" (input offset 5)
// -- "{\"a\":1,}"
//   json.Delim   {          offset 0->1 more=true
//   string       a          offset 1->4 more=true
//   float64      1          offset 4->6 more=true
//   error syntax offset=7 "invalid character '}' looking for beginning of object key string" (input offset 7)
// -- "[1,]"
//   json.Delim   [          offset 0->1 more=true
//   float64      1          offset 1->2 more=true
//   error syntax offset=3 "invalid character ']' looking for beginning of value" (input offset 3)
// -- "[1 2]"
//   json.Delim   [          offset 0->1 more=true
//   float64      1          offset 1->2 more=true
//   error syntax offset=3 "invalid character '2' after array element" (input offset 3)
// -- "{1:2}"
//   json.Delim   {          offset 0->1 more=true
//   error syntax offset=1 "invalid character '1'" (input offset 1)
// -- "{\"a\":}"
//   json.Delim   {          offset 0->1 more=true
//   string       a          offset 1->4 more=true
//   error syntax offset=5 "invalid character '}' looking for beginning of value" (input offset 5)
// -- "]"
//   error syntax offset=0 "invalid character ']' looking for beginning of value" (input offset 0)
// -- "}"
//   error syntax offset=0 "invalid character '}' looking for beginning of value" (input offset 0)
// -- "[1"
//   json.Delim   [          offset 0->1 more=true
//   float64      1          offset 1->2 more=false
//   error *errors.errorString EOF (input offset 2)
// -- "{\"a\":1"
//   json.Delim   {          offset 0->1 more=true
//   string       a          offset 1->4 more=true
//   float64      1          offset 4->6 more=false
//   error *errors.errorString EOF (input offset 6)
// -- "{\"a\""
//   json.Delim   {          offset 0->1 more=true
//   string       a          offset 1->4 more=false
//   error *errors.errorString EOF (input offset 4)
// -- ","
//   error syntax offset=0 "invalid character ',' looking for beginning of value" (input offset 0)
// -- ":"
//   error syntax offset=0 "invalid character ':' looking for beginning of value" (input offset 0)
// -- "{\"a\":1}}"
//   json.Delim   {          offset 0->1 more=true
//   string       a          offset 1->4 more=true
//   float64      1          offset 4->6 more=false
//   json.Delim   }          offset 6->7 more=false
//   error syntax offset=7 "invalid character '}' looking for beginning of value" (input offset 7)
// -- "[1]]"
//   json.Delim   [          offset 0->1 more=true
//   float64      1          offset 1->2 more=false
//   json.Delim   ]          offset 2->3 more=false
//   error syntax offset=3 "invalid character ']' looking for beginning of value" (input offset 3)
// -- "{\"a\":1,\"a\":[2]}"
//   json.Delim   {          offset 0->1 more=true
//   string       a          offset 1->4 more=true
//   float64      1          offset 4->6 more=true
//   string       a          offset 6->10 more=true
//   json.Delim   [          offset 10->12 more=true
//   float64      2          offset 12->13 more=false
//   json.Delim   ]          offset 13->14 more=false
//   json.Delim   }          offset 14->15 more=false
//   error *errors.errorString EOF (input offset 15)
// == Token mixed with Decode
// [ <nil>
// item {N:1} <nil>
// item {N:2} <nil>
// item {N:3} <nil>
// ] <nil>
// map[after:true] <nil>
// k "v" <nil>
// w [1 2] <nil> more=false
// } <nil>
// EOF
