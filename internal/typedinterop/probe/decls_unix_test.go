//go:build unix

package probe

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A compiler that fails in the middle of a run: the names of that run have the error;
// nothing hangs or panics.
func TestDeclsFakeCompilers(t *testing.T) {
	dc := DeclConfig{Includes: []string{"<cstdio>"}}
	for name, c := range map[string]struct{ script, want string }{
		"crash":   {`kill -SEGV $$`, "signal"},
		"garbage": {`echo hello`, "AST dump"},
		"fails":   {`echo "clang: error: broken" >&2; exit 2`, "broken"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Config{CXX: fakeCXX(t, c.script), Dir: t.TempDir()}
			tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "Foo"}, {DeclFunc, "bar"}})
			if err != nil {
				t.Fatal(err)
			}
			for _, k := range []string{"type Foo", "func bar"} {
				if !strings.Contains(tab.Errors[k], c.want) {
					t.Errorf("%s: %q", k, tab.Errors[k])
				}
			}
		})
	}
	// A compiler that hangs is killed at the timeout.
	cfg := Config{CXX: fakeCXX(t, `sleep 300`), Dir: t.TempDir(), Timeout: 200 * time.Millisecond}
	start := time.Now()
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "Foo"}})
	if err != nil || !strings.Contains(tab.Errors["type Foo"], "timeout") || time.Since(start) > 30*time.Second {
		t.Errorf("hang: %v %v after %v", err, tab.Errors, time.Since(start))
	}
}

// A template instance that is also instantiated explicitly (extern template) is dumped twice, once
// as a stub: the definition is the one that is used. Non-type arguments are part of the name.
func TestDeclsTemplateInstances(t *testing.T) {
	cfg, dc := declsSetup(t)
	tab, err := new(Runner).Decls(context.Background(), cfg, dc, []DeclQuery{{DeclType, "Wrap<int>"}, {DeclType, "Wrap<long>"}, {DeclType, "Buf<char, 8>"}, {DeclType, "Buf<int, 3>"}})
	if err != nil {
		t.Fatal(err)
	}
	w := tab.Records["Wrap<int>"]
	if w == nil || !w.Complete || !w.Template || fieldList(w) != "v:int" || w.Size != 4 {
		t.Fatalf("Wrap<int>: %+v (%v)", w, tab.Errors)
	}
	if m := w.MethodsNamed("get"); len(m) != 1 || m[0].Sig.T.Elem.String() != "int" {
		t.Errorf("Wrap<int>::get: %+v", m)
	}
	if l := tab.Records["Wrap<long>"]; l == nil || fieldList(l) != "v:long" || l.Size != 8 || l.Methods[0].Sig.T == nil || l.Methods[0].Sig.T.Elem.String() != "long" {
		t.Errorf("Wrap<long>: %+v", l)
	}
	b := tab.Records["Buf<char, 8>"]
	if b == nil || fieldList(b) != "data:char[8]" || b.Size != 8 {
		t.Errorf("Buf<char, 8>: %+v (%v)", b, tab.Errors)
	}
	if b3 := tab.Records["Buf<int, 3>"]; b3 == nil || fieldList(b3) != "data:int[3]" || b3.Size != 12 {
		t.Errorf("Buf<int, 3>: %+v", b3)
	}
}

// DeclConfig.Parallel bounds the compiler runs at the same time.
func TestDeclsParallelBound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FAKE_DIR", dir)
	cxx := fakeCXX(t, `f="$FAKE_DIR/run.$$"
: > "$f"
ls "$FAKE_DIR" | grep -c '^run\.' >> "$FAKE_DIR/counts"
sleep 0.3
rm -f "$f"`)
	var qs []DeclQuery
	for i := 0; i < 12; i++ {
		qs = append(qs, DeclQuery{DeclType, "Name" + strconv.Itoa(i)})
	}
	for _, par := range []int{1, 3} {
		os.Remove(filepath.Join(dir, "counts"))
		rn := new(Runner)
		if _, err := rn.Decls(context.Background(), Config{CXX: cxx, Dir: t.TempDir()}, DeclConfig{Parallel: par}, qs); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(filepath.Join(dir, "counts"))
		most, n := 0, 0
		for _, l := range strings.Fields(string(b)) {
			c, _ := strconv.Atoi(l)
			most = max(most, c)
			n++
		}
		if n != 12 || most > par || par > 1 && most < 2 {
			t.Errorf("Parallel %d: %d runs, at most %d at once", par, n, most)
		}
	}
}
