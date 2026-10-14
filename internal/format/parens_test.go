package format

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// parensFile makes a file of the statements body, one function of a
// package with the types that the statements use.
func parensFile(body string) string {
	return "package p\n\nfunc f() {\n" + body + "}\n"
}

// TestKeepParens checks keepParens: in the header of an if, for or switch
// statement, a composite literal of a generic type keeps its parentheses,
// which go/printer removes (see parens.go). runFormatCases also checks
// that the result formats again without a change, so that it parses.
func TestKeepParens(t *testing.T) {
	runFormatCases(t, []formatCase{{
		name: "range over a gocpp container",
		src:  parensFile("\tfor _, x := range (std.vector[int]{7, 8}) {\n\t\tuse(x)\n\t}\n"),
	}, {
		name: "range without key",
		src:  parensFile("\tfor range (T[int]{1, 2}) {\n\t}\n"),
	}, {
		name: "range with two type arguments",
		src:  parensFile("\tfor k, v := range (pkg.T[int, string]{}) {\n\t\tuse(k, v)\n\t}\n"),
	}, {
		name: "range over a method call on a literal",
		src:  parensFile("\tfor range (T[int]{1}.M()) {\n\t}\n"),
	}, {
		// go/printer keeps the parentheses of (f(T{1})) too, though they
		// aren't needed: the same rule for T[int].
		name: "range over a call with a literal argument",
		src:  parensFile("\tfor range (f(T[int]{1})) {\n\t}\n"),
	}, {
		name: "if",
		src:  parensFile("\tif (T[int]{} == x) {\n\t}\n"),
	}, {
		name: "if with an init statement",
		src:  parensFile("\tif x := (T[int]{}); (x == T[int]{}) {\n\t}\n"),
	}, {
		name: "switch",
		src:  parensFile("\tswitch (std.vector[int]{}) {\n\t}\n"),
	}, {
		name: "switch with an init statement",
		src:  parensFile("\tswitch x := 1; (T[int]{x}) {\n\t}\n"),
	}, {
		name: "for with a condition",
		src:  parensFile("\tfor (T[int]{} != x) {\n\t}\n"),
	}, {
		name: "for with three clauses",
		src:  parensFile("\tfor i := (T[int]{}); (i != T[int]{}); i = (T[int]{}) {\n\t}\n"),
	}, {
		name: "else if",
		src:  parensFile("\tif a {\n\t} else if (T[int]{} == x) {\n\t}\n"),
	}, {
		name: "a type T keeps them, as in gofmt",
		src:  parensFile("\tfor range (T{1}) {\n\t}\n\tif (pkg.T{} == x) {\n\t}\n"),
	}, {
		name: "the rest of the line is formatted",
		src:  parensFile("\tfor _,x:=range ( std.vector[ int ]{7,8} ) {\n\t}\n"),
		want: parensFile("\tfor _, x := range (std.vector[int]{7, 8}) {\n\t}\n"),
	}, {
		name: "double parentheses become one pair",
		src:  parensFile("\tfor range ((T[int]{1})) {\n\t}\n"),
		want: parensFile("\tfor range (T[int]{1}) {\n\t}\n"),
	}, {
		name: "two literals",
		src:  parensFile("\tif (T[int]{} == U[int]{}) {\n\t}\n"),
	}, {
		// The inner header keeps its parentheses. The outer ones go: the
		// literal in them is in parentheses of its own.
		name: "a statement in a function literal in the header",
		src:  parensFile("\tif (func() bool {\n\t\tfor range (T[int]{}) {\n\t\t}\n\t\treturn true\n\t}()) {\n\t}\n"),
		want: parensFile("\tif func() bool {\n\t\tfor range (T[int]{}) {\n\t\t}\n\t\treturn true\n\t}() {\n\t}\n"),
	}, {
		// The literal's type gets the position of the old type, so a
		// comment before it stays before it.
		name: "a comment before the literal",
		src:  parensFile("\tfor range ( /* a */ T[int]{1}) {\n\t}\n"),
	}, {
		name: "a comment just before the literal",
		src:  parensFile("\tfor range (/* a */T[int]{1}) {\n\t}\n"),
		want: parensFile("\tfor range ( /* a */ T[int]{1}) {\n\t}\n"),
	}, {
		name: "a comment just after the type",
		src:  parensFile("\tfor range (T[int]/* b */{1}) {\n\t}\n"),
		want: parensFile("\tfor range (T[int] /* b */ {1}) {\n\t}\n"),
	}, {
		// The second literal gets the type name.
		name: "the first literal has a comment in its type",
		src:  parensFile("\tif (T[ /* c */ int]{} == U[int]{}) {\n\t}\n"),
	}, {
		name: "a type on two lines joins onto one",
		src:  parensFile("\tfor range (T[int,\n\t\tstring]{}) {\n\t}\n"),
		want: parensFile("\tfor range (T[int, string]{}) {\n\t}\n"),
	}, {
		name: "a type with a trailing comma",
		src:  parensFile("\tfor range (T[\n\t\tint,\n\t\tstring,\n\t]{}) {\n\t}\n"),
		want: parensFile("\tfor range (T[int, string]{}) {\n\t}\n"),
	}, {
		name: "an expression on two lines in the type",
		src:  parensFile("\tfor range (std.array[int, (N+\n\t\t1)*2]{}) {\n\t}\n"),
		want: parensFile("\tfor range (std.array[int, (N+1)*2]{}) {\n\t}\n"),
	}, {
		name: "a struct with one field in the type",
		src:  parensFile("\tfor range (T[struct {\n\t\ta int\n\t}]{}) {\n\t}\n"),
		want: parensFile("\tfor range (T[struct{ a int }]{}) {\n\t}\n"),
	}, {
		// S{} keeps the parentheses in go/printer, so keepParens leaves
		// the type of the other literal alone: it would be an error.
		name: "a type T next to a type that can't be on one line",
		src:  parensFile("\tif (S{} == T[struct {\n\t\ta int\n\t\tb int\n\t}]{}) {\n\t}\n"),
	}})
}

