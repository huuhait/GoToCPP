package bindgen

import (
	"io"
	"math/big"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

func TestBalanced(t *testing.T) {
	for in, want := range map[string]bool{
		"1":                     true,
		"(1 + 2)":               true,
		"((Color){ 1, 2 })":     true,
		`"a)b"`:                 true,
		`'('`:                   true,
		`"esc\" ("`:             true,
		"{":                     false,
		"1)":                    false,
		"(]":                    false,
		"1;":                    false,
		"#x":                    false,
		`"open`:                 false,
		"f(a, [b]) + {c}":       true,
		"do { x(); } while (0)": false, // has ;
		"((int[2]){ 1, 2 })":    true,
	} {
		if got := balanced(in); got != want {
			t.Errorf("balanced(%q) = %v", in, got)
		}
	}
}

func TestGuardsAndImplementationMacros(t *testing.T) {
	for _, c := range []struct {
		name, body string
		guard      bool
	}{
		{"FOO_H", "", true}, {"_STDIO_H", "1", true}, {"FOO_HPP", "1", true}, {"X_H_", "", true},
		{"FOO_INCLUDED", "", true}, {"FOO_H", "2", false}, {"HEIGHT", "1", false}, {"SIZE_H_VALUE", "1", false},
	} {
		if got := isGuard(&macro{Name: c.name, Body: c.body}); got != c.guard {
			t.Errorf("isGuard(%s %q) = %v", c.name, c.body, got)
		}
	}
	for _, c := range []struct {
		name, file string
		want       bool
	}{
		{"_GLIBCXX_X", "/usr/include/vector", true},
		{"_LIBCPP_X", "/x/y.h", true},
		{"_IOFBF", "/usr/include/stdio.h", false},
		{"_PRIVATE", "/usr/include/bits/x.h", true},
		{"_lower", "/usr/include/bits/x.h", false},
		{"PLAIN", "/usr/include/bits/x.h", false},
		{"_", "/usr/include/bits/x.h", false},
	} {
		if got := implementationMacro(&macro{Name: c.name, File: c.file}); got != c.want {
			t.Errorf("implementationMacro(%s in %s) = %v", c.name, c.file, got)
		}
	}
}

func TestDecodeCString(t *testing.T) {
	good := map[string]string{
		`"abc"`:          "abc",
		`"a\nb\tc"`:      "a\nb\tc",
		`"\x41\101\0"`:   "AA\x00",
		`"q\"\\\'\?"`:    `q"\'?`,
		`"\a\b\f\v\e\r"`: "\a\b\f\v\x1b\r",
		`""`:             "",
	}
	for in, want := range good {
		got, ok := decodeCString(in)
		if !ok || got != want {
			t.Errorf("decodeCString(%s) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{`abc`, `"abc`, `"\q"`, `"abc\"`, `"\xZZ"`, ``} {
		if got, ok := decodeCString(bad); ok {
			t.Errorf("decodeCString(%s) = %q, true", bad, got)
		}
	}
}

func TestLiterals(t *testing.T) {
	for f, want := range map[float64]string{0.5: "0.5", 1: "1.0", 1e300: "1e+300", -2: "-2.0", 3.14159: "3.14159", 1e21: "1e+21"} {
		if got := floatLiteral(f); got != want {
			t.Errorf("floatLiteral(%v) = %q, want %q", f, got, want)
		}
	}
	i := func(s string) *probe.Const {
		v, _ := new(big.Int).SetString(s, 10)
		return &probe.Const{Kind: "int", Int: v}
	}
	for _, c := range []struct {
		c    *probe.Const
		want string
		ok   bool
	}{
		{i("-5"), "-5", true},
		{i("18446744073709551615"), "18446744073709551615", true},
		{&probe.Const{Kind: "bool", Int: big.NewInt(1)}, "true", true},
		{&probe.Const{Kind: "bool", Int: big.NewInt(0)}, "false", true},
		{&probe.Const{Kind: "float", Float: 0.25}, "0.25", true},
		{&probe.Const{Kind: "float", Float: 3}, "3.0", true},
		{&probe.Const{Kind: "string", Str: "a\"b\n\x00é"}, `"a\"b\n\x00é"`, true},
		{&probe.Const{Kind: "string", Str: "\xff"}, `"\xff"`, true},
		{&probe.Const{Kind: "weird"}, "", false},
	} {
		got, ok := constLiteral(c.c)
		if got != c.want || ok != c.ok {
			t.Errorf("constLiteral(%+v) = %q, %v; want %q, %v", c.c, got, ok, c.want, c.ok)
		}
	}
	inf := &probe.Const{Kind: "float", Float: 1 / zero()}
	if _, ok := constLiteral(inf); ok {
		t.Error("an infinity is not a Go constant")
	}
	nan := &probe.Const{Kind: "float", Float: zero() / zero()}
	if _, ok := constLiteral(nan); ok {
		t.Error("NaN is not a Go constant")
	}
}

func zero() float64 { return 0 }

func TestGoIdentKeywords(t *testing.T) {
	for _, kw := range strings.Fields("break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var init") {
		if got := goIdent(kw); got != kw+"_" {
			t.Errorf("goIdent(%q) = %q", kw, got)
		}
	}
}

func TestGoIdent(t *testing.T) {
	for in, want := range map[string]string{
		"x": "x", "type": "type_", "func": "func_", "map": "map_", "range": "range_", "go": "go_",
		"int32": "int32_", "init": "init_", "any": "any_", "C": "C_", "unsafe": "unsafe_", "true": "true_",
		"a-b": "a_b", "3d": "_3d", "": "_", "string": "string", "len": "len", "int": "int", "uint": "uint",
		"ünï": "ünï",
	} {
		if got := goIdent(in); got != want {
			t.Errorf("goIdent(%q) = %q, want %q", in, got, want)
		}
	}
	if paramName("__stream") != "stream" || paramName("_") != "_" || paramName("__") != "__" || paramName("x_") != "x_" {
		t.Error("paramName")
	}
}

func TestPackageNames(t *testing.T) {
	for _, c := range []struct {
		header, want string
		err          string
	}{
		{"<raylib.h>", "raylib", ""},
		{"<SDL2/SDL.h>", "SDL", ""},
		{"./physics.h", "physics", ""},
		{"my-lib.h", "my_lib", ""},
		{"<foo.bar.h>", "foo_bar", ""},
		{"<vector>", "vector", ""},
		{"<string.h>", "", "predeclared"},
		{"<map>", "", "keyword"},
		{"<3dmath.h>", "", "not a package name"},
		{"<init.h>", "", "cannot be a package name"},
		{"<C.h>", "", "cannot be a package name"},
		{"<_.h>", "", "cannot be a package name"},
	} {
		cfg := Config{Header: c.header}
		got, err := cfg.defaultPackage()
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) || !strings.Contains(err.Error(), "-pkg") {
				t.Errorf("%s: err = %v, want %q and a hint for -pkg", c.header, err, c.err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: %q, %v; want %q", c.header, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "a b", "1x", "type", "string", "C", "init", "_", "a-b"} {
		if validPackage(bad) == nil {
			t.Errorf("validPackage(%q) is nil", bad)
		}
	}
	if validPackage("raylib") != nil || validPackage("SDL") != nil || validPackage("p_1") != nil {
		t.Error("validPackage rejects a good name")
	}
}

func TestAllows(t *testing.T) {
	mk := func(include, exclude []string) *Config {
		c := &Config{}
		var err error
		if c.include, err = compilePatterns(include, "include"); err != nil {
			t.Fatal(err)
		}
		if c.exclude, err = compilePatterns(exclude, "exclude"); err != nil {
			t.Fatal(err)
		}
		return c
	}
	if !mk(nil, nil).allows("anything") || !mk(nil, nil).allows("") {
		t.Error("no patterns allow everything")
	}
	c := mk([]string{"^Vec", `^geo::`}, []string{"Internal$"})
	for name, want := range map[string]bool{
		"Vec2": true, "geo::x": true, "other": false, "VecInternal": false, "geo::Internal": false, "": true,
	} {
		if c.allows(name) != want {
			t.Errorf("allows(%q) = %v", name, !want)
		}
	}
	if _, err := compilePatterns([]string{"("}, "include"); err == nil || !strings.Contains(err.Error(), "include") {
		t.Errorf("bad pattern: %v", err)
	}
}

func TestEnumValues(t *testing.T) {
	// A = 5, B (6), C = -3, D (-2), E = 18446744073709551615
	mk := func(name, val string) jnode {
		n := jnode{Kind: "EnumConstantDecl", Name: name}
		if val != "" {
			n.Inner = []jnode{{Kind: "ImplicitCastExpr", Inner: []jnode{{Kind: "ConstantExpr", Value: []byte(`"` + val + `"`)}}}}
		}
		return n
	}
	e := &jnode{Kind: "EnumDecl", Inner: []jnode{mk("A", "5"), mk("B", ""), mk("C", "-3"), mk("D", ""), mk("E", "18446744073709551615"), mk("F", "")}}
	var got []string
	for _, v := range enumValues(e) {
		got = append(got, v.Name+"="+v.Value.String())
	}
	if want := "A=5 B=6 C=-3 D=-2 E=18446744073709551615 F=18446744073709551616"; strings.Join(got, " ") != want {
		t.Errorf("enumValues = %v, want %s", got, want)
	}
}

func TestUnknownPatternsAndOptions(t *testing.T) {
	cfg := Config{Header: "<x.h>", Std: "c++17"}
	if err := cfg.prepare(t.Context()); err == nil || !strings.Contains(err.Error(), "C++20") {
		t.Errorf("std c++17: %v", err)
	}
	cfg = Config{Header: "<x.h>", Scope: "wide"}
	if err := cfg.prepare(t.Context()); err == nil || !strings.Contains(err.Error(), "unknown scope") {
		t.Errorf("scope: %v", err)
	}
	cfg = Config{Header: "<x.h>", Package: "type"}
	if err := cfg.prepare(t.Context()); err == nil || !strings.Contains(err.Error(), "keyword") {
		t.Errorf("package: %v", err)
	}
	cfg = Config{Header: "<x.h", Package: "x"}
	if err := cfg.prepare(t.Context()); err == nil || !strings.Contains(err.Error(), "not a header") {
		t.Errorf("header: %v", err)
	}
	cfg = Config{}
	if err := cfg.prepare(t.Context()); err == nil {
		t.Error("no header")
	}
	cfg = Config{Header: "<x.h>", Include: []string{"["}, Package: "x"}
	if err := cfg.prepare(t.Context()); err == nil || !strings.Contains(err.Error(), "include pattern") {
		t.Errorf("pattern: %v", err)
	}
	for _, std := range []string{"c++20", "gnu++20", "c++2a", "c++23", "c++2b", "gnu++26", "c++2c"} {
		if !stdRE.MatchString(std) {
			t.Errorf("%s rejected", std)
		}
	}
	for _, std := range []string{"c++17", "c++14", "gnu++17", "c++11", "c17", "c++", "c++200"} {
		if stdRE.MatchString(std) {
			t.Errorf("%s accepted", std)
		}
	}
}

func mustParse(t *testing.T, spelling string) *ctype.Type {
	t.Helper()
	ty, err := ctype.Parse(spelling)
	if err != nil {
		t.Fatal(err)
	}
	return ty
}

func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

func TestScrub(t *testing.T) {
	for in, want := range map[string]string{
		"braces for WithAnon::(anonymous union at /home/me/x.h:13:18)": "braces for WithAnon::(anonymous union)",
		"struct (unnamed struct at ./a/b.hpp:3:4) *":                   "struct (unnamed struct) *",
		"see /usr/include/x.h:12:3 and y.cpp:7":                        "see <location> and <location>",
		"plain text":                                                   "plain text",
	} {
		if got := scrub(in); got != want {
			t.Errorf("scrub(%q) = %q, want %q", in, got, want)
		}
	}
}
