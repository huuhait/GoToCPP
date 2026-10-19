package bindgen

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The folding of struct macros is checked against the compiler's own evaluation: random
// expressions (casts between all the integer and float types, typedef-spelled types, the
// operators, the literals at the edges of their ranges) are written as single-field struct
// macros; a C++ program initializes a variable of the field's type with each expression and
// prints it; the view's literal must be that value. What C leaves undefined (the compiler
// warns: division by zero, shifts out of range, overflow) is a separate group: there the
// view must make a view, and the macro is translated or listed, never a panic.

var fuzzTypes = []string{"int", "unsigned", "long", "unsigned long", "short", "unsigned short", "signed char", "unsigned char", "char",
	"long long", "double", "float", "uint64_t", "int64_t", "uint32_t", "int8_t", "size_t", "bool", "long double", "__int128"}

var fuzzOps = []string{"0", "1", "-1", "2", "5", "7", "31", "32", "63", "127", "128", "255", "256", "-128", "32767", "65535", "2147483647",
	"2147483648", "4294967295u", "4294967296LL", "9223372036854775807LL", "18446744073709551615ULL", "-9223372036854775807LL-1",
	"'a'", `'\xff'`, `'\n'`, "1.5", "0.1f", "3.14159", "1e10", "0.1", "(size_t)-1", "(uint8_t)200", "(int8_t)200", "(uint32_t)4294967295",
	"(uint16_t)65535", "(short)-1", "(unsigned char)255", "(char)200", "true", "(unsigned)-1", "(long)-1", "(unsigned long)-1", "-5",
	"(float)1/3", "1.0/3", "2147483647+1", "(uint32_t)0xFFFFFFFF", "(uint64_t)1<<63", "1UL<<31", "~0", "~0u", "~0ul", "-0.5f", "100000",
	"3000000000u", "16777217", "9007199254740993LL", "(float)16777217", "(0.0/0.0)", "(1.0/0.0)", "1e999", "-1e999", "(long double)0.1", "(__int128)5", "(long)(3.14159)", "(short)(-2.5)", "-7/2", "-7%3", "!5", "!0.0"}

var fuzzBin = []string{"+", "-", "*", "/", "%", "<<", ">>", "&", "|", "^"}
var fuzzUn = []string{"-", "~", "!", "+"}
var fuzzCast = []string{"int", "unsigned", "long", "unsigned long", "short", "unsigned char", "signed char", "char", "float", "double",
	"uint8_t", "int16_t", "uint64_t", "long long", "size_t", "uint32_t", "int8_t", "bool"}

func fuzzExpr(r *rand.Rand, depth int) string {
	pick := func(l []string) string { return l[r.IntN(len(l))] }
	switch p := r.Float64(); {
	case p < 0.3 || depth > 2:
		return pick(fuzzOps)
	case p < 0.45:
		return fmt.Sprintf("%s(%s)", pick(fuzzUn), pick(fuzzOps))
	case p < 0.6:
		return fmt.Sprintf("(%s)(%s)", pick(fuzzCast), fuzzExpr(r, depth+1))
	}
	a, b := pick(fuzzOps), pick(fuzzOps)
	if r.Float64() < 0.3 {
		a = fuzzExpr(r, depth+1)
	}
	if r.Float64() < 0.3 {
		b = fuzzExpr(r, depth+1)
	}
	return fmt.Sprintf("((%s) %s (%s))", a, pick(fuzzBin), b)
}

func fuzzStruct(t string) string { return "S_" + regexp.MustCompile(`\W`).ReplaceAllString(t, "_") }

type fuzzCase struct{ typ, expr string }

func fuzzPrint(t string, id int) string {
	switch t {
	case "double", "float":
		return fmt.Sprintf(`{ %s v = (E%d); printf("M%d %%.17g\n", (double)v); }`, t, id, id)
	case "char", "signed char", "short", "int", "long", "long long", "int64_t", "int8_t":
		return fmt.Sprintf(`{ %s v = (E%d); printf("M%d %%lld\n", (long long)v); }`, t, id, id)
	}
	return fmt.Sprintf(`{ %s v = (E%d); printf("M%d %%llu\n", (unsigned long long)v); }`, t, id, id)
}

// fuzzFiles writes the header with the macros and the program that prints their values.
func fuzzFiles(cases []fuzzCase) (header, program string) {
	var h, p strings.Builder
	h.WriteString("#include <stdint.h>\n#include <stddef.h>\n")
	for _, t := range fuzzTypes {
		fmt.Fprintf(&h, "typedef struct { %s v; } %s;\n", t, fuzzStruct(t))
	}
	p.WriteString("#include <stdio.h>\n#include \"fuzz.h\"\nint main() {\n")
	for i, c := range cases {
		fmt.Fprintf(&h, "#define E%d %s\n#define M%d (%s){ E%d }\n", i, c.expr, i, fuzzStruct(c.typ), i)
		p.WriteString(fuzzPrint(c.typ, i) + "\n")
	}
	p.WriteString("return 0; }\n")
	return h.String(), p.String()
}

