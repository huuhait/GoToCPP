package main

// The code blocks of the tour and of the guides are tests. A ```go block in these docs must be
// marked, on the line before it (blank lines between are allowed):
//
//	<!-- program: 08 id=name go exit=N check -->  a whole program (package main): it is written
//	      to a folder, checked with gocpp check at GOCPP_GATE_PHASE=08, and run; its output must
//	      be the ```text block that follows, marked <!-- output -->. The flags: id=name names it
//	      (for <!-- emitted: name -->), go says that it is valid Go and must print the same
//	      with the go command, exit=N is the exit status (default 0), check says that it is only
//	      checked and not run.
//	<!-- excerpt: examples/raylib/main.gp -->     lines of that file of the repository, in
//	      order; a line "..." (or "// ...") stands for the lines that are left out
//	<!-- illustration: reason -->                 a fragment, or code that can't run here
//
// A ```cpp block marked <!-- emitted: name --> shows lines of the C++ that gocpp emit prints for
// the program of that name (the same "..." rule, and #line directives and the indentation do
// not count). So nothing in these docs describes code that the compiler does not accept or
// generate.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/version"
)

// snippetDocs are the docs (relative to docs/, globs) whose blocks are tests.
var snippetDocs = []string{"index.md", "faq.md", "tour/*.md", "guides/go-developers.md", "guides/cpp-developers.md", "guides/interop-examples.md", "guides/windows.md", "guides/windows-cross.md", "guides/windows-interop.md"}

type snippet struct {
	doc    string
	line   int
	kind   string // program, excerpt, illustration, emitted
	phase  string
	id     string
	ref    string // the file of an excerpt, the program of an emitted block
	goOK   bool
	check  bool
	exit   int
	code   string
	output string
	hasOut bool
}

var markerRx = regexp.MustCompile(`^<!-- (program|excerpt|illustration|output|emitted|rejected)(?:: (.*?))? -->$`)

// parseSnippets reads the blocks of one doc; errors are reported with the line.
func parseSnippets(doc, src string, errorf func(format string, a ...any)) []*snippet {
	lines := strings.Split(src, "\n")
	var out []*snippet
	var last *snippet
	var pendKind, pendArgs string
	pendLine := 0
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if m := markerRx.FindStringSubmatch(l); m != nil {
			if pendKind != "" {
				errorf("%s:%d: the marker %q is not followed by a block", doc, pendLine, pendKind)
			}
			pendKind, pendArgs, pendLine = m[1], m[2], i+1
			continue
		}
		if !strings.HasPrefix(l, "```") {
			if strings.TrimSpace(l) != "" && pendKind != "" {
				errorf("%s:%d: the marker %q is not followed by a block", doc, pendLine, pendKind)
				pendKind = ""
			}
			continue
		}
		lang := strings.TrimSpace(strings.TrimPrefix(l, "```"))
		start := i + 1
		var body []string
		i++
		for i < len(lines) && lines[i] != "```" {
			body = append(body, lines[i])
			i++
		}
		code := strings.Join(body, "\n")
		switch {
		case lang == "go":
			s := &snippet{doc: doc, line: start - 1 + 0, kind: pendKind, code: code}
			s.line = start
			switch pendKind {
			case "program":
				parseProgramArgs(s, pendArgs, func(f string, a ...any) { errorf("%s:%d: "+f, append([]any{doc, pendLine}, a...)...) })
			case "excerpt":
				s.ref = strings.TrimSpace(pendArgs)
			case "illustration":
				if strings.TrimSpace(pendArgs) == "" {
					errorf("%s:%d: an illustration marker needs a reason", doc, pendLine)
				}
			default:
				errorf("%s:%d: a go block with no <!-- program: PHASE -->, <!-- excerpt: PATH --> or <!-- illustration: reason --> marker before it", doc, start)
				s.kind = "illustration"
			}
			out = append(out, s)
			last = s
		case pendKind == "excerpt":
			// An excerpt of a file that is not Go: an output.txt, a header, a gocpp.mod.
			out = append(out, &snippet{doc: doc, line: start, kind: "excerpt", ref: strings.TrimSpace(pendArgs), code: code})
		case pendKind == "output":
			if last == nil || last.kind != "program" {
				errorf("%s:%d: an output block with no program before it", doc, start)
			} else {
				last.output, last.hasOut = code, true
			}
		case pendKind == "rejected":
			out = append(out, &snippet{doc: doc, line: start, kind: "rejected", ref: strings.TrimSpace(pendArgs), code: code})
		case pendKind == "emitted":
			out = append(out, &snippet{doc: doc, line: start, kind: "emitted", ref: strings.TrimSpace(pendArgs), code: code})
		case pendKind != "":
			errorf("%s:%d: the marker %q is in front of a %q block", doc, pendLine, pendKind, lang)
		}
		pendKind = ""
	}
	if pendKind != "" {
		errorf("%s:%d: the marker %q is not followed by a block", doc, pendLine, pendKind)
	}
	return out
}

