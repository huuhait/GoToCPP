package tests

// TestGDBPrinters: the gdb support of gocpp (runtime/debug/gocpp-gdb.py, embedded in every
// program's .debug_gdb_scripts section) in scripted gdb sessions. tests/gdb/README.md describes
// the format of the sessions.

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var updateGDB = flag.Bool("update-gdb", false, "rewrite tests/gdb/*.golden from what gdb prints (-O0 sessions, first compiler)")

// A gdbScript is a session file of tests/gdb: gdb commands, each with the regexps that its output
// must match (#want lines), for the optimized sessions.
type gdbScript struct {
	file string
	cmds []gdbScriptCmd
}

type gdbScriptCmd struct {
	text string
	line int // its line in the file, for messages
	want []*regexp.Regexp
	keep *regexp.Regexp // #filter: only the lines that match are compared
}

var (
	gdbTagRx   = regexp.MustCompile(`//\s*@(\w+)\s*$`)
	gdbUseRx   = regexp.MustCompile(`@([a-z]\w*)`)
	gdbPhaseRx = regexp.MustCompile(`(?m)^// phase: (\d+)`)
	gdbStopRx  = regexp.MustCompile(`(?m)^(Temporary b|B)reakpoint \d+(\.\d+)?, `)
	gdbNoiseRx = regexp.MustCompile(`(?m)^(\[New Thread .*|\[Thread .* exited\]|\[Thread debugging .*|Using host libthread_db.*|warning: Error disabling address space randomization.*|warning: could not find .*|\[Inferior \d+ \(process \d+\) killed\])\n`)
	gdbAddrRx  = regexp.MustCompile(`0x[0-9a-f]{4,}`)
	gdbPathRx  = regexp.MustCompile(`(?:/[^/\s:()'"]+)+/([^/\s:()'"]+\.(?:gp|cpp|hpp|h|inc))`)
	// The C++ name and arguments of the function in a stop line are the compiler's; the .gp line is the point.
	gdbStopFnRx = regexp.MustCompile(`(?m)^((?:Thread \d+ "[^"]*" hit )?(?:Temporary b|B)reakpoint [\d.]+,) .* at (\S+:\d+)$`)
	gdbPidRx    = regexp.MustCompile(`\b(LWP|process|Thread) \d+`)
	gdbPyErrRx  = regexp.MustCompile(`Python Exception|Traceback \(most recent call last\)|Error occurred in Python|Python script error`)
	gdbWantLine = regexp.MustCompile(`^#want (.+)$`)
)

// readGDBScript reads a session file; tags maps the @tag of a command to a line of the program.
func readGDBScript(path string, tags map[string]int) (*gdbScript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s := &gdbScript{file: path}
	for i, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		n := i + 1
		if m := gdbWantLine.FindStringSubmatch(line); m != nil {
			if len(s.cmds) == 0 {
				return nil, fmt.Errorf("%s:%d: #want before the first command", path, n)
			}
			rx, err := regexp.Compile(m[1])
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, n, err)
			}
			last := &s.cmds[len(s.cmds)-1]
			last.want = append(last.want, rx)
			continue
		}
		if f, ok := strings.CutPrefix(line, "#filter "); ok {
			if len(s.cmds) == 0 {
				return nil, fmt.Errorf("%s:%d: #filter before the first command", path, n)
			}
			rx, err := regexp.Compile(f)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %v", path, n, err)
			}
			s.cmds[len(s.cmds)-1].keep = rx
			continue
		}
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		var bad string
		text := gdbUseRx.ReplaceAllStringFunc(line, func(m string) string {
			l, ok := tags[m[1:]]
			if !ok {
				bad = m
			}
			return strconv.Itoa(l)
		})
		if bad != "" {
			return nil, fmt.Errorf("%s:%d: no line of the program is marked %q", path, n, "// "+bad)
		}
		s.cmds = append(s.cmds, gdbScriptCmd{text: text, line: n})
	}
	return s, nil
}

// output returns the output of cmd, normalized and, with #filter, cut to the lines that match.
func (cmd gdbScriptCmd) output(raw string) string {
	out := normalizeGDB(raw)
	if cmd.keep == nil {
		return out
	}
	var b strings.Builder
	for _, line := range strings.SplitAfter(out, "\n") {
		if cmd.keep.MatchString(line) {
			b.WriteString(line)
		}
	}
	return b.String()
}

// normalizeGDB makes gdb's output the same on every machine and run: no thread and process
// numbers, no addresses, no folders.
func normalizeGDB(out string) string {
	out = gdbNoiseRx.ReplaceAllString(out, "")
	out = gdbPathRx.ReplaceAllString(out, "$1")
	out = gdbStopFnRx.ReplaceAllString(out, "$1 at $2")
	out = gdbAddrRx.ReplaceAllString(out, "0xADDR")
	out = gdbPidRx.ReplaceAllString(out, "$1 N")
	return out
}

