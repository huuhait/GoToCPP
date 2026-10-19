package bindgen

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func headersDir(t testing.TB) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("testdata", "headers"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// generate runs bindgen on a header of testdata/headers (or a system header).
func generate(t testing.TB, header string, set func(*Config)) *Result {
	t.Helper()
	res, err := tryGenerate(t, header, set)
	if err != nil {
		t.Fatalf("bindgen %s: %v", header, err)
	}
	return res
}

func tryGenerate(t testing.TB, header string, set func(*Config)) (*Result, error) {
	t.Helper()
	cfg := Config{Header: header, Dir: headersDir(t), Clang: tsupport.Clang(t), Timeout: 5 * time.Minute}
	if set != nil {
		set(&cfg)
	}
	return Generate(context.Background(), cfg)
}

// checkGP says that a view is formatted the way gocpp fmt formats it and that
// gocpp check accepts it.
func checkGP(t testing.TB, name string, res *Result) {
	t.Helper()
	again, err := format.Source(name+".gp", res.Source)
	if err != nil {
		t.Fatalf("%s: the output does not parse: %v", name, err)
	}
	if string(again) != string(res.Source) {
		t.Errorf("%s: the output is not stable under gocpp fmt", name)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "view.gp"), res.Source, 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := tsupport.CheckDir(t, dir, "view.gp")
	for _, d := range p.Errors.Diagnostics() {
		t.Errorf("%s: gocpp check: %v", name, d)
	}
	for _, bad := range []string{os.TempDir() + "/gocpp-bindgen", "/nix/store/", "/usr/include"} {
		if strings.Contains(string(res.Source), bad) {
			t.Errorf("%s: the output has the path %s", name, bad)
		}
	}
}

// golden compares a view with testdata/golden/<name>.gp.
func golden(t testing.TB, name string, res *Result) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".gp")
	if *update {
		if err := os.WriteFile(path, res.Source, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no golden file (run with -update): %v", err)
	}
	if string(want) != string(res.Source) {
		t.Errorf("%s differs from %s (run with -update and look at the diff)\n--- got\n%s", name, path, firstDiff(string(want), string(res.Source)))
	}
}

func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if a != b {
			return fmt.Sprintf("line %d:\n  want %q\n  got  %q", i+1, a, b)
		}
	}
	return ""
}

var goldenCases = []struct {
	name   string
	header string
	set    func(*Config)
}{
	{"c_basic", "c_basic.h", nil},
	{"c_macros", "c_macros.h", nil},
	{"std_names", "std_names.h", nil},
	{"anon_decl", "anon_decl.h", nil},
	{"anon_misc", "anon_misc.h", nil},
	{"struct_macros", "struct_macros.h", nil},
	{"struct_ptrs", "struct_ptrs.h", nil},
	{"layout_attrs", "layout_attrs.h", nil},
	{"unusual_types", "unusual_types.h", nil},
	{"vector_types", "vector_types.h", nil},
	{"lows", "lows.h", nil},
	{"fold_cases", "fold_cases.h", nil},
	{"enum_macro_names", "enum_macro_names.h", nil},
	{"cxx_features", "cxx_features.hpp", nil},
	{"umbrella_auto", "umbrella/umbrella.h", nil},
	{"umbrella_header", "umbrella/umbrella.h", func(c *Config) { c.Scope = ScopeHeader }},
	{"umbrella_dir", "umbrella/umbrella.h", func(c *Config) { c.Scope = ScopeDir }},
	{"umbrella_tree", "umbrella/umbrella.h", func(c *Config) { c.Scope = ScopeTree }},
	{"flags_default", "flags.h", func(c *Config) { c.Includes = []string{"inc"} }},
	{"flags_defined", "flags.h", func(c *Config) {
		c.Includes = []string{"inc"}
		c.Defines = []string{"FEATURE_X", "LEVEL=7"}
		c.Std = "c++23"
	}},
	{"c_basic_include", "c_basic.h", func(c *Config) { c.Include = []string{`^(Vec2|Node|add|range)$`, `^ANON_ONE$`} }},
	{"c_basic_exclude", "c_basic.h", func(c *Config) {
		c.Exclude = []string{`^_`, `Callback|Handler`, `^(map|stat|struct_stat)$`, `^(table|names)$`}
	}},
	{"cxx_exclude_geo", "cxx_features.hpp", func(c *Config) { c.Exclude = []string{`^geo::(Shape|Circle|detail)`}; c.Package = "geoview" }},
	{"c_macros_pkg", "c_macros.h", func(c *Config) {
		c.Package = "macros"
		c.Include = []string{`^(RED|SEGMENT|VERSION.*|Color|Pair|Seg)$`}
	}},
}

