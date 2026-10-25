// strconv (Go's own package): Atoi/Itoa, ParseInt/ParseUint with bases and bit sizes,
// FormatInt in bases 2 and 16, ParseFloat/FormatFloat with formats 'f' and 'e', ParseBool,
// Quote/QuoteRune/Unquote, ParseComplex/FormatComplex, and AppendInt.
package main

import "strconv"

func main() {
	n, err := strconv.Atoi("123")
	println(n, err == nil)
	_, err = strconv.Atoi("x")
	println(err != nil)
	println(strconv.Itoa(-45))
	v, err := strconv.ParseInt("-0x1f", 0, 64)
	println(v, err == nil)
	u, err := strconv.ParseUint("ff", 16, 8)
	println(u, err == nil)
	println(strconv.FormatInt(255, 16), strconv.FormatInt(255, 2))
	f, err := strconv.ParseFloat("3.5", 64)
	println(f, err == nil)
	println(strconv.FormatFloat(3.14159, 'f', 2, 64))
	println(strconv.FormatFloat(1000, 'e', 1, 64))
	b, err := strconv.ParseBool("true")
	println(b, err == nil)
	println(strconv.Quote("go\t!"), strconv.QuoteRune('é'))
	s, err := strconv.Unquote(`"go\t!"`)
	println(s, err == nil)
	c, err := strconv.ParseComplex("1+2i", 128)
	println(real(c), imag(c), err == nil)
	println(strconv.FormatComplex(complex(1.5, -2.5), 'f', 1, 128))
	println(string(strconv.AppendInt([]byte("n="), 42, 10)))
}
