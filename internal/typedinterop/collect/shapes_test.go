package collect

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func TestMain(m *testing.M) { tsupport.Main(m) }

const stdImports = `	"fmt"
	cstdio "<cstdio>"
	cstring "<cstring>"
	cmath "<cmath>"
	cstdlib "<cstdlib>"
	"<vector>"
	"<string>"
	"<map>"
	"<optional>"
	"<chrono>"
	"<algorithm>"
	"<utility>"
`

// A shape is a body of main; testdata/shapes/<name>.txt has the references that
// the collector makes of it: the lines of Set.Format without the number and the
// sites, in order (reviewed when it changes: -update).
type shape struct {
	name string
	body string
}

func runShape(t *testing.T, s shape) {
	t.Helper()
	src := "package main\n\nimport (\n" + stdImports + ")\n\nfunc main() {\n" + s.body + "\n\tfmt.Println()\n}\n"
	p, fset := tsupport.CheckSource(t, src, nil)
	if errs := p.Errors.Diagnostics(); len(errs) > 0 {
		t.Fatalf("the program doesn't check: %v\n%s", errs, src)
	}
	set := Collect(p, fset)
	var b strings.Builder
	set.Format(&b, false)
	got := b.String()
	if os.Getenv("SHAPE_DUMP") != "" {
		t.Logf("\n%s", got)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		if _, rest, ok := strings.Cut(line, " "); ok && line != "" {
			lines = append(lines, rest)
		}
	}
	got = strings.Join(lines, "\n") + "\n"
	path := filepath.Join("testdata", "shapes", s.name+".txt")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(want) != got {
		t.Errorf("references changed (run with -update and review the diff):\n%s", diffText(string(want), got))
	}
}

func TestShapes(t *testing.T) {
	for _, s := range shapeCases {
		t.Run(s.name, func(t *testing.T) { runShape(t, s) })
	}
}

func goldenDir() string { return filepath.Join("testdata", "golden") }

