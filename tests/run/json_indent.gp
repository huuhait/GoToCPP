// phase: 09
// encoding/json Indent, Compact, MarshalIndent, HTMLEscape and Valid: prefixes, empty containers, nesting, errors, escaping.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Item struct {
	ID    int               `json:"id"`
	Name  string            `json:"name"`
	Tags  []string          `json:"tags"`
	Attrs map[string]any    `json:"attrs"`
	Sub   *Item             `json:"sub,omitempty"`
	Empty struct{}          `json:"empty"`
	ES    []int             `json:"es"`
	EM    map[string]string `json:"em"`
}

func indent(label, src, prefix, ind string) {
	var buf bytes.Buffer
	err := json.Indent(&buf, []byte(src), prefix, ind)
	fmt.Printf("--- %s (err=%v)\n%s|\n", label, err, buf.String())
}

func compact(label, src string) {
	var buf bytes.Buffer
	err := json.Compact(&buf, []byte(src))
	fmt.Printf("compact %s: %q err=%v\n", label, buf.String(), err)
}

func main() {
	it := Item{ID: 1, Name: "x<y", Tags: []string{"a", "b"}, Attrs: map[string]any{"z": 1, "a": []any{}, "m": map[string]any{}}, Sub: &Item{ID: 2}}
	for _, c := range [][2]string{{"", "  "}, {"", "\t"}, {">> ", "--"}, {"", ""}, {"#", ""}} {
		b, err := json.MarshalIndent(it, c[0], c[1])
		fmt.Printf("--- MarshalIndent prefix=%q indent=%q err=%v\n%s\n", c[0], c[1], err, b)
	}
	b, err := json.MarshalIndent([]int{}, "", " ")
	fmt.Printf("%s %v\n", b, err)
	b, err = json.MarshalIndent(map[string]int{}, "", " ")
	fmt.Printf("%s %v\n", b, err)
	b, err = json.MarshalIndent(nil, "", " ")
	fmt.Printf("%s %v\n", b, err)
	b, err = json.MarshalIndent("s", ">", " ")
	fmt.Printf("%s %v\n", b, err)
	b, err = json.MarshalIndent([]any{[]any{}, map[string]any{}, []any{1}}, "", "\t")
	fmt.Printf("%s %v\n", b, err)

	indent("object", `{"a":1,"b":[1,2,{"c":null}],"d":{}}`, "", "  ")
	indent("spaces in input", " { \"a\" : 1 ,\n\"b\" : [ ] , \"c\" : { } } ", "", "  ")
	indent("prefix", `[1,[2,3],{"a":[]}]`, "// ", "    ")
	indent("scalar", `  "str"  `, "", "  ")
	indent("number", `1.5e3`, "p", "  ")
	indent("trailing space kept before error", `[1,2`, "", "  ")
	indent("invalid", `{"a" 1}`, "", "  ")
	indent("empty", ``, "", "  ")
	indent("trailing garbage", `[1] x`, "", "  ")
	indent("html not escaped", `"<&>"`, "", "  ")
	indent("long array", `[1,2,3,4,5,6,7,8,9,10]`, "", " ")

	compact("object", " { \"a\" : [ 1 , 2 ] , \"b\" : \"x y\" } ")
	compact("newlines", "[\n1,\n2\n]\n")
	compact("string kept", `{"a b":"c  d"}`)
	compact("html", `"<script>&"`)
	compact("sep", "\""+string(rune(0x2028))+string(rune(0x2029))+"\"")
	compact("invalid", `{"a":}`)
	compact("empty", ``)
	compact("partial", `[1,`)
	compact("deep", `[[[[ ]]]]`)

	var buf bytes.Buffer
	buf.WriteString("keep:")
	err = json.Compact(&buf, []byte(`[1, x]`))
	fmt.Printf("%q %v\n", buf.String(), err)
	buf.Reset()
	buf.WriteString("keep:")
	err = json.Indent(&buf, []byte(`[1, x]`), "", " ")
	fmt.Printf("%q %v\n", buf.String(), err)

	buf.Reset()
	json.HTMLEscape(&buf, []byte(`{"a":"<tag> & 'q' <"}`))
	fmt.Println(buf.String())

	// AppendIndent style: Indent appends to a buffer that already has data.
	buf.Reset()
	buf.WriteString("prefix\n")
	json.Indent(&buf, []byte(`{"k":[1]}`), "  ", "\t")
	fmt.Printf("%q\n", buf.String())

	// Marshal output is compact; MarshalIndent of a RawMessage is re-indented.
	b, _ = json.MarshalIndent(struct{ R json.RawMessage }{json.RawMessage(`{"x":[1,2],"y":{}}`)}, "", "  ")
	fmt.Printf("%s\n", b)
}

