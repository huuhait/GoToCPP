// phase: 09
// Go 1.25's strconv/atoi_test.go (the tables of ParseInt, ParseUint, Atoi, errors and bit sizes) through a small harness; expected output from real Go.

package main

import (
	"errors"
	"fmt"
	"reflect"
	. "strconv"
)

var BenchSink int

type parseUint64Test struct {
	in  string
	out uint64
	err error
}

var parseUint64Tests = []parseUint64Test{
	{"", 0, ErrSyntax},
	{"0", 0, nil},
	{"1", 1, nil},
	{"12345", 12345, nil},
	{"012345", 12345, nil},
	{"12345x", 0, ErrSyntax},
	{"98765432100", 98765432100, nil},
	{"18446744073709551615", 1<<64 - 1, nil},
	{"18446744073709551616", 1<<64 - 1, ErrRange},
	{"18446744073709551620", 1<<64 - 1, ErrRange},
	{"1_2_3_4_5", 0, ErrSyntax}, // base=10 so no underscores allowed
	{"_12345", 0, ErrSyntax},
	{"1__2345", 0, ErrSyntax},
	{"12345_", 0, ErrSyntax},
	{"-0", 0, ErrSyntax},
	{"-1", 0, ErrSyntax},
	{"+1", 0, ErrSyntax},
}

type parseUint64BaseTest struct {
	in   string
	base int
	out  uint64
	err  error
}

var parseUint64BaseTests = []parseUint64BaseTest{
	{"", 0, 0, ErrSyntax},
	{"0", 0, 0, nil},
	{"0x", 0, 0, ErrSyntax},
	{"0X", 0, 0, ErrSyntax},
	{"1", 0, 1, nil},
	{"12345", 0, 12345, nil},
	{"012345", 0, 012345, nil},
	{"0x12345", 0, 0x12345, nil},
	{"0X12345", 0, 0x12345, nil},
	{"12345x", 0, 0, ErrSyntax},
	{"0xabcdefg123", 0, 0, ErrSyntax},
	{"123456789abc", 0, 0, ErrSyntax},
	{"98765432100", 0, 98765432100, nil},
	{"18446744073709551615", 0, 1<<64 - 1, nil},
	{"18446744073709551616", 0, 1<<64 - 1, ErrRange},
	{"18446744073709551620", 0, 1<<64 - 1, ErrRange},
	{"0xFFFFFFFFFFFFFFFF", 0, 1<<64 - 1, nil},
	{"0x10000000000000000", 0, 1<<64 - 1, ErrRange},
	{"01777777777777777777777", 0, 1<<64 - 1, nil},
	{"01777777777777777777778", 0, 0, ErrSyntax},
	{"02000000000000000000000", 0, 1<<64 - 1, ErrRange},
	{"0200000000000000000000", 0, 1 << 61, nil},
	{"0b", 0, 0, ErrSyntax},
	{"0B", 0, 0, ErrSyntax},
	{"0b101", 0, 5, nil},
	{"0B101", 0, 5, nil},
	{"0o", 0, 0, ErrSyntax},
	{"0O", 0, 0, ErrSyntax},
	{"0o377", 0, 255, nil},
	{"0O377", 0, 255, nil},

	// underscores allowed with base == 0 only
	{"1_2_3_4_5", 0, 12345, nil}, // base 0 => 10
	{"_12345", 0, 0, ErrSyntax},
	{"1__2345", 0, 0, ErrSyntax},
	{"12345_", 0, 0, ErrSyntax},

	{"1_2_3_4_5", 10, 0, ErrSyntax}, // base 10
	{"_12345", 10, 0, ErrSyntax},
	{"1__2345", 10, 0, ErrSyntax},
	{"12345_", 10, 0, ErrSyntax},

	{"0x_1_2_3_4_5", 0, 0x12345, nil}, // base 0 => 16
	{"_0x12345", 0, 0, ErrSyntax},
	{"0x__12345", 0, 0, ErrSyntax},
	{"0x1__2345", 0, 0, ErrSyntax},
	{"0x1234__5", 0, 0, ErrSyntax},
	{"0x12345_", 0, 0, ErrSyntax},

	{"1_2_3_4_5", 16, 0, ErrSyntax}, // base 16
	{"_12345", 16, 0, ErrSyntax},
	{"1__2345", 16, 0, ErrSyntax},
	{"1234__5", 16, 0, ErrSyntax},
	{"12345_", 16, 0, ErrSyntax},

	{"0_1_2_3_4_5", 0, 012345, nil}, // base 0 => 8 (0377)
	{"_012345", 0, 0, ErrSyntax},
	{"0__12345", 0, 0, ErrSyntax},
	{"01234__5", 0, 0, ErrSyntax},
	{"012345_", 0, 0, ErrSyntax},

	{"0o_1_2_3_4_5", 0, 012345, nil}, // base 0 => 8 (0o377)
	{"_0o12345", 0, 0, ErrSyntax},
	{"0o__12345", 0, 0, ErrSyntax},
	{"0o1234__5", 0, 0, ErrSyntax},
	{"0o12345_", 0, 0, ErrSyntax},

	{"0_1_2_3_4_5", 8, 0, ErrSyntax}, // base 8
	{"_012345", 8, 0, ErrSyntax},
	{"0__12345", 8, 0, ErrSyntax},
	{"01234__5", 8, 0, ErrSyntax},
	{"012345_", 8, 0, ErrSyntax},

	{"0b_1_0_1", 0, 5, nil}, // base 0 => 2 (0b101)
	{"_0b101", 0, 0, ErrSyntax},
	{"0b__101", 0, 0, ErrSyntax},
	{"0b1__01", 0, 0, ErrSyntax},
	{"0b10__1", 0, 0, ErrSyntax},
	{"0b101_", 0, 0, ErrSyntax},

	{"1_0_1", 2, 0, ErrSyntax}, // base 2
	{"_101", 2, 0, ErrSyntax},
	{"1_01", 2, 0, ErrSyntax},
	{"10_1", 2, 0, ErrSyntax},
	{"101_", 2, 0, ErrSyntax},
}

