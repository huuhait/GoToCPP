package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/explain"
)

// TestExplainTopicsGolden: gocpp explain TOPIC for every topic is its golden file: the text, the links and
// the live example that the real compiler emits (with the gates open, so the output does not
// change when a phase is finished). -update-annotate rewrites the goldens.
func TestExplainTopicsGolden(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	for _, tp := range explain.Topics() {
		t.Run(tp.Name, func(t *testing.T) {
			r := gocppRun("", false, "explain", tp.Name)
			if r.code != 0 || r.stderr != "" {
				t.Fatalf("gocpp explain %s = %d %q", tp.Name, r.code, r.stderr)
			}
			golden := filepath.Join(pkgDir(t), "testdata", "explain", tp.Name+".golden")
			if *updateAnnotate {
				if err := os.WriteFile(golden, []byte(r.stdout), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (-update-annotate writes it)", err)
			}
			if r.stdout != string(want) {
				t.Errorf("gocpp explain %s differs from %s:\n%s", tp.Name, golden, firstDiff(string(want), r.stdout))
			}
		})
	}
}

// TestExplainLiveExampleIsEmit: the C++ in the live example of a topic is what gocpp emit -annotate
// writes for the example program (the "real compiler" rule): the text after the source, minus the
// four-space indent, equals the view without its banner and without the pkg_init file.
func TestExplainLiveExampleIsEmit(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	for _, tp := range explain.Topics() {
		t.Run(tp.Name, func(t *testing.T) {
			r := gocppRun("", false, "explain", tp.Name)
			_, after, ok := strings.Cut(r.stdout, "The C++ that gocpp emit -annotate writes for it:\n\n")
			if !ok {
				t.Fatalf("no live example:\n%s", r.stdout)
			}
			var got []string
			for _, l := range strings.Split(strings.TrimRight(after, "\n"), "\n") {
				got = append(got, strings.TrimPrefix(l, "    "))
			}
			withExample(t, strings.TrimSuffix(tp.Example, ".gp"))
			ann := gocppRun("", false, "emit", "-annotate", tp.Example)
			view := strings.SplitN(ann.stdout, "\n", 3)[2] // without the banner
			view, _, _ = strings.Cut(view, "//// gpkg/command-line-arguments/pkg_init.cpp\n")
			if strings.TrimRight(view, "\n") != strings.Join(got, "\n") {
				t.Errorf("the example of %s is not the emit:\n%s", tp.Name, firstDiff(view, strings.Join(got, "\n")))
			}
		})
	}
}

// TestExplainGatedExample: the example of a topic of a later phase says that its gates are open.
func TestExplainGatedExample(t *testing.T) {
	r := gocppRun("", false, "explain", "channels")
	if r.code != 0 {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(r.stdout, "gocpp build rejects this program until Phase 08") {
		t.Errorf("no note about the gates:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, "gp::make_chan") {
		t.Errorf("no C++ for the example:\n%s", r.stdout)
	}
	// A topic of a finished phase has no note.
	if r := gocppRun("", false, "explain", "boxing"); strings.Contains(r.stdout, "gocpp build rejects") {
		t.Errorf("boxing has a note about the gates")
	}
	t.Setenv(gatePhaseEnv, "08")
	if r := gocppRun("", false, "explain", "channels"); strings.Contains(r.stdout, "gocpp build rejects") {
		t.Errorf("a gocpp with the gates open still has the note")
	}
}

func TestExplainList(t *testing.T) {
	r := gocppRun("", false, "explain", "-list")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("%+v", r)
	}
	for _, tp := range explain.Topics() {
		if !regexp.MustCompile(`(?m)^` + tp.Name + ` +` + regexp.QuoteMeta(tp.Summary) + `$`).MatchString(r.stdout) {
			t.Errorf("-list has no line for %s", tp.Name)
		}
	}
	j := gocppRun("", false, "explain", "-list", "-json")
	var items []struct{ Name, Title, Summary string }
	if err := json.Unmarshal([]byte(j.stdout), &items); err != nil || len(items) != len(explain.Topics()) {
		t.Fatalf("-list -json: %v %d\n%s", err, len(items), j.stdout)
	}
	m := gocppRun("", false, "explain", "-list", "-messages")
	if m.code != 0 || !strings.Contains(m.stdout, "interfaces are not supported yet (Phase 05)\n") || !strings.Contains(m.stdout, "%s.%s uses C/C++ names, so it can only be instantiated in its own package (Phase 12)\n") {
		t.Errorf("-list -messages:\n%s", m.stdout)
	}
	mj := gocppRun("", false, "explain", "-list", "-messages", "-json")
	var ms []struct{ ID, Source, Message string }
	if err := json.Unmarshal([]byte(mj.stdout), &ms); err != nil || len(ms) != len(explain.Messages()) {
		t.Fatalf("-list -messages -json: %v", err)
	}
}

func TestExplainJSON(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	r := gocppRun("", false, "explain", "defer", "-json")
	var v struct {
		Name    string
		Text    []string
		See     []string
		Example struct{ Source, Cpp string }
	}
	if err := json.Unmarshal([]byte(r.stdout), &v); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	if v.Name != "defer" || len(v.Text) < 2 || len(v.See) == 0 || !strings.Contains(v.Example.Cpp, "gp_df.gp_push_call") || !strings.Contains(v.Example.Source, "defer func()") {
		t.Errorf("%+v", v)
	}
	m := gocppRun("", false, "explain", "-json", "./main.gp:5:12: interfaces are not supported yet (Phase 05)")
	var mv struct {
		Message string
		Matches []struct{ ID, Fix, Topic string }
	}
	if err := json.Unmarshal([]byte(m.stdout), &mv); err != nil || len(mv.Matches) != 1 || mv.Matches[0].Topic != "interfaces" || mv.Message != "interfaces are not supported yet (Phase 05)" {
		t.Errorf("%v %+v", err, mv)
	}
}

func TestExplainDeterministic(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	first := gocppRun("", false, "explain", "generics")
	for range 3 {
		if again := gocppRun("", false, "explain", "generics"); again != first {
			t.Fatalf("two runs differ:\n%s", firstDiff(first.stdout, again.stdout))
		}
	}
}

func TestExplainErrors(t *testing.T) {
	tests := []struct {
		args   []string
		code   int
		stderr string // a part
	}{
		{[]string{"defr"}, 2, "unknown topic \"defr\". Did you mean: defer?"},
		{[]string{"zzzzzzzz"}, 2, "unknown topic \"zzzzzzzz\"\nRun 'gocpp explain -list' for the topics."},
		{[]string{"-list", "x"}, 2, "-list takes no arguments"},
		{[]string{"-messages"}, 2, "-messages needs -list"},
		{[]string{"-nochecks", "boxing"}, 2, "-nochecks needs file.gp:LINE"},
		{[]string{}, 2, "usage: gocpp explain"},
		{[]string{"no", "such", "message"}, 2, "no explanation for the message \"no such message\""},
		{[]string{"nofile.gp:3"}, 2, "nofile.gp"},
		{[]string{"-bogus"}, 2, "flag provided but not defined: -bogus"},
		{[]string{""}, 2, "no topic given"},
		{[]string{"  "}, 2, "no topic given"},
		{[]string{"defer", "boxing"}, 2, "is 2 topics: gocpp explain takes one at a time (gocpp explain defer; gocpp explain boxing)"},
		{[]string{"geom/geom.gp"}, 2, "geom/geom.gp needs a line: gocpp explain geom/geom.gp:LINE"},
		{[]string{"main.go"}, 2, "main.go needs a line"},
	}
	for _, tc := range tests {
		r := gocppRun("", false, append([]string{"explain"}, tc.args...)...)
		if r.code != tc.code || r.stdout != "" || !strings.Contains(r.stderr, tc.stderr) {
			t.Errorf("gocpp explain %v = %+v, want exit %d and %q", tc.args, r, tc.code, tc.stderr)
		}
	}
	if r := gocppRun("", false, "explain", "-h"); r.code != 0 || !strings.Contains(r.stderr, "usage: gocpp explain") {
		t.Errorf("explain -h = %+v", r)
	}
	withExample(t, "boxing")
	for _, arg := range []string{"boxing.gp:0", "boxing.gp:5-3", "boxing.gp:999"} {
		r := gocppRun("", false, "explain", arg)
		if r.code != 2 || r.stdout != "" || r.stderr == "" {
			t.Errorf("gocpp explain %s = %+v", arg, r)
		}
	}
}

// TestExplainMessage: the messages that the compiler prints are explained.
func TestExplainMessage(t *testing.T) {
	r := gocppRun("", false, "explain", "./main.gp:5:12: interfaces are not supported yet (Phase 05)")
	if r.code != 0 || !strings.Contains(r.stdout, "Phase 05 (interfaces and") || !strings.Contains(r.stdout, "gocpp explain interfaces") {
		t.Errorf("%+v", r)
	}
	// The words of an unquoted message are joined.
	u := gocppRun("", false, "explain", "main.gp:5:12:", "interfaces", "are", "not", "supported", "yet", "(Phase", "05)")
	if u != r {
		t.Errorf("an unquoted message differs:\n%s", u.stdout)
	}
}

// TestExplainEveryRealGateMessage: for each phase gate that gocpp check really prints, explain says
// something (the table is made from the same rows, this checks the whole chain with a real program).
func TestExplainEveryRealGateMessage(t *testing.T) {
	progs := map[string]string{
		"go statements":    "package main\n\nfunc main() {\n\tgo func() {}()\n}\n",
		"generic types":    "package main\n\ntype S[T any] struct{ x T }\n\nfunc main() {}\n",
		"select":           "package main\n\nfunc main() {\n\tselect {}\n}\n",
		"package net/http": "package main\n\nimport \"net/http\"\n\nfunc main() { _ = http.StatusOK }\n",
		"must calls":       "package main\n\nfunc f() (int, error) { return 1, nil }\n\nfunc main() {\n\tx := f()!\n\t_ = x\n}\n",
	}
	for name, src := range progs {
		t.Run(name, func(t *testing.T) {
			inDir(t, map[string]string{"main.gp": src})
			c := gocppRun("", false, "check", "main.gp")
			var gate string
			for _, l := range strings.Split(c.stderr, "\n") {
				if strings.Contains(l, "(Phase") || strings.Contains(l, "not supported in gocpp") {
					gate = l
					break
				}
			}
			if gate == "" {
				t.Skipf("gocpp check prints no gate for %s any more:\n%s", name, c.stderr)
			}
			r := gocppRun("", false, "explain", gate)
			if r.code != 0 || !strings.Contains(r.stdout, "What to do:") {
				t.Errorf("gocpp explain %q = %+v", gate, r)
			}
		})
	}
}

// TestExplainLine: gocpp explain file.gp:LINE.
func TestExplainLine(t *testing.T) {
	withExample(t, "defer")
	r := gocppRun("", false, "explain", "defer.gp:9-12")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("%+v", r)
	}
	for _, want := range []string{"// ---- ./defer.gp:9: defer func() {\n", "// gp[defer] col 2: defer:", "gp_df.gp_push_call(", "// ---- ./defer.gp:11:", "(*err) = gp::fmt::Errorf(", "gocpp explain closures, gocpp explain defer, gocpp explain interfaces explains the mechanisms\n"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("explain defer.gp:9-12 lacks %q:\n%s", want, r.stdout)
		}
	}
	if strings.Contains(r.stdout, "#line") || strings.Contains(r.stdout, "defer.gp:5:") {
		t.Errorf("lines outside the range or #line directives:\n%s", r.stdout)
	}
	// One line.
	one := gocppRun("", false, "explain", "defer.gp:9")
	if one.code != 0 || !strings.Contains(one.stdout, "gp_df.gp_push_call(") || strings.Contains(one.stdout, "// ---- defer.gp:10") {
		t.Errorf("explain defer.gp:9:\n%s", one.stdout)
	}
	// A line without C++ of its own says so.
	blank := gocppRun("", false, "explain", "defer.gp:2")
	if blank.code != 0 || !strings.Contains(blank.stdout, "no C++ comes from this line") {
		t.Errorf("a blank line:\n%s", blank.stdout)
	}
	// -nochecks, and the JSON.
	withExample(t, "checks")
	n := gocppRun("", false, "explain", "-nochecks", "checks.gp:1-40")
	p := gocppRun("", false, "explain", "checks.gp:1-40")
	if !strings.Contains(n.stdout, "removed by -nochecks") || strings.Contains(p.stdout, "removed by -nochecks") {
		t.Errorf("-nochecks:\n%s\n%s", p.stdout, n.stdout)
	}
	j := gocppRun("", false, "explain", "-json", "checks.gp:1-40")
	var v struct {
		File   string
		First  int
		Last   int
		Chunks []struct {
			GP      struct{ File string }
			Reasons []struct{ Kind string }
			Cpp     []string
		}
	}
	if err := json.Unmarshal([]byte(j.stdout), &v); err != nil || v.File != "./checks.gp" || v.First != 1 || v.Last != 14 || len(v.Chunks) == 0 {
		t.Fatalf("%v\n%s", err, j.stdout)
	}
	if f := v.Chunks[0].GP.File; f != "./checks.gp" {
		t.Errorf("the JSON names the file %q, want ./checks.gp (not an absolute path)", f)
	}
	// An error of the program is the compiler's.
	inDir(t, map[string]string{"bad.gp": "package main\n\nfunc main() { x := 1 }\n"})
	if e := gocppRun("", false, "explain", "bad.gp:3"); e.code != 1 || !strings.Contains(e.stderr, "declared and not used") {
		t.Errorf("a program with an error: %+v", e)
	}
}

