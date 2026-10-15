// Gen-example-output writes the expected output (output.txt) of the target
// programs in examples/ by running them with real Go, so that no expected
// output of a Go-compatible program is ever written by hand.
//
// Usage, from the repository root:
//
//	go run ./tools/gen-example-output [-check] [-race] [-v] [-dir examples] [name ...]
//
// Without names it handles every example. An example is a folder
// examples/<name>/ with a main.gp, of one of three kinds:
//
//   - go: the .gp files are valid Go (no header imports, no C/C++ files, no
//     gocpp.mod). The tool copies the folder into a temporary module, renames
//     .gp to .go, builds it with "go build" and runs it.
//   - twin: the program uses gocpp-only syntax (! or macros). Its folder
//     holds twin/, a Go module with the program expanded by hand into the Go
//     that gocpp generates; the tool builds and runs the twin.
//   - interop: the program uses C/C++ (header imports, .c/.cpp/.h files or a
//     gocpp.mod). Go can't run it, so its output.txt is written by hand; the
//     tool only checks that output.txt exists.
//
// A program runs with the example folder as its working directory, the
// arguments from args.txt (one line, split on spaces), the extra environment
// variables from env.txt (KEY=VALUE lines) and the contents of stdin.txt as
// standard input. Its exit status must be the one in the "// ExitCode: N"
// line at the top of main.gp (0 when there is none). Its standard output is
// the expected output.
//
// Before running a program the tool runs "go vet", and "go test" when there
// are _test.gp files; both must pass. It also checks the header lines of every
// main.gp, that the .gp files are formatted like gofmt (gocpp-only syntax is
// mapped to same-shape Go first), and that the macro use sites written in a
// twin (File and Line) point at the right lines of the gocpp program. With
// -check nothing is written: the tool reports every output.txt that is
// missing or out of date and exits with 1.
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Kinds of examples.
const (
	kindGo      = "go"
	kindTwin    = "twin"
	kindInterop = "interop"
)

// runTimeout bounds one build, test or run step, so that a program that hangs
// can't hang the tool.
const runTimeout = 2 * time.Minute

type config struct {
	check bool // compare, don't write
	race  bool // build and test with -race
	v     bool // print the commands and their output
	out   io.Writer
}

func main() {
	var cfg config
	flag.BoolVar(&cfg.check, "check", false, "don't write: fail if an output.txt is missing or out of date")
	flag.BoolVar(&cfg.race, "race", false, "build and test with the race detector")
	flag.BoolVar(&cfg.v, "v", false, "print every command and its output")
	dir := flag.String("dir", "", "the examples folder (default: examples/ in the repository root)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: gen-example-output [-check] [-race] [-v] [-dir examples] [name ...]")
		flag.PrintDefaults()
	}
	flag.Parse()
	cfg.out = os.Stdout

	root := *dir
	if root == "" {
		var err error
		if root, err = findExamples(); err != nil {
			fmt.Fprintln(os.Stderr, "gen-example-output:", err)
			os.Exit(2)
		}
	}
	if err := run(cfg, root, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "gen-example-output:", err)
		os.Exit(1)
	}
}

// findExamples looks for examples/ next to the go.mod of the repository,
// starting in the current folder and going up.
func findExamples() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		ex := filepath.Join(dir, "examples")
		if fi, err := os.Stat(ex); err == nil && fi.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return ex, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no examples/ folder found; run in the repository or pass -dir")
		}
		dir = parent
	}
}

// run handles the examples named in names, or all of them, and returns an
// error when any of them failed.
func run(cfg config, root string, names []string) error {
	all, err := listExamples(root)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		names = all
	}
	failed := 0
	for _, name := range names {
		if !slices.Contains(all, name) {
			fmt.Fprintf(cfg.out, "%-14s FAIL: no example folder with a main.gp\n", name)
			failed++
			continue
		}
		msg, err := handle(cfg, filepath.Join(root, name))
		if err != nil {
			fmt.Fprintf(cfg.out, "%-14s FAIL: %v\n", name, err)
			failed++
			continue
		}
		fmt.Fprintf(cfg.out, "%-14s %s\n", name, msg)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d examples failed", failed, len(names))
	}
	return nil
}