type parseInt64Test struct {
	in  string
	out int64
	err error
}

var parseInt64Tests = []parseInt64Test{
	{"", 0, ErrSyntax},
	{"0", 0, nil},
	{"-0", 0, nil},
	{"+0", 0, nil},
	{"1", 1, nil},
	{"-1", -1, nil},
	{"+1", 1, nil},
	{"12345", 12345, nil},
	{"-12345", -12345, nil},
	{"012345", 12345, nil},
	{"-012345", -12345, nil},
	{"98765432100", 98765432100, nil},
	{"-98765432100", -98765432100, nil},
	{"9223372036854775807", 1<<63 - 1, nil},
	{"-9223372036854775807", -(1<<63 - 1), nil},
	{"9223372036854775808", 1<<63 - 1, ErrRange},
	{"-9223372036854775808", -1 << 63, nil},
	{"9223372036854775809", 1<<63 - 1, ErrRange},
	{"-9223372036854775809", -1 << 63, ErrRange},
	{"-1_2_3_4_5", 0, ErrSyntax}, // base=10 so no underscores allowed
	{"-_12345", 0, ErrSyntax},
	{"_12345", 0, ErrSyntax},
	{"1__2345", 0, ErrSyntax},
	{"12345_", 0, ErrSyntax},
	{"123%45", 0, ErrSyntax},
}

type parseInt64BaseTest struct {
	in   string
	base int
	out  int64
	err  error
}