var phaseRx = regexp.MustCompile(`^[0-9]{2}$`)

func parseProgramArgs(s *snippet, args string, errorf func(format string, a ...any)) {
	f := strings.Fields(args)
	if len(f) == 0 || !phaseRx.MatchString(f[0]) {
		errorf("a program marker starts with the two-digit phase: <!-- program: 05 -->")
		return
	}
	s.phase = f[0]
	for _, w := range f[1:] {
		switch {
		case w == "go":
			s.goOK = true
		case w == "check":
			s.check = true
		case strings.HasPrefix(w, "id="):
			s.id = strings.TrimPrefix(w, "id=")
		case strings.HasPrefix(w, "exit="):
			n, err := strconv.Atoi(strings.TrimPrefix(w, "exit="))
			if err != nil {
				errorf("bad %q", w)
			}
			s.exit = n
		default:
			errorf("unknown flag %q in a program marker", w)
		}
	}
}

// docSnippets returns the blocks of all snippet docs.
func docSnippets(t *testing.T) []*snippet {
	t.Helper()
	var all []*snippet
	for _, pat := range snippetDocs {
		files, err := filepath.Glob(filepath.Join("..", "..", "docs", filepath.FromSlash(pat)))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			rel, _ := filepath.Rel(filepath.Join("..", "..", "docs"), f)
			all = append(all, parseSnippets(filepath.ToSlash(rel), string(b), t.Errorf)...)
		}
	}
	return all
}

// TestSnippetMarkers: every go block of the tour and the guides says what it is, a program has
// its output, and the blocks that refer to a program or a file refer to one that exists.
func TestSnippetMarkers(t *testing.T) {
	snips := docSnippets(t)
	if len(snips) == 0 {
		t.Fatal("no snippet found")
	}
	ids := map[string]string{}
	for _, s := range snips {
		if s.kind == "program" && s.id != "" {
			if prev, dup := ids[s.doc+"/"+s.id]; dup {
				t.Errorf("%s:%d: the id %q is used twice (%s)", s.doc, s.line, s.id, prev)
			}
			ids[s.doc+"/"+s.id] = fmt.Sprint(s.line)
		}
	}
	for _, s := range snips {
		switch s.kind {
		case "program":
			if !s.check && !s.hasOut && !strings.Contains(s.code, "// no output") {
				t.Errorf("%s:%d: a program that is run needs an <!-- output --> block (or the flag check, or a comment \"// no output\")", s.doc, s.line)
			}
			if !strings.Contains(s.code, "package main") {
				t.Errorf("%s:%d: a program marker is for a whole program (package main); use an illustration for a fragment", s.doc, s.line)
			}
		case "excerpt":
			if _, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(s.ref))); err != nil {
				t.Errorf("%s:%d: the excerpt file %s: %v", s.doc, s.line, s.ref, err)
			}
		case "emitted", "rejected":
			if _, ok := ids[s.doc+"/"+s.ref]; !ok {
				t.Errorf("%s:%d: no program with id=%s in this doc", s.doc, s.line, s.ref)
			}
		}
	}
}

// ---- running the compiler ----

var (
	gocppOnce sync.Once
	gocppBin  string
	gocppErr  error
	cacheDir  string
	workRoot  string
)