// listExamples returns the names of the folders in root that have a main.gp.
func listExamples(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && fileExists(filepath.Join(root, e.Name(), "main.gp")) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// handle checks one example and, for the go and twin kinds, produces its
// output. It returns a one-line summary.
func handle(cfg config, dir string) (string, error) {
	h, err := readHeader(filepath.Join(dir, "main.gp"))
	if err != nil {
		return "", err
	}
	kind, err := classify(dir)
	if err != nil {
		return "", err
	}
	if err := checkFormat(dir, kind); err != nil {
		return "", err
	}
	if kind == kindTwin {
		if err := checkTwinSites(dir); err != nil {
			return "", err
		}
	}
	outPath := filepath.Join(dir, "output.txt")
	if kind == kindInterop {
		if !fileExists(outPath) {
			return "", errors.New("interop example without output.txt (write it by hand)")
		}
		return fmt.Sprintf("phase %02d, interop: output.txt is hand-written (only checked to exist)", h.phase), nil
	}

	tmp, err := os.MkdirTemp("", "gen-example-output-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)

	// modDir is the Go module to build: a copy of the example, or its twin.
	modDir := filepath.Join(dir, "twin")
	if kind == kindGo {
		modDir = filepath.Join(tmp, "src")
		if err := copyAsGo(dir, modDir, filepath.Base(dir)); err != nil {
			return "", err
		}
	}
	bin := filepath.Join(tmp, filepath.Base(dir))
	build := []string{"build", "-o", bin}
	vet := []string{"vet", "./..."}
	test := []string{"test", "-count=1", "./..."}
	if cfg.race {
		build = append(build, "-race")
		test = append(test, "-race")
	}
	if err := goCmd(cfg, modDir, append(build, ".")...); err != nil {
		return "", err
	}
	if err := goCmd(cfg, modDir, vet...); err != nil {
		return "", err
	}
	tested := ""
	if hasTests(modDir) {
		if err := goCmd(cfg, modDir, test...); err != nil {
			return "", err
		}
		tested = ", go test ok"
	}

	got, err := runProgram(cfg, bin, dir, h.exitCode)
	if err != nil {
		return "", err
	}
	want, err := os.ReadFile(outPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	how := "go run"
	if kind == kindTwin {
		how = "twin"
	}
	switch {
	case err == nil && bytes.Equal(got, want):
		return fmt.Sprintf("phase %02d, %s%s: output.txt is up to date", h.phase, how, tested), nil
	case cfg.check && err != nil:
		return "", errors.New("output.txt is missing (run without -check to write it)")
	case cfg.check:
		return "", fmt.Errorf("output.txt is out of date (run without -check to rewrite it): %s", firstDiff(want, got))
	}
	if err := os.WriteFile(outPath, got, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("phase %02d, %s%s: output.txt written", h.phase, how, tested), nil
}

// header holds the lines at the top of main.gp that the tools read.
type header struct {
	phase    int
	exitCode int
}

var (
	phaseRe    = regexp.MustCompile(`^// phase: (\d\d)$`)
	exitCodeRe = regexp.MustCompile(`^// ExitCode: (\d+)$`)
)

// readHeader reads the comment lines before the package clause of main.gp.
// The first line must be "// phase: NN" with a phase of the plan (00 to 15).
func readHeader(path string) (header, error) {
	f, err := os.Open(path)
	if err != nil {
		return header{}, err
	}
	defer f.Close()
	h := header{phase: -1}
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimRight(sc.Text(), "\r")
		if n == 1 {
			m := phaseRe.FindStringSubmatch(line)
			if m == nil {
				return h, fmt.Errorf(`main.gp: the first line must be "// phase: NN", not %q`, line)
			}
			h.phase, _ = strconv.Atoi(m[1])
			if h.phase > 15 {
				return h, fmt.Errorf("main.gp: phase %02d does not exist (00 to 15)", h.phase)
			}
			continue
		}
		if !strings.HasPrefix(line, "//") && strings.TrimSpace(line) != "" {
			break // the package clause or a declaration: the header is over
		}
		if m := exitCodeRe.FindStringSubmatch(line); m != nil {
			h.exitCode, _ = strconv.Atoi(m[1])
		}
	}
	if err := sc.Err(); err != nil {
		return h, err
	}
	if h.phase < 0 {
		return h, errors.New("main.gp is empty")
	}
	return h, nil
}

// headerExts are the file name endings of C/C++ headers (Phase 01 §2.3).
var headerExts = []string{".h", ".hh", ".hpp", ".hxx", ".h++", ".inl"}

// cppExts are the C/C++ files that make an example an interop example.
var cppExts = append([]string{".c", ".cc", ".cpp", ".cxx"}, headerExts...)

// classify says how the output of the example in dir is produced: it has a
// twin; or it uses C/C++ (a header import, a C/C++ file or a gocpp.mod); or
// it must be plain Go.
func classify(dir string) (string, error) {
	if fi, err := os.Stat(filepath.Join(dir, "twin")); err == nil && fi.IsDir() {
		if !fileExists(filepath.Join(dir, "twin", "go.mod")) {
			return "", errors.New("twin/ has no go.mod")
		}
		return kindTwin, nil
	}
	var gpFiles []string
	interop := false
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := d.Name()
		if name == "gocpp.mod" || slices.Contains(cppExts, filepath.Ext(name)) {
			interop = true
		}
		if filepath.Ext(name) != ".gp" {
			return nil
		}
		gpFiles = append(gpFiles, path)
		// The imports parse as Go in every .gp file: a header import is
		// only a string.
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); isHeaderImport(p) {
				interop = true
			}
		}
		return nil
	})
	if err != nil || interop {
		return kindInterop, err
	}
	for _, path := range gpFiles {
		if _, err := parser.ParseFile(token.NewFileSet(), path, nil, 0); err != nil {
			return "", fmt.Errorf("not valid Go, and there is no twin/ (gocpp-only syntax needs one): %v", err)
		}
	}
	return kindGo, nil
}

