package main

import (
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lower"
)

// TestTranslateNames checks the rewriting of C++ names in compiler messages (step 13.1).
func TestTranslateNames(t *testing.T) {
	tests := []struct{ in, want string }{
		// gocpp types
		{"gp::string", "string"},
		{"cannot convert 'gp::string' to 'const char*'", "cannot convert 'string' to 'const char*'"},
		{"gp::slice<int64_t>", "[]int64_t"},
		{"gp::slice<long>", "[]int64_t"},
		{"gp::slice<long int>", "[]int64_t"},
		{"gp::slice<int>", "[]int32"},
		{"gp::slice<uint8_t>", "[]byte"},
		{"gp::slice<unsigned char>", "[]byte"},
		{"gp::slice<double>", "[]float64"},
		{"gp::slice<float>", "[]float32"},
		{"gp::slice<bool>", "[]bool"},
		{"gp::slice<gp::string>", "[]string"},
		{"gp::slice<gp::slice<int64_t> >", "[][]int64_t"},
		{"gp::slice<gp::slice<int64_t>>", "[][]int64_t"},
		{"gp::array<int64_t, 3>", "[3]int64_t"},
		{"gp::array<gp::string, 4UL>", "[4]string"},
		{"gp::ptr<gpkg_main::Point>", "*main.Point"},
		{"gp::cptr<FILE>", "*FILE"},
		{"gp::ptr<gp::ptr<int64_t> >", "**int64_t"},
		{"gp::map<gp::string, int64_t>", "map[string]int64_t"},
		{"gp::map<gp::string, gp::slice<gpkg_main::T> >", "map[string][]main.T"},
		{"gp::chan<int64_t>", "chan int64_t"},
		{"gp::chan<gp::any>", "chan any"},
		{"gp::any", "any"},
		{"gp::error", "error"},
		{"gp::complex128", "complex128"},
		{"gp::func<void()>", "func()"},
		{"gp::func<int64_t(int64_t, gp::string)>", "func(int64_t, string) int64_t"},
		{"gp::func<void(gp::ptr<gpkg_main::T>)>", "func(*main.T)"},
		{"gp::func<std::tuple<int64_t, gp::error>(gp::string)>", "func(string) (int64_t, error)"},
		{"gp::func<gp::func<void()>(int64_t)>", "func(int64_t) func()"},
		{"gp::func<gp::string(void)>", "func() string"},
		{"gp::newtype<gpkg_main::Celsius, double>", "main.Celsius"},
		// packages
		{"gpkg_main::Point", "main.Point"},
		{"::gpkg_main::Point", "main.Point"},
		{"gpkg_main::Point::Len", "main.Point.Len"},
		{"gpkg_github_dcom__huuhait__game__engine::Body", "engine.Body"},
		{"gpkg_github_dcom__mattn__go_hsqlite3::DB", "go-sqlite3.DB"},
		{"gpkg_gopkg_din__yaml_dv3::Node", "yaml.Node"},
		{"gpkg_example_dcom__lib__v2::T", "lib.T"},
		{"gpkg_a_ub_tc_pd::T", "a_b~c+d.T"},
		{"gpkg__xc3_xa9::T", "é.T"},
		{"gpkg_main::Stack<int64_t>", "main.Stack[int64_t]"},
		{"gpkg_main::Pair<gp::string, int64_t>", "main.Pair[string, int64_t]"},
		{"gpkg_io::EOF_", "io.EOF"},
		{"gpkg_main::new_", "main.new"},
		{"gpkg_main", "main"},
		{"no member named 'x' in 'gpkg_main::T'", "no member named 'x' in 'main.T'"},
		// C++ types stay
		{"std::vector<int>", "std::vector<int>"},
		{"std::vector<long>", "std::vector<long>"},
		{"std::vector<int, std::allocator<int> >", "std::vector<int>"},
		{"std::vector<gp::string>", "std::vector<string>"},
		{"std::vector<gp::string, std::allocator<gp::string> >", "std::vector<string>"},
		{"std::__cxx11::list<int>", "std::list<int>"},
		{"std::__1::vector<int>", "std::vector<int>"},
		{"std::__cxx11::basic_string<char>", "std::string"},
		{"std::basic_string<char, std::char_traits<char>, std::allocator<char> >", "std::string"},
		{"std::__cxx11::basic_string<char, std::char_traits<char>, std::allocator<char> >", "std::string"},
		{"std::basic_string<wchar_t>", "std::wstring"},
		{"std::basic_string_view<char, std::char_traits<char> >", "std::string_view"},
		{"std::map<int, int, std::less<int>, std::allocator<std::pair<const int, int> > >", "std::map<int, int>"},
		{"std::unordered_map<int, double, std::hash<int>, std::equal_to<int>, std::allocator<std::pair<const int, double> > >", "std::unordered_map<int, double>"},
		{"std::vector<std::__cxx11::basic_string<char> >", "std::vector<std::string>"},
		{"std::vector<int>::iterator", "std::vector<int>::iterator"},
		{"std::sort<int>", "std::sort<int>"},
		{"std::max", "std::max"},
		{"std::tuple<int, long>", "std::tuple<int, long>"},
		{"std::less<int>", "std::less<int>"},
		{"point", "point"},
		{"const char*", "const char*"},
		// messages with several names
		{"cannot convert 'gp::slice<int64_t>' to 'gp::slice<long int>' in initialization", "cannot convert '[]int64_t' to '[]int64_t' in initialization"},
		{"no viable conversion from 'gp::slice<int>' to 'gp::slice<int64_t>'", "no viable conversion from '[]int32' to '[]int64_t'"},
		{"cannot convert ‘gp::map<gp::string, int>’ to ‘int’", "cannot convert ‘map[string]int32’ to ‘int’"},
		// not names of gocpp: no change
		{"my_gpkg_x::T", "my_gpkg_x::T"},
		{"mygp::string", "mygp::string"},
		{"foo::gp::string", "foo::gp::string"},
		{"foo::gpkg_main::T", "foo::gpkg_main::T"},
		{"xstd::vector<int>", "xstd::vector<int>"},
		{"gp::range: the elements are C arrays, which can't be copied", "gp::range: the elements are C arrays, which can't be copied"},
		{"gp::cgo::as<int>", "gp::cgo::as<int>"},
		{"gp::fmt::Println<gp::string>", "gp::fmt::Println<string>"},
		{"gp::slice<int>::iterator", "gp::slice<int>::iterator"},
		{"gp::unknown<gp::string, long>", "gp::unknown<string, long>"},
		{"gpkg_main_x::T", "gpkg_main_x::T"}, // not a namespace that NamespaceName makes
		{"gpkg_x_::T", "gpkg_x_::T"},
		// broken: the compiler cut the type
		{"gp::slice<int64_t", "gp::slice<int64_t"},
		{"gp::map<gp::string, ...>", "gp::map<string, ...>"},
		{"gp::slice<int64_t>\nnext gp::string", "[]int64_t\nnext string"},
		{"", ""},
		{"gp::", "gp::"},
		{"std::", "std::"},
		{"gp::slice<>", "gp::slice<>"},
	}
	var tr nameTranslator
	for _, tt := range tests {
		if got := tr.translate(tt.in); got != tt.want {
			t.Errorf("translate(%q)\n got %q\nwant %q", tt.in, got, tt.want)
		}
	}
}