var diagLine = regexp.MustCompile(`(?m)^[^\n]*?(\w+\.(?:cpp|h)):(\d+):\d+: (?:error|warning)`)

// fuzzSet generates the candidate expressions of a seed and sorts them: those the
// compiler rejects are dropped, those it warns about with the flags that turn undefined
// behaviour into errors are the "ub" group, and the rest are the "safe" group.
func fuzzSet(t *testing.T, cxx string, seed uint64, n int) (safe, ub []fuzzCase) {
	r := rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
	seen := map[fuzzCase]bool{}
	var cands []fuzzCase
	for len(cands) < n*3 {
		c := fuzzCase{fuzzTypes[r.IntN(len(fuzzTypes))], fuzzExpr(r, 0)}
		if !seen[c] {
			seen[c] = true
			cands = append(cands, c)
		}
	}
	dir := t.TempDir()
	check := func(cs []fuzzCase, flags ...string) map[int]bool {
		h, p := fuzzFiles(cs)
		os.WriteFile(filepath.Join(dir, "fuzz.h"), []byte(h), 0o644)
		os.WriteFile(filepath.Join(dir, "fuzz.cpp"), []byte(p), 0o644)
		args := append([]string{"-std=c++20", "-fsyntax-only", "-ferror-limit=0", "-fno-caret-diagnostics"}, flags...)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, cxx, append(args, filepath.Join(dir, "fuzz.cpp"))...)
		build.InGroup(cmd)
		out, _ := cmd.CombinedOutput()
		bad := map[int]bool{}
		for _, m := range diagLine.FindAllStringSubmatch(string(out), -1) {
			line, _ := strconv.Atoi(m[2])
			switch m[1] {
			case "fuzz.cpp": // the program: its first case is on line 4
				bad[line-4] = true
			case "fuzz.h": // the header: 2 includes, the structs, then two lines per case
				bad[(line-3-len(fuzzTypes))/2] = true
			}
		}
		return bad
	}
	// 1. what does not compile at all (errors only: -w hides the warnings)
	bad := check(cands, "-w")
	var ok []fuzzCase
	for i, c := range cands {
		if !bad[i] {
			ok = append(ok, c)
		}
	}
	// the errors may hide others: once more
	bad = check(ok, "-w")
	var ok2 []fuzzCase
	for i, c := range ok {
		if !bad[i] {
			ok2 = append(ok2, c)
		}
	}
	// 2. undefined behaviour, as the compiler sees it
	bad = check(ok2, "-Werror=division-by-zero", "-Werror=shift-count-overflow", "-Werror=shift-count-negative",
		"-Werror=shift-negative-value", "-Werror=integer-overflow", "-Werror=float-conversion", "-Werror=literal-conversion",
		"-Werror=constant-conversion", "-Werror=implicit-int-float-conversion", "-Werror=overflow", "-Werror=c++11-narrowing")
	for i, c := range ok2 {
		if bad[i] {
			ub = append(ub, c)
		} else {
			safe = append(safe, c)
		}
	}
	if len(safe) > n {
		safe = safe[:n]
	}
	if len(ub) > 40 {
		ub = ub[:40]
	}
	return
}

var viewVar = regexp.MustCompile(`(?m)^\s+(M\d+)\s+= \w+\{(.*)\}$`)
var viewNot = regexp.MustCompile(`\bM\d+\b`)

