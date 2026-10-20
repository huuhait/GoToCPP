// phase: 09
// encoding/json string encoding: escapes, control characters, HTML escaping, invalid UTF-8, U+2028 and U+2029, SetEscapeHTML, map keys.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func show(label string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		return
	}
	fmt.Printf("%s: %s\n", label, b)
}

func showq(label string, v any) {
	b, _ := json.Marshal(v)
	fmt.Printf("%s: %+q\n", label, b)
}

func noescape(label string, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		return
	}
	fmt.Printf("%s: %s", label, buf.String())
}

func main() {
	show("plain", "hello world")
	show("quote", `say "hi"`)
	show("backslash", `a\b`)
	show("newline", "a\nb\rc\td")
	show("backspace formfeed", "\b\f")
	show("control", "\x00\x01\x1f\x7f")
	show("html", "<script>alert('x') & </script>")
	show("amp", "a&b")
	show("gt lt", "<>")
	show("line sep", "a\u2028b\u2029c")
	show("unicode", "héllo, 世界 😀")
	show("invalid utf8 byte", "a\xffb")
	show("invalid utf8 trunc", "\xe4\xb8")
	show("overlong", "\xc0\xaf")
	show("surrogate encoded", "\xed\xa0\x80")
	showq("replacement", "\ufffd")
	showq("bom", "\xef\xbb\xbf")
	show("long", string(bytes.Repeat([]byte("abc\n"), 20)))
	show("slash", "a/b")
	show("apostrophe", "it's")
	show("map keys", map[string]int{"<a>": 1, "b\"c": 2, "d\ne": 3, "é": 4, "\xff": 5})
	show("bytes", []byte{0xff, 0xfe, 0xfd})
	show("struct field names escape", struct {
		A string `json:"x<y"`
	}{"v"})
	noescape("noescape html", "<a href=\"x\">&</a>")
	noescape("noescape sep", "\u2028\u2029")
	noescape("noescape invalid", "\xff")
	noescape("noescape control", "\x01")
	noescape("noescape key", map[string]int{"<k>": 1})
	var buf bytes.Buffer
	json.HTMLEscape(&buf, []byte(`{"a":"<b>&\u2028"}`+"\u2028\u2029"))
	fmt.Println(buf.String())
}

// Output:
// plain: "hello world"
// quote: "say \"hi\""
// backslash: "a\\b"
// newline: "a\nb\rc\td"
// backspace formfeed: "\b\f"
// control: "\u0000\u0001\u001f"
// html: "\u003cscript\u003ealert('x') \u0026 \u003c/script\u003e"
// amp: "a\u0026b"
// gt lt: "\u003c\u003e"
// line sep: "a\u2028b\u2029c"
// unicode: "héllo, 世界 😀"
// invalid utf8 byte: "a\ufffdb"
// invalid utf8 trunc: "\ufffd\ufffd"
// overlong: "\ufffd\ufffd"
// surrogate encoded: "\ufffd\ufffd\ufffd"
// replacement: "\"\ufffd\""
// bom: "\"\ufeff\""
// long: "abc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\nabc\n"
// slash: "a/b"
// apostrophe: "it's"
// map keys: {"\u003ca\u003e":1,"b\"c":2,"d\ne":3,"é":4,"\ufffd":5}
// bytes: "//79"
// struct field names escape: {"x\u003cy":"v"}
// noescape html: "<a href=\"x\">&</a>"
// noescape sep: "\u2028\u2029"
// noescape invalid: "\ufffd"
// noescape control: "\u0001"
// noescape key: {"<k>":1}
// {"a":"\u003cb\u003e\u0026\u2028"}\u2028\u2029
