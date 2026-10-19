// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type T struct {
	A int    `json:"a" xml:"aa" yaml:"a,omitempty"`
	B string `json:"b,omitempty"`
	C bool   `json:"-"`
	D int    `empty:""`
	E int
	F int `k1:"v1" k2:"v2" k3:"v3"`
	G int `  spaced:"yes"   other:"no"  `
}

func main() {
	t := reflect.TypeOf(T{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fmt.Printf("%s tag=%q json=%q xml=%q\n", f.Name, string(f.Tag), f.Tag.Get("json"), f.Tag.Get("xml"))
		for _, k := range []string{"json", "yaml", "empty", "k2", "spaced", "other", "missing"} {
			v, ok := f.Tag.Lookup(k)
			fmt.Printf("  lookup %-8s %q %v\n", k, v, ok)
		}
	}
}

// Output:
// A tag="json:\"a\" xml:\"aa\" yaml:\"a,omitempty\"" json="a" xml="aa"
//   lookup json     "a" true
//   lookup yaml     "a,omitempty" true
//   lookup empty    "" false
//   lookup k2       "" false
//   lookup spaced   "" false
//   lookup other    "" false
//   lookup missing  "" false
// B tag="json:\"b,omitempty\"" json="b,omitempty" xml=""
//   lookup json     "b,omitempty" true
//   lookup yaml     "" false
//   lookup empty    "" false
//   lookup k2       "" false
//   lookup spaced   "" false
//   lookup other    "" false
//   lookup missing  "" false
// C tag="json:\"-\"" json="-" xml=""
//   lookup json     "-" true
//   lookup yaml     "" false
//   lookup empty    "" false
//   lookup k2       "" false
//   lookup spaced   "" false
//   lookup other    "" false
//   lookup missing  "" false
// D tag="empty:\"\"" json="" xml=""
//   lookup json     "" false
//   lookup yaml     "" false
//   lookup empty    "" true
//   lookup k2       "" false
//   lookup spaced   "" false
//   lookup other    "" false
//   lookup missing  "" false
// E tag="" json="" xml=""
//   lookup json     "" false
//   lookup yaml     "" false
//   lookup empty    "" false
//   lookup k2       "" false
//   lookup spaced   "" false
//   lookup other    "" false
//   lookup missing  "" false
// F tag="k1:\"v1\" k2:\"v2\" k3:\"v3\"" json="" xml=""
//   lookup json     "" false
//   lookup yaml     "" false
//   lookup empty    "" false
//   lookup k2       "v2" true
//   lookup spaced   "" false
//   lookup other    "" false
//   lookup missing  "" false
// G tag="  spaced:\"yes\"   other:\"no\"  " json="" xml=""
//   lookup json     "" false
//   lookup yaml     "" false
//   lookup empty    "" false
//   lookup k2       "" false
//   lookup spaced   "yes" true
//   lookup other    "no" true
//   lookup missing  "" false
