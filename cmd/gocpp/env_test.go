package main

import (
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The tests of gocpp env (env.go, Phase 06 step 6.12).

var updateGolden = flag.Bool("update", false, "write the golden files of cmd/gocpp/testdata")

// testPkgDir is the folder of this package: the tests change the current folder.
var testPkgDir = func() string {
	d, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return d
}()

// hermeticEnv clears every variable of gocpp env and sets the ones of the test, so that the output
// does not depend on the machine: no compiler is found (GOCPP_CXX names none).
func hermeticEnv(t *testing.T, set map[string]string) {
	t.Helper()
	for _, v := range envVars {
		t.Setenv(v.name, "")
	}
	t.Setenv("XDG_CACHE_HOME", "/xdg")
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "amd64")
	t.Setenv("GOCPP_CXX", "/nonexistent/c++")
	for k, v := range set {
		t.Setenv(k, v)
	}
}

var runtimeFolder = regexp.MustCompile(`/xdg/gocpp/rt/[A-Za-z0-9._-]+`)

func normalizeEnv(s, dir string) string {
	s = strings.ReplaceAll(s, dir, "$DIR")
	return runtimeFolder.ReplaceAllString(s, "/xdg/gocpp/rt/$$NAME")
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(testPkgDir, "testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Errorf("%s differs (go test ./cmd/gocpp -run %s -update writes it); got:\n%s", path, t.Name(), got)
	}
}

func TestEnvGolden(t *testing.T) {
	hermeticEnv(t, nil)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/m\n"})
	r := gocppRun("", false, "env")
	if r.code != 0 {
		t.Fatalf("gocpp env = %+v", r)
	}
	if !strings.Contains(r.stderr, "GOCPP_CXX") {
		t.Errorf("stderr = %q, want the message about the compiler that is not found", r.stderr)
	}
	golden(t, "env.golden", normalizeEnv(r.stdout, dir))
}

func TestEnvJSONGolden(t *testing.T) {
	hermeticEnv(t, nil)
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/m\n"})
	r := gocppRun("", false, "env", "-json")
	if r.code != 0 {
		t.Fatalf("gocpp env -json = %+v", r)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(r.stdout), &m); err != nil {
		t.Fatalf("not a JSON object: %v\n%s", err, r.stdout)
	}
	golden(t, "env-json.golden", normalizeEnv(r.stdout, dir))
}

func TestEnvSortedAndComplete(t *testing.T) {
	hermeticEnv(t, nil)
	inDir(t, nil)
	r := gocppRun("", false, "env")
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(r.stdout), "\n") {
		name, _, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("a line that is not NAME='value': %q", line)
		}
		names = append(names, name)
	}
	if !slices.IsSorted(names) {
		t.Errorf("the names are not sorted: %q", names)
	}
	if len(names) != len(envVars) {
		t.Errorf("%d lines, %d variables", len(names), len(envVars))
	}
	for _, v := range envVars {
		if v.doc == "" {
			t.Errorf("the variable %s has no description", v.name)
		}
	}
}

func TestEnvValueFromEnvironment(t *testing.T) {
	hermeticEnv(t, map[string]string{"GOCPP_CXXFLAGS": "-fpass-plugin=/x/p.so -DX=1", "GOCPP_LDFLAGS": "-lm", "GOCPP_OBJCACHE_SIZE": "5G"})
	inDir(t, nil)
	if r := gocppRun("", false, "env", "GOCPP_CXXFLAGS", "GOCPP_LDFLAGS", "GOCPP_OBJCACHE_SIZE", "GOCPP_LSP_CACHE_AGE"); r.stdout != "-fpass-plugin=/x/p.so -DX=1\n-lm\n5G\n120h\n" {
		t.Errorf("gocpp env NAME... = %+v", r)
	}
	r := gocppRun("", false, "env")
	if !strings.Contains(r.stdout, "GOCPP_CXXFLAGS='-fpass-plugin=/x/p.so -DX=1'\n") {
		t.Errorf("gocpp env lacks GOCPP_CXXFLAGS:\n%s", r.stdout)
	}
}

func TestEnvQuoting(t *testing.T) {
	hermeticEnv(t, map[string]string{"GOCPP_CXXFLAGS": `-DNAME='it''s'`})
	inDir(t, nil)
	if r := gocppRun("", false, "env"); !strings.Contains(r.stdout, `GOCPP_CXXFLAGS='-DNAME='"'"'it'"'"''"'"'s'"'"''`+"\n") && !strings.Contains(r.stdout, `'"'"'`) {
		t.Errorf("a single quote is not written the way a shell reads back:\n%s", r.stdout)
	}
}

