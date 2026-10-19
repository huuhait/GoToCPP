package bindgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// dump is a small clang-style dump: pretty printed, files named where they
// change, includedFrom inside a location, empty locations at depth 3.
const dump = `{
  "id": "0x1",
  "kind": "TranslationUnitDecl",
  "loc": {},
  "range": {
    "begin": {},
    "end": {}
  },
  "inner": [
    {
      "id": "0x2",
      "kind": "TypedefDecl",
      "loc": {
        "offset": 1,
        "file": "/a/x.h",
        "line": 1,
        "col": 1,
        "tokLen": 1,
        "includedFrom": {
          "file": "/a/main.cpp"
        }
      },
      "name": "T { \"file\": \"/evil\" } [",
      "type": {"qualType": "int"}
    },
    {
      "id": "0x3",
      "kind": "FunctionDecl",
      "loc": {
        "offset": 5,
        "line": 2,
        "col": 1,
        "tokLen": 1
      },
      "name": "f\\"
    },
    {
      "id": "0x4",
      "kind": "VarDecl",
      "loc": {
        "offset": 9,
        "file": "/a/y.h",
        "line": 1,
        "col": 1,
        "tokLen": 1,
        "includedFrom": {
          "file": "/a/x.h"
        }
      },
      "range": {
        "begin": {
          "offset": 9,
          "col": 1
        },
        "end": {}
      },
      "inner": [
        {
          "id": "0x5",
          "kind": "IntegerLiteral",
          "range": {
            "begin": {
              "spellingLoc": {"offset": 3, "file": "/a/z.h", "line": 7},
              "expansionLoc": {"offset": 4, "file": "/a/y.h", "line": 2}
            }
          }
        }
      ]
    },
    {
      "id": "0x6",
      "kind": "EnumDecl",
      "loc": {"line": 3}
    }
  ]
}
`

type seen struct{ kind, file string }

func scanAll(t *testing.T, r io.Reader) []seen {
	t.Helper()
	var got []seen
	err := scanTU(r, func(d *rawDecl) error {
		if !json.Valid(d.Data) {
			t.Errorf("%s: the text of the declaration is not JSON: %q", d.Kind, d.Data)
		}
		got = append(got, seen{d.Kind, d.File})
		return nil
	})
	if err != nil {
		t.Fatalf("scanTU: %v", err)
	}
	return got
}

func TestScanTUFiles(t *testing.T) {
	want := []seen{
		{"TypedefDecl", ""},
		{"FunctionDecl", "/a/x.h"}, // includedFrom did not change the file; the typedef set /a/x.h
		{"VarDecl", "/a/x.h"},
		{"EnumDecl", "/a/y.h"}, // the macro locations of the literal ended in /a/y.h
	}
	for name, r := range map[string]io.Reader{
		"whole":   strings.NewReader(dump),
		"1 byte":  iotest.OneByteReader(strings.NewReader(dump)),
		"3 bytes": &chunkReader{s: dump, n: 3},
		"half":    iotest.HalfReader(strings.NewReader(dump)),
	} {
		got := scanAll(t, r)
		if len(got) != len(want) {
			t.Fatalf("%s: %d declarations, want %d: %v", name, len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: declaration %d = %v, want %v", name, i, got[i], want[i])
			}
		}
	}
}

// chunkReader reads n bytes at a time.
type chunkReader struct {
	s string
	n int
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if c.s == "" {
		return 0, io.EOF
	}
	k := min(c.n, len(p), len(c.s))
	copy(p, c.s[:k])
	c.s = c.s[k:]
	return k, nil
}

func TestScanTUErrors(t *testing.T) {
	for name, in := range map[string]string{
		"empty":    "",
		"text":     "hello, world\n",
		"no inner": `{"kind": "TranslationUnitDecl"}`,
	} {
		err := scanTU(strings.NewReader(in), func(*rawDecl) error {
			t.Errorf("%s: visited a declaration", name)
			return nil
		})
		if name == "no inner" {
			if err != nil {
				t.Errorf("a dump with no declarations: %v", err)
			}
		} else if !errors.Is(err, errNotDump) {
			t.Errorf("%s: err = %v, want errNotDump", name, err)
		}
	}
	// a dump that is cut off
	err := scanTU(strings.NewReader(dump[:len(dump)-30]), func(*rawDecl) error { return nil })
	if !errors.Is(err, errNotDump) {
		t.Errorf("cut dump: err = %v", err)
	}
	// the visitor stops the scan
	stop := errors.New("stop")
	n := 0
	err = scanTU(strings.NewReader(dump), func(*rawDecl) error { n++; return stop })
	if err != stop || n != 1 {
		t.Errorf("stop: err = %v after %d declarations", err, n)
	}
	// a read error is reported
	err = scanTU(io.MultiReader(strings.NewReader(dump[:100]), iotest.ErrReader(errors.New("boom"))), func(*rawDecl) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("read error: err = %v", err)
	}
}

