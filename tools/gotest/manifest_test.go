package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree writes files under dir/test.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		p := filepath.Join(root, "test", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

var fixturePin = Pin{Version: "go1.25.14", URL: "https://x", SHA256: strings.Repeat("ab", 32)}

func fixtureFiles() map[string]string {
	return map[string]string{
		"a.go":           "// run\n\npackage main\n",
		"b.go":           "// errorcheck\n\npackage p\n",
		"c.go":           "//go:build windows\n\n// run\n\npackage main\n",
		"fixedbugs/x.go": "// compile\n\npackage p\n",
		"fixedbugs/y.go": "// run -race\n\npackage main\n",
		"ken/k.go":       "// run\n\npackage main\n\nimport \"runtime\"\n\nfunc main() { runtime.SetFinalizer(nil, nil) }\n",
		"ken/notgo.txt":  "ignored",
		"d.go":           "// rundir\n\npackage ignored\n",
		"d.dir/p.go":     "package p\n",
		"asm.go":         "// rundir\n\npackage ignored\n",
		"asm.dir/a.s":    "TEXT ·f(SB)\n",
		"other/z.go":     "// run\n\npackage main\n", // a folder Go's runner does not read
		"typeparam/t.go": "// run\n\npackage main\n",
		".hidden.go":     "// run\n\npackage main\n",
	}
}

func TestBuildManifest(t *testing.T) {
	root := writeTree(t, fixtureFiles())
	ts, err := BuildManifest(root, testTarget, nil, map[string]string{"typeparam/t.go": "needs 7.3"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, x := range ts {
		got[x.Path] = x.Status + "|" + x.Needs
	}
	want := map[string]string{
		"a.go":           "applicable|",
		"b.go":           "applicable|",
		"c.go":           "skip:build-tag|",
		"fixedbugs/x.go": "applicable|",
		"fixedbugs/y.go": "skip:gc-flags|",
		"ken/k.go":       "skip:gc-finalizer|",
		"d.go":           "applicable|",
		"asm.go":         "skip:assembly|",
		"typeparam/t.go": "applicable|needs 7.3",
	}
	if len(got) != len(want) {
		t.Errorf("got %d tests %v, want %d", len(got), got, len(want))
	}
	for p, w := range want {
		if got[p] != w {
			t.Errorf("%s: %q, want %q", p, got[p], w)
		}
	}
}

func TestManifestIsDeterministic(t *testing.T) {
	root := writeTree(t, fixtureFiles())
	var texts []string
	for i := 0; i < 3; i++ {
		ts, err := BuildManifest(root, testTarget, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, FormatManifest(fixturePin, ts))
	}
	if texts[0] != texts[1] || texts[1] != texts[2] {
		t.Error("two runs on the same tree differ")
	}
	// sorted by path
	ts, _ := ParseManifest(texts[0])
	for i := 1; i < len(ts); i++ {
		if ts[i-1].Path >= ts[i].Path {
			t.Errorf("not sorted: %s before %s", ts[i-1].Path, ts[i].Path)
		}
	}
}

func TestManifestRoundTrip(t *testing.T) {
	root := writeTree(t, fixtureFiles())
	ts, _ := BuildManifest(root, testTarget, nil, map[string]string{"a.go": "needs 9.4"})
	text := FormatManifest(fixturePin, ts)
	back, err := ParseManifest(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != len(ts) {
		t.Fatalf("%d tests back, want %d", len(back), len(ts))
	}
	for i := range ts {
		if back[i].Path != ts[i].Path || back[i].Directive != ts[i].Directive || back[i].Status != ts[i].Status || back[i].Needs != ts[i].Needs {
			t.Errorf("%+v became %+v", ts[i], back[i])
		}
	}
	if FormatManifest(fixturePin, back) != text {
		t.Error("format(parse(x)) != x")
	}
}

func TestParseManifestErrors(t *testing.T) {
	for name, text := range map[string]string{
		"fields":     "a.go\trun\tapplicable\n",
		"status":     "a.go\trun\tmaybe\t-\n",
		"reason":     "a.go\trun\tskip:nonsense\t-\n",
		"needs":      "a.go\trun\tapplicable\tsoon\n",
		"skip+needs": "a.go\trun\tskip:cgo\tneeds 7.3\n",
		"duplicate":  "a.go\trun\tapplicable\t-\na.go\trun\tapplicable\t-\n",
	} {
		if _, err := ParseManifest(text); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseNeeds(t *testing.T) {
	m, err := ParseNeeds("# c\na.go needs 7.3  # why\nb/c.go needs 9.4\n")
	if err != nil || m["a.go"] != "needs 7.3" || m["b/c.go"] != "needs 9.4" {
		t.Errorf("%v %v", m, err)
	}
	if _, err := ParseNeeds("a.go wants 7.3\n"); err == nil {
		t.Error("bad word accepted")
	}
}

func TestCommittedManifestIsWellFormed(t *testing.T) {
	b, err := os.ReadFile("manifest.txt")
	if err != nil {
		t.Skip("no manifest.txt yet")
	}
	ts, err := ParseManifest(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) < 2000 {
		t.Errorf("only %d tests", len(ts))
	}
	nb, err := os.ReadFile("expect.txt")
	if err == nil {
		needs, err := ParseNeeds(string(nb))
		if err != nil {
			t.Fatal(err)
		}
		by := map[string]Test{}
		for _, x := range ts {
			by[x.Path] = x
		}
		for p, n := range needs {
			if by[p].Needs != n {
				t.Errorf("expect.txt says %s %s, manifest.txt says %q: run go run ./tools/gotest manifest", p, n, by[p].Needs)
			}
		}
	}
	if pb, err := os.ReadFile("passing.txt"); err == nil {
		app := map[string]bool{}
		for _, x := range ts {
			if !x.Skipped() {
				app[x.Path] = true
			}
		}
		for _, p := range ParsePassing(string(pb)) {
			if !app[p] {
				t.Errorf("passing.txt lists %s, not an applicable test", p)
			}
		}
	}
}
