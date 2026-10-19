package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The golden tests of the diagnostics (step 13.1): every .gp file or folder (with a main.gp) in
// testdata/diag is a program with errors, and its golden file holds the exact text that gocpp
// prints: the source excerpts with the carets, the C++ errors with gocpp type names and the
// notes. A program that has C or C++ code has one golden file per compiler, because gcc and
// clang word their errors differently (name.g++.golden, name.clang++.golden); the others have
// one, name.golden. The first line of a program may be
//
//	//test: cmd=check color=always excerpt=never cpperrors=full crlf=1 env=NAME=value,NAME=value
//
// (the defaults: cmd=build, -color=never, -excerpt=always). GOCPP_UPDATE_DIAG=1 rewrites the golden files.
// The program is copied to a temporary folder as main.gp, so the reports say ./main.gp.

var updateDiag = os.Getenv("GOCPP_UPDATE_DIAG") == "1" // rewrite the golden files

// A diagCase is one program of testdata/diag.
type diagCase struct {
	name string
	root string // the absolute path of testdata/diag
	dir  string // the folder with main.gp, or "" for the file name.gp
	cxx  bool   // the program has C or C++ code: golden files per compiler
	opts map[string]string
}

var cxxImportRx = regexp.MustCompile(`(?m)^\s*(?:import\s+)?(?:\w+\s+)?"(?:<[^"]*>|[^"]*\.h)"`)

func diagCases(t *testing.T) []diagCase {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("testdata", "diag"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var cases []diagCase
	for _, e := range entries {
		var c diagCase
		var main string
		switch {
		case e.IsDir():
			c = diagCase{name: e.Name(), root: root, dir: filepath.Join(root, e.Name())}
			main = filepath.Join(c.dir, "main.gp")
		case strings.HasSuffix(e.Name(), ".gp"):
			c = diagCase{name: strings.TrimSuffix(e.Name(), ".gp"), root: root}
			main = filepath.Join(root, e.Name())
		default:
			continue
		}
		src, err := os.ReadFile(main)
		if err != nil {
			t.Fatal(err)
		}
		c.cxx = cxxImportRx.Match(src)
		c.opts = map[string]string{}
		if first, _, _ := strings.Cut(string(src), "\n"); strings.HasPrefix(first, "//test:") {
			for _, f := range strings.Fields(strings.TrimPrefix(first, "//test:")) {
				k, v, _ := strings.Cut(f, "=")
				c.opts[k] = v
			}
		}
		cases = append(cases, c)
	}
	return cases
}

// runDiagCase runs the program of c in a temporary folder and returns what gocpp prints on
// standard error.
func runDiagCase(t *testing.T, c diagCase) string {
	t.Helper()
	dir := t.TempDir()
	if c.dir != "" {
		copyTree(t, c.dir, dir)
	} else {
		src, err := os.ReadFile(filepath.Join(c.root, c.name+".gp"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		if c.opts["crlf"] == "1" { // stored with "\n": the repository's format tests want LF files
			text = strings.ReplaceAll(text, "\n", "\r\n")
		}
		writeFiles(t, dir, map[string]string{"main.gp": text})
	}
	t.Chdir(dir)
	t.Setenv("GOCPP_GATE_PHASE", "08")
	for _, k := range []string{"NO_COLOR", "CLICOLOR_FORCE", "GOCPP_COLOR", "GOCPP_EXCERPT"} {
		t.Setenv(k, "")
	}
	for _, kv := range strings.Split(c.opts["env"], ",") {
		if k, v, ok := strings.Cut(kv, "="); ok {
			t.Setenv(k, v)
		}
	}
	color, excerpt := "never", "always"
	if v := c.opts["color"]; v != "" {
		color = v
	}
	if v := c.opts["excerpt"]; v != "" {
		excerpt = v
	}
	args := []string{c.opts["cmd"]}
	if args[0] == "" {
		args[0] = "build"
	}
	if c.opts["env"] == "" || c.opts["color"] != "" {
		args = append(args, "-color="+color)
	}
	args = append(args, "-excerpt="+excerpt)
	if args[0] == "build" {
		args = append(args, "-o", "out.bin")
		if v := c.opts["cpperrors"]; v != "" {
			args = append(args, "-cpperrors="+v)
		}
	}
	args = append(args, ".")
	r := gocppRun("", false, args...)
	if r.code != 1 {
		t.Fatalf("gocpp %v = %+v, want exit 1", args, r)
	}
	out := strings.ReplaceAll(r.stderr, dir, "$DIR")
	return out
}

// copyTree copies the folder src to dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// checkGolden compares got with the golden file (or rewrites it with GOCPP_UPDATE_DIAG=1).
func checkGolden(t *testing.T, file, got string) {
	t.Helper()
	if updateDiag {
		if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v (run with GOCPP_UPDATE_DIAG=1)", err)
	}
	if got != string(want) {
		t.Errorf("%s:\n--- got\n%s--- want\n%s", file, got, want)
	}
}

func TestDiagGolden(t *testing.T) {
	cases := diagCases(t)
	if len(cases) < 40 {
		t.Fatalf("only %d programs in testdata/diag, want at least 40", len(cases))
	}
	for _, c := range cases {
		golden := filepath.Join(c.root, c.name)
		if !c.cxx {
			t.Run(c.name, func(t *testing.T) {
				out := runDiagCase(t, c)
				if c.opts["cpperrors"] == "full" {
					return
				}
				checkGolden(t, golden+".golden", out)
			})
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, cc testCXX) {
				out := runDiagCase(t, c)
				if c.opts["cpperrors"] == "full" {
					// The compiler's own text, with its machine paths: only what makes it raw.
					if !strings.Contains(out, "gp::slice<") || !strings.Contains(out, "error: ") {
						t.Errorf("-cpperrors=full output is not the compiler's:\n%s", out)
					}
					return
				}
				checkGolden(t, golden+"."+filepath.Base(cc.name)+".golden", out)
			})
		})
	}
}