var parseInt64BaseTests = []parseInt64BaseTest{
	{"", 0, 0, ErrSyntax},
	{"0", 0, 0, nil},
	{"-0", 0, 0, nil},
	{"1", 0, 1, nil},
	{"-1", 0, -1, nil},
	{"12345", 0, 12345, nil},
	{"-12345", 0, -12345, nil},
	{"012345", 0, 012345, nil},
	{"-012345", 0, -012345, nil},
	{"0x12345", 0, 0x12345, nil},
	{"-0X12345", 0, -0x12345, nil},
	{"12345x", 0, 0, ErrSyntax},
	{"-12345x", 0, 0, ErrSyntax},
	{"98765432100", 0, 98765432100, nil},
	{"-98765432100", 0, -98765432100, nil},
	{"9223372036854775807", 0, 1<<63 - 1, nil},
	{"-9223372036854775807", 0, -(1<<63 - 1), nil},
	{"9223372036854775808", 0, 1<<63 - 1, ErrRange},
	{"-9223372036854775808", 0, -1 << 63, nil},
	{"9223372036854775809", 0, 1<<63 - 1, ErrRange},
	{"-9223372036854775809", 0, -1 << 63, ErrRange},

	// other bases
	{"g", 17, 16, nil},
	{"10", 25, 25, nil},
	{"holycow", 35, (((((17*35+24)*35+21)*35+34)*35+12)*35+24)*35 + 32, nil},
	{"holycow", 36, (((((17*36+24)*36+21)*36+34)*36+12)*36+24)*36 + 32, nil},

	// base 2
	{"0", 2, 0, nil},
	{"-1", 2, -1, nil},
	{"1010", 2, 10, nil},
	{"1000000000000000", 2, 1 << 15, nil},
	{"111111111111111111111111111111111111111111111111111111111111111", 2, 1<<63 - 1, nil},
	{"1000000000000000000000000000000000000000000000000000000000000000", 2, 1<<63 - 1, ErrRange},
	{"-1000000000000000000000000000000000000000000000000000000000000000", 2, -1 << 63, nil},
	{"-1000000000000000000000000000000000000000000000000000000000000001", 2, -1 << 63, ErrRange},

	// base 8
	{"-10", 8, -8, nil},
	{"57635436545", 8, 057635436545, nil},
	{"100000000", 8, 1 << 24, nil},

	// base 16
	{"10", 16, 16, nil},
	{"-123456789abcdef", 16, -0x123456789abcdef, nil},
	{"7fffffffffffffff", 16, 1<<63 - 1, nil},

	// underscores
	{"-0x_1_2_3_4_5", 0, -0x12345, nil},
	{"0x_1_2_3_4_5", 0, 0x12345, nil},
	{"-_0x12345", 0, 0, ErrSyntax},
	{"_-0x12345", 0, 0, ErrSyntax},
	{"_0x12345", 0, 0, ErrSyntax},
	{"0x__12345", 0, 0, ErrSyntax},
	{"0x1__2345", 0, 0, ErrSyntax},
	{"0x1234__5", 0, 0, ErrSyntax},
	{"0x12345_", 0, 0, ErrSyntax},

	{"-0_1_2_3_4_5", 0, -012345, nil}, // octal
	{"0_1_2_3_4_5", 0, 012345, nil},   // octal
	{"-_012345", 0, 0, ErrSyntax},
	{"_-012345", 0, 0, ErrSyntax},
	{"_012345", 0, 0, ErrSyntax},
	{"0__12345", 0, 0, ErrSyntax},
	{"01234__5", 0, 0, ErrSyntax},
	{"012345_", 0, 0, ErrSyntax},

	{"+0xf", 0, 0xf, nil},
	{"-0xf", 0, -0xf, nil},
	{"0x+f", 0, 0, ErrSyntax},
	{"0x-f", 0, 0, ErrSyntax},
}

type parseUint32Test struct {
	in  string
	out uint32
	err error
}

var parseUint32Tests = []parseUint32Test{
	{"", 0, ErrSyntax},
	{"0", 0, nil},
	{"1", 1, nil},
	{"12345", 12345, nil},
	{"012345", 12345, nil},
	{"12345x", 0, ErrSyntax},
	{"987654321", 987654321, nil},
	{"4294967295", 1<<32 - 1, nil},
	{"4294967296", 1<<32 - 1, ErrRange},
	{"1_2_3_4_5", 0, ErrSyntax}, // base=10 so no underscores allowed
	{"_12345", 0, ErrSyntax},
	{"_12345", 0, ErrSyntax},
	{"1__2345", 0, ErrSyntax},
	{"12345_", 0, ErrSyntax},
}

type parseInt32Test struct {
	in  string
	out int32
	err error
}

