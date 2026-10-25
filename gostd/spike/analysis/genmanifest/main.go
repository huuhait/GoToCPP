// Command genmanifest writes gostd/spike/analysis/programs/manifest.json: it runs every program
// with go run (the oracle), optionally with gocpp run -gc-engine (GOCPP), and records the
// constructs each program covers (the table in this file). Job 16.1f2-plan.
//
//	go run ./gostd/spike/analysis/genmanifest gostd/spike/analysis/programs
//	GOCPP=/path/to/gocpp go run ./gostd/spike/analysis/genmanifest gostd/spike/analysis/programs
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type gocppStatus struct {
	Status string `json:"status"`           // "ok" or "error"
	Output string `json:"output,omitempty"` // the full output when ok
	Error  string `json:"error,omitempty"`  // from "main.go:" on, when error
}

type program struct {
	Name       string      `json:"name"`
	Dir        string      `json:"dir"`
	Constructs []string    `json:"constructs"`
	Imports    []string    `json:"imports"`
	Expected   string      `json:"expected"`
	Gocpp      gocppStatus `json:"gocpp"`
}

type manifest struct {
	Schema        int       `json:"schema"`
	GoVersion     string    `json:"go_version"`
	GOOS          string    `json:"goos"`
	GOARCH        string    `json:"goarch"`
	Note          string    `json:"note"`
	ExpectedNotes []string  `json:"expected_notes"`
	Programs      []program `json:"programs"`
}

var constructs = map[string][]string{
	"00_baseline":    {"package main", "func declarations", "calls", ":= and var", "if/else", "for (three-clause, condition-only)", "break/continue", "integer arithmetic and comparisons", "shifts/div/rem with Go semantics", "string concat/compare", "len(string)", "print/println", "bool", "evaluation order"},
	"01_decls":       {"package-level consts", "iota", "package-level vars", "package-level named type", "func init and its order", "constant expressions", "string concatenation"},
	"02_values":      {"integer widths (int8..uint64, uintptr)", "unsigned wrap", "float32/float64", "byte/rune", "numeric conversions", "rune to string", "min/max", "shifts", "bitwise ops", "untyped constants"},
	"03_strings":     {"string literals and escapes", "string indexing (byte)", "string slicing", "len", "range over string (runes)", "comparison", "concatenation", "string<->[]byte", "string<->[]rune", "append to []byte"},
	"04_slices":      {"make with len/cap", "append", "copy", "len/cap", "two- and three-index slicing", "array values", "keyed array literal", "range over slice/array/int", "nil slice", "composite literals"},
	"05_maps":        {"map types", "map literals", "index and index-comma-ok", "assignment", "delete", "clear", "len", "range over map", "nil map", "make with hint", "struct keys"},
	"06_structs":     {"struct types", "fields", "struct tags", "embedding and promotion", "zero-size fields (struct{}, [0]int)", "anonymous struct", "keyed/positional composite literals", "pointers to structs", "new(T)", "struct value copy"},
	"07_methods":     {"value receivers", "pointer receivers", "method values", "method expressions", "promoted methods", "receiver copy semantics"},
	"08_interfaces":  {"interface declarations", "satisfaction", "dynamic dispatch", "type assertion (comma-ok and direct)", "type switch", "nil interface", "empty interface", "Stringer", "interface slices"},
	"09_closures":    {"func values", "closures", "higher-order functions", "captured variables", "per-iteration loop variables (Go 1.22)", "recursive func value"},
	"10_defer":       {"defer (LIFO)", "panic", "recover", "multiple results", "named results", "deferred mutation of named results"},
	"11_control":     {"expression switch", "tagless switch", "switch init", "fallthrough", "labeled break/continue", "goto"},
	"12_variadic":    {"variadic functions", "call with no/one/many", "... spread", "func value of a variadic function"},
	"13_generics":    {"constraint interfaces with type terms", "generic funcs", "generic types and methods", "inferred and explicit type arguments", "generic higher-order function"},
	"14_unsafe":      {"unsafe.Sizeof/Alignof/Offsetof", "unsafe.String", "unsafe.SliceData", "unsafe.Pointer round trip", "nil typed pointer"},
	"15_goroutines":  {"go statements", "buffered/unbuffered channels", "channel send/receive", "range over channel (in worker)", "close", "select with default", "select with two cases", "receive from closed channel"},
	"16_binarytrees": {"structs", "pointers", "methods", "recursion", "slice-free tree walk", "goroutines", "channels", "composite literals"},
	"17_list":        {"imports", "container/list", "any interface", "type assertion", "linked pointers", "methods", "iteration"},
	"18_strings_pkg": {"imports", "strings package", "variadic NewReplacer", "strings.Builder", "func values", "range over iter.Seq (range-over-func)", "closures"},
	"19_strconv_pkg": {"imports", "strconv package", "error interface", "floats", "complex", "byte slices", "quotes"},
	"20_sort_pkg":    {"imports", "sort package", "generics via slices.Sort", "interface satisfaction", "embedded Interface (Reverse)", "func literals", "any + internal/reflectlite (Slice/SliceStable)"},
	"21_unicode":     {"imports", "unicode package", "package-level map literals (six maps)", "map lookups", "slice fields", "named slice type methods (SpecialCase)", "package initialization order"},
}