// isHeaderImport reports whether an import path names a C/C++ header or a
// C++ namespace instead of a gocpp package (Phase 01 §2.3).
func isHeaderImport(path string) bool {
	if strings.HasPrefix(path, "<") || strings.HasPrefix(path, "::") || strings.HasPrefix(path, "./") {
		return true
	}
	return slices.Contains(headerExts, filepath.Ext(path))
}

// copyAsGo copies the example in src to a new module in dst, with every .gp
// file renamed to .go. The files that only describe a run are left out.
func copyAsGo(src, dst, name string) error {
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if rel == "output.txt" || rel == "args.txt" || rel == "stdin.txt" || rel == "env.txt" {
			return nil
		}
		if strings.HasSuffix(target, ".gp") {
			target = strings.TrimSuffix(target, ".gp") + ".go"
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		return err
	}
	mod := fmt.Sprintf("module example.com/%s\n\ngo 1.25\n", name)
	return os.WriteFile(filepath.Join(dst, "go.mod"), []byte(mod), 0o644)
}

func hasTests(dir string) bool {
	found := false
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), "_test.go") {
			found = true
			return filepath.SkipAll
		}
		return err
	})
	return found
}

// goEnv is the environment of every go command: no workspace file and no
// toolchain download can change what is built, and GOAMD64=v1, because gc
// fuses a*b + c from v3 on and gocpp gives the results without fusing.
func goEnv() []string {
	return append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOAMD64=v1")
}

// goCmd runs the go command in dir and fails with its output.
func goCmd(cfg config, dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = goEnv()
	out, err := cmd.CombinedOutput()
	if cfg.v {
		fmt.Fprintf(cfg.out, "  (in %s) go %s\n%s", dir, strings.Join(args, " "), indent(out))
	}
	if err != nil {
		return fmt.Errorf("go %s: %v\n%s", args[0], err, indent(out))
	}
	return nil
}

// runProgram runs bin in the example folder dir and returns its standard
// output. The exit status must be want.
func runProgram(cfg config, bin, dir string, want int) ([]byte, error) {
	args, err := readArgs(filepath.Join(dir, "args.txt"))
	if err != nil {
		return nil, err
	}
	env, err := readEnv(filepath.Join(dir, "env.txt"))
	if err != nil {
		return nil, err
	}
	var stdin io.Reader = strings.NewReader("")
	if data, err := os.ReadFile(filepath.Join(dir, "stdin.txt")); err == nil {
		stdin = bytes.NewReader(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if cfg.v {
		line := strings.Join(append([]string{filepath.Base(bin)}, args...), " ")
		fmt.Fprintf(cfg.out, "  (in %s) %s\n%s", dir, line, indent(stderr.Bytes()))
	}
	code := 0
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr) && exitErr.Exited():
		code = exitErr.ExitCode()
	case err != nil:
		return nil, fmt.Errorf("running the program: %v\n%s", err, indent(stderr.Bytes()))
	}
	if code != want {
		return nil, fmt.Errorf("the program exited with %d, want %d\n%s", code, want, indent(stderr.Bytes()))
	}
	return stdout.Bytes(), nil
}

// readArgs reads the one line of args.txt, split on spaces. A missing file
// means no arguments.
func readArgs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	text := strings.TrimRight(string(data), "\r\n")
	if strings.ContainsAny(text, "\r\n") {
		return nil, errors.New("args.txt must have exactly one line")
	}
	return strings.Fields(text), nil
}

// readEnv reads the KEY=VALUE lines of env.txt. A missing file means none.
func readEnv(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var env []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if k, _, ok := strings.Cut(line, "="); !ok || k == "" {
			return nil, fmt.Errorf("env.txt: %q is not KEY=VALUE", line)
		}
		env = append(env, line)
	}
	return env, nil
}

