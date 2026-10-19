package gen

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

// One file set and one importer for all programs: the importer type-checks fmt (and what it
// imports) from source once, and keeps the result.
var (
	fset = token.NewFileSet()
	imp  = importer.ForCompiler(fset, "source", nil)
)

// goCheck type-checks the program with go/types (the real one, with the real fmt) and returns
// the errors.
func goCheck(src []byte) []string {
	f, err := parser.ParseFile(fset, "gen.go", src, parser.SkipObjectResolution)
	if err != nil {
		return []string{err.Error()}
	}
	var errs []string
	conf := types.Config{
		Importer:  imp,
		GoVersion: "go1.25",
		Error:     func(err error) { errs = append(errs, err.Error()) },
	}
	conf.Check("main", fset, []*ast.File{f}, nil)
	return errs
}

func report(t *testing.T, what string, src []byte, errs []string) {
	t.Helper()
	lines := strings.Split(string(src), "\n")
	var b strings.Builder
	for i, e := range errs {
		if i == 5 {
			break
		}
		fmt.Fprintln(&b, e)
		var n int
		if _, err := fmt.Sscanf(strings.TrimPrefix(e, "gen.go:"), "%d:", &n); err == nil && n >= 1 && n <= len(lines) {
			fmt.Fprintf(&b, "\t> %s\n", lines[n-1])
		}
	}
	t.Errorf("%s: %d errors:\n%s", what, len(errs), b.String())
}

// TestProgramsTypeCheck: every program is valid Go.
func TestProgramsTypeCheck(t *testing.T) {
	n := 2000
	if testing.Short() {
		n = 300
	}
	if v, err := strconv.Atoi(os.Getenv("GOCPP_GEN_N")); err == nil {
		n = v // a bigger sweep: GOCPP_GEN_N=50000
	}
	fails := 0
	for seed := uint64(1); seed <= uint64(n) && fails < 5; seed++ {
		src := FromSeed(seed, Default())
		if errs := goCheck(src); len(errs) > 0 {
			fails++
			report(t, fmt.Sprintf("seed %d", seed), src, errs)
		}
	}
}

// TestFromBytesEnds: any bytes, also none and a short or all-zero string, give a program that ends
// quickly and type-checks (a source that is used up must not make a loop that asks "again?" endless).
func TestFromBytesEnds(t *testing.T) {
	inputs := [][]byte{nil, {0}, make([]byte, 40), {1, 2, 3}, []byte("\xa8w\xaa\xab\xee\x00\xa05\x01\x1b;>ED\xaa\x19"), {255, 255, 255, 255}}
	for i, in := range inputs {
		done := make(chan []byte, 1)
		go func() { done <- FromBytes(in, Default()) }()
		select {
		case src := <-done:
			if errs := goCheck(src); len(errs) > 0 {
				report(t, fmt.Sprintf("input %d", i), src, errs)
			}
		case <-time.After(30 * time.Second):
			t.Fatalf("input %d: the generator does not end", i)
		}
	}
}

// The constructs of Options.Extras must show up: a generator that silently stops writing one is a
// loss of coverage that nothing else notices.
func TestGeneratorCoversConstructs(t *testing.T) {
	markers := map[string]*regexp.Regexp{
		"multi-value forwarding":    regexp.MustCompile(`\bf\d+\(f\d+\(`),
		"variadic forwarding":       regexp.MustCompile(`\bf\d+\(f\d+\([^()]*\)\)`),
		"iota constants":            regexp.MustCompile(`= iota`),
		"shifted iota":              regexp.MustCompile(`1 << \(3 \* iota\)`),
		"shift by a variable":       regexp.MustCompile(`(<<|>>)=? \(`),
		"shift assignment":          regexp.MustCompile(`(<<=|>>=) `),
		"1 << n conversion":         regexp.MustCompile(`\w+\(1 << `),
		"rune literals":             regexp.MustCompile(`'(\\.[0-9a-fA-F]*|[^'\\])'`),
		"three-index slices":        regexp.MustCompile(`\w\[\d*:[^\]:]+:[^\]:]+\]`),
		"named slice conversions":   regexp.MustCompile(`MyBytes\d+[({]`),
		"named map type":            regexp.MustCompile(`MyMap\d+\{\}`),
		"method expressions":        regexp.MustCompile(`(?m):= (\(\*)?[TNI]\d+\)?\.m\d+$`),
		"anonymous structs":         regexp.MustCompile(`struct\{ f0 `),
		"struct tags":               regexp.MustCompile("`db:\"x\\d+,opt\" gocpp:\"x\\d+\"`"),
		"goto":                      regexp.MustCompile(`goto G\d+`),
		"string of a rune":          regexp.MustCompile(`string\(\(?(\w|')`),
		"typed iota constants":      regexp.MustCompile(`c\d+ N\d+ = iota`),
		"closure of forwarded call": regexp.MustCompile(`= func\(p\d+ `),
	}
	const n = 300
	count := map[string]int{}
	for seed := uint64(1); seed <= n; seed++ {
		src := string(FromSeed(seed, Default()))
		for name, rx := range markers {
			if rx.MatchString(src) {
				count[name]++
			}
		}
	}
	for name := range markers {
		if count[name] < 10 {
			t.Errorf("%s: in %d of %d programs, want at least 10", name, count[name], n)
		}
	}
}

// The real Go compiler accepts the programs (go build: the type checker of the gc compiler and
// its other checks, goto and labels, unused variables and imports; go vet's analyzers are not
// asked: self-assignments and the like are written on purpose). GOCPP_GEN_VET=n programs (40).
func TestGeneratedGoCompiles(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go command")
	}
	n := 40
	if v, err := strconv.Atoi(os.Getenv("GOCPP_GEN_VET")); err == nil {
		n = v
	}
	if testing.Short() {
		n = 10
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module gentest\n\ngo 1.25\n"), 0o644)
	for seed := 1; seed <= n; seed++ {
		dir := filepath.Join(root, fmt.Sprintf("p%d", seed))
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "main.go"), FromSeed(uint64(seed), Default()), 0o644)
	}
	cmd := exec.Command(goBin, "build", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		if len(out) > 3000 {
			out = out[:3000]
		}
		t.Errorf("go build: %v\n%s", err, out)
	}
}
