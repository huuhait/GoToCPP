//go:build linux

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The slow-clang tests (step 12.13d): the language server with a C++ compiler that is slow, hangs
// or crashes, by explicit gates and not by sleeps. The fake compiler is a shell script that
// records its process and waits for a gate file before every run that does C/C++ work (the
// questions that find the compiler are answered at once); it then runs the real compiler, or
// crashes. Every test shows that the compiler was still blocked when the server answered.

// slowClangScript is the fake compiler. SLOWCLANG_DIR holds the gate (the file "open"), the mode
// ("crash", "late" or nothing) and the log of the runs (one process id per line). In the mode
// "late" the runs that make the names (the header index, "-ast-list", and the receiver members,
// "-code-completion-at") run the real compiler to its end at once and hold its answer back (the
// file "held" has the process and the kind of each) until the file "release" exists: a slow
// compiler that has read the files long ago. The loops give up after a minute, so that a test
// that fails leaves nothing behind.
const slowClangScript = `#!/bin/sh
case " $* " in
  *" --version "*|*" -dM -E -x c++ - "*)
    if [ -n "$SLOWCLANG_REAL" ]; then exec "$SLOWCLANG_REAL" "$@"; fi
    case " $* " in
      *" --version "*) printf 'clang version 18.0.0\n' ;;
      *) printf '#define __clang__ 1\n#define __clang_major__ 18\n#define __GNUC__ 4\n' ;;
    esac
    exit 0
    ;;
esac
printf '%s\n' "$$" >> "$SLOWCLANG_DIR/runs"
tries=0
while [ ! -e "$SLOWCLANG_DIR/open" ]; do
  tries=$((tries + 1))
  if [ "$tries" -gt 6000 ]; then exit 3; fi
  sleep 0.01
done
case "$(cat "$SLOWCLANG_DIR/mode" 2>/dev/null)" in
  crash) kill -SEGV $$ ;;
  late)
    case " $* " in
      *" -ast-list "*) kind=names ;;
      *-code-completion-at=*) kind=members ;;
      *) exec "$SLOWCLANG_REAL" "$@" ;;
    esac
    out=$(mktemp -d "$SLOWCLANG_DIR/late.XXXXXX")
    "$SLOWCLANG_REAL" "$@" > "$out/stdout" 2> "$out/stderr"
    status=$?
    printf '%s %s\n' "$$" "$kind" >> "$SLOWCLANG_DIR/held"
    tries=0
    while [ ! -e "$SLOWCLANG_DIR/release" ]; do
      tries=$((tries + 1))
      if [ "$tries" -gt 6000 ]; then exit 3; fi
      sleep 0.01
    done
    cat "$out/stdout"
    cat "$out/stderr" >&2
    exit $status
    ;;
esac
if [ -n "$SLOWCLANG_REAL" ]; then exec "$SLOWCLANG_REAL" "$@"; fi
exit 1
`

// The file and the places of the cursor that the requests of the tests use.
const (
	slowForeignFile      = "foreign/main.gp"
	slowPlainFile        = "plain/main.gp"
	markAPICompletion    = "_ = api.|add(1, 2)"
	markAPIName          = "_ = api.ad|d(1, 2)"
	markMemberCompletion = "_ = win.|size()"
	markMemberName       = "_ = win.si|ze()"
	markVariable         = "win| := api.open()"
)

type slowClang struct {
	t        *testing.T
	dir      string
	compiler string
	real     string // the real compiler that answers once the gate is open, or ""
}