func TestTranslateNamesOptions(t *testing.T) {
	tr := nameTranslator{pkgName: func(path string) string {
		if path == "github.com/x/y" {
			return "github.com/x/y" // two packages named y: the path
		}
		return ""
	}}
	if got, want := tr.translate("gp::ptr<gpkg_github_dcom__x__y::T> and gpkg_main::U"), "*github.com/x/y.T and main.U"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	win := nameTranslator{windows: true}
	if got, want := win.translate("gp::slice<long> gp::slice<long long> gp::slice<unsigned long>"), "[]int32 []int64_t []uint32"; got != want {
		t.Errorf("windows: got %q, want %q", got, want)
	}
	lin := nameTranslator{}
	if got, want := lin.translate("gp::slice<long> gp::slice<long long> gp::slice<unsigned long>"), "[]int64_t []int64 []uint64_t"; got != want {
		t.Errorf("linux: got %q, want %q", got, want)
	}
}

func TestSymbolName(t *testing.T) {
	var tr nameTranslator
	for _, tt := range []struct{ in, want string }{
		{"gpkg_main::main()", "main.main"},
		{"gpkg_main::T::M(long, gp::string)", "main.T.M"},
		{"gpkg_github_dcom__a__b::F(int)", "b.F"},
		{"gpkg_main::main()::$_0::operator()(long) const", "main.main"},
		{"gpkg_main::main()::{lambda(long)#1}::operator()(long) const", "main.main"},
		{"gp::rt_init()", "gp::rt_init"},
		{"main", "main"},
	} {
		if got := tr.symbolName(tt.in); got != tt.want {
			t.Errorf("symbolName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestTranslateRoundTrip: random package paths go through NamespaceName, a type name and
// translate, and come out as the package's name and the type.
func TestTranslateRoundTrip(t *testing.T) {
	rnd := rand.New(rand.NewPCG(7, 9))
	alphabet := []string{"a", "b", "z", "x", "9", "_", ".", "-", "~", "+", "é", "日", "/", "q", "go", "v2"}
	var tr nameTranslator
	for i := 0; i < 3000; i++ {
		var b strings.Builder
		for n := 1 + rnd.IntN(10); n > 0; n-- {
			b.WriteString(alphabet[rnd.IntN(len(alphabet))])
		}
		path := b.String()
		if strings.HasSuffix(path, "/") || strings.HasPrefix(path, "/") || strings.Contains(path, "//") {
			continue
		}
		ns := lower.NamespaceName(path)
		got := tr.translate("cannot convert 'gp::ptr<" + ns + "::T>' to '" + ns + "::U'")
		name := defaultPkgName(path)
		if want := "cannot convert '*" + name + ".T' to '" + name + ".U'"; got != want {
			t.Fatalf("path %q ns %q:\n got %q\nwant %q", path, ns, got, want)
		}
	}
}

// TestMessage checks what a report shows of a compiler message: the names turned into
// gocpp's, and clang's and gcc's extras left out.
func TestMessage(t *testing.T) {
	var tr nameTranslator
	tr.setHints([]string{"main.Point", "main.Celsius", "chan main.Point"}) // the types of the .gp line
	tests := []struct{ in, want string }{
		// clang writes the type without its namespace and with the aka of the runtime
		{"no viable conversion from 'slice<long>' to 'int'", "no viable conversion from '[]int64_t' to 'int'"},
		{"no viable conversion from 'map<string, long>' to 'int'", "no viable conversion from 'map[string]int64_t' to 'int'"},
		{"no viable conversion from '[]int' (aka 'slice<long>') to 'int'", "no viable conversion from '[]int' to 'int'"},
		{"no viable conversion from 'gp::slice<long>' (aka 'slice<long>') to 'int'", "no viable conversion from '[]int64_t' to 'int'"},
		{"no viable conversion from 'string' (aka 'gp::string') to 'int'", "no viable conversion from 'string' to 'int'"},
		{"no viable conversion from 'chan<Point>' to 'int'", "no viable conversion from 'chan Point' to 'int'"},
		{"no viable conversion from 'ptr<Point>' to 'int'", "no viable conversion from '*Point' to 'int'"},
		{"no viable conversion from 'callback<std::tuple<long, error> (string), func<std::tuple<long, error> (string)>>' to 'int'", "no viable conversion from 'func(string) (int64_t, error)' to 'int'"},
		// clang leaves out the namespace of a type of the package, and so it does for a C or C++ type of
		// the same name: a bare name stays as the compiler wrote it, whatever the line says
		{"no viable conversion from 'Point' to 'int'", "no viable conversion from 'Point' to 'int'"},
		{"cannot initialize a parameter of type 'int' with an rvalue of type 'Point *'", "cannot initialize a parameter of type 'int' with an rvalue of type 'Point *'"},
		{"no member named 'Point' in 'Celsius'", "no member named 'Point' in 'Celsius'"},
		{"no viable conversion from 'main.Point' to 'int'", "no viable conversion from 'main.Point' to 'int'"},
		{"no viable conversion from 'other::Point' to 'int'", "no viable conversion from 'other::Point' to 'int'"},
		{"no viable conversion from 'std::vector<Point>' to 'int'", "no viable conversion from 'std::vector<Point>' to 'int'"},
		// the namespace in the text says it is the package's type; the aka says the same
		{"no viable conversion from 'Node' (aka 'gpkg_main::Node') to 'Node'", "no viable conversion from 'main.Node' to 'Node'"},
		{"could not convert 'x' from 'gpkg_main::Node*' to 'Node*'", "could not convert 'x' from 'main.Node*' to 'Node*'"},
		{"invalid conversion from 'gpkg_main::Point*' to 'int' [-fpermissive]", "invalid conversion from 'main.Point*' to 'int'"},
		// clang's aka of a C++ template only spells it out
		{"no viable conversion from '[]string' to 'const std::vector<std::string>' (aka 'const vector<basic_string<char>>')", "no viable conversion from '[]string' to 'const std::vector<std::string>'"},
		// the aka of a plain typedef says something: it stays
		{"unknown type name 'myint' (aka 'int')", "unknown type name 'myint' (aka 'int')"},
	}
	for _, tt := range tests {
		if got := tr.message(tt.in); got != tt.want {
			t.Errorf("message(%q)\n got %q\nwant %q", tt.in, got, tt.want)
		}
	}
}

// TestHints checks the Go types that one C++ type stands for: int and int64 are int64_t, uint,
// uint64 and uintptr are uint64_t, and every channel is gp::chan. The line of the error says which
// the user wrote; without it (or with two that fit) the message keeps the C++ name and never
// guesses.
func TestHints(t *testing.T) {
	tests := []struct {
		hints []string
		in    string
		want  string
	}{
		{[]string{"[]int64"}, "gp::slice<int64_t>", "[]int64"},
		{[]string{"[]int"}, "gp::slice<long>", "[]int"},
		{[]string{"[]int", "int"}, "gp::slice<long int>", "[]int"},
		{[]string{"[]int", "[]int64"}, "gp::slice<long int>", "[]int64_t"},
		{nil, "gp::slice<long int>", "[]int64_t"},
		{[]string{"[]uint64"}, "gp::slice<unsigned long>", "[]uint64"},
		{[]string{"[]uintptr"}, "gp::slice<uint64_t>", "[]uintptr"},
		{[]string{"[]uint"}, "gp::slice<uint64_t>", "[]uint"},
		{[]string{"[]uint", "[]uintptr"}, "gp::slice<uint64_t>", "[]uint64_t"},
		{[]string{"[]rune"}, "gp::slice<int>", "[]rune"},
		{[]string{"[]uint8"}, "gp::slice<uint8_t>", "[]uint8"},
		{nil, "gp::slice<uint8_t>", "[]byte"},
		{[]string{"chan<- int"}, "gp::chan<int64_t>", "chan<- int"},
		{[]string{"<-chan int64"}, "gp::chan<long>", "<-chan int64"},
		{[]string{"chan int", "<-chan int"}, "gp::chan<long>", "gp::chan<int64_t>"},
		{nil, "gp::chan<gp::string>", "chan string"},
		{nil, "gp::chan<std::string>", "chan std::string"},
		{[]string{"chan std.string", "[]int"}, "gp::chan<std::string>", "chan std::string"},
		{[]string{"<-chan string", "chan<- string"}, "gp::chan<gp::string>", "gp::chan<string>"},
		{[]string{"complex128"}, "gp::complex<double>", "complex128"},
		{[]string{"chan<- string"}, "gp::chan<gp::string>", "chan<- string"},
		{[]string{"map[string]int64"}, "gp::map<gp::string, int64_t>", "map[string]int64"},
		{[]string{"func(int64, []uint64) int64"}, "gp::func<int64_t(int64_t, gp::slice<uint64_t>)>", "func(int64, []uint64) int64"},
		{[]string{"main.Box[int64]"}, "gpkg_main::Box<int64_t>", "main.Box[int64]"},
		{[]string{"[]int64"}, "gp::slice<int64_t> and gp::slice<int64_t>", "[]int64 and []int64"},
		{[]string{"[]string"}, "gp::slice<gp::string>", "[]string"},
	}
	for _, tt := range tests {
		var tr nameTranslator
		tr.setHints(tt.hints)
		if got := tr.translate(tt.in); got != tt.want {
			t.Errorf("hints %q: translate(%q)\n got %q\nwant %q", tt.hints, tt.in, got, tt.want)
		}
	}
	var tr0 nameTranslator
	for _, tt := range []struct{ in, want string }{
		{"cannot convert 'gp::chan<gp_anon_a9e1fb1acaf8c517>' to 'int'", "cannot convert 'chan struct{...}' to 'int'"},
		{"cannot convert 'gp_anon_i5b009a885429a612' to 'int'", "cannot convert 'interface{...}' to 'int'"},
		{"cannot convert 'gp_anon_5b009a885429a612*' to 'int'", "cannot convert 'struct{...}*' to 'int'"},
		{"cannot convert 'std::string_view' {aka 'std::string_view'} to 'int'", "cannot convert 'std::string_view' to 'int'"},
		{"cannot convert 'gp::complex128' {aka 'gp::complex<double>'} to 'int'", "cannot convert 'complex128' to 'int'"},
		{"cannot convert 'std::vector<int>' {aka 'std::vector<int, std::allocator<int> >'} to 'int'", "cannot convert 'std::vector<int>' to 'int'"},
		{"cannot convert 'x' to 'y' [-Wtemplate-body]", "cannot convert 'x' to 'y'"},
		{"static assertion failed: \"gp::slice<long> is wrong\" for gp::string", "static assertion failed: \"gp::slice<long> is wrong\" for string"},
		{"in (lambda at /tmp/q/main.gp:16:63)", "in (lambda at ./main.gp:16:63)"},
	} {
		tr0.shortFile = func(f string) string { return "./" + filepath.Base(f) }
		if got := tr0.message(tt.in); got != tt.want {
			t.Errorf("message(%q)\n got %q\nwant %q", tt.in, got, tt.want)
		}
	}
}

func TestCanonGo(t *testing.T) {
	same := [][2]string{{"int64", "int"}, {"uintptr", "uint64"}, {"byte", "uint8"}, {"rune", "int32"}, {"<-chan int", "chan<- int64"}, {"map[string][]rune", "map[string][]int32"}}
	for _, p := range same {
		if canonGo(p[0]) != canonGo(p[1]) {
			t.Errorf("canonGo(%q) = %q, canonGo(%q) = %q: want equal", p[0], canonGo(p[0]), p[1], canonGo(p[1]))
		}
	}
	differ := [][2]string{{"int", "int32"}, {"uint", "int"}, {"[]int", "int"}, {"int16", "int"}, {"chan int", "int"}, {"uint16", "uint8"}}
	for _, p := range differ {
		if canonGo(p[0]) == canonGo(p[1]) {
			t.Errorf("canonGo(%q) = canonGo(%q) = %q: want different", p[0], p[1], canonGo(p[0]))
		}
	}
}

// TestMessageIdentity: a message that names nothing of gocpp (no gp::, gpkg_ or std:: name) is
// shown as the compiler wrote it, whatever the package declares: no bare word of it is read as a
// type of the package, or as a member or a suggestion.
func TestMessageIdentity(t *testing.T) {
	words := []string{"Point", "Node", "push_bak", "size", "value", "x", "int", "'", "'Point'", "'Node'", "'push_bak'", "'size'", "'value'",
		"no member named", "did you mean", "has no member named", "cannot convert", "to", "from", "in", "undeclared identifier",
		"candidate:", "(aka", "Point*", "Node &", "::", ".", "const char*", "long int", "[-fpermissive]", "main.Point"}
	rnd := rand.New(rand.NewPCG(7, 8))
	var tr nameTranslator
	tr.setHints([]string{"main.Other", "h.Node", "[]int", "main.Node"}) // h.Node: another Node on the line
	for i := 0; i < 3000; i++ {
		var parts []string
		for n := 1 + rnd.IntN(8); n > 0; n-- {
			parts = append(parts, words[rnd.IntN(len(words))])
		}
		in := strings.Join(parts, " ")
		if strings.Contains(in, "[-fpermissive]") || strings.Contains(in, "(aka") {
			continue // the two extras that message drops on purpose
		}
		if got := tr.message(in); got != in {
			t.Fatalf("message(%q) = %q, want the same", in, got)
		}
	}
	for _, in := range []string{
		"no member named 'push_bak' in 'Point'",
		"'Node' has no member named 'value'; did you mean 'size'?",
		"no viable conversion from 'Node' to 'int'",
		"use of undeclared identifier 'Point'",
	} {
		if got := tr.message(in); got != in {
			t.Errorf("message(%q) = %q, want the same", in, got)
		}
	}
}

// TestDefaultArgsOnly: the trailing arguments of a container that the compiler spells out are
// dropped when they are the defaults, and kept when the user chose them.
func TestDefaultArgsOnly(t *testing.T) {
	var tr nameTranslator
	for _, tt := range []struct{ in, want string }{
		{"std::map<int, int, std::less<int>, std::allocator<std::pair<const int, int> > >", "std::map<int, int>"},
		{"std::map<int, int, std::less<void>, std::allocator<std::pair<const int, int> > >", "std::map<int, int, std::less<void>>"},
		{"std::map<int, int, std::greater<int>, std::allocator<std::pair<const int, int> > >", "std::map<int, int, std::greater<int>>"},
		{"std::set<int, std::less<int>, std::allocator<long> >", "std::set<int, std::less<int>, std::allocator<long>>"},
		{"std::vector<int, std::allocator<int> >", "std::vector<int>"},
		{"std::vector<int, std::allocator<char> >", "std::vector<int, std::allocator<char>>"},
		{"std::unordered_set<int, std::hash<long>, std::equal_to<int>, std::allocator<int> >", "std::unordered_set<int, std::hash<long>>"},
		{"std::unordered_set<int, std::hash<int>, std::equal_to<int>, std::allocator<int> >", "std::unordered_set<int>"},
	} {
		if got := tr.translate(tt.in); got != tt.want {
			t.Errorf("translate(%q)\n got %q\nwant %q", tt.in, got, tt.want)
		}
	}
}