// gdbVariant is one way to build and debug a program of tests/gdb.
type gdbVariant struct {
	name    string
	session string   // the session file is NAME+session+".gdb"; "" compares with the golden file, others check #want lines only
	flags   string   // more C++ flags (GOCPP_CXXFLAGS)
	trim    bool     // paths are trimmed (-ffile-prefix-map to /trim)
	subst   bool     // gdb gets set substitute-path /trim DIR
	asan    bool     // gocpp build -asan
	only    []string // only these programs; nil for all
	cxx     []string // only these compilers; nil for all
}

var gdbVariants = []gdbVariant{
	{name: "O0"},
	{name: "O2", session: ".O2", flags: "-O2"},
	{name: "trimpath", trim: true, subst: true, only: []string{"values", "frames"}},
	// The sources are not where the debug info says: values still print, closures keep C++ names.
	{name: "nosource", session: ".nosrc", trim: true, only: []string{"frames"}},
	// AddressSanitizer (clang++'s runtime is not found in some installations: g++ only).
	{name: "asan", asan: true, only: []string{"values", "chans"}, cxx: []string{"g++"}},
}

func TestGDBPrinters(t *testing.T) {
	c := setup(t)
	gdb, err := exec.LookPath("gdb")
	if err != nil {
		c.skipGDB(t, fmt.Sprintf("gdb is not installed (the dev shell, nix develop, has it): %v", err))
	}
	files, err := filepath.Glob(filepath.Join(repoRoot, "tests", "gdb", "*.gp"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no programs in tests/gdb: %v", err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".gp")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			m := gdbPhaseRx.FindSubmatch(src)
			if m == nil {
				t.Fatalf("%s has no // phase: line", file)
			}
			phase, _ := strconv.Atoi(string(m[1]))
			if why := phaseGate(phase, c.phase, c.phaseFrom); why != "" {
				t.Skip(why)
			}
			tags := map[string]int{}
			for i, line := range strings.Split(string(src), "\n") {
				if m := gdbTagRx.FindStringSubmatch(line); m != nil {
					tags[m[1]] = i + 1
				}
			}
			c.forEachCompiler(t, func(t *testing.T, cc compiler) {
				for _, v := range gdbVariants {
					if v.only != nil && !slices.Contains(v.only, name) || v.cxx != nil && !slices.Contains(v.cxx, cc.name) {
						continue
					}
					t.Run(v.name, func(t *testing.T) {
						c.gdbPrintTest(t, gdb, cc, v, file, name, phase, tags)
					})
				}
			})
		})
	}
}

