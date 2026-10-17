package format

import (
	"go/token"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// The tests in this file check the formatting of the syntax of Phase 10:
// F!(x), func f!(), #Name(...), macro, lazy and the lines of macros on top
// of a func or type. The printer is gocpp's fork of go/printer
// (internal/syntax/printer), which has its own tests; these are the tests of
// gocpp fmt: Source and Fragment, with gofmt's rules for everything else.

// TestPhase10Source checks Source for the syntax of Phase 10: the output
// (a case without want is formatted already), and that formatting it again
// changes nothing.
func TestPhase10Source(t *testing.T) {
	runFormatCases(t, []formatCase{{
		name: "must calls",
		src: `package p

func f() {
	x := os.ReadFile!(path)
	y := parse![int](x)
	defer f.Close!()
	g!()
}
`,
	}, {
		name: "must calls are spaced like calls",
		src:  "package p\nfunc f(){x:=os.ReadFile!( path );y:=parse![ int ](x,y)}\n",
		want: "package p\n\nfunc f() { x := os.ReadFile!(path); y := parse![int](x, y) }\n",
	}, {
		name: "must functions",
		src: `package p

func loadConfig!(path string) Config {
	return cfg
}

func (s *Server) Start!() {
	s.ln.Close!()
}

func main!() {
	cfg := loadConfig!("app.json")
	_ = cfg
}

func parse![T any](s string) T {
	return zero
}
`,
	}, {
		name: "macro declarations",
		src: `package p

// Assert panics when cond is false.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

macro Dbg[T any](v T) T {
	return v
}

macro VM() {
	vmEnter(#func())
	defer vmExit(#func())
	#body()
}
`,
	}, {
		name: "macro declarations are spaced like funcs",
		src:  "package p\nmacro  Assert( cond bool,msg   lazy string ){if !cond{panic(msg())}}\n",
		want: "package p\n\nmacro Assert(cond bool, msg lazy string) {\n\tif !cond {\n\t\tpanic(msg())\n\t}\n}\n",
	}, {
		name: "lines of macros on top",
		src: `package p

#util.Trace()
#Recover()
func loadConfig!(path string) Config {
	#util.Assert(path != "", "path is empty")
	return cfg
}

#util.Locked(&s.mu)
func (s *Store) Put(k string, v int) {
	s.m[k] = v
}

#Derive(String, Equal)
type Color int

#Derive(JSON)
type (
	A int
	B int
)
`,
	}, {
		name: "lines of macros after other declarations",
		src:  "package p\nvar x int\n#a(  1 )\nfunc f(){}\ntype T int\n#b()\ntype U int\n",
		want: "package p\n\nvar x int\n\n#a(1)\nfunc f() {}\n\ntype T int\n\n#b()\ntype U int\n",
	}, {
		name: "a blank line after the lines of macros stays",
		src:  "package p\n#a()\n\nfunc f() {}\n",
		want: "package p\n\n#a()\n\nfunc f() {}\n",
	}, {
		name: "doc comments",
		src: `package p

// F does.
#util.Trace()
func F() {}

// M is a macro.
macro M() {}
`,
	}, {
		// go/printer gives a comment between two declarations an empty line
		// before it, and so does the printer here, also between the lines
		// of macros and the func.
		name: "a comment between the lines of macros and the func",
		src:  "package p\n#a()\n// c\nfunc f() {}\n",
		want: "package p\n\n#a()\n\n// c\nfunc f() {}\n",
	}, {
		name: "go syntax near the new names",
		src:  "package p\n\nfunc f() {\n\tif!(ok) {\n\t}\n\tif a!=b && !c {\n\t}\n\ts := \"#a(f!())\" // #b(g!())\n\t_ = s\n}\n",
		want: "package p\n\nfunc f() {\n\tif !(ok) {\n\t}\n\tif a != b && !c {\n\t}\n\ts := \"#a(f!())\" // #b(g!())\n\t_ = s\n}\n",
	}, {
		name: "macro is a name in a function",
		src:  "package p\n\nfunc f(macro int, lazy string) {\n\tmacro++\n\tlazy = \"x\"\n}\n\nvar macro, lazy int\n",
	}, {
		name: "gocpp names and syntax together",
		src: `package p

import "<vector>"

#util.Trace()
func f!(v std.vector[int]) std.map[std.string, int] {
	m := std.map[std.string, int]{}
	m[#file()] = parse![int](#text(v))
	return m
}
`,
	}})
}

// TestPhase10Fragment checks Fragment: a list of declarations, and a list
// of statements, with a macro use in them.
func TestPhase10Fragment(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"must call", "x:=F!(a)", "x := F!(a)"},
		{"must call statements", "\n\tx:=F!(a)\n\tG!( x )\n", "\n\tx := F!(a)\n\tG!(x)\n"},
		{"macro use", "#util.Assert(a,b)", "#util.Assert(a, b)"},
		{"macro uses", "#util.Assert(a,b)\nx:=#Dbg(1)", "#util.Assert(a, b)\nx := #Dbg(1)"},
		{"macro use then statements", "\t#a()\n\tif x{ #b() }\n", "\t#a()\n\tif x {\n\t\t#b()\n\t}\n"},
		{"must function", "func f!()T{return x}", "func f!() T { return x }"},
		{"macro declaration", "macro M(a lazy int){#body()}", "macro M(a lazy int) { #body() }"},
		{"macro declarations", "macro M(){}\nmacro N(){}", "macro M() {}\nmacro N() {}"},
		{"lines of macros on top", "#a()\nfunc f(){}", "#a()\nfunc f() {}"},
		{"an expression", "F!(a)+#b(c)", "F!(a) + #b(c)"},
		// The word macro starts a macro declaration only with a name after
		// it; as the start of a statement it is a name.
		{"macro as a name: define", "macro:=1", "macro := 1"},
		{"macro as a name: increment", "macro++", "macro++"},
		{"macro as a name: assign", "macro=3", "macro = 3"},
		{"macro as a name: list", "macro,y:=1,2", "macro, y := 1, 2"},
		{"macro as a name: call", "macro(1)", "macro(1)"},
		{"macro as a name: statements", "\n\tmacro:=1\n\tmacro++\n", "\n\tmacro := 1\n\tmacro++\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Fragment("<standard input>", []byte(tc.src))
			if err != nil {
				t.Fatalf("Fragment: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Fragment gives %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPhase10Errors checks that a file with a syntax error in the syntax of
// Phase 10 is not formatted, and the errors are the parser's.
func TestPhase10Errors(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"package p\nvar _ = F![int]\n", "x.gp:2:9: F! must be called: write F!(...)\n"},
		{"package p\nvar _ = #y\n", "x.gp:2:9: #y must be called: write #y(...)\n"},
		{"package p\nvar #x int\n", "x.gp:2:5: cannot use #x here: a name with # can only be the start of a macro call\n"},
		{"package p\nvar f!(int)\n", "x.gp:2:5: cannot use f! here: a name with ! can only be a called function or method, or a func declaration\n"},
		{"package p\nmacro M(lazy string) {}\n", "x.gp:2:9: cannot use lazy as a parameter name: lazy is a modifier, write \"name lazy T\"\n"},
		{"package p\nmacro M()\n", "x.gp:2:7: missing macro body\n"},
		{"package p\n#a()\nvar x int\n", "x.gp:2:1: macro calls must be on top of a func or type declaration\n"},
		{"package p\nfunc g() { fns[i]!() }\n", "x.gp:2:18: expected ';', found '!'\n"},
	} {
		got, err := Source("x.gp", []byte(tc.src))
		if got != nil {
			t.Errorf("%q: Source returned source:\n%s", tc.src, got)
		}
		if text := errorText(err); text != tc.want {
			t.Errorf("%q: errors:\n%s\nwant:\n%s", tc.src, text, tc.want)
		}
	}
}

// TestPhase10Program checks a longer program that is formatted already: a
// program with every form together stays as it is. (The printer's own tests
// do the round trip of the tree; the syntax package's test does it for the
// whole Go standard library through Source.)
func TestPhase10Program(t *testing.T) {
	const src = `package main

import (
	"fmt"
	"os"
	"sync"
)

macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s", #file(), #line(), msg()))
	}
}

macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

macro Once(o *sync.Once) {
	o.Do(func() { #body() })
}

type Store struct {
	mu sync.Mutex
	m  map[string]int
}

#Locked(&s.mu)
func (s *Store) Put(k string, v int) {
	s.m[k] = v
}

#Once(&once)
func setup!() {
	#Assert(len(os.Args) > 0, "no args")
	data := os.ReadFile!("config")
	_ = data
}

func main!() {
	setup!()
	for i := 0; i < F!(3); i++ {
		if !(i != 1) {
			continue
		}
		fmt.Println(i, #text(i))
	}
}
`
	got, err := Source("x.gp", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Errorf("the program is not formatted as written:\n%s", got)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "x.gp", string(got), ParserMode); err != nil {
		t.Errorf("the formatted program does not parse: %v", err)
	}
}
