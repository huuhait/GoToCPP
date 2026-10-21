// phase: 09
// encoding/json syntax errors: every truncation and a set of single-byte mutations of valid
// documents give the same error text and offset as Go, through Unmarshal, Valid and a Decoder.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var corpus = []string{
	`{"a":[1,2.5e3,-0.0,true,false,null,"s\u00e9\n"],"b":{"c":{}},"d":[]}`,
	`[[],[[]],{"k":[{"x":1}]}, "tail" ]`,
	`"plain \"quoted\" \\ \/ \b\f\n\r\t \ud83d\ude00"`,
	`-12.5E+7`,
	"{\n\t\"key\" :\r\n [ 1 , 2 ]\n}",
	`{"":0,"\u0000":1}`,
}

func describe(err error) string {
	if err == nil {
		return "ok"
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Sprintf("syntax offset=%d %q", syntax.Offset, syntax.Error())
	}
	return fmt.Sprintf("%T %q", err, err.Error())
}

func decoderResult(input []byte) string {
	decoder := json.NewDecoder(bytes.NewReader(input))
	var value any
	err := decoder.Decode(&value)
	if err == io.EOF {
		return "EOF"
	}
	return fmt.Sprintf("%s off=%d", describe(err), decoder.InputOffset())
}

func check(input []byte) {
	var value any
	err := json.Unmarshal(input, &value)
	fmt.Printf("%-40q valid=%-5v unmarshal=%s | decoder=%s\n", input, json.Valid(input), describe(err), decoderResult(input))
}

func main() {
	fmt.Println("== truncations")
	for index, document := range corpus {
		fmt.Println("-- document", index)
		for cut := 0; cut <= len(document); cut++ {
			check([]byte(document[:cut]))
		}
	}

	fmt.Println("== single byte mutations")
	replacements := []byte{'}', ']', ',', ':', '"', '\\', 'x', '0', '-', '.', 'e', 0, '\n'}
	for index, document := range corpus[:3] {
		fmt.Println("-- document", index)
		for position := 0; position < len(document); position += 3 {
			for _, replacement := range replacements {
				mutated := []byte(document)
				mutated[position] = replacement
				check(mutated)
			}
		}
	}

	fmt.Println("== trailing data")
	for _, input := range []string{`1 2`, `{} {}`, `[]x`, `"a" "b"`, `null,`, `true false`, "1\n\n", ` 1 `, `01`, `1.`, `.5`, `+1`, `0x10`, `NaN`, `Infinity`, `tru`, `nul`, `fals`, `nulll`} {
		check([]byte(input))
	}
}

