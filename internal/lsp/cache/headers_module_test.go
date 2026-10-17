package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
)

// TestHeaderFileModule: go to definition on the import of a module header
// (step 6.1b) opens the file that the loader found, with no search of the
// package folder or of the compiler's folders.
func TestHeaderFileModule(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"gocpp.mod":  "module example.com/app\n",
		"main.gp":    "package main\n\nimport \"example.com/app/native/x.h\"\n\nfunc main() { x.f() }\n",
		"native/x.h": "void f();\n",
	}
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prog := load.Load(load.Config{Dir: root}, ".")
	if prog.HasErrors() || len(prog.Roots) != 1 || len(prog.Roots[0].Files) != 1 || len(prog.Roots[0].Files[0].Info.Includes) != 1 {
		t.Fatalf("load: %+v", prog.Errors.Diagnostics())
	}
	inc := prog.Roots[0].Files[0].Info.Includes[0]
	p := NewPipeline(testConfig())
	t.Cleanup(p.Close)
	// The folder of the .gp file is another one: the file is not found by name there.
	got, err := p.HeaderFile(context.Background(), t.TempDir(), inc)
	if want := filepath.Join(root, "native", "x.h"); err != nil || got != want {
		t.Errorf("HeaderFile = %q, %v; want %q", got, err, want)
	}
	// A header that is only in the overlay (an unsaved buffer) resolves too.
	over := filepath.Join(root, "native", "y.h")
	src := "package main\n\nimport \"example.com/app/native/y.h\"\n\nfunc main() { y.g() }\n"
	if err := os.WriteFile(filepath.Join(root, "main.gp"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	prog = load.Load(load.Config{Dir: root, Overlay: map[string][]byte{over: []byte("void g();\n")}}, ".")
	if prog.HasErrors() {
		t.Fatalf("load with an overlay: %+v", prog.Errors.Diagnostics())
	}
	got, err = p.HeaderFile(context.Background(), root, prog.Roots[0].Files[0].Info.Includes[0])
	if err != nil || got != over {
		t.Errorf("an overlay header: %q, %v; want %q", got, err, over)
	}
}