func TestGolden(t *testing.T) {
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			res := generate(t, c.header, c.set)
			golden(t, c.name, res)
			checkGP(t, c.name, res)
		})
	}
}

// TestDeterministic generates every golden case twice, from two folders with
// copies of the headers: the views are the same.
func TestDeterministic(t *testing.T) {
	src := headersDir(t)
	copyTree := func() string {
		dst := t.TempDir()
		if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
			t.Fatal(err)
		}
		return dst
	}
	a, b := copyTree(), copyTree()
	for _, c := range goldenCases[:6] {
		t.Run(c.name, func(t *testing.T) {
			r1, err := tryGenerate(t, c.header, func(cfg *Config) {
				if c.set != nil {
					c.set(cfg)
				}
				cfg.Dir = a
			})
			if err != nil {
				t.Fatal(err)
			}
			r2, err := tryGenerate(t, c.header, func(cfg *Config) {
				if c.set != nil {
					c.set(cfg)
				}
				cfg.Dir = b
			})
			if err != nil {
				t.Fatal(err)
			}
			if string(r1.Source) != string(r2.Source) {
				t.Errorf("two runs differ: %s", firstDiff(string(r1.Source), string(r2.Source)))
			}
		})
	}
}

// TestContent checks facts about the views that the golden files show, one
// by one, so that a change of the golden file cannot hide a lost fact.
func TestContent(t *testing.T) {
	squeeze := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	has := func(t *testing.T, res *Result, lines ...string) {
		t.Helper()
		for _, l := range lines {
			if !strings.Contains(squeeze(string(res.Source)), squeeze(l)) {
				t.Errorf("the view lacks %q", l)
			}
		}
	}
	lacks := func(t *testing.T, res *Result, lines ...string) {
		t.Helper()
		for _, l := range lines {
			if strings.Contains(squeeze(string(res.Source)), squeeze(l)) {
				t.Errorf("the view has %q", l)
			}
		}
	}
	t.Run("c", func(t *testing.T) {
		res := generate(t, "c_basic.h", nil)
		has(t, res,
			"// Code generated by gocpp bindgen from c_basic.h. DO NOT EDIT.\n\npackage c_basic\n",
			"type Vec2 struct {\n\tx, y C.float\n}",
			"type Rgba struct {\n\tr, g, b, a C.uchar\n}", // an unnamed struct takes the name of its typedef
			"type Opaque struct{}",                        // a forward declaration
			"type tagged_t = tagged",
			"type struct_stat struct {", // a tag named like a function
			"func stat(path *C.char, buf *struct_stat) C.int",
			"type Color C.uint", // enums over unsigned int ...
			"type Mode C.int",   // ... unless a value is negative
			"MODE_NEG Mode = -1",
			"type Big C.ulong", // ... or does not fit
			"BIG_HIGH Big = 8589934591",
			"\tANON_ONE = 1\n", // an enum with no name
			"// union: the fields share their storage\ntype Num struct {",
			"a C.uint // bit field of 3 bits",
			"type Callback = func(C.int, *C.char, unsafe.Pointer) C.int",
			"func log_message(fmt *C.char, args ...any)",
			"func vlog(fmt *C.char, ap C.va_list) C.int",
			"func count_bytes(data *uint8, n C.size_t, out *uint32) C.size_t",
			"func take_matrix(m *[3]C.int)",
			"func unnamed_params(C.int, *C.char, Vec2) C.int",
			"func range_(type_ C.int, func_ C.int) C.int", // Go keywords
			"type map_ struct {\n\tgo_, select_ C.int\n}",
			"var unknown_size [0]C.int",
			"func twice(x C.int) C.int",
			"type FILE = _IO_FILE",       // declared in <stdio.h>, used here
			"type Public = _private_rec", // a private name is written when another declaration uses it
			"type _private_rec struct {\n\tv C.int\n}",
			"func init_(mode C.int) C.int",
			"import \"unsafe\"",
		)
		lacks(t, res, "twice(x C.int) C.int {", "uint8_t", "_unused_rec", "_hidden_fn")
		if regexp.MustCompile(`(^|[^.\w])size_t`).Match(res.Source) {
			t.Error("a size_t that is not C.size_t")
		}
	})
	t.Run("macros", func(t *testing.T) {
		res := generate(t, "c_macros.h", nil)
		has(t, res,
			`VERSION       = "1.2.3"`, "MASK", "= 19", "NEG", "= -5",
			"= 18446744073709551615", "PI_F", "= 3.14159\n", "ONE_F", "= 1.0\n", "HUGE_D", "= 1e+300",
			"= 65\n", "YES", "= true", "NO", "= false", `"line\n\ttab \"quoted\" \\ AA"`, "= 12\n",
			"DEFAULT_MODE", "Mode = 2", "_KEPT_UNDERSCORE", "REDEF", "= 2\n",
			"RED     = Color{255, 0, 0, 255}", "SEGMENT = Seg{Pair{1, 2}, Pair{3, 4}}", "PARTIAL = Color{255, 0, 0, 0}",
			"function-like: // MAX(a,b)", "a pointer, not a Go constant: // NULL_PTR",
			"BRACE", "SEMI", "TYPE_ALIAS", "EXTERN_WORD", "not a constant: // PAIRS",
			"its value depends on where it is used: // FILE_ALIAS, FILE_NAME, LINE_NO",
			"NEGPAIR = Pair{-1, -2}", "HOLDER", "= Holder{1, nil, Pair{0, 0}}", "UNION_VAL", "= U{f: 1.5}", "ROW", "= Rowv{[4]C.int{1, 2}}",
		)
		lacks(t, res, "UNDEF_ME", "__RESERVED", "EMPTY", "C_MACROS_H", "REDEF = 1", "import \"unsafe\"")
	})
	t.Run("cxx", func(t *testing.T) {
		res := generate(t, "cxx_features.hpp", nil)
		has(t, res,
			"type geo_Vec struct {\n\tx, y C.double\n}",
			"func (this geo_Vec) length() C.double",  // a const method has a value receiver
			"func (this *geo_Vec) scale(k C.double)", // the others a pointer
			"//\tconstructor geo_Vec(x C.double, y C.double)",
			"//\toperator+(o *geo_Vec) geo_Vec",
			"//\tstatic method zero() geo_Vec",
			"//\tstatic member count C.int",
			"//\toverload: (this *geo_Shape) move(dx C.double, dy C.double)",
			"func (this geo_Shape) area() C.double // virtual, pure",
			"geo_Shape // base class",
			"type geo_Level C.uchar", "geo_Level_High geo_Level = 10", // enum class: the name is part of the constant's
			"geo_P_ONE geo_Plain = 1", // a plain enum: it is not
			"type geo_Shape_Style struct {", "type geo_Shape_Kind C.uint", "geo_Shape_Circle geo_Shape_Kind = 0",
			"geo_detail_kLimit C.int = 64", "geo_kMax          C.int  = 12", "geo_kFlag         C.bool = true",
			"var geo_kPi C.double", // const, but not a constant expression
			"func geo_version() C.int",
			"// overload: geo::dot(a *geo_Vec, b *geo_Vec, w C.double) C.double",
			"func outer_inner_deep_fn(outer_inner_Deep) C.int",
			"type std_string struct{}",
			"// struct template geo::Box<typename T>",
			"// function template geo::pick<typename T, int N>",
		)
		lacks(t, res, "hidden_in_anonymous_namespace", "in_private_ns", "hidden_", "guarded_", "secret", "geo_v2_", "deleted_fn")
		has(t, res, "type geo_Derived struct {\n\td C.int\n}", // a private base is not a field
			"type geo_Open struct {\n\tgeo_Vec // base class\n\to C.int\n}") // a protected one neither
		lacks(t, res, "geo_Circle // base class\n\to")
	})
	t.Run("unnamed types keep their declarators", func(t *testing.T) {
		res := generate(t, "anon_decl.h", nil)
		has(t, res,
			"entries [16]struct {\n key, val C.int\n } // unnamed struct type",
			"cursor *struct {\n c C.char\n } // unnamed struct type",
			"vals [2][3]struct {\n i C.int\n f C.float\n } // unnamed union type",
			"modes [4]C.uint // unnamed enum",
			"nest [2]struct {\n inner [3]struct {\n q C.char\n }",
			"x struct {\n a C.short\n } // unnamed struct type",
			"after C.int")
		res = generate(t, "anon_misc.h", nil)
		has(t, res,
			"var glob_anon struct {\n x, y C.int\n }",
			"var glob_union [2]struct {\n i C.int\n f C.float\n }",
			"type AnonPtr = *struct {\n a C.int\n }",
			"type AnonArr = [3]struct {\n a C.int\n }",
			"mp *struct {\n deep C.int\n }",
			"decltype(glob_anon) is written as unsafe.Pointer", // listed, not silent
			"var ns_nv struct {\n u C.int\n }",
			"var genum [2]C.uint", "type TArr = [3]C.uint") // an unnamed enum under declarators is its integer type
		lacks(t, res, "struct{}\n\nvar", "= *struct{}", "= [3]struct{}")
	})
	t.Run("struct macros", func(t *testing.T) {
		res := generate(t, "struct_macros.h", nil)
		has(t, res,
			"SM_PTR = WithPtr{\"s\", nil, nil}",   // 0 for a pointer is nil
			"SM_BITS = WithBits{K_B, 3, 4, 9}",    // an enumerator is named (the literals are written after the names)
			"SM_CONSTEXPR_REF = Four{7, 2, 3, 4}", // an enumerator into an int field is its value
			"SM_ANON = WithAnon{struct {\n i C.int\n f C.float\n }{i: 3}, 4}",
			"SM_NEST_CALL = Four{3, 12, -5, -1}", // the operators are folded
			"SM_NUMS = Nums{-9223372036854775808, 18446744073709551615, -3, -4}",
			"SM_PAIR_EXPR = Pair{7, 1e+300}",
			"TILDE_S = TS{-1, -1}",                 // ~0 is -1, not ^0
			"TILDE_U = TU{4294967295, 4294967295}", // ~0u and ~0 into unsigned fields wrap to the type
			"SHIFT_S = TS{16, 16}",
			"SM_RECT2 = Rect{V2{0, 0}, V2{3, 4}}",
			"SM_DEEP = Deep{Rect{V2{1, 2}, V2{3, 4}}, Color{9, 8, 7, 6}, 42}")
		lacks(t, res, "(anonymous", "(unnamed", ".h:")
		res = generate(t, "struct_ptrs.h", nil)
		has(t, res,
			"P_ZERO = P{nil, nil, nil}",
			"P_NULLPTR = P{nil, nil, nil}",
			"S8_M = S8{[8]C.char{97, 98, 99}, 1}", // a string for a char array is its bytes
			"S8_M2 = S8{[8]C.char{97, 98}, 1}",
			"SA_M = SA{[2]*C.char{\"x\", \"y\"}}",
			"SA2_ZERO = SA2{[2]*C.char{nil, nil}, [2]func(C.int){nil, nil}}") // 0 in an array of pointers
		res = generate(t, "struct_macros.h", nil)
		has(t, res, "NaN): // NEG_ZERO_PAIR, SM_PAIR")
	})
	t.Run("scopes of unnamed types", func(t *testing.T) {
		res := generate(t, "anon_decl.h", nil)
		has(t, res,
			"var m1_mv struct {\n mm C.int\n }",
			"var m2_cv struct {\n cc C.char\n }",
			"var m2_cp *struct {\n cc C.char\n }",      // the declarator of the second name
			"var after_named struct {\n b C.short\n }", // a typedef-named record before it is not its type
			"var plain_named NamedAnon", "type NamedAnon struct {\n a C.int\n }")
	})
	t.Run("folded values", func(t *testing.T) {
		res := generate(t, "fold_cases.h", nil)
		has(t, res,
			"LIM_ALL = Lim{18446744073709551615, 4294967295, 18446744073709551615, 0.10000000149011612, 2, -1}", // typedef-spelled types wrap, a float widens as a float, a long cast truncates, '\xff' is -1
			"U_NEG = U{18446744073709551615, 4294967294}",                                                       // the field's type wraps the constant
			"SMALL_CAST = Small{-56, 52, -4464, 1}",
			"SMALL_NOT = Small{55, 0, 0, 0}",
			"DIV_OK = Div{-3, -1, 3, 0, -2}", // -7/2 and -7%3 truncate toward zero, (short)(-2.5) is -2
			"BANG = Two{0, 1}",
			"F32 = FD{0.33333334, 0.10000000149011612}",
			"F32_2 = FD{0.1, 0.3333333333333333}",
			"F_NEG = FD{-0.5, -2}",
			"INT2FLOAT = FD{16777216, 9.007199254740992e+15}",
			"NEG_SINGLE = FD{-0.3, -0.3}", // a negative float32 that is not whole: its shortest text
			"NOOP = UT{7}",                // a cast to a typedef of the field's own type
			"SZ1 = SZ{40, 8, 8}",          // sizeof and alignof of a type: asked of the compiler
			"EC_WRAP = HasEC{0}",          // a number converted to an enum with a fixed underlying type keeps its bits
			"EC_OK = HasEC{200}",
			"BF1 = BF{1099511627775, 3}", // a bit field keeps the low bits (40)
			"SB1 = SB{-7}",               // ... and its sign (4 bits)
			"a 128-bit or long double field: // I1, I2, LD1, LDA1, LDE1",
			"a conversion that the view cannot fold: // BIG_LIT, FLOAT_BIG, INF_INT, NAN_INT", // no panic for NaN and infinities // an integer converted to float is rounded to the float's precision
			"FLOAT_BIG", "INT_OVF", "MIN_DIV", "SHIFT_LIMIT", "SHIFT_NEG", "DIV_ZERO", "REM_ZERO")
		lacks(t, res, "FLOAT_BIG =", "INT_OVF =", "MIN_DIV =", "SHIFT_LIMIT =", "SHIFT_NEG =", "DIV_ZERO =", "REM_ZERO =")
	})
	t.Run("attributes", func(t *testing.T) {
		res := generate(t, "layout_attrs.h", nil)
		has(t, res,
			"// layout: packed\ntype Packed struct {",
			"// layout: aligned(16)\ntype Aligned struct {",
			"// layout: aligned(32)\ntype Aligned32 struct {",
			"d C.char // aligned(8)",
			"// layout differs (these records are packed or over-aligned; the view does not say how):\n//\tAlign2, Aligned, Aligned32, Packed")
		res = generate(t, "unusual_types.h", nil)
		has(t, res,
			"at C.int // _Atomic",
			"al C.char // aligned(16)",
			"pm unsafe.Pointer // a pointer to member is written as unsafe.Pointer",
			"type bi24 [4]byte",
			"type v4f [16]byte",
			"//\tW")
		lacks(t, res, "undefined")
		res = generate(t, "vector_types.h", nil)
		has(t, res, "// aligned(16)\ntype v4 [16]byte", "func addv(a v4, b v4) v4",
			"// aligned(32)\ntype v3d [32]byte", // ext_vector_type(3) of double: the real sizeof, not 3*8
			"// aligned(16)\ntype v3f [16]byte",
			"// aligned(8)\ntype b65 [16]byte", // _BitInt(65)
			"type b8 [1]byte")
	})
	t.Run("small cases", func(t *testing.T) {
		res := generate(t, "lows.h", nil)
		has(t, res,
			"type FnPtr = func(C.int)", // a pointer to a function typedef is a function pointer
			"type FnPtrPtr = *FnPtr",
			"func install(cb FnPtr, other func(C.int))",
			"type Flagbit C.uint", "FLAG_TOP Flagbit = 2147483648", // 0x80000000u does not fit an int
			"UNI_ÄÖ = 3",
			"POS_ZERO = 0.0",
			"value has no Go constant: // NEG_ZERO",
			"its name is a C++ keyword or a name the probe uses: // auto, bool, map, size, val")
		lacks(t, res, "SOCK_STREAM_2", "SOCK_DGRAM_2") // a macro named like its own enumerator
		res2 := generate(t, "enum_macro_names.h", nil)
		has(t, res2, "FP_NAN Fp = 0", "has the name of an enumerator: // FP_NAN")
		lacks(t, res2, "FP_NAN_2", "FP_INFINITE_2", "FP_INFINITE =")
		has(t, res, "SOCK_STREAM Sock = 1")
	})
	t.Run("standard names", func(t *testing.T) {
		res := generate(t, "std_names.h", nil)
		has(t, res, "func low_byte(value int32) uint8", "func measure(data *uint8, n C.size_t) C.size_t", "func vmeasure(fmt *C.char, ap C.va_list) C.int")
		lacks(t, res, "type uint8_t", "type size_t", "type va_list", "type int32_t")
	})
	t.Run("umbrella", func(t *testing.T) {
		for scope, wantNames := range map[Scope][]string{
			ScopeHeader: {"UMBRELLA_VERSION"},
			ScopeDir:    {"UMBRELLA_VERSION", "PartA", "PartB", "part_a", "PART_B_MAGIC"},
			ScopeTree:   {"UMBRELLA_VERSION", "PartA", "PartB", "part_a", "PART_B_MAGIC", "func deep(", "DEEP_MAGIC"},
			ScopeAuto:   {"UMBRELLA_VERSION", "PartA", "PartB", "part_a", "PART_B_MAGIC"},
		} {
			res := generate(t, "umbrella/umbrella.h", func(c *Config) { c.Scope = scope })
			has(t, res, wantNames...)
			if scope != ScopeTree {
				lacks(t, res, "deep", "DEEP_MAGIC")
			}
			if scope == ScopeHeader {
				lacks(t, res, "PartA", "part_a", "MAGIC")
			}
			lacks(t, res, "fopen", "EOF") // <stdio.h> is not in the view
		}
	})
}