var parseInt32Tests = []parseInt32Test{
	{"", 0, ErrSyntax},
	{"0", 0, nil},
	{"-0", 0, nil},
	{"1", 1, nil},
	{"-1", -1, nil},
	{"12345", 12345, nil},
	{"-12345", -12345, nil},
	{"012345", 12345, nil},
	{"-012345", -12345, nil},
	{"12345x", 0, ErrSyntax},
	{"-12345x", 0, ErrSyntax},
	{"987654321", 987654321, nil},
	{"-987654321", -987654321, nil},
	{"2147483647", 1<<31 - 1, nil},
	{"-2147483647", -(1<<31 - 1), nil},
	{"2147483648", 1<<31 - 1, ErrRange},
	{"-2147483648", -1 << 31, nil},
	{"2147483649", 1<<31 - 1, ErrRange},
	{"-2147483649", -1 << 31, ErrRange},
	{"-1_2_3_4_5", 0, ErrSyntax}, // base=10 so no underscores allowed
	{"-_12345", 0, ErrSyntax},
	{"_12345", 0, ErrSyntax},
	{"1__2345", 0, ErrSyntax},
	{"12345_", 0, ErrSyntax},
	{"123%45", 0, ErrSyntax},
}

type numErrorTest struct {
	num, want string
}

var numErrorTests = []numErrorTest{
	{"0", `strconv.ParseFloat: parsing "0": failed`},
	{"`", "strconv.ParseFloat: parsing \"`\": failed"},
	{"1\x00.2", `strconv.ParseFloat: parsing "1\x00.2": failed`},
}

func init() {
	// The parse routines return NumErrors wrapping
	// the error and the string. Convert the tables above.
	for i := range parseUint64Tests {
		test := &parseUint64Tests[i]
		checks++
		if test.err != nil {
			test.err = &NumError{"ParseUint", test.in, test.err}
		}
	}
	for i := range parseUint64BaseTests {
		test := &parseUint64BaseTests[i]
		checks++
		if test.err != nil {
			test.err = &NumError{"ParseUint", test.in, test.err}
		}
	}
	for i := range parseInt64Tests {
		test := &parseInt64Tests[i]
		checks++
		if test.err != nil {
			test.err = &NumError{"ParseInt", test.in, test.err}
		}
	}
	for i := range parseInt64BaseTests {
		test := &parseInt64BaseTests[i]
		checks++
		if test.err != nil {
			test.err = &NumError{"ParseInt", test.in, test.err}
		}
	}
	for i := range parseUint32Tests {
		test := &parseUint32Tests[i]
		checks++
		if test.err != nil {
			test.err = &NumError{"ParseUint", test.in, test.err}
		}
	}
	for i := range parseInt32Tests {
		test := &parseInt32Tests[i]
		checks++
		if test.err != nil {
			test.err = &NumError{"ParseInt", test.in, test.err}
		}
	}
}

func TestParseUint32(t *testT) {
	for i := range parseUint32Tests {
		test := &parseUint32Tests[i]
		out, err := ParseUint(test.in, 10, 32)
		checks++
		if uint64(test.out) != out || !reflect.DeepEqual(test.err, err) {
			t.Errorf("ParseUint(%q, 10, 32) = %v, %v want %v, %v",
				test.in, out, err, test.out, test.err)
		}
	}
}

func TestParseUint64(t *testT) {
	for i := range parseUint64Tests {
		test := &parseUint64Tests[i]
		out, err := ParseUint(test.in, 10, 64)
		checks++
		if test.out != out || !reflect.DeepEqual(test.err, err) {
			t.Errorf("ParseUint(%q, 10, 64) = %v, %v want %v, %v",
				test.in, out, err, test.out, test.err)
		}
	}
}

func TestParseUint64Base(t *testT) {
	for i := range parseUint64BaseTests {
		test := &parseUint64BaseTests[i]
		out, err := ParseUint(test.in, test.base, 64)
		checks++
		if test.out != out || !reflect.DeepEqual(test.err, err) {
			t.Errorf("ParseUint(%q, %v, 64) = %v, %v want %v, %v",
				test.in, test.base, out, err, test.out, test.err)
		}
	}
}

