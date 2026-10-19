package diff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Kind is the kind of a mismatch.
type Kind string

const (
	KindOutput    Kind = "output"    // stdout differs
	KindExit      Kind = "exit"      // exit code differs
	KindStderr    Kind = "stderr"    // first stderr line differs (standalone programs)
	KindBuild     Kind = "build"     // gocpp rejects a program that Go accepts
	KindICE       Kind = "ice"       // gocpp's C++ did not compile
	KindCrash     Kind = "crash"     // the subject died or hung in the middle of a bundle
	KindTimeout   Kind = "timeout"   // the subject hung
	KindSanitizer Kind = "sanitizer" // ASan / UBSan report
)

// Mismatch is a difference between the reference and a subject for one program.
type Mismatch struct {
	Prog int
	Kind Kind
	// Sig is the text the classifier matches: the first differing line pair, or the first
	// error line.
	Sig    string
	Want   string // reference output of the program (or the line), clipped
	Got    string
	Detail string
	// Scenario is the scenario ("== name" line of the output) that the first difference is in.
	Scenario string
	// WantFull and GotFull are the whole outputs of an output mismatch (Want and Got are
	// clipped): the address filter must see all of it.
	WantFull, GotFull string
}

func (m Mismatch) String() string {
	return fmt.Sprintf("program %d: %s: %s", m.Prog, m.Kind, strings.ReplaceAll(m.Sig, "\n", " | "))
}

var markerRx = regexp.MustCompile(`(?m)^#### (\d+)$`)

// SplitOutput splits the stdout of a bundle at its "#### N" marker lines into the output of
// each program. The order of ids is the order in the output. Text before the first marker is
// ignored.
func SplitOutput(out string) (ids []int, parts map[int]string) {
	parts = map[int]string{}
	locs := markerRx.FindAllStringSubmatchIndex(out, -1)
	for i, l := range locs {
		id, _ := strconv.Atoi(out[l[2]:l[3]])
		end := len(out)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		start := l[1] + 1
		if start > end {
			start = end
		}
		ids = append(ids, id)
		parts[id] = out[start:end]
	}
	return
}

// FirstDiff describes the first line at which a and b differ.
func FirstDiff(want, got string) (sig string, line int) {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		} else {
			a = "<end of output>"
		}
		if i < len(g) {
			b = g[i]
		} else {
			b = "<end of output>"
		}
		if a != b {
			return "- " + clip(a, 200) + "\n+ " + clip(b, 200), i + 1
		}
	}
	return "", 0
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

var sanitizerRx = regexp.MustCompile(`AddressSanitizer|UndefinedBehaviorSanitizer|LeakSanitizer|ThreadSanitizer|\.(?:cpp|hpp|cc|h|c):\d+:\d+: runtime error:`)

// IsSanitizer reports whether stderr holds a sanitizer report (also an UndefinedBehaviorSanitizer
// line, "file.cpp:12:5: runtime error: ...": a Go panic line has no such position).
func IsSanitizer(stderr string) bool { return sanitizerRx.MatchString(stderr) }

// RefOut is what the reference printed, per program: stdout and stderr.
type RefOut struct{ Out, Err map[int]string }

// RefOf splits the output of a reference run of a recovered bundle.
func RefOf(r Result) RefOut {
	_, o := SplitOutput(r.Stdout)
	_, e := SplitOutput(r.Stderr)
	return RefOut{Out: o, Err: e}
}

// scenarioAt returns the scenario of the 1-based line of an output: the last "== name" line
// before it.
func scenarioAt(out string, line int) string {
	lines := strings.Split(out, "\n")
	for i := min(line-1, len(lines)) - 1; i >= 0; i-- {
		if name, ok := strings.CutPrefix(lines[i], "== "); ok {
			return name
		}
	}
	return ""
}

// outputMismatch makes the mismatch of a program whose stdout differs.
func outputMismatch(id int, want, got string) Mismatch {
	sig, line := FirstDiff(want, got)
	return Mismatch{Prog: id, Kind: KindOutput, Sig: sig, Want: clip(want, 2000), Got: clip(got, 2000),
		Scenario: scenarioAt(want, line), WantFull: want, GotFull: got}
}