// TestGuideExamples: the output blocks of docs/guides/explain.md are what gocpp prints now. A block is
// `<!-- gocpp: EXAMPLE ARGS -->` followed by a fenced block: the command's output, run in a folder
// that has the example EXAMPLE.gp. When ARGS start with the word stderr, the block is what the
// command writes to standard error (the diagnostics of -gp:m) and the rest of ARGS is the command.
func TestGuideExamples(t *testing.T) {
	path := filepath.Join(pkgDir(t), "..", "..", "docs", "guides", "explain.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rx := regexp.MustCompile("(?s)<!-- gocpp: (\\S+) ([^>]*?) -->\n```[a-z]*\n(.*?)```")
	blocks := rx.FindAllStringSubmatchIndex(string(data), -1)
	if len(blocks) < 3 {
		t.Fatalf("%d output blocks in docs/guides/explain.md, want at least 3", len(blocks))
	}
	text := string(data)
	var out strings.Builder
	last := 0
	for _, b := range blocks {
		example, args := text[b[2]:b[3]], strings.Fields(text[b[4]:b[5]])
		fromStderr := len(args) > 0 && args[0] == "stderr"
		if fromStderr {
			args = args[1:]
		}
		withExample(t, example)
		r := gocppRun("", false, args...)
		if r.code != 0 {
			t.Errorf("gocpp %s: %+v", strings.Join(args, " "), r)
		}
		shown := r.stdout
		if fromStderr {
			shown = r.stderr
		}
		got := strings.TrimRight(shown, "\n") + "\n"
		if !*updateAnnotate && got != text[b[6]:b[7]] {
			t.Errorf("docs/guides/explain.md: the output of `gocpp %s` is not what it prints now (-update-annotate rewrites the guide):\n%s", strings.Join(args, " "), firstDiff(text[b[6]:b[7]], got))
		}
		out.WriteString(text[last:b[6]] + got)
		last = b[7]
	}
	out.WriteString(text[last:])
	if *updateAnnotate {
		if err := os.WriteFile(path, []byte(out.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestExplainCommandTable: the command is built and says what it does.
func TestExplainCommandTable(t *testing.T) {
	c := lookup("explain")
	if c == nil || c.run == nil || c.phase != 13 {
		t.Fatalf("%+v", c)
	}
	if r := gocppRun("", false, "help"); !strings.Contains(r.stdout, "explain") || strings.Contains(r.stdout, "explain a construct, a .gp line or an error message (Phase") {
		t.Errorf("help:\n%s", r.stdout)
	}
}

// TestExplainReferenceTopics: the reference topics (the types, arrays, multiple results, complex
// numbers, named types, fmt, the files and #line) are found by name, by alias and by the words of
// the spec section, also when the words are not quoted.
func TestExplainReferenceTopics(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	for args, want := range map[string]string{
		"types": "The type mapping", "arrays": "Arrays", "results": "Multiple results", "multiple results": "Multiple results",
		"complex numbers": "Complex numbers", "named types": "Named types", "fmt": "fmt and the builtins",
		"files": "The generated files", "linemap": "#line and debugging", "debugging": "#line and debugging",
	} {
		r := gocppRun("", false, append([]string{"explain"}, strings.Fields(args)...)...)
		if r.code != 0 || r.stderr != "" || !strings.Contains(r.stdout, want) || !strings.Contains(r.stdout, "The C++ that gocpp emit -annotate writes for it:") {
			t.Errorf("gocpp explain %s = %d %q, want %q", args, r.code, r.stderr, want)
		}
	}
	list := gocppRun("", false, "explain", "-list")
	for _, name := range []string{"types", "arrays", "results", "complex", "named", "fmt", "files", "linemap"} {
		if !strings.Contains(list.stdout, "\n"+name+" ") {
			t.Errorf("-list has no topic %s:\n%s", name, list.stdout)
		}
	}
}

// TestExplainLineOfALibrary: gocpp explain file.gp:LINE for a file of a package that is not main is
// the line of the program of the module that uses it, found from the gocpp.mod above the file.
func TestExplainLineOfALibrary(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	inDir(t, map[string]string{
		"gocpp.mod":    "module example.com/lib\n\ngocpp 0.1\ncxx c++20\n",
		"main.gp":      "package main\n\nimport (\n\t\"fmt\"\n\t\"example.com/lib/geom\"\n)\n\nfunc main() { fmt.Println(geom.Twice(4)) }\n",
		"geom/geom.gp": "package geom\n\n// Twice doubles n.\nfunc Twice(n int) int {\n\treturn n * 2\n}\n",
	})
	r := gocppRun("", false, "explain", "geom/geom.gp:5")
	if r.code != 0 || r.stderr != "" || !strings.Contains(r.stdout, "geom.gp:5: return n * 2") || !strings.Contains(r.stdout, "return n * 2;") {
		t.Errorf("explain geom/geom.gp:5 = %+v", r)
	}
	j := gocppRun("", false, "explain", "-json", "geom/geom.gp:4-5")
	if j.code != 0 || !strings.Contains(j.stdout, `"file": "geom/geom.gp"`) {
		t.Errorf("-json = %+v", j)
	}
	// A file of the main package of the same module still works.
	if m := gocppRun("", false, "explain", "main.gp:8"); m.code != 0 || !strings.Contains(m.stdout, "main.gp:8") {
		t.Errorf("explain main.gp:8 = %+v", m)
	}
	// Without a gocpp.mod there is no program to put the library in.
	inDir(t, map[string]string{"lib/lib.gp": "package lib\n\nfunc F() int { return 1 }\n"})
	e := gocppRun("", false, "explain", "lib/lib.gp:3")
	if e.code != 2 || !strings.Contains(e.stderr, "which is not a main package, and no gocpp.mod above it") {
		t.Errorf("a library with no module = %+v", e)
	}
}

// TestExplainJSONHasNoHTMLEscapes: the C++ and the messages are JSON as they are: < and > are not
// \u003c and \u003e.
func TestExplainJSONHasNoHTMLEscapes(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	for _, args := range [][]string{{"explain", "-json", "closures"}, {"explain", "-json", "-list"}, {"explain", "-json", "channels.gp:1-30"}} {
		if args[len(args)-1] == "channels.gp:1-30" {
			withExample(t, "channels")
		}
		r := gocppRun("", false, args...)
		if r.code != 0 || strings.Contains(r.stdout, `\u003`) || strings.Contains(r.stdout, `\u0026`) {
			t.Errorf("gocpp %v: %d, escapes in:\n%s", args, r.code, r.stdout)
		}
	}
	withExample(t, "closures")
	if r := gocppRun("", false, "emit", "-annotate", "-json", "closures.gp"); strings.Contains(r.stdout, `\u003`) || !strings.Contains(r.stdout, "<int = 1>") {
		t.Errorf("emit -annotate -json has escapes or lost the lambda's <int = 1>")
	}
}

// TestExplainLineOfAMultiFileMain: a file of a main package with several files is explained with the
// other files of its folder, which define what it calls.
func TestExplainLineOfAMultiFileMain(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	inDir(t, map[string]string{
		"main.gp": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tx := helper(2)\n\tp := &x\n\tfmt.Println(*p)\n}\n",
		"util.gp": "package main\n\nfunc helper(n int) int {\n\tm := map[int]int{}\n\tm[n]++\n\treturn m[n]\n}\n",
	})
	if r := gocppRun("", false, "explain", "main.gp:6"); r.code != 0 || !strings.Contains(r.stdout, "x := helper(2)") || !strings.Contains(r.stdout, "x stays on the C++ stack") {
		t.Errorf("explain main.gp:6 = %+v", r)
	}
	if r := gocppRun("", false, "explain", "util.gp:5"); r.code != 0 || !strings.Contains(r.stdout, "gp::ref(m, n)++;") {
		t.Errorf("explain util.gp:5 = %+v", r)
	}
	// A file that has an error of its own reports the error of the file.
	inDir(t, map[string]string{"a.gp": "package main\n\nfunc main() { x := 1 }\n", "b.gp": "package main\n\nfunc g() {}\n"})
	if r := gocppRun("", false, "explain", "a.gp:3"); r.code != 1 || !strings.Contains(r.stderr, "declared and not used") {
		t.Errorf("a broken file: %+v", r)
	}
}

// TestExplainTopicWithoutExample: a topic whose live example can't be made is an error, not a
// success with half an answer.
func TestExplainTopicWithoutExample(t *testing.T) {
	t.Setenv(gatePhaseEnv, "15")
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if r := gocppRun("", false, "explain", "defer"); r.code == 0 || !strings.Contains(r.stderr, "could not be made") {
		t.Errorf("explain defer with no temp folder = %+v", r)
	}
}

// TestExplainFileWithoutLineOnWindowsPath: a Windows path with a drive letter is a file argument that
// needs a line, not an unknown topic.
func TestExplainFileWithoutLineOnWindowsPath(t *testing.T) {
	for _, arg := range []string{`C:\x\y.gp`, `C:/x/y.gp`, `geom/geom.gp`} {
		if r := gocppRun("", false, "explain", arg); r.code != 2 || !strings.Contains(r.stderr, "needs a line") {
			t.Errorf("explain %s = %+v", arg, r)
		}
	}
}
