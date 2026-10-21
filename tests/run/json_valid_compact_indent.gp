// phase: 09
// encoding/json Valid, Compact, Indent, HTMLEscape and MarshalIndent over a corpus: valid and
// invalid documents, whitespace forms, escapes preserved, error offsets, appended output on error.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

var corpus = []string{
	``, ` `, `null`, ` null `, "\tnull\n", `nul`, `true`, `True`, `false`, `0`, `-0`, `00`, `-`, `1.`, `1.0`, `1e`, `1e+`, `1E-2`, `.1`, `+1`,
	`"a"`, `"a`, `"a\"`, `"é"`, `"\ud800"`, `"\x"`, "\"\t\"", `""`, `''`,
	`[]`, `[ ]`, `[1,2,3]`, `[1,]`, `[,1]`, `[1 2]`, `[[[[]]]]`, `[`, `]`, `[]]`,
	`{}`, `{ }`, `{"a":1}`, `{"a":1,}`, `{a:1}`, `{"a" 1}`, `{"a":}`, `{"a":1 "b":2}`, `{"a":1,"a":2}`, `{"a":{"b":{"c":[]}}}`, `{`, `}`, `{}}`,
	"{\n  \"a\" : [ 1 ,\r\n 2 ] ,\t\"b\" : null\n}\n",
	`[1,2,3]  garbage`, `[1,2,3]   `, "\ufeff[1]", "[1]\x00", "\x00", " \u00a0[1]", "[\u00a01]",
	`{"html":"<>&\u2028\u2029","e":"\n\t\u0000"}`, `[1.5e300,-0.0,0.000001]`,
	`{"a":[{"b":1},{"c":[true,false,null]}],"d":"e"}`,
}

func errorText(err error) string {
	if err == nil {
		return "ok"
	}
	if syntax, isSyntax := err.(*json.SyntaxError); isSyntax {
		return fmt.Sprintf("syntax(%d) %v", syntax.Offset, err)
	}
	return err.Error()
}

func main() {
	fmt.Println("== Valid, Compact, Indent")
	for _, document := range corpus {
		input := []byte(document)
		var compact bytes.Buffer
		compactErr := json.Compact(&compact, input)
		var indented bytes.Buffer
		indentErr := json.Indent(&indented, input, "", "  ")
		var prefixed bytes.Buffer
		prefixErr := json.Indent(&prefixed, input, "> ", "\t")
		fmt.Printf("%-40q valid=%-5v compact=%q (%s) indent=%q (%s) prefixed=%q (%s)\n", document, json.Valid(input), compact.String(), errorText(compactErr), indented.String(), errorText(indentErr), prefixed.String(), errorText(prefixErr))
	}

	fmt.Println("== appends to existing buffer, also on error")
	for _, document := range []string{`{"a":[1,2]}`, `{"a":[1,2`, `[1,2]x`} {
		buffer := bytes.NewBufferString("PRE:")
		err := json.Compact(buffer, []byte(document))
		fmt.Printf("compact %q => %q %s\n", document, buffer.String(), errorText(err))
		buffer = bytes.NewBufferString("PRE:")
		err = json.Indent(buffer, []byte(document), "", " ")
		fmt.Printf("indent  %q => %q %s\n", document, buffer.String(), errorText(err))
	}

	fmt.Println("== MarshalIndent")
	values := []any{
		nil, 1, "s<>", []int{}, []int(nil), map[string]int{}, struct{}{}, []any{}, [0]int{},
		[]any{1, []any{}, map[string]any{}, []any{[]any{}}},
		map[string]any{"a": []int{1, 2}, "b": map[string]any{"c": nil}, "d": []any{}},
		struct {
			A int
			B []string
			C map[string]int
			D struct{}
			E *int
		}{A: 1, B: []string{"x"}, C: map[string]int{"k": 1}},
	}
	for _, value := range values {
		for _, shape := range []struct{ prefix, indent string }{{"", "  "}, {"##", ""}, {"", "\t\t"}, {"\n", " "}} {
			encoded, err := json.MarshalIndent(value, shape.prefix, shape.indent)
			fmt.Printf("%q,%q %v: %q\n", shape.prefix, shape.indent, err, encoded)
		}
	}

	fmt.Println("== HTMLEscape")
	for _, document := range []string{`<>&`, `"<<"`, "\u2028 \u2029 \xe2\x80", "\xe2\x80\xa8\xe2\x80\xa9", `{"<":"&"}`, ``, "<\u2027>\u202a"} {
		var buffer bytes.Buffer
		json.HTMLEscape(&buffer, []byte(document))
		fmt.Printf("%q => %q\n", document, buffer.String())
	}
}

