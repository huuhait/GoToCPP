package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// The tolerance of the JSON reader (Phase 12 step 12.12, risk row "clang's JSON format changes
// between versions: a tolerant reader"). Only clang 21 is in the dev shell, so these tests do
// not compare versions; they take the real dump of this clang and change it in the ways in which
// the dump of another version can differ without changing what it says: fields the reader does
// not use appear, disappear or move, and the keys of an object come in another order. The
// decoded declarations must be the same as from the original dump. (tools/clang-matrix.sh runs
// the tests against every clang++ that is installed, which is the real comparison.)
//
// The changes are made to the bytes of each object after ScanDecls has cut the stream, so the
// decoders are what these tests cover; TestScannerFindsNamesWithReorderedKeys runs the scanner
// itself on a dump with reversed keys.

// An ordered JSON object, to keep the order of the keys (which Go's maps lose) and to change it.
type orderedObject struct {
	keys   []string
	values map[string]any
}

// parseOrdered reads a JSON value into orderedObject, []any, json.Number, string, bool or nil.
func parseOrdered(t testing.TB, dec *json.Decoder) any {
	t.Helper()
	tok, err := dec.Token()
	if err != nil {
		t.Fatal(err)
	}
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			o := &orderedObject{values: map[string]any{}}
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					t.Fatal(err)
				}
				key := k.(string)
				o.keys = append(o.keys, key)
				o.values[key] = parseOrdered(t, dec)
			}
			if _, err := dec.Token(); err != nil {
				t.Fatal(err)
			}
			return o
		case '[':
			list := []any{}
			for dec.More() {
				list = append(list, parseOrdered(t, dec))
			}
			if _, err := dec.Token(); err != nil {
				t.Fatal(err)
			}
			return list
		}
	}
	return tok
}

func writeOrdered(t testing.TB, buf *bytes.Buffer, v any) {
	t.Helper()
	switch v := v.(type) {
	case *orderedObject:
		buf.WriteByte('{')
		for i, k := range v.keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteByte(':')
			writeOrdered(t, buf, v.values[k])
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeOrdered(t, buf, e)
		}
		buf.WriteByte(']')
	default:
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(b)
	}
}

// A dump change is a change of every object of the tree.
type dumpChange struct {
	name  string
	apply func(o *orderedObject)
}

// ignored are the fields of clang's dump that the reader does not use.
var ignored = []string{
	"range", "loc", "mangledName", "previousDecl", "parentDeclContextId",
	"isUsed", "isReferenced", "inline",
}

var dumpChanges = []dumpChange{
	{"unknown fields first", func(o *orderedObject) {
		o.keys = slices.Insert(o.keys, 0, "gocppFutureField")
		nested := &orderedObject{keys: []string{"y"}, values: map[string]any{"y": nil}}
		o.values["gocppFutureField"] = []any{json.Number("1"), "x", nested}
	}},
	{"unknown fields last", func(o *orderedObject) {
		o.keys = append(o.keys, "zzzFutureField")
		o.values["zzzFutureField"] = "later"
	}},
	{"keys reversed", func(o *orderedObject) { slices.Reverse(o.keys) }},
	{"keys sorted", func(o *orderedObject) { slices.Sort(o.keys) }},
	{"unused fields dropped", func(o *orderedObject) {
		o.keys = slices.DeleteFunc(o.keys, func(k string) bool {
			if slices.Contains(ignored, k) {
				delete(o.values, k)
				return true
			}
			return false
		})
	}},
}

// changeDump applies the change to every object of the JSON value in raw.
func changeDump(t testing.TB, raw []byte, change dumpChange) []byte {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	tree := parseOrdered(t, dec)
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case *orderedObject:
			for _, k := range slices.Clone(v.keys) {
				walk(v.values[k])
			}
			change.apply(v)
		case []any:
			for _, e := range v {
				walk(e)
			}
		}
	}
	walk(tree)
	var buf bytes.Buffer
	writeOrdered(t, &buf, tree)
	return buf.Bytes()
}

