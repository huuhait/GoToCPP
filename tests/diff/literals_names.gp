// phase: 02

// Literals and names: non-ASCII and escaped string and rune literals,
// every form of number literal, untyped and typed bool constants, Go
// names that are C++ keywords or macros of the runtime's headers, and
// predeclared names shadowed by variables.
package main

import "fmt"

const yes = true
const no bool = false

func main() {
	s := "é \U0001F600 \u00e9 \xc3\xa9 \377 \a\b\f\v"
	fmt.Println(s, len(s), s[0], s[3], s[len(s)-1])
	fmt.Println('\'', '\\', '\x00', '\n', '\t', 'é', '\U0001F600', '\u00e9', '\377', '"')
	fmt.Println("\"quoted\"", `back\slash`, "a\x00b", len("a\x00b"), "\\")
	fmt.Println(0o17, 017, 0o17, 0x_FF, 0xff, 0b1010, 0b_1_0, 1_000_000)
	fmt.Println(0x1p-2, 0x1.8p1, 0x.8p0, 1e3, 1e-3, .5, 5., 0.)
	var i8 int8 = -0x80
	var u16 uint16 = 0o177777
	fmt.Println(i8, u16, 0x7fff_ffff_ffff_ffff, 'a'+1)

	b := yes
	c := no || yes
	var d bool = yes && !no
	const local = yes == no
	fmt.Println(yes, no, b, c, d, local, !local)

	class, new, delete := 1, 2, 3
	NULL, EOF, errno := 4, 5, 6
	stdin, unix, linux := "in", "unix", "linux"
	assert, int64_t := true, 7.5
	fmt.Println(class, new, delete, NULL, EOF, errno, stdin, unix, linux, assert, int64_t)
	template, namespace, this, operator := 8, 9, 10, 11
	fmt.Println(template+namespace+this+operator, class*new-delete)
	gp_t0, gp_x := "gp_t0", 12
	fmt.Println(gp_t0, gp_x)

	{
		len := 3
		fmt.Println(len * 2)
	}
	{
		int := "int"
		fmt.Println(int + "!")
	}
	{
		true := false
		fmt.Println(true, !true)
	}
	{
		print := 5
		fmt.Println(print)
	}
	fmt.Println(len("still the built-in"), true)
}