// sourceImports returns the import paths of a Go file, sorted and without duplicates. The
// manifest's imports column is derived from the program's own source, never typed by hand.
func sourceImports(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("%s:%s: %v", path, spec.Path.Value, err)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

func run(dir, name string, args ...string) (string, int) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return buf.String(), code
}

func main() {
	root := os.Args[1]
	gocpp := os.Getenv("GOCPP")
	entries, err := os.ReadDir(root)
	if err != nil {
		panic(err)
	}
	m := manifest{
		Schema:    1,
		GoVersion: "go1.25.14",
		GOOS:      "linux",
		GOARCH:    "amd64",
		Note:      "Generated by gostd/spike/analysis/genmanifest (job 16.1f2-plan) with go run (the oracle) and, when GOCPP was set, gocpp run -gc-engine. The gocpp column is the measurement of job 16.1c on branch p16 (before 16.1f-load and the 16.1f emitter jobs); update it when a job adds a construct.",
		ExpectedNotes: []string{
			"Float, complex and pointer values print with Go's built-in println format (+2.500000e+000, (1.5-2.5i)), not fmt's.",
			"02_values and 14_unsafe print width-dependent numbers (uint wrap, uintptr, struct sizes): they are linux/amd64/64-bit int's, and the manifest is linux/amd64.",
		},
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir := filepath.Join(root, name)
		out, code := run(dir, "go", "run", ".")
		if code != 0 {
			panic(fmt.Sprintf("%s: go run exit %d:\n%s", name, code, out))
		}
		imps, err := sourceImports(filepath.Join(dir, "main.go"))
		if err != nil {
			panic(err)
		}
		p := program{Name: name, Dir: name, Constructs: constructs[name], Imports: imps, Expected: out}
		if gocpp != "" {
			gout, gcode := run(dir, gocpp, "run", "-gc-engine", ".")
			if gcode == 0 {
				p.Gocpp = gocppStatus{Status: "ok", Output: gout}
			} else {
				msg := gout
				if i := strings.Index(msg, "main.go:"); i >= 0 {
					msg = msg[i:]
				}
				msg = strings.TrimSuffix(msg, "\n")
				p.Gocpp = gocppStatus{Status: "error", Error: msg}
			}
		} else {
			p.Gocpp = gocppStatus{Status: "unmeasured"}
		}
		m.Programs = append(m.Programs, p)
	}
	sort.Slice(m.Programs, func(i, j int) bool { return m.Programs[i].Name < m.Programs[j].Name })
	data, err := json.MarshalIndent(m, "", "\t")
	if err != nil {
		panic(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), data, 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s (%d programs)\n", filepath.Join(root, "manifest.json"), len(m.Programs))
}
