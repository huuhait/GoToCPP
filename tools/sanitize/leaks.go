package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// The leak audit (step 15.11, docs/guides/sanitizers.md).
//
// gocpp frees memory by reference counting, so a cycle of values that the program drops is never
// freed (divergence D1 of docs/spec/language.md), and LeakSanitizer reports it. A test that makes
// one on purpose, or by idiom, says "// lsan: cycle" in its header, and then runs with
// LeakSanitizer off. That exemption would also hide a real leak in the same program, so it is
// bounded:
//
//   - tools/sanitize/leaks.txt lists every program with the marker: its kind of cycle, why, and the
//     most it may leak, in bytes and allocations, for each compiler;
//   - "leaks -check" (part of go test) fails when a marker has no line, a line has no marker, a kind
//     is unknown or a line has no reason;
//   - "leaks" builds each listed program with -asan, with LeakSanitizer ON, and fails when it leaks
//     nothing (the cycle is gone: remove the marker and the line, so that a leak that comes back
//     is reported), when it leaks more than its ceiling (a new leak on top of the cycle), or when it
//     makes any other sanitizer report.

// The kinds of cycle. A line of leaks.txt names one; "mixed" is for a long program that has
// several, and its reason says which.
var leakKinds = map[string]string{
	"closure-self":    "a recursive closure holds itself through the variable that boxes it",
	"mutual-pointers": "values or types that point at each other (embedded pointers, parent and child links, mutually recursive generic types)",
	"doubly-linked":   "a doubly linked structure whose prev and next are both strong",
	"cond-locker":     "a sync.Cond whose Locker points into the struct that holds the Cond",
	"self-pointer":    "a value that points to itself",
	"cyclic-data":     "data with a cycle that the test builds to compare or walk it (reflect.DeepEqual on cycles)",
	"context-tree":    "context parents and children that hold each other until the tree is canceled",
	"mixed":           "several of the kinds above in one long program (the reason lists them)",
}

// A LeakEntry is one line of leaks.txt.
type LeakEntry struct {
	Path    string // relative to the repository root
	Kind    string
	Ceiling map[string]Leak // by compiler: g++, clang++
	Slack   Leak            // allowed above the ceiling, for a program whose leak varies from run to run (threads); zero for the others
	Reason  string
}

// leakCompilers are the compilers that every entry has a ceiling for.
var leakCompilers = []string{"g++", "clang++"}

var ceilingRx = regexp.MustCompile(`^(g\+\+|clang\+\+|slack)=(\d+)/(\d+)$`)

