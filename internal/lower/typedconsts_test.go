package lower

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
)

// printCond writes the static_assert of a constant use the way the translation unit has it.
func printCond(t *testing.T, u check.ConstUse) string {
	t.Helper()
	cond, why := constCond(u, cxxName(u.Spelling))
	if cond == nil {
		return "unchecked: " + why
	}
	out := string(cppir.Print(&cppir.File{Decls: []cppir.Decl{&cppir.StaticAssert{Cond: cond, Msg: "m"}}}, ""))
	return strings.TrimSpace(out)
}

func TestConstCondKinds(t *testing.T) {
	big128, _ := new(big.Int).SetString("1267650600228229401496703205376", 10)
	for _, tc := range []struct {
		name string
		u    check.ConstUse
		want string
	}{
		{"int", check.ConstUse{Spelling: "BUFSIZ", Answer: check.ConstAnswer{Kind: check.ConstInt, Int: big.NewInt(8192)}}, `static_assert(static_cast<long long>(BUFSIZ) == 8192LL, "m");`},
		{"negative", check.ConstUse{Spelling: "EOF", Answer: check.ConstAnswer{Kind: check.ConstInt, Int: big.NewInt(-1)}}, `static_assert(static_cast<long long>(EOF) == -1LL, "m");`},
		{"int64 min", check.ConstUse{Spelling: "X", Answer: check.ConstAnswer{Kind: check.ConstInt, Int: big.NewInt(math.MinInt64)}}, `static_assert(static_cast<long long>(X) == (-9223372036854775807LL - 1), "m");`},
		{"uint64", check.ConstUse{Spelling: "X", Answer: check.ConstAnswer{Kind: check.ConstInt, Int: new(big.Int).SetUint64(math.MaxUint64)}}, `static_assert(static_cast<unsigned long long>(X) == 18446744073709551615ULL, "m");`},
		{"wider", check.ConstUse{Spelling: "X", Answer: check.ConstAnswer{Kind: check.ConstInt, Int: big128}}, "unchecked: the value is wider than 64 bits"},
		{"char", check.ConstUse{Spelling: "SEP", Answer: check.ConstAnswer{Kind: check.ConstChar, Int: big.NewInt('/')}}, `static_assert(static_cast<long long>(SEP) == 47LL, "m");`},
		{"bool", check.ConstUse{Spelling: "ON", Answer: check.ConstAnswer{Kind: check.ConstBool, Int: big.NewInt(1)}}, `static_assert(static_cast<long long>(ON) == 1LL, "m");`},
		{"float", check.ConstUse{Spelling: "::kHalf", Value: "0.5", Answer: check.ConstAnswer{Kind: check.ConstFloat, Float: 0.5}}, `static_assert(static_cast<double>(::kHalf) == 0.5, "m");`},
		{"hexfloat", check.ConstUse{Spelling: "M_PI", Value: "0x1.921fb54442d18p+01", Answer: check.ConstAnswer{Kind: check.ConstFloat, Float: math.Pi}}, `static_assert(static_cast<double>(M_PI) == 0x1.921fb54442d18p+01, "m");`},
		{"member", check.ConstUse{Spelling: "(::kPt).x", Answer: check.ConstAnswer{Kind: check.ConstInt, Int: big.NewInt(3)}}, `static_assert(static_cast<long long>((::kPt).x) == 3LL, "m");`},
		{"string", check.ConstUse{Spelling: "NAME", Answer: check.ConstAnswer{Kind: check.ConstString, Str: "a\"b\n"}}, `static_assert(std::string_view(NAME, sizeof(NAME) - 1) == std::string_view("a\"b\012", 4), "m");`},
		{"nul and high bytes", check.ConstUse{Spelling: "S", Answer: check.ConstAnswer{Kind: check.ConstString, Str: "\x00\xe9?1"}}, `static_assert(std::string_view(S, sizeof(S) - 1) == std::string_view("\000\351\0771", 4), "m");`},
		{"none", check.ConstUse{Spelling: "X"}, "unchecked: no value"},
	} {
		if got := printCond(t, tc.u); got != tc.want {
			t.Errorf("%s:\n got %s\nwant %s", tc.name, got, tc.want)
		}
	}
}

func TestCString(t *testing.T) {
	for in, want := range map[string]string{"": `""`, "abc": `"abc"`, `a"\`: `"a\"\\"`, "\n\t": `"\012\011"`, "\x01a": `"\001a"`, "?": `"\077"`, "\xff": `"\377"`, "\x7f": `"\177"`} {
		if got := cString(in); got != want {
			t.Errorf("cString(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestPlain(t *testing.T) {
	if got := plain(`a b"c\d.e-f/g_h9`); got != `a?b?c?d.e-f/g_h9` {
		t.Errorf("plain: %s", got)
	}
}