// TestKeepParensNotNeeded checks that the parentheses that the parser
// doesn't need still go away, as in gofmt.
func TestKeepParensNotNeeded(t *testing.T) {
	runFormatCases(t, []formatCase{{
		// The "{" after a literal type starts the literal.
		name: "literal types",
		src:  parensFile("\tfor range ([]T[int]{}) {\n\t}\n\tfor range (map[K]V{}) {\n\t}\n\tfor range ([2]T[int]{}) {\n\t}\n\tif (struct{}{} == x) {\n\t}\n"),
		want: parensFile("\tfor range []T[int]{} {\n\t}\n\tfor range map[K]V{} {\n\t}\n\tfor range [2]T[int]{} {\n\t}\n\tif struct{}{} == x {\n\t}\n"),
	}, {
		// The inner literal is in the braces of the outer one.
		name: "a literal in a literal",
		src:  parensFile("\tfor range ([]T[int]{T[int]{1}}) {\n\t}\n"),
		want: parensFile("\tfor range []T[int]{T[int]{1}} {\n\t}\n"),
	}, {
		// The inner parentheses stay; the outer ones go.
		name: "a literal in parentheses of its own",
		src:  parensFile("\tfor range ((T[int]{}).M()) {\n\t}\n"),
		want: parensFile("\tfor range (T[int]{}).M() {\n\t}\n"),
	}, {
		name: "no literal",
		src:  parensFile("\tfor range (xs) {\n\t}\n\tif (a == b[int]) {\n\t}\n\tswitch (x) {\n\t}\n\tfor (i < n) {\n\t}\n"),
		want: parensFile("\tfor range xs {\n\t}\n\tif a == b[int] {\n\t}\n\tswitch x {\n\t}\n\tfor i < n {\n\t}\n"),
	}, {
		// The literal in the inner parentheses is left alone: its type
		// would be an error.
		name: "a type that can't be on one line in parentheses of its own",
		src:  parensFile("\tfor range ((T[struct {\n\t\ta int\n\t\tb int\n\t}]{}).M()) {\n\t}\n"),
		want: parensFile("\tfor range (T[struct {\n\t\ta int\n\t\tb int\n\t}]{}).M() {\n\t}\n"),
	}, {
		name: "no parentheses",
		src:  parensFile("\tfor range f(T[int]{1}) {\n\t}\n\tif (T[int]{}).ok() {\n\t}\n"),
	}})
}