func TestMain(m *testing.M) {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	var err error
	workRoot, err = os.MkdirTemp("", "docsite-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	cacheDir = filepath.Join(workRoot, "cache")
	code := m.Run()
	os.RemoveAll(workRoot)
	os.Exit(code)
}

// gocpp builds the compiler once and returns the path of the binary.
func gocpp(t *testing.T) string {
	t.Helper()
	gocppOnce.Do(func() {
		gocppBin = filepath.Join(workRoot, "gocpp")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", gocppBin, "github.com/huuhait/gocpp/cmd/gocpp")
		setProcessGroup(cmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			gocppErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if gocppErr != nil {
		t.Fatal(gocppErr)
	}
	return gocppBin
}

// runIn runs the command in dir with the gate phase and returns its standard output, standard
// error and exit status.
func runIn(t *testing.T, dir, phase string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	setProcessGroup(cmd)
	cmd.Env = append(os.Environ(), "GOCPP_GATE_PHASE="+phase, "XDG_CACHE_HOME="+cacheDir, "GOCPP_MODCACHE=off", "GOCPP_OFFLINE=1", "NO_COLOR=1")
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return so.String(), se.String(), ee.ExitCode()
		}
		t.Fatalf("%v: %v", args, err)
	}
	return so.String(), se.String(), 0
}

// programDir writes the program of a snippet into a new folder.
func programDir(t *testing.T, s *snippet, name string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(s.code+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func programs(t *testing.T) []*snippet {
	var out []*snippet
	for _, s := range docSnippets(t) {
		if s.kind == "program" {
			out = append(out, s)
		}
	}
	return out
}

func subName(s *snippet) string { return fmt.Sprintf("%s:%d", s.doc, s.line) }

// TestSnippetsCheck: every program of the docs passes gocpp check at its phase, and does not
// pass one phase earlier when its marker says that it needs the phase (a program that is marked
// 08 must really need 08, so the docs name the right switch).
func TestSnippetsCheck(t *testing.T) {
	bin := gocpp(t)
	for _, s := range programs(t) {
		t.Run(subName(s), func(t *testing.T) {
			t.Parallel()
			dir := programDir(t, s, "main.gp")
			_, stderr, code := runIn(t, dir, s.phase, bin, "check", ".")
			if code != 0 {
				t.Fatalf("gocpp check failed at phase %s:\n%s\n--- the program\n%s", s.phase, stderr, s.code)
			}
		})
	}
}

// TestSnippetsPhase: a program marked with a phase above the compiler's own is rejected without
// the switch (the docs tell people to set GOCPP_GATE_PHASE; this finds a marker that says more
// than needed or that has no effect), and is accepted with it.
func TestSnippetsPhase(t *testing.T) {
	bin := gocpp(t)
	for _, s := range programs(t) {
		if s.phase <= fmt.Sprintf("%02d", version.Phase) {
			continue
		}
		t.Run(subName(s), func(t *testing.T) {
			t.Parallel()
			dir := programDir(t, s, "main.gp")
			_, stderr, code := runIn(t, dir, "00", bin, "check", ".")
			if code == 0 {
				t.Fatalf("the program is accepted without GOCPP_GATE_PHASE, so the phase %s of its marker is not needed", s.phase)
			}
			if !strings.Contains(stderr, "Phase") {
				t.Errorf("the rejection does not name a phase:\n%s", stderr)
			}
			// The marker is not lower than needed: one phase below it, the program is rejected too.
			n, _ := strconv.Atoi(s.phase)
			below := fmt.Sprintf("%02d", n-1)
			if _, _, code := runIn(t, dir, below, bin, "check", "."); code == 0 {
				t.Errorf("the program is accepted at GOCPP_GATE_PHASE=%s: its marker %s says more than it needs", below, s.phase)
			}
		})
	}
}

func haveCXX() bool {
	for _, c := range []string{"g++", "clang++"} {
		if _, err := exec.LookPath(c); err == nil {
			return true
		}
	}
	return false
}

// TestSnippetsRejected: the error messages that the docs quote for a program that needs a phase
// are the lines that gocpp check prints without the switch.
func TestSnippetsRejected(t *testing.T) {
	bin := gocpp(t)
	byID := map[string]*snippet{}
	for _, s := range docSnippets(t) {
		if s.kind == "program" && s.id != "" {
			byID[s.doc+"/"+s.id] = s
		}
	}
	n := 0
	for _, s := range docSnippets(t) {
		if s.kind != "rejected" {
			continue
		}
		n++
		t.Run(subName(s), func(t *testing.T) {
			t.Parallel()
			p := byID[s.doc+"/"+s.ref]
			dir := programDir(t, p, "main.gp")
			_, stderr, code := runIn(t, dir, "00", bin, "check", ".")
			if code == 0 {
				t.Fatal("gocpp check accepts the program without the switch")
			}
			have := nonBlank(stderr, nil)
			for _, want := range nonBlank(s.code, nil) {
				if !slices.Contains(have, want) {
					t.Errorf("the doc quotes %q, which gocpp check does not print:\n%s", want, stderr)
				}
			}
		})
	}
	if n == 0 {
		t.Error("no rejected block in the docs")
	}
}

// TestSnippetsRun: the programs run and print the output that the docs show.
func TestSnippetsRun(t *testing.T) {
	if !haveCXX() {
		if os.Getenv("GOCPP_REQUIRE_CXX") != "" {
			t.Fatal("no C++ compiler")
		}
		t.Skip("no C++ compiler")
	}
	bin := gocpp(t)
	for _, s := range programs(t) {
		if s.check {
			continue
		}
		t.Run(subName(s), func(t *testing.T) {
			t.Parallel()
			dir := programDir(t, s, "main.gp")
			stdout, stderr, code := runIn(t, dir, s.phase, bin, "run", ".")
			if code != s.exit {
				t.Fatalf("exit status %d, want %d\nstdout:\n%s\nstderr:\n%s", code, s.exit, stdout, stderr)
			}
			if got, want := strings.TrimRight(stdout, "\n"), strings.TrimRight(s.output, "\n"); got != want {
				t.Errorf("output differs\n--- the doc\n%s\n--- gocpp run\n%s", want, got)
			}
		})
	}
}

// TestExamplesCheck: the example programs that the guides walk through pass gocpp check at their
// own phase.
func TestExamplesCheck(t *testing.T) {
	bin := gocpp(t)
	for _, ex := range []string{"sqlite", "raylib", "cpp-std", "mixed-sources"} {
		t.Run(ex, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join("..", "..", "examples", ex)
			_, stderr, code := runIn(t, dir, "04", bin, "check", ".")
			if code != 0 {
				t.Fatalf("gocpp check examples/%s:\n%s", ex, stderr)
			}
		})
	}
}

// requireTool skips the test when the tool is missing, or fails when the requirement variable
// is set (as GOCPP_REQUIRE_CXX does for the C++ compilers).
func requireTool(t *testing.T, tool, requireEnv string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err == nil {
		return
	}
	if os.Getenv(requireEnv) != "" {
		t.Fatalf("no %s command, and %s is set", tool, requireEnv)
	}
	t.Skipf("no %s command", tool)
}

// goRun builds the program in dir with the go command and runs it; it returns the standard
// output and the exit status of the program (not of go run, which reports 1 for any failure).
func goRun(t *testing.T, dir string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	env := append(os.Environ(), "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOCACHE="+filepath.Join(workRoot, "gocache"))
	build := exec.CommandContext(ctx, "go", "build", "-o", "prog", ".")
	build.Dir, build.Env = dir, env
	setProcessGroup(build)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	run := exec.CommandContext(ctx, filepath.Join(dir, "prog"))
	run.Dir, run.Env = dir, env
	setProcessGroup(run)
	var so bytes.Buffer
	run.Stdout = &so
	err := run.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		return so.String(), ee.ExitCode()
	}
	if err != nil {
		t.Fatalf("running the program: %v", err)
	}
	return so.String(), 0
}

// TestSnippetsVsGo: a program that says it is valid Go prints the same, with the same exit
// status, when the go command builds it. GOCPP_REQUIRE_GO makes a missing go command a failure.
func TestSnippetsVsGo(t *testing.T) {
	requireTool(t, "go", "GOCPP_REQUIRE_GO")
	for _, s := range programs(t) {
		if !s.goOK || s.check {
			continue
		}
		t.Run(subName(s), func(t *testing.T) {
			t.Parallel()
			dir := programDir(t, s, "main.go")
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module snippet\n\ngo 1.25\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			stdout, code := goRun(t, dir)
			if code != s.exit {
				t.Errorf("the program exits with status %d with the go command, the doc says %d", code, s.exit)
			}
			if got, want := strings.TrimRight(stdout, "\n"), strings.TrimRight(s.output, "\n"); got != want {
				t.Errorf("the doc and real Go differ\n--- the doc\n%s\n--- go\n%s", want, got)
			}
		})
	}
}

// ---- excerpts and emitted C++ ----

// lineSegments splits the lines of a block at the elision lines ("..." or "// ..."); each
// segment is a list of lines without blank lines.
func lineSegments(block string, norm func(string) string) [][]string {
	var segs [][]string
	var cur []string
	for _, l := range strings.Split(block, "\n") {
		t := strings.TrimSpace(l)
		if t == "..." || t == "// ..." {
			if len(cur) > 0 {
				segs = append(segs, cur)
			}
			cur = nil
			continue
		}
		if t == "" {
			continue
		}
		cur = append(cur, norm(l))
	}
	if len(cur) > 0 {
		segs = append(segs, cur)
	}
	return segs
}

// containsInOrder reports whether every segment occurs as a run of consecutive lines of text,
// the segments in the order of the text.
func containsInOrder(text []string, segs [][]string) (missing []string, ok bool) {
	pos := 0
	for _, seg := range segs {
		found := -1
		for i := pos; i+len(seg) <= len(text); i++ {
			match := true
			for k, l := range seg {
				if text[i+k] != l {
					match = false
					break
				}
			}
			if match {
				found = i
				break
			}
		}
		if found < 0 {
			return seg, false
		}
		pos = found + len(seg)
	}
	return nil, true
}

func trimNorm(s string) string { return strings.TrimSpace(s) }

func nonBlank(text string, drop func(string) bool) []string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "" || (drop != nil && drop(l)) {
			continue
		}
		out = append(out, strings.TrimSpace(l))
	}
	return out
}

