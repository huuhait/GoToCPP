package doc

import (
	"bytes"
	"fmt"
	"go/ast"
	godoc "go/doc"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/testgo"
)

// The differential test: the same packages as .go files and as .gp files, and
// the output of the real go doc against the output of Run, byte for byte.
// testdata/diffmod has the packages (with the extension .src: gofmt and go vet
// would read .go files in testdata).

// diffModule copies testdata/diffmod to a new folder, as .go files with a
// go.mod and as .gp files with a gocpp.mod, and returns the two folders.
func diffModule(t *testing.T) (goDir, gpDir string) {
	t.Helper()
	root := t.TempDir()
	goDir, gpDir = filepath.Join(root, "go"), filepath.Join(root, "gp")
	err := filepath.WalkDir("testdata/diffmod", func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel("testdata/diffmod", p)
		stem := strings.TrimSuffix(rel, ".src")
		if stem == rel {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, f := range []struct{ dir, ext string }{{goDir, ".go"}, {gpDir, ".gp"}} {
			dst := filepath.Join(f.dir, stem+f.ext)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(goDir, "go.mod"), "module example.com/diffmod\n\ngo 1.25\n")
	write(t, filepath.Join(gpDir, "gocpp.mod"), "module example.com/diffmod\n")
	return goDir, gpDir
}

func write(t *testing.T, name, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// goDoc runs the real go doc.
type goDoc struct {
	goBin string
	env   []string
}

func newGoDoc(t *testing.T) *goDoc {
	t.Helper()
	bin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go is not installed: %v", err)
	}
	// go doc must be the Go that the port follows (go.mod): a newer one prints other texts and
	// refuses other flags (Go 1.27 refuses -all with -short).
	out, err := exec.Command(bin, "env", "GOVERSION").Output()
	if err != nil {
		t.Fatalf("go env GOVERSION: %v", err)
	}
	testgo.SkipUnlessModuleVersion(t, "go", string(out))
	tmp := t.TempDir()
	// Run (below) documents for linux/amd64, so go doc must too, not for the system of the test.
	return &goDoc{goBin: bin, env: append(os.Environ(),
		"GOTOOLCHAIN=local", "GOPROXY=off", "GOFLAGS=", "GOWORK=off", "GO111MODULE=on", "GOOS=linux", "GOARCH=amd64",
		"GOCACHE="+filepath.Join(tmp, "cache"), "GOPATH="+filepath.Join(tmp, "path"), "GOMODCACHE="+filepath.Join(tmp, "mod"))}
}

// run returns what "go doc args" prints on standard output, and its error
// text (the standard error without the "doc: " prefix, "" for success).
func (g *goDoc) run(dir string, args ...string) (out, errText string) {
	cmd := exec.Command(g.goBin, append([]string{"doc"}, args...)...)
	cmd.Dir = dir
	cmd.Env = g.env
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	if err := cmd.Run(); err != nil && e.Len() == 0 {
		return o.String(), err.Error()
	}
	return o.String(), strings.TrimSuffix(strings.TrimPrefix(e.String(), "doc: "), "\n")
}

// gpDoc runs Run, with the flags in the form of go doc's command line.
func gpDoc(dir string, args ...string) (out, errText string) {
	var opts Options
	for len(args) > 0 && strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "-u":
			opts.Unexported = true
		case "-c":
			opts.MatchCase = true
		case "-all":
			opts.All = true
		case "-cmd":
			opts.Cmd = true
		case "-src":
			opts.Src = true
		case "-short":
			opts.Short = true
		default:
			panic("flag " + args[0])
		}
		args = args[1:]
	}
	var buf bytes.Buffer
	err := Run(&buf, &Env{Dir: dir, GOOS: "linux", GOARCH: "amd64"}, opts, args)
	if err != nil {
		return buf.String(), err.Error()
	}
	return buf.String(), ""
}

