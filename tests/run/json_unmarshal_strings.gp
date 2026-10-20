// phase: 09
// encoding/json string decoding: escapes, \u sequences, surrogate pairs and lone surrogates, invalid UTF-8 input, long strings, keys with escapes, byte slices.

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func str(in string) {
	var s string
	err := json.Unmarshal([]byte(in), &s)
	fmt.Printf("%-30q -> %+q err=%v\n", in, s, err)
}

func main() {
	for _, in := range []string{
		`""`, `"abc"`, `"a\"b"`, `"a\\b"`, `"a\/b"`, `"\b\f\n\r\t"`, `"@u0041"`, `"@u00e9"`, `"@u20ac"`, `"@u0000"`,
		`"@uD83D@uDE00"`, `"@ud83d@ude00"`, `"@ud83d"`, `"@ude00"`, `"@ud83dx"`, `"@ud83d@u0041"`, `"@ud83d@ud83d@ude00"`,
		`"@uFFFF"`, `"@u2028@u2029"`, `"héllo 世界"`, "\"\xff\"", "\"a\xc3\"", "\"\xed\xa0\x80\"", "\"\xf4\x90\x80\x80\"", "\"\xc0\x80\"",
		`"\U0041"`, `"\x41"`, `"\0"`, `"\'"`, `"@u00"`, `"@uZZZZ"`, `"@ud800@udc00"`, `"@udbff@udfff"`, `"@udc00@ud800"`,
		`"` + strings.Repeat("0123456789", 100) + `"`,
		`"` + strings.Repeat(`@u00e9`, 50) + `"`,
	} {
		in = strings.ReplaceAll(in, "@u", `\u`)
		if len(in) > 60 {
			var s string
			err := json.Unmarshal([]byte(in), &s)
			fmt.Printf("long(%d) -> len %d err=%v\n", len(in), len(s), err)
			continue
		}
		str(in)
	}

	// Keys with escapes match field names.
	type T struct {
		A string `json:"a\"b"`
		B string `json:"é"`
		C string
	}
	var t T
	err := json.Unmarshal([]byte(strings.ReplaceAll(`{"a\"b":"1","@u00e9":"2","@u0043":"3"}`, "@u", `\u`)), &t)
	fmt.Printf("%+v %v\n", t, err)

	var m map[string]int
	err = json.Unmarshal([]byte(strings.ReplaceAll(`{"@ud83d@ude00":1,"@ud83d":2,"a\nb":3}`, "@u", `\u`)), &m)
	fmt.Printf("%+q %v\n", m, err)

	var b []byte
	for _, in := range []string{`"AQID"`, `"AQI="`, `"AQ=="`, `"AQ"`, `"A"`, `""`, `"AQID\n"`, `"AQ ID"`, `"-_-_"`, `"+/+/"`, `"A=="`} {
		b = nil
		err = json.Unmarshal([]byte(in), &b)
		fmt.Printf("bytes %-10s %v %v\n", in, b, err)
	}
	var arr [3]byte
	fmt.Println(json.Unmarshal([]byte(`"AQID"`), &arr), arr)

	var ss []string
	err = json.Unmarshal([]byte(strings.ReplaceAll(`["a","b@u0063","@u00e9",""]`, "@u", `\u`)), &ss)
	fmt.Printf("%q %v\n", ss, err)

	var anyv any
	err = json.Unmarshal([]byte(strings.ReplaceAll(`{"s":"@u00e9\n","l":["@ud83d@ude00"]}`, "@u", `\u`)), &anyv)
	fmt.Printf("%+q %v\n", anyv, err)

	// Invalid UTF-8 is replaced when decoding into a string, and kept in RawMessage.
	var raw json.RawMessage
	err = json.Unmarshal([]byte("\"\xff\""), &raw)
	fmt.Printf("%+q %v\n", string(raw), err)
}

