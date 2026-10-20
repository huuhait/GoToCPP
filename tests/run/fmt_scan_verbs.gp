// phase: 09

// fmt.Sscanf with every verb on every kind of target: integers in the bases and with prefixes and underscores, runes, floats,
// complex numbers, booleans, strings (%s %q %x %X %v), byte slices, widths, and the values that are stored when it fails.
package main

import (
	"fmt"
)

func show(p any) string {
	switch v := p.(type) {
	case *bool:
		return fmt.Sprint(*v)
	case *int:
		return fmt.Sprint(*v)
	case *int8:
		return fmt.Sprint(*v)
	case *int16:
		return fmt.Sprint(*v)
	case *int32:
		return fmt.Sprint(*v)
	case *int64:
		return fmt.Sprint(*v)
	case *uint:
		return fmt.Sprint(*v)
	case *uint8:
		return fmt.Sprint(*v)
	case *uint16:
		return fmt.Sprint(*v)
	case *uint32:
		return fmt.Sprint(*v)
	case *uint64:
		return fmt.Sprint(*v)
	case *uintptr:
		return fmt.Sprint(*v)
	case *float32:
		return fmt.Sprint(*v)
	case *float64:
		return fmt.Sprint(*v)
	case *complex64:
		return fmt.Sprint(*v)
	case *complex128:
		return fmt.Sprint(*v)
	case *string:
		return fmt.Sprintf("%q", *v)
	case *[]byte:
		return fmt.Sprintf("%q", *v)
	}
	return "?"
}

func try(input, format string, p any) {
	n, err := fmt.Sscanf(input, format, p)
	fmt.Printf("%-22q %-8q n=%d err=%v val=%s\n", input, format, n, err, show(p))
}

func main() {
	var i int
	var i8 int8
	var i16 int16
	var i32 int32
	var i64 int64
	var u uint
	var u8 uint8
	var u16 uint16
	var u32 uint32
	var u64 uint64
	var up uintptr
	var f32 float32
	var f64 float64
	var c64 complex64
	var c128 complex128
	var b bool
	var s string
	var bs []byte
	var r rune

	fmt.Println("-- integers")
	for _, in := range []string{"42", "-42", "+42", "0", "-0", "007", "0x1F", "0X1f", "-0x1f", "0b101", "0B101", "0o17", "0O17", "017", "1_000", "0x_ff", "1__0", "_1", "1_", "0x", "0b", "12abc", "abc", " 12", "\t\n12", "99999999999999999999", "-9223372036854775808", "9223372036854775807", "9223372036854775808"} {
		try(in, "%v", &i)
	}
	fmt.Println("-- bases")
	for _, f := range []string{"%d", "%b", "%o", "%x", "%X", "%U", "%c", "%s", "%t", "%e"} {
		for _, in := range []string{"101", "7f", "7F", "U+0041", "A", "-17"} {
			try(in, f, &i)
		}
	}
	fmt.Println("-- sized")
	try("127", "%d", &i8)
	try("128", "%d", &i8)
	try("-128", "%d", &i8)
	try("-129", "%d", &i8)
	try("32767", "%d", &i16)
	try("32768", "%d", &i16)
	try("2147483647", "%d", &i32)
	try("2147483648", "%d", &i32)
	try("9223372036854775807", "%d", &i64)
	try("9223372036854775808", "%d", &i64)
	try("255", "%d", &u8)
	try("256", "%d", &u8)
	try("-1", "%d", &u8)
	try("65535", "%d", &u16)
	try("65536", "%d", &u16)
	try("4294967295", "%d", &u32)
	try("4294967296", "%d", &u32)
	try("18446744073709551615", "%d", &u64)
	try("18446744073709551616", "%d", &u64)
	try("18446744073709551615", "%d", &u)
	try("77", "%d", &up)
	try("0xffffffff", "%v", &u32)
	try("ff", "%x", &u8)
	try("100", "%x", &u8)
	try("U+10FFFF", "%U", &u32)
	try("U+110000", "%U", &u32)
	try("u+41", "%U", &i)
	try("U41", "%U", &i)

	fmt.Println("-- runes")
	for _, in := range []string{"a", "é", "世", "😀", "\xff", " ", "\n", ""} {
		try(in, "%c", &r)
		try(in, "%c", &i8)
		try(in, "%c", &u8)
	}

	fmt.Println("-- floats")
	for _, in := range []string{"1", "-1.5", "+1.5", ".5", "5.", "1e3", "1E3", "1e+3", "1e-3", "1e", "1e+", "1.5e3.5", "0x1p4", "0x1.8p1", "0X1P-2", "0x1", "0x.8p1", "1_0.5", "1__0.5", "1e1_0", "inf", "Inf", "-Inf", "+INF", "infinity", "nan", "NaN", "+nan", "1p3", "1.5p-2", "3e0p2", "1e400", "1e-400", "abc", "-", "--1", "0x1p", "1.2.3"} {
		try(in, "%v", &f64)
	}
	for _, f := range []string{"%e", "%E", "%f", "%F", "%g", "%G", "%b", "%d", "%x", "%s"} {
		try("2.5", f, &f64)
	}
	try("3.4e38", "%g", &f32)
	try("3.5e38", "%g", &f32)
	try("1e-46", "%g", &f32)
	try("0.1", "%g", &f32)
	try("16777217", "%g", &f32)

	fmt.Println("-- complex")
	for _, in := range []string{"1+2i", "(1+2i)", "(1+2i", "1+2", "1-2i", "-1.5e2+2.5e-1i", "0x1p1+0x1p2i", "2i", "1", "(1 + 2i)", "1+2i3", "inf+nani", "+Inf-Infi", "1e400+1i"} {
		try(in, "%v", &c128)
	}
	for _, f := range []string{"%e", "%f", "%g", "%d", "%s"} {
		try("(1+2i)", f, &c64)
	}
	try("1e39+1i", "%v", &c64)

	fmt.Println("-- booleans")
	for _, in := range []string{"true", "TRUE", "True", "t", "T", "false", "FALSE", "False", "f", "F", "0", "1", "tru", "truee", "fals", "tr", "2", "yes", "tRuE", "fAlSe", "t rue", ""} {
		try(in, "%v", &b)
	}
	try("true", "%t", &b)
	try("true", "%d", &b)
	try("true", "%s", &b)

	fmt.Println("-- strings")
	for _, in := range []string{"hello", "hello world", "  hello", "hello\nworld", "héllo wörld", "a\tb", "\xff\xfe x", "", "   "} {
		try(in, "%s", &s)
		try(in, "%v", &s)
	}
	for _, in := range []string{`"hello world"`, `"a\tb\n"`, `"é\x41\101\U0001F600"`, "`raw\\n \"q\"`", `""`, "``", `"unterminated`, "`unterminated", `"bad\q"`, `"\x4"`, `"\u12"`, `"\400"`, `"\ud800"`, `"a"b`, `noquote`, `'c'`, `"a
b"`, `"\"`, `"\\"`, `"é"`, "\"\xff\"", `  "sp"`} {
		try(in, "%q", &s)
	}
	for _, in := range []string{"68656c6c6f", "68656C6C6F", "6", "686", "6865 6c6c", "zz", "68zz", "", "00ff"} {
		try(in, "%x", &s)
		try(in, "%X", &bs)
	}
	for _, f := range []string{"%d", "%c", "%e", "%t", "%U", "%b", "%o"} {
		try("abc", f, &s)
	}

	fmt.Println("-- byte slices")
	for _, in := range []string{"hello", "hello world", "é", ""} {
		try(in, "%s", &bs)
		try(in, "%v", &bs)
	}
	try(`"q\x00"`, "%q", &bs)
	try("6869", "%x", &bs)

	fmt.Println("-- widths")
	for _, f := range []string{"%1s", "%2s", "%3s", "%10s", "%0s", "%1d", "%2d", "%3d", "%10d", "%2v", "%2c", "%1c", "%3q", "%4x", "%1f", "%3f", "%5f", "%2e"} {
		try("12345", f, &s)
		try("12345", f, &i)
		try("héllo wörld", f, &s)
		try("1.5e3", f, &f64)
		try(`"abc"`, f, &s)
		try("日本語テキスト", f, &s)
	}
	try("1234567", "%5s", &s)
	try("12 34", "%5s", &s)
	try("-12", "%2d", &i)
	try("+1", "%1d", &i)
	try("0x1f", "%3v", &i)
	try("0x1f", "%4v", &i)

	fmt.Println("-- values on failure")
	i = 7
	s = "keep"
	f64 = 1.5
	try("x", "%d", &i)
	try("", "%d", &i)
	try("", "%s", &s)
	try("x1", "%f", &f64)
	try("99999999999999999999", "%d", &i)
	try("-", "%d", &i)
	try("+", "%d", &i)
	try("1e", "%f", &f64)
}