func TestScanTUBigDeclaration(t *testing.T) {
	old := maxDeclBytes
	maxDeclBytes = 500
	defer func() { maxDeclBytes = old }()
	var kinds []string
	var skipped []bool
	err := scanTU(strings.NewReader(strings.Replace(dump, `"name": "f\\"`, `"name": "`+strings.Repeat("x", 600)+`"`, 1)), func(d *rawDecl) error {
		kinds = append(kinds, d.Kind)
		skipped = append(skipped, d.Skipped)
		if d.Skipped && d.Data != nil {
			t.Error("a skipped declaration has text")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != 4 || kinds[1] != "FunctionDecl" || !skipped[1] || skipped[0] || skipped[3] {
		t.Errorf("kinds %v skipped %v", kinds, skipped)
	}
}

func TestKindOf(t *testing.T) {
	for in, want := range map[string]string{
		"{\n  \"id\": \"0x1\",\n  \"kind\": \"FunctionDecl\",\n": "FunctionDecl",
		`{"kind":"X"}`:   "X",
		`{"id": "kind"}`: "",
		`{"kind": 3}`:    "",
		``:               "",
	} {
		if got := kindOf([]byte(in)); got != want {
			t.Errorf("kindOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFileTracker(t *testing.T) {
	var tr fileTracker
	if f := tr.loc(&jloc{File: "a.h"}); f != "a.h" {
		t.Errorf("file = %q", f)
	}
	if f := tr.loc(&jloc{Line: 3}); f != "a.h" {
		t.Errorf("a location without a file keeps the file: %q", f)
	}
	if f := tr.loc(nil); f != "a.h" {
		t.Errorf("nil: %q", f)
	}
	// spelling first, then expansion: the expansion's file is the one that stays
	f := tr.loc(&jloc{SpellingLoc: &jloc{File: "m.h"}, ExpansionLoc: &jloc{File: "b.h"}})
	if f != "b.h" || tr.file != "b.h" {
		t.Errorf("macro location: %q", f)
	}
	f = tr.loc(&jloc{SpellingLoc: &jloc{File: "m.h"}, ExpansionLoc: &jloc{}})
	if f != "m.h" {
		t.Errorf("expansion in the spelling's file: %q", f)
	}
}

func TestQuickLoc(t *testing.T) {
	n := []byte(`{"id":"0x1","kind":"FunctionDecl","loc":{"offset":1,"file":"a.h","includedFrom":{"file":"m.cpp"}},"range":{"begin":{"file":"zzz"}}}`)
	l := quickLoc(n)
	if l == nil || l.File != "a.h" {
		t.Errorf("quickLoc = %+v", l)
	}
	if quickLoc([]byte(`{"kind":"X"}`)) != nil {
		t.Error("a node without a loc has one")
	}
}

func FuzzScanTU(f *testing.F) {
	f.Add(dump)
	f.Add(`{"inner":[{}]}`)
	f.Add(`{"inner":[{"a":"\"}"}]}`)
	f.Add("{")
	f.Fuzz(func(t *testing.T, in string) {
		var n int
		err := scanTU(bytes.NewReader([]byte(in)), func(d *rawDecl) error {
			n++
			if !d.Skipped && len(d.Data) == 0 {
				t.Error("empty declaration")
			}
			return nil
		})
		// whatever the text is, a well-formed document scans the way encoding/json reads it
		var doc struct {
			Inner []json.RawMessage `json:"inner"`
		}
		if err == nil && json.Unmarshal([]byte(in), &doc) == nil && n != len(doc.Inner) {
			t.Errorf("scanned %d declarations, encoding/json finds %d", n, len(doc.Inner))
		}
	})
}

// Objects at the depth of a declaration that are not in the "inner" array of the root
// (the begin and end of "range" are there in every dump) are not declarations; and a
// string that has the text of a key inside it, with escaped quotes, is a string.
func TestScanTUHostile(t *testing.T) {
	in := `{
  "id": "0x1",
  "kind": "TranslationUnitDecl",
  "other": [ {"kind": "NotADecl"}, [ {"kind": "NorThis"} ] ],
  "range": {"begin": {"kind": "NoNo"}, "end": {}},
  "inner": [
    {
      "id": "0x2",
      "kind": "VarDecl",
      "name": "x\"}, \"file\": \"/evil.h\", \"y\": {\"z\": \"",
      "loc": {"offset": 1, "line": 1}
    },
    {
      "id": "0x3",
      "kind": "VarDecl",
      "name": "back\\",
      "loc": {"file": "/good.h", "line": 2}
    },
    {
      "id": "0x4",
      "kind": "VarDecl"
    }
  ]
}`
	got := scanAll(t, strings.NewReader(in))
	want := []seen{{"VarDecl", ""}, {"VarDecl", ""}, {"VarDecl", "/good.h"}}
	if len(got) != len(want) {
		t.Fatalf("%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("declaration %d = %v, want %v", i, got[i], want[i])
		}
	}
	for name, r := range map[string]io.Reader{"1 byte": iotest.OneByteReader(strings.NewReader(in)), "7 bytes": &chunkReader{s: in, n: 7}} {
		if got := scanAll(t, r); len(got) != len(want) || got[2] != want[2] {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestUnescapeJSONString(t *testing.T) {
	for in, want := range map[string]string{
		`plain`:        "plain",
		`a\tb`:         "a\tb",
		`a\nb\\c\"d`:   "a\nb\\c\"d",
		`C:\\dir\\x.h`: `C:\dir\x.h`,
		`end\`:         `end\`,
	} {
		if got := unescapeJSONString(in); got != want {
			t.Errorf("unescapeJSONString(%q) = %q, want %q", in, got, want)
		}
	}
}
