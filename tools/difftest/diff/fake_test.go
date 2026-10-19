package diff

import (
	"context"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// fakeBackend is a Backend made of a function.
type fakeBackend struct {
	name string
	f    func(src string) Result
	runs atomic.Int64
}

func (b *fakeBackend) Name() string { return b.name }
func (b *fakeBackend) Run(ctx context.Context, src string) (Result, error) {
	b.runs.Add(1)
	return b.f(src), nil
}

var (
	impOnce sync.Once
	imp     types.Importer
	impMu   sync.Mutex
)

// typeCheckErr returns the first type error of a Go source, "" if there is none.
func typeCheckErr(src string) string {
	impOnce.Do(func() { imp = importer.ForCompiler(token.NewFileSet(), "source", nil) })
	fset := token.NewFileSet()
	var files []*ast.File
	for _, sf := range SplitFiles(src) {
		f, err := parser.ParseFile(fset, sf.Name+".go", sf.Text, 0)
		if err != nil {
			return err.Error()
		}
		files = append(files, f)
	}
	var first string
	conf := types.Config{Importer: imp, Error: func(err error) {
		if first == "" {
			first = err.Error()
		}
	}}
	impMu.Lock()
	defer impMu.Unlock()
	conf.Check("main", fset, files, nil)
	return first
}

var markerLineRx = regexp.MustCompile(`(?m)^\tfmt\.Println\("#### (\d+)"\)$`)

// programIDs returns the programs a bundle runs.
func programIDs(src string) []int {
	var ids []int
	for _, m := range markerLineRx.FindAllStringSubmatch(src, -1) {
		var id int
		fmt.Sscan(m[1], &id)
		ids = append(ids, id)
	}
	return ids
}

// fakeGo is a reference that type-checks the bundle and "runs" it: each program prints one
// line that depends only on its id.
func fakeGo() *fakeBackend { return fakeGoOut(nil) }

// fakeGoOut is fakeGo with the output of a program of the bundle given by out (nil: "result N").
func fakeGoOut(out func(src string, id int) string) *fakeBackend {
	return &fakeBackend{name: "go", f: func(src string) Result {
		if e := typeCheckErr(src); e != "" {
			// rewrite the position the way go build prints it
			return Result{BuildErr: "./" + e}
		}
		var sb strings.Builder
		for _, id := range programIDs(src) {
			if out != nil {
				if o := out(progSource(src, id), id); o != "" {
					fmt.Fprintf(&sb, "#### %d\n== prog\n%s\n", id, o)
					continue
				}
			}
			fmt.Fprintf(&sb, "#### %d\n== prog\nresult %d\n", id, id*7)
		}
		return Result{BuildOK: true, Stdout: sb.String()}
	}}
}

// fakeCpp is a subject that prints what the reference prints, except for programs whose
// source contains the bug marker: those print "wrong".
func fakeCpp(bug func(src string, id int) string) *fakeBackend {
	return &fakeBackend{name: "gocpp/fake", f: func(src string) Result {
		if strings.Contains(src, "REJECTED(7)") {
			return Result{BuildErr: "main.gp:7:2: unsupported construct\n"}
		}
		var out strings.Builder
		for _, id := range programIDs(src) {
			// the part of src that belongs to the program
			fmt.Fprintf(&out, "#### %d\n== prog\n", id)
			if w := bug(progSource(src, id), id); w != "" {
				out.WriteString(w + "\n")
			} else {
				fmt.Fprintf(&out, "result %d\n", id*7)
			}
		}
		return Result{BuildOK: true, Stdout: out.String()}
	}}
}

// progSource returns the declarations of program id of a bundle (of any number of files).
func progSource(src string, id int) string {
	for _, f := range SplitFiles(src) {
		if s := progSource1(f.Text, id); s != "" {
			return s
		}
	}
	return ""
}

func progSource1(src string, id int) string {
	start := strings.Index(src, fmt.Sprintf("// ---- program %d ", id))
	if start < 0 {
		return ""
	}
	end := strings.Index(src[start+1:], "// ---- program ")
	if end < 0 {
		end = strings.Index(src[start:], "func main()")
	} else {
		end++
	}
	if end < 0 {
		return src[start:]
	}
	return src[start : start+end]
}
