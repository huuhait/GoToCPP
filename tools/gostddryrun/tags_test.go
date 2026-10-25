package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fakeGOROOT = "testdata/goroot"

func TestReadToolDefaults(t *testing.T) {
	d, err := readToolDefaults(fakeGOROOT)
	if err != nil {
		t.Fatal(err)
	}
	if d.Version != "go1.25.14" || d.Minor != "go1.25" {
		t.Errorf("version = %q/%q, want go1.25.14/go1.25", d.Version, d.Minor)
	}
	want := []string{"aliastypeparams", "dwarf5", "regabiargs", "regabiwrappers", "swissmap", "synchashtriemap"}
	if strings.Join(d.Experiments, ",") != strings.Join(want, ",") {
		t.Errorf("experiments = %v, want %v", d.Experiments, want)
	}
	if !d.Verified || d.Table != "go1.25" {
		t.Errorf("verified = %v, table = %q; want the go1.25 table entry", d.Verified, d.Table)
	}
	if d.DefaultGOAMD64 != "v1" || d.DefaultGOARM64 != "v8.0" || d.DefaultGOEXPERIMENT != "" {
		t.Errorf("defaults = %q/%q/%q", d.DefaultGOAMD64, d.DefaultGOARM64, d.DefaultGOEXPERIMENT)
	}
	var files []string
	for _, s := range d.Sources {
		files = append(files, s.File)
	}
	for _, want := range []string{"VERSION", "src/internal/buildcfg/zbootstrap.go", "src/internal/buildcfg/exp.go"} {
		if !contains(files, want) {
			t.Errorf("sources %v do not name %s", files, want)
		}
	}
}

func TestReadToolDefaultsRejectsChangedBaseline(t *testing.T) {
	dir := copyTree(t, fakeGOROOT)
	path := filepath.Join(dir, "src/internal/buildcfg/exp.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(data), "SwissMap:        true", "SwissMap:        false", 1)
	if changed == string(data) {
		t.Fatal("the test did not find SwissMap in the fake exp.go")
	}
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = readToolDefaults(dir)
	if err == nil || !strings.Contains(err.Error(), "knownDefaults") {
		t.Fatalf("err = %v; want a table mismatch that names knownDefaults", err)
	}
}

func TestReadToolDefaultsUnknownVersion(t *testing.T) {
	dir := copyTree(t, fakeGOROOT)
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("go1.26.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	zb := filepath.Join(dir, "src/internal/buildcfg/zbootstrap.go")
	data, err := os.ReadFile(zb)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zb, []byte(strings.Replace(string(data), "go1.25.14", "go1.26.0", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := readToolDefaults(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d.Minor != "go1.26" || d.Table != "" || d.Verified {
		t.Errorf("unknown version: minor=%q table=%q verified=%v", d.Minor, d.Table, d.Verified)
	}
	// The derived values are still the ones of the tree.
	if strings.Join(d.Experiments, ",") == "" {
		t.Error("no experiments derived from the tree")
	}
}

func TestApplyGOEXPERIMENT(t *testing.T) {
	fields := []string{"SwissMap", "RegabiWrappers", "RegabiArgs", "Dwarf5", "Synctest", "Arenas"}
	baseline := func() map[string]bool {
		return map[string]bool{"SwissMap": true, "RegabiWrappers": true, "RegabiArgs": true, "Dwarf5": true}
	}
	for _, tc := range []struct {
		name string
		expr string
		want string
		err  bool
	}{
		{"empty", "", "dwarf5,regabiargs,regabiwrappers,swissmap", false},
		{"none", "none", "", false},
		{"no and new", "noswissmap,synctest", "dwarf5,regabiargs,regabiwrappers,synctest", false},
		{"noregabi", "noregabi", "dwarf5,swissmap", false},
		{"unknown", "bogus", "", true},
	} {
		got, err := applyGOEXPERIMENT(baseline(), fields, tc.expr)
		if tc.err {
			if err == nil {
				t.Errorf("%s: no error for %q", tc.name, tc.expr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if strings.Join(got, ",") != tc.want {
			t.Errorf("%s: applyGOEXPERIMENT(%q) = %v, want %s", tc.name, tc.expr, got, tc.want)
		}
	}
}

func TestTagSet(t *testing.T) {
	d, err := readToolDefaults(fakeGOROOT)
	if err != nil {
		t.Fatal(err)
	}
	linux, err := d.tagSet("linux", "amd64", []string{"netgo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(linux.ReleaseTags) != 25 || linux.ReleaseTags[0] != "go1.1" || linux.ReleaseTags[24] != "go1.25" {
		t.Errorf("release tags = %v", linux.ReleaseTags)
	}
	for _, want := range []string{
		"amd64.v1", "goexperiment.aliastypeparams", "goexperiment.dwarf5",
		"goexperiment.regabiargs", "goexperiment.regabiwrappers",
		"goexperiment.swissmap", "goexperiment.synchashtriemap",
	} {
		if !contains(linux.ToolTags, want) {
			t.Errorf("tool tags %v do not contain %s", linux.ToolTags, want)
		}
	}
	for _, want := range []string{"purego", "math_big_pure_go", "unix", "netgo"} {
		if !contains(linux.BuildTags, want) {
			t.Errorf("build tags %v do not contain %s", linux.BuildTags, want)
		}
	}
	for _, want := range []string{"gocpp", "linux", "amd64", "go1.25", "goexperiment.swissmap"} {
		if !contains(linux.All, want) {
			t.Errorf("all tags %v do not contain %s", linux.All, want)
		}
	}
	if linux.Compiler != "gocpp" || linux.CgoEnabled {
		t.Errorf("compiler = %q, cgo = %v", linux.Compiler, linux.CgoEnabled)
	}

	windows, err := d.tagSet("windows", "amd64", nil)
	if err != nil {
		t.Fatal(err)
	}
	if contains(windows.BuildTags, "unix") {
		t.Errorf("windows build tags contain unix: %v", windows.BuildTags)
	}

	arm, err := d.tagSet("linux", "arm64", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(arm.ToolTags, "arm64.v8.0") || contains(arm.ToolTags, "amd64.v1") {
		t.Errorf("arm64 tool tags = %v", arm.ToolTags)
	}

	for _, bad := range []string{"gc", "gccgo", "gocpp", "cgo"} {
		if _, err := d.tagSet("linux", "amd64", []string{bad}); err == nil {
			t.Errorf("-tags %s was accepted", bad)
		}
	}
	if _, err := d.tagSet("darwin", "amd64", nil); err == nil {
		t.Error("-goos darwin was accepted")
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// copyTree copies a small testdata tree into a temporary directory, so a test
// can change a file without touching the repository.
func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}