// TestSnippetExcerpts: the excerpts of the example programs are lines of the real files.
func TestSnippetExcerpts(t *testing.T) {
	for _, s := range docSnippets(t) {
		if s.kind != "excerpt" {
			continue
		}
		t.Run(subName(s), func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(s.ref)))
			if err != nil {
				t.Fatal(err)
			}
			text := nonBlank(string(b), nil)
			segs := lineSegments(s.code, trimNorm)
			if len(segs) == 0 {
				t.Fatal("empty excerpt")
			}
			if seg, ok := containsInOrder(text, segs); !ok {
				t.Errorf("these lines are not in %s (as consecutive lines, in the order of the doc):\n%s", s.ref, strings.Join(seg, "\n"))
			}
		})
	}
}

// TestSnippetEmitted: the C++ shown for a program is what gocpp emit prints for it.
func TestSnippetEmitted(t *testing.T) {
	bin := gocpp(t)
	byID := map[string]*snippet{}
	for _, s := range docSnippets(t) {
		if s.kind == "program" && s.id != "" {
			byID[s.doc+"/"+s.id] = s
		}
	}
	for _, s := range docSnippets(t) {
		if s.kind != "emitted" {
			continue
		}
		t.Run(subName(s), func(t *testing.T) {
			t.Parallel()
			p := byID[s.doc+"/"+s.ref]
			if p == nil {
				t.Fatalf("no program %s", s.ref)
			}
			dir := programDir(t, p, "main.gp")
			stdout, stderr, code := runIn(t, dir, p.phase, bin, "emit", ".")
			if code != 0 {
				t.Fatalf("gocpp emit: %s", stderr)
			}
			text := nonBlank(stdout, func(l string) bool { return strings.HasPrefix(l, "#line") })
			segs := lineSegments(s.code, trimNorm)
			if seg, ok := containsInOrder(text, segs); !ok {
				t.Errorf("these lines are not in the C++ of gocpp emit (as consecutive lines, in order):\n%s\n--- emitted\n%s", strings.Join(seg, "\n"), stdout)
			}
		})
	}
}

// TestGateNotesCurrent: the tour and the guides tell people to set GOCPP_GATE_PHASE for the
// features of the phases that are not finished. When every phase of the plan is finished
// (internal/version), those notes are stale and must be removed.
func TestGateNotesCurrent(t *testing.T) {
	if version.Phase < 10 {
		t.Skipf("phase %d is the last finished phase: the notes are still true", version.Phase)
	}
	for _, pat := range snippetDocs {
		files, _ := filepath.Glob(filepath.Join("..", "..", "docs", filepath.FromSlash(pat)))
		for _, f := range files {
			b, _ := os.ReadFile(f)
			if bytes.Contains(b, []byte("GOCPP_GATE_PHASE")) {
				t.Errorf("%s still mentions GOCPP_GATE_PHASE, but phase %d is finished: update the page", f, version.Phase)
			}
		}
	}
}