func TestFoldFuzz(t *testing.T) {
	clang := tsupport.Clang(t)
	seeds := []uint64{1, 2, 3}
	n := 150
	if v := os.Getenv("GOCPP_FOLDFUZZ_N"); v != "" {
		n, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("GOCPP_FOLDFUZZ_SEEDS"); v != "" {
		k, _ := strconv.Atoi(v)
		seeds = nil
		for i := 1; i <= k; i++ {
			seeds = append(seeds, uint64(i))
		}
	}
	for _, seed := range seeds {
		t.Run(fmt.Sprint("seed", seed), func(t *testing.T) {
			t.Parallel()
			safe, ub := fuzzSet(t, clang.Path, seed, n)
			if len(safe) < n/2 || len(ub) == 0 {
				t.Fatalf("only %d safe and %d undefined candidates", len(safe), len(ub))
			}
			runFuzzGroup(t, clang.Path, safe, true)
			runFuzzGroup(t, clang.Path, ub, false)
		})
	}
}

func runFuzzGroup(t *testing.T, cxx string, cases []fuzzCase, compare bool) {
	dir := t.TempDir()
	h, p := fuzzFiles(cases)
	hdr := filepath.Join(dir, "fuzz.h")
	os.WriteFile(hdr, []byte(h), 0o644)
	os.WriteFile(filepath.Join(dir, "fuzz.cpp"), []byte(p), 0o644)
	res, err := Generate(context.Background(), Config{Header: hdr, Dir: dir, Clang: fuzzCXX(t), CacheDir: "off", Timeout: 5 * time.Minute, Include: []string{`^M\d+$`}})
	if err != nil {
		t.Fatalf("bindgen: %v", err)
	}
	view := string(res.Source)
	if !compare {
		return // the view was made; what it says about undefined behaviour is not compared
	}
	// the expected values
	exe := filepath.Join(dir, "fuzz")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	comp := exec.CommandContext(ctx, cxx, "-std=c++20", "-w", "-o", exe, filepath.Join(dir, "fuzz.cpp"))
	build.InGroup(comp)
	if out, err := comp.CombinedOutput(); err != nil {
		t.Fatalf("the expectation program: %v\n%s", err, out)
	}
	cmd := exec.CommandContext(ctx, exe)
	build.InGroup(cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("the expectation program: %v", err)
	}
	exp := map[string]string{}
	for _, l := range strings.Split(out.String(), "\n") {
		if f := strings.Fields(l); len(f) == 2 {
			exp[f[0]] = f[1]
		}
	}
	got := map[string]string{}
	for _, m := range viewVar.FindAllStringSubmatch(view, -1) {
		got[m[1]] = m[2]
	}
	notTranslated := map[string]bool{}
	if i := strings.Index(view, "// Not translated"); i >= 0 {
		for _, m := range viewNot.FindAllString(view[i:], -1) {
			notTranslated[m] = true
		}
	}
	wrong, listed := 0, 0
	for i, c := range cases {
		k := fmt.Sprintf("M%d", i)
		lit, ok := got[k]
		if !ok {
			if !notTranslated[k] {
				t.Errorf("%s (%s){ %s } is neither translated nor listed", k, c.typ, c.expr)
			}
			listed++
			continue
		}
		if !fuzzSame(c.typ, lit, exp[k]) {
			wrong++
			t.Errorf("%s (%s){ %s }: the view says %s, the compiler %s", k, c.typ, c.expr, lit, exp[k])
		}
	}
	if len(cases)-listed < len(cases)/3 {
		t.Errorf("only %d of %d macros were translated: the check says too little", len(cases)-listed, len(cases))
	}
	t.Logf("%d macros: %d compared, %d listed, %d wrong", len(cases), len(cases)-listed, listed, wrong)
}

func fuzzSame(typ, lit, want string) bool {
	if typ == "double" || typ == "float" {
		g, err1 := strconv.ParseFloat(lit, 64)
		w, err2 := strconv.ParseFloat(want, 64)
		if err1 != nil || err2 != nil {
			return false
		}
		if typ == "float" {
			g, w = float64(float32(g)), float64(float32(w))
		}
		return g == w || math.IsNaN(g) && math.IsNaN(w)
	}
	if typ == "bool" && lit != "true" && lit != "false" {
		return false // a C.bool field takes Go's true and false
	}
	if typ == "bool" {
		if l, ok := map[string]string{"true": "1", "false": "0"}[lit]; ok {
			lit = l
		}
	}
	a, ok1 := new(big.Int).SetString(lit, 0)
	b, ok2 := new(big.Int).SetString(want, 0)
	return ok1 && ok2 && a.Cmp(b) == 0
}

func fuzzCXX(t *testing.T) build.CXX { return tsupport.Clang(t) }

// Cases the compiler would not let through a macro (it rejects the division by zero), checked
// on the tree directly: the folding refuses what C leaves undefined.
func TestFoldRefuses(t *testing.T) {
	typ := &jtype{QualType: "int"}
	lit := func(v string) jnode {
		return jnode{Kind: "IntegerLiteral", Type: typ, Value: []byte(`"` + v + `"`)}
	}
	bin := func(op, a, b string) *jnode {
		return &jnode{Kind: "BinaryOperator", Opcode: op, Type: typ, Inner: []jnode{lit(a), lit(b)}}
	}
	for _, c := range []struct {
		n    *jnode
		want string // "" for refused
	}{
		{bin("/", "7", "2"), "3"}, {bin("/", "-7", "2"), "-3"}, {bin("%", "-7", "3"), "-1"},
		{bin("/", "7", "0"), ""}, {bin("%", "7", "0"), ""},
		{bin("/", "-2147483648", "-1"), ""}, {bin("%", "-2147483648", "-1"), ""},
		{bin("+", "2147483647", "1"), ""}, {bin("<<", "1", "32"), ""}, {bin("<<", "1", "-1"), ""}, {bin(">>", "-8", "1"), "-4"},
		{bin("<<", "1", "31"), "-2147483648"},
	} {
		v, ok := fold(c.n)
		got := ""
		if ok {
			got, _ = v.literal()
		}
		if got != c.want {
			t.Errorf("%s %s %s = %q, want %q", c.n.Inner[0].Value, c.n.Opcode, c.n.Inner[1].Value, got, c.want)
		}
	}
}
