package typedinterop

import (
	"strings"
	"testing"
)

const (
	impVector = "\t\"<vector>\"\n"
	impString = "\t\"<string>\"\n"
	impMap    = "\t\"<map>\"\n"
	impAlgo   = "\t\"<algorithm>\"\n"
	impCstdio = "\tcstdio \"<cstdio>\"\n"
	impCstr   = "\tcstring \"<cstring>\"\n"
	impCmath  = "\tcmath \"<cmath>\"\n"
)

func TestClangStd(t *testing.T) {
	runCases(t, []clangCase{
		{name: "vector size", imports: impVector, body: "\tv := std.vector[int]{1, 2, 3}\n\tn := v.size()\n\t_ = n",
			want: []expect{{"v.size()", "prvalue unsigned long"}, {"local:n", "type unsigned long"}, {"local:v", "type std::vector<long>"}}},
		{name: "vector front is an lvalue", imports: impVector, body: "\tv := std.vector[int]{1}\n\tx := v.front()\n\t_ = x",
			want: []expect{{"v.front()", "lvalue long"}, {"local:x", "type long"}}},
		{name: "vector push_back is void", imports: impVector, body: "\tv := std.vector[int]{}\n\tv.push_back(4)",
			want: []expect{{"v.push_back(4)", "prvalue void"}}},
		{name: "vector at with a Go index", imports: impVector, body: "\tv := std.vector[int]{1}\n\ti := 0\n\tx := v.at(i)\n\t_ = x",
			want: []expect{{"v.at(i)", "lvalue long"}}},
		{name: "vector index", imports: impVector, body: "\tv := std.vector[int]{1, 2}\n\tx := v[1]\n\t_ = x",
			want: []expect{{"v[1]", "lvalue long"}}},
		{name: "vector of vectors", imports: impVector, body: "\tv := std.vector[std.vector[int]]{}\n\tn := v.size()\n\t_ = n",
			want: []expect{{"v.size()", "prvalue unsigned long"}, {"local:v", "type std::vector<std::vector<long>>"}}},
		{name: "string c_str", imports: impString, body: "\ts := std.string(\"abc\")\n\tp := s.c_str()\n\t_ = p",
			want: []expect{{"s.c_str()", "prvalue const char *"}, {"local:s", "type std::basic_string<char>"}}},
		{name: "string npos static member", imports: impString, body: "\tx := std.string.npos\n\t_ = x",
			want: []expect{{"C.std.string.npos", "lvalue const unsigned long"}}},
		{name: "string append returns a reference", imports: impString, body: "\ts := std.string(\"a\")\n\tr := s.append(\"b\")\n\t_ = r",
			want: []expect{{"s.append(\"b\")", "lvalue std::basic_string<char>"}}},
		{name: "map size", imports: impMap + impString, body: "\tm := std.map[string, int]{}\n\tn := m.size()\n\t_ = n",
			want: []expect{{"m.size()", "prvalue unsigned long"}}},
		{name: "map range", imports: impMap + impString, body: "\tm := std.map[string, int]{}\n\tfor k, v := range m {\n\t\t_, _ = k, v\n\t}",
			want: []expect{{"local:k", "type gocpp_probe_internal_pr::string"}, {"local:v", "type long"}}},
		{name: "range over a vector", imports: impVector, body: "\tv := std.vector[int]{1}\n\tfor i, x := range v {\n\t\t_, _ = i, x\n\t}",
			want: []expect{{"local:i", "type long"}, {"local:x", "type long"}}},
		{name: "optional", imports: "\t\"<optional>\"\n", body: "\to := std.optional[int]{}\n\tx := o.value_or(3)\n\t_ = x",
			want: []expect{{"o.value_or(3)", "prvalue long"}}},
		{name: "chrono now", imports: "\t\"<chrono>\"\n", body: "\tt := std.chrono.steady_clock.now()\n\t_ = t",
			want: []expect{{"C.std.chrono.steady_clock.now()", "~prvalue std::chrono::time_point<std::chrono::steady_clock"}}},
		{name: "minmax tuple", imports: impAlgo, body: "\ta, b := std.minmax(3, 4)\n\t_, _ = a, b",
			want: []expect{{"C.std.minmax(3, 4)", "prvalue std::pair<const int &, const int &>"}, {"local:a", "type int"}, {"local:b", "type int"}}},
		{name: "max returns a const reference", imports: impAlgo, body: "\tx := std.max(3, 4)\n\t_ = x",
			want: []expect{{"C.std.max(3, 4)", "lvalue const int"}}},
		{name: "max with mixed constants fails", imports: impAlgo, body: "\tx := std.max(2.5, 3)\n\t_ = x",
			want: []expect{{"C.std.max(2.5, 3)", "error: no matching function for call to 'max'"}}},
		{name: "max with an explicit type", imports: impAlgo, body: "\tx := std.max[float64](1, 2)\n\t_ = x",
			want: []expect{{"C.std.max[float64](1, 2)", "lvalue const double"}}},
		{name: "get from a tuple", imports: "\t\"<tuple>\"\n", body: "\tt := std.make_tuple(1, 2.5)\n\tx := std.get[0](t)\n\t_ = x",
			want: []expect{{"C.std.get[0](t)", "lvalue int"}, {"local:t", "type std::tuple<int, double>"}}},
		{name: "move gives an xvalue", imports: impVector + "\t\"<utility>\"\n", body: "\tv := std.vector[int]{1}\n\tw := std.move(v)\n\t_ = w",
			want: []expect{{"C.std.move(v)", "xvalue std::vector<long>"}}},
		{name: "string find is npos aware", imports: impString, body: "\ts := std.string(\"abc\")\n\ti := s.find(\"b\")\n\t_ = i",
			want: []expect{{"s.find(\"b\")", "prvalue unsigned long"}}},
		{name: "comparison of C++ values", imports: impString, body: "\ts := std.string(\"abc\")\n\tt := std.string(\"abd\")\n\tok := s == t\n\t_ = ok",
			want: []expect{{"s == t", "prvalue bool"}}},
		{name: "mixed arithmetic", imports: impString, body: "\ts := std.string(\"abc\")\n\tn := s.size() + len(\"xy\")\n\t_ = n",
			want: []expect{{"s.size() + len(\"xy\")", "prvalue unsigned long"}}},
	})
}

