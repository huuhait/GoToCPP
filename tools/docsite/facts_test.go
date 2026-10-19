package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/toolchain"
	"github.com/huuhait/gocpp/internal/version"
)

// The facts that the pages state about gocpp itself are checked against gocpp: the commands they
// name, the standard packages they list, the phase they call finished and the platforms.

// snippetDocFiles returns the paths of the docs whose claims are checked (snippetDocs, globbed).
func snippetDocFiles(t *testing.T) []string {
	t.Helper()
	var all []string
	for _, pat := range snippetDocs {
		files, err := filepath.Glob(filepath.Join("..", "..", "docs", filepath.FromSlash(pat)))
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, files...)
	}
	// The page about the site itself names commands too.
	all = append(all, filepath.Join("..", "..", "docs", "guides", "docs-site.md"))
	return all
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var (
	cmdSpanRx  = regexp.MustCompile("`(?:\\$ )?gocpp ([a-z]+)[^`]*`")
	cmdLineRx  = regexp.MustCompile(`(?m)^\s*(?:\$ )?(?:[A-Z_]+=\S+ )*gocpp ([a-z]+)`)
	fenceBodyR = regexp.MustCompile("(?s)```(?:sh|text)?\n(.*?)```")
)

// commandsIn returns the gocpp commands that a doc names in code spans and in sh blocks.
func commandsIn(src string) []string {
	set := map[string]bool{}
	for _, m := range cmdSpanRx.FindAllStringSubmatch(src, -1) {
		set[m[1]] = true
	}
	for _, blk := range fenceBodyR.FindAllStringSubmatch(src, -1) {
		for _, m := range cmdLineRx.FindAllStringSubmatch(blk[1], -1) {
			set[m[1]] = true
		}
	}
	var out []string
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func TestCommandsInFindsCommands(t *testing.T) {
	src := "Run `gocpp emit .` and `GOCPP_GATE_PHASE=08 gocpp run .`, not `gocpp.mod` or `gocpp` alone.\n\n```sh\ngocpp build -x .\n$ gocpp check .\nGOCPP_X=1 gocpp vet ./...\ngo run ./tools/docsite\n```\n"
	got := commandsIn(src)
	if want := []string{"build", "check", "emit", "vet"}; !slices.Equal(got, want) {
		t.Errorf("commands %v, want %v", got, want)
	}
}

// TestCommandsExist: every gocpp command that these docs name exists in this gocpp (it does not
// answer "not available yet" or "unknown command"). A command that is planned is not mentioned
// as if it worked.
func TestCommandsExist(t *testing.T) {
	bin := gocpp(t)
	cmds := map[string][]string{}
	for _, f := range snippetDocFiles(t) {
		for _, c := range commandsIn(readFile(t, f)) {
			cmds[c] = append(cmds[c], filepath.Base(f))
		}
	}
	if len(cmds) < 8 {
		t.Fatalf("only %d commands found: %v", len(cmds), cmds)
	}
	for c, docs := range cmds {
		t.Run(c, func(t *testing.T) {
			t.Parallel()
			stdout, stderr, _ := runIn(t, t.TempDir(), "00", bin, c, "-h")
			out := stdout + stderr
			for _, bad := range []string{"not available yet", "unknown command"} {
				if strings.Contains(out, bad) {
					t.Errorf("`gocpp %s` (named in %v) says %q:\n%s", c, docs, bad, out)
				}
			}
		})
	}
}

var stdlibMarkerRx = regexp.MustCompile(`<!-- stdlib: (present|missing|stubs|native-stubs) (.*?) -->`)

// stdlibClaims returns the packages that the docs say are present and missing.
func stdlibClaims(t *testing.T) (present, missing []string) {
	t.Helper()
	for _, f := range snippetDocFiles(t) {
		for _, m := range stdlibMarkerRx.FindAllStringSubmatch(readFile(t, f), -1) {
			switch m[1] {
			case "present":
				present = append(present, strings.Fields(m[2])...)
			case "missing":
				missing = append(missing, strings.Fields(m[2])...)
			}
		}
	}
	return
}

func importOnly(pkg string) *snippet {
	return &snippet{code: "package main\n\nimport _ \"" + pkg + "\"\n\nfunc main() {}"}
}

// TestStdlibClaims: the packages that the docs list as there are exactly the packages of stdlib/,
// and they import; the packages they list as missing fail to import with "not available yet" even
// with every phase open (GOCPP_GATE_PHASE=15).
func TestStdlibClaims(t *testing.T) {
	bin := gocpp(t)
	present, missing := stdlibClaims(t)
	have, err := stdlibPackages(embeddedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(present)
	if !slices.Equal(present, have) {
		t.Errorf("the docs list the present packages\n  %v\nbut stdlib/ has\n  %v", present, have)
	}
	if len(missing) < 20 {
		t.Fatalf("only %d missing packages listed", len(missing))
	}
	for _, p := range present {
		t.Run("present/"+p, func(t *testing.T) {
			t.Parallel()
			_, stderr, code := runIn(t, programDir(t, importOnly(p), "main.gp"), "15", bin, "check", ".")
			if code != 0 {
				t.Errorf("import %q fails:\n%s", p, stderr)
			}
		})
	}
	for _, p := range missing {
		t.Run("missing/"+p, func(t *testing.T) {
			t.Parallel()
			if slices.Contains(have, p) {
				t.Fatalf("%s is listed as missing but stdlib/ has it", p)
			}
			_, stderr, code := runIn(t, programDir(t, importOnly(p), "main.gp"), "15", bin, "check", ".")
			if code == 0 || !strings.Contains(stderr, "package "+p+" is not available yet") {
				t.Errorf("import %q: exit %d\n%s", p, code, stderr)
			}
		})
	}
}

var pkgCountRx = regexp.MustCompile(`(?:has|:) (\d+) packages`)

// TestStdlibCount: the number of packages that the docs state is the number of packages of stdlib/.
func TestStdlibCount(t *testing.T) {
	have, err := stdlibPackages(embeddedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range snippetDocFiles(t) {
		for _, m := range pkgCountRx.FindAllStringSubmatch(readFile(t, f), -1) {
			n++
			if got, _ := strconv.Atoi(m[1]); got != len(have) {
				t.Errorf("%s says %s packages, stdlib/ has %d", filepath.Base(f), m[1], len(have))
			}
		}
	}
	if n < 3 {
		t.Errorf("only %d statements of the package count found", n)
	}
}

var finishedRx = regexp.MustCompile(`Phases 00 to (\d\d) are finished`)
var matrixPhaseRx = regexp.MustCompile("`gocpp version` prints `phase (\\d\\d)`")

// TestStatusText: the pages that say which phases are finished say version.Phase.
func TestStatusText(t *testing.T) {
	want := strconv.Itoa(version.Phase)
	if len(want) < 2 {
		want = "0" + want
	}
	root := filepath.Join("..", "..", "docs")
	for _, f := range []string{"index.md", "tour/README.md"} {
		m := finishedRx.FindStringSubmatch(readFile(t, filepath.Join(root, f)))
		if m == nil {
			t.Errorf("%s does not say \"Phases 00 to NN are finished\"", f)
		} else if m[1] != want {
			t.Errorf("%s says phases 00 to %s are finished, internal/version.Phase is %s", f, m[1], want)
		}
	}
	m := matrixPhaseRx.FindStringSubmatch(readFile(t, filepath.Join(root, "spec", "feature-matrix.md")))
	if m == nil || m[1] != want {
		t.Errorf("spec/feature-matrix.md does not say that gocpp version prints phase %s (found %v)", want, m)
	}
}

var gateNoteRx = regexp.MustCompile(`GOCPP_GATE_PHASE=(\d\d)`)

// TestLessonGateNotes: the switch that a lesson names in its text is the phase of the program of the
// lesson (so no lesson tells readers to open less or more than the program needs), and a lesson whose
// program needs a phase above the finished ones names it.
func TestLessonGateNotes(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "tour")
	files, _ := filepath.Glob(filepath.Join(root, "[0-9][0-9]-*.md"))
	if len(files) != 13 {
		t.Fatalf("%d lessons", len(files))
	}
	for _, f := range files {
		src := readFile(t, f)
		named := map[string]bool{}
		for _, m := range gateNoteRx.FindAllStringSubmatch(src, -1) {
			named[m[1]] = true
		}
		need := map[string]bool{}
		for _, s := range parseSnippets(filepath.Base(f), src, t.Errorf) {
			if s.kind == "program" && s.phase > fmt.Sprintf("%02d", version.Phase) {
				need[s.phase] = true
			}
		}
		if !maps.Equal(named, need) {
			t.Errorf("%s: the text names the switches %v, the programs need %v", filepath.Base(f), keys(named), keys(need))
		}
	}
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var platformsRx = regexp.MustCompile(`<!-- platforms: (.*?) -->`)

// TestPlatforms: the hosts that the FAQ lists as supported are the hosts that gocpp can download its
// Windows toolchain for (internal/toolchain.Archives), which is what the packaging guide says.
func TestPlatforms(t *testing.T) {
	m := platformsRx.FindStringSubmatch(readFile(t, filepath.Join("..", "..", "docs", "faq.md")))
	if m == nil {
		t.Fatal("faq.md has no platforms marker")
	}
	var hosts []string
	for _, a := range toolchain.Archives {
		hosts = append(hosts, a.Host)
	}
	sort.Strings(hosts)
	got := strings.Fields(m[1])
	sort.Strings(got)
	if !slices.Equal(got, hosts) {
		t.Errorf("the FAQ lists the platforms %v, internal/toolchain.Archives has %v", got, hosts)
	}
}

var backtickRx = regexp.MustCompile("`([a-z][a-z0-9/]*)`")

// TestStdlibListsMatchMarkers: the list of packages that a reader sees is the list that the tests
// check (the names in backquotes in the paragraph after a "stdlib:" marker are the names in it).
func TestStdlibListsMatchMarkers(t *testing.T) {
	n := 0
	for _, f := range snippetDocFiles(t) {
		src := readFile(t, f)
		for _, loc := range stdlibMarkerRx.FindAllStringSubmatchIndex(src, -1) {
			n++
			names := strings.Fields(src[loc[4]:loc[5]])
			rest := src[loc[1]:]
			if i := strings.Index(rest, "\n\n"); i >= 0 {
				rest = rest[:i]
			}
			var shown []string
			for _, m := range backtickRx.FindAllStringSubmatch(rest, -1) {
				shown = append(shown, m[1])
			}
			sort.Strings(shown)
			shown = slices.Compact(shown)
			sort.Strings(names)
			if !slices.Equal(shown, names) {
				t.Errorf("%s: the marker lists %v but the text after it shows %v", filepath.Base(f), names, shown)
			}
		}
	}
	if n != 4 {
		t.Errorf("%d stdlib markers, want 4", n)
	}
}

// TestStdlibStubs: the packages that the docs call declaration stubs (or stubs that the runtime
// implements) are the packages that stdlib/ has as stubs. When a stub becomes a real package
// (flag did, with its port) this fails and names the list to change.
func TestStdlibStubs(t *testing.T) {
	var claimed []string
	for _, f := range snippetDocFiles(t) {
		for _, m := range stdlibMarkerRx.FindAllStringSubmatch(readFile(t, f), -1) {
			if m[1] == "stubs" || m[1] == "native-stubs" {
				claimed = append(claimed, strings.Fields(m[2])...)
			}
		}
	}
	have, err := stdlibPackages(embeddedStdlib())
	if err != nil {
		t.Fatal(err)
	}
	var stubs []string
	for _, p := range have {
		if isStub(embeddedStdlib(), p) {
			stubs = append(stubs, p)
		}
	}
	sort.Strings(claimed)
	if !slices.Equal(claimed, stubs) {
		t.Errorf("the docs (go-developers.md) call these packages stubs:\n  %v\nbut stdlib/ has these declaration stubs:\n  %v\nchange the stubs and native-stubs markers and the text around them", claimed, stubs)
	}
}

// TestExamplesPending: tour lesson 8 says that wc and grep-lite run (flag is real since the 9.9 gap): the
// examples have no "// pending:" line, and the lesson no longer calls them pending.
func TestExamplesPending(t *testing.T) {
	text := readFile(t, filepath.Join("..", "..", "docs", "tour", "08-stdlib.md"))
	if regexp.MustCompile(`still marked\s+pending`).MatchString(text) {
		t.Fatal("tour/08-stdlib.md still says that wc and grep-lite are pending")
	}
	for _, ex := range []string{"wc", "grep-lite"} {
		src := readFile(t, filepath.Join("..", "..", "examples", ex, "main.gp"))
		if regexp.MustCompile(`(?m)^// pending:`).MatchString(src) {
			t.Errorf("examples/%s has a // pending: line, but tour lesson 8 says it runs", ex)
		}
	}
}

// TestExpandLessons: the lessons that point at gocpp expand (the must and the macros lessons: `!` and
// macros need no switch since step 10.12, the lesson's own switch is for the library it uses) use
// programs that it expands, so the sentence is true.
func TestExpandLessons(t *testing.T) {
	bin := gocpp(t)
	n := 0
	for _, s := range programs(t) {
		if (s.id != "must" && s.id != "macros") || !strings.HasPrefix(s.doc, "tour/") {
			continue
		}
		n++
		t.Run(subName(s), func(t *testing.T) {
			t.Parallel()
			stdout, stderr, code := runIn(t, programDir(t, s, "main.gp"), s.phase, bin, "expand", ".")
			if code != 0 || strings.TrimSpace(stdout) == "" {
				t.Errorf("gocpp expand: exit %d\n%s%s", code, stdout, stderr)
			}
		})
	}
	if n < 2 {
		t.Errorf("%d lessons with ! or macros", n)
	}
}