func TestStats(t *testing.T) {
	res := generate(t, "c_macros.h", nil)
	if res.Package != "c_macros" || res.Stats.Consts != 18 || res.Stats.Macros != 27 || res.Stats.Skipped != 10 || res.Stats.Types != 7 || res.Stats.Files != 1 || len(res.Files) != 1 || res.Stats.Total <= 0 {
		t.Errorf("stats %+v, package %q, files %v", res.Stats, res.Package, res.Files)
	}
	if !strings.HasSuffix(res.Files[0], "c_macros.h") || !filepath.IsAbs(res.Files[0]) {
		t.Errorf("files %v", res.Files)
	}
}

func TestErrors(t *testing.T) {
	clang := tsupport.Clang(t)
	_ = clang
	cases := []struct {
		name   string
		header string
		set    func(*Config)
		want   string
	}{
		{"missing file", "nothere.h", nil, "cannot find header nothere.h"},
		{"missing system header", "<nothere.h>", nil, "cannot find header <nothere.h>"},
		{"a folder", "umbrella", nil, "cannot find header umbrella"},
		{"missing include", "flags.h", nil, "cannot find header local_dep.h (needed by flags.h)"},
		{"broken header", "broken.h", nil, "header broken.h doesn't compile: broken.h:2:1: error: expected unqualified-id"},
		{"bad package", "c_basic.h", func(c *Config) { c.Package = "func" }, `"func" is a Go keyword`},
		{"derived package", "<string.h>", nil, "choose a package name with -pkg"},
		{"bad scope", "c_basic.h", func(c *Config) { c.Scope = "everything" }, "unknown scope"},
		{"old standard", "c_basic.h", func(c *Config) { c.Std = "c++14" }, "C++20 or newer"},
		{"bad pattern", "c_basic.h", func(c *Config) { c.Exclude = []string{"("} }, "bad exclude pattern"},
		{"no header", "", nil, "no header"},
		{"half bracket", "<c_basic.h", nil, "is not a header"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := tryGenerate(t, c.header, c.set)
			if err == nil {
				t.Fatalf("no error; got\n%s", res.Source)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q, want %q", err, c.want)
			}
			if strings.Contains(err.Error(), headersDir(t)) {
				t.Errorf("the error has the absolute path of the test folder: %v", err)
			}
		})
	}
}

