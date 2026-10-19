// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	tags := []reflect.StructTag{
		`a:"b"`,
		`a:"b" c:"d"`,
		`a:"b\"c"`,
		`a:"b\\c"`,
		`a:"line\nbreak"`,
		`a:"tab\there"`,
		`a:"\u00e9\x41\101"`,
		`a: "b"`,
		`a:b`,
		`a:"b`,
		`:"b"`,
		`a"b:"c"`,
		`a:"b"c:"d"`,
		`a:"b"  c:"d"`,
		``,
		` `,
		`a:"" b:"x"`,
		`é:"unicode key"`,
		`a:"b" a:"second"`,
		"a:\"nul\x00\"",
		`key:"va\lue"`,
		`k:"\q"`,
	}
	for _, tag := range tags {
		a, ok := tag.Lookup("a")
		c, ok2 := tag.Lookup("c")
		fmt.Printf("%-24q a=%q,%v c=%q,%v get=%q\n", string(tag), a, ok, c, ok2, tag.Get("b"))
	}
}

// Output:
// "a:\"b\""                a="b",true c="",false get=""
// "a:\"b\" c:\"d\""        a="b",true c="d",true get=""
// "a:\"b\\\"c\""           a="b\"c",true c="",false get=""
// "a:\"b\\\\c\""           a="b\\c",true c="",false get=""
// "a:\"line\\nbreak\""     a="line\nbreak",true c="",false get=""
// "a:\"tab\\there\""       a="tab\there",true c="",false get=""
// "a:\"\\u00e9\\x41\\101\"" a="éAA",true c="",false get=""
// "a: \"b\""               a="",false c="",false get=""
// "a:b"                    a="",false c="",false get=""
// "a:\"b"                  a="",false c="",false get=""
// ":\"b\""                 a="",false c="",false get=""
// "a\"b:\"c\""             a="",false c="",false get=""
// "a:\"b\"c:\"d\""         a="b",true c="d",true get=""
// "a:\"b\"  c:\"d\""       a="b",true c="d",true get=""
// ""                       a="",false c="",false get=""
// " "                      a="",false c="",false get=""
// "a:\"\" b:\"x\""         a="",true c="",false get="x"
// "é:\"unicode key\""      a="",false c="",false get=""
// "a:\"b\" a:\"second\""   a="b",true c="",false get=""
// "a:\"nul\x00\""          a="nul\x00",true c="",false get=""
// "key:\"va\\lue\""        a="",false c="",false get=""
// "k:\"\\q\""              a="",false c="",false get=""