// ParseLeaks reads leaks.txt: "path | kind | g++=BYTES/ALLOCS clang++=BYTES/ALLOCS | reason", with
// # comments and blank lines.
func ParseLeaks(r io.Reader) ([]LeakEntry, error) {
	var out []LeakEntry
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.SplitN(line, " | ", 4)
		if len(f) != 4 {
			return nil, fmt.Errorf("leaks.txt:%d: want \"path | kind | g++=B/A clang++=B/A | reason\"", n)
		}
		e := LeakEntry{Path: strings.TrimSpace(f[0]), Kind: strings.TrimSpace(f[1]), Reason: strings.TrimSpace(f[3]), Ceiling: map[string]Leak{}}
		for _, tok := range strings.Fields(f[2]) {
			m := ceilingRx.FindStringSubmatch(tok)
			if m == nil {
				return nil, fmt.Errorf("leaks.txt:%d: bad ceiling %q (want g++=BYTES/ALLOCS)", n, tok)
			}
			b, _ := strconv.Atoi(m[2])
			a, _ := strconv.Atoi(m[3])
			if m[1] == "slack" {
				e.Slack = Leak{b, a}
				continue
			}
			if _, dup := e.Ceiling[m[1]]; dup {
				return nil, fmt.Errorf("leaks.txt:%d: two ceilings for %s", n, m[1])
			}
			e.Ceiling[m[1]] = Leak{b, a}
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// FormatLeaks writes the entries in the file's form, sorted by path.
func FormatLeaks(es []LeakEntry) string {
	es = slices.Clone(es)
	sort.Slice(es, func(i, j int) bool { return es[i].Path < es[j].Path })
	var b strings.Builder
	b.WriteString(leaksHeader)
	for _, e := range es {
		var cs []string
		for _, c := range leakCompilers {
			if l, ok := e.Ceiling[c]; ok {
				cs = append(cs, fmt.Sprintf("%s=%d/%d", c, l.Bytes, l.Allocs))
			}
		}
		if e.Slack != (Leak{}) {
			cs = append(cs, fmt.Sprintf("slack=%d/%d", e.Slack.Bytes, e.Slack.Allocs))
		}
		fmt.Fprintf(&b, "%s | %s | %s | %s\n", e.Path, e.Kind, strings.Join(cs, " "), e.Reason)
	}
	return b.String()
}

const leaksHeader = `# The programs that may leak a reference cycle (step 15.11, docs/guides/sanitizers.md).
#
# Every test with a "// lsan: cycle" header line is here: gocpp frees by reference counting and
# never frees a cycle (divergence D1), so LeakSanitizer is off for these programs in a normal
# sanitizer run. Here each one is bounded instead: "go run ./tools/sanitize leaks" builds it with
# -asan and LeakSanitizer ON, and fails when it leaks nothing (remove the marker and the line), more
# than the ceiling (a real leak on top of the cycle) or when another sanitizer reports.
#
# path | kind | ceiling per compiler, g++=BYTES/ALLOCATIONS clang++=BYTES/ALLOCATIONS [slack=BYTES/ALLOCATIONS] | reason
#
# slack is for a program whose leak differs from run to run (it starts goroutines, and what a leaked
# queue or a thread's bookkeeping holds depends on how they interleave); the limit is the ceiling plus
# the slack, at most 4 KiB. No line has one today: make the program deterministic first. The allocation slack must be 0 (the policy test enforces it): one more allocation is the sign of a leak.
#
# Kinds: closure-self, mutual-pointers, doubly-linked, cond-locker, self-pointer, cyclic-data,
# context-tree, mixed. "go run ./tools/sanitize leaks -update" records the measured sizes (it only
# lowers a ceiling; -allow-grow raises one, and the change needs a reviewer who agrees that it is
# a cycle and not a leak).
`

// markerRoots are the folders that are searched for "// lsan: cycle": tests/ (without testdata
// folders: the header tests have the line to test the parser), the emitter's inputs and the
// examples.
var markerRoots = []string{"tests", "internal/cppgen/testdata/emit", "examples"}

// FindMarkers returns the programs with the "// lsan: cycle" header line, relative to root.
func FindMarkers(root string) ([]string, error) {
	var out []string
	for _, base := range markerRoots {
		dir := filepath.Join(root, filepath.FromSlash(base))
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" && base == "tests" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(p) != ".gp" {
				return nil
			}
			h, err := ReadHeader(p)
			if err != nil {
				return err
			}
			if h.Cycle {
				rel, err := filepath.Rel(root, p)
				if err != nil {
					return err
				}
				out = append(out, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// CheckLeaks compares the list with the markers found in the tree and returns every problem.
func CheckLeaks(entries []LeakEntry, markers []string) []string {
	var probs []string
	listed := map[string]bool{}
	for _, e := range entries {
		switch {
		case listed[e.Path]:
			probs = append(probs, fmt.Sprintf("%s: listed twice", e.Path))
		case e.Path == "" || strings.HasPrefix(e.Path, "/") || strings.Contains(e.Path, ".."):
			probs = append(probs, fmt.Sprintf("%q: want a path relative to the repository root", e.Path))
		}
		listed[e.Path] = true
		if _, ok := leakKinds[e.Kind]; !ok {
			probs = append(probs, fmt.Sprintf("%s: unknown kind %q (want %s)", e.Path, e.Kind, strings.Join(sortedKinds(), ", ")))
		}
		if len(strings.Fields(e.Reason)) < 3 || strings.HasPrefix(e.Reason, "TODO") {
			probs = append(probs, fmt.Sprintf("%s: the reason is missing or still a TODO (say what the cycle is)", e.Path))
		}
		if e.Slack.Allocs != 0 || e.Slack.Bytes < 0 {
			probs = append(probs, fmt.Sprintf("%s: the allocation slack must be 0 (a leak of one allocation must be caught), not %d", e.Path, e.Slack.Allocs))
		}
		if e.Slack.Bytes > maxByteSlack {
			probs = append(probs, fmt.Sprintf("%s: the byte slack %d is more than %d: make the program deterministic instead (a leak that small hides in a slack that big)", e.Path, e.Slack.Bytes, maxByteSlack))
		}
		for _, c := range leakCompilers {
			l, ok := e.Ceiling[c]
			if !ok {
				probs = append(probs, fmt.Sprintf("%s: no ceiling for %s", e.Path, c))
			} else if l.Bytes <= 0 || l.Allocs <= 0 {
				probs = append(probs, fmt.Sprintf("%s: the ceiling for %s is %d bytes in %d allocations: a program that leaks nothing has no marker", e.Path, c, l.Bytes, l.Allocs))
			}
		}
	}
	marked := map[string]bool{}
	for _, m := range markers {
		marked[m] = true
		if !listed[m] {
			probs = append(probs, fmt.Sprintf("%s: has \"// lsan: cycle\" but is not in tools/sanitize/leaks.txt (add a line with its kind, the reason and the measured ceiling: go run ./tools/sanitize leaks -update)", m))
		}
	}
	for _, e := range entries {
		if !marked[e.Path] {
			probs = append(probs, fmt.Sprintf("%s: listed in leaks.txt, but the program does not exist or has no \"// lsan: cycle\" line", e.Path))
		}
	}
	sort.Strings(probs)
	return probs
}

func sortedKinds() []string {
	out := make([]string, 0, len(leakKinds))
	for k := range leakKinds {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// A LeakVerdict is the judgment of one program with one compiler.
type LeakVerdict struct {
	Fail    bool
	Message string
	Tighten bool // the program leaks less than its ceiling: the ceiling can be lowered
}

// JudgeLeak compares what a run with LeakSanitizer on reported with the ceiling.
func JudgeLeak(reports []Report, leak *Leak, ceil, slack Leak) LeakVerdict {
	for _, r := range reports {
		if r.Kind != LSan {
			return LeakVerdict{Fail: true, Message: "another sanitizer report: " + r.String()}
		}
	}
	switch {
	case leak == nil && len(reports) > 0:
		return LeakVerdict{Fail: true, Message: "LeakSanitizer reported a leak, but not its size (no SUMMARY line): the output is not understood"}
	case leak == nil:
		return LeakVerdict{Fail: true, Message: "the program leaks nothing any more: remove \"// lsan: cycle\" and the line in leaks.txt, so that a leak that comes back is reported"}
	case leak.Bytes > ceil.Bytes+slack.Bytes || leak.Allocs > ceil.Allocs+slack.Allocs:
		return LeakVerdict{Fail: true, Message: fmt.Sprintf("leaks %d bytes in %d allocations, more than the ceiling %d in %d (plus slack %d in %d): a leak on top of the cycle? (if it is only more cycle, say so in a review and run leaks -update -allow-grow)",
			leak.Bytes, leak.Allocs, ceil.Bytes, ceil.Allocs, slack.Bytes, slack.Allocs)}
	case slack != (Leak{}):
		return LeakVerdict{}
	case leak.Bytes < ceil.Bytes || leak.Allocs < ceil.Allocs:
		return LeakVerdict{Message: fmt.Sprintf("leaks %d bytes in %d allocations, below the ceiling %d in %d: run leaks -update to lower it", leak.Bytes, leak.Allocs, ceil.Bytes, ceil.Allocs), Tighten: true}
	}
	return LeakVerdict{}
}

// maxByteSlack is the most byte slack a line may have. No line uses slack today: the programs that leaked a
// different amount from run to run were made deterministic (a queue with room for all its items, a Done
// channel made before the cancel).
const maxByteSlack = 4096

// leaksFile is the path of the list, relative to the root.
const leaksFile = "tools/sanitize/leaks.txt"

func loadLeaks(root string) ([]LeakEntry, error) {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(leaksFile)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseLeaks(f)
}

func cmdLeaks(args []string, stdout, stderr io.Writer) error {
	var cf commonFlags
	fs := newFlagSet("leaks", stderr)
	cf.register(fs)
	cf.cxx = strings.Join(leakCompilers, ",") // the ceilings are per compiler: audit both
	fs.Lookup("cxx").DefValue = cf.cxx
	check := fs.Bool("check", false, "only compare the markers with leaks.txt (no builds)")
	update := fs.Bool("update", false, "record the measured sizes as the ceilings (lowers them; adds missing lines for new markers)")
	allowGrow := fs.Bool("allow-grow", false, "with -update: also raise a ceiling")
	filter := fs.String("filter", "", "only entries whose path matches this regexp")
	if err := fs.Parse(args); err != nil {
		return usageError("%v", err)
	}
	root := cf.root
	if root == "" {
		var err error
		if root, err = repoRoot(); err != nil {
			return err
		}
	}
	entries, err := loadLeaks(root)
	if err != nil {
		return err
	}
	markers, err := FindMarkers(root)
	if err != nil {
		return err
	}
	if *check {
		if probs := CheckLeaks(entries, markers); len(probs) > 0 {
			for _, p := range probs {
				fmt.Fprintln(stderr, "leaks:", p)
			}
			return exitError{1, fmt.Sprintf("%d problems in %s", len(probs), leaksFile)}
		}
		fmt.Fprintf(stdout, "leaks: %d programs with \"// lsan: cycle\", all listed in %s\n", len(markers), leaksFile)
		return nil
	}
	var rx *regexp.Regexp
	if *filter != "" {
		if rx, err = regexp.Compile(*filter); err != nil {
			return usageError("-filter: %v", err)
		}
	}
	if !*update {
		if probs := CheckLeaks(entries, markers); len(probs) > 0 {
			for _, p := range probs {
				fmt.Fprintln(stderr, "leaks:", p)
			}
			return exitError{1, fmt.Sprintf("%d problems in %s (leaks -check)", len(probs), leaksFile)}
		}
	}
	cf.root = root
	if cf.jobs < 1 {
		return usageError("-j %d: want 1 or more", cf.jobs)
	}
	s, err := cf.open(stderr)
	if err != nil {
		return err
	}
	defer s.close()
	return auditLeaks(s, entries, markers, rx, *update, *allowGrow, stdout, stderr)
}

// auditLeaks builds every listed program with -asan and LeakSanitizer on, with each compiler.
func auditLeaks(s *session, entries []LeakEntry, markers []string, rx *regexp.Regexp, update, allowGrow bool, stdout, stderr io.Writer) error {
	cf, root := &s.commonFlags, s.root
	mode, _ := ModeByName("asan")
	byPath := map[string]*LeakEntry{}
	for i := range entries {
		byPath[entries[i].Path] = &entries[i]
	}
	if update { // new markers get a line with a placeholder kind that the author must correct
		for _, m := range markers {
			if byPath[m] == nil {
				entries = append(entries, LeakEntry{Path: m, Kind: "mixed", Ceiling: map[string]Leak{}, Reason: "TODO: say what the cycle is"})
			}
		}
		byPath = map[string]*LeakEntry{}
		for i := range entries {
			byPath[entries[i].Path] = &entries[i]
		}
	}
	var progs []*Program
	for _, e := range entries {
		if rx != nil && !rx.MatchString(e.Path) {
			continue
		}
		h, err := ReadHeader(filepath.Join(root, filepath.FromSlash(e.Path)))
		if err != nil {
			return err
		}
		h.Cycle = false // LeakSanitizer ON: that is the audit
		if why := h.Skip(99, true); why != "" {
			return fmt.Errorf("%s: can't audit it: %s", e.Path, why)
		}
		progs = append(progs, &Program{Path: filepath.Join(root, filepath.FromSlash(e.Path)), Rel: e.Path, Header: h})
	}
	if len(progs) == 0 {
		if len(entries) == 0 {
			fmt.Fprintf(stdout, "leaks: no program has \"// lsan: cycle\": nothing to audit\n")
			return nil
		}
		return fmt.Errorf("no programs in %s match", leaksFile)
	}
	failures := 0
	for _, cxx := range splitList(cf.cxx) {
		if cxx != "" && !slices.Contains(leakCompilers, cxx) {
			return usageError("-cxx %q: the leak ceilings are for %s", cxx, strings.Join(leakCompilers, " and "))
		}
		r := s.runner(mode, cxx)
		fmt.Fprintf(stderr, "sanitize: leak audit of %d programs with %s\n", len(progs), cxx)
		results := runAll(r, progs, cf.jobs, nil)
		for i, res := range results {
			e := byPath[res.Program]
			// A leak report is not a failure of the run here: judge it against the ceiling.
			if res.Status == StatusBuildFailed || res.Status == StatusTimeout || (res.Status == StatusBadExit && res.Leak == nil && len(res.Reports) == 0) {
				fmt.Fprintf(stderr, "FAIL %s [%s]: %s\n%s\n", res.Program, cxx, res.Status, indent(res.Detail))
				failures++
				continue
			}
			ceil, have := e.Ceiling[cxx]
			if update && res.Leak != nil && onlyLSan(res.Reports) {
				switch {
				case !have:
					e.Ceiling[cxx] = *res.Leak
					fmt.Fprintf(stdout, "%s [%s]: ceiling %d/%d\n", res.Program, cxx, res.Leak.Bytes, res.Leak.Allocs)
				case e.Slack != (Leak{}) && res.Leak.Bytes <= ceil.Bytes+e.Slack.Bytes && res.Leak.Allocs <= ceil.Allocs+e.Slack.Allocs:
					// within the slack: the ceiling stays
				case allowGrow || (res.Leak.Bytes <= ceil.Bytes && res.Leak.Allocs <= ceil.Allocs):
					if *res.Leak != ceil {
						e.Ceiling[cxx] = *res.Leak
						fmt.Fprintf(stdout, "%s [%s]: ceiling %d/%d -> %d/%d\n", res.Program, cxx, ceil.Bytes, ceil.Allocs, res.Leak.Bytes, res.Leak.Allocs)
					}
				default:
					fmt.Fprintf(stderr, "FAIL %s [%s]: leaks %d/%d, above the ceiling %d/%d (-allow-grow raises it)\n", res.Program, cxx, res.Leak.Bytes, res.Leak.Allocs, ceil.Bytes, ceil.Allocs)
					failures++
					continue
				}
				results[i].Status = StatusOK // the leak is the program's cycle, recorded as its ceiling
				continue
			}
			v := JudgeLeak(res.Reports, res.Leak, ceil, e.Slack)
			if !have {
				v = LeakVerdict{Fail: true, Message: "no ceiling for " + cxx}
			}
			switch {
			case v.Fail:
				fmt.Fprintf(stderr, "FAIL %s [%s]: %s\n", res.Program, cxx, v.Message)
				results[i].Status, results[i].Detail = StatusReport, v.Message
				failures++
			case v.Message != "":
				fmt.Fprintf(stdout, "note %s [%s]: %s\n", res.Program, cxx, v.Message)
				results[i].Status = StatusOK
			default:
				results[i].Status = StatusOK
				if cf.verbose {
					fmt.Fprintf(stdout, "ok   %s [%s]: %d bytes in %d allocations\n", res.Program, cxx, res.Leak.Bytes, res.Leak.Allocs)
				}
			}
		}
		sum := newSummary(s.stage, mode, cxx, "leaks", nil, results)
		sum.Mode = "leaks"
		if err := sum.write(cf.out); err != nil {
			return err
		}
		fmt.Fprintln(stdout, sum.Line())
	}
	if update {
		if failures > 0 {
			return exitError{1, fmt.Sprintf("%d failures: leaks.txt not written", failures)}
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(leaksFile)), []byte(FormatLeaks(entries)), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "leaks: wrote", leaksFile)
		return nil
	}
	if failures > 0 {
		return exitError{1, fmt.Sprintf("%d leak audit failures (logs in %s)", failures, cf.out)}
	}
	return nil
}

// onlyLSan reports whether the reports are LeakSanitizer's alone.
func onlyLSan(rs []Report) bool {
	for _, r := range rs {
		if r.Kind != LSan {
			return false
		}
	}
	return true
}