func TestNoClang(t *testing.T) {
	t.Setenv("GOCPP_CLANG", filepath.Join(t.TempDir(), "no-such-clang"))
	_, err := Generate(context.Background(), Config{Header: "c_basic.h", Dir: headersDir(t)})
	if !errors.Is(err, ErrNoClang) {
		t.Fatalf("err = %v, want ErrNoClang", err)
	}
	// g++ is not clang
	if gpp, e := exec.LookPath("g++"); e == nil {
		t.Setenv("GOCPP_CLANG", gpp)
		_, err = Generate(context.Background(), Config{Header: "c_basic.h", Dir: headersDir(t)})
		if !errors.Is(err, ErrNoClang) {
			t.Fatalf("g++: err = %v, want ErrNoClang", err)
		}
	}
}

func TestCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Generate(ctx, Config{Header: "c_basic.h", Dir: headersDir(t), Clang: tsupport.Clang(t)})
	if err == nil {
		t.Fatal("a canceled context gives a view")
	}
}

// TestRealMachineryOnly keeps the compiler's output honest: the compiler
// that the tests use is the one that built these views.
func TestCompilerIsClang(t *testing.T) {
	cxx := tsupport.Clang(t)
	if cxx.Family != build.Clang {
		t.Fatal(cxx)
	}
}