// CompareBundle compares the per-program output of the reference (ref, from RefOf)
// and the result of a subject for the programs ids of a recovered bundle. Programs that match are not returned. The second result lists
// programs that were not run by the subject (it died before them): they must be run again.
func CompareBundle(ids []int, ref RefOut, sub Result) (mm []Mismatch, unrun []int) {
	rparts := ref.Out
	if !sub.BuildOK {
		return nil, nil // the caller attributes the build failure
	}
	sids, sparts := SplitOutput(sub.Stdout)
	_, serr := SplitOutput(sub.Stderr)
	checkErr := sub.Exit == 0 && !sub.TimedOut && !sub.Truncated && !IsSanitizer(sub.Stderr)
	ran := map[int]bool{}
	for _, id := range sids {
		ran[id] = true
	}
	crashed := sub.Exit != 0 || sub.TimedOut || sub.Truncated
	last := -1
	if len(sids) > 0 {
		last = sids[len(sids)-1]
	}
	for _, id := range ids {
		want, ok := rparts[id]
		if !ok {
			continue // the reference did not print it: a generator problem, handled by the caller
		}
		if !ran[id] {
			if crashed {
				unrun = append(unrun, id)
			} else {
				mm = append(mm, Mismatch{Prog: id, Kind: KindOutput, Sig: "- (program output)\n+ <missing>", Want: clip(want, 400)})
			}
			continue
		}
		got := sparts[id]
		if id == last && crashed {
			kind := KindCrash
			if sub.TimedOut {
				kind = KindTimeout
			}
			if IsSanitizer(sub.Stderr) {
				kind = KindSanitizer
			}
			sig, _ := FirstDiff(want, got)
			if kind == KindSanitizer {
				sig = sanitizerSig(sub.Stderr)
			} else if kind == KindCrash {
				sig = firstLine(sub.Stderr)
			}
			mm = append(mm, Mismatch{Prog: id, Kind: kind, Sig: sig, Want: clip(want, 400), Got: clip(got, 400), Detail: clip(sub.Stderr, 2000)})
			continue
		}
		if got != want {
			mm = append(mm, outputMismatch(id, want, got))
			continue
		}
		// what the builtin print and println wrote: the same lines, in the same order
		if wantE, ok := ref.Err[id]; ok && checkErr && serr[id] != wantE {
			sig, line := FirstDiff(wantE, serr[id])
			mm = append(mm, Mismatch{Prog: id, Kind: KindStderr, Sig: sig, Want: clip(wantE, 2000), Got: clip(serr[id], 2000), Scenario: scenarioAt(wantE, line)})
		}
	}
	// a sanitizer report that did not stop the program is still a failure of every program it
	// came with: report it against the last program
	if !crashed && IsSanitizer(sub.Stderr) && len(ids) > 0 {
		mm = append(mm, Mismatch{Prog: progOfReport(sub.Stderr, ids), Kind: KindSanitizer, Sig: sanitizerSig(sub.Stderr), Detail: clip(sub.Stderr, 2000)})
	}
	return mm, unrun
}

var progNameRx = regexp.MustCompile(`\bp(\d+)_`)

// progOfReport finds the program that a sanitizer report that came at the end of the run (a
// leak) is about: the first generated name (pN_...) in its stack frames, else the last program.
func progOfReport(report string, ids []int) int {
	in := map[int]bool{}
	for _, id := range ids {
		in[id] = true
	}
	for _, m := range progNameRx.FindAllStringSubmatch(report, -1) {
		if id, _ := strconv.Atoi(m[1]); in[id] {
			return id
		}
	}
	return ids[len(ids)-1]
}

var sanKindRx = regexp.MustCompile(`(AddressSanitizer|LeakSanitizer|UndefinedBehaviorSanitizer|ThreadSanitizer)[^\n]*|runtime error: [^\n]*`)

