package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/explain"
)

const explainUsage = `usage: gocpp explain [-json] [-nochecks] topic | file.gp:LINE[-LINE] | "error message"
       gocpp explain -list [-messages] [-json]

gocpp explain says what a construct lowers to and why the C++ looks as it does.

  gocpp explain defer          a topic: what the mechanism is, with an example that the real
                               compiler emits now (gocpp explain -list shows the topics)
  gocpp explain main.gp:12     the C++ of line 12 of main.gp with the reasons for its choices
                               (the same notes as gocpp emit -annotate)
  gocpp explain "interfaces are not supported yet (Phase 05)"
                               an error message: what it means and what to do. gocpp has no error
                               codes: the message is its own name. A "file:line:col: " in front
                               is ignored.

`

// explainFlags are the flags of gocpp explain.
type explainFlags struct {
	list     bool
	messages bool
	json     bool
	nochecks bool
}

// runExplain is "gocpp explain" (Phase 13 step 13.5, docs/guides/explain.md).
func runExplain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var f explainFlags
	flags := flag.NewFlagSet("gocpp explain", flag.ContinueOnError)
	flags.BoolVar(&f.list, "list", false, "list the topics (with -messages: the error messages that have an explanation)")
	flags.BoolVar(&f.messages, "messages", false, "with -list: list the error messages")
	flags.BoolVar(&f.json, "json", false, "print JSON")
	flags.BoolVar(&f.nochecks, "nochecks", false, "for file.gp:LINE: say that the bounds and nil checks are removed, as gocpp build -nochecks does")
	// The flags may come after the topic: gocpp explain boxing -json.
	var rest []string
	for {
		if code, done := parseFlags("explain", explainUsage, flags, args, stderr); done {
			return code
		}
		if flags.NArg() == 0 {
			break
		}
		rest = append(rest, flags.Arg(0))
		args = flags.Args()[1:]
	}
	if f.messages && !f.list {
		fmt.Fprintln(stderr, "gocpp explain: -messages needs -list")
		return 2
	}
	if f.list {
		if len(rest) > 0 {
			fmt.Fprintln(stderr, "gocpp explain: -list takes no arguments")
			return 2
		}
		var err error
		if f.messages {
			err = explain.WriteMessageList(stdout, f.json)
		} else {
			err = explain.WriteList(stdout, f.json)
		}
		return explainDone(err, stderr)
	}
	if len(rest) == 0 {
		fmt.Fprint(stderr, explainUsage)
		flags.SetOutput(stderr)
		flags.PrintDefaults()
		return 2
	}
	if f.nochecks && !lineArgRx.MatchString(strings.Join(rest, " ")) {
		fmt.Fprintln(stderr, "gocpp explain: -nochecks needs file.gp:LINE")
		return 2
	}
	// A message has spaces: the words of an unquoted message are joined.
	arg := strings.Join(rest, " ")
	switch {
	case strings.TrimSpace(arg) == "":
		fmt.Fprint(stderr, "gocpp explain: no topic given (gocpp explain -list shows the topics)\n")
		return 2
	case lineArgRx.MatchString(arg):
		return explainLine(arg, &f, stdout, stderr)
	case fileArgRx.MatchString(arg):
		fmt.Fprintf(stderr, "gocpp explain: %s needs a line: gocpp explain %s:LINE (or :FIRST-LAST) shows the C++ of that line; gocpp emit -annotate PROGRAM shows a whole program\n", arg, arg)
		return 2
	case explain.LooksLikeMessage(arg) && explain.FindTopic(arg) == nil:
		return explainMessage(arg, &f, stdout, stderr)
	}
	t := explain.FindTopic(arg)
	if t == nil {
		return explainUnknown(arg, stderr)
	}
	live, err := liveExample(t)
	if err != nil {
		fmt.Fprintf(stderr, "gocpp explain: the example of %s could not be made: %v\n", t.Name, err)
		return exitErrors
	}
	if f.json {
		return explainDone(explain.WriteTopicJSON(stdout, t, live), stderr)
	}
	return explainDone(explain.WriteTopic(stdout, t, live), stderr)
}

func explainDone(err error, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "gocpp explain: %v\n", err)
		return exitErrors
	}
	return 0
}

// explainUnknown reports an argument that is no topic: with the close names, and the way to
// ask about a message.
func explainUnknown(arg string, stderr io.Writer) int {
	fmt.Fprintf(stderr, "gocpp explain: unknown topic %q", arg)
	if s := explain.Suggest(arg); len(s) > 0 {
		fmt.Fprintf(stderr, ". Did you mean: %s?", strings.Join(s, ", "))
	}
	fmt.Fprint(stderr, "\nRun 'gocpp explain -list' for the topics.\n")
	return 2
}