// hashFiles is the key of a golden file of a header that is not ours: the
// compiler and the contents of the files of the view.
func hashFiles(t testing.TB, cxx build.CXX, res *Result) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", cxx.Version)
	for _, f := range res.Files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%d\n", len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// The compiler's own include path is what makes a header "alone" in scope auto: the
// folder of <stdio.h> is one of its folders, and the folder of a header of ours is not.
func TestSearchRoots(t *testing.T) {
	cxx := tsupport.Clang(t)
	tl := &tool{cxx: cxx, dir: headersDir(t), tmp: t.TempDir()}
	roots := tl.searchRoots(context.Background())
	if len(roots) == 0 {
		t.Fatal("no search roots")
	}
	res := generate(t, "<stdio.h>", func(c *Config) { c.Scope = ScopeHeader })
	if dir := filepath.Dir(res.Files[0]); !slices.Contains(roots, dir) {
		t.Errorf("the folder of <stdio.h>, %s, is not one of %v", dir, roots)
	}
	if slices.Contains(roots, headersDir(t)) {
		t.Error("the test folder is a search root")
	}
	// so the header of the include path is taken alone, and the one in a folder of its own is not
	auto := generate(t, "<stdio.h>", nil)
	dir := generate(t, "<stdio.h>", func(c *Config) { c.Scope = ScopeDir })
	if auto.Stats.Files >= dir.Stats.Files {
		t.Errorf("auto takes %d files, dir %d: auto should take fewer for a header of the include path", auto.Stats.Files, dir.Stats.Files)
	}
}