// Output:
// -- integers
// "42"                   "%v"     n=1 err=<nil> val=42
// "-42"                  "%v"     n=1 err=<nil> val=-42
// "+42"                  "%v"     n=1 err=<nil> val=42
// "0"                    "%v"     n=1 err=<nil> val=0
// "-0"                   "%v"     n=1 err=<nil> val=0
// "007"                  "%v"     n=1 err=<nil> val=7
// "0x1F"                 "%v"     n=1 err=<nil> val=31
// "0X1f"                 "%v"     n=1 err=<nil> val=31
// "-0x1f"                "%v"     n=1 err=<nil> val=-31
// "0b101"                "%v"     n=1 err=<nil> val=5
// "0B101"                "%v"     n=1 err=<nil> val=5
// "0o17"                 "%v"     n=1 err=<nil> val=15
// "0O17"                 "%v"     n=1 err=<nil> val=15
// "017"                  "%v"     n=1 err=<nil> val=15
// "1_000"                "%v"     n=1 err=<nil> val=1000
// "0x_ff"                "%v"     n=1 err=<nil> val=255
// "1__0"                 "%v"     n=0 err=strconv.ParseInt: parsing "1__0": invalid syntax val=255
// "_1"                   "%v"     n=0 err=strconv.ParseInt: parsing "_1": invalid syntax val=255
// "1_"                   "%v"     n=0 err=strconv.ParseInt: parsing "1_": invalid syntax val=255
// "0x"                   "%v"     n=0 err=strconv.ParseInt: parsing "0x": invalid syntax val=255
// "0b"                   "%v"     n=0 err=strconv.ParseInt: parsing "0b": invalid syntax val=255
// "12abc"                "%v"     n=1 err=<nil> val=12
// "abc"                  "%v"     n=0 err=expected integer val=12
// " 12"                  "%v"     n=1 err=<nil> val=12
// "\t\n12"               "%v"     n=0 err=unexpected newline val=12
// "99999999999999999999" "%v"     n=0 err=strconv.ParseInt: parsing "99999999999999999999": value out of range val=12
// "-9223372036854775808" "%v"     n=1 err=<nil> val=-9223372036854775808
// "9223372036854775807"  "%v"     n=1 err=<nil> val=9223372036854775807
// "9223372036854775808"  "%v"     n=0 err=strconv.ParseInt: parsing "9223372036854775808": value out of range val=9223372036854775807
// -- bases
// "101"                  "%d"     n=1 err=<nil> val=101
// "7f"                   "%d"     n=1 err=<nil> val=7
// "7F"                   "%d"     n=1 err=<nil> val=7
// "U+0041"               "%d"     n=0 err=expected integer val=7
// "A"                    "%d"     n=0 err=expected integer val=7
// "-17"                  "%d"     n=1 err=<nil> val=-17
// "101"                  "%b"     n=1 err=<nil> val=5
// "7f"                   "%b"     n=0 err=expected integer val=5
// "7F"                   "%b"     n=0 err=expected integer val=5
// "U+0041"               "%b"     n=0 err=expected integer val=5
// "A"                    "%b"     n=0 err=expected integer val=5
// "-17"                  "%b"     n=1 err=<nil> val=-1
// "101"                  "%o"     n=1 err=<nil> val=65
// "7f"                   "%o"     n=1 err=<nil> val=7
// "7F"                   "%o"     n=1 err=<nil> val=7
// "U+0041"               "%o"     n=0 err=expected integer val=7
// "A"                    "%o"     n=0 err=expected integer val=7
// "-17"                  "%o"     n=1 err=<nil> val=-15
// "101"                  "%x"     n=1 err=<nil> val=257
// "7f"                   "%x"     n=1 err=<nil> val=127
// "7F"                   "%x"     n=1 err=<nil> val=127
// "U+0041"               "%x"     n=0 err=expected integer val=127
// "A"                    "%x"     n=1 err=<nil> val=10
// "-17"                  "%x"     n=1 err=<nil> val=-23
// "101"                  "%X"     n=1 err=<nil> val=257
// "7f"                   "%X"     n=1 err=<nil> val=127
// "7F"                   "%X"     n=1 err=<nil> val=127
// "U+0041"               "%X"     n=0 err=expected integer val=127
// "A"                    "%X"     n=1 err=<nil> val=10
// "-17"                  "%X"     n=1 err=<nil> val=-23
// "101"                  "%U"     n=0 err=bad unicode format  val=-23
// "7f"                   "%U"     n=0 err=bad unicode format  val=-23
// "7F"                   "%U"     n=0 err=bad unicode format  val=-23
// "U+0041"               "%U"     n=1 err=<nil> val=65
// "A"                    "%U"     n=0 err=bad unicode format  val=65
// "-17"                  "%U"     n=0 err=bad unicode format  val=65
// "101"                  "%c"     n=1 err=<nil> val=49
// "7f"                   "%c"     n=1 err=<nil> val=55
// "7F"                   "%c"     n=1 err=<nil> val=55
// "U+0041"               "%c"     n=1 err=<nil> val=85
// "A"                    "%c"     n=1 err=<nil> val=65
// "-17"                  "%c"     n=1 err=<nil> val=45
// "101"                  "%s"     n=0 err=bad verb '%s' for integer val=45
// "7f"                   "%s"     n=0 err=bad verb '%s' for integer val=45
// "7F"                   "%s"     n=0 err=bad verb '%s' for integer val=45
// "U+0041"               "%s"     n=0 err=bad verb '%s' for integer val=45
// "A"                    "%s"     n=0 err=bad verb '%s' for integer val=45
// "-17"                  "%s"     n=0 err=bad verb '%s' for integer val=45
// "101"                  "%t"     n=0 err=bad verb '%t' for integer val=45
// "7f"                   "%t"     n=0 err=bad verb '%t' for integer val=45
// "7F"                   "%t"     n=0 err=bad verb '%t' for integer val=45
// "U+0041"               "%t"     n=0 err=bad verb '%t' for integer val=45
// "A"                    "%t"     n=0 err=bad verb '%t' for integer val=45
// "-17"                  "%t"     n=0 err=bad verb '%t' for integer val=45
// "101"                  "%e"     n=0 err=bad verb '%e' for integer val=45
// "7f"                   "%e"     n=0 err=bad verb '%e' for integer val=45
// "7F"                   "%e"     n=0 err=bad verb '%e' for integer val=45
// "U+0041"               "%e"     n=0 err=bad verb '%e' for integer val=45
// "A"                    "%e"     n=0 err=bad verb '%e' for integer val=45
// "-17"                  "%e"     n=0 err=bad verb '%e' for integer val=45
// -- sized
// "127"                  "%d"     n=1 err=<nil> val=127
// "128"                  "%d"     n=0 err=integer overflow on token 128 val=127
// "-128"                 "%d"     n=1 err=<nil> val=-128
// "-129"                 "%d"     n=0 err=integer overflow on token -129 val=-128
// "32767"                "%d"     n=1 err=<nil> val=32767
// "32768"                "%d"     n=0 err=integer overflow on token 32768 val=32767
// "2147483647"           "%d"     n=1 err=<nil> val=2147483647
// "2147483648"           "%d"     n=0 err=integer overflow on token 2147483648 val=2147483647
// "9223372036854775807"  "%d"     n=1 err=<nil> val=9223372036854775807
// "9223372036854775808"  "%d"     n=0 err=strconv.ParseInt: parsing "9223372036854775808": value out of range val=9223372036854775807
// "255"                  "%d"     n=1 err=<nil> val=255
// "256"                  "%d"     n=0 err=unsigned integer overflow on token 256 val=255
// "-1"                   "%d"     n=0 err=expected integer val=255
// "65535"                "%d"     n=1 err=<nil> val=65535
// "65536"                "%d"     n=0 err=unsigned integer overflow on token 65536 val=65535
// "4294967295"           "%d"     n=1 err=<nil> val=4294967295
// "4294967296"           "%d"     n=0 err=unsigned integer overflow on token 4294967296 val=4294967295
// "18446744073709551615" "%d"     n=1 err=<nil> val=18446744073709551615
// "18446744073709551616" "%d"     n=0 err=strconv.ParseUint: parsing "18446744073709551616": value out of range val=18446744073709551615
// "18446744073709551615" "%d"     n=1 err=<nil> val=18446744073709551615
// "77"                   "%d"     n=1 err=<nil> val=77
// "0xffffffff"           "%v"     n=1 err=<nil> val=4294967295
// "ff"                   "%x"     n=1 err=<nil> val=255
// "100"                  "%x"     n=0 err=unsigned integer overflow on token 100 val=255
// "U+10FFFF"             "%U"     n=1 err=<nil> val=1114111
// "U+110000"             "%U"     n=1 err=<nil> val=1114112
// "u+41"                 "%U"     n=0 err=bad unicode format  val=45
// "U41"                  "%U"     n=0 err=bad unicode format  val=45
// -- runes
// "a"                    "%c"     n=1 err=<nil> val=97
// "a"                    "%c"     n=1 err=<nil> val=97
// "a"                    "%c"     n=1 err=<nil> val=97
// "é"                    "%c"     n=1 err=<nil> val=233
// "é"                    "%c"     n=0 err=overflow on character value é val=97
// "é"                    "%c"     n=0 err=overflow on character value é val=97
// "世"                    "%c"     n=1 err=<nil> val=19990
// "世"                    "%c"     n=0 err=overflow on character value 世 val=97
// "世"                    "%c"     n=0 err=overflow on character value 世 val=97
// "😀"                    "%c"     n=1 err=<nil> val=128512
// "😀"                    "%c"     n=0 err=overflow on character value 😀 val=97
// "😀"                    "%c"     n=0 err=overflow on character value 😀 val=97
// "\xff"                 "%c"     n=1 err=<nil> val=65533
// "\xff"                 "%c"     n=0 err=overflow on character value � val=97
// "\xff"                 "%c"     n=0 err=overflow on character value � val=97
// " "                    "%c"     n=1 err=<nil> val=32
// " "                    "%c"     n=1 err=<nil> val=32
// " "                    "%c"     n=1 err=<nil> val=32
// "\n"                   "%c"     n=1 err=<nil> val=10
// "\n"                   "%c"     n=1 err=<nil> val=10
// "\n"                   "%c"     n=1 err=<nil> val=10
// ""                     "%c"     n=0 err=EOF val=10
// ""                     "%c"     n=0 err=EOF val=10
// ""                     "%c"     n=0 err=EOF val=10
// -- floats
// "1"                    "%v"     n=1 err=<nil> val=1
// "-1.5"                 "%v"     n=1 err=<nil> val=-1.5
// "+1.5"                 "%v"     n=1 err=<nil> val=1.5
// ".5"                   "%v"     n=1 err=<nil> val=0.5
// "5."                   "%v"     n=1 err=<nil> val=5
// "1e3"                  "%v"     n=1 err=<nil> val=1000
// "1E3"                  "%v"     n=1 err=<nil> val=1000
// "1e+3"                 "%v"     n=1 err=<nil> val=1000
// "1e-3"                 "%v"     n=1 err=<nil> val=0.001
// "1e"                   "%v"     n=0 err=strconv.ParseFloat: parsing "1e": invalid syntax val=0.001
// "1e+"                  "%v"     n=0 err=strconv.ParseFloat: parsing "1e+": invalid syntax val=0.001
// "1.5e3.5"              "%v"     n=1 err=<nil> val=1500
// "0x1p4"                "%v"     n=1 err=<nil> val=16
// "0x1.8p1"              "%v"     n=1 err=<nil> val=3
// "0X1P-2"               "%v"     n=1 err=<nil> val=0.25
// "0x1"                  "%v"     n=0 err=strconv.ParseFloat: parsing "0x1": invalid syntax val=0.25
// "0x.8p1"               "%v"     n=1 err=<nil> val=1
// "1_0.5"                "%v"     n=1 err=<nil> val=10.5
// "1__0.5"               "%v"     n=0 err=strconv.ParseFloat: parsing "1__0.5": invalid syntax val=10.5
// "1e1_0"                "%v"     n=1 err=<nil> val=1e+10
// "inf"                  "%v"     n=1 err=<nil> val=+Inf
// "Inf"                  "%v"     n=1 err=<nil> val=+Inf
// "-Inf"                 "%v"     n=1 err=<nil> val=-Inf
// "+INF"                 "%v"     n=1 err=<nil> val=+Inf
// "infinity"             "%v"     n=1 err=<nil> val=+Inf
// "nan"                  "%v"     n=1 err=<nil> val=NaN
// "NaN"                  "%v"     n=1 err=<nil> val=NaN
// "+nan"                 "%v"     n=0 err=strconv.ParseFloat: parsing "+": invalid syntax val=NaN
// "1p3"                  "%v"     n=1 err=<nil> val=8
// "1.5p-2"               "%v"     n=1 err=<nil> val=0.375
// "3e0p2"                "%v"     n=1 err=<nil> val=3
// "1e400"                "%v"     n=0 err=strconv.ParseFloat: parsing "1e400": value out of range val=3
// "1e-400"               "%v"     n=1 err=<nil> val=0
// "abc"                  "%v"     n=0 err=strconv.ParseFloat: parsing "": invalid syntax val=0
// "-"                    "%v"     n=0 err=strconv.ParseFloat: parsing "-": invalid syntax val=0
// "--1"                  "%v"     n=0 err=strconv.ParseFloat: parsing "-": invalid syntax val=0
// "0x1p"                 "%v"     n=0 err=strconv.ParseFloat: parsing "0x1p": invalid syntax val=0
// "1.2.3"                "%v"     n=1 err=<nil> val=1.2
// "2.5"                  "%e"     n=1 err=<nil> val=2.5
// "2.5"                  "%E"     n=1 err=<nil> val=2.5
// "2.5"                  "%f"     n=1 err=<nil> val=2.5
// "2.5"                  "%F"     n=1 err=<nil> val=2.5
// "2.5"                  "%g"     n=1 err=<nil> val=2.5
// "2.5"                  "%G"     n=1 err=<nil> val=2.5
// "2.5"                  "%b"     n=1 err=<nil> val=2.5
// "2.5"                  "%d"     n=0 err=bad verb '%d' for float64 val=2.5
// "2.5"                  "%x"     n=0 err=bad verb '%x' for float64 val=2.5
// "2.5"                  "%s"     n=0 err=bad verb '%s' for float64 val=2.5
// "3.4e38"               "%g"     n=1 err=<nil> val=3.4e+38
// "3.5e38"               "%g"     n=0 err=strconv.ParseFloat: parsing "3.5e38": value out of range val=3.4e+38
// "1e-46"                "%g"     n=1 err=<nil> val=0
// "0.1"                  "%g"     n=1 err=<nil> val=0.1
// "16777217"             "%g"     n=1 err=<nil> val=1.6777216e+07
// -- complex
// "1+2i"                 "%v"     n=1 err=<nil> val=(1+2i)
// "(1+2i)"               "%v"     n=1 err=<nil> val=(1+2i)
// "(1+2i"                "%v"     n=0 err=syntax error scanning complex number val=(1+2i)
// "1+2"                  "%v"     n=0 err=syntax error scanning complex number val=(1+2i)
// "1-2i"                 "%v"     n=1 err=<nil> val=(1-2i)
// "-1.5e2+2.5e-1i"       "%v"     n=1 err=<nil> val=(-150+0.25i)
// "0x1p1+0x1p2i"         "%v"     n=1 err=<nil> val=(2+4i)
// "2i"                   "%v"     n=0 err=syntax error scanning complex number val=(2+4i)
// "1"                    "%v"     n=0 err=syntax error scanning complex number val=(2+4i)
// "(1 + 2i)"             "%v"     n=0 err=syntax error scanning complex number val=(2+4i)
// "1+2i3"                "%v"     n=1 err=<nil> val=(1+2i)
// "inf+nani"             "%v"     n=0 err=strconv.ParseFloat: parsing "+nan": invalid syntax val=(1+2i)
// "+Inf-Infi"            "%v"     n=1 err=<nil> val=(+Inf-Infi)
// "1e400+1i"             "%v"     n=0 err=strconv.ParseFloat: parsing "1e400": value out of range val=(+Inf-Infi)
// "(1+2i)"               "%e"     n=1 err=<nil> val=(1+2i)
// "(1+2i)"               "%f"     n=1 err=<nil> val=(1+2i)
// "(1+2i)"               "%g"     n=1 err=<nil> val=(1+2i)
// "(1+2i)"               "%d"     n=0 err=bad verb '%d' for complex val=(1+2i)
// "(1+2i)"               "%s"     n=0 err=bad verb '%s' for complex val=(1+2i)
// "1e39+1i"              "%v"     n=0 err=strconv.ParseFloat: parsing "1e39": value out of range val=(1+2i)
// -- booleans
// "true"                 "%v"     n=1 err=<nil> val=true
// "TRUE"                 "%v"     n=1 err=<nil> val=true
// "True"                 "%v"     n=1 err=<nil> val=true
// "t"                    "%v"     n=1 err=<nil> val=true
// "T"                    "%v"     n=1 err=<nil> val=true
// "false"                "%v"     n=1 err=<nil> val=false
// "FALSE"                "%v"     n=1 err=<nil> val=false
// "False"                "%v"     n=1 err=<nil> val=false
// "f"                    "%v"     n=1 err=<nil> val=false
// "F"                    "%v"     n=1 err=<nil> val=false
// "0"                    "%v"     n=1 err=<nil> val=false
// "1"                    "%v"     n=1 err=<nil> val=true
// "tru"                  "%v"     n=0 err=syntax error scanning boolean val=true
// "truee"                "%v"     n=1 err=<nil> val=true
// "fals"                 "%v"     n=0 err=syntax error scanning boolean val=true
// "tr"                   "%v"     n=0 err=syntax error scanning boolean val=true
// "2"                    "%v"     n=1 err=<nil> val=false
// "yes"                  "%v"     n=1 err=<nil> val=false
// "tRuE"                 "%v"     n=1 err=<nil> val=true
// "fAlSe"                "%v"     n=1 err=<nil> val=false
// "t rue"                "%v"     n=1 err=<nil> val=true
// ""                     "%v"     n=0 err=EOF val=true
// "true"                 "%t"     n=1 err=<nil> val=true
// "true"                 "%d"     n=0 err=bad verb '%d' for boolean val=true
// "true"                 "%s"     n=0 err=bad verb '%s' for boolean val=true
// -- strings
// "hello"                "%s"     n=1 err=<nil> val="hello"
// "hello"                "%v"     n=1 err=<nil> val="hello"
// "hello world"          "%s"     n=1 err=<nil> val="hello"
// "hello world"          "%v"     n=1 err=<nil> val="hello"
// "  hello"              "%s"     n=1 err=<nil> val="hello"
// "  hello"              "%v"     n=1 err=<nil> val="hello"
// "hello\nworld"         "%s"     n=1 err=<nil> val="hello"
// "hello\nworld"         "%v"     n=1 err=<nil> val="hello"
// "héllo wörld"          "%s"     n=1 err=<nil> val="héllo"
// "héllo wörld"          "%v"     n=1 err=<nil> val="héllo"
// "a\tb"                 "%s"     n=1 err=<nil> val="a"
// "a\tb"                 "%v"     n=1 err=<nil> val="a"
// "\xff\xfe x"           "%s"     n=1 err=<nil> val="��"
// "\xff\xfe x"           "%v"     n=1 err=<nil> val="��"
// ""                     "%s"     n=0 err=EOF val="��"
// ""                     "%v"     n=0 err=EOF val="��"
// "   "                  "%s"     n=0 err=EOF val="��"
// "   "                  "%v"     n=0 err=EOF val="��"
// "\"hello world\""      "%q"     n=1 err=<nil> val="hello world"
// "\"a\\tb\\n\""         "%q"     n=1 err=<nil> val="a\tb\n"
// "\"é\\x41\\101\\U0001F600\"" "%q"     n=1 err=<nil> val="éAA😀"
// "`raw\\n \"q\"`"       "%q"     n=1 err=<nil> val="raw\\n \"q\""
// "\"\""                 "%q"     n=1 err=<nil> val=""
// "``"                   "%q"     n=1 err=<nil> val=""
// "\"unterminated"       "%q"     n=0 err=unexpected EOF val=""
// "`unterminated"        "%q"     n=0 err=unexpected EOF val=""
// "\"bad\\q\""           "%q"     n=0 err=invalid syntax val=""
// "\"\\x4\""             "%q"     n=0 err=invalid syntax val=""
// "\"\\u12\""            "%q"     n=0 err=invalid syntax val=""
// "\"\\400\""            "%q"     n=0 err=invalid syntax val=""
// "\"\\ud800\""          "%q"     n=0 err=invalid syntax val=""
// "\"a\"b"               "%q"     n=1 err=<nil> val="a"
// "noquote"              "%q"     n=0 err=expected quoted string val="a"
// "'c'"                  "%q"     n=0 err=expected quoted string val="a"
// "\"a\nb\""             "%q"     n=0 err=invalid syntax val="a"
// "\"\\\""               "%q"     n=0 err=unexpected EOF val="a"
// "\"\\\\\""             "%q"     n=1 err=<nil> val="\\"
// "\"é\""                "%q"     n=1 err=<nil> val="é"
// "\"\xff\""             "%q"     n=1 err=<nil> val="�"
// "  \"sp\""             "%q"     n=1 err=<nil> val="sp"
// "68656c6c6f"           "%x"     n=1 err=<nil> val="hello"
// "68656c6c6f"           "%X"     n=1 err=<nil> val="hello"
// "68656C6C6F"           "%x"     n=1 err=<nil> val="hello"
// "68656C6C6F"           "%X"     n=1 err=<nil> val="hello"
// "6"                    "%x"     n=0 err=unexpected EOF val="hello"
// "6"                    "%X"     n=0 err=unexpected EOF val="hello"
// "686"                  "%x"     n=0 err=unexpected EOF val="hello"
// "686"                  "%X"     n=0 err=unexpected EOF val="hello"
// "6865 6c6c"            "%x"     n=1 err=<nil> val="he"
// "6865 6c6c"            "%X"     n=1 err=<nil> val="he"
// "zz"                   "%x"     n=0 err=no hex data for %x string val="he"
// "zz"                   "%X"     n=0 err=no hex data for %x string val="he"
// "68zz"                 "%x"     n=1 err=<nil> val="h"
// "68zz"                 "%X"     n=1 err=<nil> val="h"
// ""                     "%x"     n=0 err=EOF val="h"
// ""                     "%X"     n=0 err=EOF val="h"
// "00ff"                 "%x"     n=1 err=<nil> val="\x00\xff"
// "00ff"                 "%X"     n=1 err=<nil> val="\x00\xff"
// "abc"                  "%d"     n=0 err=bad verb '%d' for string val="\x00\xff"
// "abc"                  "%c"     n=0 err=bad verb '%c' for string val="\x00\xff"
// "abc"                  "%e"     n=0 err=bad verb '%e' for string val="\x00\xff"
// "abc"                  "%t"     n=0 err=bad verb '%t' for string val="\x00\xff"
// "abc"                  "%U"     n=0 err=bad verb '%U' for string val="\x00\xff"
// "abc"                  "%b"     n=0 err=bad verb '%b' for string val="\x00\xff"
// "abc"                  "%o"     n=0 err=bad verb '%o' for string val="\x00\xff"
// -- byte slices
// "hello"                "%s"     n=1 err=<nil> val="hello"
// "hello"                "%v"     n=1 err=<nil> val="hello"
// "hello world"          "%s"     n=1 err=<nil> val="hello"
// "hello world"          "%v"     n=1 err=<nil> val="hello"
// "é"                    "%s"     n=1 err=<nil> val="é"
// "é"                    "%v"     n=1 err=<nil> val="é"
// ""                     "%s"     n=0 err=EOF val="é"
// ""                     "%v"     n=0 err=EOF val="é"
// "\"q\\x00\""           "%q"     n=1 err=<nil> val="q\x00"
// "6869"                 "%x"     n=1 err=<nil> val="hi"
// -- widths
// "12345"                "%1s"    n=1 err=<nil> val="1"
// "12345"                "%1s"    n=0 err=bad verb '%s' for integer val=45
// "héllo wörld"          "%1s"    n=1 err=<nil> val="h"
// "1.5e3"                "%1s"    n=0 err=bad verb '%s' for float64 val=2.5
// "\"abc\""              "%1s"    n=1 err=<nil> val="\""
// "日本語テキスト"              "%1s"    n=1 err=<nil> val="日"
// "12345"                "%2s"    n=1 err=<nil> val="12"
// "12345"                "%2s"    n=0 err=bad verb '%s' for integer val=45
// "héllo wörld"          "%2s"    n=1 err=<nil> val="hé"
// "1.5e3"                "%2s"    n=0 err=bad verb '%s' for float64 val=2.5
// "\"abc\""              "%2s"    n=1 err=<nil> val="\"a"
// "日本語テキスト"              "%2s"    n=1 err=<nil> val="日本"
// "12345"                "%3s"    n=1 err=<nil> val="123"
// "12345"                "%3s"    n=0 err=bad verb '%s' for integer val=45
// "héllo wörld"          "%3s"    n=1 err=<nil> val="hél"
// "1.5e3"                "%3s"    n=0 err=bad verb '%s' for float64 val=2.5
// "\"abc\""              "%3s"    n=1 err=<nil> val="\"ab"
// "日本語テキスト"              "%3s"    n=1 err=<nil> val="日本語"
// "12345"                "%10s"   n=1 err=<nil> val="12345"
// "12345"                "%10s"   n=0 err=bad verb '%s' for integer val=45
// "héllo wörld"          "%10s"   n=1 err=<nil> val="héllo"
// "1.5e3"                "%10s"   n=0 err=bad verb '%s' for float64 val=2.5
// "\"abc\""              "%10s"   n=1 err=<nil> val="\"abc\""
// "日本語テキスト"              "%10s"   n=1 err=<nil> val="日本語テキスト"
// "12345"                "%0s"    n=0 err=EOF val="日本語テキスト"
// "12345"                "%0s"    n=0 err=EOF val=45
// "héllo wörld"          "%0s"    n=0 err=EOF val="日本語テキスト"
// "1.5e3"                "%0s"    n=0 err=bad verb '%s' for float64 val=2.5
// "\"abc\""              "%0s"    n=0 err=EOF val="日本語テキスト"
// "日本語テキスト"              "%0s"    n=0 err=EOF val="日本語テキスト"
// "12345"                "%1d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "12345"                "%1d"    n=1 err=<nil> val=1
// "héllo wörld"          "%1d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "1.5e3"                "%1d"    n=0 err=bad verb '%d' for float64 val=2.5
// "\"abc\""              "%1d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "日本語テキスト"              "%1d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "12345"                "%2d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "12345"                "%2d"    n=1 err=<nil> val=12
// "héllo wörld"          "%2d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "1.5e3"                "%2d"    n=0 err=bad verb '%d' for float64 val=2.5
// "\"abc\""              "%2d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "日本語テキスト"              "%2d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "12345"                "%3d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "12345"                "%3d"    n=1 err=<nil> val=123
// "héllo wörld"          "%3d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "1.5e3"                "%3d"    n=0 err=bad verb '%d' for float64 val=2.5
// "\"abc\""              "%3d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "日本語テキスト"              "%3d"    n=0 err=bad verb '%d' for string val="日本語テキスト"
// "12345"                "%10d"   n=0 err=bad verb '%d' for string val="日本語テキスト"
// "12345"                "%10d"   n=1 err=<nil> val=12345
// "héllo wörld"          "%10d"   n=0 err=bad verb '%d' for string val="日本語テキスト"
// "1.5e3"                "%10d"   n=0 err=bad verb '%d' for float64 val=2.5
// "\"abc\""              "%10d"   n=0 err=bad verb '%d' for string val="日本語テキスト"
// "日本語テキスト"              "%10d"   n=0 err=bad verb '%d' for string val="日本語テキスト"
// "12345"                "%2v"    n=1 err=<nil> val="12"
// "12345"                "%2v"    n=1 err=<nil> val=12
// "héllo wörld"          "%2v"    n=1 err=<nil> val="hé"
// "1.5e3"                "%2v"    n=1 err=<nil> val=1
// "\"abc\""              "%2v"    n=1 err=<nil> val="\"a"
// "日本語テキスト"              "%2v"    n=1 err=<nil> val="日本"
// "12345"                "%2c"    n=0 err=bad verb '%c' for string val="日本"
// "12345"                "%2c"    n=1 err=<nil> val=49
// "héllo wörld"          "%2c"    n=0 err=bad verb '%c' for string val="日本"
// "1.5e3"                "%2c"    n=0 err=bad verb '%c' for float64 val=1
// "\"abc\""              "%2c"    n=0 err=bad verb '%c' for string val="日本"
// "日本語テキスト"              "%2c"    n=0 err=bad verb '%c' for string val="日本"
// "12345"                "%1c"    n=0 err=bad verb '%c' for string val="日本"
// "12345"                "%1c"    n=1 err=<nil> val=49
// "héllo wörld"          "%1c"    n=0 err=bad verb '%c' for string val="日本"
// "1.5e3"                "%1c"    n=0 err=bad verb '%c' for float64 val=1
// "\"abc\""              "%1c"    n=0 err=bad verb '%c' for string val="日本"
// "日本語テキスト"              "%1c"    n=0 err=bad verb '%c' for string val="日本"
// "12345"                "%3q"    n=0 err=expected quoted string val="日本"
// "12345"                "%3q"    n=0 err=bad verb '%q' for integer val=49
// "héllo wörld"          "%3q"    n=0 err=expected quoted string val="日本"
// "1.5e3"                "%3q"    n=0 err=bad verb '%q' for float64 val=1
// "\"abc\""              "%3q"    n=0 err=unexpected EOF val="日本"
// "日本語テキスト"              "%3q"    n=0 err=expected quoted string val="日本"
// "12345"                "%4x"    n=1 err=<nil> val="\x124"
// "12345"                "%4x"    n=1 err=<nil> val=4660
// "héllo wörld"          "%4x"    n=0 err=no hex data for %x string val="\x124"
// "1.5e3"                "%4x"    n=0 err=bad verb '%x' for float64 val=1
// "\"abc\""              "%4x"    n=0 err=no hex data for %x string val="\x124"
// "日本語テキスト"              "%4x"    n=0 err=no hex data for %x string val="\x124"
// "12345"                "%1f"    n=0 err=bad verb '%f' for string val="\x124"
// "12345"                "%1f"    n=0 err=bad verb '%f' for integer val=4660
// "héllo wörld"          "%1f"    n=0 err=bad verb '%f' for string val="\x124"
// "1.5e3"                "%1f"    n=1 err=<nil> val=1
// "\"abc\""              "%1f"    n=0 err=bad verb '%f' for string val="\x124"
// "日本語テキスト"              "%1f"    n=0 err=bad verb '%f' for string val="\x124"
// "12345"                "%3f"    n=0 err=bad verb '%f' for string val="\x124"
// "12345"                "%3f"    n=0 err=bad verb '%f' for integer val=4660
// "héllo wörld"          "%3f"    n=0 err=bad verb '%f' for string val="\x124"
// "1.5e3"                "%3f"    n=1 err=<nil> val=1.5
// "\"abc\""              "%3f"    n=0 err=bad verb '%f' for string val="\x124"
// "日本語テキスト"              "%3f"    n=0 err=bad verb '%f' for string val="\x124"
// "12345"                "%5f"    n=0 err=bad verb '%f' for string val="\x124"
// "12345"                "%5f"    n=0 err=bad verb '%f' for integer val=4660
// "héllo wörld"          "%5f"    n=0 err=bad verb '%f' for string val="\x124"
// "1.5e3"                "%5f"    n=1 err=<nil> val=1500
// "\"abc\""              "%5f"    n=0 err=bad verb '%f' for string val="\x124"
// "日本語テキスト"              "%5f"    n=0 err=bad verb '%f' for string val="\x124"
// "12345"                "%2e"    n=0 err=bad verb '%e' for string val="\x124"
// "12345"                "%2e"    n=0 err=bad verb '%e' for integer val=4660
// "héllo wörld"          "%2e"    n=0 err=bad verb '%e' for string val="\x124"
// "1.5e3"                "%2e"    n=1 err=<nil> val=1
// "\"abc\""              "%2e"    n=0 err=bad verb '%e' for string val="\x124"
// "日本語テキスト"              "%2e"    n=0 err=bad verb '%e' for string val="\x124"
// "1234567"              "%5s"    n=1 err=<nil> val="12345"
// "12 34"                "%5s"    n=1 err=<nil> val="12"
// "-12"                  "%2d"    n=1 err=<nil> val=-1
// "+1"                   "%1d"    n=0 err=EOF val=-1
// "0x1f"                 "%3v"    n=1 err=<nil> val=1
// "0x1f"                 "%4v"    n=1 err=<nil> val=31
// -- values on failure
// "x"                    "%d"     n=0 err=expected integer val=7
// ""                     "%d"     n=0 err=EOF val=7
// ""                     "%s"     n=0 err=EOF val="keep"
// "x1"                   "%f"     n=0 err=strconv.ParseFloat: parsing "": invalid syntax val=1.5
// "99999999999999999999" "%d"     n=0 err=strconv.ParseInt: parsing "99999999999999999999": value out of range val=7
// "-"                    "%d"     n=0 err=EOF val=7
// "+"                    "%d"     n=0 err=EOF val=7
// "1e"                   "%f"     n=0 err=strconv.ParseFloat: parsing "1e": invalid syntax val=1.5