// sanitizerSig is the signature of a sanitizer report: its kind line ("ERROR: LeakSanitizer:
// detected memory leaks", "runtime error: signed integer overflow ...").
func sanitizerSig(report string) string {
	kind := firstLine(report)
	if m := sanKindRx.FindString(report); m != "" {
		kind = clip(m, 200)
	}
	if st := stackSig(report); st != "" {
		kind += " @ " + st
	}
	return kind
}

var frameRx = regexp.MustCompile(`(?m)^\s*#\d+ 0x[0-9a-f]+ in (\S+)`)

// stackSig names the first frames of a report that are not the allocator's or the standard
// library's, with the numbers of generated names masked: two reports with different stacks
// are different findings (also when their first line is the same).
func stackSig(report string) string {
	var fs []string
	for _, m := range frameRx.FindAllStringSubmatch(report, -1) {
		f := m[1]
		if strings.HasPrefix(f, "operator") || strings.HasPrefix(f, "std::") || strings.HasPrefix(f, "__gnu_cxx") ||
			strings.HasPrefix(f, "malloc") || strings.HasPrefix(f, "calloc") || strings.HasPrefix(f, "__") {
			continue
		}
		fs = append(fs, numRx.ReplaceAllString(pfxRx.ReplaceAllString(f, "pN_"), "N"))
		if len(fs) == 3 {
			break
		}
	}
	return clip(strings.Join(fs, " < "), 200)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	l, _, _ := strings.Cut(s, "\n")
	return clip(l, 300)
}

var (
	hexRx   = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	goroRx  = regexp.MustCompile(`goroutine \d+`)
	recovRx = regexp.MustCompile(` \[recovered\]`)
)

// MaskStderr returns the line of the stderr of a program that died that says why (the
// "panic: " or "fatal error: " line, else the first), with what differs between
// implementations (addresses, the "[recovered]" note, a trailing "goroutine N") masked.
func MaskStderr(s string) string {
	_, l := panicSplit(s)
	l = hexRx.ReplaceAllString(l, "0xADDR")
	l = recovRx.ReplaceAllString(l, "")
	l = goroRx.ReplaceAllString(l, "goroutine N")
	return l
}

// panicSplit splits the stderr of a program into what the program printed before it died (the
// builtin println's lines) and the line that says why it died.
func preOf(s string) string {
	pre, _ := panicSplit(s)
	return pre
}

func panicSplit(s string) (pre, reason string) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, "panic: ") || strings.HasPrefix(l, "fatal error: ") || strings.HasPrefix(l, "runtime: ") {
			return strings.Join(lines[:i], "\n"), clip(l, 300)
		}
	}
	return strings.TrimSpace(s), ""
}

// CompareStandalone compares two runs of a single program: the exit code, the stdout and the
// masked first stderr line.
func CompareStandalone(id int, ref, sub Result) []Mismatch {
	var mm []Mismatch
	if sub.TimedOut {
		return []Mismatch{{Prog: id, Kind: KindTimeout, Sig: "timeout", Detail: clip(sub.Stderr, 1000)}}
	}
	if IsSanitizer(sub.Stderr) {
		return []Mismatch{{Prog: id, Kind: KindSanitizer, Sig: sanitizerSig(sub.Stderr), Detail: clip(sub.Stderr, 2000)}}
	}
	if ref.Exit != sub.Exit {
		mm = append(mm, Mismatch{Prog: id, Kind: KindExit, Sig: fmt.Sprintf("- exit %d\n+ exit %d", ref.Exit, sub.Exit), Detail: clip(sub.Stderr, 1000)})
	}
	if ref.Stdout != sub.Stdout {
		mm = append(mm, outputMismatch(id, ref.Stdout, sub.Stdout))
	}
	// what the program printed with println before it died, then why it died
	if pa, pb := preOf(ref.Stderr), preOf(sub.Stderr); pa != pb {
		sig, _ := FirstDiff(pa, pb)
		mm = append(mm, Mismatch{Prog: id, Kind: KindStderr, Sig: sig})
	}
	if a, b := MaskStderr(ref.Stderr), MaskStderr(sub.Stderr); a != b {
		mm = append(mm, Mismatch{Prog: id, Kind: KindStderr, Sig: "- " + a + "\n+ " + b})
	}
	return mm
}