// explainMessage is gocpp explain "message".
func explainMessage(arg string, f *explainFlags, stdout, stderr io.Writer) int {
	ms := explain.FindMessage(arg)
	if len(ms) == 0 {
		if words := strings.Fields(arg); len(words) > 1 && allTopics(words) {
			fmt.Fprintf(stderr, "gocpp explain: %q is %d topics: gocpp explain takes one at a time (gocpp explain %s)\n", arg, len(words), strings.Join(words, "; gocpp explain "))
			return 2
		}
		fmt.Fprintf(stderr, "gocpp explain: no explanation for the message %q\nRun 'gocpp explain -list -messages' for the messages that have one, or 'gocpp explain -list' for the topics.\n", explain.StripPosition(arg))
		return 2
	}
	return explainDone(explain.WriteMessage(stdout, arg, ms, f.json), stderr)
}

// allTopics reports whether each of the words names a topic.
func allTopics(words []string) bool {
	for _, w := range words {
		if explain.FindTopic(w) == nil {
			return false
		}
	}
	return true
}

// fileArgRx matches a .gp file name with no line.
var fileArgRx = regexp.MustCompile(`^(?:[A-Za-z]:)?[^:]+\.(?:gp|go)$`)

// lineArgRx matches file.gp:LINE and file.gp:FIRST-LAST.
var lineArgRx = regexp.MustCompile(`^(.+\.(?:gp|go)):([0-9]+)(?:-([0-9]+))?$`)

// explainLine is gocpp explain file.gp:LINE: the C++ of the line with the reasons.
func explainLine(arg string, f *explainFlags, stdout, stderr io.Writer) int {
	m := lineArgRx.FindStringSubmatch(arg)
	file := m[1]
	first, _ := strconv.Atoi(m[2])
	last := first
	if m[3] != "" {
		last, _ = strconv.Atoi(m[3])
	}
	if first < 1 || last < first {
		fmt.Fprintf(stderr, "gocpp explain: %s: the lines must be counted from 1, first to last\n", arg)
		return 2
	}
	if _, err := os.Stat(file); err != nil {
		fmt.Fprintf(stderr, "gocpp explain: %v\n", err)
		return 2
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		abs = file
	}
	// A file of a main package is a program of its own. A file of another package belongs to
	// the program of the module that uses it: the one at the module's root.
	targets := []string{file}
	library := packageName(abs) != "main"
	if !library && otherFiles(abs) {
		// A main package of several files: the file alone may not compile (it calls a function
		// of another file), so the folder is the second try.
		targets = append(targets, filepath.Dir(file))
	}
	if library {
		root := moduleRoot(filepath.Dir(abs))
		if root == "" {
			fmt.Fprintf(stderr, "gocpp explain: %s is in package %s, which is not a main package, and no gocpp.mod above it names a program that uses it\nExplain a line of a main program that imports it, or a file of a main package\n", file, packageName(abs))
			return 2
		}
		targets = []string{root}
	}
	d, set, code := loadForExplain(stderr, targets...)
	if set == nil {
		return code
	}
	defer d.stopSignals()
	target := targets[0]
	v, code := d.annotated(set, f.nochecks, true, nil)
	if v == nil {
		return code
	}
	src := ""
	for _, p := range v.SourceFiles() {
		if samePath(p, abs, d.check.dir) {
			src = p
		}
	}
	if src == "" && library {
		fmt.Fprintf(stderr, "gocpp explain: %s is not used by the program at %s: it has no C++ in it\n", file, target)
		return 2
	}
	if src == "" {
		// A file with no C++ at all (only types, say): its reasons are the unplaced ones.
		src = abs
	}
	text := string(readGP(src))
	lines := strings.Split(text, "\n")
	if text == "" || first > len(lines) {
		fmt.Fprintf(stderr, "gocpp explain: %s has no line %d\n", file, first)
		return 2
	}
	last = min(last, len(lines))
	chunks := v.Lines(src, first, last)
	unplaced := v.UnplacedLines(src, first, last)
	if f.json {
		out := struct {
			File     string                   `json:"file"`
			First    int                      `json:"first"`
			Last     int                      `json:"last"`
			Chunks   []explain.Chunk          `json:"chunks"`
			Unplaced []explain.UnplacedReason `json:"unplaced,omitempty"`
		}{d.showPath(src), first, last, explain.RelChunks(chunks, d.showPath), unplaced}
		for i := range out.Unplaced {
			out.Unplaced[i].File = d.showPath(out.Unplaced[i].File)
		}
		if out.Chunks == nil {
			out.Chunks = []explain.Chunk{}
		}
		return explainDone(explainJSON(stdout, out), stderr)
	}
	var b bytes.Buffer
	rel := func(p string) string { return d.showPath(p) }
	kinds := map[string]bool{}
	if len(chunks) == 0 {
		for n := first; n <= last; n++ {
			fmt.Fprintf(&b, "// ---- %s:%d: %s\n", file, n, strings.TrimSpace(lines[n-1]))
		}
		b.WriteString("// no C++ comes from " + plural(last > first, "these lines", "this line") + ": a comment, a blank line, a declaration whose C++ is a member of another\n// line's type, or a line that is only a part of a larger statement (try the line that starts it)\n")
	} else {
		if err := explain.WriteChunks(&b, chunks, rel); err != nil {
			return explainDone(err, stderr)
		}
		for _, c := range chunks {
			for _, r := range c.Reasons {
				kinds[r.Kind] = true
			}
		}
	}
	for _, u := range unplaced {
		fmt.Fprintf(&b, "// %s:%d\n", rel(u.File), u.Line)
		var w bytes.Buffer
		explain.WriteChunks(&w, []explain.Chunk{{Reasons: []explain.Reason{u.Reason}}}, rel)
		b.Write(w.Bytes())
		kinds[u.Kind] = true
	}
	if len(kinds) > 0 {
		names := make([]string, 0, len(kinds))
		for k := range kinds {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "\n// gocpp explain %s explains the mechanisms\n", strings.Join(names, ", gocpp explain "))
	}
	_, err = stdout.Write(b.Bytes())
	return explainDone(err, stderr)
}

