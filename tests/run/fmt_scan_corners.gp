// phase: 09

// Corners of the scanner that the other fmt_scan programs do not reach: look-ahead of a RuneScanner,
// literals that do not match, bad verbs at the end of the input, a width too large, consume without
// accept (%U), a lone 0 prefix, a Scan method that calls Token twice, ranges, and slices of named bytes.
package main

import (
	"fmt"
	"io"
	"math"
	"strings"
)

type tok2 struct{ a, b string }

func (t *tok2) Scan(st fmt.ScanState, verb rune) error {
	x, err := st.Token(true, nil)
	if err != nil {
		return err
	}
	t.a = string(x)
	y, err := st.Token(true, nil)
	if err != nil {
		return err
	}
	t.b = string(y)
	w, ok := st.Width()
	t.b += fmt.Sprint("|", w, ok)
	return nil
}

func main() {
	// S37: slices of a named byte type.
	namedBytes()
	var a, b int
	var s string

	// S22: a RuneScanner keeps the look-ahead rune between calls.
	r := strings.NewReader("12abc 5")
	n, err := fmt.Fscanf(r, "%d", &a)
	fmt.Println("S22", n, err, a)
	n, err = fmt.Fscanf(r, "%s", &s)
	fmt.Println("S22", n, err, s)

	// S28: a literal that does not match is put back into a RuneScanner.
	r2 := strings.NewReader("y5")
	n, err = fmt.Fscanf(r2, "x%d", &a)
	rest, _ := io.ReadAll(r2)
	fmt.Printf("S28 %d %v %q\n", n, err, rest)

	// S29: a bad verb at the end of the input is io.EOF (notEOF comes first).
	n, err = fmt.Sscanf("", "%z", &a)
	fmt.Println("S29", n, err)
	n, err = fmt.Sscanf("   ", "%z", &a)
	fmt.Println("S29b", n, err)

	// S30: the bytes after a bad UTF-8 sequence are kept for the next rune.
	n, err = fmt.Sscan("\xe2\x82x", &s)
	fmt.Printf("S30 %d %v %q\n", n, err, s)

	// S32: a width too large for parsenum is dropped with the rest of the verb.
	n, err = fmt.Sscanf("5 6", "%9999999d %d", &a, &b)
	fmt.Println("S32", n, err, a, b)

	// S34: consume without accept does not put the rune back (%U).
	r3 := strings.NewReader("X+1")
	n, err = fmt.Fscanf(r3, "%U", &a)
	rest, _ = io.ReadAll(r3)
	fmt.Printf("S34 %d %v %q\n", n, err, rest)

	// S13: a lone 0 prefix scans octal digits only.
	a, b = -1, -1
	n, err = fmt.Sscan("089", &a, &b)
	fmt.Println("S13", n, err, a, b)
	n, err = fmt.Sscan("0o17 0O17 0b11 0B11 0x1f 0X1F", &a, &b, &a, &b, &a, &b)
	fmt.Println("S14", n, err, a, b)
	var u8 uint8
	n, err = fmt.Sscan("0B11", &u8)
	fmt.Println("S14u", n, err, u8)

	// S15: a plus sign.
	n, err = fmt.Sscan("+5 -6", &a, &b)
	fmt.Println("S15", n, err, a, b)

	// S25/S26: Token twice in a Scan method, and Width.
	var t tok2
	n, err = fmt.Sscan("one two", &t)
	fmt.Println("S26", n, err, t.a, t.b)
	n, err = fmt.Sscanf("one two", "%5v", &t)
	fmt.Println("S25", n, err, t.a, t.b)

	// S16: int16 and uint16 overflow.
	var i16 int16
	var u16 uint16
	n, err = fmt.Sscan("40000", &i16)
	fmt.Println("S16", n, err, i16)
	n, err = fmt.Sscan("70000", &u16)
	fmt.Println("S16u", n, err, u16)

	// S31: float32 range.
	var f32 float32
	n, err = fmt.Sscan("1e39", &f32)
	fmt.Println("S31", n, err, f32)

	// S18/S19: complex.
	var c complex128
	n, err = fmt.Sscan("(1-2i", &c)
	fmt.Println("S18", n, err, c)
	n, err = fmt.Sscan("1-2i", &c)
	fmt.Println("S19", n, err, c)

	// S24: %x with no hex data.
	n, err = fmt.Sscanf("zz", "%x", &s)
	fmt.Println("S24", n, err)

	// S11/S12: booleans.
	var bo bool
	n, err = fmt.Sscan("tru", &bo)
	fmt.Println("S11", n, err, bo)
	n, err = fmt.Sscan("1", &bo)
	fmt.Println("S12", n, err, bo)

	// S27: a literal at the end of the input.
	n, err = fmt.Sscanf("5", "%dx", &a)
	fmt.Println("S27", n, err, a)

	// S21: \r\n in Sscanln.
	n, err = fmt.Sscanln("5\r\n", &a)
	fmt.Println("S21", n, err, a)

	// The NaN that ParseFloat returns is math.NaN(), whose bits are 0x7ff8000000000001.
	var nan float64
	n, err = fmt.Sscan("NaN", &nan)
	fmt.Printf("NaN %d %v %#x\n", n, err, math.Float64bits(nan))

	// S06: %% literal.
	n, err = fmt.Sscanf("50%", "%d%%", &a)
	fmt.Println("S06", n, err, a)
}

type B byte
type BS []B
type Raw []byte

func namedBytes() {
	var a []B
	var b BS
	var c Raw
	n, err := fmt.Sscan("hi there yo", &a, &b, &c)
	fmt.Println(n, err, a, b, c)
	n, err = fmt.Sscanf("6869 7a", "%x %q", &a, &b)
	fmt.Println(n, err, a, b)
}

// Output:
// 3 <nil> [104 105] [116 104 101 114 101] [121 111]
// 1 expected quoted string [104 105] [116 104 101 114 101]
// S22 1 <nil> 12
// S22 1 <nil> abc
// S28 0 input does not match format "y5"
// S29 0 EOF
// S29b 0 EOF
// S30 1 <nil> "��x"
// S32 2 <nil> 5 6
// S34 0 bad unicode format  "+1"
// S13 2 <nil> 0 89
// S14 6 <nil> 31 31
// S14u 1 <nil> 3
// S15 2 <nil> 5 -6
// S26 1 <nil> one two|0 false
// S25 1 <nil> one t|5 true
// S16 0 integer overflow on token 40000 0
// S16u 0 unsigned integer overflow on token 70000 0
// S31 0 strconv.ParseFloat: parsing "1e39": value out of range 0
// S18 0 syntax error scanning complex number (0+0i)
// S19 1 <nil> (1-2i)
// S24 0 no hex data for %x string
// S11 0 syntax error scanning boolean false
// S12 1 <nil> true
// S27 1 unexpected EOF 5
// S21 1 <nil> 5
// NaN 1 <nil> 0x7ff8000000000001
// S06 1 <nil> 50