func TestGoldens(t *testing.T) {
	root := filepath.Join("..", "..", "..", "tests", "interop")
	for _, name := range goldenPrograms {
		t.Run(name, func(t *testing.T) {
			dir, err := filepath.Abs(root)
			if err != nil {
				t.Fatal(err)
			}
			p, fset := tsupport.CheckDir(t, dir, name+".gp")
			if errs := p.Errors.Diagnostics(); len(errs) > 0 {
				t.Skipf("the program doesn't check here (a missing library?): %v", errs[0])
			}
			set := Collect(p, fset)
			if len(set.Refs) == 0 {
				t.Fatal("no references")
			}
			got := set.String()
			path := filepath.Join(goldenDir(), name+".txt")
			if *update {
				if err := os.MkdirAll(goldenDir(), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			if string(want) != got {
				t.Errorf("the references changed (run with -update and review the diff):\n%s", diffText(string(want), got))
			}
		})
	}
}

func diffText(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	n := 0
	for i := 0; i < max(len(w), len(g)) && n < 20; i++ {
		var a, c string
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			c = g[i]
		}
		if a != c {
			b.WriteString("- " + a + "\n+ " + c + "\n")
			n++
		}
	}
	return b.String()
}

var goldenPrograms = []string{
	"cxx_containers", "c_unions", "c_operators", "cookbook_std_containers", "cxx_features", "minmax_values",
	"range_containers", "template_or_index", "policy_rows", "cmath_funcs", "stdio_calls", "string_ops",
	"vector_ops", "map_ops", "c_bitfields", "cstr_holder", "embed_cxx", "closure_members", "cxx_inheritance",
	"defer_objects", "pointer_chains", "iterator_offsets", "header_namespace", "helper_header",
	"macro_collisions", "doc_namespaces", "unsafe_pointer", "generics_cxx_explicit", "cookbook_sqlite", "cookbook_zlib",
}

var shapeCases = []shape{
	{name: "call_c_function", body: "\tn := cstring.strlen(\"abc\")\n\t_ = n"},
	{name: "call_with_go_string", body: "\ts := \"abc\"\n\tn := cstring.strlen(C.cstr(s))\n\t_ = n"},
	{name: "call_go_int", body: "\tk := 3\n\tx := cmath.abs(k)\n\t_ = x"},
	{name: "call_go_float", body: "\tf := 2.5\n\tx := cmath.sqrt(f)\n\t_ = x"},
	{name: "call_untyped_float", body: "\tx := cmath.sqrt(2)\n\t_ = x"},
	{name: "call_untyped_rune", body: "\tx := cstring.strchr(C.cstr(\"abc\"), 'b')\n\t_ = x"},
	{name: "macro_value", body: "\tvar buf [cstdio.BUFSIZ]byte\n\t_ = buf"},
	{name: "macro_in_const", body: "\tconst n = cstdio.EOF\n\t_ = n"},
	{name: "macro_in_case", body: "\tc := 1\n\tswitch c {\n\tcase cstdio.EOF:\n\t}"},
	{name: "macro_value_expr", body: "\tx := cstdio.EOF\n\t_ = x"},
	{name: "template_instance", body: "\tvar v std.vector[int]\n\t_ = v"},
	{name: "template_two_args", body: "\tvar m std.map[string, int]\n\t_ = m"},
	{name: "template_literal", body: "\tv := std.vector[int]{1, 2, 3}\n\t_ = v"},
	{name: "member_call_on_local", body: "\tv := std.vector[int]{1, 2, 3}\n\tn := v.size()\n\t_ = n"},
	{name: "member_chain", body: "\tv := std.vector[int]{1, 2, 3}\n\tn := v.front()\n\tm := n + 1\n\t_ = m"},
	{name: "local_chain", body: "\ts := std.string(\"abc\")\n\tn := s.size()\n\tm := n * 2\n\t_ = m"},
	{name: "tuple_assign", body: "\ta, b := std.minmax(3, 4)\n\t_, _ = a, b"},
	{name: "range_value", body: "\tv := std.vector[int]{1, 2}\n\tfor _, x := range v {\n\t\t_ = x\n\t}"},
	{name: "range_map", body: "\tm := std.map[string, int]{}\n\tfor k, v := range m {\n\t\t_, _ = k, v\n\t}"},
	{name: "operator_mixed", body: "\ts := std.string(\"abc\")\n\tn := s.size() + len(\"x\")\n\t_ = n"},
	{name: "operator_compare", body: "\ts := std.string(\"abc\")\n\tok := s.size() > 2\n\t_ = ok"},
	{name: "operator_unary", body: "\tx := -cstdio.EOF\n\t_ = x"},
	{name: "operator_andnot", body: "\tx := cstdio.EOF &^ 3\n\t_ = x"},
	{name: "index_value", body: "\tv := std.vector[int]{1, 2, 3}\n\tx := v[1]\n\t_ = x"},
	{name: "index_go_var", body: "\tv := std.vector[int]{1, 2, 3}\n\ti := 1\n\tx := v[i]\n\t_ = x"},
	{name: "address_of", body: "\tv := std.vector[int]{1}\n\tp := &v\n\t_ = p"},
	{name: "deref", body: "\tv := std.vector[int]{1}\n\tp := &v\n\tq := *p\n\t_ = q"},
	{name: "conversion_to_go", body: "\tn := int(cstring.strlen(C.cstr(\"a\")))\n\t_ = n"},
	{name: "convert_to_cint", body: "\tk := 3\n\tx := cmath.abs(C.int(k))\n\t_ = x"},
	{name: "cgo_type_decl", body: "\tvar x C.int\n\t_ = x"},
	{name: "namespace_path", body: "\tt := std.chrono.steady_clock.now()\n\t_ = t"},
	{name: "paren", body: "\tx := (cstdio.EOF)\n\t_ = x"},
	{name: "dedup", body: "\ta := cstring.strlen(C.cstr(\"abc\"))\n\tb := cstring.strlen(C.cstr(\"abc\"))\n\t_, _ = a, b"},
	{name: "no_cxx_no_refs", body: "\tx := 1 + 2\n\t_ = x"},
	{name: "go_function_call", body: "\tk := cmath.abs(len(\"abc\"))\n\t_ = k"},
	{name: "variadic_args", body: "\tcstdio.printf(C.cstr(\"%d %d\\n\"), 1, 2)"},
	{name: "member_without_call", body: "\tp := std.pair[int, int]{1, 2}\n\tx := p.first\n\t_ = x"},
}