func plural(many bool, a, b string) string {
	if many {
		return a
	}
	return b
}

// samePath reports whether the paths a and b (relative to dir if they are not absolute) name the
// same file.
func samePath(a, b, dir string) bool {
	abs := func(p string) string {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		return filepath.Clean(p)
	}
	return abs(a) == abs(b)
}

// liveExample emits the example of the topic t with the real compiler: the annotated C++ of its
// program. The program is written to a folder of its own, so that nothing of the user's is
// involved; the places in the output name the example, not the folder.
func liveExample(t *explain.Topic) (*explain.Live, error) {
	dir, err := os.MkdirTemp("", "gocpp-explain-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	src := t.ExampleSource()
	path := filepath.Join(dir, t.Example)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		return nil, err
	}
	var errs bytes.Buffer
	d, code := newDriver("explain", &buildFlags{}, io.Discard, &errs)
	if d == nil {
		return nil, fmt.Errorf("gocpp exits with status %d: %s", code, strings.TrimSpace(errs.String()))
	}
	defer d.stopSignals()
	d.check.minPhase = t.Phase
	d.check.dir = dir
	set, _ := d.load([]string{path})
	if set == nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(errs.String()))
	}
	v, _ := d.annotated(set, false, false, nil)
	if v == nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(errs.String()))
	}
	v = v.OnlyFiles(func(p string) bool { return !strings.HasSuffix(p, "pkg_init.cpp") })
	var b bytes.Buffer
	rel := func(p string) string {
		if samePath(p, path, dir) {
			return "./" + t.Example // as the headers of a program in the current folder
		}
		return p
	}
	if err := v.WriteText(&b, rel); err != nil {
		return nil, err
	}
	text := b.String()
	// Without the two lines of the banner of gocpp emit -annotate.
	for range 2 {
		if i := strings.IndexByte(text, '\n'); i >= 0 {
			text = text[i+1:]
		}
	}
	live := &explain.Live{Source: src, Cpp: text}
	if t.Phase > gatePhaseOf() {
		live.Gated = t.Phase
	}
	return live, nil
}

// gatePhaseOf returns the phase of the gates of this gocpp.
func gatePhaseOf() int {
	p, err := gatePhase(os.Getenv(gatePhaseEnv))
	if err != nil {
		return 0
	}
	return p
}

// explainJSON writes v as indented JSON.
func explainJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false) // the C++ and the messages have < and >: no \u003c
	e.SetIndent("", "  ")
	return e.Encode(v)
}

var packageRx = regexp.MustCompile(`(?m)^package[ \t]+([A-Za-z_][A-Za-z0-9_]*)`)

// packageName returns the name in the package clause of the .gp file path, or "main" if it can't
// be read (the loader reports the problem).
func packageName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "main"
	}
	if m := packageRx.FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return "main"
}

// moduleRoot returns the folder with the gocpp.mod that dir is in, or "".
func moduleRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "gocpp.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// loadForExplain loads the first of the targets that loads without errors and returns its driver,
// which the caller stops. If none loads, the errors of the first one are written to stderr.
func loadForExplain(stderr io.Writer, targets ...string) (*driver, *buildSet, int) {
	var firstErrs bytes.Buffer
	firstCode := 0
	for i, t := range targets {
		var errs bytes.Buffer
		d, code := newDriver("explain", &buildFlags{}, io.Discard, &errs)
		if d == nil {
			stderr.Write(errs.Bytes())
			return nil, nil, code
		}
		set, code := d.load([]string{t})
		if set != nil {
			return d, set, 0
		}
		d.stopSignals()
		if i == 0 {
			firstErrs, firstCode = errs, code
		}
	}
	stderr.Write(firstErrs.Bytes())
	return nil, nil, firstCode
}

// otherFiles reports whether the folder of the .gp file path has other .gp files that are not
// tests: the package may have several files.
func otherFiles(path string) bool {
	ents, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return false
	}
	for _, e := range ents {
		n := e.Name()
		if strings.HasSuffix(n, ".gp") && !strings.HasSuffix(n, "_test.gp") && n != filepath.Base(path) {
			return true
		}
	}
	return false
}