func TestParseInt32(t *testT) {
	for i := range parseInt32Tests {
		test := &parseInt32Tests[i]
		out, err := ParseInt(test.in, 10, 32)
		checks++
		if int64(test.out) != out || !reflect.DeepEqual(test.err, err) {
			t.Errorf("ParseInt(%q, 10 ,32) = %v, %v want %v, %v",
				test.in, out, err, test.out, test.err)
		}
	}
}

func TestParseInt64(t *testT) {
	for i := range parseInt64Tests {
		test := &parseInt64Tests[i]
		out, err := ParseInt(test.in, 10, 64)
		checks++
		if test.out != out || !reflect.DeepEqual(test.err, err) {
			t.Errorf("ParseInt(%q, 10, 64) = %v, %v want %v, %v",
				test.in, out, err, test.out, test.err)
		}
	}
}

func TestParseInt64Base(t *testT) {
	for i := range parseInt64BaseTests {
		test := &parseInt64BaseTests[i]
		out, err := ParseInt(test.in, test.base, 64)
		checks++
		if test.out != out || !reflect.DeepEqual(test.err, err) {
			t.Errorf("ParseInt(%q, %v, 64) = %v, %v want %v, %v",
				test.in, test.base, out, err, test.out, test.err)
		}
	}
}

func TestParseUint(t *testT) {
	switch IntSize {
	case 32:
		for i := range parseUint32Tests {
			test := &parseUint32Tests[i]
			out, err := ParseUint(test.in, 10, 0)
			checks++
			if uint64(test.out) != out || !reflect.DeepEqual(test.err, err) {
				t.Errorf("ParseUint(%q, 10, 0) = %v, %v want %v, %v",
					test.in, out, err, test.out, test.err)
			}
		}
	case 64:
		for i := range parseUint64Tests {
			test := &parseUint64Tests[i]
			out, err := ParseUint(test.in, 10, 0)
			checks++
			if test.out != out || !reflect.DeepEqual(test.err, err) {
				t.Errorf("ParseUint(%q, 10, 0) = %v, %v want %v, %v",
					test.in, out, err, test.out, test.err)
			}
		}
	}
}

func TestParseInt(t *testT) {
	switch IntSize {
	case 32:
		for i := range parseInt32Tests {
			test := &parseInt32Tests[i]
			out, err := ParseInt(test.in, 10, 0)
			checks++
			if int64(test.out) != out || !reflect.DeepEqual(test.err, err) {
				t.Errorf("ParseInt(%q, 10, 0) = %v, %v want %v, %v",
					test.in, out, err, test.out, test.err)
			}
		}
	case 64:
		for i := range parseInt64Tests {
			test := &parseInt64Tests[i]
			out, err := ParseInt(test.in, 10, 0)
			checks++
			if test.out != out || !reflect.DeepEqual(test.err, err) {
				t.Errorf("ParseInt(%q, 10, 0) = %v, %v want %v, %v",
					test.in, out, err, test.out, test.err)
			}
		}
	}
}

func TestAtoi(t *testT) {
	switch IntSize {
	case 32:
		for i := range parseInt32Tests {
			test := &parseInt32Tests[i]
			out, err := Atoi(test.in)
			var testErr error
			checks++
			if test.err != nil {
				testErr = &NumError{"Atoi", test.in, test.err.(*NumError).Err}
			}
			checks++
			if int(test.out) != out || !reflect.DeepEqual(testErr, err) {
				t.Errorf("Atoi(%q) = %v, %v want %v, %v",
					test.in, out, err, test.out, testErr)
			}
		}
	case 64:
		for i := range parseInt64Tests {
			test := &parseInt64Tests[i]
			out, err := Atoi(test.in)
			var testErr error
			checks++
			if test.err != nil {
				testErr = &NumError{"Atoi", test.in, test.err.(*NumError).Err}
			}
			checks++
			if test.out != int64(out) || !reflect.DeepEqual(testErr, err) {
				t.Errorf("Atoi(%q) = %v, %v want %v, %v",
					test.in, out, err, test.out, testErr)
			}
		}
	}
}

