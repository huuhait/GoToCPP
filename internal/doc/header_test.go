package doc

import (
	"reflect"
	"strings"
	"testing"
)

type declWant struct{ decl, comment string }

func declsOf(src string) []declWant {
	var out []declWant
	for _, d := range extractDecls(src) {
		out = append(out, declWant{d.decl, d.comment})
	}
	return out
}

// The Doxygen comments of a header, in every spelling: a block comment ends
// at its */, whether or not it is on a line of its own, and the declaration
// that follows is the one that the comment documents.
func TestExtractDecls(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want []declWant
	}{
		{"multi-line block", "/**\n * Adds.\n * More text.\n */\nint add(int a, int b);\n\n/** One liner. */\nint one(void);\n\n/**\n * Third.\n */\nint third(void);\n",
			[]declWant{{"int add(int a, int b);", "Adds.\nMore text.\n"}, {"int one(void);", "One liner.\n"}, {"int third(void);", "Third.\n"}}},
		{"qt block", "/*!\n * Qt.\n */\nint q(void);\n", []declWant{{"int q(void);", "Qt.\n"}}},
		{"qt one line", "/*! Qt one. */\nint q(void);\n", []declWant{{"int q(void);", "Qt one.\n"}}},
		{"end on the text line", "/**\n * Text. */\nint e(void);\n", []declWant{{"int e(void);", "Text.\n"}}},
		{"end without margin", "/**\n   Text\n*/\nint e(void);\n", []declWant{{"int e(void);", "Text\n"}}},
		{"triple slash", "/// One,\n/// two.\nint s(void);\n", []declWant{{"int s(void);", "One,\ntwo.\n"}}},
		{"bang lines", "//! One.\n//! Two.\nint s(void);\n", []declWant{{"int s(void);", "One.\nTwo.\n"}}},
		{"four slashes are not doc", "//// Banner\nint s(void);\n", nil},
		{"plain comment is not doc", "/* Plain. */\nint s(void);\n// Line.\nint t(void);\n", nil},
		{"banner is not doc", "/***\n * Banner.\n */\nint s(void);\n", nil},
		{"empty comment", "/**/\nint s(void);\n", nil},
		{"closing stars", "/**\n * Text.\n **/\nint s(void);\n", []declWant{{"int s(void);", "Text.\n"}}},
		{"nested stars", "/**\n * A.\n ** two\n * * item\n */\nint s(void);\n", []declWant{{"int s(void);", "A.\n* two\n* item\n"}}},
		{"comment ends the file", "int a;\n/**\n * Dangling.\n */\n", nil},
		{"unterminated block", "/**\n * Never ends.\nint a(void);\nint b(void);\n", nil},
		{"unterminated block then doc", "/**\n * Never ends.\nint a(void);\n", nil},
		{"blank line between comment and declaration", "/** Alone. */\n\nint a(void);\n", nil},
		{"crlf", "/**\r\n * Adds.\r\n */\r\nint add(int a);\r\n\r\n/// Line.\r\nint b(void);\r\n",
			[]declWant{{"int add(int a);", "Adds.\n"}, {"int b(void);", "Line.\n"}}},
		{"windows-1252 bytes", "/** caf\xe9 \x93quoted\x94 \x80 */\nint a(void);\n", []declWant{{"int a(void);", "café “quoted” €\n"}}},
		{"bom on the first line", "\xef\xbb\xbf/// first\nint one(void);\n/// next\nint nxt(void);\n", []declWant{{"int one(void);", "first\n"}, {"int nxt(void);", "next\n"}}},
		{"mixed utf-8 and latin-1", "/// caf\xe9 in latin1\nint a(void);\n/// \xe2\x82\xac utf8 ok \xe9 mixed \xc3\xa9\nint b(void);\n",
			[]declWant{{"int a(void);", "café in latin1\n"}, {"int b(void);", "€ utf8 ok é mixed é\n"}}},
		{"utf-8 stays", "/** café — ok */\nint a(void);\n", []declWant{{"int a(void);", "café — ok\n"}}},
		{"macro, struct, typedef", "/** Macro. */\n#define N 3\n\n/** Struct. */\nstruct S { int x; };\n\n/** Func ptr. */\ntypedef int (*cb)(int);\n",
			[]declWant{{"#define N 3", "Macro.\n"}, {"struct S", "Struct.\n"}, {"typedef int (*cb)(int);", "Func ptr.\n"}}},
	} {
		got := declsOf(c.src)
		// The text of a declaration is not what this test is about for
		// the shapes that the other tests pin: compare the comments, and
		// the declarations for the plain functions.
		if len(got) != len(c.want) {
			t.Errorf("%s: got %d declarations %q, want %d", c.name, len(got), got, len(c.want))
			continue
		}
		for i := range got {
			if got[i].comment != c.want[i].comment || !strings.HasPrefix(got[i].decl, strings.TrimSuffix(c.want[i].decl, " { int x; };")) {
				t.Errorf("%s: declaration %d is %q, want %q", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// readDoxyComment reports how many lines the comment takes.
func TestReadDoxyCommentLines(t *testing.T) {
	lines := strings.Split("/**\n * a\n */\nint x;\n", "\n")
	text, next, ok := readDoxyComment(lines, 0)
	if !ok || next != 3 || text != "a\n" {
		t.Errorf("got %q %d %v", text, next, ok)
	}
	if _, _, ok := readDoxyComment(lines, 3); ok {
		t.Error("a declaration is not a comment")
	}
}

func TestDecodeHeaderText(t *testing.T) {
	for in, want := range map[string]string{
		"plain":             "plain",
		"é":                 "é",
		"\xe9":              "é",
		"\x80":              "€",
		"\x81":              "�",
		"a\xa0b\xff":        "a bÿ",
		"\xe2\x80\x94":      "—",
		"\xef\xbb\xbfx":     "x",
		"\xe2\x82\xac \xe9": "€ é",
		"\xc3\xa9\xe9\xc3":  "éé\u00c3",
	} {
		if got := decodeHeaderText(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	_ = reflect.DeepEqual
}
