// phase: 09
// strconv integers against Go: ParseInt, ParseUint and Atoi for every base (0, 2 to 36) and bit size (0, 8, 16, 32, 64) on inputs with signs, prefixes, underscores, overflow and junk (the value and the exact error text), FormatInt, FormatUint, AppendInt and Itoa in every base; a hash of 300000 generated numbers.

package main

import (
	"errors"
	"fmt"
	"strconv"
)

var inputs = []string{
	"", "0", "-0", "+0", "1", "-1", "+1", "00", "007", "-007", "0x", "0X1f", "0xFF", "-0xff", "+0x7fffffff", "0b101", "0B11", "0o17", "0O777",
	"017", "08", "09", "0_8", "1_000", "1__0", "_1", "1_", "0x_1f", "0x1_f", "0b_1", "0b1_0", "0o_7", "0_7", "0_x7",
	"127", "128", "-128", "-129", "255", "256", "32767", "32768", "-32768", "-32769", "65535", "65536",
	"2147483647", "2147483648", "-2147483648", "-2147483649", "4294967295", "4294967296",
	"9223372036854775807", "9223372036854775808", "-9223372036854775808", "-9223372036854775809",
	"18446744073709551615", "18446744073709551616", "99999999999999999999999", "-99999999999999999999999",
	"zz", "ZZ", "z", "Z", "10", "1010", "ff", "FF", "fF", "g", "-", "+", "--1", "+-1", " 1", "1 ", "1.5", "1e3", "0x1p3", "١٢٣", "1\x00", "\xff",
	"1zzzzzzzzzzzz", "3w5e11264sgsf", "3w5e11264sgsg", "-3w5e11264sgsg", "1y2p0ij32e8e7", "1y2p0ij32e8e8",
	"0x7fffffffffffffff", "0x8000000000000000", "0xffffffffffffffff", "0x10000000000000000",
	"0b1111111111111111111111111111111111111111111111111111111111111111", "0b10000000000000000000000000000000000000000000000000000000000000000",
}

var bases = []int{0, 1, 2, 3, 8, 10, 16, 36, 37, -1}
var sizes = []int{0, 8, 16, 32, 64, 65, -1}

func errstr(err error) string {
	if err == nil {
		return "<nil>"
	}
	s := err.Error()
	var ne *strconv.NumError
	if errors.As(err, &ne) {
		k := "?"
		switch {
		case errors.Is(err, strconv.ErrRange):
			k = "range"
		case errors.Is(err, strconv.ErrSyntax):
			k = "syntax"
		}
		return fmt.Sprintf("%s [%s %s %q %s]", s, k, ne.Func, ne.Num, ne.Err)
	}
	return s + " [other]"
}

type hasher struct{ h uint64 }

func (h *hasher) add(s string) {
	if h.h == 0 {
		h.h = 14695981039346656037
	}
	for i := 0; i < len(s); i++ {
		h.h ^= uint64(s[i])
		h.h *= 1099511628211
	}
	h.h ^= 0xff
	h.h *= 1099511628211
}

func main() {
	// 1. the full cross product, hashed per function, with the first lines printed
	var hi, hu, ha hasher
	n := 0
	for _, in := range inputs {
		for _, b := range bases {
			for _, sz := range sizes {
				v, err := strconv.ParseInt(in, b, sz)
				line := fmt.Sprintf("%q %d %d -> %d %s", in, b, sz, v, errstr(err))
				hi.add(line)
				u, err := strconv.ParseUint(in, b, sz)
				line2 := fmt.Sprintf("%q %d %d -> %d %s", in, b, sz, u, errstr(err))
				hu.add(line2)
				n++
				if b == 0 && sz == 64 && n%7 == 0 {
					fmt.Println(line)
					fmt.Println(line2)
				}
			}
		}
		a, err := strconv.Atoi(in)
		line := fmt.Sprintf("%q -> %d %s", in, a, errstr(err))
		ha.add(line)
		fmt.Println(line)
	}
	fmt.Println(n, "combinations")
	fmt.Printf("ParseInt %016x\nParseUint %016x\nAtoi %016x\n", hi.h, hu.h, ha.h)

	// 2. formatting in every base
	vals := []int64{0, 1, -1, 7, 10, 35, 36, 37, 99, 100, 255, 256, 1000, 65535, 65536, -65536, 1 << 31, -(1 << 31), 1<<63 - 1, -1 << 63, 123456789012345678}
	var hf hasher
	for _, v := range vals {
		for b := 2; b <= 36; b++ {
			s := strconv.FormatInt(v, b)
			hf.add(s)
			back, err := strconv.ParseInt(s, b, 64)
			if err != nil || back != v {
				fmt.Println("round trip failed", v, b, s, back, err)
			}
			u := strconv.FormatUint(uint64(v), b)
			hf.add(u)
			ub, err := strconv.ParseUint(u, b, 64)
			if err != nil || ub != uint64(v) {
				fmt.Println("uint round trip failed", v, b, u, ub, err)
			}
			buf := strconv.AppendInt([]byte("x="), v, b)
			hf.add(string(buf))
			buf = strconv.AppendUint(buf[:0], uint64(v), b)
			hf.add(string(buf))
		}
		fmt.Println(v, strconv.Itoa(int(v)), strconv.FormatInt(v, 2), strconv.FormatInt(v, 16), strconv.FormatInt(v, 36), strconv.FormatUint(uint64(v), 10))
	}
	fmt.Printf("Format %016x\n", hf.h)

	// 3. generated numbers: format, parse back, in many bases
	var hg hasher
	state := uint64(0x9E3779B97F4A7C15)
	bad := 0
	for i := 0; i < 300000; i++ {
		state += 0x9E3779B97F4A7C15
		z := state
		z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
		z = (z ^ (z >> 27)) * 0x94D049BB133111EB
		z ^= z >> 31
		shift := uint(i % 64)
		v := int64(z >> shift)
		if i%3 == 0 {
			v = -v
		}
		base := 2 + i%35
		s := strconv.FormatInt(v, base)
		hg.add(s)
		back, err := strconv.ParseInt(s, base, 64)
		if err != nil || back != v {
			bad++
		}
		sz := []int{8, 16, 32, 64}[i%4]
		w, err := strconv.ParseInt(s, base, sz)
		hg.add(fmt.Sprint(w, err != nil))
		if i%1000 == 0 {
			fmt.Println(i, base, s, w, errstr(err))
		}
	}
	fmt.Printf("generated %016x bad=%d\n", hg.h, bad)
}