// symbols lists the names that the .go files of the package in dir declare:
// "T", "T.Method", "T.Field", "F", "C", with the ones that are unexported
// marked.
func symbols(t *testing.T, dir string) (exported, unexported []string) {
	t.Helper()
	fset := token.NewFileSet()
	//lint:ignore SA1019 the test reads one folder with a file filter
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_windows.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, ap := range pkgs {
		dp := godoc.New(ap, "x", godoc.AllDecls)
		add := func(n string) { names = append(names, n) }
		for _, c := range dp.Consts {
			for _, n := range c.Names {
				add(n)
			}
		}
		for _, v := range dp.Vars {
			for _, n := range v.Names {
				add(n)
			}
		}
		for _, f := range dp.Funcs {
			add(f.Name)
		}
		for _, ty := range dp.Types {
			add(ty.Name)
			for _, c := range ty.Consts {
				for _, n := range c.Names {
					add(n)
				}
			}
			for _, v := range ty.Vars {
				for _, n := range v.Names {
					add(n)
				}
			}
			for _, f := range ty.Funcs {
				add(f.Name)
			}
			for _, m := range ty.Methods {
				add(ty.Name + "." + m.Name)
			}
			for _, spec := range ty.Decl.Specs {
				ts := spec.(*ast.TypeSpec)
				switch x := ts.Type.(type) {
				case *ast.StructType:
					for _, f := range x.Fields.List {
						for _, n := range f.Names {
							add(ty.Name + "." + n.Name)
						}
					}
				case *ast.InterfaceType:
					for _, f := range x.Methods.List {
						for _, n := range f.Names {
							add(ty.Name + "." + n.Name)
						}
					}
				}
			}
		}
	}
	sort.Strings(names)
	names = slices.Compact(names)
	for _, n := range names {
		if token.IsExported(n) && !strings.Contains(n, ".") || strings.Contains(n, ".") && token.IsExported(n[strings.Index(n, ".")+1:]) && token.IsExported(n) {
			exported = append(exported, n)
		} else {
			unexported = append(unexported, n)
		}
	}
	return exported, unexported
}

var installedIn = regexp.MustCompile(`installed in "example.com/diffmod/\w+"`)

// A diffCase is one run of go doc and of Run that must print the same.
type diffCase struct{ args []string }

func TestDiffGoDoc(t *testing.T) {
	g := newGoDoc(t)
	goDir, gpDir := diffModule(t)
	pkgs, err := os.ReadDir(goDir)
	if err != nil {
		t.Fatal(err)
	}
	var cases []diffCase
	add := func(args ...string) { cases = append(cases, diffCase{args}) }
	for _, e := range pkgs {
		if !e.IsDir() {
			continue
		}
		p := e.Name()
		if p == "zdupe" {
			continue
		}
		for _, flags := range [][]string{nil, {"-all"}, {"-short"}, {"-u"}, {"-u", "-all"}, {"-src"}, {"-cmd"}, {"-cmd", "-all"}, {"-short", "-all"}} {
			add(append(slices.Clone(flags), p)...)
		}
		exp, unexp := symbols(t, filepath.Join(goDir, p))
		for _, s := range exp {
			add(p, s)
			add("-src", p, s)
			if !strings.Contains(s, ".") {
				add("-short", p, s)
				add(p + "." + s) // the one-argument form
				add(p, strings.ToLower(s))
			}
		}
		for _, s := range unexp {
			add("-u", p, s)
			add(p, s) // not found: the same error
		}
	}
	var mu sync.Mutex
	var failures []string
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, c := range cases {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			wantOut, wantErr := g.run(goDir, c.args...)
			// Documented difference: an error names a package by its
			// import path; go doc names the folder of the package
			// that it found with go list.
			wantErr = strings.ReplaceAll(wantErr, goDir, "example.com/diffmod")
			gotOut, gotErr := gpDoc(gpDir, c.args...)
			// More documented differences: .gp files, and the import
			// path where go doc, which reads the folder with go/build,
			// prints "." or the folder.
			gotErr = strings.ReplaceAll(gotErr, "buildable .gp source", "buildable Go source")
			gotErr = strings.ReplaceAll(gotErr, gpDir, "example.com/diffmod")
			gotErr = installedIn.ReplaceAllString(gotErr, `installed in "."`)
			if wantOut != gotOut || wantErr != gotErr {
				mu.Lock()
				failures = append(failures, fmt.Sprintf("gocpp doc %s\n--- go doc (stderr %q)\n%s--- gocpp doc (stderr %q)\n%s", strings.Join(c.args, " "), wantErr, wantOut, gotErr, gotOut))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	t.Logf("%d invocations compared", len(cases))
	sort.Strings(failures)
	for i, f := range failures {
		if i >= 8 {
			t.Errorf("... and %d more differences", len(failures)-i)
			break
		}
		t.Error(f)
	}
}
