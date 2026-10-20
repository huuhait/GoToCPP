// phase: 09
// encoding/json errors: SyntaxError messages and offsets, UnmarshalTypeError fields, InvalidUnmarshalError, trailing data, Valid, truncated input, depth limit.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type S struct {
	A int
	B struct {
		C []string
		D map[string]int
	}
	E *int
	F float32
}

func describe(err error) string {
	if err == nil {
		return "<nil>"
	}
	var se *json.SyntaxError
	var ute *json.UnmarshalTypeError
	var iue *json.InvalidUnmarshalError
	switch {
	case errors.As(err, &se):
		return fmt.Sprintf("SyntaxError{%q Offset:%d}", se.Error(), se.Offset)
	case errors.As(err, &ute):
		return fmt.Sprintf("UnmarshalTypeError{Value:%q Type:%v Offset:%d Struct:%q Field:%q} %q", ute.Value, ute.Type, ute.Offset, ute.Struct, ute.Field, ute.Error())
	case errors.As(err, &iue):
		return fmt.Sprintf("InvalidUnmarshalError{Type:%v} %q", iue.Type, iue.Error())
	}
	return fmt.Sprintf("%T %q", err, err.Error())
}

func syn(in string) {
	var v any
	err := json.Unmarshal([]byte(in), &v)
	fmt.Printf("%-28q %s\n", in, describe(err))
}