// gocppTypeRx matches a gocpp type name in a C++ error: a slice, map, pointer, channel or
// function type, or a type of a package.
var gocppTypeRx = regexp.MustCompile(`C\+\+: .*'(?:\*|\[\d*\]|map\[|chan |func\(|string'|any'|error'|[a-z][a-z0-9]*\.[A-Z])`)

// TestDiagGocppNames is the exit criterion of Phase 13: at least 20 error tests show C++ errors
// with gocpp type names, for each compiler, and none shows the runtime's C++ names.
func TestDiagGocppNames(t *testing.T) {
	for _, cc := range []string{"g++", "clang++"} {
		files, _ := filepath.Glob(filepath.Join("testdata", "diag", "*."+cc+".golden"))
		n := 0
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			text := string(b)
			if strings.Contains(text, "\x1b[") {
				text = regexp.MustCompile("\x1b\\[[0-9;]*m").ReplaceAllString(text, "")
			}
			if gocppTypeRx.MatchString(text) {
				n++
			}
			for _, bad := range []string{"gp::slice", "gp::map", "gp::ptr", "gp::string", "gpkg_", "gp::any", "gp::error", "gp_anon"} {
				if strings.Contains(text, bad) {
					t.Errorf("%s shows the C++ name %s", f, bad)
				}
			}
		}
		if n < 20 {
			t.Errorf("%s: %d golden files show gocpp type names, want at least 20", cc, n)
		}
	}
}

// TestDiagGoTypesExact checks that the programs whose Go type shares its C++ type with another
// (int and int64, byte and uint8, the three directions of a channel) show the type that the
// program has: a wrong "int" for an int64 fails. The golden files hold the text, this holds the
// meaning.
func TestDiagGoTypesExact(t *testing.T) {
	want := map[string]string{
		"names_slice_int":           "'[]int'",
		"names_slice_int64":         "'[]int64'",
		"names_slice_uint":          "'[]uint'",
		"names_slice_uint64":        "'[]uint64'",
		"names_slice_uintptr":       "'[]uintptr'",
		"names_slice_rune":          "'[]rune'",
		"names_slice_int32":         "'[]int32'",
		"names_slice_uint8":         "'[]uint8'",
		"names_slice_byte":          "'[]byte'",
		"names_chan_int":            "'chan int'",
		"names_chan_send":           "'chan<- int'",
		"names_chan_send_int64":     "'chan<- int64'",
		"names_chan_recv":           "'<-chan int64'",
		"names_slice_chan_dir":      "'[]<-chan int64'",
		"names_map_string_int64":    "'map[string]int64'",
		"names_map_string_int":      "'map[string]int'",
		"names_func_int64":          "'func(int64, []uint64) int64'",
		"names_ambiguous_int_int64": "'[]int64_t'", // the line has []int and []int64: the C++ name, not a guess
	}
	for name, text := range want {
		for _, cc := range []string{"g++", "clang++"} {
			b, err := os.ReadFile(filepath.Join("testdata", "diag", name+"."+cc+".golden"))
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			if !strings.Contains(string(b), text) {
				t.Errorf("%s.%s.golden does not show %s:\n%s", name, cc, text, b)
			}
		}
	}
}

// TestDiagNoWrongQualification: a member, a suggestion or a C++ type that is named like a type of
// the package is not shown as that type (main.push_bak, main.Node).
func TestDiagNoWrongQualification(t *testing.T) {
	for _, name := range []string{"local_member_push_bak", "local_member_size", "local_member_value_map", "local_node_clash"} {
		for _, cc := range []string{"g++", "clang++"} {
			b, err := os.ReadFile(filepath.Join("testdata", "diag", name+"."+cc+".golden"))
			if err != nil {
				t.Errorf("%v", err)
				continue
			}
			for _, m := range qualifiedNameRx.FindAllString(string(b), -1) {
				if m != "main.gp" {
					t.Errorf("%s.%s.golden qualifies a name with %s:\n%s", name, cc, m, b)
				}
			}
		}
	}
}

var qualifiedNameRx = regexp.MustCompile(`\bmain\.[A-Za-z_][A-Za-z0-9_]*`)

// TestDiagCTypeKeepsName: a C or C++ type that is named like a type of the package keeps its name:
// the message never says that something converts to main.Node, main.Item or main.Point when the
// target is the header's type, and a bare name of clang is not read as the package's.
func TestDiagCTypeKeepsName(t *testing.T) {
	for _, name := range []string{"local_node_same_line", "local_clash_param", "local_clash_template"} {
		for _, cc := range []string{"g++", "clang++"} {
			b, err := os.ReadFile(filepath.Join("testdata", "diag", name+"."+cc+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			text := string(b)
			for _, bad := range []string{"to 'main.Node'", "to 'main.Node*'", "to 'main.Node *'", "'const main.Node &'", "main.Node &&", "to 'main.Item'", "to 'main.Point'", "from 'int' to 'main.", "main.Node<"} {
				if strings.Contains(text, bad) {
					t.Errorf("%s.%s: the C type is shown as the package's (%s):\n%s", name, cc, bad, text)
				}
			}
		}
	}
	for _, cc := range []string{"g++", "clang++"} {
		b, err := os.ReadFile(filepath.Join("testdata", "diag", "local_node_same_line."+cc+".golden"))
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"main.Node", "to 'Node"} {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s: missing %q:\n%s", cc, want, b)
			}
		}
	}
}
