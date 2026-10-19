package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	for _, c := range []struct{ out, kind, want string }{
		{"exit status 1\ngocpp build: stat /tmp/x/gocpp-build123/prog: no such file or directory\n", "build-failed", "build-failed: gocpp build: stat /tmp/x/gocpp-build<N>..."},
		{"", "wrong-output", "wrong-output"},
	} {
		if got := classify(c.out, c.kind); got != c.want {
			t.Errorf("classify(%q) = %q, want %q", c.out, got, c.want)
		}
	}
}

// The programs print what want says (the test reads their source, not a build).
func TestProgramsAndAnswers(t *testing.T) {
	dir := t.TempDir()
	s := &stress{cfg: config{work: dir, programs: 4}}
	if err := s.writePrograms(); err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{0, 1, 2, 3, sharedIndex} {
		if _, err := os.Stat(filepath.Join(s.dir(i), "main.gp")); err != nil {
			t.Errorf("program %d: %v", i, err)
		}
		if want(i) == "" {
			t.Errorf("program %d has no answer", i)
		}
	}
}

func TestWipeModes(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want []string // what is left in the work folder ("-" for the folder gone)
	}{
		{"folder", []string{"-"}},
		{"contents", nil},
		{"glob", []string{".dot"}},
	} {
		tmp := t.TempDir()
		w := filepath.Join(tmp, "gocpp-build1")
		os.MkdirAll(filepath.Join(w, "sub"), 0o755)
		for _, f := range []string{"a.o", ".dot", "sub/x"} {
			os.WriteFile(filepath.Join(w, f), nil, 0o644)
		}
		wipe(w, tmp, tc.mode)
		var left []string
		if ents, err := os.ReadDir(w); err != nil {
			left = []string{"-"}
		} else {
			for _, e := range ents {
				left = append(left, e.Name())
			}
		}
		if len(left) != len(tc.want) || len(left) > 0 && left[0] != tc.want[0] {
			t.Errorf("-wipe-mode %s leaves %q, want %q", tc.mode, left, tc.want)
		}
	}
	tmp := t.TempDir()
	os.Mkdir(filepath.Join(tmp, "gocpp-build2"), 0o755)
	wipe(filepath.Join(tmp, "gocpp-build2"), tmp, "tmp")
	if ents, _ := os.ReadDir(tmp); len(ents) != 0 {
		t.Errorf("-wipe-mode tmp leaves %d entries", len(ents))
	}
}
