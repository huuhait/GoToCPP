package doc

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/huuhait/gocpp/internal/modfetch"
)

// writeTree writes the files (path relative to dir: contents).
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func gp(who string) string {
	return "// Package p is the " + who + " one.\npackage p\n\n// F is in " + who + ".\nfunc F() {}\n"
}

func runIn(t *testing.T, env *Env, opts Options, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := Run(&buf, env, opts, args)
	return buf.String(), err
}

// The places that are searched, in order: the standard library, the main
// module, the modules it requires (replaced, then the cache). A name found in
// an earlier place hides the same name in a later one, whatever the depth of
// the folders.
func TestLookupOrder(t *testing.T) {
	std := fstest.MapFS{
		"encoding/json/json.gp": {Data: []byte(strings.ReplaceAll(gp("STD"), "package p", "package json"))},
		"errors/errors.gp":      {Data: []byte(strings.ReplaceAll(gp("STD errors"), "package p", "package errors"))},
	}
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"gocpp.mod":            "module example.com/m\n\nrequire example.com/dep v1.0.0\nreplace example.com/dep => ./dep\n",
		"json/json.gp":         strings.ReplaceAll(gp("MAIN json"), "package p", "package json"),
		"errors/errors.gp":     strings.ReplaceAll(gp("MAIN errors"), "package p", "package errors"),
		"deep/er/util/util.gp": strings.ReplaceAll(gp("MAIN util"), "package p", "package util"),
		"dep/gocpp.mod":        "module example.com/dep\n",
		"dep/util/util.gp":     strings.ReplaceAll(gp("DEP util"), "package p", "package util"),
		"dep/only/only.gp":     strings.ReplaceAll(gp("DEP only"), "package p", "package only"),
		"dep/json/json.gp":     strings.ReplaceAll(gp("DEP json"), "package p", "package json"),
		"dep/errors/errors.gp": strings.ReplaceAll(gp("DEP errors"), "package p", "package errors"),
		"dep/o/oth/oth.gp":     strings.ReplaceAll(gp("DEP other"), "package p", "package oth"),
		"mid/dep2/oth/oth.gp":  strings.ReplaceAll(gp("MAIN oth"), "package p", "package oth"),
	})
	env := &Env{Dir: dir, GOOS: "linux", GOARCH: "amd64", Stdlib: std, StdlibDir: "$STD"}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"json"}, "is the STD one"},          // partial path: std first
		{[]string{"json", "F"}, "F is in STD."},       // two arguments
		{[]string{"json.F"}, "F is in STD."},          // symbol
		{[]string{"encoding/json"}, "is the STD one"}, // full path
		{[]string{"errors"}, "is the STD errors one"}, // full path of std, also a main folder
		{[]string{"util"}, "is the MAIN util one"},    // main module before a required one, deeper or not
		{[]string{"oth"}, "is the MAIN oth one"},      // main (depth 3) before dep (depth 2)
		{[]string{"only"}, "is the DEP only one"},
		{[]string{"example.com/dep/util"}, "is the DEP util one"},
		{[]string{"example.com/m/json"}, "is the MAIN json one"},
		{[]string{"example.com/dep/json"}, "is the DEP json one"},
	} {
		out, err := runIn(t, env, Options{}, c.args...)
		if err != nil || !strings.Contains(out, c.want) {
			t.Errorf("%v: %v\n%s", c.args, err, out)
		}
	}
	// A symbol that only a later place has is found after the earlier places
	// have said no (json.Only is in no std package).
	writeTree(t, dir, map[string]string{"dep/json/json.gp": "package json\n\n// Only is in DEP.\nfunc Only() {}\n"})
	out, err := runIn(t, env, Options{}, "json.Only")
	if err != nil || !strings.Contains(out, "Only is in DEP.") {
		t.Errorf("json.Only: %v\n%s", err, out)
	}
}

// The module cache is the last place: a required module that is not replaced
// or vendored is read from the folder GOCPP_MODCACHE names.
func TestLookupModuleCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("GOCPP_MODCACHE", cache)
	dir := t.TempDir()
	std := fstest.MapFS{
		"cachedstd/c.gp": {Data: []byte("// Package cachedstd is STD.\npackage cachedstd\n")},
	}
	writeTree(t, cache, map[string]string{
		"example.com/cached@v1.0.0/gocpp.mod":        "module example.com/cached\n",
		"example.com/cached@v1.0.0/lib/lib.gp":       "// Package lib is the CACHE one.\npackage lib\n\n// Cached is in the cache.\nfunc Cached() {}\n",
		"example.com/cached@v1.0.0/shared/shared.gp": "// Package shared is the CACHE one.\npackage shared\n",
		"example.com/cached@v1.0.0/cachedstd/c.gp":   "// Package cachedstd is the CACHE one.\npackage cachedstd\n",
	})
	// gocpp.sum: the hashes of the cached module.
	modDir := filepath.Join(cache, "example.com", "cached@v1.0.0")
	h, err := modfetch.HashDir(context.Background(), modDir, "example.com/cached", "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	hm, err := modfetch.HashFile(filepath.Join(modDir, "gocpp.mod"), "gocpp.mod")
	if err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{
		"gocpp.mod":        "module example.com/m\n\nrequire example.com/cached v1.0.0\n",
		"gocpp.sum":        "example.com/cached v1.0.0 " + h + "\nexample.com/cached v1.0.0/gocpp.mod " + hm + "\n",
		"shared/shared.gp": "// Package shared is the MAIN one.\npackage shared\n",
	})
	env := &Env{Dir: dir, GOOS: "linux", GOARCH: "amd64", Stdlib: std, StdlibDir: "$STD"}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"lib"}, "Package lib is the CACHE one."},
		{[]string{"lib.Cached"}, "Cached is in the cache."},
		{[]string{"example.com/cached/lib"}, "Package lib is the CACHE one."},
		{[]string{"shared"}, "Package shared is the MAIN one."},
		{[]string{"cachedstd"}, "Package cachedstd is STD."},
	} {
		out, err := runIn(t, env, Options{}, c.args...)
		if err != nil || !strings.Contains(out, c.want) {
			t.Errorf("%v: %v\n%s", c.args, err, out)
		}
	}
}