func bitSizeErrStub(name string, bitSize int) error {
	return &NumError{name, "0", errors.New("invalid bit size " + Itoa(bitSize))}
}

func baseErrStub(name string, base int) error {
	return &NumError{name, "0", errors.New("invalid base " + Itoa(base))}
}

func noErrStub(name string, arg int) error {
	return nil
}

type parseErrorTest struct {
	arg     int
	errStub func(name string, arg int) error
}

var parseBitSizeTests = []parseErrorTest{
	{-1, bitSizeErrStub},
	{0, noErrStub},
	{64, noErrStub},
	{65, bitSizeErrStub},
}

var parseBaseTests = []parseErrorTest{
	{-1, baseErrStub},
	{0, noErrStub},
	{1, baseErrStub},
	{2, noErrStub},
	{36, noErrStub},
	{37, baseErrStub},
}

func equalError(a, b error) bool {
	checks++
	if a == nil {
		return b == nil
	}
	checks++
	if b == nil {
		return a == nil
	}
	return a.Error() == b.Error()
}

func TestParseIntBitSize(t *testT) {
	for i := range parseBitSizeTests {
		test := &parseBitSizeTests[i]
		testErr := test.errStub("ParseInt", test.arg)
		_, err := ParseInt("0", 0, test.arg)
		checks++
		if !equalError(testErr, err) {
			t.Errorf("ParseInt(\"0\", 0, %v) = 0, %v want 0, %v",
				test.arg, err, testErr)
		}
	}
}

func TestParseUintBitSize(t *testT) {
	for i := range parseBitSizeTests {
		test := &parseBitSizeTests[i]
		testErr := test.errStub("ParseUint", test.arg)
		_, err := ParseUint("0", 0, test.arg)
		checks++
		if !equalError(testErr, err) {
			t.Errorf("ParseUint(\"0\", 0, %v) = 0, %v want 0, %v",
				test.arg, err, testErr)
		}
	}
}

func TestParseIntBase(t *testT) {
	for i := range parseBaseTests {
		test := &parseBaseTests[i]
		testErr := test.errStub("ParseInt", test.arg)
		_, err := ParseInt("0", test.arg, 0)
		checks++
		if !equalError(testErr, err) {
			t.Errorf("ParseInt(\"0\", %v, 0) = 0, %v want 0, %v",
				test.arg, err, testErr)
		}
	}
}

func TestParseUintBase(t *testT) {
	for i := range parseBaseTests {
		test := &parseBaseTests[i]
		testErr := test.errStub("ParseUint", test.arg)
		_, err := ParseUint("0", test.arg, 0)
		checks++
		if !equalError(testErr, err) {
			t.Errorf("ParseUint(\"0\", %v, 0) = 0, %v want 0, %v",
				test.arg, err, testErr)
		}
	}
}

func TestNumError(t *testT) {
	for _, test := range numErrorTests {
		err := &NumError{
			Func: "ParseFloat",
			Num:  test.num,
			Err:  errors.New("failed"),
		}
		checks++
		if got := err.Error(); got != test.want {
			t.Errorf(`(&NumError{"ParseFloat", %q, "failed"}).Error() = %v, want %v`, test.num, got, test.want)
		}
	}
}

func TestNumErrorUnwrap(t *testT) {
	err := &NumError{Err: ErrSyntax}
	checks++
	if !errors.Is(err, ErrSyntax) {
		t.Error("errors.Is failed, wanted success")
	}
}

type benchCase struct {
	name string
	num  int64
}

// ---- a small harness instead of package testing (step 9.9); Go's tests are below, unchanged except as listed above ----

type testT struct {
	name   string
	failed bool
}

type fatalExit struct{}
type skipExit struct{}

var subtests int

// checks counts the if statements that the tests of the file run: a Test that returns early shows in the expected output.
var checks int