// newSlowClang writes the fake compiler, points the server at it (GOCPP_CLANG) and makes sure
// that no process of it outlives the test. With needReal the test is skipped when the machine has
// no clang.
func newSlowClang(t *testing.T, needReal bool) *slowClang {
	t.Helper()
	g := &slowClang{t: t, dir: t.TempDir()}
	if clang, err := probe.FindClang(context.Background()); err == nil {
		g.real = clang.Path
	} else if needReal {
		t.Skip(err)
	}
	g.compiler = filepath.Join(g.dir, "clang++")
	if err := os.WriteFile(g.compiler, []byte(slowClangScript), 0o755); err != nil {
		t.Fatal(err)
	}
	// Several runs wait at the gate at once (the index has four): they need places.
	t.Setenv(probe.EnvJobs, "4")
	t.Setenv("SLOWCLANG_DIR", g.dir)
	t.Setenv("SLOWCLANG_REAL", g.real)
	t.Setenv("GOCPP_CLANG", g.compiler)
	t.Setenv("GOCPP_INTEROP", "typed")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Cleanup(probe.SetRacyWindow(0))
	t.Cleanup(func() {
		for _, pid := range g.processes() {
			syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	return g
}

// openGate lets every compiler run that waits, and every later one, go on.
func (g *slowClang) openGate() {
	g.t.Helper()
	if err := os.WriteFile(filepath.Join(g.dir, "open"), nil, 0o644); err != nil {
		g.t.Fatal(err)
	}
}

// closeGate makes the runs that start from now on wait again.
func (g *slowClang) closeGate() {
	g.t.Helper()
	if err := os.Remove(filepath.Join(g.dir, "open")); err != nil {
		g.t.Fatal(err)
	}
}

func (g *slowClang) setMode(mode string) {
	g.t.Helper()
	if err := os.WriteFile(filepath.Join(g.dir, "mode"), []byte(mode), 0o644); err != nil {
		g.t.Fatal(err)
	}
}

// runs returns the process ids of the compiler runs that reached the gate, in order.
func (g *slowClang) runs() []int {
	data, _ := os.ReadFile(filepath.Join(g.dir, "runs"))
	var pids []int
	for _, field := range strings.Fields(string(data)) {
		if pid, err := strconv.Atoi(field); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// processes returns the live processes that carry this test's gate directory in their
// environment: the compiler runs and everything they started.
func (g *slowClang) processes() []int {
	entries, _ := os.ReadDir("/proc")
	want := []byte("SLOWCLANG_DIR=" + g.dir + "\x00")
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		environ, err := os.ReadFile("/proc/" + entry.Name() + "/environ")
		if err == nil && strings.Contains(string(environ), string(want)) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// holding reports whether a run of the kind ("names" or "members") has the answer of the real
// compiler and holds it.
func (g *slowClang) holding(kind string) bool {
	data, _ := os.ReadFile(filepath.Join(g.dir, "held"))
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasSuffix(line, " "+kind) {
			return true
		}
	}
	return false
}

// release lets the held answers out.
func (g *slowClang) release() {
	g.t.Helper()
	if err := os.WriteFile(filepath.Join(g.dir, "release"), nil, 0o644); err != nil {
		g.t.Fatal(err)
	}
}

// waitRuns waits until at least n runs have reached the gate and returns their processes. The
// wait ends when the event happens, or the test fails after a bound that no healthy machine
// reaches.
func (g *slowClang) waitRuns(n int) []int {
	g.t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		if pids := g.runs(); len(pids) >= n {
			return pids
		}
		if time.Now().After(deadline) {
			g.t.Fatalf("fewer than %d compiler runs reached the gate", n)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitGone waits until the process is not running (a zombie counts as gone).
func (g *slowClang) waitGone(pid int) {
	g.t.Helper()
	deadline := time.Now().Add(time.Minute)
	for slices.Contains(g.processes(), pid) {
		if time.Now().After(deadline) {
			g.t.Fatalf("compiler process %d is still running", pid)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitAnyGone waits until a compiler run that reached the gate has ended without the gate being
// opened: stopped by the server (its time limit, or a newer snapshot) or crashed.
func (g *slowClang) waitAnyGone() int {
	g.t.Helper()
	gone := -1
	slowWait(g.t, "a compiler run to end", func() bool {
		live := g.processes()
		for _, pid := range g.runs() {
			if !slices.Contains(live, pid) {
				gone = pid
				return true
			}
		}
		return false
	})
	return gone
}

// stillBlocked checks that the gate is closed and that a compiler run waits at it. A new
// snapshot cancels the runs of the old one and starts them again, so the wait for a run in the
// gap is bounded; it is the state, not its duration, that the tests rely on.
func (g *slowClang) stillBlocked(what string) {
	g.t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		if _, err := os.Stat(filepath.Join(g.dir, "open")); err == nil {
			g.t.Fatalf("%s: the gate is open", what)
		}
		live := g.processes()
		for _, pid := range g.runs() {
			if slices.Contains(live, pid) {
				return
			}
		}
		if time.Now().After(deadline) {
			g.t.Fatalf("%s: no compiler run waits at the gate (runs %v, processes %v)",
				what, g.runs(), live)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

const slowAPIHeader = `#include <string>
// Adds numbers.
int add(int a, int b);
struct Window {
  // Pixel size.
  int size() const;
  int w;
};
Window open();
`

const slowForeignSource = `package foreign
import "./api.hpp"
import "<string>"

func f() {
	_ = api.add(1, 2)
	var name std.string
	_ = name
	win := api.open()
	_ = win.size()
	_ = win.w
}
`

const slowPlainSource = `package plain

var Value = 1

func Twice(x int) int { return x * Value * 2 }

func f() {
	_ = Value
	Missing()
}
`

// slowTypoSource uses a namespace that does not exist: the error comes only when the compiler has
// answered (the editor-visible refresh path of the Phase 11 probe).
const slowTypoSource = `package typo
import "<string>"

func f() { _ = stdd.string("x") }
`

func slowFiles() map[string]string {
	return map[string]string{
		"gocpp.mod":       "module example.com/slow\n",
		slowForeignFile:   slowForeignSource,
		"foreign/api.hpp": slowAPIHeader,
		slowPlainFile:     slowPlainSource,
		"typo/main.gp":    slowTypoSource,
	}
}

// slowConfig is the pipeline of the tests with the fake compiler for the namespace probe too.
func slowConfig() cache.Config {
	cfg := testConfig()
	cfg.CXX = func(ctx context.Context) (build.CXX, error) { return probe.FindClang(ctx) }
	return cfg
}

// startSlow starts a server on a new copy of the files, with the compiler of the environment as
// the pipeline sees it now (typed or opaque).
func startSlow(t *testing.T, cfg cache.Config, files map[string]string) (*dclient, *Server) {
	t.Helper()
	root := writeTree(t, files)
	s := newServer(nil, cfg, debounce.RealClock{})
	c := &dclient{client: startServer(t, s, nil), root: root}
	t.Cleanup(s.pipeline.Close)
	if resp := c.call("initialize", markdownInit); resp.Error != nil {
		t.Fatal(resp.Error)
	}
	c.notify("initialized", "{}")
	return c, s
}

// A slowAnswer is the result of one request, the root of the tree replaced.
type slowAnswers map[string]string

func (c *dclient) answer(answers slowAnswers, name, method, params string) {
	c.t.Helper()
	resp := c.call(method, params)
	text := string(resp.Result)
	if resp.Error != nil {
		text = "error: " + resp.Error.Message
	}
	answers[name] = strings.ReplaceAll(text, filepath.ToSlash(c.root), "ROOT")
}

// slowRequests runs every kind of request of the editor on the open files, the typing of a
// character included, and returns the answers by name. The files are the ones of slowFiles.
func slowRequests(t *testing.T, c *dclient, hook func(step string)) slowAnswers {
	t.Helper()
	answers := make(slowAnswers)
	foreign, plain := slowForeignSource, slowPlainSource
	c.open(slowForeignFile, foreign)
	c.open(slowPlainFile, plain)
	answers["diagnostics plain"] = format(c.next(slowPlainFile))
	answers["diagnostics foreign"] = format(c.next(slowForeignFile))
	hook("diagnostics")
	ask := func(name, method, file, src, mark string) {
		t.Helper()
		c.answer(answers, name, method, c.posParams(file, src, mark))
		hook(name)
	}
	const completion, hover, definition = "textDocument/completion", "textDocument/hover",
		"textDocument/definition"
	ask("completion header package", completion, slowForeignFile, foreign, markAPICompletion)
	ask("completion member", completion, slowForeignFile, foreign, markMemberCompletion)
	ask("completion namespace", completion, slowForeignFile, foreign, "var name std.|string")
	ask("completion plain", completion, slowPlainFile, plain, "_ = Va|lue")
	ask("hover api", hover, slowForeignFile, foreign, markAPIName)
	ask("hover variable", hover, slowForeignFile, foreign, markVariable)
	ask("hover member", hover, slowForeignFile, foreign, markMemberName)
	ask("hover namespace", hover, slowForeignFile, foreign, "var name std.str|ing")
	ask("hover plain", hover, slowPlainFile, plain, "func Tw|ice")
	ask("definition api", definition, slowForeignFile, foreign, markAPIName)
	ask("definition member", definition, slowForeignFile, foreign, markMemberName)
	ask("definition plain", definition, slowPlainFile, plain, "x * Va|lue")
	ask("declaration api", "textDocument/declaration", slowForeignFile, foreign, markAPIName)
	ask("declaration member", "textDocument/declaration", slowForeignFile, foreign, markMemberName)
	ask("declaration plain", "textDocument/declaration", slowPlainFile, plain, "x * Va|lue")
	ask("signature help", "textDocument/signatureHelp", slowForeignFile, foreign, "api.add(|1, 2)")
	ask("prepare rename", "textDocument/prepareRename", slowPlainFile, plain, "func Tw|ice")

	const renameAt = `{"textDocument":{"uri":%q},` +
		`"position":{"line":%d,"character":%d},"newName":%q}`
	rename := fmt.Sprintf(renameAt, c.uri(slowPlainFile), 4, 6, "Double")
	c.answer(answers, "rename plain", "textDocument/rename", rename)
	hook("rename plain")
	renameLocal := fmt.Sprintf(renameAt, c.uri(slowForeignFile), 8, 1, "window")
	c.answer(answers, "rename local of foreign", "textDocument/rename", renameLocal)
	hook("rename local")
	formatDoc := func(name, file string) {
		params := fmt.Sprintf(`{"textDocument":{"uri":%q},`+
			`"options":{"tabSize":4,"insertSpaces":false}}`, c.uri(file))
		c.answer(answers, name, "textDocument/formatting", params)
		hook(name)
	}
	formatDoc("formatting foreign", slowForeignFile)
	formatDoc("formatting plain", slowPlainFile)

	// Typing: a comment inside the function, one character at a time, then the questions again.
	typed := strings.Replace(foreign, "\t_ = win.w\n", "\t_ = win.w\n\t// x\n", 1)
	c.change(slowForeignFile, 2, typed)
	hook("typing")
	answers["diagnostics after typing"] = format(c.next(slowForeignFile))
	ask("hover api after typing", hover, slowForeignFile, typed, markAPIName)
	ask("completion header package after typing", completion, slowForeignFile, typed,
		markAPICompletion)
	return answers
}

// TestSlowClangNoRequestWaitsAndAnswersAreThoseWithoutTypedData runs every kind of request while
// the compiler is blocked, and the answers equal those of a server in opaque mode.
func TestSlowClangNoRequestWaitsAndAnswersAreThoseWithoutTypedData(t *testing.T) {
	checkGoroutines(t)
	// The reference: opaque mode, no compiler at all.
	t.Setenv("GOCPP_INTEROP", "")
	reference, _ := startSlow(t, testConfig(), slowFiles())
	want := slowRequests(t, reference, func(string) {})
	reference.call("shutdown", "")
	reference.notify("exit", "")
	reference.wait()

	gate := newSlowClang(t, false)
	c, _ := startSlow(t, slowConfig(), slowFiles())
	started := false
	got := slowRequests(t, c, func(step string) {
		if !started {
			gate.waitRuns(1)
			started = true
		}
		gate.stillBlocked(step)
	})
	// A package whose header imports the compiler has not checked yet has no errors to show:
	// its diagnostics come at once, provisional, and are not those of a server with no compiler.
	for _, name := range []string{"diagnostics foreign", "diagnostics after typing"} {
		if got[name] != "" {
			t.Errorf("%s while the compiler is blocked: %q, want none", name, got[name])
		}
		delete(want, name)
	}
	for name, text := range want {
		if got[name] != text {
			t.Errorf("%s while the compiler is blocked:\n%s\nwithout typed data:\n%s",
				name, got[name], text)
		}
	}
	if len(want) == 0 {
		t.Fatal("no answers to compare")
	}
	c.call("shutdown", "")
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d", code)
	}
}

// slowWait polls until check returns true. The bound is a failure bound: no healthy machine
// reaches it, and what the test asserts is the state that it waited for, not how long it took.
func slowWait(t *testing.T, what string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for !check() {
		if time.Now().After(deadline) {
			t.Fatalf("%s never happened", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// completionLabels returns the labels of the completion at the | of mark.
func (c *dclient) completionLabels(file, src, mark string) []string {
	c.t.Helper()
	resp := c.call("textDocument/completion", c.posParams(file, src, mark))
	if resp.Error != nil {
		c.t.Fatalf("completion at %q: %v", mark, resp.Error)
	}
	var list protocol.CompletionList
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		c.t.Fatal(err)
	}
	return labels(list.Items)
}

// hoverText is the text of the hover at the | of mark, or "".
// foreignLabels and foreignHover ask about the file with the C/C++ names.
func (c *dclient) foreignLabels(src, mark string) []string {
	return c.completionLabels(slowForeignFile, src, mark)
}

func (c *dclient) foreignHover(src, mark string) string {
	return c.hoverText(slowForeignFile, src, mark)
}

func (c *dclient) hoverText(file, src, mark string) string {
	c.t.Helper()
	text, _ := c.hover(file, src, mark)
	return text
}

// TestSlowClangDataAppearsWhenTheCompilerAnswers: before the gate opens the editor has the plain
// answers; after it, completion, hover, definition and the native type of a variable have the
// C/C++ data in later requests, and the diagnostics of a package whose header imports were
// pending are published again with the error that the compiler found (the refresh path).
func TestSlowClangDataAppearsWhenTheCompilerAnswers(t *testing.T) {
	gate := newSlowClang(t, true)
	c, _ := startSlow(t, slowConfig(), slowFiles())
	foreign := slowForeignSource
	c.open(slowForeignFile, foreign)
	c.open("typo/main.gp", slowTypoSource)
	want(t, c.next("typo/main.gp"), 1, "")
	gate.waitRuns(1)

	got := c.foreignLabels(foreign, markAPICompletion)
	if slices.Contains(got, "add") {
		t.Errorf("a header name before the compiler has answered: %q", got)
	}
	text := c.foreignHover(foreign, markAPIName)
	if strings.Contains(text, "declared at") {
		t.Errorf("a declaration before the compiler has answered: %q", text)
	}
	gate.stillBlocked("before the gate opens")

	gate.openGate()
	slowWait(t, "header names", func() bool {
		return slices.Contains(c.foreignLabels(foreign, markAPICompletion), "add")
	})
	slowWait(t, "the declaration of api.add", func() bool {
		return strings.Contains(c.foreignHover(foreign, markAPIName), "int add(int a, int b)")
	})
	slowWait(t, "the native type of win", func() bool {
		return strings.Contains(c.foreignHover(foreign, markVariable), "C/C++ native type")
	})
	slowWait(t, "the members of win", func() bool {
		return slices.Contains(c.foreignLabels(foreign, markMemberCompletion), "size")
	})
	slowWait(t, "the definition of api.add", func() bool {
		locs := c.locationsAt("textDocument/definition", slowForeignFile, foreign, markAPIName)
		return len(locs) == 1 && strings.HasSuffix(string(locs[0].URI), "/foreign/api.hpp")
	})
	// The refresh path: the editor gets the diagnostics again, without asking.
	published := c.next("typo/main.gp")
	if got := format(published); !strings.Contains(got, "undefined: stdd") {
		t.Errorf("diagnostics published after the compiler answered:\n%s", got)
	}
}

// TestSlowClangShutdownKillsTheBlockedCompiler: shutdown and exit leave no compiler process, and
// no goroutine, although the compiler never answered.
func TestSlowClangShutdownKillsTheBlockedCompiler(t *testing.T) {
	checkGoroutines(t)
	gate := newSlowClang(t, false)
	c, _ := startSlow(t, slowConfig(), slowFiles())
	c.open(slowForeignFile, slowForeignSource)
	c.open("typo/main.gp", slowTypoSource)
	gate.waitRuns(1)
	gate.stillBlocked("before shutdown")
	if resp := c.call("shutdown", ""); resp.Error != nil {
		t.Fatal(resp.Error)
	}
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d", code)
	}
	for _, pid := range gate.processes() {
		t.Errorf("process %d of the compiler is left after exit", pid)
	}
}

const slowStaleHeader = `struct A { int fieldA; };
struct B { int fieldB; };
A openA();
B openB();
`

func slowStaleSource(kind, field string) string {
	return "package stale\nimport \"./api.hpp\"\n\nfunc f() {\n\twin := api.open" + kind +
		"()\n\t_ = win." + field + "\n}\n"
}

// TestSlowClangEditAfterDataNeverServesTheOldFacts: the facts of one text are not answered for
// the next one, whatever the compiler is doing: the members of a variable that has another type
// after the edit are the plain answer until the compiler has answered for the new text.
func TestSlowClangEditAfterDataNeverServesTheOldFacts(t *testing.T) {
	gate := newSlowClang(t, true)
	files := map[string]string{
		"gocpp.mod": "module example.com/stale\n",
		"main.gp":   slowStaleSource("A", "fieldA"),
		"api.hpp":   slowStaleHeader,
	}
	c, _ := startSlow(t, slowConfig(), files)
	before, after := slowStaleSource("A", "fieldA"), slowStaleSource("B", "fieldB")
	c.open("main.gp", before)
	gate.openGate()
	slowWait(t, "the members of the first text", func() bool {
		return slices.Contains(c.completionLabels("main.gp", before, "_ = win.|fieldA"), "fieldA")
	})

	gate.closeGate()
	c.change("main.gp", 2, after)
	got := c.completionLabels("main.gp", after, "_ = win.|fieldB")
	if slices.Contains(got, "fieldA") || slices.Contains(got, "fieldB") {
		t.Errorf("members right after the edit, with the compiler blocked: %q", got)
	}
	text := c.hoverText("main.gp", after, "win| := api.openB()")
	if strings.Contains(text, "C/C++ native type") {
		t.Errorf("a native type right after the edit, with the compiler blocked: %q", text)
	}
	gate.openGate()
	slowWait(t, "the members of the second text", func() bool {
		got := c.completionLabels("main.gp", after, "_ = win.|fieldB")
		if slices.Contains(got, "fieldA") {
			t.Fatalf("a member of the old text after the edit: %q", got)
		}
		return slices.Contains(got, "fieldB")
	})
}

// TestSlowClangEditWhileTheCompilerRunsNeverServesTheOldFacts: an edit while the compiler works on
// the old text; when the compiler is let go, only the new text's members come.
func TestSlowClangEditWhileTheCompilerRunsNeverServesTheOldFacts(t *testing.T) {
	gate := newSlowClang(t, true)
	files := map[string]string{
		"gocpp.mod": "module example.com/stale\n",
		"main.gp":   slowStaleSource("A", "fieldA"),
		"api.hpp":   slowStaleHeader,
	}
	c, _ := startSlow(t, slowConfig(), files)
	before, after := slowStaleSource("A", "fieldA"), slowStaleSource("B", "fieldB")
	c.open("main.gp", before)
	// Asking for the package starts the work.
	c.completionLabels("main.gp", before, "_ = win.|fieldA")
	gate.waitRuns(1)
	gate.stillBlocked("before the edit")

	c.change("main.gp", 2, after)
	got := c.completionLabels("main.gp", after, "_ = win.|fieldB")
	if slices.Contains(got, "fieldA") {
		t.Errorf("members of the old text after the edit: %q", got)
	}
	gate.openGate()
	slowWait(t, "the members of the second text", func() bool {
		got := c.completionLabels("main.gp", after, "_ = win.|fieldB")
		if slices.Contains(got, "fieldA") {
			t.Fatalf("a member of the old text after the edit: %q", got)
		}
		return slices.Contains(got, "fieldB")
	})
}

const slowChangingHeader = `struct A { int w; int %s; };
A openA();
`

// TestSlowClangHeaderChangedWhileTheCompilerFinishesPublishesNoOldFacts: the compiler has read the
// header and is slow to answer; the header changes on disk, with no notice to the server (a
// watcher that missed it); the answers that the compiler finally gives are not published, and
// the facts that come after are those of the new header.
func TestSlowClangHeaderChangedWhileTheCompilerFinishesPublishesNoOldFacts(t *testing.T) {
	gate := newSlowClang(t, true)
	source := "package stale\nimport \"./api.hpp\"\n\n" +
		"func f() {\n\twin := api.openA()\n\t_ = win.w\n}\n"
	files := map[string]string{"gocpp.mod": "module example.com/stale\n", "main.gp": source,
		"api.hpp": fmt.Sprintf(slowChangingHeader, "fieldOld")}
	c, _ := startSlow(t, slowConfig(), files)
	gate.setMode("late")
	gate.openGate()
	c.open("main.gp", source)
	slowWait(t, "the member run to hold its answer", func() bool {
		c.completionLabels("main.gp", source, "_ = win.|w") // keeps the package asked for
		return gate.holding("members")
	})
	writeFile(t, c.root, "api.hpp", fmt.Sprintf(slowChangingHeader, "fieldNew"))
	gate.setMode("")
	gate.release()
	slowWait(t, "the members of the new header", func() bool {
		got := c.completionLabels("main.gp", source, "_ = win.|w")
		if slices.Contains(got, "fieldOld") {
			t.Fatalf("a member of the old header: %q", got)
		}
		return slices.Contains(got, "fieldNew")
	})
}

// slowDegraded checks the plain answers of a server whose compiler gave nothing: no C/C++ data
// in any answer, the plain ones intact.
func slowDegraded(t *testing.T, c *dclient) {
	t.Helper()
	foreign, plain := slowForeignSource, slowPlainSource
	got := c.foreignLabels(foreign, markAPICompletion)
	if slices.Contains(got, "add") {
		t.Errorf("a header name from a compiler that gave nothing: %q", got)
	}
	text := c.foreignHover(foreign, markAPIName)
	if strings.Contains(text, "declared at") {
		t.Errorf("a declaration from a compiler that gave nothing: %q", text)
	}
	text = c.foreignHover(foreign, markVariable)
	if strings.Contains(text, "C/C++ native type") {
		t.Errorf("a native type from a compiler that gave nothing: %q", text)
	}
	got = c.completionLabels(slowPlainFile, plain, "_ = Va|lue")
	if !slices.Contains(got, "Value") {
		t.Errorf("plain completion lost: %q", got)
	}
	if text := c.hoverText(slowPlainFile, plain, "func Tw|ice"); !strings.Contains(text, "Twice") {
		t.Errorf("plain hover lost: %q", text)
	}
}

// TestSlowClangHangingCompilerIsKilledAndTheEditorKeepsThePlainAnswers: a compiler that never
// answers is stopped by the run's time limit; the answers stay the plain ones, and shutdown leaves
// no process and no goroutine.
func TestSlowClangHangingCompilerIsKilledAndTheEditorKeepsThePlainAnswers(t *testing.T) {
	checkGoroutines(t)
	t.Setenv(probe.EnvTimeout, "300ms")
	gate := newSlowClang(t, false)
	c, _ := startSlow(t, slowConfig(), slowFiles())
	// The package with the header imports is opened last, and nothing is edited: a run that ends
	// with the gate closed was stopped by its time limit (or by the snapshot that a changed
	// answer of the namespace probe makes, which starts the work again).
	c.open(slowPlainFile, slowPlainSource)
	c.open(slowForeignFile, slowForeignSource)
	gate.waitRuns(1)
	gate.waitAnyGone()
	gate.stillBlocked("after the limit")
	slowDegraded(t, c)
	if resp := c.call("shutdown", ""); resp.Error != nil {
		t.Fatal(resp.Error)
	}
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d", code)
	}
	for _, pid := range gate.processes() {
		t.Errorf("process %d of the compiler is left after exit", pid)
	}
}

// TestSlowClangCrashingCompilerLeavesThePlainAnswers: a compiler that dies by a signal on every
// run gives the plain answers, and the server shuts down cleanly.
func TestSlowClangCrashingCompilerLeavesThePlainAnswers(t *testing.T) {
	checkGoroutines(t)
	gate := newSlowClang(t, false)
	gate.setMode("crash")
	gate.openGate()
	c, _ := startSlow(t, slowConfig(), slowFiles())
	c.open(slowForeignFile, slowForeignSource)
	c.open(slowPlainFile, slowPlainSource)
	gate.waitRuns(1)
	gate.waitAnyGone()
	slowDegraded(t, c)
	if resp := c.call("shutdown", ""); resp.Error != nil {
		t.Fatal(resp.Error)
	}
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d", code)
	}
	for _, pid := range gate.processes() {
		t.Errorf("process %d of the compiler is left after exit", pid)
	}
}