// Output:
// "\"\""                         -> "" err=<nil>
// "\"abc\""                      -> "abc" err=<nil>
// "\"a\\\"b\""                   -> "a\"b" err=<nil>
// "\"a\\\\b\""                   -> "a\\b" err=<nil>
// "\"a\\/b\""                    -> "a/b" err=<nil>
// "\"\\b\\f\\n\\r\\t\""          -> "\b\f\n\r\t" err=<nil>
// "\"\\u0041\""                  -> "A" err=<nil>
// "\"\\u00e9\""                  -> "\u00e9" err=<nil>
// "\"\\u20ac\""                  -> "\u20ac" err=<nil>
// "\"\\u0000\""                  -> "\x00" err=<nil>
// "\"\\uD83D\\uDE00\""           -> "\U0001f600" err=<nil>
// "\"\\ud83d\\ude00\""           -> "\U0001f600" err=<nil>
// "\"\\ud83d\""                  -> "\ufffd" err=<nil>
// "\"\\ude00\""                  -> "\ufffd" err=<nil>
// "\"\\ud83dx\""                 -> "\ufffdx" err=<nil>
// "\"\\ud83d\\u0041\""           -> "\ufffdA" err=<nil>
// "\"\\ud83d\\ud83d\\ude00\""    -> "\ufffd\U0001f600" err=<nil>
// "\"\\uFFFF\""                  -> "\uffff" err=<nil>
// "\"\\u2028\\u2029\""           -> "\u2028\u2029" err=<nil>
// "\"héllo 世界\""                 -> "h\u00e9llo \u4e16\u754c" err=<nil>
// "\"\xff\""                     -> "\ufffd" err=<nil>
// "\"a\xc3\""                    -> "a\ufffd" err=<nil>
// "\"\xed\xa0\x80\""             -> "\ufffd\ufffd\ufffd" err=<nil>
// "\"\xf4\x90\x80\x80\""         -> "\ufffd\ufffd\ufffd\ufffd" err=<nil>
// "\"\xc0\x80\""                 -> "\ufffd\ufffd" err=<nil>
// "\"\\U0041\""                  -> "" err=invalid character 'U' in string escape code
// "\"\\x41\""                    -> "" err=invalid character 'x' in string escape code
// "\"\\0\""                      -> "" err=invalid character '0' in string escape code
// "\"\\'\""                      -> "" err=invalid character '\'' in string escape code
// "\"\\u00\""                    -> "" err=invalid character '"' in \u hexadecimal character escape
// "\"\\uZZZZ\""                  -> "" err=invalid character 'Z' in \u hexadecimal character escape
// "\"\\ud800\\udc00\""           -> "\U00010000" err=<nil>
// "\"\\udbff\\udfff\""           -> "\U0010ffff" err=<nil>
// "\"\\udc00\\ud800\""           -> "\ufffd\ufffd" err=<nil>
// long(1002) -> len 1000 err=<nil>
// long(302) -> len 100 err=<nil>
// {A: B:2 C:3} <nil>
// map["a\nb":'\x03' "\ufffd":'\x02' "\U0001f600":'\x01'] <nil>
// bytes "AQID"     [1 2 3] <nil>
// bytes "AQI="     [1 2] <nil>
// bytes "AQ=="     [1] <nil>
// bytes "AQ"       [] illegal base64 data at input byte 0
// bytes "A"        [] illegal base64 data at input byte 0
// bytes ""         [] <nil>
// bytes "AQID\n"   [1 2 3] <nil>
// bytes "AQ ID"    [] illegal base64 data at input byte 2
// bytes "-_-_"     [] illegal base64 data at input byte 0
// bytes "+/+/"     [251 255 191] <nil>
// bytes "A=="      [] illegal base64 data at input byte 1
// json: cannot unmarshal string into Go value of type [3]uint8 [0 0 0]
// ["a" "bc" "é" ""] <nil>
// map["l":["\U0001f600"] "s":"\u00e9\n"] <nil>
// "\"\xff\"" <nil>
