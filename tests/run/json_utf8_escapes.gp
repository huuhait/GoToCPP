// phase: 09
// encoding/json strings: invalid UTF-8 and escapes in both directions, surrogate pairs, the
// HTML-safe set, U+2028/U+2029, control bytes, object keys, SetEscapeHTML and HTMLEscape.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func main() {
	fmt.Println("== encode every byte value below 0x80 and a few above")
	var all []byte
	for value := 0; value < 0x80; value++ {
		all = append(all, byte(value))
	}
	encoded, err := json.Marshal(string(all))
	fmt.Printf("%s %v\n", encoded, err)

	fmt.Println("== encode invalid UTF-8")
	for _, text := range []string{"\xff", "a\xffb", "\xc0\x80", "\xe2\x82", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xf0\x9f\x98", "\xc3\xa9", "\u2028\u2029", "\U0001F600", "\ufffd", "\x7f", "<>&", "ÿĀ"} {
		withHTML, errHTML := json.Marshal(text)
		var buffer bytes.Buffer
		encoder := json.NewEncoder(&buffer)
		encoder.SetEscapeHTML(false)
		errPlain := encoder.Encode(text)
		fmt.Printf("%q => %s %v | noHTML %q %v\n", text, withHTML, errHTML, buffer.String(), errPlain)
		byteSlice, _ := json.Marshal([]string{text})
		keyed, _ := json.Marshal(map[string]int{text: 1})
		fmt.Printf("   slice %s map %s\n", byteSlice, keyed)
	}

	fmt.Println("== decode escapes")
	for _, input := range []string{
		`"\u0041\u00e9\u20ac"`, `"\ud83d\ude00"`, `"\ud83d"`, `"\ude00"`, `"\ud83dx"`, `"\ud83d\u0041"`, `"\ud83d\ud83d"`,
		`"\uD83D\uDE00"`, `"\u00"`, `"\u00zz"`, `"\x41"`, `"\'"`, `"\/"`, `"\0"`, `"a\
b"`, "\"a\nb\"", "\"a\tb\"", "\"a\x7fb\"", "\"a\x00b\"", "\"\xff\"", "\"\xc3\"", "\"\xed\xa0\x80\"", "\"\xe2\x80\xa8\"",
		`"\u0000"`, `"\uffff"`, `"\ufffe"`, `"\u2028"`, `"\\u0041"`, `"\"`, `"abc`, `"\ud800\udc00"`, `"\udbff\udfff"`, `"\ud800\ue000"`,
	} {
		var text string
		err := json.Unmarshal([]byte(input), &text)
		var anyValue any
		errAny := json.Unmarshal([]byte(input), &anyValue)
		fmt.Printf("%-24q => %q err=%v | any=%q err=%v\n", input, text, err, anyValue, errAny)
	}

	fmt.Println("== keys")
	for _, input := range []string{`{"a\u0062":1}`, `{"\ud83d\ude00":1}`, "{\"\xff\":1}", `{"a":1,"a":2}`, `{"A":1,"a":2}`, `{"\u0000":1}`, `{"":1}`} {
		var value map[string]int
		err := json.Unmarshal([]byte(input), &value)
		fmt.Printf("%-24q => %q err=%v\n", input, value, err)
	}
	type Cased struct {
		Value  int `json:"Straße"`
		Other  int `json:"k"`
		Kelvin int `json:"kelvin"`
	}
	for _, input := range []string{`{"Straße":1}`, `{"STRASSE":1}`, `{"strasse":1}`, `{"K":1}`, `{"\u212a":1}`, `{"KELVIN":2}`, `{"k":3}`, `{"\u212aelvin":4}`} {
		var value Cased
		err := json.Unmarshal([]byte(input), &value)
		fmt.Printf("%-20q => %+v err=%v\n", input, value, err)
	}

	fmt.Println("== bytes")
	for _, data := range [][]byte{nil, {}, {0}, {1, 2, 3}, []byte("hello"), []byte("\xff\xfe"), bytes.Repeat([]byte{0xab}, 50)} {
		encoded, err := json.Marshal(data)
		var back []byte
		errBack := json.Unmarshal(encoded, &back)
		fmt.Printf("%v => %s %v => %v %v\n", data, encoded, err, back, errBack)
	}
	for _, input := range []string{`"aGVsbG8="`, `"aGVsbG8"`, `"aGVsbG8=="`, `"!!!!"`, `"aGVs bG8="`, `"aGVs\nbG8="`, `""`, `"YQ=="`, `"YQ="`, `"YR=="`, `[104,101]`, `[256]`, `[-1]`, `null`, `"="`} {
		var back []byte
		err := json.Unmarshal([]byte(input), &back)
		fmt.Printf("%-16s => %v %v\n", input, back, err)
	}

	fmt.Println("== HTMLEscape and Compact")
	for _, input := range []string{`{"a":"<script>&\u2028\u2029"}`, "<>&", "\u2028\u2029", `"<>&"`, "\xff<", `{"k":"v"}   `} {
		var buffer bytes.Buffer
		json.HTMLEscape(&buffer, []byte(input))
		var compact bytes.Buffer
		err := json.Compact(&compact, []byte(input))
		fmt.Printf("%q => escape %q | compact %q %v\n", input, buffer.String(), compact.String(), err)
	}
}