// Output:
// == truncations
// -- document 0
// ""                                       valid=false unmarshal=syntax offset=0 "unexpected end of JSON input" | decoder=EOF
// "{"                                      valid=false unmarshal=syntax offset=1 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\""                                    valid=false unmarshal=syntax offset=2 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a"                                   valid=false unmarshal=syntax offset=3 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\""                                 valid=false unmarshal=syntax offset=4 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":"                                valid=false unmarshal=syntax offset=5 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":["                               valid=false unmarshal=syntax offset=6 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1"                              valid=false unmarshal=syntax offset=7 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,"                             valid=false unmarshal=syntax offset=8 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2"                            valid=false unmarshal=syntax offset=9 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2."                           valid=false unmarshal=syntax offset=10 "invalid character ' ' after decimal point in numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5"                          valid=false unmarshal=syntax offset=11 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e"                         valid=false unmarshal=syntax offset=12 "invalid character ' ' in exponent of numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3"                        valid=false unmarshal=syntax offset=13 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,"                       valid=false unmarshal=syntax offset=14 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-"                      valid=false unmarshal=syntax offset=15 "invalid character ' ' in numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0"                     valid=false unmarshal=syntax offset=16 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0."                    valid=false unmarshal=syntax offset=17 "invalid character ' ' after decimal point in numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0"                   valid=false unmarshal=syntax offset=18 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,"                  valid=false unmarshal=syntax offset=19 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,t"                 valid=false unmarshal=syntax offset=20 "invalid character ' ' in literal true (expecting 'r')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,tr"                valid=false unmarshal=syntax offset=21 "invalid character ' ' in literal true (expecting 'u')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,tru"               valid=false unmarshal=syntax offset=22 "invalid character ' ' in literal true (expecting 'e')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true"              valid=false unmarshal=syntax offset=23 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,"             valid=false unmarshal=syntax offset=24 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,f"            valid=false unmarshal=syntax offset=25 "invalid character ' ' in literal false (expecting 'a')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fa"           valid=false unmarshal=syntax offset=26 "invalid character ' ' in literal false (expecting 'l')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal"          valid=false unmarshal=syntax offset=27 "invalid character ' ' in literal false (expecting 's')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fals"         valid=false unmarshal=syntax offset=28 "invalid character ' ' in literal false (expecting 'e')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false"        valid=false unmarshal=syntax offset=29 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,"       valid=false unmarshal=syntax offset=30 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,n"      valid=false unmarshal=syntax offset=31 "invalid character ' ' in literal null (expecting 'u')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nu"     valid=false unmarshal=syntax offset=32 "invalid character ' ' in literal null (expecting 'l')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul"    valid=false unmarshal=syntax offset=33 "invalid character ' ' in literal null (expecting 'l')" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null"   valid=false unmarshal=syntax offset=34 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,"  valid=false unmarshal=syntax offset=35 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"" valid=false unmarshal=syntax offset=36 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s" valid=false unmarshal=syntax offset=37 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\" valid=false unmarshal=syntax offset=38 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u" valid=false unmarshal=syntax offset=39 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u0" valid=false unmarshal=syntax offset=40 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00" valid=false unmarshal=syntax offset=41 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e" valid=false unmarshal=syntax offset=42 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9" valid=false unmarshal=syntax offset=43 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\" valid=false unmarshal=syntax offset=44 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n" valid=false unmarshal=syntax offset=45 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"" valid=false unmarshal=syntax offset=46 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"]" valid=false unmarshal=syntax offset=47 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"]," valid=false unmarshal=syntax offset=48 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"" valid=false unmarshal=syntax offset=49 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b" valid=false unmarshal=syntax offset=50 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"" valid=false unmarshal=syntax offset=51 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":" valid=false unmarshal=syntax offset=52 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{" valid=false unmarshal=syntax offset=53 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"" valid=false unmarshal=syntax offset=54 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c" valid=false unmarshal=syntax offset=55 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\"" valid=false unmarshal=syntax offset=56 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":" valid=false unmarshal=syntax offset=57 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{" valid=false unmarshal=syntax offset=58 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}" valid=false unmarshal=syntax offset=59 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}" valid=false unmarshal=syntax offset=60 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}," valid=false unmarshal=syntax offset=61 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"" valid=false unmarshal=syntax offset=62 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d" valid=false unmarshal=syntax offset=63 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\"" valid=false unmarshal=syntax offset=64 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":" valid=false unmarshal=syntax offset=65 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[" valid=false unmarshal=syntax offset=66 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]" valid=false unmarshal=syntax offset=67 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// -- document 1
// ""                                       valid=false unmarshal=syntax offset=0 "unexpected end of JSON input" | decoder=EOF
// "["                                      valid=false unmarshal=syntax offset=1 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[["                                     valid=false unmarshal=syntax offset=2 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[]"                                    valid=false unmarshal=syntax offset=3 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],"                                   valid=false unmarshal=syntax offset=4 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],["                                  valid=false unmarshal=syntax offset=5 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[["                                 valid=false unmarshal=syntax offset=6 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]"                                valid=false unmarshal=syntax offset=7 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]]"                               valid=false unmarshal=syntax offset=8 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],"                              valid=false unmarshal=syntax offset=9 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{"                             valid=false unmarshal=syntax offset=10 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\""                           valid=false unmarshal=syntax offset=11 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k"                          valid=false unmarshal=syntax offset=12 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\""                        valid=false unmarshal=syntax offset=13 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":"                       valid=false unmarshal=syntax offset=14 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":["                      valid=false unmarshal=syntax offset=15 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{"                     valid=false unmarshal=syntax offset=16 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\""                   valid=false unmarshal=syntax offset=17 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x"                  valid=false unmarshal=syntax offset=18 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\""                valid=false unmarshal=syntax offset=19 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":"               valid=false unmarshal=syntax offset=20 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1"              valid=false unmarshal=syntax offset=21 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}"             valid=false unmarshal=syntax offset=22 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]"            valid=false unmarshal=syntax offset=23 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}"           valid=false unmarshal=syntax offset=24 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]},"          valid=false unmarshal=syntax offset=25 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, "         valid=false unmarshal=syntax offset=26 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \""       valid=false unmarshal=syntax offset=27 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"t"      valid=false unmarshal=syntax offset=28 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"ta"     valid=false unmarshal=syntax offset=29 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai"    valid=false unmarshal=syntax offset=30 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail"   valid=false unmarshal=syntax offset=31 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\"" valid=false unmarshal=syntax offset=32 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" " valid=false unmarshal=syntax offset=33 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// -- document 2
// ""                                       valid=false unmarshal=syntax offset=0 "unexpected end of JSON input" | decoder=EOF
// "\""                                     valid=false unmarshal=syntax offset=1 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"p"                                    valid=false unmarshal=syntax offset=2 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"pl"                                   valid=false unmarshal=syntax offset=3 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"pla"                                  valid=false unmarshal=syntax offset=4 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plai"                                 valid=false unmarshal=syntax offset=5 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain"                                valid=false unmarshal=syntax offset=6 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain "                               valid=false unmarshal=syntax offset=7 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\"                             valid=false unmarshal=syntax offset=8 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\""                           valid=false unmarshal=syntax offset=9 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"q"                          valid=false unmarshal=syntax offset=10 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"qu"                         valid=false unmarshal=syntax offset=11 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quo"                        valid=false unmarshal=syntax offset=12 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quot"                       valid=false unmarshal=syntax offset=13 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quote"                      valid=false unmarshal=syntax offset=14 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted"                     valid=false unmarshal=syntax offset=15 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\"                   valid=false unmarshal=syntax offset=16 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\""                 valid=false unmarshal=syntax offset=17 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" "                valid=false unmarshal=syntax offset=18 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\"              valid=false unmarshal=syntax offset=19 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\"            valid=false unmarshal=syntax offset=20 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ "           valid=false unmarshal=syntax offset=21 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\"         valid=false unmarshal=syntax offset=22 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/"        valid=false unmarshal=syntax offset=23 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ "       valid=false unmarshal=syntax offset=24 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\"     valid=false unmarshal=syntax offset=25 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b"    valid=false unmarshal=syntax offset=26 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\"  valid=false unmarshal=syntax offset=27 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f" valid=false unmarshal=syntax offset=28 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\" valid=false unmarshal=syntax offset=29 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n" valid=false unmarshal=syntax offset=30 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\" valid=false unmarshal=syntax offset=31 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r" valid=false unmarshal=syntax offset=32 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\" valid=false unmarshal=syntax offset=33 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t" valid=false unmarshal=syntax offset=34 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t " valid=false unmarshal=syntax offset=35 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\" valid=false unmarshal=syntax offset=36 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\u" valid=false unmarshal=syntax offset=37 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud" valid=false unmarshal=syntax offset=38 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8" valid=false unmarshal=syntax offset=39 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83" valid=false unmarshal=syntax offset=40 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d" valid=false unmarshal=syntax offset=41 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\" valid=false unmarshal=syntax offset=42 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\u" valid=false unmarshal=syntax offset=43 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ud" valid=false unmarshal=syntax offset=44 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude" valid=false unmarshal=syntax offset=45 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude0" valid=false unmarshal=syntax offset=46 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00" valid=false unmarshal=syntax offset=47 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// -- document 3
// ""                                       valid=false unmarshal=syntax offset=0 "unexpected end of JSON input" | decoder=EOF
// "-"                                      valid=false unmarshal=syntax offset=1 "invalid character ' ' in numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// "-1"                                     valid=true  unmarshal=ok | decoder=ok off=2
// "-12"                                    valid=true  unmarshal=ok | decoder=ok off=3
// "-12."                                   valid=false unmarshal=syntax offset=4 "invalid character ' ' after decimal point in numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// "-12.5"                                  valid=true  unmarshal=ok | decoder=ok off=5
// "-12.5E"                                 valid=false unmarshal=syntax offset=6 "invalid character ' ' in exponent of numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// "-12.5E+"                                valid=false unmarshal=syntax offset=7 "invalid character ' ' in exponent of numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// "-12.5E+7"                               valid=true  unmarshal=ok | decoder=ok off=8
// -- document 4
// ""                                       valid=false unmarshal=syntax offset=0 "unexpected end of JSON input" | decoder=EOF
// "{"                                      valid=false unmarshal=syntax offset=1 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n"                                    valid=false unmarshal=syntax offset=2 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t"                                  valid=false unmarshal=syntax offset=3 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\""                                valid=false unmarshal=syntax offset=4 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"k"                               valid=false unmarshal=syntax offset=5 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"ke"                              valid=false unmarshal=syntax offset=6 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key"                             valid=false unmarshal=syntax offset=7 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\""                           valid=false unmarshal=syntax offset=8 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" "                          valid=false unmarshal=syntax offset=9 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :"                         valid=false unmarshal=syntax offset=10 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r"                       valid=false unmarshal=syntax offset=11 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n"                     valid=false unmarshal=syntax offset=12 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n "                    valid=false unmarshal=syntax offset=13 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n ["                   valid=false unmarshal=syntax offset=14 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ "                  valid=false unmarshal=syntax offset=15 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1"                 valid=false unmarshal=syntax offset=16 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1 "                valid=false unmarshal=syntax offset=17 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1 ,"               valid=false unmarshal=syntax offset=18 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1 , "              valid=false unmarshal=syntax offset=19 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1 , 2"             valid=false unmarshal=syntax offset=20 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1 , 2 "            valid=false unmarshal=syntax offset=21 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1 , 2 ]"           valid=false unmarshal=syntax offset=22 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1 , 2 ]\n"         valid=false unmarshal=syntax offset=23 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\n\t\"key\" :\r\n [ 1 , 2 ]\n}"        valid=true  unmarshal=ok | decoder=ok off=24
// -- document 5
// ""                                       valid=false unmarshal=syntax offset=0 "unexpected end of JSON input" | decoder=EOF
// "{"                                      valid=false unmarshal=syntax offset=1 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\""                                    valid=false unmarshal=syntax offset=2 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\""                                  valid=false unmarshal=syntax offset=3 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":"                                 valid=false unmarshal=syntax offset=4 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0"                                valid=false unmarshal=syntax offset=5 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,"                               valid=false unmarshal=syntax offset=6 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\""                             valid=false unmarshal=syntax offset=7 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\"                           valid=false unmarshal=syntax offset=8 "invalid character ' ' in string escape code" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u"                          valid=false unmarshal=syntax offset=9 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u0"                         valid=false unmarshal=syntax offset=10 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u00"                        valid=false unmarshal=syntax offset=11 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u000"                       valid=false unmarshal=syntax offset=12 "invalid character ' ' in \\u hexadecimal character escape" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u0000"                      valid=false unmarshal=syntax offset=13 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u0000\""                    valid=false unmarshal=syntax offset=14 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u0000\":"                   valid=false unmarshal=syntax offset=15 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u0000\":1"                  valid=false unmarshal=syntax offset=16 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"\":0,\"\\u0000\":1}"                 valid=true  unmarshal=ok | decoder=ok off=17
// == single byte mutations
// -- document 0
// "}\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character '}' looking for beginning of value" | decoder=syntax offset=1 "invalid character '}' looking for beginning of value" off=0
// "]\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character ']' looking for beginning of value" | decoder=syntax offset=1 "invalid character ']' looking for beginning of value" off=0
// ",\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character ',' looking for beginning of value" | decoder=syntax offset=1 "invalid character ',' looking for beginning of value" off=0
// ":\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character ':' looking for beginning of value" | decoder=syntax offset=1 "invalid character ':' looking for beginning of value" off=0
// "\"\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=3 "invalid character 'a' after top-level value" | decoder=ok off=2
// "\\\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=1 "invalid character '\\\\' looking for beginning of value" off=0
// "x\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=1 "invalid character 'x' looking for beginning of value" off=0
// "0\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=2 "invalid character '\"' after top-level value" | decoder=ok off=1
// "-\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=2 "invalid character '\"' in numeric literal" | decoder=syntax offset=2 "invalid character '\"' in numeric literal" off=0
// ".\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character '.' looking for beginning of value" | decoder=syntax offset=1 "invalid character '.' looking for beginning of value" off=0
// "e\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=1 "invalid character 'e' looking for beginning of value" off=0
// "\x00\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=1 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=1 "invalid character '\\x00' looking for beginning of value" off=0
// "\n\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=5 "invalid character ':' after top-level value" | decoder=ok off=4
// "{\"a}:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"a]:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"a,:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"a::[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\\:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=5 "invalid character ':' in string escape code" | decoder=syntax offset=5 "invalid character ':' in string escape code" off=0
// "{\"ax:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"a0:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"a-:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"a.:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"ae:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after object key" | decoder=syntax offset=37 "invalid character 's' after object key" off=0
// "{\"a\x00:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=4 "invalid character '\\x00' in string literal" | decoder=syntax offset=4 "invalid character '\\x00' in string literal" off=0
// "{\"a\n:[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=4 "invalid character '\\n' in string literal" | decoder=syntax offset=4 "invalid character '\\n' in string literal" off=0
// "{\"a\":[},2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=7 "invalid character '}' looking for beginning of value" | decoder=syntax offset=7 "invalid character '}' looking for beginning of value" off=0
// "{\"a\":[],2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=9 "invalid character '2' looking for beginning of object key string" | decoder=syntax offset=9 "invalid character '2' looking for beginning of object key string" off=0
// "{\"a\":[,,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=7 "invalid character ',' looking for beginning of value" | decoder=syntax offset=7 "invalid character ',' looking for beginning of value" off=0
// "{\"a\":[:,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=7 "invalid character ':' looking for beginning of value" | decoder=syntax offset=7 "invalid character ':' looking for beginning of value" off=0
// "{\"a\":[\",2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after array element" | decoder=syntax offset=37 "invalid character 's' after array element" off=0
// "{\"a\":[\\,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=7 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=7 "invalid character '\\\\' looking for beginning of value" off=0
// "{\"a\":[x,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=7 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=7 "invalid character 'x' looking for beginning of value" off=0
// "{\"a\":[0,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[-,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=8 "invalid character ',' in numeric literal" | decoder=syntax offset=8 "invalid character ',' in numeric literal" off=0
// "{\"a\":[.,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=7 "invalid character '.' looking for beginning of value" | decoder=syntax offset=7 "invalid character '.' looking for beginning of value" off=0
// "{\"a\":[e,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=7 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=7 "invalid character 'e' looking for beginning of value" off=0
// "{\"a\":[\x00,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=7 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=7 "invalid character '\\x00' looking for beginning of value" off=0
// "{\"a\":[\n,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=8 "invalid character ',' looking for beginning of value" | decoder=syntax offset=8 "invalid character ',' looking for beginning of value" off=0
// "{\"a\":[1,2}5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=10 "invalid character '}' after array element" | decoder=syntax offset=10 "invalid character '}' after array element" off=0
// "{\"a\":[1,2]5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=11 "invalid character '5' after object key:value pair" | decoder=syntax offset=11 "invalid character '5' after object key:value pair" off=0
// "{\"a\":[1,2,5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2:5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=10 "invalid character ':' after array element" | decoder=syntax offset=10 "invalid character ':' after array element" off=0
// "{\"a\":[1,2\"5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=10 "invalid character '\"' after array element" | decoder=syntax offset=10 "invalid character '\"' after array element" off=0
// "{\"a\":[1,2\\5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=10 "invalid character '\\\\' after array element" | decoder=syntax offset=10 "invalid character '\\\\' after array element" off=0
// "{\"a\":[1,2x5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=10 "invalid character 'x' after array element" | decoder=syntax offset=10 "invalid character 'x' after array element" off=0
// "{\"a\":[1,205e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2-5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=10 "invalid character '-' after array element" | decoder=syntax offset=10 "invalid character '-' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2e5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=12 "invalid character 'e' after array element" | decoder=syntax offset=12 "invalid character 'e' after array element" off=0
// "{\"a\":[1,2\x005e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=10 "invalid character '\\x00' after array element" | decoder=syntax offset=10 "invalid character '\\x00' after array element" off=0
// "{\"a\":[1,2\n5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=11 "invalid character '5' after array element" | decoder=syntax offset=11 "invalid character '5' after array element" off=0
// "{\"a\":[1,2.5e},-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character '}' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character '}' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e],-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character ']' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character ']' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e,,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character ',' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character ',' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e:,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character ':' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character ':' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e\",-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character '\"' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character '\"' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e\\,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character '\\\\' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character '\\\\' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5ex,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character 'x' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character 'x' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e0,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e-,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=14 "invalid character ',' in exponent of numeric literal" | decoder=syntax offset=14 "invalid character ',' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e.,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character '.' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character '.' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5ee,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character 'e' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character 'e' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e\x00,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character '\\x00' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character '\\x00' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e\n,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=13 "invalid character '\\n' in exponent of numeric literal" | decoder=syntax offset=13 "invalid character '\\n' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e3,-}.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character '}' in numeric literal" | decoder=syntax offset=16 "invalid character '}' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-].0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character ']' in numeric literal" | decoder=syntax offset=16 "invalid character ']' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-,.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character ',' in numeric literal" | decoder=syntax offset=16 "invalid character ',' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-:.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character ':' in numeric literal" | decoder=syntax offset=16 "invalid character ':' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-\".0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character '\"' in numeric literal" | decoder=syntax offset=16 "invalid character '\"' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-\\.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character '\\\\' in numeric literal" | decoder=syntax offset=16 "invalid character '\\\\' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-x.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character 'x' in numeric literal" | decoder=syntax offset=16 "invalid character 'x' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,--.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character '-' in numeric literal" | decoder=syntax offset=16 "invalid character '-' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-..0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character '.' in numeric literal" | decoder=syntax offset=16 "invalid character '.' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-e.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character 'e' in numeric literal" | decoder=syntax offset=16 "invalid character 'e' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-\x00.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character '\\x00' in numeric literal" | decoder=syntax offset=16 "invalid character '\\x00' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-\n.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=16 "invalid character '\\n' in numeric literal" | decoder=syntax offset=16 "invalid character '\\n' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-0.0}true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=19 "invalid character '}' after array element" | decoder=syntax offset=19 "invalid character '}' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0]true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=20 "invalid character 't' after object key:value pair" | decoder=syntax offset=20 "invalid character 't' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0:true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=19 "invalid character ':' after array element" | decoder=syntax offset=19 "invalid character ':' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0\"true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=19 "invalid character '\"' after array element" | decoder=syntax offset=19 "invalid character '\"' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0\\true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after array element" | decoder=syntax offset=19 "invalid character '\\\\' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0xtrue,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=19 "invalid character 'x' after array element" | decoder=syntax offset=19 "invalid character 'x' after array element" off=0
// "{\"a\":[1,2.5e3,-0.00true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=20 "invalid character 't' after array element" | decoder=syntax offset=20 "invalid character 't' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0-true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=19 "invalid character '-' after array element" | decoder=syntax offset=19 "invalid character '-' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0.true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=19 "invalid character '.' after array element" | decoder=syntax offset=19 "invalid character '.' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0etrue,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=20 "invalid character 't' in exponent of numeric literal" | decoder=syntax offset=20 "invalid character 't' in exponent of numeric literal" off=0
// "{\"a\":[1,2.5e3,-0.0\x00true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=19 "invalid character '\\x00' after array element" | decoder=syntax offset=19 "invalid character '\\x00' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0\ntrue,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=20 "invalid character 't' after array element" | decoder=syntax offset=20 "invalid character 't' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,tr}e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character '}' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character '}' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr]e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character ']' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character ']' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr,e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character ',' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character ',' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr:e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character ':' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character ':' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr\"e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character '\"' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character '\"' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr\\e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character '\\\\' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character '\\\\' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,trxe,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character 'x' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character 'x' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr0e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character '0' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character '0' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr-e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character '-' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character '-' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr.e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character '.' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character '.' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tree,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character 'e' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character 'e' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr\x00e,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character '\\x00' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character '\\x00' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,tr\ne,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=22 "invalid character '\\n' in literal true (expecting 'u')" | decoder=syntax offset=22 "invalid character '\\n' in literal true (expecting 'u')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,}alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character '}' looking for beginning of value" | decoder=syntax offset=25 "invalid character '}' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,]alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character ']' looking for beginning of value" | decoder=syntax offset=25 "invalid character ']' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,,alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character ',' looking for beginning of value" | decoder=syntax offset=25 "invalid character ',' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,:alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character ':' looking for beginning of value" | decoder=syntax offset=25 "invalid character ':' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,\"alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after array element" | decoder=syntax offset=37 "invalid character 's' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,\\alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=25 "invalid character '\\\\' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,xalse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=25 "invalid character 'x' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,0alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=26 "invalid character 'a' after array element" | decoder=syntax offset=26 "invalid character 'a' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,-alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=26 "invalid character 'a' in numeric literal" | decoder=syntax offset=26 "invalid character 'a' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,.alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character '.' looking for beginning of value" | decoder=syntax offset=25 "invalid character '.' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,ealse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=25 "invalid character 'e' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,\x00alse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=25 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=25 "invalid character '\\x00' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,\nalse,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=26 "invalid character 'a' looking for beginning of value" | decoder=syntax offset=26 "invalid character 'a' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal}e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character '}' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character '}' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal]e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character ']' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character ']' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal,e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character ',' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character ',' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal:e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character ':' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character ':' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal\"e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character '\"' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character '\"' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal\\e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character '\\\\' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character '\\\\' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,falxe,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character 'x' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character 'x' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal0e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character '0' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character '0' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal-e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character '-' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character '-' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal.e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character '.' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character '.' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,falee,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character 'e' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character 'e' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal\x00e,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character '\\x00' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character '\\x00' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,fal\ne,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=28 "invalid character '\\n' in literal false (expecting 's')" | decoder=syntax offset=28 "invalid character '\\n' in literal false (expecting 's')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,}ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character '}' looking for beginning of value" | decoder=syntax offset=31 "invalid character '}' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,]ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character ']' looking for beginning of value" | decoder=syntax offset=31 "invalid character ']' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,,ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character ',' looking for beginning of value" | decoder=syntax offset=31 "invalid character ',' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,:ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character ':' looking for beginning of value" | decoder=syntax offset=31 "invalid character ':' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,\"ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character 's' after array element" | decoder=syntax offset=37 "invalid character 's' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,\\ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=31 "invalid character '\\\\' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,xull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=31 "invalid character 'x' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,0ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=32 "invalid character 'u' after array element" | decoder=syntax offset=32 "invalid character 'u' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,-ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=32 "invalid character 'u' in numeric literal" | decoder=syntax offset=32 "invalid character 'u' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,.ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character '.' looking for beginning of value" | decoder=syntax offset=31 "invalid character '.' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,eull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=31 "invalid character 'e' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,\x00ull,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=31 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=31 "invalid character '\\x00' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,\null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=32 "invalid character 'u' looking for beginning of value" | decoder=syntax offset=32 "invalid character 'u' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul},\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character '}' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character '}' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul],\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character ']' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character ']' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul,,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character ',' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character ',' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul:,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character ':' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character ':' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul\",\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character '\"' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character '\"' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul\\,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character '\\\\' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character '\\\\' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nulx,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character 'x' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character 'x' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul0,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character '0' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character '0' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul-,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character '-' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character '-' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul.,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character '.' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character '.' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nule,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character 'e' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character 'e' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul\x00,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character '\\x00' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character '\\x00' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,nul\n,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=34 "invalid character '\\n' in literal null (expecting 'l')" | decoder=syntax offset=34 "invalid character '\\n' in literal null (expecting 'l')" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"}\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"]\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\",\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\":\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"\"\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=38 "invalid character '\\\\' after array element" | decoder=syntax offset=38 "invalid character '\\\\' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"\\\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"x\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"0\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"-\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\".\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"e\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"\x00\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character '\\x00' in string literal" | decoder=syntax offset=37 "invalid character '\\x00' in string literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"\n\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=37 "invalid character '\\n' in string literal" | decoder=syntax offset=37 "invalid character '\\n' in string literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u}0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character '}' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '}' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u]0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character ']' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character ']' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u,0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character ',' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character ',' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u:0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character ':' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character ':' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u\"0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character '\"' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '\"' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u\\0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character '\\\\' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '\\\\' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\ux0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character 'x' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character 'x' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u-0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character '-' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '-' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u.0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character '.' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '.' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\ue0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u\x000e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character '\\x00' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '\\x00' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u\n0e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=40 "invalid character '\\n' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '\\n' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e}\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character '}' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character '}' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e]\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character ']' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character ']' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e,\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character ',' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character ',' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e:\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character ':' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character ':' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e\"\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character '\"' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character '\"' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e\\\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character '\\\\' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character '\\\\' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00ex\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character 'x' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character 'x' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e0\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e-\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character '-' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character '-' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e.\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character '.' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character '.' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00ee\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e\x00\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character '\\x00' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character '\\x00' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e\n\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=43 "invalid character '\\n' in \\u hexadecimal character escape" | decoder=syntax offset=43 "invalid character '\\n' in \\u hexadecimal character escape" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n}],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n]],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n,],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n:],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\\],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=47 "invalid character ']' in string escape code" | decoder=syntax offset=47 "invalid character ']' in string escape code" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\nx],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n0],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n-],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n.],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\ne],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' after array element" | decoder=syntax offset=50 "invalid character 'b' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\x00],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=46 "invalid character '\\x00' in string literal" | decoder=syntax offset=46 "invalid character '\\x00' in string literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\n],\"b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=46 "invalid character '\\n' in string literal" | decoder=syntax offset=46 "invalid character '\\n' in string literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],}b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character '}' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character '}' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],]b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character ']' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character ']' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],,b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character ',' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character ',' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],:b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character ':' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character ':' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\\b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character '\\\\' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character '\\\\' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],xb\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character 'x' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character 'x' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],0b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character '0' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character '0' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],-b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character '-' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character '-' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],.b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character '.' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character '.' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],eb\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character 'e' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character 'e' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\x00b\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=49 "invalid character '\\x00' looking for beginning of object key string" | decoder=syntax offset=49 "invalid character '\\x00' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\nb\":{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=50 "invalid character 'b' looking for beginning of object key string" | decoder=syntax offset=50 "invalid character 'b' looking for beginning of object key string" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"}{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character '}' after object key" | decoder=syntax offset=52 "invalid character '}' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"]{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character ']' after object key" | decoder=syntax offset=52 "invalid character ']' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\",{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character ',' after object key" | decoder=syntax offset=52 "invalid character ',' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"\"{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character '\"' after object key" | decoder=syntax offset=52 "invalid character '\"' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"\\{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character '\\\\' after object key" | decoder=syntax offset=52 "invalid character '\\\\' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"x{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character 'x' after object key" | decoder=syntax offset=52 "invalid character 'x' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"0{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character '0' after object key" | decoder=syntax offset=52 "invalid character '0' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"-{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character '-' after object key" | decoder=syntax offset=52 "invalid character '-' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\".{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character '.' after object key" | decoder=syntax offset=52 "invalid character '.' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"e{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character 'e' after object key" | decoder=syntax offset=52 "invalid character 'e' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"\x00{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=52 "invalid character '\\x00' after object key" | decoder=syntax offset=52 "invalid character '\\x00' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\"\n{\"c\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=53 "invalid character '{' after object key" | decoder=syntax offset=53 "invalid character '{' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"}\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"]\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\",\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\":\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"\"\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=56 "invalid character '\"' after object key" | decoder=syntax offset=56 "invalid character '\"' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"\\\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=63 "invalid character 'd' after object key" | decoder=syntax offset=63 "invalid character 'd' after object key" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"x\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"0\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"-\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\".\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"e\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"\x00\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=55 "invalid character '\\x00' in string literal" | decoder=syntax offset=55 "invalid character '\\x00' in string literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"\n\":{}},\"d\":[]}" valid=false unmarshal=syntax offset=55 "invalid character '\\n' in string literal" | decoder=syntax offset=55 "invalid character '\\n' in string literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":}}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character '}' looking for beginning of value" | decoder=syntax offset=58 "invalid character '}' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":]}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character ']' looking for beginning of value" | decoder=syntax offset=58 "invalid character ']' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":,}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character ',' looking for beginning of value" | decoder=syntax offset=58 "invalid character ',' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\"::}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character ':' looking for beginning of value" | decoder=syntax offset=58 "invalid character ':' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":\"}},\"d\":[]}" valid=false unmarshal=syntax offset=63 "invalid character 'd' after object key:value pair" | decoder=syntax offset=63 "invalid character 'd' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":\\}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=58 "invalid character '\\\\' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":x}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=58 "invalid character 'x' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":0}},\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character ',' after top-level value" | decoder=ok off=60
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":-}},\"d\":[]}" valid=false unmarshal=syntax offset=59 "invalid character '}' in numeric literal" | decoder=syntax offset=59 "invalid character '}' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":.}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character '.' looking for beginning of value" | decoder=syntax offset=58 "invalid character '.' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":e}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=58 "invalid character 'e' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":\x00}},\"d\":[]}" valid=false unmarshal=syntax offset=58 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=58 "invalid character '\\x00' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":\n}},\"d\":[]}" valid=false unmarshal=syntax offset=59 "invalid character '}' looking for beginning of value" | decoder=syntax offset=59 "invalid character '}' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}}\"d\":[]}" valid=false unmarshal=syntax offset=62 "invalid character '\"' after top-level value" | decoder=ok off=61
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}]\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character ']' after object key:value pair" | decoder=syntax offset=61 "invalid character ']' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}:\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character ':' after object key:value pair" | decoder=syntax offset=61 "invalid character ':' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}\"\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character '\"' after object key:value pair" | decoder=syntax offset=61 "invalid character '\"' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}\\\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character '\\\\' after object key:value pair" | decoder=syntax offset=61 "invalid character '\\\\' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}x\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character 'x' after object key:value pair" | decoder=syntax offset=61 "invalid character 'x' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}0\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character '0' after object key:value pair" | decoder=syntax offset=61 "invalid character '0' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}-\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character '-' after object key:value pair" | decoder=syntax offset=61 "invalid character '-' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}.\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character '.' after object key:value pair" | decoder=syntax offset=61 "invalid character '.' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}e\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character 'e' after object key:value pair" | decoder=syntax offset=61 "invalid character 'e' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}\x00\"d\":[]}" valid=false unmarshal=syntax offset=61 "invalid character '\\x00' after object key:value pair" | decoder=syntax offset=61 "invalid character '\\x00' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}}\n\"d\":[]}" valid=false unmarshal=syntax offset=62 "invalid character '\"' after object key:value pair" | decoder=syntax offset=62 "invalid character '\"' after object key:value pair" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d}:[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d]:[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d,:[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d::[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\\:[]}" valid=false unmarshal=syntax offset=65 "invalid character ':' in string escape code" | decoder=syntax offset=65 "invalid character ':' in string escape code" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"dx:[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d0:[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d-:[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d.:[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"de:[]}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\x00:[]}" valid=false unmarshal=syntax offset=64 "invalid character '\\x00' in string literal" | decoder=syntax offset=64 "invalid character '\\x00' in string literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\n:[]}" valid=false unmarshal=syntax offset=64 "invalid character '\\n' in string literal" | decoder=syntax offset=64 "invalid character '\\n' in string literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[}}" valid=false unmarshal=syntax offset=67 "invalid character '}' looking for beginning of value" | decoder=syntax offset=67 "invalid character '}' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[]}" valid=true  unmarshal=ok | decoder=ok off=68
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[,}" valid=false unmarshal=syntax offset=67 "invalid character ',' looking for beginning of value" | decoder=syntax offset=67 "invalid character ',' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[:}" valid=false unmarshal=syntax offset=67 "invalid character ':' looking for beginning of value" | decoder=syntax offset=67 "invalid character ':' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[\"}" valid=false unmarshal=syntax offset=68 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[\\}" valid=false unmarshal=syntax offset=67 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=67 "invalid character '\\\\' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[x}" valid=false unmarshal=syntax offset=67 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=67 "invalid character 'x' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[0}" valid=false unmarshal=syntax offset=68 "invalid character '}' after array element" | decoder=syntax offset=68 "invalid character '}' after array element" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[-}" valid=false unmarshal=syntax offset=68 "invalid character '}' in numeric literal" | decoder=syntax offset=68 "invalid character '}' in numeric literal" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[.}" valid=false unmarshal=syntax offset=67 "invalid character '.' looking for beginning of value" | decoder=syntax offset=67 "invalid character '.' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[e}" valid=false unmarshal=syntax offset=67 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=67 "invalid character 'e' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[\x00}" valid=false unmarshal=syntax offset=67 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=67 "invalid character '\\x00' looking for beginning of value" off=0
// "{\"a\":[1,2.5e3,-0.0,true,false,null,\"s\\u00e9\\n\"],\"b\":{\"c\":{}},\"d\":[\n}" valid=false unmarshal=syntax offset=68 "invalid character '}' looking for beginning of value" | decoder=syntax offset=68 "invalid character '}' looking for beginning of value" off=0
// -- document 1
// "}[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character '}' looking for beginning of value" | decoder=syntax offset=1 "invalid character '}' looking for beginning of value" off=0
// "][],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character ']' looking for beginning of value" | decoder=syntax offset=1 "invalid character ']' looking for beginning of value" off=0
// ",[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character ',' looking for beginning of value" | decoder=syntax offset=1 "invalid character ',' looking for beginning of value" off=0
// ":[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character ':' looking for beginning of value" | decoder=syntax offset=1 "invalid character ':' looking for beginning of value" off=0
// "\"[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=12 "invalid character 'k' after top-level value" | decoder=ok off=11
// "\\[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=1 "invalid character '\\\\' looking for beginning of value" off=0
// "x[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=1 "invalid character 'x' looking for beginning of value" off=0
// "0[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=2 "invalid character '[' after top-level value" | decoder=ok off=1
// "-[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=2 "invalid character '[' in numeric literal" | decoder=syntax offset=2 "invalid character '[' in numeric literal" off=0
// ".[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character '.' looking for beginning of value" | decoder=syntax offset=1 "invalid character '.' looking for beginning of value" off=0
// "e[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=1 "invalid character 'e' looking for beginning of value" off=0
// "\x00[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=1 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=1 "invalid character '\\x00' looking for beginning of value" off=0
// "\n[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character ',' after top-level value" | decoder=ok off=3
// "[[]}[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character '}' after array element" | decoder=syntax offset=4 "invalid character '}' after array element" off=0
// "[[]][[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=5 "invalid character '[' after top-level value" | decoder=ok off=4
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[]:[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character ':' after array element" | decoder=syntax offset=4 "invalid character ':' after array element" off=0
// "[[]\"[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character '\"' after array element" | decoder=syntax offset=4 "invalid character '\"' after array element" off=0
// "[[]\\[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character '\\\\' after array element" | decoder=syntax offset=4 "invalid character '\\\\' after array element" off=0
// "[[]x[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character 'x' after array element" | decoder=syntax offset=4 "invalid character 'x' after array element" off=0
// "[[]0[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character '0' after array element" | decoder=syntax offset=4 "invalid character '0' after array element" off=0
// "[[]-[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character '-' after array element" | decoder=syntax offset=4 "invalid character '-' after array element" off=0
// "[[].[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character '.' after array element" | decoder=syntax offset=4 "invalid character '.' after array element" off=0
// "[[]e[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character 'e' after array element" | decoder=syntax offset=4 "invalid character 'e' after array element" off=0
// "[[]\x00[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=4 "invalid character '\\x00' after array element" | decoder=syntax offset=4 "invalid character '\\x00' after array element" off=0
// "[[]\n[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=5 "invalid character '[' after array element" | decoder=syntax offset=5 "invalid character '[' after array element" off=0
// "[[],[[}],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=7 "invalid character '}' looking for beginning of value" | decoder=syntax offset=7 "invalid character '}' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[,],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=7 "invalid character ',' looking for beginning of value" | decoder=syntax offset=7 "invalid character ',' looking for beginning of value" off=0
// "[[],[[:],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=7 "invalid character ':' looking for beginning of value" | decoder=syntax offset=7 "invalid character ':' looking for beginning of value" off=0
// "[[],[[\"],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=12 "invalid character 'k' after array element" | decoder=syntax offset=12 "invalid character 'k' after array element" off=0
// "[[],[[\\],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=7 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=7 "invalid character '\\\\' looking for beginning of value" off=0
// "[[],[[x],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=7 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=7 "invalid character 'x' looking for beginning of value" off=0
// "[[],[[0],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=34 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[-],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=8 "invalid character ']' in numeric literal" | decoder=syntax offset=8 "invalid character ']' in numeric literal" off=0
// "[[],[[.],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=7 "invalid character '.' looking for beginning of value" | decoder=syntax offset=7 "invalid character '.' looking for beginning of value" off=0
// "[[],[[e],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=7 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=7 "invalid character 'e' looking for beginning of value" off=0
// "[[],[[\x00],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=7 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=7 "invalid character '\\x00' looking for beginning of value" off=0
// "[[],[[\n],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=34 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],}\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character '}' looking for beginning of value" | decoder=syntax offset=10 "invalid character '}' looking for beginning of value" off=0
// "[[],[[]],]\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character ']' looking for beginning of value" | decoder=syntax offset=10 "invalid character ']' looking for beginning of value" off=0
// "[[],[[]],,\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character ',' looking for beginning of value" | decoder=syntax offset=10 "invalid character ',' looking for beginning of value" off=0
// "[[],[[]],:\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character ':' looking for beginning of value" | decoder=syntax offset=10 "invalid character ':' looking for beginning of value" off=0
// "[[],[[]],\"\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=12 "invalid character 'k' after array element" | decoder=syntax offset=12 "invalid character 'k' after array element" off=0
// "[[],[[]],\\\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=10 "invalid character '\\\\' looking for beginning of value" off=0
// "[[],[[]],x\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=10 "invalid character 'x' looking for beginning of value" off=0
// "[[],[[]],0\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=11 "invalid character '\"' after array element" | decoder=syntax offset=11 "invalid character '\"' after array element" off=0
// "[[],[[]],-\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=11 "invalid character '\"' in numeric literal" | decoder=syntax offset=11 "invalid character '\"' in numeric literal" off=0
// "[[],[[]],.\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character '.' looking for beginning of value" | decoder=syntax offset=10 "invalid character '.' looking for beginning of value" off=0
// "[[],[[]],e\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=10 "invalid character 'e' looking for beginning of value" off=0
// "[[],[[]],\x00\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=10 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=10 "invalid character '\\x00' looking for beginning of value" off=0
// "[[],[[]],\n\"k\":[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=14 "invalid character ':' after array element" | decoder=syntax offset=14 "invalid character ':' after array element" off=0
// "[[],[[]],{\"k}:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"k]:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"k,:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"k::[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\\:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=14 "invalid character ':' in string escape code" | decoder=syntax offset=14 "invalid character ':' in string escape code" off=0
// "[[],[[]],{\"kx:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"k0:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"k-:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"k.:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"ke:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after object key" | decoder=syntax offset=18 "invalid character 'x' after object key" off=0
// "[[],[[]],{\"k\x00:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=13 "invalid character '\\x00' in string literal" | decoder=syntax offset=13 "invalid character '\\x00' in string literal" off=0
// "[[],[[]],{\"k\n:[{\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=13 "invalid character '\\n' in string literal" | decoder=syntax offset=13 "invalid character '\\n' in string literal" off=0
// "[[],[[]],{\"k\":[}\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=16 "invalid character '}' looking for beginning of value" | decoder=syntax offset=16 "invalid character '}' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[]\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=17 "invalid character '\"' after object key:value pair" | decoder=syntax offset=17 "invalid character '\"' after object key:value pair" off=0
// "[[],[[]],{\"k\":[,\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=16 "invalid character ',' looking for beginning of value" | decoder=syntax offset=16 "invalid character ',' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[:\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=16 "invalid character ':' looking for beginning of value" | decoder=syntax offset=16 "invalid character ':' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[\"\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=18 "invalid character 'x' after array element" | decoder=syntax offset=18 "invalid character 'x' after array element" off=0
// "[[],[[]],{\"k\":[\\\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=16 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=16 "invalid character '\\\\' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[x\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=16 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=16 "invalid character 'x' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[0\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=17 "invalid character '\"' after array element" | decoder=syntax offset=17 "invalid character '\"' after array element" off=0
// "[[],[[]],{\"k\":[-\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=17 "invalid character '\"' in numeric literal" | decoder=syntax offset=17 "invalid character '\"' in numeric literal" off=0
// "[[],[[]],{\"k\":[.\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=16 "invalid character '.' looking for beginning of value" | decoder=syntax offset=16 "invalid character '.' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[e\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=16 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=16 "invalid character 'e' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[\x00\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=16 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=16 "invalid character '\\x00' looking for beginning of value" off=0
// "[[],[[]],{\"k\":[\n\"x\":1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=20 "invalid character ':' after array element" | decoder=syntax offset=20 "invalid character ':' after array element" off=0
// "[[],[[]],{\"k\":[{\"x}:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"x]:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"x,:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"x::1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\\:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=20 "invalid character ':' in string escape code" | decoder=syntax offset=20 "invalid character ':' in string escape code" off=0
// "[[],[[]],{\"k\":[{\"xx:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"x0:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"x-:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"x.:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"xe:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=28 "invalid character 't' after object key" | decoder=syntax offset=28 "invalid character 't' after object key" off=0
// "[[],[[]],{\"k\":[{\"x\x00:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=19 "invalid character '\\x00' in string literal" | decoder=syntax offset=19 "invalid character '\\x00' in string literal" off=0
// "[[],[[]],{\"k\":[{\"x\n:1}]}, \"tail\" ]" valid=false unmarshal=syntax offset=19 "invalid character '\\n' in string literal" | decoder=syntax offset=19 "invalid character '\\n' in string literal" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1]]}, \"tail\" ]" valid=false unmarshal=syntax offset=22 "invalid character ']' after object key:value pair" | decoder=syntax offset=22 "invalid character ']' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":1,]}, \"tail\" ]" valid=false unmarshal=syntax offset=23 "invalid character ']' looking for beginning of object key string" | decoder=syntax offset=23 "invalid character ']' looking for beginning of object key string" off=0
// "[[],[[]],{\"k\":[{\"x\":1:]}, \"tail\" ]" valid=false unmarshal=syntax offset=22 "invalid character ':' after object key:value pair" | decoder=syntax offset=22 "invalid character ':' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":1\"]}, \"tail\" ]" valid=false unmarshal=syntax offset=22 "invalid character '\"' after object key:value pair" | decoder=syntax offset=22 "invalid character '\"' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":1\\]}, \"tail\" ]" valid=false unmarshal=syntax offset=22 "invalid character '\\\\' after object key:value pair" | decoder=syntax offset=22 "invalid character '\\\\' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":1x]}, \"tail\" ]" valid=false unmarshal=syntax offset=22 "invalid character 'x' after object key:value pair" | decoder=syntax offset=22 "invalid character 'x' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":10]}, \"tail\" ]" valid=false unmarshal=syntax offset=23 "invalid character ']' after object key:value pair" | decoder=syntax offset=23 "invalid character ']' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":1-]}, \"tail\" ]" valid=false unmarshal=syntax offset=22 "invalid character '-' after object key:value pair" | decoder=syntax offset=22 "invalid character '-' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":1.]}, \"tail\" ]" valid=false unmarshal=syntax offset=23 "invalid character ']' after decimal point in numeric literal" | decoder=syntax offset=23 "invalid character ']' after decimal point in numeric literal" off=0
// "[[],[[]],{\"k\":[{\"x\":1e]}, \"tail\" ]" valid=false unmarshal=syntax offset=23 "invalid character ']' in exponent of numeric literal" | decoder=syntax offset=23 "invalid character ']' in exponent of numeric literal" off=0
// "[[],[[]],{\"k\":[{\"x\":1\x00]}, \"tail\" ]" valid=false unmarshal=syntax offset=22 "invalid character '\\x00' after object key:value pair" | decoder=syntax offset=22 "invalid character '\\x00' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":1\n]}, \"tail\" ]" valid=false unmarshal=syntax offset=23 "invalid character ']' after object key:value pair" | decoder=syntax offset=23 "invalid character ']' after object key:value pair" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}} \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character '}' after array element" | decoder=syntax offset=25 "invalid character '}' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}] \"tail\" ]" valid=false unmarshal=syntax offset=27 "invalid character '\"' after top-level value" | decoder=ok off=25
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}: \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character ':' after array element" | decoder=syntax offset=25 "invalid character ':' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}\" \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character '\"' after array element" | decoder=syntax offset=25 "invalid character '\"' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}\\ \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character '\\\\' after array element" | decoder=syntax offset=25 "invalid character '\\\\' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}x \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character 'x' after array element" | decoder=syntax offset=25 "invalid character 'x' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}0 \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character '0' after array element" | decoder=syntax offset=25 "invalid character '0' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}- \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character '-' after array element" | decoder=syntax offset=25 "invalid character '-' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}. \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character '.' after array element" | decoder=syntax offset=25 "invalid character '.' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}e \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character 'e' after array element" | decoder=syntax offset=25 "invalid character 'e' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}\x00 \"tail\" ]" valid=false unmarshal=syntax offset=25 "invalid character '\\x00' after array element" | decoder=syntax offset=25 "invalid character '\\x00' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}\n \"tail\" ]" valid=false unmarshal=syntax offset=27 "invalid character '\"' after array element" | decoder=syntax offset=27 "invalid character '\"' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"}ail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"]ail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \",ail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \":ail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"\"ail\" ]" valid=false unmarshal=syntax offset=29 "invalid character 'a' after array element" | decoder=syntax offset=29 "invalid character 'a' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"\\ail\" ]" valid=false unmarshal=syntax offset=29 "invalid character 'a' in string escape code" | decoder=syntax offset=29 "invalid character 'a' in string escape code" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"xail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"0ail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"-ail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \".ail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"eail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"\x00ail\" ]" valid=false unmarshal=syntax offset=28 "invalid character '\\x00' in string literal" | decoder=syntax offset=28 "invalid character '\\x00' in string literal" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"\nail\" ]" valid=false unmarshal=syntax offset=28 "invalid character '\\n' in string literal" | decoder=syntax offset=28 "invalid character '\\n' in string literal" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai}\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai]\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai,\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai:\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai\"\" ]" valid=false unmarshal=syntax offset=32 "invalid character '\"' after array element" | decoder=syntax offset=32 "invalid character '\"' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai\\\" ]" valid=false unmarshal=syntax offset=34 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"taix\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai0\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai-\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai.\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"taie\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai\x00\" ]" valid=false unmarshal=syntax offset=31 "invalid character '\\x00' in string literal" | decoder=syntax offset=31 "invalid character '\\x00' in string literal" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tai\n\" ]" valid=false unmarshal=syntax offset=31 "invalid character '\\n' in string literal" | decoder=syntax offset=31 "invalid character '\\n' in string literal" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" }" valid=false unmarshal=syntax offset=34 "invalid character '}' after array element" | decoder=syntax offset=34 "invalid character '}' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ]" valid=true  unmarshal=ok | decoder=ok off=34
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ," valid=false unmarshal=syntax offset=34 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" :" valid=false unmarshal=syntax offset=34 "invalid character ':' after array element" | decoder=syntax offset=34 "invalid character ':' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" \"" valid=false unmarshal=syntax offset=34 "invalid character '\"' after array element" | decoder=syntax offset=34 "invalid character '\"' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" \\" valid=false unmarshal=syntax offset=34 "invalid character '\\\\' after array element" | decoder=syntax offset=34 "invalid character '\\\\' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" x" valid=false unmarshal=syntax offset=34 "invalid character 'x' after array element" | decoder=syntax offset=34 "invalid character 'x' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" 0" valid=false unmarshal=syntax offset=34 "invalid character '0' after array element" | decoder=syntax offset=34 "invalid character '0' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" -" valid=false unmarshal=syntax offset=34 "invalid character '-' after array element" | decoder=syntax offset=34 "invalid character '-' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" ." valid=false unmarshal=syntax offset=34 "invalid character '.' after array element" | decoder=syntax offset=34 "invalid character '.' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" e" valid=false unmarshal=syntax offset=34 "invalid character 'e' after array element" | decoder=syntax offset=34 "invalid character 'e' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" \x00" valid=false unmarshal=syntax offset=34 "invalid character '\\x00' after array element" | decoder=syntax offset=34 "invalid character '\\x00' after array element" off=0
// "[[],[[]],{\"k\":[{\"x\":1}]}, \"tail\" \n" valid=false unmarshal=syntax offset=34 "unexpected end of JSON input" | decoder=*errors.errorString "unexpected EOF" off=0
// -- document 2
// "}plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character '}' looking for beginning of value" | decoder=syntax offset=1 "invalid character '}' looking for beginning of value" off=0
// "]plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character ']' looking for beginning of value" | decoder=syntax offset=1 "invalid character ']' looking for beginning of value" off=0
// ",plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character ',' looking for beginning of value" | decoder=syntax offset=1 "invalid character ',' looking for beginning of value" off=0
// ":plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character ':' looking for beginning of value" | decoder=syntax offset=1 "invalid character ':' looking for beginning of value" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\\plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character '\\\\' looking for beginning of value" | decoder=syntax offset=1 "invalid character '\\\\' looking for beginning of value" off=0
// "xplain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character 'x' looking for beginning of value" | decoder=syntax offset=1 "invalid character 'x' looking for beginning of value" off=0
// "0plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=2 "invalid character 'p' after top-level value" | decoder=ok off=1
// "-plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=2 "invalid character 'p' in numeric literal" | decoder=syntax offset=2 "invalid character 'p' in numeric literal" off=0
// ".plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character '.' looking for beginning of value" | decoder=syntax offset=1 "invalid character '.' looking for beginning of value" off=0
// "eplain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character 'e' looking for beginning of value" | decoder=syntax offset=1 "invalid character 'e' looking for beginning of value" off=0
// "\x00plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=1 "invalid character '\\x00' looking for beginning of value" | decoder=syntax offset=1 "invalid character '\\x00' looking for beginning of value" off=0
// "\nplain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=2 "invalid character 'p' looking for beginning of value" | decoder=syntax offset=2 "invalid character 'p' looking for beginning of value" off=0
// "\"pl}in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"pl]in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"pl,in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"pl:in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"pl\"in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=5 "invalid character 'i' after top-level value" | decoder=ok off=4
// "\"pl\\in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=5 "invalid character 'i' in string escape code" | decoder=syntax offset=5 "invalid character 'i' in string escape code" off=0
// "\"plxin \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"pl0in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"pl-in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"pl.in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plein \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"pl\x00in \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=4 "invalid character '\\x00' in string literal" | decoder=syntax offset=4 "invalid character '\\x00' in string literal" off=0
// "\"pl\nin \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=4 "invalid character '\\n' in string literal" | decoder=syntax offset=4 "invalid character '\\n' in string literal" off=0
// "\"plain}\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain]\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain,\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain:\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain\"\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=8 "invalid character '\\\\' after top-level value" | decoder=ok off=7
// "\"plain\\\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=10 "invalid character 'q' after top-level value" | decoder=ok off=9
// "\"plainx\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain0\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain-\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain.\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plaine\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain\x00\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=7 "invalid character '\\x00' in string literal" | decoder=syntax offset=7 "invalid character '\\x00' in string literal" off=0
// "\"plain\n\\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=7 "invalid character '\\n' in string literal" | decoder=syntax offset=7 "invalid character '\\n' in string literal" off=0
// "\"plain \\\"}uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"]uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\",uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\":uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"\"uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=11 "invalid character 'u' after top-level value" | decoder=ok off=10
// "\"plain \\\"\\uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=12 "invalid character 'o' in \\u hexadecimal character escape" | decoder=syntax offset=12 "invalid character 'o' in \\u hexadecimal character escape" off=0
// "\"plain \\\"xuoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"0uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"-uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\".uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"euoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"\x00uoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=10 "invalid character '\\x00' in string literal" | decoder=syntax offset=10 "invalid character '\\x00' in string literal" off=0
// "\"plain \\\"\nuoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=10 "invalid character '\\n' in string literal" | decoder=syntax offset=10 "invalid character '\\n' in string literal" off=0
// "\"plain \\\"quo}ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quo]ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quo,ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quo:ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quo\"ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=14 "invalid character 'e' after top-level value" | decoder=ok off=13
// "\"plain \\\"quo\\ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=14 "invalid character 'e' in string escape code" | decoder=syntax offset=14 "invalid character 'e' in string escape code" off=0
// "\"plain \\\"quoxed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quo0ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quo-ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quo.ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoeed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quo\x00ed\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=13 "invalid character '\\x00' in string literal" | decoder=syntax offset=13 "invalid character '\\x00' in string literal" off=0
// "\"plain \\\"quo\ned\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=13 "invalid character '\\n' in string literal" | decoder=syntax offset=13 "invalid character '\\n' in string literal" off=0
// "\"plain \\\"quoted}\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quoted]\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quoted,\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quoted:\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quoted\"\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=17 "invalid character '\"' after top-level value" | decoder=ok off=16
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quotedx\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quoted0\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quoted-\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quoted.\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quotede\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\\\' after top-level value" | decoder=ok off=17
// "\"plain \\\"quoted\x00\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=16 "invalid character '\\x00' in string literal" | decoder=syntax offset=16 "invalid character '\\x00' in string literal" off=0
// "\"plain \\\"quoted\n\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=16 "invalid character '\\n' in string literal" | decoder=syntax offset=16 "invalid character '\\n' in string literal" off=0
// "\"plain \\\"quoted\\\" }\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" ]\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" ,\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" :\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" \"\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=20 "invalid character '\\\\' after top-level value" | decoder=ok off=19
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" x\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" 0\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" -\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" .\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" e\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=21 "invalid character ' ' in string escape code" | decoder=syntax offset=21 "invalid character ' ' in string escape code" off=0
// "\"plain \\\"quoted\\\" \x00\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\x00' in string literal" | decoder=syntax offset=19 "invalid character '\\x00' in string literal" off=0
// "\"plain \\\"quoted\\\" \n\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=19 "invalid character '\\n' in string literal" | decoder=syntax offset=19 "invalid character '\\n' in string literal" off=0
// "\"plain \\\"quoted\\\" \\\\ }/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ ]/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ ,/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ :/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \"/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=23 "invalid character '/' after top-level value" | decoder=ok off=22
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ x/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ 0/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ -/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ ./ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ e/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \x00/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=22 "invalid character '\\x00' in string literal" | decoder=syntax offset=22 "invalid character '\\x00' in string literal" off=0
// "\"plain \\\"quoted\\\" \\\\ \n/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=22 "invalid character '\\n' in string literal" | decoder=syntax offset=22 "invalid character '\\n' in string literal" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ }b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ ]b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ ,b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ :b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \"b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=26 "invalid character 'b' after top-level value" | decoder=ok off=25
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ xb\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ 0b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ -b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ .b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ eb\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \x00b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=25 "invalid character '\\x00' in string literal" | decoder=syntax offset=25 "invalid character '\\x00' in string literal" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \nb\\f\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=25 "invalid character '\\n' in string literal" | decoder=syntax offset=25 "invalid character '\\n' in string literal" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\}\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character '}' in string escape code" | decoder=syntax offset=28 "invalid character '}' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\]\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character ']' in string escape code" | decoder=syntax offset=28 "invalid character ']' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\,\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character ',' in string escape code" | decoder=syntax offset=28 "invalid character ',' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\:\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character ':' in string escape code" | decoder=syntax offset=28 "invalid character ':' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\\"\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\\\\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\x\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character 'x' in string escape code" | decoder=syntax offset=28 "invalid character 'x' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\0\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character '0' in string escape code" | decoder=syntax offset=28 "invalid character '0' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\-\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character '-' in string escape code" | decoder=syntax offset=28 "invalid character '-' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\.\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character '.' in string escape code" | decoder=syntax offset=28 "invalid character '.' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\e\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character 'e' in string escape code" | decoder=syntax offset=28 "invalid character 'e' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\\x00\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character '\\x00' in string escape code" | decoder=syntax offset=28 "invalid character '\\x00' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\\n\\n\\r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=28 "invalid character '\\n' in string escape code" | decoder=syntax offset=28 "invalid character '\\n' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n}r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n]r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n,r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n:r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\"r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=32 "invalid character 'r' after top-level value" | decoder=ok off=31
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\nxr\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n0r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n-r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n.r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\ner\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\x00r\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=31 "invalid character '\\x00' in string literal" | decoder=syntax offset=31 "invalid character '\\x00' in string literal" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\nr\\t \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=31 "invalid character '\\n' in string literal" | decoder=syntax offset=31 "invalid character '\\n' in string literal" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\} \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character '}' in string escape code" | decoder=syntax offset=34 "invalid character '}' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\] \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character ']' in string escape code" | decoder=syntax offset=34 "invalid character ']' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\, \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character ',' in string escape code" | decoder=syntax offset=34 "invalid character ',' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\: \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character ':' in string escape code" | decoder=syntax offset=34 "invalid character ':' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\\" \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\\\ \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\x \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character 'x' in string escape code" | decoder=syntax offset=34 "invalid character 'x' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\0 \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character '0' in string escape code" | decoder=syntax offset=34 "invalid character '0' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\- \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character '-' in string escape code" | decoder=syntax offset=34 "invalid character '-' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\. \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character '.' in string escape code" | decoder=syntax offset=34 "invalid character '.' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\e \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character 'e' in string escape code" | decoder=syntax offset=34 "invalid character 'e' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\\x00 \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character '\\x00' in string escape code" | decoder=syntax offset=34 "invalid character '\\x00' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\\n \\ud83d\\ude00\"" valid=false unmarshal=syntax offset=34 "invalid character '\\n' in string escape code" | decoder=syntax offset=34 "invalid character '\\n' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\}d83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character '}' in string escape code" | decoder=syntax offset=37 "invalid character '}' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\]d83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character ']' in string escape code" | decoder=syntax offset=37 "invalid character ']' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\,d83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character ',' in string escape code" | decoder=syntax offset=37 "invalid character ',' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\:d83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character ':' in string escape code" | decoder=syntax offset=37 "invalid character ':' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\\"d83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\\\d83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\xd83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character 'x' in string escape code" | decoder=syntax offset=37 "invalid character 'x' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\0d83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character '0' in string escape code" | decoder=syntax offset=37 "invalid character '0' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\-d83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character '-' in string escape code" | decoder=syntax offset=37 "invalid character '-' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\.d83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character '.' in string escape code" | decoder=syntax offset=37 "invalid character '.' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ed83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character 'e' in string escape code" | decoder=syntax offset=37 "invalid character 'e' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\\x00d83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character '\\x00' in string escape code" | decoder=syntax offset=37 "invalid character '\\x00' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\\nd83d\\ude00\"" valid=false unmarshal=syntax offset=37 "invalid character '\\n' in string escape code" | decoder=syntax offset=37 "invalid character '\\n' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8}d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character '}' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '}' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8]d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character ']' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character ']' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8,d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character ',' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character ',' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8:d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character ':' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character ':' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8\"d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character '\"' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '\"' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8\\d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character '\\\\' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '\\\\' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8xd\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character 'x' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character 'x' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud80d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8-d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character '-' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '-' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8.d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character '.' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '.' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8ed\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8\x00d\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character '\\x00' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '\\x00' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud8\nd\\ude00\"" valid=false unmarshal=syntax offset=40 "invalid character '\\n' in \\u hexadecimal character escape" | decoder=syntax offset=40 "invalid character '\\n' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\}de00\"" valid=false unmarshal=syntax offset=43 "invalid character '}' in string escape code" | decoder=syntax offset=43 "invalid character '}' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\]de00\"" valid=false unmarshal=syntax offset=43 "invalid character ']' in string escape code" | decoder=syntax offset=43 "invalid character ']' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\,de00\"" valid=false unmarshal=syntax offset=43 "invalid character ',' in string escape code" | decoder=syntax offset=43 "invalid character ',' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\:de00\"" valid=false unmarshal=syntax offset=43 "invalid character ':' in string escape code" | decoder=syntax offset=43 "invalid character ':' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\\"de00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\\\de00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\xde00\"" valid=false unmarshal=syntax offset=43 "invalid character 'x' in string escape code" | decoder=syntax offset=43 "invalid character 'x' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\0de00\"" valid=false unmarshal=syntax offset=43 "invalid character '0' in string escape code" | decoder=syntax offset=43 "invalid character '0' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\-de00\"" valid=false unmarshal=syntax offset=43 "invalid character '-' in string escape code" | decoder=syntax offset=43 "invalid character '-' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\.de00\"" valid=false unmarshal=syntax offset=43 "invalid character '.' in string escape code" | decoder=syntax offset=43 "invalid character '.' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ede00\"" valid=false unmarshal=syntax offset=43 "invalid character 'e' in string escape code" | decoder=syntax offset=43 "invalid character 'e' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\\x00de00\"" valid=false unmarshal=syntax offset=43 "invalid character '\\x00' in string escape code" | decoder=syntax offset=43 "invalid character '\\x00' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\\nde00\"" valid=false unmarshal=syntax offset=43 "invalid character '\\n' in string escape code" | decoder=syntax offset=43 "invalid character '\\n' in string escape code" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude}0\"" valid=false unmarshal=syntax offset=46 "invalid character '}' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character '}' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude]0\"" valid=false unmarshal=syntax offset=46 "invalid character ']' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character ']' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude,0\"" valid=false unmarshal=syntax offset=46 "invalid character ',' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character ',' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude:0\"" valid=false unmarshal=syntax offset=46 "invalid character ':' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character ':' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude\"0\"" valid=false unmarshal=syntax offset=46 "invalid character '\"' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character '\"' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude\\0\"" valid=false unmarshal=syntax offset=46 "invalid character '\\\\' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character '\\\\' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\udex0\"" valid=false unmarshal=syntax offset=46 "invalid character 'x' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character 'x' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude00\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude-0\"" valid=false unmarshal=syntax offset=46 "invalid character '-' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character '-' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude.0\"" valid=false unmarshal=syntax offset=46 "invalid character '.' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character '.' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\udee0\"" valid=true  unmarshal=ok | decoder=ok off=48
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude\x000\"" valid=false unmarshal=syntax offset=46 "invalid character '\\x00' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character '\\x00' in \\u hexadecimal character escape" off=0
// "\"plain \\\"quoted\\\" \\\\ \\/ \\b\\f\\n\\r\\t \\ud83d\\ude\n0\"" valid=false unmarshal=syntax offset=46 "invalid character '\\n' in \\u hexadecimal character escape" | decoder=syntax offset=46 "invalid character '\\n' in \\u hexadecimal character escape" off=0
// == trailing data
// "1 2"                                    valid=false unmarshal=syntax offset=3 "invalid character '2' after top-level value" | decoder=ok off=1
// "{} {}"                                  valid=false unmarshal=syntax offset=4 "invalid character '{' after top-level value" | decoder=ok off=2
// "[]x"                                    valid=false unmarshal=syntax offset=3 "invalid character 'x' after top-level value" | decoder=ok off=2
// "\"a\" \"b\""                            valid=false unmarshal=syntax offset=5 "invalid character '\"' after top-level value" | decoder=ok off=3
// "null,"                                  valid=false unmarshal=syntax offset=5 "invalid character ',' after top-level value" | decoder=ok off=4
// "true false"                             valid=false unmarshal=syntax offset=6 "invalid character 'f' after top-level value" | decoder=ok off=4
// "1\n\n"                                  valid=true  unmarshal=ok | decoder=ok off=1
// " 1 "                                    valid=true  unmarshal=ok | decoder=ok off=2
// "01"                                     valid=false unmarshal=syntax offset=2 "invalid character '1' after top-level value" | decoder=ok off=1
// "1."                                     valid=false unmarshal=syntax offset=2 "invalid character ' ' after decimal point in numeric literal" | decoder=*errors.errorString "unexpected EOF" off=0
// ".5"                                     valid=false unmarshal=syntax offset=1 "invalid character '.' looking for beginning of value" | decoder=syntax offset=1 "invalid character '.' looking for beginning of value" off=0
// "+1"                                     valid=false unmarshal=syntax offset=1 "invalid character '+' looking for beginning of value" | decoder=syntax offset=1 "invalid character '+' looking for beginning of value" off=0
// "0x10"                                   valid=false unmarshal=syntax offset=2 "invalid character 'x' after top-level value" | decoder=ok off=1
// "NaN"                                    valid=false unmarshal=syntax offset=1 "invalid character 'N' looking for beginning of value" | decoder=syntax offset=1 "invalid character 'N' looking for beginning of value" off=0
// "Infinity"                               valid=false unmarshal=syntax offset=1 "invalid character 'I' looking for beginning of value" | decoder=syntax offset=1 "invalid character 'I' looking for beginning of value" off=0
// "tru"                                    valid=false unmarshal=syntax offset=3 "invalid character ' ' in literal true (expecting 'e')" | decoder=*errors.errorString "unexpected EOF" off=0
// "nul"                                    valid=false unmarshal=syntax offset=3 "invalid character ' ' in literal null (expecting 'l')" | decoder=*errors.errorString "unexpected EOF" off=0
// "fals"                                   valid=false unmarshal=syntax offset=4 "invalid character ' ' in literal false (expecting 'e')" | decoder=*errors.errorString "unexpected EOF" off=0
// "nulll"                                  valid=false unmarshal=syntax offset=5 "invalid character 'l' after top-level value" | decoder=ok off=4
