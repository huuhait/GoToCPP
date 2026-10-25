// unicode (Go's own package, imported by strings): the package-level map literals of
// unicode/tables.go (six maps of RangeTable pointers and strings, built by package
// initialization), map lookups, the RangeTable slices, and the SpecialCase methods that
// strings.ToUpperSpecial uses.
package main

import "unicode"

func main() {
	println(unicode.IsUpper('A'), unicode.IsLower('a'), unicode.IsDigit('7'))
	println(string(unicode.ToUpper('é')), string(unicode.ToLower('É')))
	println(unicode.Is(unicode.Greek, 'α'))
	rt := unicode.Categories["Lu"]
	println(rt != nil, len(rt.R16) > 0)
	println(len(unicode.Scripts), len(unicode.Properties) > 0)
	println(unicode.In('ß', unicode.Latin))
	println(string(unicode.ToTitle('a')))
	var sc unicode.SpecialCase = unicode.TurkishCase
	println(string(sc.ToUpper('i')), string(sc.ToLower('I')))
}