// No view mentions where the headers were: not a path of this machine, not the place of an
// unnamed type that clang prints.
var machineText = regexp.MustCompile(`/tmp|/nix|/home|/usr/|/var/|\.(h|hpp|hh|hxx|inl|c|cc|cpp):\d+`)

func TestNoMachinePaths(t *testing.T) {
	var files []string
	for _, dir := range []string{"golden", filepath.Join("golden", "zoo")} {
		m, _ := filepath.Glob(filepath.Join("testdata", dir, "*.gp"))
		files = append(files, m...)
	}
	if len(files) < 20 {
		t.Fatalf("only %d golden files", len(files))
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "const") || strings.Contains(line, `"/`) || strings.Contains(line, `= "`) {
				continue // a string constant of the header is the header's (P_tmpdir = "/tmp")
			}
			if machineText.MatchString(line) {
				t.Errorf("%s:%d has text of this machine: %s", f, i+1, line)
			}
		}
	}
}

// Generate leaves nothing behind in the temporary folder: after a view, after an error and
// after a canceled run.
func TestTempFolderIsClean(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	left := func(what string) {
		t.Helper()
		if es, _ := os.ReadDir(tmp); len(es) != 0 {
			t.Errorf("after %s the temporary folder has %d entries, the first is %s", what, len(es), es[0].Name())
		}
	}
	generate(t, "c_macros.h", func(c *Config) { c.CacheDir = "off" })
	left("a view")
	if _, err := tryGenerate(t, "broken.h", nil); err == nil {
		t.Fatal("no error")
	}
	left("an error")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	Generate(ctx, Config{Header: "c_macros.h", Dir: headersDir(t), Clang: tsupport.Clang(t)})
	left("a canceled run")
	if _, err := tryGenerate(t, "nothere.h", nil); err == nil {
		t.Fatal("no error")
	}
	left("a missing header")
}