// checkFormat checks that every .gp file of the example parses and is
// formatted like gofmt would format it. Go-compatible files are checked as
// they are; in the others, gocpp-only syntax is first mapped to Go of the
// same shape (gocppToGo). A file that doesn't parse is an error, also after
// the mapping.
func checkFormat(dir, kind string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".gp" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		mapped := src
		if kind != kindGo {
			mapped = gocppToGo(src)
		}
		formatted, err := format.Source(mapped)
		switch {
		case err != nil && kind == kindGo:
			return fmt.Errorf("%s does not parse: %v", rel, err)
		case err != nil:
			return fmt.Errorf("%s does not parse, also with the gocpp-only syntax mapped to Go: %v "+
				"(if it is gocpp syntax that gocppToGo doesn't map yet, extend gocppToGo)", rel, err)
		case !bytes.Equal(formatted, mapped):
			return fmt.Errorf("%s is not gofmt-clean: %s", rel, firstDiff(formatted, mapped))
		}
		return nil
	})
}

// checkTwinSites checks the macro use sites written in a twin: every
// composite literal with a File key naming a .gp file and a Line key must
// point at a line of that file with a # on it, and each string of its Args
// (the source text of the arguments) must be on that line. A site that no
// longer matches the gocpp program would make #file() and #line() in the
// expected output wrong.
func checkTwinSites(dir string) error {
	lines := map[string][]string{}
	var problems []string
	err := filepath.WalkDir(filepath.Join(dir, "twin"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			file, line, args, ok := siteFields(lit)
			if !ok {
				return true
			}
			src, ok := lines[file]
			if !ok {
				data, err := os.ReadFile(filepath.Join(dir, file))
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s: %v", fset.Position(lit.Pos()), err))
					return true
				}
				src = strings.Split(string(data), "\n")
				lines[file] = src
			}
			switch {
			case line < 1 || line > len(src):
				problems = append(problems, fmt.Sprintf("%s: %s has no line %d", fset.Position(lit.Pos()), file, line))
			case !strings.Contains(src[line-1], "#"):
				problems = append(problems, fmt.Sprintf("%s: %s:%d has no macro use: %q", fset.Position(lit.Pos()), file, line, src[line-1]))
			default:
				for _, a := range args {
					if !strings.Contains(src[line-1], a) {
						problems = append(problems, fmt.Sprintf("%s: %s:%d does not contain %q", fset.Position(lit.Pos()), file, line, a))
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("twin sites don't match the program:\n    %s", strings.Join(problems, "\n    "))
	}
	return nil
}

// siteFields reads File, Line and Args from a composite literal like
// macro.Site{File: "main.gp", Line: 12, Args: []string{"x"}}. ok is false
// when the literal is not a site.
func siteFields(lit *ast.CompositeLit) (file string, line int, args []string, ok bool) {
	hasLine := false
	for _, e := range lit.Elts {
		kv, isKV := e.(*ast.KeyValueExpr)
		if !isKV {
			continue
		}
		key, isIdent := kv.Key.(*ast.Ident)
		if !isIdent {
			continue
		}
		switch v := kv.Value.(type) {
		case *ast.BasicLit:
			switch {
			case key.Name == "File" && v.Kind == token.STRING:
				file, _ = strconv.Unquote(v.Value)
			case key.Name == "Line" && v.Kind == token.INT:
				line, _ = strconv.Atoi(v.Value)
				hasLine = true
			}
		case *ast.CompositeLit:
			if key.Name != "Args" {
				continue
			}
			for _, a := range v.Elts {
				if s, isLit := a.(*ast.BasicLit); isLit && s.Kind == token.STRING {
					text, _ := strconv.Unquote(s.Value)
					args = append(args, text)
				}
			}
		}
	}
	return file, line, args, hasLine && strings.HasSuffix(file, ".gp")
}

// keywordAfterDot matches a Go keyword used as a C++ name after a dot
// (std.map, node.type()); gocppToGo writes it in upper case, which keeps its
// length.
var keywordAfterDot = regexp.MustCompile(`\.(break|case|chan|const|continue|default|defer|else|fallthrough|for|func|go|goto|if|import|interface|map|package|range|return|select|struct|switch|type|var)\b`)

// gocppSyntax maps the rest of the gocpp-only syntax to Go of the same shape.
var gocppSyntax = []struct {
	re   *regexp.Regexp
	repl string
}{
	// A macro declaration at package level becomes a func.
	{regexp.MustCompile(`(?m)^macro (\w)`), "func $1"},
	// A lazy parameter becomes a plain one.
	{regexp.MustCompile(`(\(|, )(\w+) lazy `), "$1$2 "},
	// A macro on top of a declaration becomes a comment.
	{regexp.MustCompile(`(?m)^#(\w.*)$`), "// ONTOP $1"},
	// name!( and name![ become one name: the letter ǃ stands for the !.
	{regexp.MustCompile(`(\w)!([(\[])`), "${1}ǃ$2"},
	// #Name inside code becomes one name, with the letter ǂ.
	{regexp.MustCompile(`#([A-Za-z_])`), "ǂ$1"},
}

// gocppToGo maps gocpp-only syntax to Go of the same shape, like
// spikes/2026-10-13/tools/gpfmtcheck.py, so gofmt can check the layout. It
// covers the three Phase 01 parser patches (D3, D4, D20) and the Phase 10
// syntax (!, #, macro, lazy). Not covered yet: a member type of a template
// instance used as a type (std.vector[int].iterator), and a type argument
// that is a constant expression other than a decimal number.
func gocppToGo(src []byte) []byte {
	src = keywordAfterDot.ReplaceAllFunc(src, bytes.ToUpper)
	for _, s := range gocppSyntax {
		src = s.re.ReplaceAll(src, []byte(s.repl))
	}
	return mapCppNames(src)
}

// mapCppNames maps the C++ names that the stock parser rejects in a type:
// a number among the type arguments (D4: std.array[int, 5]) becomes a name
// of the same width (std.array[int, N]), and in a run of three or more
// names joined by dots (D20: std.chrono.seconds) every dot after the first
// becomes _ (std.chrono_seconds). Both are Go of the same shape anywhere,
// also where the stock parser already accepts them (a[5], a.b.c). Strings
// and comments are not changed.
func mapCppNames(src []byte) []byte {
	type tok struct {
		off int // byte offset in src
		tok token.Token
		lit string
	}
	file := token.NewFileSet().AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, nil, 0) // no error handler: gofmt reports the errors later
	var toks []tok
	for {
		pos, t, lit := s.Scan()
		if t == token.EOF {
			break
		}
		toks = append(toks, tok{file.Offset(pos), t, lit})
	}
	out := bytes.Clone(src)
	var open []token.Token // the brackets that are open
	for i, t := range toks {
		switch t.tok {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			open = append(open, t.tok)
		case token.RPAREN, token.RBRACK, token.RBRACE:
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
		case token.INT:
			inBrackets := len(open) > 0 && open[len(open)-1] == token.LBRACK
			if inBrackets && isDecimal(t.lit) && i > 0 && i+1 < len(toks) &&
				(toks[i-1].tok == token.LBRACK || toks[i-1].tok == token.COMMA) &&
				(toks[i+1].tok == token.RBRACK || toks[i+1].tok == token.COMMA) {
				copy(out[t.off:], bytes.Repeat([]byte("N"), len(t.lit)))
			}
		case token.PERIOD:
			// name.name.name, with no space around the dots: this dot is not the
			// first one of the run.
			if i >= 3 && i+1 < len(toks) &&
				toks[i-3].tok == token.IDENT && toks[i-2].tok == token.PERIOD &&
				toks[i-1].tok == token.IDENT && toks[i+1].tok == token.IDENT &&
				toks[i-1].off+len(toks[i-1].lit) == t.off && toks[i+1].off == t.off+1 &&
				toks[i-2].off+1 == toks[i-1].off {
				out[t.off] = '_'
			}
		}
	}
	return out
}

func isDecimal(lit string) bool {
	for _, c := range lit {
		if c < '0' || c > '9' {
			return false
		}
	}
	return lit != ""
}

// firstDiff describes the first line where want and got differ.
func firstDiff(want, got []byte) string {
	w, g := splitLines(want), splitLines(got)
	for i := range max(len(w), len(g)) {
		switch {
		case i >= len(w):
			return fmt.Sprintf("line %d: unexpected %q", i+1, g[i])
		case i >= len(g):
			return fmt.Sprintf("line %d: missing %q", i+1, w[i])
		case w[i] != g[i]:
			return fmt.Sprintf("line %d: want %q, got %q", i+1, w[i], g[i])
		}
	}
	return "no difference"
}

// splitLines splits b after each newline. A final newline doesn't start
// another line.
func splitLines(b []byte) []string {
	lines := strings.SplitAfter(string(b), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func indent(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	s := strings.TrimRight(string(b), "\n")
	return "    " + strings.ReplaceAll(s, "\n", "\n    ") + "\n"
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