// TestKeepParensFragment checks keepParens in a fragment of statements,
// which gocpp fmt reads from standard input.
func TestKeepParensFragment(t *testing.T) {
	got, err := Fragment("<standard input>", []byte("for range (T[int]{1}) {}"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "for range (T[int]{1}) {\n}"; string(got) != want {
		t.Errorf("Fragment gives %q, want %q", got, want)
	}
}

// TestKeepParensErrors checks the types that keepParens can't put on one
// line (see parens.go). Source returns an error then, not code that
// doesn't parse or that changes when it is formatted again.
func TestKeepParensErrors(t *testing.T) {
	const msg = "cannot keep the parentheses around this composite literal: "
	for _, c := range []struct{ name, body, want string }{{
		name: "a struct with two fields",
		body: "\tfor range (T[struct {\n\t\ta int\n\t\tb int\n\t}]{}) {\n\t}\n",
		want: "x.gp:4:13: " + msg + "its type doesn't fit on one line; give the type a name\n",
	}, {
		name: "an interface with two methods",
		body: "\tif (T[interface {\n\t\tM()\n\t\tN()\n\t}]{} == x) {\n\t}\n",
		want: "x.gp:4:6: " + msg + "its type doesn't fit on one line; give the type a name\n",
	}, {
		name: "a // comment in the type",
		body: "\tfor range (T[ // c\n\t\tint]{1}) {\n\t}\n",
		want: "x.gp:4:13: " + msg + "its type has a comment in it\n",
	}, {
		name: "a /* */ comment in the type",
		body: "\tfor range (T[ /* c */ int]{}) {\n\t}\n",
		want: "x.gp:4:13: " + msg + "its type has a comment in it\n",
	}, {
		name: "a /* */ comment at the end of a line in the type",
		body: "\tfor range (T[int, /* c */\n\t\tstring]{}) {\n\t}\n",
		want: "x.gp:4:13: " + msg + "its type has a comment in it\n",
	}, {
		name: "a /* */ comment on two lines in the type",
		body: "\tfor range (T[ /* a\n\tb */ int]{}) {\n\t}\n",
		want: "x.gp:4:13: " + msg + "its type has a comment in it\n",
	}, {
		name: "a comment in a type argument of a qualified type",
		body: "\tif (pkg.T[int /* c */]{} == x) {\n\t}\n",
		want: "x.gp:4:6: " + msg + "its type has a comment in it\n",
	}, {
		// The error is at the first literal, with its reason.
		name: "no literal of the header can have a type name",
		body: "\tif (T[ /* c */ int]{} == U[struct {\n\t\ta int\n\t\tb int\n\t}]{}) {\n\t}\n",
		want: "x.gp:4:6: " + msg + "its type has a comment in it\n",
	}, {
		name: "two errors",
		body: "\tfor range (T[ // c\n\t\tint]{1}) {\n\t}\n\tswitch (U[ // d\n\t\tint]{}) {\n\t}\n",
		want: "x.gp:4:13: " + msg + "its type has a comment in it\n" +
			"x.gp:7:10: " + msg + "its type has a comment in it\n",
	}} {
		t.Run(c.name, func(t *testing.T) {
			got, err := Source("x.gp", []byte(parensFile(c.body)))
			if got != nil {
				t.Errorf("Source returned source:\n%s\nwant none", got)
			}
			if text := errorText(err); text != c.want {
				t.Errorf("errors:\n%s\nwant:\n%s", text, c.want)
			}
		})
	}
}

// TestKeepParensGofmt checks that the real gofmt still has the bug that
// keepParens works around: for Go generics, it removes the parentheses,
// and its result doesn't parse. If a new Go fixes it, keepParens is no
// longer needed, and the package doc must lose its third rule.
func TestKeepParensGofmt(t *testing.T) {
	gofmt := realGofmt(t)
	if gofmt == "" {
		t.Skip("there is no real gofmt in $GOROOT/bin")
	}
	const src = "package p\n\ntype T[P any] []P\n\nfunc f() {\n\tfor range (T[int]{1, 2}) {\n\t}\n}\n"
	name := filepath.Join(t.TempDir(), "x.go")
	if err := os.WriteFile(name, []byte(src), 0o666); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runGofmt(t, gofmt, name)
	if code != 0 {
		t.Fatalf("gofmt %s: exit status %d\n%s", name, code, stderr)
	}
	if !strings.Contains(string(out), "for range T[int]{1, 2} {") {
		t.Fatalf("gofmt keeps the parentheses now:\n%s", out)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "x.go", out, 0); err == nil {
		t.Errorf("gofmt's result parses now:\n%s", out)
	}
	got, err := Source("x.go", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Errorf("Source gives\n%s\nwant\n%s", got, src)
	}
}

// TestKeepParensCorpus formats the corpus file of job 1.15 that showed the
// bug: the result keeps the parentheses and parses.
func TestKeepParensCorpus(t *testing.T) {
	name := filepath.Join("..", "check", "testdata", "corpus", "containers.gp")
	src := readFile(t, name)
	got, err := Source(name, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "for _, x := range (std.vector[int]{7, 8}) {") {
		t.Errorf("Source lost the parentheses in %s:\n%s", name, got)
	}
	again, err := Source(name, got)
	if err != nil {
		t.Fatalf("the formatted file doesn't format again: %v", err)
	}
	if string(again) != string(got) {
		t.Errorf("formatting %s again changes it:\n%s", name, showDiff(got, again))
	}
}