func (t *testT) Helper()                           {}
func (t *testT) Parallel()                         {}
func (t *testT) Name() string                      { return t.name }
func (t *testT) Log(args ...any)                   {}
func (t *testT) Logf(format string, args ...any)   {}
func (t *testT) Skip(args ...any)                  { panic(skipExit{}) }
func (t *testT) Skipf(f string, args ...any)       { panic(skipExit{}) }
func (t *testT) report(msg string)                 { t.failed = true; fmt.Println("FAIL", t.name+":", msg) }
func (t *testT) Errorf(format string, args ...any) { t.report(fmt.Sprintf(format, args...)) }
func (t *testT) Error(args ...any)                 { t.report(fmt.Sprint(args...)) }
func (t *testT) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	panic(fatalExit{})
}
func (t *testT) Fatal(args ...any) {
	t.Error(args...)
	panic(fatalExit{})
}

func (t *testT) Run(name string, f func(t *testT)) bool {
	sub := &testT{name: t.name + "/" + name}
	subtests++
	runTest(sub, f)
	if sub.failed {
		t.failed = true
	}
	return !sub.failed
}

func runTest(t *testT, f func(t *testT)) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case fatalExit, skipExit:
			default:
				t.failed = true
				fmt.Println("PANIC", t.name+":", r)
			}
		}
	}()
	f(t)
}

type testCase struct {
	name string
	f    func(t *testT)
}

func runTests(tests []testCase) {
	failed := 0
	for _, tc := range tests {
		t := &testT{name: tc.name}
		subtests = 0
		checks = 0
		runTest(t, tc.f)
		if t.failed {
			failed++
			fmt.Println("--- FAIL:", tc.name)
		} else {
			fmt.Println("ok", tc.name, "subtests:", subtests, "checks:", checks)
		}
	}
	fmt.Println("tests:", len(tests), "failed:", failed)
	if failed > 0 {
		panic("tests failed")
	}
}

// A deterministic generator (splitmix64) in place of math/rand, so that Go and gocpp run the same inputs.
type rngT struct{ s uint64 }

var rng = &rngT{0x9E3779B97F4A7C15}

func (r *rngT) Uint64() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) Uint32() uint32   { return uint32(r.Uint64() >> 32) }
func (r *rngT) Intn(n int) int   { return int(r.Uint64()>>1) % n }
func (r *rngT) Float64() float64 { return float64(r.Uint64()>>11) / (1 << 53) }

func main() {
	tests_ := []testCase{
		{"TestParseUint32", TestParseUint32},
		{"TestParseUint64", TestParseUint64},
		{"TestParseUint64Base", TestParseUint64Base},
		{"TestParseInt32", TestParseInt32},
		{"TestParseInt64", TestParseInt64},
		{"TestParseInt64Base", TestParseInt64Base},
		{"TestParseUint", TestParseUint},
		{"TestParseInt", TestParseInt},
		{"TestAtoi", TestAtoi},
		{"TestParseIntBitSize", TestParseIntBitSize},
		{"TestParseUintBitSize", TestParseUintBitSize},
		{"TestParseIntBase", TestParseIntBase},
		{"TestParseUintBase", TestParseUintBase},
		{"TestNumError", TestNumError},
		{"TestNumErrorUnwrap", TestNumErrorUnwrap},
	}
	runTests(tests_)
}

// Output:
// ok TestParseUint32 subtests: 0 checks: 14
// ok TestParseUint64 subtests: 0 checks: 17
// ok TestParseUint64Base subtests: 0 checks: 75
// ok TestParseInt32 subtests: 0 checks: 25
// ok TestParseInt64 subtests: 0 checks: 25
// ok TestParseInt64Base subtests: 0 checks: 60
// ok TestParseUint subtests: 0 checks: 17
// ok TestParseInt subtests: 0 checks: 25
// ok TestAtoi subtests: 0 checks: 50
// ok TestParseIntBitSize subtests: 0 checks: 10
// ok TestParseUintBitSize subtests: 0 checks: 10
// ok TestParseIntBase subtests: 0 checks: 15
// ok TestParseUintBase subtests: 0 checks: 15
// ok TestNumError subtests: 0 checks: 3
// ok TestNumErrorUnwrap subtests: 0 checks: 1
// tests: 15 failed: 0