// Output:
// == encode every byte value below 0x80 and a few above
// "\u0000\u0001\u0002\u0003\u0004\u0005\u0006\u0007\b\t\n\u000b\f\r\u000e\u000f\u0010\u0011\u0012\u0013\u0014\u0015\u0016\u0017\u0018\u0019\u001a\u001b\u001c\u001d\u001e\u001f !\"#$%\u0026'()*+,-./0123456789:;\u003c=\u003e?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~" <nil>
// == encode invalid UTF-8
// "\xff" => "\ufffd" <nil> | noHTML "\"\\ufffd\"\n" <nil>
//    slice ["\ufffd"] map {"\ufffd":1}
// "a\xffb" => "a\ufffdb" <nil> | noHTML "\"a\\ufffdb\"\n" <nil>
//    slice ["a\ufffdb"] map {"a\ufffdb":1}
// "\xc0\x80" => "\ufffd\ufffd" <nil> | noHTML "\"\\ufffd\\ufffd\"\n" <nil>
//    slice ["\ufffd\ufffd"] map {"\ufffd\ufffd":1}
// "\xe2\x82" => "\ufffd\ufffd" <nil> | noHTML "\"\\ufffd\\ufffd\"\n" <nil>
//    slice ["\ufffd\ufffd"] map {"\ufffd\ufffd":1}
// "\xed\xa0\x80" => "\ufffd\ufffd\ufffd" <nil> | noHTML "\"\\ufffd\\ufffd\\ufffd\"\n" <nil>
//    slice ["\ufffd\ufffd\ufffd"] map {"\ufffd\ufffd\ufffd":1}
// "\xf4\x90\x80\x80" => "\ufffd\ufffd\ufffd\ufffd" <nil> | noHTML "\"\\ufffd\\ufffd\\ufffd\\ufffd\"\n" <nil>
//    slice ["\ufffd\ufffd\ufffd\ufffd"] map {"\ufffd\ufffd\ufffd\ufffd":1}
// "\xf0\x9f\x98" => "\ufffd\ufffd\ufffd" <nil> | noHTML "\"\\ufffd\\ufffd\\ufffd\"\n" <nil>
//    slice ["\ufffd\ufffd\ufffd"] map {"\ufffd\ufffd\ufffd":1}
// "é" => "é" <nil> | noHTML "\"é\"\n" <nil>
//    slice ["é"] map {"é":1}
// "\u2028\u2029" => "\u2028\u2029" <nil> | noHTML "\"\\u2028\\u2029\"\n" <nil>
//    slice ["\u2028\u2029"] map {"\u2028\u2029":1}
// "😀" => "😀" <nil> | noHTML "\"😀\"\n" <nil>
//    slice ["😀"] map {"😀":1}
// "�" => "�" <nil> | noHTML "\"�\"\n" <nil>
//    slice ["�"] map {"�":1}
// "\x7f" => "" <nil> | noHTML "\"\x7f\"\n" <nil>
//    slice [""] map {"":1}
// "<>&" => "\u003c\u003e\u0026" <nil> | noHTML "\"<>&\"\n" <nil>
//    slice ["\u003c\u003e\u0026"] map {"\u003c\u003e\u0026":1}
// "ÿĀ" => "ÿĀ" <nil> | noHTML "\"ÿĀ\"\n" <nil>
//    slice ["ÿĀ"] map {"ÿĀ":1}
// == decode escapes
// "\"\\u0041\\u00e9\\u20ac\"" => "Aé€" err=<nil> | any="Aé€" err=<nil>
// "\"\\ud83d\\ude00\""     => "😀" err=<nil> | any="😀" err=<nil>
// "\"\\ud83d\""            => "�" err=<nil> | any="�" err=<nil>
// "\"\\ude00\""            => "�" err=<nil> | any="�" err=<nil>
// "\"\\ud83dx\""           => "�x" err=<nil> | any="�x" err=<nil>
// "\"\\ud83d\\u0041\""     => "�A" err=<nil> | any="�A" err=<nil>
// "\"\\ud83d\\ud83d\""     => "��" err=<nil> | any="��" err=<nil>
// "\"\\uD83D\\uDE00\""     => "😀" err=<nil> | any="😀" err=<nil>
// "\"\\u00\""              => "" err=invalid character '"' in \u hexadecimal character escape | any=%!q(<nil>) err=invalid character '"' in \u hexadecimal character escape
// "\"\\u00zz\""            => "" err=invalid character 'z' in \u hexadecimal character escape | any=%!q(<nil>) err=invalid character 'z' in \u hexadecimal character escape
// "\"\\x41\""              => "" err=invalid character 'x' in string escape code | any=%!q(<nil>) err=invalid character 'x' in string escape code
// "\"\\'\""                => "" err=invalid character '\'' in string escape code | any=%!q(<nil>) err=invalid character '\'' in string escape code
// "\"\\/\""                => "/" err=<nil> | any="/" err=<nil>
// "\"\\0\""                => "" err=invalid character '0' in string escape code | any=%!q(<nil>) err=invalid character '0' in string escape code
// "\"a\\\nb\""             => "" err=invalid character '\n' in string escape code | any=%!q(<nil>) err=invalid character '\n' in string escape code
// "\"a\nb\""               => "" err=invalid character '\n' in string literal | any=%!q(<nil>) err=invalid character '\n' in string literal
// "\"a\tb\""               => "" err=invalid character '\t' in string literal | any=%!q(<nil>) err=invalid character '\t' in string literal
// "\"a\x7fb\""             => "a\x7fb" err=<nil> | any="a\x7fb" err=<nil>
// "\"a\x00b\""             => "" err=invalid character '\x00' in string literal | any=%!q(<nil>) err=invalid character '\x00' in string literal
// "\"\xff\""               => "�" err=<nil> | any="�" err=<nil>
// "\"\xc3\""               => "�" err=<nil> | any="�" err=<nil>
// "\"\xed\xa0\x80\""       => "���" err=<nil> | any="���" err=<nil>
// "\"\u2028\""             => "\u2028" err=<nil> | any="\u2028" err=<nil>
// "\"\\u0000\""            => "\x00" err=<nil> | any="\x00" err=<nil>
// "\"\\uffff\""            => "\uffff" err=<nil> | any="\uffff" err=<nil>
// "\"\\ufffe\""            => "\ufffe" err=<nil> | any="\ufffe" err=<nil>
// "\"\\u2028\""            => "\u2028" err=<nil> | any="\u2028" err=<nil>
// "\"\\\\u0041\""          => "\\u0041" err=<nil> | any="\\u0041" err=<nil>
// "\"\\\""                 => "" err=unexpected end of JSON input | any=%!q(<nil>) err=unexpected end of JSON input
// "\"abc"                  => "" err=unexpected end of JSON input | any=%!q(<nil>) err=unexpected end of JSON input
// "\"\\ud800\\udc00\""     => "𐀀" err=<nil> | any="𐀀" err=<nil>
// "\"\\udbff\\udfff\""     => "\U0010ffff" err=<nil> | any="\U0010ffff" err=<nil>
// "\"\\ud800\\ue000\""     => "�\ue000" err=<nil> | any="�\ue000" err=<nil>
// == keys
// "{\"a\\u0062\":1}"       => map["ab":'\x01'] err=<nil>
// "{\"\\ud83d\\ude00\":1}" => map["😀":'\x01'] err=<nil>
// "{\"\xff\":1}"           => map["�":'\x01'] err=<nil>
// "{\"a\":1,\"a\":2}"      => map["a":'\x02'] err=<nil>
// "{\"A\":1,\"a\":2}"      => map["A":'\x01' "a":'\x02'] err=<nil>
// "{\"\\u0000\":1}"        => map["\x00":'\x01'] err=<nil>
// "{\"\":1}"               => map["":'\x01'] err=<nil>
// "{\"Straße\":1}"     => {Value:1 Other:0 Kelvin:0} err=<nil>
// "{\"STRASSE\":1}"    => {Value:0 Other:0 Kelvin:0} err=<nil>
// "{\"strasse\":1}"    => {Value:0 Other:0 Kelvin:0} err=<nil>
// "{\"K\":1}"          => {Value:0 Other:1 Kelvin:0} err=<nil>
// "{\"\\u212a\":1}"    => {Value:0 Other:1 Kelvin:0} err=<nil>
// "{\"KELVIN\":2}"     => {Value:0 Other:0 Kelvin:2} err=<nil>
// "{\"k\":3}"          => {Value:0 Other:3 Kelvin:0} err=<nil>
// "{\"\\u212aelvin\":4}" => {Value:0 Other:0 Kelvin:4} err=<nil>
// == bytes
// [] => null <nil> => [] <nil>
// [] => "" <nil> => [] <nil>
// [0] => "AA==" <nil> => [0] <nil>
// [1 2 3] => "AQID" <nil> => [1 2 3] <nil>
// [104 101 108 108 111] => "aGVsbG8=" <nil> => [104 101 108 108 111] <nil>
// [255 254] => "//4=" <nil> => [255 254] <nil>
// [171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171] => "q6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6urq6s=" <nil> => [171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171 171] <nil>
// "aGVsbG8="       => [104 101 108 108 111] <nil>
// "aGVsbG8"        => [] illegal base64 data at input byte 4
// "aGVsbG8=="      => [] illegal base64 data at input byte 8
// "!!!!"           => [] illegal base64 data at input byte 0
// "aGVs bG8="      => [] illegal base64 data at input byte 4
// "aGVs\nbG8="     => [104 101 108 108 111] <nil>
// ""               => [] <nil>
// "YQ=="           => [97] <nil>
// "YQ="            => [] illegal base64 data at input byte 3
// "YR=="           => [97] <nil>
// [104,101]        => [104 101] <nil>
// [256]            => [0] json: cannot unmarshal number 256 into Go value of type uint8
// [-1]             => [0] json: cannot unmarshal number -1 into Go value of type uint8
// null             => [] <nil>
// "="              => [] illegal base64 data at input byte 0
// == HTMLEscape and Compact
// "{\"a\":\"<script>&\\u2028\\u2029\"}" => escape "{\"a\":\"\\u003cscript\\u003e\\u0026\\u2028\\u2029\"}" | compact "{\"a\":\"<script>&\\u2028\\u2029\"}" <nil>
// "<>&" => escape "\\u003c\\u003e\\u0026" | compact "" invalid character '<' looking for beginning of value
// "\u2028\u2029" => escape "\\u2028\\u2029" | compact "" invalid character 'â' looking for beginning of value
// "\"<>&\"" => escape "\"\\u003c\\u003e\\u0026\"" | compact "\"<>&\"" <nil>
// "\xff<" => escape "\xff\\u003c" | compact "" invalid character 'ÿ' looking for beginning of value
// "{\"k\":\"v\"}   " => escape "{\"k\":\"v\"}   " | compact "{\"k\":\"v\"}" <nil>