// The probe's cache on disk (step 12.7) answers the macros of a header that has not changed:
// the view is the same byte for byte, the compiler is not asked, and a -D or an edit of the
// header (its contents are in the key) asks again.
func TestProbeCache(t *testing.T) {
	cache := t.TempDir()
	dir := t.TempDir()
	hdr := filepath.Join(dir, "cached.h")
	write := func(v string) {
		t.Helper()
		src := "#pragma once\n#define VALUE " + v + "\n#ifdef EXTRA\n#define EXTRA_VALUE 7\n#endif\n#define NAME \"cached\"\n"
		if err := os.WriteFile(hdr, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		// the cache does not trust a header that changed in the last two seconds (the racy-file
		// rule, by the time of the status change too): wait them out
		time.Sleep(2200 * time.Millisecond)
	}
	run := func(defs ...string) *Result {
		t.Helper()
		return generate(t, "cached.h", func(c *Config) { c.Dir = dir; c.CacheDir = cache; c.Defines = defs })
	}
	write("1")
	cold := run()
	if cold.Stats.ProbeCached {
		t.Error("the first run was answered by an empty cache")
	}
	if es, _ := os.ReadDir(cache); len(es) == 0 {
		t.Error("the cache folder is empty after a run")
	}
	warm := run()
	if !warm.Stats.ProbeCached {
		t.Error("the second run was not answered by the cache")
	}
	if string(cold.Source) != string(warm.Source) {
		t.Errorf("cold and warm differ: %s", firstDiff(string(cold.Source), string(warm.Source)))
	}
	if !strings.Contains(string(cold.Source), "VALUE = 1") {
		t.Errorf("the view lacks VALUE = 1:\n%s", cold.Source)
	}
	withD := run("EXTRA")
	if withD.Stats.ProbeCached || !strings.Contains(string(withD.Source), "EXTRA_VALUE = 7") {
		t.Errorf("-DEXTRA: cached %v\n%s", withD.Stats.ProbeCached, withD.Source)
	}
	write("2")
	edited := run()
	if edited.Stats.ProbeCached || !strings.Contains(string(edited.Source), "VALUE = 2") {
		t.Errorf("after an edit: cached %v\n%s", edited.Stats.ProbeCached, edited.Source)
	}
	if again := run(); !again.Stats.ProbeCached || string(again.Source) != string(edited.Source) {
		t.Error("the edited header is not served from the cache the second time")
	}
	// no cache: every run asks
	off := generate(t, "cached.h", func(c *Config) { c.Dir = dir; c.CacheDir = "off" })
	if off.Stats.ProbeCached || string(off.Source) != string(edited.Source) {
		t.Error("off")
	}
}

// A stand-in when the compiler cannot be asked: the size from the tree (a _BitInt rounds up to a power of two
// below 8 bytes and to 8-byte words above), an empty struct when the tree does not say.
func TestStandInFallback(t *testing.T) {
	node := func(kind, typ string, elems ...jnode) *jnode {
		return &jnode{Kind: "TypedefDecl", Inner: []jnode{{Kind: kind, Type: &jtype{QualType: typ}, NumElements: 3, Inner: elems}}}
	}
	for _, c := range []struct {
		name string
		n    *jnode
		want string
	}{
		{"_BitInt(65)", node("BitIntType", "_BitInt(65)"), "type T [16]byte\n\n"},
		{"unsigned _BitInt(24)", node("BitIntType", "unsigned _BitInt(24)"), "type T [4]byte\n\n"},
		{"_BitInt(8)", node("BitIntType", "_BitInt(8)"), "type T [1]byte\n\n"},
		{"vector of 3 floats", node("ExtVectorType", "x", jnode{Type: &jtype{QualType: "float"}}), "type T [12]byte\n\n"},
		{"unknown element", node("VectorType", "x", jnode{Type: &jtype{QualType: "__fp16"}}), "type T struct{}\n\n"},
		{"nothing", &jnode{Kind: "TypedefDecl"}, "type T struct{}\n\n"},
	} {
		g := newTestGen()
		d := &gdecl{node: c.n, goName: "T"}
		g.sizes = map[*gdecl][2]int{d: {}} // asked, and the compiler did not say
		var b strings.Builder
		g.standIn(&b, d)
		if b.String() != c.want {
			t.Errorf("%s: %q, want %q", c.name, b.String(), c.want)
		}
	}
	// the compiler's sizeof wins over the tree's, and an alignment is a comment
	g := newTestGen()
	d := &gdecl{node: node("ExtVectorType", "x", jnode{Type: &jtype{QualType: "double"}}), goName: "V"}
	g.sizes = map[*gdecl][2]int{d: {32, 32}}
	var b strings.Builder
	g.standIn(&b, d)
	if b.String() != "// aligned(32)\ntype V [32]byte\n\n" {
		t.Errorf("%q", b.String())
	}
}