func TestEnvSecretNotShown(t *testing.T) {
	hermeticEnv(t, map[string]string{"GOCPP_MODAUTH": "example.com=SECRET-TOKEN"})
	inDir(t, nil)
	for _, args := range [][]string{{"env"}, {"env", "-json"}, {"env", "GOCPP_MODAUTH"}} {
		r := gocppRun("", false, args...)
		if strings.Contains(r.stdout+r.stderr, "SECRET-TOKEN") || !strings.Contains(r.stdout, "(set)") {
			t.Errorf("gocpp %v shows the credentials or not that they are set: %+v", args, r)
		}
	}
}

func TestEnvErrors(t *testing.T) {
	hermeticEnv(t, nil)
	inDir(t, nil)
	if r := gocppRun("", false, "env", "NOPE"); r.code != 2 || !strings.Contains(r.stderr, "unknown variable NOPE") {
		t.Errorf("gocpp env NOPE = %+v", r)
	}
	if r := gocppRun("", false, "env", "-nope"); r.code != 2 {
		t.Errorf("gocpp env -nope = %+v", r)
	}
	if r := gocppRun("", false, "env", "-h"); r.code != 0 || !strings.Contains(r.stderr, "usage: gocpp env") {
		t.Errorf("gocpp env -h = %+v", r)
	}
	t.Setenv("GOOS", "plan9x")
	if r := gocppRun("", false, "env"); r.code != 2 {
		t.Errorf("gocpp env with a bad GOOS = %+v", r)
	}
}

func TestEnvFoundValues(t *testing.T) {
	c := anyCXX(t)
	hermeticEnv(t, map[string]string{"GOCPP_CXX": c.name})
	dir := inDir(t, map[string]string{"gocpp.mod": "module example.com/m\n"})
	r := gocppRun("", false, "env", "GOCPPCXX", "GOCPPCXXVERSION", "GOCPPMOD", "GOCPPVERSION")
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if r.code != 0 || len(lines) != 4 {
		t.Fatalf("gocpp env = %+v", r)
	}
	if lines[0] != c.cxx.Path || lines[1] != c.cxx.Version || lines[2] != filepath.Join(dir, "gocpp.mod") || lines[3] == "" {
		t.Errorf("found values = %q, want the compiler %q %q and gocpp.mod", lines, c.cxx.Path, c.cxx.Version)
	}
}

// TestEnvListsEveryVariable reads the Go code of the repository: every GOCPP_ name in a string
// of it (the names that it reads from the environment) has a line in gocpp env (envVars), and none has a line without being read.
func TestEnvListsEveryVariable(t *testing.T) {
	root := filepath.Join(testPkgDir, "..", "..")
	name := regexp.MustCompile(`"(GOCPP_[A-Z0-9_]+)"`)
	found := make(map[string]bool)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.IsDir() && (d.Name() == "testdata" || d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "tests" || d.Name() == "plans"):
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go"):
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if filepath.Base(p) == "env.go" && filepath.Base(filepath.Dir(p)) == "gocpp" {
			return nil // the list itself
		}
		for _, m := range name.FindAllStringSubmatch(string(data), -1) {
			found[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	listed := make(map[string]bool)
	for _, v := range envVars {
		listed[v.name] = true
	}
	// Variables that only tests and benchmarks of the repository read, or that gocpp env shows
	// under another name.
	internal := func(n string) bool {
		return strings.HasPrefix(n, "GOCPP_TEST_") || strings.HasPrefix(n, "GOCPP_BENCH") || n == "GOCPP_REQUIRE_CXX" || n == "GOCPP_RUN_VS_GO" ||
			n == "GOCPP_ALLOW_INTERNAL" || n == "GOCPP_REQUIRE_PORTSTD" || n == "GOCPP_LIB_" // the harness's, a tool's, and the prefix of a header guard
	}
	for n := range found {
		if !listed[n] && !internal(n) {
			t.Errorf("%s is read by the Go code but gocpp env does not list it (add it to envVars in cmd/gocpp/env.go)", n)
		}
	}
	for _, v := range envVars {
		// GOCPP_CFLAGS_ALLOW and the like are made from the name of the flags' variable.
		base := strings.TrimSuffix(strings.TrimSuffix(v.name, "_DISALLOW"), "_ALLOW")
		if strings.HasPrefix(v.name, "GOCPP_") && !found[v.name] && !found[base] {
			t.Errorf("gocpp env lists %s, but no Go code names it", v.name)
		}
	}
}