// Output:
// "" -> 0 strconv.Atoi: parsing "": invalid syntax [syntax Atoi "" invalid syntax]
// "0" -> 0 <nil>
// "-0" -> 0 <nil>
// "+0" -> 0 <nil>
// "1" -> 1 <nil>
// "-1" -> -1 <nil>
// "+1" -> 1 <nil>
// "00" -> 0 <nil>
// "007" -> 7 <nil>
// "-007" -> -7 <nil>
// "0x" -> 0 strconv.Atoi: parsing "0x": invalid syntax [syntax Atoi "0x" invalid syntax]
// "0X1f" -> 0 strconv.Atoi: parsing "0X1f": invalid syntax [syntax Atoi "0X1f" invalid syntax]
// "0xFF" -> 0 strconv.Atoi: parsing "0xFF": invalid syntax [syntax Atoi "0xFF" invalid syntax]
// "-0xff" -> 0 strconv.Atoi: parsing "-0xff": invalid syntax [syntax Atoi "-0xff" invalid syntax]
// "+0x7fffffff" -> 0 strconv.Atoi: parsing "+0x7fffffff": invalid syntax [syntax Atoi "+0x7fffffff" invalid syntax]
// "0b101" -> 0 strconv.Atoi: parsing "0b101": invalid syntax [syntax Atoi "0b101" invalid syntax]
// "0B11" -> 0 strconv.Atoi: parsing "0B11": invalid syntax [syntax Atoi "0B11" invalid syntax]
// "0o17" -> 0 strconv.Atoi: parsing "0o17": invalid syntax [syntax Atoi "0o17" invalid syntax]
// "0O777" -> 0 strconv.Atoi: parsing "0O777": invalid syntax [syntax Atoi "0O777" invalid syntax]
// "017" -> 17 <nil>
// "08" -> 8 <nil>
// "09" -> 9 <nil>
// "0_8" -> 0 strconv.Atoi: parsing "0_8": invalid syntax [syntax Atoi "0_8" invalid syntax]
// "1_000" -> 0 strconv.Atoi: parsing "1_000": invalid syntax [syntax Atoi "1_000" invalid syntax]
// "1__0" -> 0 strconv.Atoi: parsing "1__0": invalid syntax [syntax Atoi "1__0" invalid syntax]
// "_1" -> 0 strconv.Atoi: parsing "_1": invalid syntax [syntax Atoi "_1" invalid syntax]
// "1_" -> 0 strconv.Atoi: parsing "1_": invalid syntax [syntax Atoi "1_" invalid syntax]
// "0x_1f" -> 0 strconv.Atoi: parsing "0x_1f": invalid syntax [syntax Atoi "0x_1f" invalid syntax]
// "0x1_f" -> 0 strconv.Atoi: parsing "0x1_f": invalid syntax [syntax Atoi "0x1_f" invalid syntax]
// "0b_1" -> 0 strconv.Atoi: parsing "0b_1": invalid syntax [syntax Atoi "0b_1" invalid syntax]
// "0b1_0" -> 0 strconv.Atoi: parsing "0b1_0": invalid syntax [syntax Atoi "0b1_0" invalid syntax]
// "0o_7" -> 0 strconv.Atoi: parsing "0o_7": invalid syntax [syntax Atoi "0o_7" invalid syntax]
// "0_7" -> 0 strconv.Atoi: parsing "0_7": invalid syntax [syntax Atoi "0_7" invalid syntax]
// "0_x7" -> 0 strconv.Atoi: parsing "0_x7": invalid syntax [syntax Atoi "0_x7" invalid syntax]
// "127" -> 127 <nil>
// "128" -> 128 <nil>
// "-128" -> -128 <nil>
// "-129" -> -129 <nil>
// "255" -> 255 <nil>
// "256" -> 256 <nil>
// "32767" -> 32767 <nil>
// "32768" -> 32768 <nil>
// "-32768" -> -32768 <nil>
// "-32769" -> -32769 <nil>
// "65535" -> 65535 <nil>
// "65536" -> 65536 <nil>
// "2147483647" -> 2147483647 <nil>
// "2147483648" -> 2147483648 <nil>
// "-2147483648" -> -2147483648 <nil>
// "-2147483649" -> -2147483649 <nil>
// "4294967295" -> 4294967295 <nil>
// "4294967296" -> 4294967296 <nil>
// "9223372036854775807" -> 9223372036854775807 <nil>
// "9223372036854775808" -> 9223372036854775807 strconv.Atoi: parsing "9223372036854775808": value out of range [range Atoi "9223372036854775808" value out of range]
// "-9223372036854775808" -> -9223372036854775808 <nil>
// "-9223372036854775809" -> -9223372036854775808 strconv.Atoi: parsing "-9223372036854775809": value out of range [range Atoi "-9223372036854775809" value out of range]
// "18446744073709551615" -> 9223372036854775807 strconv.Atoi: parsing "18446744073709551615": value out of range [range Atoi "18446744073709551615" value out of range]
// "18446744073709551616" -> 9223372036854775807 strconv.Atoi: parsing "18446744073709551616": value out of range [range Atoi "18446744073709551616" value out of range]
// "99999999999999999999999" -> 9223372036854775807 strconv.Atoi: parsing "99999999999999999999999": value out of range [range Atoi "99999999999999999999999" value out of range]
// "-99999999999999999999999" -> -9223372036854775808 strconv.Atoi: parsing "-99999999999999999999999": value out of range [range Atoi "-99999999999999999999999" value out of range]
// "zz" -> 0 strconv.Atoi: parsing "zz": invalid syntax [syntax Atoi "zz" invalid syntax]
// "ZZ" -> 0 strconv.Atoi: parsing "ZZ": invalid syntax [syntax Atoi "ZZ" invalid syntax]
// "z" -> 0 strconv.Atoi: parsing "z": invalid syntax [syntax Atoi "z" invalid syntax]
// "Z" -> 0 strconv.Atoi: parsing "Z": invalid syntax [syntax Atoi "Z" invalid syntax]
// "10" -> 10 <nil>
// "1010" -> 1010 <nil>
// "ff" -> 0 strconv.Atoi: parsing "ff": invalid syntax [syntax Atoi "ff" invalid syntax]
// "FF" -> 0 strconv.Atoi: parsing "FF": invalid syntax [syntax Atoi "FF" invalid syntax]
// "fF" -> 0 strconv.Atoi: parsing "fF": invalid syntax [syntax Atoi "fF" invalid syntax]
// "g" -> 0 strconv.Atoi: parsing "g": invalid syntax [syntax Atoi "g" invalid syntax]
// "-" -> 0 strconv.Atoi: parsing "-": invalid syntax [syntax Atoi "-" invalid syntax]
// "+" -> 0 strconv.Atoi: parsing "+": invalid syntax [syntax Atoi "+" invalid syntax]
// "--1" -> 0 strconv.Atoi: parsing "--1": invalid syntax [syntax Atoi "--1" invalid syntax]
// "+-1" -> 0 strconv.Atoi: parsing "+-1": invalid syntax [syntax Atoi "+-1" invalid syntax]
// " 1" -> 0 strconv.Atoi: parsing " 1": invalid syntax [syntax Atoi " 1" invalid syntax]
// "1 " -> 0 strconv.Atoi: parsing "1 ": invalid syntax [syntax Atoi "1 " invalid syntax]
// "1.5" -> 0 strconv.Atoi: parsing "1.5": invalid syntax [syntax Atoi "1.5" invalid syntax]
// "1e3" -> 0 strconv.Atoi: parsing "1e3": invalid syntax [syntax Atoi "1e3" invalid syntax]
// "0x1p3" -> 0 strconv.Atoi: parsing "0x1p3": invalid syntax [syntax Atoi "0x1p3" invalid syntax]
// "١٢٣" -> 0 strconv.Atoi: parsing "١٢٣": invalid syntax [syntax Atoi "١٢٣" invalid syntax]
// "1\x00" -> 0 strconv.Atoi: parsing "1\x00": invalid syntax [syntax Atoi "1\x00" invalid syntax]
// "\xff" -> 0 strconv.Atoi: parsing "\xff": invalid syntax [syntax Atoi "\xff" invalid syntax]
// "1zzzzzzzzzzzz" -> 0 strconv.Atoi: parsing "1zzzzzzzzzzzz": invalid syntax [syntax Atoi "1zzzzzzzzzzzz" invalid syntax]
// "3w5e11264sgsf" -> 0 strconv.Atoi: parsing "3w5e11264sgsf": invalid syntax [syntax Atoi "3w5e11264sgsf" invalid syntax]
// "3w5e11264sgsg" -> 0 strconv.Atoi: parsing "3w5e11264sgsg": invalid syntax [syntax Atoi "3w5e11264sgsg" invalid syntax]
// "-3w5e11264sgsg" -> 0 strconv.Atoi: parsing "-3w5e11264sgsg": invalid syntax [syntax Atoi "-3w5e11264sgsg" invalid syntax]
// "1y2p0ij32e8e7" -> 0 strconv.Atoi: parsing "1y2p0ij32e8e7": invalid syntax [syntax Atoi "1y2p0ij32e8e7" invalid syntax]
// "1y2p0ij32e8e8" -> 0 strconv.Atoi: parsing "1y2p0ij32e8e8": invalid syntax [syntax Atoi "1y2p0ij32e8e8" invalid syntax]
// "0x7fffffffffffffff" -> 0 strconv.Atoi: parsing "0x7fffffffffffffff": invalid syntax [syntax Atoi "0x7fffffffffffffff" invalid syntax]
// "0x8000000000000000" -> 0 strconv.Atoi: parsing "0x8000000000000000": invalid syntax [syntax Atoi "0x8000000000000000" invalid syntax]
// "0xffffffffffffffff" -> 0 strconv.Atoi: parsing "0xffffffffffffffff": invalid syntax [syntax Atoi "0xffffffffffffffff" invalid syntax]
// "0x10000000000000000" -> 0 strconv.Atoi: parsing "0x10000000000000000": invalid syntax [syntax Atoi "0x10000000000000000" invalid syntax]
// "0b1111111111111111111111111111111111111111111111111111111111111111" -> 0 strconv.Atoi: parsing "0b1111111111111111111111111111111111111111111111111111111111111111": invalid syntax [syntax Atoi "0b1111111111111111111111111111111111111111111111111111111111111111" invalid syntax]
// "0b10000000000000000000000000000000000000000000000000000000000000000" -> 0 strconv.Atoi: parsing "0b10000000000000000000000000000000000000000000000000000000000000000": invalid syntax [syntax Atoi "0b10000000000000000000000000000000000000000000000000000000000000000" invalid syntax]
// 6580 combinations
// ParseInt 073cb1ccf0ff2b76
// ParseUint 4681a5660ca56096
// Atoi 460b84e25b1a06c3
// 0 0 0 0 0 0
// 1 1 1 1 1 1
// -1 -1 -1 -1 -1 18446744073709551615
// 7 7 111 7 7 7
// 10 10 1010 a a 10
// 35 35 100011 23 z 35
// 36 36 100100 24 10 36
// 37 37 100101 25 11 37
// 99 99 1100011 63 2r 99
// 100 100 1100100 64 2s 100
// 255 255 11111111 ff 73 255
// 256 256 100000000 100 74 256
// 1000 1000 1111101000 3e8 rs 1000
// 65535 65535 1111111111111111 ffff 1ekf 65535
// 65536 65536 10000000000000000 10000 1ekg 65536
// -65536 -65536 -10000000000000000 -10000 -1ekg 18446744073709486080
// 2147483648 2147483648 10000000000000000000000000000000 80000000 zik0zk 2147483648
// -2147483648 -2147483648 -10000000000000000000000000000000 -80000000 -zik0zk 18446744071562067968
// 9223372036854775807 9223372036854775807 111111111111111111111111111111111111111111111111111111111111111 7fffffffffffffff 1y2p0ij32e8e7 9223372036854775807
// -9223372036854775808 -9223372036854775808 -1000000000000000000000000000000000000000000000000000000000000000 -8000000000000000 -1y2p0ij32e8e8 9223372036854775808
// 123456789012345678 123456789012345678 110110110100110110100101110100110001100001111001101001110 1b69b4ba630f34e xrls1wq49ji 123456789012345678
// Format 3ab54e364b958fe8
// 0 2 -110111001111000100111100110101010100001101110010110010111110100 -128 strconv.ParseInt: parsing "-110111001111000100111100110101010100001101110010110010111110100": value out of range [range ParseInt "-110111001111000100111100110101010100001101110010110010111110100" value out of range]
// 1000 22 2hg4gh 127 strconv.ParseInt: parsing "2hg4gh": value out of range [range ParseInt "2hg4gh" value out of range]
// 2000 7 21646520133312143 127 strconv.ParseInt: parsing "21646520133312143": value out of range [range ParseInt "21646520133312143" value out of range]
// 3000 27 -f -15 <nil>
// 4000 12 9b2b66368 127 strconv.ParseInt: parsing "9b2b66368": value out of range [range ParseInt "9b2b66368" value out of range]
// 5000 32 18q5069616bl 127 strconv.ParseInt: parsing "18q5069616bl": value out of range [range ParseInt "18q5069616bl" value out of range]
// 6000 17 -9cbb -128 strconv.ParseInt: parsing "-9cbb": value out of range [range ParseInt "-9cbb" value out of range]
// 7000 2 1010010011110010000011011011001001110001 127 strconv.ParseInt: parsing "1010010011110010000011011011001001110001": value out of range [range ParseInt "1010010011110010000011011011001001110001" value out of range]
// 8000 22 -120j50616179251 -128 strconv.ParseInt: parsing "-120j50616179251": value out of range [range ParseInt "-120j50616179251" value out of range]
// 9000 7 -62653216 -128 strconv.ParseInt: parsing "-62653216": value out of range [range ParseInt "-62653216" value out of range]
// 10000 27 6lbpm4j0g3 127 strconv.ParseInt: parsing "6lbpm4j0g3": value out of range [range ParseInt "6lbpm4j0g3" value out of range]
// 11000 12 b6 127 strconv.ParseInt: parsing "b6": value out of range [range ParseInt "b6" value out of range]
// 12000 32 -1epe7ai -128 strconv.ParseInt: parsing "-1epe7ai": value out of range [range ParseInt "-1epe7ai" value out of range]
// 13000 17 33905g90913934 127 strconv.ParseInt: parsing "33905g90913934": value out of range [range ParseInt "33905g90913934" value out of range]
// 14000 2 1001101101001110 127 strconv.ParseInt: parsing "1001101101001110": value out of range [range ParseInt "1001101101001110" value out of range]
// 15000 22 -f7ie6934i -128 strconv.ParseInt: parsing "-f7ie6934i": value out of range [range ParseInt "-f7ie6934i" value out of range]
// 16000 7 20041450255103231061651 127 strconv.ParseInt: parsing "20041450255103231061651": value out of range [range ParseInt "20041450255103231061651" value out of range]
// 17000 27 f3hqf 127 strconv.ParseInt: parsing "f3hqf": value out of range [range ParseInt "f3hqf" value out of range]
// 18000 12 -2410bb167b2052 -128 strconv.ParseInt: parsing "-2410bb167b2052": value out of range [range ParseInt "-2410bb167b2052" value out of range]
// 19000 32 v 31 <nil>
// 20000 17 40e052fc 127 strconv.ParseInt: parsing "40e052fc": value out of range [range ParseInt "40e052fc" value out of range]
// 21000 2 -11101101110000000110011001010000111111111000100011111000 -128 strconv.ParseInt: parsing "-11101101110000000110011001010000111111111000100011111000": value out of range [range ParseInt "-11101101110000000110011001010000111111111000100011111000" value out of range]
// 22000 22 19a8 127 strconv.ParseInt: parsing "19a8": value out of range [range ParseInt "19a8" value out of range]
// 23000 7 4562505266025 127 strconv.ParseInt: parsing "4562505266025": value out of range [range ParseInt "4562505266025" value out of range]
// 24000 27 -4pbi86cql781m -128 strconv.ParseInt: parsing "-4pbi86cql781m": value out of range [range ParseInt "-4pbi86cql781m" value out of range]
// 25000 12 1bb5a26 127 strconv.ParseInt: parsing "1bb5a26": value out of range [range ParseInt "1bb5a26" value out of range]
// 26000 32 6homv24428 127 strconv.ParseInt: parsing "6homv24428": value out of range [range ParseInt "6homv24428" value out of range]
// 27000 17 -a0 -128 strconv.ParseInt: parsing "-a0": value out of range [range ParseInt "-a0" value out of range]
// 28000 2 1010001000100001011111011110011 127 strconv.ParseInt: parsing "1010001000100001011111011110011": value out of range [range ParseInt "1010001000100001011111011110011" value out of range]
// 29000 22 41bdb4fdj90jc 127 strconv.ParseInt: parsing "41bdb4fdj90jc": value out of range [range ParseInt "41bdb4fdj90jc" value out of range]
// 30000 7 -31563 -128 strconv.ParseInt: parsing "-31563": value out of range [range ParseInt "-31563" value out of range]
// 31000 27 21eon5d9c 127 strconv.ParseInt: parsing "21eon5d9c": value out of range [range ParseInt "21eon5d9c" value out of range]
// 32000 12 3667490307800ab226 127 strconv.ParseInt: parsing "3667490307800ab226": value out of range [range ParseInt "3667490307800ab226" value out of range]
// 33000 32 -c2sfm -128 strconv.ParseInt: parsing "-c2sfm": value out of range [range ParseInt "-c2sfm" value out of range]
// 34000 17 531fg069ag27 127 strconv.ParseInt: parsing "531fg069ag27": value out of range [range ParseInt "531fg069ag27" value out of range]
// 35000 2 1010101 85 <nil>
// 36000 22 -13c0lbk3 -128 strconv.ParseInt: parsing "-13c0lbk3": value out of range [range ParseInt "-13c0lbk3" value out of range]
// 37000 7 43104553631654355600 127 strconv.ParseInt: parsing "43104553631654355600": value out of range [range ParseInt "43104553631654355600" value out of range]
// 38000 27 2fqb 127 strconv.ParseInt: parsing "2fqb": value out of range [range ParseInt "2fqb" value out of range]
// 39000 12 -129612830a6 -128 strconv.ParseInt: parsing "-129612830a6": value out of range [range ParseInt "-129612830a6" value out of range]
// 40000 32 6tj42ag8tnr7d 127 strconv.ParseInt: parsing "6tj42ag8tnr7d": value out of range [range ParseInt "6tj42ag8tnr7d" value out of range]
// 41000 17 20d374 127 strconv.ParseInt: parsing "20d374": value out of range [range ParseInt "20d374" value out of range]
// 42000 2 -111101100100101011011100111001110110011111111100 -128 strconv.ParseInt: parsing "-111101100100101011011100111001110110011111111100": value out of range [range ParseInt "-111101100100101011011100111001110110011111111100" value out of range]
// 43000 22 5b 121 <nil>
// 44000 7 205322644114 127 strconv.ParseInt: parsing "205322644114": value out of range [range ParseInt "205322644114" value out of range]
// 45000 27 -97j3ppfpqcf9 -128 strconv.ParseInt: parsing "-97j3ppfpqcf9": value out of range [range ParseInt "-97j3ppfpqcf9" value out of range]
// 46000 12 1a324 127 strconv.ParseInt: parsing "1a324": value out of range [range ParseInt "1a324" value out of range]
// 47000 32 6d3a50hh 127 strconv.ParseInt: parsing "6d3a50hh": value out of range [range ParseInt "6d3a50hh" value out of range]
// 48000 17 161e13f583f41f6g 127 strconv.ParseInt: parsing "161e13f583f41f6g": value out of range [range ParseInt "161e13f583f41f6g" value out of range]
// 49000 2 101110110011010100010111 127 strconv.ParseInt: parsing "101110110011010100010111": value out of range [range ParseInt "101110110011010100010111" value out of range]
// 50000 22 73769lhdclb 127 strconv.ParseInt: parsing "73769lhdclb": value out of range [range ParseInt "73769lhdclb" value out of range]
// 51000 7 -204 -102 <nil>
// 52000 27 52hm0ae 127 strconv.ParseInt: parsing "52hm0ae": value out of range [range ParseInt "52hm0ae" value out of range]
// 53000 12 82a30499485b58 127 strconv.ParseInt: parsing "82a30499485b58": value out of range [range ParseInt "82a30499485b58" value out of range]
// 54000 32 -ao5 -128 strconv.ParseInt: parsing "-ao5": value out of range [range ParseInt "-ao5" value out of range]
// 55000 17 7874cg302e 127 strconv.ParseInt: parsing "7874cg302e": value out of range [range ParseInt "7874cg302e" value out of range]
// 56000 2 -110110010111001111110100110100101000100001100011000111010101101 -128 strconv.ParseInt: parsing "-110110010111001111110100110100101000100001100011000111010101101": value out of range [range ParseInt "-110110010111001111110100110100101000100001100011000111010101101" value out of range]
// 57000 22 -12ag2l -128 strconv.ParseInt: parsing "-12ag2l": value out of range [range ParseInt "-12ag2l" value out of range]
// 58000 7 10145624601504221 127 strconv.ParseInt: parsing "10145624601504221": value out of range [range ParseInt "10145624601504221" value out of range]
// 59000 27 4j 127 <nil>
// 60000 12 -8b9099441 -128 strconv.ParseInt: parsing "-8b9099441": value out of range [range ParseInt "-8b9099441" value out of range]
// 61000 32 a92plb5gvhp 127 strconv.ParseInt: parsing "a92plb5gvhp": value out of range [range ParseInt "a92plb5gvhp" value out of range]
// 62000 17 290 127 strconv.ParseInt: parsing "290": value out of range [range ParseInt "290" value out of range]
// 63000 2 -1010110110111100000101001010011001000110 -128 strconv.ParseInt: parsing "-1010110110111100000101001010011001000110": value out of range [range ParseInt "-1010110110111100000101001010011001000110" value out of range]
// 64000 22 -17f5cjdai049i29 -128 strconv.ParseInt: parsing "-17f5cjdai049i29": value out of range [range ParseInt "-17f5cjdai049i29" value out of range]
// 65000 7 36364456 127 strconv.ParseInt: parsing "36364456": value out of range [range ParseInt "36364456" value out of range]
// 66000 27 -fek6gnm99f -128 strconv.ParseInt: parsing "-fek6gnm99f": value out of range [range ParseInt "-fek6gnm99f" value out of range]
// 67000 12 47 55 <nil>
// 68000 32 3j60m9 127 strconv.ParseInt: parsing "3j60m9": value out of range [range ParseInt "3j60m9" value out of range]
// 69000 17 -448a87a80cgb91 -128 strconv.ParseInt: parsing "-448a87a80cgb91": value out of range [range ParseInt "-448a87a80cgb91" value out of range]
// 70000 2 110000100101111 127 strconv.ParseInt: parsing "110000100101111": value out of range [range ParseInt "110000100101111" value out of range]
// 71000 22 jgbcd2dlg 127 strconv.ParseInt: parsing "jgbcd2dlg": value out of range [range ParseInt "jgbcd2dlg" value out of range]
// 72000 7 21052065216505223331025 127 strconv.ParseInt: parsing "21052065216505223331025": value out of range [range ParseInt "21052065216505223331025" value out of range]
// 73000 27 glo57 127 strconv.ParseInt: parsing "glo57": value out of range [range ParseInt "glo57" value out of range]
// 74000 12 1186a0b16994a8 127 strconv.ParseInt: parsing "1186a0b16994a8": value out of range [range ParseInt "1186a0b16994a8" value out of range]
// 75000 32 -1e -46 <nil>
// 76000 17 9c2b5397 127 strconv.ParseInt: parsing "9c2b5397": value out of range [range ParseInt "9c2b5397" value out of range]
// 77000 2 1100101100100100111110011010110000110001011010010101111 127 strconv.ParseInt: parsing "1100101100100100111110011010110000110001011010010101111": value out of range [range ParseInt "1100101100100100111110011010110000110001011010010101111" value out of range]
// 78000 22 -5a9h -128 strconv.ParseInt: parsing "-5a9h": value out of range [range ParseInt "-5a9h" value out of range]
// 79000 7 105562430246636 127 strconv.ParseInt: parsing "105562430246636": value out of range [range ParseInt "105562430246636" value out of range]
// 80000 27 -10jlb4c0alof68 -128 strconv.ParseInt: parsing "-10jlb4c0alof68": value out of range [range ParseInt "-10jlb4c0alof68" value out of range]
// 81000 12 -3aa4181 -128 strconv.ParseInt: parsing "-3aa4181": value out of range [range ParseInt "-3aa4181" value out of range]
// 82000 32 4p02j5toq0 127 strconv.ParseInt: parsing "4p02j5toq0": value out of range [range ParseInt "4p02j5toq0" value out of range]
// 83000 17 97 127 strconv.ParseInt: parsing "97": value out of range [range ParseInt "97" value out of range]
// 84000 2 -1101010110011001001101001110100 -128 strconv.ParseInt: parsing "-1101010110011001001101001110100": value out of range [range ParseInt "-1101010110011001001101001110100" value out of range]
// 85000 22 43b32258dbdgl 127 strconv.ParseInt: parsing "43b32258dbdgl": value out of range [range ParseInt "43b32258dbdgl" value out of range]
// 86000 7 2202 127 strconv.ParseInt: parsing "2202": value out of range [range ParseInt "2202" value out of range]
// 87000 27 -225onjpe9 -128 strconv.ParseInt: parsing "-225onjpe9": value out of range [range ParseInt "-225onjpe9" value out of range]
// 88000 12 -3246723aa648254222 -128 strconv.ParseInt: parsing "-3246723aa648254222": value out of range [range ParseInt "-3246723aa648254222" value out of range]
// 89000 32 3i4uc 127 strconv.ParseInt: parsing "3i4uc": value out of range [range ParseInt "3i4uc" value out of range]
// 90000 17 -77c8e34fefc2 -128 strconv.ParseInt: parsing "-77c8e34fefc2": value out of range [range ParseInt "-77c8e34fefc2" value out of range]
// 91000 2 10010100 127 strconv.ParseInt: parsing "10010100": value out of range [range ParseInt "10010100" value out of range]
// 92000 22 17bh1gd8 127 strconv.ParseInt: parsing "17bh1gd8": value out of range [range ParseInt "17bh1gd8" value out of range]
// 93000 7 -4644154644012655145 -128 strconv.ParseInt: parsing "-4644154644012655145": value out of range [range ParseInt "-4644154644012655145" value out of range]
// 94000 27 f9c 127 strconv.ParseInt: parsing "f9c": value out of range [range ParseInt "f9c" value out of range]
// 95000 12 13986317882b 127 strconv.ParseInt: parsing "13986317882b": value out of range [range ParseInt "13986317882b" value out of range]
// 96000 32 2sfaangkcm4eb 127 strconv.ParseInt: parsing "2sfaangkcm4eb": value out of range [range ParseInt "2sfaangkcm4eb" value out of range]
// 97000 17 917034 127 strconv.ParseInt: parsing "917034": value out of range [range ParseInt "917034" value out of range]
// 98000 2 11111011101000010111110110110101110111001010100 127 strconv.ParseInt: parsing "11111011101000010111110110110101110111001010100": value out of range [range ParseInt "11111011101000010111110110110101110111001010100" value out of range]
// 99000 22 -21 -45 <nil>
// 100000 7 202511661400 127 strconv.ParseInt: parsing "202511661400": value out of range [range ParseInt "202511661400" value out of range]
// 101000 27 27bpl4ohkkj8 127 strconv.ParseInt: parsing "27bpl4ohkkj8": value out of range [range ParseInt "27bpl4ohkkj8" value out of range]
// 102000 12 -19b66 -128 strconv.ParseInt: parsing "-19b66": value out of range [range ParseInt "-19b66" value out of range]
// 103000 32 ltq3ll58 127 strconv.ParseInt: parsing "ltq3ll58": value out of range [range ParseInt "ltq3ll58" value out of range]
// 104000 17 -5c7ggdf173fb984 -128 strconv.ParseInt: parsing "-5c7ggdf173fb984": value out of range [range ParseInt "-5c7ggdf173fb984" value out of range]
// 105000 2 -110000111101110001011011 -128 strconv.ParseInt: parsing "-110000111101110001011011": value out of range [range ParseInt "-110000111101110001011011" value out of range]
// 106000 22 2b3jb9i4df6 127 strconv.ParseInt: parsing "2b3jb9i4df6": value out of range [range ParseInt "2b3jb9i4df6" value out of range]
// 107000 7 43 31 <nil>
// 108000 27 -7he9026 -128 strconv.ParseInt: parsing "-7he9026": value out of range [range ParseInt "-7he9026" value out of range]
// 109000 12 454187a84074a312 127 strconv.ParseInt: parsing "454187a84074a312": value out of range [range ParseInt "454187a84074a312" value out of range]
// 110000 32 14fq 127 strconv.ParseInt: parsing "14fq": value out of range [range ParseInt "14fq" value out of range]
// 111000 17 -68g5b9909a -128 strconv.ParseInt: parsing "-68g5b9909a": value out of range [range ParseInt "-68g5b9909a" value out of range]
// 112000 2 -11100011010000001111010111001100111010010000111110110101100001 -128 strconv.ParseInt: parsing "-11100011010000001111010111001100111010010000111110110101100001": value out of range [range ParseInt "-11100011010000001111010111001100111010010000111110110101100001" value out of range]
// 113000 22 25gb38 127 strconv.ParseInt: parsing "25gb38": value out of range [range ParseInt "25gb38" value out of range]
// 114000 7 -35146542112305445 -128 strconv.ParseInt: parsing "-35146542112305445": value out of range [range ParseInt "-35146542112305445" value out of range]
// 115000 27 48 116 <nil>
// 116000 12 59a94aa16 127 strconv.ParseInt: parsing "59a94aa16": value out of range [range ParseInt "59a94aa16" value out of range]
// 117000 32 -1sqp6hl352mc -128 strconv.ParseInt: parsing "-1sqp6hl352mc": value out of range [range ParseInt "-1sqp6hl352mc" value out of range]
// 118000 17 943e 127 strconv.ParseInt: parsing "943e": value out of range [range ParseInt "943e" value out of range]
// 119000 2 101010001100010101110110011110100111001 127 strconv.ParseInt: parsing "101010001100010101110110011110100111001": value out of range [range ParseInt "101010001100010101110110011110100111001" value out of range]
// 120000 22 1abdaccb10h9dl9 127 strconv.ParseInt: parsing "1abdaccb10h9dl9": value out of range [range ParseInt "1abdaccb10h9dl9" value out of range]
// 121000 7 11653665 127 strconv.ParseInt: parsing "11653665": value out of range [range ParseInt "11653665" value out of range]
// 122000 27 1378e137i7j 127 strconv.ParseInt: parsing "1378e137i7j": value out of range [range ParseInt "1378e137i7j" value out of range]
// 123000 12 -4 -4 <nil>
// 124000 32 36gqmbm 127 strconv.ParseInt: parsing "36gqmbm": value out of range [range ParseInt "36gqmbm" value out of range]
// 125000 17 4c760gf603c52g 127 strconv.ParseInt: parsing "4c760gf603c52g": value out of range [range ParseInt "4c760gf603c52g" value out of range]
// 126000 2 -111000100100010 -128 strconv.ParseInt: parsing "-111000100100010": value out of range [range ParseInt "-111000100100010" value out of range]
// 127000 22 f66g272h6 127 strconv.ParseInt: parsing "f66g272h6": value out of range [range ParseInt "f66g272h6" value out of range]
// 128000 7 14244121110666153506052 127 strconv.ParseInt: parsing "14244121110666153506052": value out of range [range ParseInt "14244121110666153506052" value out of range]
// 129000 27 -h21lm -128 strconv.ParseInt: parsing "-h21lm": value out of range [range ParseInt "-h21lm" value out of range]
// 130000 12 85a559098b884 127 strconv.ParseInt: parsing "85a559098b884": value out of range [range ParseInt "85a559098b884" value out of range]
// 131000 32 1n 55 <nil>
// 132000 17 -9572efc0 -128 strconv.ParseInt: parsing "-9572efc0": value out of range [range ParseInt "-9572efc0" value out of range]
// 133000 2 111100101111010011111100110101000000101100101010110011 127 strconv.ParseInt: parsing "111100101111010011111100110101000000101100101010110011": value out of range [range ParseInt "111100101111010011111100110101000000101100101010110011" value out of range]
// 134000 22 4d25 127 strconv.ParseInt: parsing "4d25": value out of range [range ParseInt "4d25" value out of range]
// 135000 7 -64466302253340 -128 strconv.ParseInt: parsing "-64466302253340": value out of range [range ParseInt "-64466302253340" value out of range]
// 136000 27 -8hjl4n8p9e908 -128 strconv.ParseInt: parsing "-8hjl4n8p9e908": value out of range [range ParseInt "-8hjl4n8p9e908" value out of range]
// 137000 12 5517531 127 strconv.ParseInt: parsing "5517531": value out of range [range ParseInt "5517531" value out of range]
// 138000 32 -7i13eajij6 -128 strconv.ParseInt: parsing "-7i13eajij6": value out of range [range ParseInt "-7i13eajij6" value out of range]
// 139000 17 c1 127 strconv.ParseInt: parsing "c1": value out of range [range ParseInt "c1" value out of range]
// 140000 2 1011100110100111001001110001100 127 strconv.ParseInt: parsing "1011100110100111001001110001100": value out of range [range ParseInt "1011100110100111001001110001100" value out of range]
// 141000 22 -b5h1jhf3fbgk -128 strconv.ParseInt: parsing "-b5h1jhf3fbgk": value out of range [range ParseInt "-b5h1jhf3fbgk" value out of range]
// 142000 7 26400 127 strconv.ParseInt: parsing "26400": value out of range [range ParseInt "26400" value out of range]
// 143000 27 1gi5eadl8 127 strconv.ParseInt: parsing "1gi5eadl8": value out of range [range ParseInt "1gi5eadl8" value out of range]
// 144000 12 -118ab701027a663a5b -128 strconv.ParseInt: parsing "-118ab701027a663a5b": value out of range [range ParseInt "-118ab701027a663a5b" value out of range]
// 145000 32 8i784 127 strconv.ParseInt: parsing "8i784": value out of range [range ParseInt "8i784" value out of range]
// 146000 17 29d169106d86 127 strconv.ParseInt: parsing "29d169106d86": value out of range [range ParseInt "29d169106d86" value out of range]
// 147000 2 -10000001 -128 strconv.ParseInt: parsing "-10000001": value out of range [range ParseInt "-10000001" value out of range]
// 148000 22 10l9agl0 127 strconv.ParseInt: parsing "10l9agl0": value out of range [range ParseInt "10l9agl0" value out of range]
// 149000 7 25223062222242501404 127 strconv.ParseInt: parsing "25223062222242501404": value out of range [range ParseInt "25223062222242501404" value out of range]
// 150000 27 -62b -128 strconv.ParseInt: parsing "-62b": value out of range [range ParseInt "-62b" value out of range]
// 151000 12 10a4a6006892 127 strconv.ParseInt: parsing "10a4a6006892": value out of range [range ParseInt "10a4a6006892" value out of range]
// 152000 32 -4rh60lplj4ms6 -128 strconv.ParseInt: parsing "-4rh60lplj4ms6": value out of range [range ParseInt "-4rh60lplj4ms6" value out of range]
// 153000 17 -acfa1b -128 strconv.ParseInt: parsing "-acfa1b": value out of range [range ParseInt "-acfa1b" value out of range]
// 154000 2 111010111010101001001000001000011000110010 127 strconv.ParseInt: parsing "111010111010101001001000001000011000110010": value out of range [range ParseInt "111010111010101001001000001000011000110010" value out of range]
// 155000 22 b6 127 strconv.ParseInt: parsing "b6": value out of range [range ParseInt "b6" value out of range]
// 156000 7 -135001632500 -128 strconv.ParseInt: parsing "-135001632500": value out of range [range ParseInt "-135001632500" value out of range]
// 157000 27 9n7j3ipima98 127 strconv.ParseInt: parsing "9n7j3ipima98": value out of range [range ParseInt "9n7j3ipima98" value out of range]
// 158000 12 2767b 127 strconv.ParseInt: parsing "2767b": value out of range [range ParseInt "2767b" value out of range]
// 159000 32 -nmo7mn47 -128 strconv.ParseInt: parsing "-nmo7mn47": value out of range [range ParseInt "-nmo7mn47" value out of range]
// 160000 17 2afb18g65gacba64 127 strconv.ParseInt: parsing "2afb18g65gacba64": value out of range [range ParseInt "2afb18g65gacba64" value out of range]
// 161000 2 101110000001110001000010 127 strconv.ParseInt: parsing "101110000001110001000010": value out of range [range ParseInt "101110000001110001000010" value out of range]
// 162000 22 -3ceccc86gge -128 strconv.ParseInt: parsing "-3ceccc86gge": value out of range [range ParseInt "-3ceccc86gge" value out of range]
// 163000 7 416 127 strconv.ParseInt: parsing "416": value out of range [range ParseInt "416" value out of range]
// 164000 27 2pcmq9b 127 strconv.ParseInt: parsing "2pcmq9b": value out of range [range ParseInt "2pcmq9b" value out of range]
// 165000 12 -1b9113bb882a9a46 -128 strconv.ParseInt: parsing "-1b9113bb882a9a46": value out of range [range ParseInt "-1b9113bb882a9a46" value out of range]
// 166000 32 1gtm 127 strconv.ParseInt: parsing "1gtm": value out of range [range ParseInt "1gtm" value out of range]
// 167000 17 46dg8b1e93 127 strconv.ParseInt: parsing "46dg8b1e93": value out of range [range ParseInt "46dg8b1e93" value out of range]
// 168000 2 1111000110110000010110010010110001100000110111001000100001101 127 strconv.ParseInt: parsing "1111000110110000010110010010110001100000110111001000100001101": value out of range [range ParseInt "1111000110110000010110010010110001100000110111001000100001101" value out of range]
// 169000 22 16h28l 127 strconv.ParseInt: parsing "16h28l": value out of range [range ParseInt "16h28l" value out of range]
// 170000 7 56334040652506302 127 strconv.ParseInt: parsing "56334040652506302": value out of range [range ParseInt "56334040652506302" value out of range]
// 171000 27 -98 -128 strconv.ParseInt: parsing "-98": value out of range [range ParseInt "-98" value out of range]
// 172000 12 9a30ba605 127 strconv.ParseInt: parsing "9a30ba605": value out of range [range ParseInt "9a30ba605" value out of range]
// 173000 32 nue3k2ch17p 127 strconv.ParseInt: parsing "nue3k2ch17p": value out of range [range ParseInt "nue3k2ch17p" value out of range]
// 174000 17 -661 -128 strconv.ParseInt: parsing "-661": value out of range [range ParseInt "-661" value out of range]
// 175000 2 101100111000110100111101101101000001001 127 strconv.ParseInt: parsing "101100111000110100111101101101000001001": value out of range [range ParseInt "101100111000110100111101101101000001001" value out of range]
// 176000 22 3d0j549baia0ee 127 strconv.ParseInt: parsing "3d0j549baia0ee": value out of range [range ParseInt "3d0j549baia0ee" value out of range]
// 177000 7 -113126346 -128 strconv.ParseInt: parsing "-113126346": value out of range [range ParseInt "-113126346" value out of range]
// 178000 27 6nlqe69bac 127 strconv.ParseInt: parsing "6nlqe69bac": value out of range [range ParseInt "6nlqe69bac" value out of range]
// 179000 12 51 61 <nil>
// 180000 32 -3gc95ak -128 strconv.ParseInt: parsing "-3gc95ak": value out of range [range ParseInt "-3gc95ak" value out of range]
// 181000 17 282226c2ec0d0 127 strconv.ParseInt: parsing "282226c2ec0d0": value out of range [range ParseInt "282226c2ec0d0" value out of range]
// 182000 2 1100111111001 127 strconv.ParseInt: parsing "1100111111001": value out of range [range ParseInt "1100111111001" value out of range]
// 183000 22 -a9gfail73 -128 strconv.ParseInt: parsing "-a9gfail73": value out of range [range ParseInt "-a9gfail73" value out of range]
// 184000 7 3023655433016236604650 127 strconv.ParseInt: parsing "3023655433016236604650": value out of range [range ParseInt "3023655433016236604650" value out of range]
// 185000 27 im3p6 127 strconv.ParseInt: parsing "im3p6": value out of range [range ParseInt "im3p6" value out of range]
// 186000 12 -1036951b23bb92 -128 strconv.ParseInt: parsing "-1036951b23bb92": value out of range [range ParseInt "-1036951b23bb92" value out of range]
// 187000 32 14 36 <nil>
// 188000 17 5351e841 127 strconv.ParseInt: parsing "5351e841": value out of range [range ParseInt "5351e841" value out of range]
// 189000 2 -1101111101100111101100001111010111001100100011011000110 -128 strconv.ParseInt: parsing "-1101111101100111101100001111010111001100100011011000110": value out of range [range ParseInt "-1101111101100111101100001111010111001100100011011000110" value out of range]
// 190000 22 2e1i 127 strconv.ParseInt: parsing "2e1i": value out of range [range ParseInt "2e1i" value out of range]
// 191000 7 36552641625550 127 strconv.ParseInt: parsing "36552641625550": value out of range [range ParseInt "36552641625550" value out of range]
// 192000 27 mhggpl1gg5ape 127 strconv.ParseInt: parsing "mhggpl1gg5ape": value out of range [range ParseInt "mhggpl1gg5ape" value out of range]
// 193000 12 30131b3 127 strconv.ParseInt: parsing "30131b3": value out of range [range ParseInt "30131b3" value out of range]
// 194000 32 2dgdp5758m 127 strconv.ParseInt: parsing "2dgdp5758m": value out of range [range ParseInt "2dgdp5758m" value out of range]
// 195000 17 -6d -115 <nil>
// 196000 2 10101110001110111001111000010110 127 strconv.ParseInt: parsing "10101110001110111001111000010110": value out of range [range ParseInt "10101110001110111001111000010110" value out of range]
// 197000 22 k84ek5glcadf 127 strconv.ParseInt: parsing "k84ek5glcadf": value out of range [range ParseInt "k84ek5glcadf" value out of range]
// 198000 7 -133030 -128 strconv.ParseInt: parsing "-133030": value out of range [range ParseInt "-133030" value out of range]
// 199000 27 img66fli 127 strconv.ParseInt: parsing "img66fli": value out of range [range ParseInt "img66fli" value out of range]
// 200000 12 2a333097a8b006136b 127 strconv.ParseInt: parsing "2a333097a8b006136b": value out of range [range ParseInt "2a333097a8b006136b" value out of range]
// 201000 32 -9q820 -128 strconv.ParseInt: parsing "-9q820": value out of range [range ParseInt "-9q820" value out of range]
// 202000 17 39420g54cegg 127 strconv.ParseInt: parsing "39420g54cegg": value out of range [range ParseInt "39420g54cegg" value out of range]
// 203000 2 1110010 114 <nil>
// 204000 22 -14ge76e8 -128 strconv.ParseInt: parsing "-14ge76e8": value out of range [range ParseInt "-14ge76e8" value out of range]
// 205000 7 10552603634041334516 127 strconv.ParseInt: parsing "10552603634041334516": value out of range [range ParseInt "10552603634041334516" value out of range]
// 206000 27 2858 127 strconv.ParseInt: parsing "2858": value out of range [range ParseInt "2858" value out of range]
// 207000 12 -1323a5b95499 -128 strconv.ParseInt: parsing "-1323a5b95499": value out of range [range ParseInt "-1323a5b95499" value out of range]
// 208000 32 -1qmhbg9o09lms -128 strconv.ParseInt: parsing "-1qmhbg9o09lms": value out of range [range ParseInt "-1qmhbg9o09lms" value out of range]
// 209000 17 68c7d7 127 strconv.ParseInt: parsing "68c7d7": value out of range [range ParseInt "68c7d7" value out of range]
// 210000 2 -110000100000010010100111010100111111101010000011 -128 strconv.ParseInt: parsing "-110000100000010010100111010100111111101010000011": value out of range [range ParseInt "-110000100000010010100111010100111111101010000011" value out of range]
// 211000 22 ai 127 strconv.ParseInt: parsing "ai": value out of range [range ParseInt "ai" value out of range]
// 212000 7 3365332356 127 strconv.ParseInt: parsing "3365332356": value out of range [range ParseInt "3365332356" value out of range]
// 213000 27 -1nikfqo64ecf -128 strconv.ParseInt: parsing "-1nikfqo64ecf": value out of range [range ParseInt "-1nikfqo64ecf" value out of range]
// 214000 12 28301 127 strconv.ParseInt: parsing "28301": value out of range [range ParseInt "28301" value out of range]
// 215000 32 3n025plj 127 strconv.ParseInt: parsing "3n025plj": value out of range [range ParseInt "3n025plj" value out of range]
// 216000 17 -10363e9f324f2b86 -128 strconv.ParseInt: parsing "-10363e9f324f2b86": value out of range [range ParseInt "-10363e9f324f2b86" value out of range]
// 217000 2 100111100111111000110011 127 strconv.ParseInt: parsing "100111100111111000110011": value out of range [range ParseInt "100111100111111000110011" value out of range]
// 218000 22 a8k6gkhclgf 127 strconv.ParseInt: parsing "a8k6gkhclgf": value out of range [range ParseInt "a8k6gkhclgf" value out of range]
// 219000 7 -66 -48 <nil>
// 220000 27 145aal0 127 strconv.ParseInt: parsing "145aal0": value out of range [range ParseInt "145aal0" value out of range]
// 221000 12 422b4248820388b8 127 strconv.ParseInt: parsing "422b4248820388b8": value out of range [range ParseInt "422b4248820388b8" value out of range]
// 222000 32 -c3n -128 strconv.ParseInt: parsing "-c3n": value out of range [range ParseInt "-c3n" value out of range]
// 223000 17 3eg99b2a20 127 strconv.ParseInt: parsing "3eg99b2a20": value out of range [range ParseInt "3eg99b2a20" value out of range]
// 224000 2 -111000010000101100010011000001100110100111110101110010101010001 -128 strconv.ParseInt: parsing "-111000010000101100010011000001100110100111110101110010101010001": value out of range [range ParseInt "-111000010000101100010011000001100110100111110101110010101010001" value out of range]
// 225000 22 -31hb6k -128 strconv.ParseInt: parsing "-31hb6k": value out of range [range ParseInt "-31hb6k" value out of range]
// 226000 7 1622646665214415 127 strconv.ParseInt: parsing "1622646665214415": value out of range [range ParseInt "1622646665214415" value out of range]
// 227000 27 8q 127 strconv.ParseInt: parsing "8q": value out of range [range ParseInt "8q" value out of range]
// 228000 12 -631369394 -128 strconv.ParseInt: parsing "-631369394": value out of range [range ParseInt "-631369394" value out of range]
// 229000 32 1n3oal04q8o9 127 strconv.ParseInt: parsing "1n3oal04q8o9": value out of range [range ParseInt "1n3oal04q8o9" value out of range]
// 230000 17 2373 127 strconv.ParseInt: parsing "2373": value out of range [range ParseInt "2373" value out of range]
// 231000 2 -10111110100011110100011100000011100101 -128 strconv.ParseInt: parsing "-10111110100011110100011100000011100101": value out of range [range ParseInt "-10111110100011110100011100000011100101" value out of range]
// 232000 22 kje268ec13lh76 127 strconv.ParseInt: parsing "kje268ec13lh76": value out of range [range ParseInt "kje268ec13lh76" value out of range]
// 233000 7 152102330 127 strconv.ParseInt: parsing "152102330": value out of range [range ParseInt "152102330" value out of range]
// 234000 27 -cclh4of739 -128 strconv.ParseInt: parsing "-cclh4of739": value out of range [range ParseInt "-cclh4of739" value out of range]
// 235000 12 101 127 strconv.ParseInt: parsing "101": value out of range [range ParseInt "101" value out of range]
// 236000 32 2j0o16 127 strconv.ParseInt: parsing "2j0o16": value out of range [range ParseInt "2j0o16" value out of range]
// 237000 17 -360a2566743ce6 -128 strconv.ParseInt: parsing "-360a2566743ce6": value out of range [range ParseInt "-360a2566743ce6" value out of range]
// 238000 2 1010010000100000 127 strconv.ParseInt: parsing "1010010000100000": value out of range [range ParseInt "1010010000100000" value out of range]
// 239000 22 eaig801k 127 strconv.ParseInt: parsing "eaig801k": value out of range [range ParseInt "eaig801k" value out of range]
// 240000 7 11410640142652341552331 127 strconv.ParseInt: parsing "11410640142652341552331": value out of range [range ParseInt "11410640142652341552331" value out of range]
// 241000 27 p736e 127 strconv.ParseInt: parsing "p736e": value out of range [range ParseInt "p736e" value out of range]
// 242000 12 b2803a4808271 127 strconv.ParseInt: parsing "b2803a4808271": value out of range [range ParseInt "b2803a4808271" value out of range]
// 243000 32 -6c -128 strconv.ParseInt: parsing "-6c": value out of range [range ParseInt "-6c" value out of range]
// 244000 17 4e91630a 127 strconv.ParseInt: parsing "4e91630a": value out of range [range ParseInt "4e91630a" value out of range]
// 245000 2 10110110100111010000110000101111111010000000101100 127 strconv.ParseInt: parsing "10110110100111010000110000101111111010000000101100": value out of range [range ParseInt "10110110100111010000110000101111111010000000101100" value out of range]
// 246000 22 -5298 -128 strconv.ParseInt: parsing "-5298": value out of range [range ParseInt "-5298" value out of range]
// 247000 7 44420622060136 127 strconv.ParseInt: parsing "44420622060136": value out of range [range ParseInt "44420622060136" value out of range]
// 248000 27 lgjno24lfa27i 127 strconv.ParseInt: parsing "lgjno24lfa27i": value out of range [range ParseInt "lgjno24lfa27i" value out of range]
// 249000 12 -118005a -128 strconv.ParseInt: parsing "-118005a": value out of range [range ParseInt "-118005a" value out of range]
// 250000 32 3fbd1qec2q 127 strconv.ParseInt: parsing "3fbd1qec2q": value out of range [range ParseInt "3fbd1qec2q" value out of range]
// 251000 17 b1 127 strconv.ParseInt: parsing "b1": value out of range [range ParseInt "b1" value out of range]
// 252000 2 -1100101000111100101111001100 -128 strconv.ParseInt: parsing "-1100101000111100101111001100": value out of range [range ParseInt "-1100101000111100101111001100" value out of range]
// 253000 22 45f7lilajhhk 127 strconv.ParseInt: parsing "45f7lilajhhk": value out of range [range ParseInt "45f7lilajhhk" value out of range]
// 254000 7 21034 127 strconv.ParseInt: parsing "21034": value out of range [range ParseInt "21034" value out of range]
// 255000 27 -2522al7ni -128 strconv.ParseInt: parsing "-2522al7ni": value out of range [range ParseInt "-2522al7ni" value out of range]
// 256000 12 -33a622403891a606a0 -128 strconv.ParseInt: parsing "-33a622403891a606a0": value out of range [range ParseInt "-33a622403891a606a0" value out of range]
// 257000 32 1n0qt 127 strconv.ParseInt: parsing "1n0qt": value out of range [range ParseInt "1n0qt" value out of range]
// 258000 17 -14a613cb6e1c -128 strconv.ParseInt: parsing "-14a613cb6e1c": value out of range [range ParseInt "-14a613cb6e1c" value out of range]
// 259000 2 1100 12 <nil>
// 260000 22 1823bb8i 127 strconv.ParseInt: parsing "1823bb8i": value out of range [range ParseInt "1823bb8i" value out of range]
// 261000 7 -55356531261155553133 -128 strconv.ParseInt: parsing "-55356531261155553133": value out of range [range ParseInt "-55356531261155553133" value out of range]
// 262000 27 25f9 127 strconv.ParseInt: parsing "25f9": value out of range [range ParseInt "25f9" value out of range]
// 263000 12 1427a4969627 127 strconv.ParseInt: parsing "1427a4969627": value out of range [range ParseInt "1427a4969627" value out of range]
// 264000 32 6kfnvfei6ao86 127 strconv.ParseInt: parsing "6kfnvfei6ao86": value out of range [range ParseInt "6kfnvfei6ao86" value out of range]
// 265000 17 2c3c2e 127 strconv.ParseInt: parsing "2c3c2e": value out of range [range ParseInt "2c3c2e" value out of range]
// 266000 2 10110000001110111011011011001001001100010110001 127 strconv.ParseInt: parsing "10110000001110111011011011001001001100010110001": value out of range [range ParseInt "10110000001110111011011011001001001100010110001" value out of range]
// 267000 22 -9b -128 strconv.ParseInt: parsing "-9b": value out of range [range ParseInt "-9b" value out of range]
// 268000 7 154433461431 127 strconv.ParseInt: parsing "154433461431": value out of range [range ParseInt "154433461431" value out of range]
// 269000 27 10mabj4ji1am 127 strconv.ParseInt: parsing "10mabj4ji1am": value out of range [range ParseInt "10mabj4ji1am" value out of range]
// 270000 12 -4456 -128 strconv.ParseInt: parsing "-4456": value out of range [range ParseInt "-4456" value out of range]
// 271000 32 8g3hbj4 127 strconv.ParseInt: parsing "8g3hbj4": value out of range [range ParseInt "8g3hbj4" value out of range]
// 272000 17 -13c32dfb5586c0c2 -128 strconv.ParseInt: parsing "-13c32dfb5586c0c2": value out of range [range ParseInt "-13c32dfb5586c0c2" value out of range]
// 273000 2 -1110101010100011011000 -128 strconv.ParseInt: parsing "-1110101010100011011000": value out of range [range ParseInt "-1110101010100011011000" value out of range]
// 274000 22 a7l294g71f 127 strconv.ParseInt: parsing "a7l294g71f": value out of range [range ParseInt "a7l294g71f" value out of range]
// 275000 7 422 127 strconv.ParseInt: parsing "422": value out of range [range ParseInt "422" value out of range]
// 276000 27 -5k9o2hd -128 strconv.ParseInt: parsing "-5k9o2hd": value out of range [range ParseInt "-5k9o2hd" value out of range]
// 277000 12 12971980372a219 127 strconv.ParseInt: parsing "12971980372a219": value out of range [range ParseInt "12971980372a219" value out of range]
// 278000 32 j1q 127 strconv.ParseInt: parsing "j1q": value out of range [range ParseInt "j1q" value out of range]
// 279000 17 -105bb54a5a -128 strconv.ParseInt: parsing "-105bb54a5a": value out of range [range ParseInt "-105bb54a5a" value out of range]
// 280000 2 1001000101000000110001010101111010010101010100111100000100011 127 strconv.ParseInt: parsing "1001000101000000110001010101111010010101010100111100000100011": value out of range [range ParseInt "1001000101000000110001010101111010010101010100111100000100011" value out of range]
// 281000 22 328j21 127 strconv.ParseInt: parsing "328j21": value out of range [range ParseInt "328j21" value out of range]
// 282000 7 -15526233026465266 -128 strconv.ParseInt: parsing "-15526233026465266": value out of range [range ParseInt "-15526233026465266" value out of range]
// 283000 27 80 127 strconv.ParseInt: parsing "80": value out of range [range ParseInt "80" value out of range]
// 284000 12 9699b3241 127 strconv.ParseInt: parsing "9699b3241": value out of range [range ParseInt "9699b3241" value out of range]
// 285000 32 -1uu7abjgji1j -128 strconv.ParseInt: parsing "-1uu7abjgji1j": value out of range [range ParseInt "-1uu7abjgji1j" value out of range]
// 286000 17 43b4 127 strconv.ParseInt: parsing "43b4": value out of range [range ParseInt "43b4" value out of range]
// 287000 2 110011000111011100111011011101001111001 127 strconv.ParseInt: parsing "110011000111011100111011011101001111001": value out of range [range ParseInt "110011000111011100111011011101001111001" value out of range]
// 288000 22 -38f752h81flk88 -128 strconv.ParseInt: parsing "-38f752h81flk88": value out of range [range ParseInt "-38f752h81flk88" value out of range]
// 289000 7 100310336 127 strconv.ParseInt: parsing "100310336": value out of range [range ParseInt "100310336" value out of range]
// 290000 27 ck247d0oc6 127 strconv.ParseInt: parsing "ck247d0oc6": value out of range [range ParseInt "ck247d0oc6" value out of range]
// 291000 12 -150 -128 strconv.ParseInt: parsing "-150": value out of range [range ParseInt "-150" value out of range]
// 292000 32 2qjabq3 127 strconv.ParseInt: parsing "2qjabq3": value out of range [range ParseInt "2qjabq3" value out of range]
// 293000 17 4ec233b635f57e 127 strconv.ParseInt: parsing "4ec233b635f57e": value out of range [range ParseInt "4ec233b635f57e" value out of range]
// 294000 2 -1100011111111100 -128 strconv.ParseInt: parsing "-1100011111111100": value out of range [range ParseInt "-1100011111111100" value out of range]
// 295000 22 k2fba169 127 strconv.ParseInt: parsing "k2fba169": value out of range [range ParseInt "k2fba169" value out of range]
// 296000 7 -15554202462263613033160 -128 strconv.ParseInt: parsing "-15554202462263613033160": value out of range [range ParseInt "-15554202462263613033160" value out of range]
// 297000 27 -n5l41 -128 strconv.ParseInt: parsing "-n5l41": value out of range [range ParseInt "-n5l41" value out of range]
// 298000 12 168640763a0618 127 strconv.ParseInt: parsing "168640763a0618": value out of range [range ParseInt "168640763a0618" value out of range]
// 299000 32 26 70 <nil>
// generated 8c26c629402eee72 bad=0