// gdbPrintTest builds the program with the compiler and the variant, runs the session of the
// program (NAME.gdb, or NAME.O2.gdb for the optimized build) and checks each command's output:
// against NAME.golden (unoptimized), or against the #want lines (optimized).
func (c *config) gdbPrintTest(t *testing.T, gdb string, cc compiler, v gdbVariant, file, name string, phase int, tags map[string]int) {
	dir := filepath.Dir(file)
	sessionFile := filepath.Join(dir, name+v.session+".gdb")
	if _, err := os.Stat(sessionFile); err != nil && v.session != "" {
		t.Skipf("no %s session for this program", v.name)
	}
	wantsOnly := v.session != ""
	script, err := readGDBScript(sessionFile, tags)
	if err != nil {
		t.Fatal(err)
	}
	flags := v.flags
	if v.trim {
		flags += " -ffile-prefix-map=" + dir + "=/trim"
		if real := realPath(dir); real != dir {
			flags += " -ffile-prefix-map=" + real + "=/trim"
		}
	}
	var env []string
	if flags != "" {
		env = append(env, "GOCPP_CXXFLAGS="+strings.TrimSpace(flags))
	}
	bin := filepath.Join(t.TempDir(), name+exe)
	build := []string{"build", "-o", bin, filepath.Base(file)}
	if v.asan {
		build = slices.Insert(build, 1, "-asan")
	}
	r := c.run(t, gocppJob(dir, env, cc.cxx, phase, build...))
	if r.problem != "" || r.exit != 0 {
		t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
	}
	// -nx: no gdbinit files. No shell to start the program, no debuginfod, no colors, no line
	// wrapping; the safe path lets gdb run the script that the program carries (what a user
	// does with gocpp debug, or in ~/.gdbinit); LC_ALL=C keeps the messages in English.
	args := []string{"-nx", "-batch",
		"-iex", "set startup-with-shell off", "-iex", "set debuginfod enabled off",
		"-iex", "set style enabled off", "-iex", "set auto-load safe-path /",
		"-ex", "set width 0", "-ex", "set height 0", "-ex", "set pagination off"}
	if v.subst {
		args = append(args, "-ex", "set substitute-path /trim "+dir)
	}
	for i, cmd := range script.cmds {
		args = append(args, "-ex", "echo \\n"+gdbStepMark(i)+"\\n", "-ex", cmd.text)
	}
	args = append(args, "--args", bin)
	show := "gdb"
	for _, a := range args {
		show += " " + shellQuote(a)
	}
	// LeakSanitizer can't run under ptrace: it ends the program with an error at exit.
	j := job{dir: t.TempDir(), env: []string{"DEBUGINFOD_URLS=", "LC_ALL=C", "ASAN_OPTIONS=detect_leaks=0"}, name: gdb, args: args, show: show, combined: true}
	res := c.run(t, j)
	if res.problem != "" {
		t.Fatalf("%s: %s (%s)\noutput:\n%s", res.cmd, res.problem, res.status, indent(string(res.stdout)))
	}
	out := string(res.stdout)
	parts := splitGDBOutput(out, len(script.cmds))
	for i, cmd := range script.cmds {
		if cmd.text == "run" && !gdbStopRx.MatchString(parts[i]) {
			if m := ptraceRx.FindString(parts[i]); m != "" {
				c.skipGDB(t, fmt.Sprintf("gdb can't trace programs here (%q): ptrace is not allowed, as in some containers and sandboxes", m))
			}
		}
	}
	if gdbPyErrRx.MatchString(out) {
		i := gdbPyErrRx.FindStringIndex(out)
		t.Errorf("gdb reported a Python error: %q\n%s", out[max(0, i[0]-300):min(len(out), i[1]+600)], indent(out))
	}
	golden := filepath.Join(dir, name+".golden")
	var want map[string]string
	if !wantsOnly {
		want = readGolden(t, golden)
	}
	for i, cmd := range script.cmds {
		t.Run(fmt.Sprintf("%02d %s", i, cmd.text), func(t *testing.T) {
			out := cmd.output(parts[i])
			for _, rx := range cmd.want {
				if !rx.MatchString(out) {
					t.Errorf("%s:%d: the output of %q doesn't match %s:\n%s", sessionFile, cmd.line, cmd.text, rx, indent(out))
				}
			}
			if wantsOnly || *updateGDB {
				return
			}
			key := fmt.Sprintf("%02d %s", i, cmd.text)
			if w, ok := want[key]; !ok {
				t.Errorf("%s has no section %q (go test ./tests -run TestGDBPrinters -update-gdb)", golden, key)
			} else if w != out {
				t.Errorf("%s: gdb printed for %q (%s %s):\n%s\nwant:\n%s\nThe session:\n\t%s", name, cmd.text, cc.name, v.name, indent(out), indent(w), res.cmd)
			}
		})
	}
	if *updateGDB && v.name == "O0" && cc.name == c.compilers[0].name {
		var b strings.Builder
		for i, cmd := range script.cmds {
			b.WriteString(fmt.Sprintf("### %02d %s\n", i, cmd.text) + cmd.output(parts[i]))
		}
		if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// realPath returns dir with its symlinks resolved.
func realPath(dir string) string {
	if p, err := filepath.EvalSymlinks(dir); err == nil {
		return p
	}
	return dir
}

// readGolden reads a golden file: sections "### NN command" followed by the normalized output.
func readGolden(t *testing.T, path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		if *updateGDB {
			return nil
		}
		t.Fatalf("%v (go test ./tests -run TestGDBPrinters -update-gdb makes it)", err)
	}
	want := map[string]string{}
	var key string
	var body strings.Builder
	flush := func() {
		if key != "" {
			want[key] = body.String()
		}
		body.Reset()
	}
	for _, line := range strings.SplitAfter(string(data), "\n") {
		if k, ok := strings.CutPrefix(line, "### "); ok {
			flush()
			key = strings.TrimSuffix(k, "\n")
			continue
		}
		body.WriteString(line)
	}
	flush()
	return want
}

// TestNormalizeGDB checks the normalization that makes golden files portable.
func TestNormalizeGDB(t *testing.T) {
	in := "[New Thread 0x7ffff77ff6c0 (LWP 848586)]\nBreakpoint 1, gpkg_main::show (n=1) at /home/u/ab/gocpp/tests/gdb/values.gp:129\n" +
		"129\tx\npp (*main.Point) = &{X:7 Y:8} @0x555555630bd0\nsmall 0x10\n[Inferior 1 (process 42) killed]\n"
	want := "Breakpoint 1, at values.gp:129\n129\tx\npp (*main.Point) = &{X:7 Y:8} @0xADDR\nsmall 0x10\n"
	if got := normalizeGDB(in); got != want {
		t.Errorf("normalizeGDB:\n got %q\nwant %q", got, want)
	}
}

func TestReadGDBScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.gdb")
	text := "# comment\nbreak x.gp:@a\n\nrun\n#want ^Breakpoint 1, \n#want 4$\nprint $gp_len(s)\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := readGDBScript(path, map[string]int{"a": 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.cmds) != 3 || s.cmds[0].text != "break x.gp:4" || s.cmds[1].text != "run" || len(s.cmds[1].want) != 2 || s.cmds[2].text != "print $gp_len(s)" {
		t.Errorf("commands = %+v", s.cmds)
	}
	if _, err := readGDBScript(path, nil); err == nil || !strings.Contains(err.Error(), `no line of the program is marked "// @a"`) {
		t.Errorf("a tag without a line: error %v", err)
	}
	if err := os.WriteFile(path, []byte("#want x\nrun\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readGDBScript(path, nil); err == nil || !strings.Contains(err.Error(), "#want before the first command") {
		t.Errorf("#want first: error %v", err)
	}
}