// Output:
// --- MarshalIndent prefix="" indent="  " err=<nil>
// {
//   "id": 1,
//   "name": "x\u003cy",
//   "tags": [
//     "a",
//     "b"
//   ],
//   "attrs": {
//     "a": [],
//     "m": {},
//     "z": 1
//   },
//   "sub": {
//     "id": 2,
//     "name": "",
//     "tags": null,
//     "attrs": null,
//     "empty": {},
//     "es": null,
//     "em": null
//   },
//   "empty": {},
//   "es": null,
//   "em": null
// }
// --- MarshalIndent prefix="" indent="\t" err=<nil>
// {
// 	"id": 1,
// 	"name": "x\u003cy",
// 	"tags": [
// 		"a",
// 		"b"
// 	],
// 	"attrs": {
// 		"a": [],
// 		"m": {},
// 		"z": 1
// 	},
// 	"sub": {
// 		"id": 2,
// 		"name": "",
// 		"tags": null,
// 		"attrs": null,
// 		"empty": {},
// 		"es": null,
// 		"em": null
// 	},
// 	"empty": {},
// 	"es": null,
// 	"em": null
// }
// --- MarshalIndent prefix=">> " indent="--" err=<nil>
// {
// >> --"id": 1,
// >> --"name": "x\u003cy",
// >> --"tags": [
// >> ----"a",
// >> ----"b"
// >> --],
// >> --"attrs": {
// >> ----"a": [],
// >> ----"m": {},
// >> ----"z": 1
// >> --},
// >> --"sub": {
// >> ----"id": 2,
// >> ----"name": "",
// >> ----"tags": null,
// >> ----"attrs": null,
// >> ----"empty": {},
// >> ----"es": null,
// >> ----"em": null
// >> --},
// >> --"empty": {},
// >> --"es": null,
// >> --"em": null
// >> }
// --- MarshalIndent prefix="" indent="" err=<nil>
// {
// "id": 1,
// "name": "x\u003cy",
// "tags": [
// "a",
// "b"
// ],
// "attrs": {
// "a": [],
// "m": {},
// "z": 1
// },
// "sub": {
// "id": 2,
// "name": "",
// "tags": null,
// "attrs": null,
// "empty": {},
// "es": null,
// "em": null
// },
// "empty": {},
// "es": null,
// "em": null
// }
// --- MarshalIndent prefix="#" indent="" err=<nil>
// {
// #"id": 1,
// #"name": "x\u003cy",
// #"tags": [
// #"a",
// #"b"
// #],
// #"attrs": {
// #"a": [],
// #"m": {},
// #"z": 1
// #},
// #"sub": {
// #"id": 2,
// #"name": "",
// #"tags": null,
// #"attrs": null,
// #"empty": {},
// #"es": null,
// #"em": null
// #},
// #"empty": {},
// #"es": null,
// #"em": null
// #}
// [] <nil>
// {} <nil>
// null <nil>
// "s" <nil>
// [
// 	[],
// 	{},
// 	[
// 		1
// 	]
// ] <nil>
// --- object (err=<nil>)
// {
//   "a": 1,
//   "b": [
//     1,
//     2,
//     {
//       "c": null
//     }
//   ],
//   "d": {}
// }|
// --- spaces in input (err=<nil>)
// {
//   "a": 1,
//   "b": [],
//   "c": {}
// } |
// --- prefix (err=<nil>)
// [
// //     1,
// //     [
// //         2,
// //         3
// //     ],
// //     {
// //         "a": []
// //     }
// // ]|
// --- scalar (err=<nil>)
// "str"  |
// --- number (err=<nil>)
// 1.5e3|
// --- trailing space kept before error (err=unexpected end of JSON input)
// |
// --- invalid (err=invalid character '1' after object key)
// |
// --- empty (err=unexpected end of JSON input)
// |
// --- trailing garbage (err=invalid character 'x' after top-level value)
// |
// --- html not escaped (err=<nil>)
// "<&>"|
// --- long array (err=<nil>)
// [
//  1,
//  2,
//  3,
//  4,
//  5,
//  6,
//  7,
//  8,
//  9,
//  10
// ]|
// compact object: "{\"a\":[1,2],\"b\":\"x y\"}" err=<nil>
// compact newlines: "[1,2]" err=<nil>
// compact string kept: "{\"a b\":\"c  d\"}" err=<nil>
// compact html: "\"<script>&\"" err=<nil>
// compact sep: "\"\u2028\u2029\"" err=<nil>
// compact invalid: "" err=invalid character '}' looking for beginning of value
// compact empty: "" err=unexpected end of JSON input
// compact partial: "" err=unexpected end of JSON input
// compact deep: "[[[[]]]]" err=<nil>
// "keep:" invalid character 'x' looking for beginning of value
// "keep:" invalid character 'x' looking for beginning of value
// {"a":"\u003ctag\u003e \u0026 'q' \u003c"}
// "prefix\n{\n  \t\"k\": [\n  \t\t1\n  \t]\n  }"
// {
//   "R": {
//     "x": [
//       1,
//       2
//     ],
//     "y": {}
//   }
// }