func main() {
	for _, in := range []string{
		``, ` `, `{`, `}`, `[`, `]`, `[1,]`, `[,1]`, `[1 2]`, `{"a"}`, `{"a":}`, `{"a":1,}`, `{a:1}`, `{'a':1}`, `{"a" 1}`,
		`tru`, `truee`, `nul`, `nulll`, `fals`, `True`, `NaN`, `Infinity`, `-`, `-a`, `01`, `1.`, `.5`, `1e`, `1e+`, `+1`, `0x10`, `1.5.5`,
		`"abc`, `"a\qb"`, `"a\u12"`, `"a\u12zz"`, "\"a\nb\"", "\"a\tb\"", "\"\x01\"", `[1]]`, `{}}`, `1 2`, `[] []`, `{"a":1} x`,
		`"\ud800"`, `"\udc00"`, `"\ud800A"`, `/* c */ 1`, `// c`, `[1,2,3`, `{"a":[1,2}`, `[{"a":1}`, "\xef\xbb\xbf1", `{"a":1,"a"`,
		`  [  1  ,  2  ,  ]  `, `[1,2,3]x`, `"x" "y"`, "[1,\n2,\n3,\n]", `{"k":tru}`, `[nulL]`, `-0`, `0.0`, `-0.0e-0`, `1E5`, `1e-5`,
	} {
		syn(in)
	}

	// UnmarshalTypeError
	var s S
	for _, in := range []string{
		`{"A":"x"}`, `{"A":1.5}`, `{"A":true}`, `{"A":[1]}`, `{"A":{}}`, `{"B":{"C":[1,"x"]}}`, `{"B":{"C":{"x":1}}}`, `{"B":{"D":{"k":"v"}}}`,
		`{"B":[]}`, `{"E":"x"}`, `{"F":1e50}`, `{"F":"x"}`, `{"A":99999999999999999999}`, `[1]`, `"str"`, `5`, `true`,
	} {
		s = S{}
		err := json.Unmarshal([]byte(in), &s)
		fmt.Printf("%-28q %s\n", in, describe(err))
	}

	// InvalidUnmarshalError
	var np *S
	var i int
	fmt.Println(describe(json.Unmarshal([]byte(`{}`), nil)))
	fmt.Println(describe(json.Unmarshal([]byte(`{}`), S{})))
	fmt.Println(describe(json.Unmarshal([]byte(`{}`), np)))
	fmt.Println(describe(json.Unmarshal([]byte(`1`), i)))
	fmt.Println(describe(json.Unmarshal([]byte(`1`), &i)))
	var m map[string]int
	fmt.Println(describe(json.Unmarshal([]byte(`{}`), m)))
	fmt.Println(describe(json.Unmarshal([]byte(`{}`), &m)))
	fmt.Println(describe(json.Unmarshal([]byte(`{"a":1}`), reflect.ValueOf(&m).Interface())))
	fmt.Println(describe(json.Unmarshal([]byte(`{`), nil)))

	// Valid
	for _, in := range []string{``, `{}`, `[1,2`, `"x"`, `nul`, `null`, ` 1 `, `{"a":[]}`, `{"a":[}`, "\x00", `1e400`, `-`, `"\u00e9"`, `"\ud800"`} {
		fmt.Printf("Valid(%q) = %v\n", in, json.Valid([]byte(in)))
	}

	// Depth limit. The scanner counts the depth before anything is built, so these decode into a RawMessage (no recursion).
	deepOK := strings.Repeat("[", 10000) + strings.Repeat("]", 10000)
	deepBad := strings.Repeat("[", 10001) + strings.Repeat("]", 10001)
	var raw json.RawMessage
	fmt.Println("depth 10000:", describe(json.Unmarshal([]byte(deepOK), &raw)), len(raw))
	fmt.Println("depth 10001:", describe(json.Unmarshal([]byte(deepBad), &raw)))
	fmt.Println("valid 10000:", json.Valid([]byte(deepOK)))
	fmt.Println("valid 10001:", json.Valid([]byte(deepBad)))
	objDeep := strings.Repeat(`{"a":`, 10001) + "1" + strings.Repeat("}", 10001)
	fmt.Println("object depth 10001:", describe(json.Unmarshal([]byte(objDeep), &raw)))
	// A value that is built can nest less deep than the limit: the decoder recurses on the stack of its goroutine.
	var v any
	deep1000 := strings.Repeat("[", 1000) + strings.Repeat("]", 1000)
	fmt.Println("depth 1000:", describe(json.Unmarshal([]byte(deep1000), &v)))
	obj1000 := strings.Repeat(`{"a":`, 1000) + "1" + strings.Repeat("}", 1000)
	fmt.Println("object depth 1000:", describe(json.Unmarshal([]byte(obj1000), &v)))
	mb, merr := json.Marshal(v)
	fmt.Println(len(mb), merr)

	// Decoder: truncated input and trailing data.
	dec := json.NewDecoder(strings.NewReader(`{"A":1`))
	fmt.Println("truncated:", describe(dec.Decode(&s)))
	dec = json.NewDecoder(strings.NewReader(`{"A":1} {"A":2} [`))
	for {
		var x S
		err := dec.Decode(&x)
		fmt.Printf("stream: %+v %s offset=%d\n", x.A, describe(err), dec.InputOffset())
		if err != nil {
			break
		}
	}
	dec = json.NewDecoder(strings.NewReader(``))
	err := dec.Decode(&s)
	fmt.Println("empty:", err, err == io.EOF)
	dec = json.NewDecoder(strings.NewReader(`   `))
	err = dec.Decode(&s)
	fmt.Println("blank:", err, err == io.EOF)
	dec = json.NewDecoder(bytes.NewReader([]byte(`[1,2`)))
	var arr []int
	err = dec.Decode(&arr)
	fmt.Println("unexpected EOF:", err, err == io.ErrUnexpectedEOF, arr)

	// A reader that fails.
	dec = json.NewDecoder(io.MultiReader(strings.NewReader(`{"A":`), errReader{}))
	err = dec.Decode(&s)
	fmt.Println("reader error:", err)
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

// Output:
// ""                           SyntaxError{"unexpected end of JSON input" Offset:0}
// " "                          SyntaxError{"unexpected end of JSON input" Offset:1}
// "{"                          SyntaxError{"unexpected end of JSON input" Offset:1}
// "}"                          SyntaxError{"invalid character '}' looking for beginning of value" Offset:1}
// "["                          SyntaxError{"unexpected end of JSON input" Offset:1}
// "]"                          SyntaxError{"invalid character ']' looking for beginning of value" Offset:1}
// "[1,]"                       SyntaxError{"invalid character ']' looking for beginning of value" Offset:4}
// "[,1]"                       SyntaxError{"invalid character ',' looking for beginning of value" Offset:2}
// "[1 2]"                      SyntaxError{"invalid character '2' after array element" Offset:4}
// "{\"a\"}"                    SyntaxError{"invalid character '}' after object key" Offset:5}
// "{\"a\":}"                   SyntaxError{"invalid character '}' looking for beginning of value" Offset:6}
// "{\"a\":1,}"                 SyntaxError{"invalid character '}' looking for beginning of object key string" Offset:8}
// "{a:1}"                      SyntaxError{"invalid character 'a' looking for beginning of object key string" Offset:2}
// "{'a':1}"                    SyntaxError{"invalid character '\\'' looking for beginning of object key string" Offset:2}
// "{\"a\" 1}"                  SyntaxError{"invalid character '1' after object key" Offset:6}
// "tru"                        SyntaxError{"invalid character ' ' in literal true (expecting 'e')" Offset:3}
// "truee"                      SyntaxError{"invalid character 'e' after top-level value" Offset:5}
// "nul"                        SyntaxError{"invalid character ' ' in literal null (expecting 'l')" Offset:3}
// "nulll"                      SyntaxError{"invalid character 'l' after top-level value" Offset:5}
// "fals"                       SyntaxError{"invalid character ' ' in literal false (expecting 'e')" Offset:4}
// "True"                       SyntaxError{"invalid character 'T' looking for beginning of value" Offset:1}
// "NaN"                        SyntaxError{"invalid character 'N' looking for beginning of value" Offset:1}
// "Infinity"                   SyntaxError{"invalid character 'I' looking for beginning of value" Offset:1}
// "-"                          SyntaxError{"invalid character ' ' in numeric literal" Offset:1}
// "-a"                         SyntaxError{"invalid character 'a' in numeric literal" Offset:2}
// "01"                         SyntaxError{"invalid character '1' after top-level value" Offset:2}
// "1."                         SyntaxError{"invalid character ' ' after decimal point in numeric literal" Offset:2}
// ".5"                         SyntaxError{"invalid character '.' looking for beginning of value" Offset:1}
// "1e"                         SyntaxError{"invalid character ' ' in exponent of numeric literal" Offset:2}
// "1e+"                        SyntaxError{"invalid character ' ' in exponent of numeric literal" Offset:3}
// "+1"                         SyntaxError{"invalid character '+' looking for beginning of value" Offset:1}
// "0x10"                       SyntaxError{"invalid character 'x' after top-level value" Offset:2}
// "1.5.5"                      SyntaxError{"invalid character '.' after top-level value" Offset:4}
// "\"abc"                      SyntaxError{"unexpected end of JSON input" Offset:4}
// "\"a\\qb\""                  SyntaxError{"invalid character 'q' in string escape code" Offset:4}
// "\"a\\u12\""                 SyntaxError{"invalid character '\"' in \\u hexadecimal character escape" Offset:7}
// "\"a\\u12zz\""               SyntaxError{"invalid character 'z' in \\u hexadecimal character escape" Offset:7}
// "\"a\nb\""                   SyntaxError{"invalid character '\\n' in string literal" Offset:3}
// "\"a\tb\""                   SyntaxError{"invalid character '\\t' in string literal" Offset:3}
// "\"\x01\""                   SyntaxError{"invalid character '\\x01' in string literal" Offset:2}
// "[1]]"                       SyntaxError{"invalid character ']' after top-level value" Offset:4}
// "{}}"                        SyntaxError{"invalid character '}' after top-level value" Offset:3}
// "1 2"                        SyntaxError{"invalid character '2' after top-level value" Offset:3}
// "[] []"                      SyntaxError{"invalid character '[' after top-level value" Offset:4}
// "{\"a\":1} x"                SyntaxError{"invalid character 'x' after top-level value" Offset:9}
// "\"\\ud800\""                <nil>
// "\"\\udc00\""                <nil>
// "\"\\ud800A\""               <nil>
// "/* c */ 1"                  SyntaxError{"invalid character '/' looking for beginning of value" Offset:1}
// "// c"                       SyntaxError{"invalid character '/' looking for beginning of value" Offset:1}
// "[1,2,3"                     SyntaxError{"unexpected end of JSON input" Offset:6}
// "{\"a\":[1,2}"               SyntaxError{"invalid character '}' after array element" Offset:10}
// "[{\"a\":1}"                 SyntaxError{"unexpected end of JSON input" Offset:8}
// "\ufeff1"                    SyntaxError{"invalid character 'ï' looking for beginning of value" Offset:1}
// "{\"a\":1,\"a\""             SyntaxError{"unexpected end of JSON input" Offset:10}
// "  [  1  ,  2  ,  ]  "       SyntaxError{"invalid character ']' looking for beginning of value" Offset:18}
// "[1,2,3]x"                   SyntaxError{"invalid character 'x' after top-level value" Offset:8}
// "\"x\" \"y\""                SyntaxError{"invalid character '\"' after top-level value" Offset:5}
// "[1,\n2,\n3,\n]"             SyntaxError{"invalid character ']' looking for beginning of value" Offset:11}
// "{\"k\":tru}"                SyntaxError{"invalid character '}' in literal true (expecting 'e')" Offset:9}
// "[nulL]"                     SyntaxError{"invalid character 'L' in literal null (expecting 'l')" Offset:5}
// "-0"                         <nil>
// "0.0"                        <nil>
// "-0.0e-0"                    <nil>
// "1E5"                        <nil>
// "1e-5"                       <nil>
// "{\"A\":\"x\"}"              UnmarshalTypeError{Value:"string" Type:int Offset:8 Struct:"S" Field:"A"} "json: cannot unmarshal string into Go struct field S.A of type int"
// "{\"A\":1.5}"                UnmarshalTypeError{Value:"number 1.5" Type:int Offset:8 Struct:"S" Field:"A"} "json: cannot unmarshal number 1.5 into Go struct field S.A of type int"
// "{\"A\":true}"               UnmarshalTypeError{Value:"bool" Type:int Offset:9 Struct:"S" Field:"A"} "json: cannot unmarshal bool into Go struct field S.A of type int"
// "{\"A\":[1]}"                UnmarshalTypeError{Value:"array" Type:int Offset:6 Struct:"S" Field:"A"} "json: cannot unmarshal array into Go struct field S.A of type int"
// "{\"A\":{}}"                 UnmarshalTypeError{Value:"object" Type:int Offset:6 Struct:"S" Field:"A"} "json: cannot unmarshal object into Go struct field S.A of type int"
// "{\"B\":{\"C\":[1,\"x\"]}}"  UnmarshalTypeError{Value:"number" Type:string Offset:12 Struct:"" Field:"B.C"} "json: cannot unmarshal number into Go struct field .B.C of type string"
// "{\"B\":{\"C\":{\"x\":1}}}"  UnmarshalTypeError{Value:"object" Type:[]string Offset:11 Struct:"" Field:"B.C"} "json: cannot unmarshal object into Go struct field .B.C of type []string"
// "{\"B\":{\"D\":{\"k\":\"v\"}}}" UnmarshalTypeError{Value:"string" Type:int Offset:18 Struct:"" Field:"B.D"} "json: cannot unmarshal string into Go struct field .B.D of type int"
// "{\"B\":[]}"                 UnmarshalTypeError{Value:"array" Type:struct { C []string; D map[string]int } Offset:6 Struct:"S" Field:"B"} "json: cannot unmarshal array into Go struct field S.B of type struct { C []string; D map[string]int }"
// "{\"E\":\"x\"}"              UnmarshalTypeError{Value:"string" Type:int Offset:8 Struct:"S" Field:"E"} "json: cannot unmarshal string into Go struct field S.E of type int"
// "{\"F\":1e50}"               UnmarshalTypeError{Value:"number 1e50" Type:float32 Offset:9 Struct:"S" Field:"F"} "json: cannot unmarshal number 1e50 into Go struct field S.F of type float32"
// "{\"F\":\"x\"}"              UnmarshalTypeError{Value:"string" Type:float32 Offset:8 Struct:"S" Field:"F"} "json: cannot unmarshal string into Go struct field S.F of type float32"
// "{\"A\":99999999999999999999}" UnmarshalTypeError{Value:"number 99999999999999999999" Type:int Offset:25 Struct:"S" Field:"A"} "json: cannot unmarshal number 99999999999999999999 into Go struct field S.A of type int"
// "[1]"                        UnmarshalTypeError{Value:"array" Type:main.S Offset:1 Struct:"" Field:""} "json: cannot unmarshal array into Go value of type main.S"
// "\"str\""                    UnmarshalTypeError{Value:"string" Type:main.S Offset:5 Struct:"" Field:""} "json: cannot unmarshal string into Go value of type main.S"
// "5"                          UnmarshalTypeError{Value:"number" Type:main.S Offset:1 Struct:"" Field:""} "json: cannot unmarshal number into Go value of type main.S"
// "true"                       UnmarshalTypeError{Value:"bool" Type:main.S Offset:4 Struct:"" Field:""} "json: cannot unmarshal bool into Go value of type main.S"
// InvalidUnmarshalError{Type:<nil>} "json: Unmarshal(nil)"
// InvalidUnmarshalError{Type:main.S} "json: Unmarshal(non-pointer main.S)"
// InvalidUnmarshalError{Type:*main.S} "json: Unmarshal(nil *main.S)"
// InvalidUnmarshalError{Type:int} "json: Unmarshal(non-pointer int)"
// <nil>
// InvalidUnmarshalError{Type:map[string]int} "json: Unmarshal(non-pointer map[string]int)"
// <nil>
// <nil>
// SyntaxError{"unexpected end of JSON input" Offset:1}
// Valid("") = false
// Valid("{}") = true
// Valid("[1,2") = false
// Valid("\"x\"") = true
// Valid("nul") = false
// Valid("null") = true
// Valid(" 1 ") = true
// Valid("{\"a\":[]}") = true
// Valid("{\"a\":[}") = false
// Valid("\x00") = false
// Valid("1e400") = true
// Valid("-") = false
// Valid("\"\\u00e9\"") = true
// Valid("\"\\ud800\"") = true
// depth 10000: <nil> 20000
// depth 10001: SyntaxError{"invalid character '[' exceeded max depth" Offset:10001}
// valid 10000: true
// valid 10001: false
// object depth 10001: SyntaxError{"invalid character '{' exceeded max depth" Offset:50001}
// depth 1000: <nil>
// object depth 1000: <nil>
// 6001 <nil>
// truncated: *errors.errorString "unexpected EOF"
// stream: 1 <nil> offset=7
// stream: 2 <nil> offset=15
// stream: 0 *errors.errorString "unexpected EOF" offset=15
// empty: EOF true
// blank: EOF true
// unexpected EOF: unexpected EOF true []
// reader error: read failed