// dumpOf compiles the source with clang and returns the top-level objects of the filtered dump.
func dumpOf(t testing.TB, cfg Config, text, filter string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	_, err := runDump(context.Background(), cfg, text, filter, func(rd RawDecl) error {
		if !rd.TooLarge {
			out[fmt.Sprintf("%03d:%s", len(out), rd.Name)] = bytes.Clone(rd.Raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestReaderIgnoresWhatItDoesNotKnow(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "tol.hpp", `#pragma once
#include <cstdio>
#include <cstring>
#include <string>
#include <vector>
#define TOL_STR "My" "App"
#define TOL_FLT (3.14159265358979 / 2)
#define TOL_INT (1 << 3)
struct Rec {
  int a;
  static const int k = 3;
  long get() const { return a; }
  Rec &self() { return *this; }
};
enum Color { Red, Green = 5 };
std::vector<long> make_vec();
Rec make_rec();
`)
	cfg := clangConfig(t, dir)
	kinds := map[string]interop.Kind{
		"BUFSIZ": interop.Macro, "TOL_STR": interop.Macro,
		"TOL_FLT": interop.Macro, "TOL_INT": interop.Macro,
		"strlen": interop.Value, "make_vec": interop.Value, "make_rec": interop.Value,
	}
	refs := []*collect.Ref{
		ref(pth("BUFSIZ")), ref(pth("TOL_STR")), ref(pth("TOL_FLT")), ref(pth("TOL_INT")),
		ref(call(pth("strlen"), lit(`"abc"`))),
		ref(call(pth("make_vec"))),
		ref(call(pth("make_rec"))),
		ref(call(pth("strlen"), lv(str()))), // an error: a std::string for a const char *
	}
	for _, r := range refs[:4] {
		r.WantValue = true
	}
	gc := GenConfig{Includes: []string{`"tol.hpp"`}, Kinds: kinds}
	src := Generate(collect.NewSet(refs...), gc)
	raws := dumpOf(t, cfg, src.Text, probeDeclPrefix)
	if len(raws) < len(refs) {
		t.Fatalf("the dump has %d declarations for %d references", len(raws), len(refs))
	}
	for _, change := range dumpChanges {
		t.Run(change.name, func(t *testing.T) {
			for name, raw := range raws {
				want, err := decodeDecl(raw)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				changed := changeDump(t, raw, change)
				got, err := decodeDecl(changed)
				if err != nil {
					t.Fatalf("%s after the change: %v", name, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s: the reader's answer changed with the dump:\nbefore %+v\nafter  %+v",
						name, want, got)
				}
			}
		})
	}
	// The answers are something: a value, a type, a string (the test does not compare nothing).
	var values, types, strs int
	for _, raw := range raws {
		f, _ := decodeDecl(raw)
		switch {
		case f.value != nil:
			values++
		case f.str != nil:
			strs++
		case f.typ != "":
			types++
		}
	}
	if values < 3 || strs < 1 || types < 3 {
		t.Errorf("the dump gave %d values, %d strings and %d types: too little to compare",
			values, strs, types)
	}
}

func TestReaderOfRecordsIgnoresWhatItDoesNotKnow(t *testing.T) {
	dir := t.TempDir()
	text := `#include <string>
struct gpRec : std::string {
  int a;
  static const int k = 3;
  long get() const { return a; }
  gpRec &self() { return *this; }
  template <class T> T conv() const { return T(); }
 private:
  int hidden;
};
enum gpColor { GpRed, GpGreen = 5 };
enum class gpMode : short { Fast = 1, Slow };
`
	cfg := clangConfig(t, dir)
	raws := dumpOf(t, cfg, text, "gp")
	var records, enums int
	for name, raw := range raws {
		var n dnode
		if err := json.Unmarshal(raw, &n); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(n.Name, "gp") || n.Kind == "" {
			continue
		}
		decode := func(raw []byte) any {
			var n dnode
			if err := json.Unmarshal(raw, &n); err != nil {
				t.Fatal(err)
			}
			switch n.Kind {
			case "CXXRecordDecl":
				r, err := decodeRecord(&n, n.Name)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				return r
			case "EnumDecl":
				e, err := decodeEnum(&n, n.Name, "int")
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				return e
			}
			return nil
		}
		want := decode(raw)
		switch want.(type) {
		case *Record:
			records++
		case *Enum:
			enums++
		default:
			continue
		}
		for _, change := range dumpChanges {
			if got := decode(changeDump(t, raw, change)); !reflect.DeepEqual(got, want) {
				t.Errorf("%s with %q: the decoded declaration changed\nbefore %+v\nafter  %+v",
					name, change.name, want, got)
			}
		}
	}
	if records < 1 || enums < 2 {
		t.Errorf("the dump gave %d records and %d enums: too little to compare", records, enums)
	}
}

// The streaming scanner finds the objects and their names whatever order the keys of an object are
// in (a version may put "name" after "inner"): the names of a dump with reversed keys are those of
// the dump as clang wrote it.
func TestScannerFindsNamesWithReorderedKeys(t *testing.T) {
	cfg := clangConfig(t, t.TempDir())
	src := Generate(collect.NewSet(ref(pth("BUFSIZ")), ref(call(pth("strlen"), lit(`"abc"`)))),
		GenConfig{Includes: []string{"<cstdio>", "<cstring>"}, Kinds: stdKinds})
	raws := dumpOf(t, cfg, src.Text, probeDeclPrefix)
	names := func(change *dumpChange) []string {
		var stream bytes.Buffer
		for i := 0; i < len(raws); i++ {
			for key, raw := range raws {
				if !strings.HasPrefix(key, fmt.Sprintf("%03d:", i)) {
					continue
				}
				if change != nil {
					raw = changeDump(t, raw, *change)
				}
				stream.Write(raw)
				stream.WriteByte('\n')
			}
		}
		var got []string
		_, err := ScanDecls(&stream, 0, func(rd RawDecl) error {
			got = append(got, rd.Name)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	reversed := dumpChange{"keys reversed", func(o *orderedObject) { slices.Reverse(o.keys) }}
	want, got := names(nil), names(&reversed)
	if len(want) < 2 || !slices.Equal(want, got) {
		t.Errorf("names with the keys as clang wrote them: %v; with the keys reversed: %v",
			want, got)
	}
}

// A complete dump is read to its end (white space after the last object is fine), a dump cut in the
// middle of an object and text that is not JSON are errors, and a dump cut between two objects is
// read up to the cut (clang killed by a timeout leaves that).
func TestReaderOfATruncatedDump(t *testing.T) {
	cfg := clangConfig(t, t.TempDir())
	gc := GenConfig{Includes: []string{"<cstdio>"}, Kinds: stdKinds}
	src := Generate(collect.NewSet(ref(pth("BUFSIZ")), ref(pth("EOF"))), gc)
	raws := dumpOf(t, cfg, src.Text, probeDeclPrefix)
	if len(raws) < 2 {
		t.Fatalf("the dump has %d objects, want at least 2", len(raws))
	}
	var all, first bytes.Buffer
	for i := 0; i < len(raws); i++ {
		for key, raw := range raws {
			if strings.HasPrefix(key, fmt.Sprintf("%03d:", i)) {
				all.Write(raw)
				all.WriteByte('\n')
				if i == 0 {
					first.Write(raw)
				}
			}
		}
	}
	count := func(r io.Reader) (int, error) {
		return ScanDecls(r, 0, func(RawDecl) error { return nil })
	}
	spaced := io.MultiReader(bytes.NewReader(all.Bytes()), strings.NewReader("\n  "))
	if n, err := count(spaced); err != nil || n != len(raws) {
		t.Errorf("a complete dump with trailing space: %d objects (want %d), error %v",
			n, len(raws), err)
	}
	if n, err := count(bytes.NewReader(first.Bytes())); err != nil || n != 1 {
		t.Errorf("a dump cut between two objects: %d objects (want 1), error %v", n, err)
	}
	if _, err := count(bytes.NewReader(all.Bytes()[:all.Len()-5])); err == nil {
		t.Errorf("a dump cut in the middle of an object was read without an error")
	}
	if _, err := count(strings.NewReader("hello, this is not JSON")); !errors.Is(err, ErrBadDump) {
		t.Errorf("text that is not JSON: error %v, want ErrBadDump", err)
	}
}
