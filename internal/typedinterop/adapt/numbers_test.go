package adapt

import (
	"go/types"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

func TestGoCName(t *testing.T) {
	tests := []struct {
		kind   types.BasicKind
		target ftype.Target
		want   string
	}{
		{types.Int32, ftype.LP64, "int"},
		{types.Uint32, ftype.LP64, "unsigned int"},
		{types.Int64, ftype.LP64, "long"},
		{types.Uint64, ftype.LP64, "unsigned long"},
		{types.Int, ftype.LP64, "long"},
		{types.Int64, ftype.LLP64, "long long"},
		{types.Uint64, ftype.LLP64, "unsigned long long"},
		{types.Int, ftype.LLP64, "long long"},
		{types.Int16, ftype.LLP64, "short"},
		{types.Uint8, ftype.LP64, "unsigned char"},
		{types.Int8, ftype.LP64, "signed char"},
		{types.Float32, ftype.LP64, "float"},
		{types.Float64, ftype.LLP64, "double"},
	}
	for _, tt := range tests {
		if got := GoCName(tt.kind, tt.target); got != tt.want {
			t.Errorf("GoCName(%v, %+v) = %q, want %q", types.Typ[tt.kind], tt.target, got, tt.want)
		}
	}
}

func TestFits(t *testing.T) {
	i8, u8 := numeric{8, true, false}, numeric{8, false, false}
	i16, u16 := numeric{16, true, false}, numeric{16, false, false}
	i32, u32 := numeric{32, true, false}, numeric{32, false, false}
	i64, u64 := numeric{64, true, false}, numeric{64, false, false}
	f32, f64 := numeric{32, true, true}, numeric{64, true, true}
	tests := []struct {
		name string
		a, b numeric
		want bool
	}{
		{"same", i32, i32, true},
		{"wider signed", i32, i64, true},
		{"narrower signed", i64, i32, false},
		{"unsigned into wider signed", u32, i64, true},
		{"unsigned into same-width signed", u32, i32, false},
		{"signed into unsigned", i8, u64, false},
		{"unsigned into wider unsigned", u16, u32, true},
		{"unsigned into narrower unsigned", u64, u8, false},
		{"u8 into i16", u8, i16, true},
		{"float widens", f32, f64, true},
		{"float narrows", f64, f32, false},
		{"integer into float", i32, f64, false},
		{"float into integer", f32, i64, false},
	}
	for _, tt := range tests {
		if got := fits(tt.a, tt.b); got != tt.want {
			t.Errorf("%s: fits(%+v, %+v) = %v, want %v", tt.name, tt.a, tt.b, got, tt.want)
		}
	}
}

func TestIntRange(t *testing.T) {
	tests := []struct {
		n      numeric
		lo, hi string
	}{
		{numeric{8, true, false}, "-128", "127"},
		{numeric{8, false, false}, "0", "255"},
		{numeric{16, true, false}, "-32768", "32767"},
		{numeric{32, false, false}, "0", "4294967295"},
		{numeric{64, true, false}, "-9223372036854775808", "9223372036854775807"},
		{numeric{64, false, false}, "0", "18446744073709551615"},
	}
	for _, tt := range tests {
		lo, hi, ok := intRange(tt.n)
		if !ok || lo.String() != tt.lo || hi.String() != tt.hi {
			t.Errorf("intRange(%+v) = %v, %v, %v; want %s, %s", tt.n, lo, hi, ok, tt.lo, tt.hi)
		}
	}
	if _, _, ok := intRange(numeric{32, true, true}); ok {
		t.Errorf("intRange of a float is a range")
	}
}

func TestCgoName(t *testing.T) {
	for in, want := range map[string]string{
		"char":               "char",
		"signed char":        "schar",
		"unsigned char":      "uchar",
		"int":                "int",
		"unsigned int":       "uint",
		"long long":          "longlong",
		"unsigned long long": "ulonglong",
		"float":              "float",
		"bool":               "bool",
	} {
		if got := CgoName(in); got != want {
			t.Errorf("CgoName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNumericOfPointerWidth(t *testing.T) {
	// Go's int is as wide as a pointer: 64 bits on both x86-64 targets, 32 on a 32-bit one.
	n, ok := numericOf(types.Int, ftype.LP64)
	if !ok || n.bits != 64 || !n.signed {
		t.Errorf("int on LP64 = %+v, %v", n, ok)
	}
	n, ok = numericOf(types.Uintptr, ftype.ILP32)
	if !ok || n.bits != 32 || n.signed {
		t.Errorf("uintptr on ILP32 = %+v, %v", n, ok)
	}
	if _, ok := numericOf(types.String, ftype.LP64); ok {
		t.Errorf("a string is a number")
	}
}
