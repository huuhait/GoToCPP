package bindgen

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// TestScanMatchesJSON reads the real dump of a few headers twice, once with
// scanTU and once as a whole with encoding/json, and checks that they agree
// on every top-level declaration: its kind, its text, and the file that clang
// had printed last when the declaration began (replayed by fileTracker over
// the whole dump).
func TestScanMatchesJSON(t *testing.T) {
	cxx := tsupport.Clang(t)
	for _, name := range []string{"c_basic.h", "c_macros.h", "umbrella/umbrella.h", "flags.h"} {
		t.Run(name, func(t *testing.T) {
			tl := &tool{cxx: cxx, dir: headersDir(t), tmp: t.TempDir(), flags: []string{"-I", "inc"}}
			src, err := tl.source("main.cpp", "#include \""+filepath.Join(headersDir(t), name)+"\"\n")
			if err != nil {
				t.Fatal(err)
			}
			var dump []byte
			_, exit, err := tl.run(context.Background(), []string{"-fsyntax-only", "-Xclang", "-ast-dump=json"}, src, func(r io.Reader) error {
				var err error
				dump, err = io.ReadAll(r)
				return err
			})
			if err != nil || exit != 0 {
				t.Fatalf("clang: exit %d, %v", exit, err)
			}
			var whole struct {
				Inner []json.RawMessage `json:"inner"`
			}
			if err := json.Unmarshal(dump, &whole); err != nil {
				t.Fatal(err)
			}
			var tr fileTracker
			i := 0
			err = scanTU(bytes.NewReader(dump), func(d *rawDecl) error {
				if i >= len(whole.Inner) {
					t.Fatalf("scanTU finds more than %d declarations", len(whole.Inner))
				}
				n, err := decodeNode(whole.Inner[i])
				if err != nil {
					t.Fatal(err)
				}
				if d.Kind != n.Kind {
					t.Errorf("declaration %d: kind %q, want %q", i, d.Kind, n.Kind)
				}
				if d.File != tr.file {
					t.Errorf("declaration %d (%s %s): file %q, want %q", i, n.Kind, n.Name, d.File, tr.file)
				}
				var a, b any
				if err := json.Unmarshal(d.Data, &a); err != nil {
					t.Fatalf("declaration %d: %v", i, err)
				}
				json.Unmarshal(whole.Inner[i], &b)
				ja, _ := json.Marshal(a)
				jb, _ := json.Marshal(b)
				if !bytes.Equal(ja, jb) {
					t.Errorf("declaration %d: the text differs", i)
				}
				tr.walk(n, nil)
				i++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if i != len(whole.Inner) || i < 5 {
				t.Errorf("%d declarations, want %d (the built-in ones are always there)", i, len(whole.Inner))
			}
		})
	}
}

// BenchmarkScanTU reads the JSON dump in the file named by GOCPP_BINDGEN_DUMP
// (made with clang++ -x c++ -fsyntax-only -Xclang -ast-dump=json header).
func BenchmarkScanTU(b *testing.B) {
	path := os.Getenv("GOCPP_BINDGEN_DUMP")
	if path == "" {
		b.Skip("GOCPP_BINDGEN_DUMP names no dump")
	}
	st, err := os.Stat(path)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(st.Size())
	for i := 0; i < b.N; i++ {
		f, err := os.Open(path)
		if err != nil {
			b.Fatal(err)
		}
		n := 0
		if err := scanTU(f, func(*rawDecl) error { n++; return nil }); err != nil {
			b.Fatal(err)
		}
		f.Close()
	}
}
