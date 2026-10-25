package analysis

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The manifest is the index of the corpus: one entry per program, with the constructs it
// covers and the exact output that real Go printed when the manifest was made. The programs
// are package main folders, so they are never imported: the only thing that runs them is this
// test (and, with GOCPP set, the optional gocpp test).

type gocppResult struct {
	Status string `json:"status"`           // "ok", "error" or "unmeasured"
	Output string `json:"output,omitempty"` // the program's output when ok
	Error  string `json:"error,omitempty"`  // from "main.go:" on, when error
}

type programEntry struct {
	Name       string      `json:"name"`
	Dir        string      `json:"dir"`
	Constructs []string    `json:"constructs"`
	Imports    []string    `json:"imports"`
	Expected   string      `json:"expected"`
	Gocpp      gocppResult `json:"gocpp"`
}

type programManifest struct {
	Schema    int            `json:"schema"`
	GoVersion string         `json:"go_version"`
	GOOS      string         `json:"goos"`
	GOARCH    string         `json:"goarch"`
	Note      string         `json:"note"`
	Programs  []programEntry `json:"programs"`
}

func loadManifest(t *testing.T) programManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("programs", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m programManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("programs/manifest.json: %v", err)
	}
	if m.Schema != 1 {
		t.Fatalf("programs/manifest.json: schema %d, want 1", m.Schema)
	}
	if len(m.Programs) == 0 {
		t.Fatal("programs/manifest.json has no programs")
	}
	return m
}

// runProgram runs "go run ." in the program's folder and returns its combined output.
func runProgram(t *testing.T, dir string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "run", ".")
	cmd.Dir = filepath.Join("programs", dir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// programDirs returns the folders under programs/ that hold a main.go: the source of truth a
// manifest entry can drift from. A new program that is not in manifest.json is a test failure,
// so it cannot quietly go untested.
func programDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir("programs")
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join("programs", e.Name(), "main.go")); err == nil {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

// sourceImports returns the import paths of a Go file, sorted, as the manifest must record them.
func sourceImports(t *testing.T, path string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	out := []string{}
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// TestPrograms is the oracle: every program is valid Go and prints exactly the output the
// manifest records. Each one runs twice, so a program whose output is not deterministic (a map
// range that prints its order, a goroutine schedule) fails here instead of making a later 16.1f
// comparison flaky. Output goes to stderr because the programs print with the built-in println,
// so the test compares the combined output.
func TestPrograms(t *testing.T) {
	m := loadManifest(t)
	// The manifest and the folders must name the same set: a program added without
	// regenerating the manifest would otherwise never run.
	inManifest := map[string]bool{}
	for _, p := range m.Programs {
		inManifest[p.Dir] = true
		if _, err := os.Stat(filepath.Join("programs", p.Dir, "main.go")); err != nil {
			t.Errorf("manifest entry %q: %v", p.Name, err)
		}
	}
	for _, dir := range programDirs(t) {
		if !inManifest[dir] {
			t.Errorf("programs/%s has a main.go but no manifest entry; run go run ./gostd/spike/analysis/genmanifest gostd/spike/analysis/programs", dir)
		}
	}
	for _, p := range m.Programs {
		t.Run(p.Name, func(t *testing.T) {
			if p.Dir == "" || p.Expected == "" {
				t.Fatalf("manifest entry %q has no dir or no expected output", p.Name)
			}
			if len(p.Constructs) == 0 {
				t.Errorf("manifest entry %q names no constructs", p.Name)
			}
			if got, want := strings.Join(sourceImports(t, filepath.Join("programs", p.Dir, "main.go")), ","), strings.Join(p.Imports, ","); got != want {
				t.Errorf("manifest imports %q, source imports %q; regenerate the manifest", want, got)
			}
			for run := 1; run <= 2; run++ {
				out, err := runProgram(t, p.Dir)
				if err != nil {
					t.Fatalf("go run (run %d) failed: %v\n%s", run, err, out)
				}
				if out != p.Expected {
					t.Fatalf("go run (run %d) printed\n%q\nwant\n%q", run, out, p.Expected)
				}
			}
		})
	}
}

// TestGocppStatus checks the manifest's recorded gocpp result against a real gocpp. It is
// skipped unless GOCPP names a gocpp binary, because the programs must be runnable in a plain
// Go environment too. The recorded column is job 16.1c's (branch p16): 00_baseline passes and
// every other program stops at the construct its entry names. When a 16.1f job makes one pass,
// this test fails until the manifest and emitter-needs.md are updated, which is the point.
func TestGocppStatus(t *testing.T) {
	gocpp := os.Getenv("GOCPP")
	if gocpp == "" {
		t.Skip("set GOCPP to a gocpp binary to check the recorded gocpp results (see emitter-needs.md)")
	}
	m := loadManifest(t)
	for _, p := range m.Programs {
		t.Run(p.Name, func(t *testing.T) {
			switch p.Gocpp.Status {
			case "unmeasured":
				t.Skip("the manifest has no gocpp measurement for this program")
			case "ok":
				out, err := runGocpp(t, gocpp, p.Dir)
				if err != nil {
					t.Fatalf("gocpp run -gc-engine failed: %v\n%s", err, out)
				}
				if out != p.Expected {
					t.Fatalf("gocpp run -gc-engine printed\n%q\nwant\n%q", out, p.Expected)
				}
			case "error":
				out, err := runGocpp(t, gocpp, p.Dir)
				if err == nil {
					t.Fatalf("gocpp run -gc-engine succeeded; the manifest records the error %q", p.Gocpp.Error)
				}
				if !strings.Contains(out, p.Gocpp.Error) {
					t.Fatalf("gocpp run -gc-engine printed\n%s\nwant the recorded error\n%s", out, p.Gocpp.Error)
				}
			default:
				t.Fatalf("manifest entry %q: unknown gocpp status %q", p.Name, p.Gocpp.Status)
			}
		})
	}
}

func runGocpp(t *testing.T, gocpp, dir string) (string, error) {
	t.Helper()
	cmd := exec.Command(gocpp, "run", "-gc-engine", ".")
	cmd.Dir = filepath.Join("programs", dir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