// Output:
// == Valid, Compact, Indent
// ""                                       valid=false compact="" (syntax(0) unexpected end of JSON input) indent="" (syntax(0) unexpected end of JSON input) prefixed="" (syntax(0) unexpected end of JSON input)
// " "                                      valid=false compact="" (syntax(0) unexpected end of JSON input) indent="" (syntax(1) unexpected end of JSON input) prefixed="" (syntax(1) unexpected end of JSON input)
// "null"                                   valid=true  compact="null" (ok) indent="null" (ok) prefixed="null" (ok)
// " null "                                 valid=true  compact="null" (ok) indent="null " (ok) prefixed="null " (ok)
// "\tnull\n"                               valid=true  compact="null" (ok) indent="null\n" (ok) prefixed="null\n" (ok)
// "nul"                                    valid=false compact="" (syntax(0) invalid character ' ' in literal null (expecting 'l')) indent="" (syntax(3) invalid character ' ' in literal null (expecting 'l')) prefixed="" (syntax(3) invalid character ' ' in literal null (expecting 'l'))
// "true"                                   valid=true  compact="true" (ok) indent="true" (ok) prefixed="true" (ok)
// "True"                                   valid=false compact="" (syntax(0) invalid character 'T' looking for beginning of value) indent="" (syntax(1) invalid character 'T' looking for beginning of value) prefixed="" (syntax(1) invalid character 'T' looking for beginning of value)
// "false"                                  valid=true  compact="false" (ok) indent="false" (ok) prefixed="false" (ok)
// "0"                                      valid=true  compact="0" (ok) indent="0" (ok) prefixed="0" (ok)
// "-0"                                     valid=true  compact="-0" (ok) indent="-0" (ok) prefixed="-0" (ok)
// "00"                                     valid=false compact="" (syntax(0) invalid character '0' after top-level value) indent="" (syntax(2) invalid character '0' after top-level value) prefixed="" (syntax(2) invalid character '0' after top-level value)
// "-"                                      valid=false compact="" (syntax(0) invalid character ' ' in numeric literal) indent="" (syntax(1) invalid character ' ' in numeric literal) prefixed="" (syntax(1) invalid character ' ' in numeric literal)
// "1."                                     valid=false compact="" (syntax(0) invalid character ' ' after decimal point in numeric literal) indent="" (syntax(2) invalid character ' ' after decimal point in numeric literal) prefixed="" (syntax(2) invalid character ' ' after decimal point in numeric literal)
// "1.0"                                    valid=true  compact="1.0" (ok) indent="1.0" (ok) prefixed="1.0" (ok)
// "1e"                                     valid=false compact="" (syntax(0) invalid character ' ' in exponent of numeric literal) indent="" (syntax(2) invalid character ' ' in exponent of numeric literal) prefixed="" (syntax(2) invalid character ' ' in exponent of numeric literal)
// "1e+"                                    valid=false compact="" (syntax(0) invalid character ' ' in exponent of numeric literal) indent="" (syntax(3) invalid character ' ' in exponent of numeric literal) prefixed="" (syntax(3) invalid character ' ' in exponent of numeric literal)
// "1E-2"                                   valid=true  compact="1E-2" (ok) indent="1E-2" (ok) prefixed="1E-2" (ok)
// ".1"                                     valid=false compact="" (syntax(0) invalid character '.' looking for beginning of value) indent="" (syntax(1) invalid character '.' looking for beginning of value) prefixed="" (syntax(1) invalid character '.' looking for beginning of value)
// "+1"                                     valid=false compact="" (syntax(0) invalid character '+' looking for beginning of value) indent="" (syntax(1) invalid character '+' looking for beginning of value) prefixed="" (syntax(1) invalid character '+' looking for beginning of value)
// "\"a\""                                  valid=true  compact="\"a\"" (ok) indent="\"a\"" (ok) prefixed="\"a\"" (ok)
// "\"a"                                    valid=false compact="" (syntax(0) unexpected end of JSON input) indent="" (syntax(2) unexpected end of JSON input) prefixed="" (syntax(2) unexpected end of JSON input)
// "\"a\\\""                                valid=false compact="" (syntax(0) unexpected end of JSON input) indent="" (syntax(4) unexpected end of JSON input) prefixed="" (syntax(4) unexpected end of JSON input)
// "\"é\""                                  valid=true  compact="\"é\"" (ok) indent="\"é\"" (ok) prefixed="\"é\"" (ok)
// "\"\\ud800\""                            valid=true  compact="\"\\ud800\"" (ok) indent="\"\\ud800\"" (ok) prefixed="\"\\ud800\"" (ok)
// "\"\\x\""                                valid=false compact="" (syntax(0) invalid character 'x' in string escape code) indent="" (syntax(3) invalid character 'x' in string escape code) prefixed="" (syntax(3) invalid character 'x' in string escape code)
// "\"\t\""                                 valid=false compact="" (syntax(0) invalid character '\t' in string literal) indent="" (syntax(2) invalid character '\t' in string literal) prefixed="" (syntax(2) invalid character '\t' in string literal)
// "\"\""                                   valid=true  compact="\"\"" (ok) indent="\"\"" (ok) prefixed="\"\"" (ok)
// "''"                                     valid=false compact="" (syntax(0) invalid character '\'' looking for beginning of value) indent="" (syntax(1) invalid character '\'' looking for beginning of value) prefixed="" (syntax(1) invalid character '\'' looking for beginning of value)
// "[]"                                     valid=true  compact="[]" (ok) indent="[]" (ok) prefixed="[]" (ok)
// "[ ]"                                    valid=true  compact="[]" (ok) indent="[]" (ok) prefixed="[]" (ok)
// "[1,2,3]"                                valid=true  compact="[1,2,3]" (ok) indent="[\n  1,\n  2,\n  3\n]" (ok) prefixed="[\n> \t1,\n> \t2,\n> \t3\n> ]" (ok)
// "[1,]"                                   valid=false compact="" (syntax(0) invalid character ']' looking for beginning of value) indent="" (syntax(4) invalid character ']' looking for beginning of value) prefixed="" (syntax(4) invalid character ']' looking for beginning of value)
// "[,1]"                                   valid=false compact="" (syntax(0) invalid character ',' looking for beginning of value) indent="" (syntax(2) invalid character ',' looking for beginning of value) prefixed="" (syntax(2) invalid character ',' looking for beginning of value)
// "[1 2]"                                  valid=false compact="" (syntax(0) invalid character '2' after array element) indent="" (syntax(4) invalid character '2' after array element) prefixed="" (syntax(4) invalid character '2' after array element)
// "[[[[]]]]"                               valid=true  compact="[[[[]]]]" (ok) indent="[\n  [\n    [\n      []\n    ]\n  ]\n]" (ok) prefixed="[\n> \t[\n> \t\t[\n> \t\t\t[]\n> \t\t]\n> \t]\n> ]" (ok)
// "["                                      valid=false compact="" (syntax(0) unexpected end of JSON input) indent="" (syntax(1) unexpected end of JSON input) prefixed="" (syntax(1) unexpected end of JSON input)
// "]"                                      valid=false compact="" (syntax(0) invalid character ']' looking for beginning of value) indent="" (syntax(1) invalid character ']' looking for beginning of value) prefixed="" (syntax(1) invalid character ']' looking for beginning of value)
// "[]]"                                    valid=false compact="" (syntax(0) invalid character ']' after top-level value) indent="" (syntax(3) invalid character ']' after top-level value) prefixed="" (syntax(3) invalid character ']' after top-level value)
// "{}"                                     valid=true  compact="{}" (ok) indent="{}" (ok) prefixed="{}" (ok)
// "{ }"                                    valid=true  compact="{}" (ok) indent="{}" (ok) prefixed="{}" (ok)
// "{\"a\":1}"                              valid=true  compact="{\"a\":1}" (ok) indent="{\n  \"a\": 1\n}" (ok) prefixed="{\n> \t\"a\": 1\n> }" (ok)
// "{\"a\":1,}"                             valid=false compact="" (syntax(0) invalid character '}' looking for beginning of object key string) indent="" (syntax(8) invalid character '}' looking for beginning of object key string) prefixed="" (syntax(8) invalid character '}' looking for beginning of object key string)
// "{a:1}"                                  valid=false compact="" (syntax(0) invalid character 'a' looking for beginning of object key string) indent="" (syntax(2) invalid character 'a' looking for beginning of object key string) prefixed="" (syntax(2) invalid character 'a' looking for beginning of object key string)
// "{\"a\" 1}"                              valid=false compact="" (syntax(0) invalid character '1' after object key) indent="" (syntax(6) invalid character '1' after object key) prefixed="" (syntax(6) invalid character '1' after object key)
// "{\"a\":}"                               valid=false compact="" (syntax(0) invalid character '}' looking for beginning of value) indent="" (syntax(6) invalid character '}' looking for beginning of value) prefixed="" (syntax(6) invalid character '}' looking for beginning of value)
// "{\"a\":1 \"b\":2}"                      valid=false compact="" (syntax(0) invalid character '"' after object key:value pair) indent="" (syntax(8) invalid character '"' after object key:value pair) prefixed="" (syntax(8) invalid character '"' after object key:value pair)
// "{\"a\":1,\"a\":2}"                      valid=true  compact="{\"a\":1,\"a\":2}" (ok) indent="{\n  \"a\": 1,\n  \"a\": 2\n}" (ok) prefixed="{\n> \t\"a\": 1,\n> \t\"a\": 2\n> }" (ok)
// "{\"a\":{\"b\":{\"c\":[]}}}"             valid=true  compact="{\"a\":{\"b\":{\"c\":[]}}}" (ok) indent="{\n  \"a\": {\n    \"b\": {\n      \"c\": []\n    }\n  }\n}" (ok) prefixed="{\n> \t\"a\": {\n> \t\t\"b\": {\n> \t\t\t\"c\": []\n> \t\t}\n> \t}\n> }" (ok)
// "{"                                      valid=false compact="" (syntax(0) unexpected end of JSON input) indent="" (syntax(1) unexpected end of JSON input) prefixed="" (syntax(1) unexpected end of JSON input)
// "}"                                      valid=false compact="" (syntax(0) invalid character '}' looking for beginning of value) indent="" (syntax(1) invalid character '}' looking for beginning of value) prefixed="" (syntax(1) invalid character '}' looking for beginning of value)
// "{}}"                                    valid=false compact="" (syntax(0) invalid character '}' after top-level value) indent="" (syntax(3) invalid character '}' after top-level value) prefixed="" (syntax(3) invalid character '}' after top-level value)
// "{\n  \"a\" : [ 1 ,\r\n 2 ] ,\t\"b\" : null\n}\n" valid=true  compact="{\"a\":[1,2],\"b\":null}" (ok) indent="{\n  \"a\": [\n    1,\n    2\n  ],\n  \"b\": null\n}\n" (ok) prefixed="{\n> \t\"a\": [\n> \t\t1,\n> \t\t2\n> \t],\n> \t\"b\": null\n> }\n" (ok)
// "[1,2,3]  garbage"                       valid=false compact="" (syntax(0) invalid character 'g' after top-level value) indent="" (syntax(10) invalid character 'g' after top-level value) prefixed="" (syntax(10) invalid character 'g' after top-level value)
// "[1,2,3]   "                             valid=true  compact="[1,2,3]" (ok) indent="[\n  1,\n  2,\n  3\n]   " (ok) prefixed="[\n> \t1,\n> \t2,\n> \t3\n> ]   " (ok)
// "\ufeff[1]"                              valid=false compact="" (syntax(0) invalid character 'ï' looking for beginning of value) indent="" (syntax(1) invalid character 'ï' looking for beginning of value) prefixed="" (syntax(1) invalid character 'ï' looking for beginning of value)
// "[1]\x00"                                valid=false compact="" (syntax(0) invalid character '\x00' after top-level value) indent="" (syntax(4) invalid character '\x00' after top-level value) prefixed="" (syntax(4) invalid character '\x00' after top-level value)
// "\x00"                                   valid=false compact="" (syntax(0) invalid character '\x00' looking for beginning of value) indent="" (syntax(1) invalid character '\x00' looking for beginning of value) prefixed="" (syntax(1) invalid character '\x00' looking for beginning of value)
// " \u00a0[1]"                             valid=false compact="" (syntax(0) invalid character 'Â' looking for beginning of value) indent="" (syntax(2) invalid character 'Â' looking for beginning of value) prefixed="" (syntax(2) invalid character 'Â' looking for beginning of value)
// "[\u00a01]"                              valid=false compact="" (syntax(0) invalid character 'Â' looking for beginning of value) indent="" (syntax(2) invalid character 'Â' looking for beginning of value) prefixed="" (syntax(2) invalid character 'Â' looking for beginning of value)
// "{\"html\":\"<>&\\u2028\\u2029\",\"e\":\"\\n\\t\\u0000\"}" valid=true  compact="{\"html\":\"<>&\\u2028\\u2029\",\"e\":\"\\n\\t\\u0000\"}" (ok) indent="{\n  \"html\": \"<>&\\u2028\\u2029\",\n  \"e\": \"\\n\\t\\u0000\"\n}" (ok) prefixed="{\n> \t\"html\": \"<>&\\u2028\\u2029\",\n> \t\"e\": \"\\n\\t\\u0000\"\n> }" (ok)
// "[1.5e300,-0.0,0.000001]"                valid=true  compact="[1.5e300,-0.0,0.000001]" (ok) indent="[\n  1.5e300,\n  -0.0,\n  0.000001\n]" (ok) prefixed="[\n> \t1.5e300,\n> \t-0.0,\n> \t0.000001\n> ]" (ok)
// "{\"a\":[{\"b\":1},{\"c\":[true,false,null]}],\"d\":\"e\"}" valid=true  compact="{\"a\":[{\"b\":1},{\"c\":[true,false,null]}],\"d\":\"e\"}" (ok) indent="{\n  \"a\": [\n    {\n      \"b\": 1\n    },\n    {\n      \"c\": [\n        true,\n        false,\n        null\n      ]\n    }\n  ],\n  \"d\": \"e\"\n}" (ok) prefixed="{\n> \t\"a\": [\n> \t\t{\n> \t\t\t\"b\": 1\n> \t\t},\n> \t\t{\n> \t\t\t\"c\": [\n> \t\t\t\ttrue,\n> \t\t\t\tfalse,\n> \t\t\t\tnull\n> \t\t\t]\n> \t\t}\n> \t],\n> \t\"d\": \"e\"\n> }" (ok)
// == appends to existing buffer, also on error
// compact "{\"a\":[1,2]}" => "PRE:{\"a\":[1,2]}" ok
// indent  "{\"a\":[1,2]}" => "PRE:{\n \"a\": [\n  1,\n  2\n ]\n}" ok
// compact "{\"a\":[1,2" => "PRE:" syntax(0) unexpected end of JSON input
// indent  "{\"a\":[1,2" => "PRE:" syntax(9) unexpected end of JSON input
// compact "[1,2]x" => "PRE:" syntax(0) invalid character 'x' after top-level value
// indent  "[1,2]x" => "PRE:" syntax(6) invalid character 'x' after top-level value
// == MarshalIndent
// "","  " <nil>: "null"
// "##","" <nil>: "null"
// "","\t\t" <nil>: "null"
// "\n"," " <nil>: "null"
// "","  " <nil>: "1"
// "##","" <nil>: "1"
// "","\t\t" <nil>: "1"
// "\n"," " <nil>: "1"
// "","  " <nil>: "\"s\\u003c\\u003e\""
// "##","" <nil>: "\"s\\u003c\\u003e\""
// "","\t\t" <nil>: "\"s\\u003c\\u003e\""
// "\n"," " <nil>: "\"s\\u003c\\u003e\""
// "","  " <nil>: "[]"
// "##","" <nil>: "[]"
// "","\t\t" <nil>: "[]"
// "\n"," " <nil>: "[]"
// "","  " <nil>: "null"
// "##","" <nil>: "null"
// "","\t\t" <nil>: "null"
// "\n"," " <nil>: "null"
// "","  " <nil>: "{}"
// "##","" <nil>: "{}"
// "","\t\t" <nil>: "{}"
// "\n"," " <nil>: "{}"
// "","  " <nil>: "{}"
// "##","" <nil>: "{}"
// "","\t\t" <nil>: "{}"
// "\n"," " <nil>: "{}"
// "","  " <nil>: "[]"
// "##","" <nil>: "[]"
// "","\t\t" <nil>: "[]"
// "\n"," " <nil>: "[]"
// "","  " <nil>: "[]"
// "##","" <nil>: "[]"
// "","\t\t" <nil>: "[]"
// "\n"," " <nil>: "[]"
// "","  " <nil>: "[\n  1,\n  [],\n  {},\n  [\n    []\n  ]\n]"
// "##","" <nil>: "[\n##1,\n##[],\n##{},\n##[\n##[]\n##]\n##]"
// "","\t\t" <nil>: "[\n\t\t1,\n\t\t[],\n\t\t{},\n\t\t[\n\t\t\t\t[]\n\t\t]\n]"
// "\n"," " <nil>: "[\n\n 1,\n\n [],\n\n {},\n\n [\n\n  []\n\n ]\n\n]"
// "","  " <nil>: "{\n  \"a\": [\n    1,\n    2\n  ],\n  \"b\": {\n    \"c\": null\n  },\n  \"d\": []\n}"
// "##","" <nil>: "{\n##\"a\": [\n##1,\n##2\n##],\n##\"b\": {\n##\"c\": null\n##},\n##\"d\": []\n##}"
// "","\t\t" <nil>: "{\n\t\t\"a\": [\n\t\t\t\t1,\n\t\t\t\t2\n\t\t],\n\t\t\"b\": {\n\t\t\t\t\"c\": null\n\t\t},\n\t\t\"d\": []\n}"
// "\n"," " <nil>: "{\n\n \"a\": [\n\n  1,\n\n  2\n\n ],\n\n \"b\": {\n\n  \"c\": null\n\n },\n\n \"d\": []\n\n}"
// "","  " <nil>: "{\n  \"A\": 1,\n  \"B\": [\n    \"x\"\n  ],\n  \"C\": {\n    \"k\": 1\n  },\n  \"D\": {},\n  \"E\": null\n}"
// "##","" <nil>: "{\n##\"A\": 1,\n##\"B\": [\n##\"x\"\n##],\n##\"C\": {\n##\"k\": 1\n##},\n##\"D\": {},\n##\"E\": null\n##}"
// "","\t\t" <nil>: "{\n\t\t\"A\": 1,\n\t\t\"B\": [\n\t\t\t\t\"x\"\n\t\t],\n\t\t\"C\": {\n\t\t\t\t\"k\": 1\n\t\t},\n\t\t\"D\": {},\n\t\t\"E\": null\n}"
// "\n"," " <nil>: "{\n\n \"A\": 1,\n\n \"B\": [\n\n  \"x\"\n\n ],\n\n \"C\": {\n\n  \"k\": 1\n\n },\n\n \"D\": {},\n\n \"E\": null\n\n}"
// == HTMLEscape
// "<>&" => "\\u003c\\u003e\\u0026"
// "\"<<\"" => "\"\\u003c\\u003c\""
// "\u2028 \u2029 \xe2\x80" => "\\u2028 \\u2029 \xe2\x80"
// "\u2028\u2029" => "\\u2028\\u2029"
// "{\"<\":\"&\"}" => "{\"\\u003c\":\"\\u0026\"}"
// "" => ""
// "<‧>\u202a" => "\\u003c‧\\u003e\u202a"