// A folder that cannot be read is told on Env.Warn, as go doc logs it, and
// the scan goes on.
func TestUnreadableFolderWarns(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permissions do not apply")
	}
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"gocpp.mod":     "module example.com/m\n",
		"a/a.gp":        "// Package a is here.\npackage a\n",
		"locked/l.gp":   "package locked\n",
		"locked/x/x.gp": "package x\n",
		"zed/z.gp":      "// Package zed is here.\npackage zed\n",
	})
	if err := os.Chmod(filepath.Join(dir, "locked"), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(dir, "locked"), 0o755)
	var warn bytes.Buffer
	env := &Env{Dir: dir, GOOS: "linux", GOARCH: "amd64", Warn: &warn}
	out, err := runIn(t, env, Options{}, "zed")
	if err != nil || !strings.Contains(out, "Package zed is here.") {
		t.Fatalf("%v\n%s", err, out)
	}
	want := "gocpp doc: open " + filepath.Join(dir, "locked") + ": permission denied\n"
	if !strings.Contains(warn.String(), want) {
		t.Errorf("warnings %q, want %q", warn.String(), want)
	}
	// Without Warn nothing is printed (and nothing breaks).
	env.Warn = nil
	if _, err := runIn(t, env, Options{}, "zed"); err != nil {
		t.Error(err)
	}
}

// A macro line right after the package clause (the package clause is read
// before the file is parsed) works through ReadDir and Render too.
func TestReadDirMacroAfterPackage(t *testing.T) {
	env := &Env{GOOS: "linux", GOARCH: "amd64"}
	pkg, err := env.ReadDir(os.DirFS(modDir(t)), "derivefirst", "example.com/gp/derivefirst")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Render(&buf, pkg, "Level", Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "#Derive(String)\ntype Level int") {
		t.Errorf("got\n%s", buf.String())
	}
}

// Examples of an external test package (package p_test) are listed, and
// those of a third package are not.
func TestExamplesExternalTestPackage(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"x/x.gp":      "package x\n\nfunc F() {}\n",
		"x/a_test.gp": "package x\n\nfunc ExampleF() {}\n",
		"x/b_test.gp": "package x_test\n\nfunc ExampleF_second() {}\n",
		"x/c_test.gp": "package other\n\nfunc ExampleF_third() {}\n",
	})
	out, err := runIn(t, &Env{Dir: dir, GOOS: "linux", GOARCH: "amd64"}, Options{Examples: true}, "x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ExampleF\n") || !strings.Contains(out, "ExampleF_second") || strings.Contains(out, "ExampleF_third") {
		t.Errorf("got\n%s", out)
	}
}

// A folder with only test files is not a package; the message says so.
func TestOnlyTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"gocpp.mod": "module example.com/m\n", "t/t_test.gp": "package t\n"})
	_, err := runIn(t, &Env{Dir: dir, GOOS: "linux", GOARCH: "amd64"}, Options{}, "./t")
	if err == nil || !strings.Contains(err.Error(), "no source-code package in directory") || !strings.Contains(err.Error(), "(only _test.gp files)") {
		t.Errorf("error %v", err)
	}
}

// A two-argument std path that gocpp does not have keeps the reason.
func TestTwoArgStdNotAvailable(t *testing.T) {
	dir := t.TempDir()
	_, err := runIn(t, &Env{Dir: dir, GOOS: "linux", GOARCH: "amd64", Stdlib: fstest.MapFS{}}, Options{}, "container/list", "List")
	if err == nil || !strings.Contains(err.Error(), "not available yet in gocpp's standard library") {
		t.Errorf("error %v", err)
	}
}

// A name that only a folder of test files matches gets the same reason as the
// folder path.
func TestOnlyTestFilesPartialPath(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"gocpp.mod": "module example.com/m\n", "t/tonly/t_test.gp": "package tonly\n"})
	for _, args := range [][]string{{"tonly"}, {"tonly", "X"}, {"tonly.X"}, {"./t/tonly"}} {
		_, err := runIn(t, &Env{Dir: dir, GOOS: "linux", GOARCH: "amd64", Stdlib: fstest.MapFS{}}, Options{}, args...)
		if err == nil || !strings.Contains(err.Error(), "no source-code package in directory") || !strings.Contains(err.Error(), "(only _test.gp files)") {
			t.Errorf("%v: error %v", args, err)
		}
	}
}