func TestClangC(t *testing.T) {
	runCases(t, []clangCase{
		{name: "strlen of a literal", imports: impCstr, body: "\tn := cstring.strlen(\"abc\")\n\t_ = n",
			want: []expect{{"C.strlen(\"abc\")", "prvalue unsigned long"}}},
		{name: "strlen of a Go string is an error as in the real build", imports: impCstr, body: "\ts := \"abc\"\n\tn := cstring.strlen(s)\n\t_ = n",
			want: []expect{{"C.strlen(s)", "error: no viable conversion from 'gocpp_probe_internal_pr::string' to 'const char *'"}}},
		{name: "strlen with cstr", imports: impCstr, body: "\ts := \"abc\"\n\tn := cstring.strlen(C.cstr(s))\n\t_ = n",
			want: []expect{{"C.strlen(C.cstr(s))", "prvalue unsigned long"}}},
		{name: "BUFSIZ", imports: impCstdio, body: "\tvar buf [cstdio.BUFSIZ]byte\n\t_ = buf",
			want: []expect{{"C.BUFSIZ", "prvalue int = 8192"}}},
		{name: "EOF is negative", imports: impCstdio, body: "\tx := cstdio.EOF\n\t_ = x",
			want: []expect{{"C.EOF", "prvalue int = -1"}}},
		{name: "INT_MAX", imports: "\tclimits \"<climits>\"\n", body: "\tconst m = climits.INT_MAX\n\t_ = m",
			want: []expect{{"C.INT_MAX", "prvalue int = 2147483647"}}},
		{name: "RAND_MAX", imports: "\tcstdlib \"<cstdlib>\"\n", body: "\tconst m = cstdlib.RAND_MAX\n\t_ = m",
			want: []expect{{"C.RAND_MAX", "prvalue int = 2147483647"}}},
		{name: "M_PI exactly", imports: impCmath, body: "\tconst p = cmath.M_PI\n\t_ = p",
			want: []expect{{"C.M_PI", "prvalue double = 3.141592653589793"}}},
		{name: "fopen result", imports: impCstdio, body: "\tf := cstdio.fopen(\"x\", \"r\")\n\t_ = f",
			want: []expect{{"C.fopen(\"x\", \"r\")", "~prvalue "}, {"local:f", "~FILE *"}}},
		{name: "stdout is a macro", imports: impCstdio, body: "\tcstdio.fputs(\"x\", cstdio.stdout)",
			want: []expect{{"C.stdout", "~FILE *"}, {"C.fputs(\"x\", C.stdout)", "prvalue int"}}},
		{name: "printf with Go arguments", imports: impCstdio, body: "\tn := 3\n\tcstdio.printf(\"%d\\n\", n)",
			want: []expect{{"C.printf(\"%d\\n\", n)", "prvalue int"}}},
		{name: "sqrt of an integer constant", imports: impCmath, body: "\tx := cmath.sqrt(2)\n\t_ = x",
			want: []expect{{"C.sqrt(2)", "prvalue double"}}},
		{name: "sqrt of a float variable", imports: impCmath, body: "\tf := 2.5\n\tx := cmath.sqrt(f)\n\t_ = x",
			want: []expect{{"C.sqrt(f)", "prvalue double"}}},
		{name: "floor of a float32 is the C function", imports: impCmath, body: "\tvar f float32 = 2.5\n\tx := cmath.floor(f)\n\t_ = x",
			want: []expect{{"C.floor(f)", "prvalue double"}}},
		{name: "abs of a Go int", imports: "\tcstdlib \"<cstdlib>\"\n", body: "\tk := 3\n\tx := cstdlib.labs(k)\n\t_ = x",
			want: []expect{{"C.labs(k)", "prvalue long"}}},
		{name: "errno is not a constant", imports: "\tcerrno \"<cerrno>\"\n", body: "\tx := cerrno.errno\n\t_ = x",
			want: []expect{{"C.errno", "lvalue int"}}},
	})
}

// A local variable whose declaration is too long to inline is a named alias
// (a DeclRef), declared once before its first use.
func TestClangLongDeclarationUsesAnAlias(t *testing.T) {
	long := strings.Repeat("abcdefghij", 40)
	rep := analyze(t, (clangCase{imports: impString, body: "\ts := std.string(\"" + long + "\")\n\tn := s.size()\n\tm := n + 1\n\t_ = m"}).program(), nil, Options{})
	if !strings.Contains(rep.Table.Source.Text, "using gp_decl_") {
		t.Fatalf("no alias in the probe file:\n%s", rep.Table.Source.Text)
	}
	if strings.Count(rep.Table.Source.Text, "using gp_decl_") != 1 {
		t.Errorf("more than one alias")
	}
	expectResult(t, rep, "s.size()", "prvalue unsigned long")
	expectResult(t, rep, "local:n", "type unsigned long")
	expectResult(t, rep, "local:m", "type unsigned long")
}
