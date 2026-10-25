package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

// fakeTypeCheckPackages is the fake tree's package set for the type check:
// a clean package with a bodyless function, a cgo file and a gocpp-dropped
// file (example), the four kinds of expected gap and the known type error.
var fakeTypeCheckPackages = []string{
	"example",
	"example/typeerr",
	"example/gcdrop",
	"example/cgodrop",
	"example/importdrop",
	"example/missingimport",
	"example/gconly",
	"internal/abi",
}

func fakeTypeCheck(t *testing.T) *TypeCheckResult {
	t.Helper()
	opts := &options{
		goroot:      fakeGOROOT,
		goos:        "linux",
		goarch:      "amd64",
		packages:    append([]string{}, fakeTypeCheckPackages...),
		packagesSet: true,
		closure:     false,
	}
	res, err := dryRun(opts)
	if err != nil {
		t.Fatal(err)
	}
	tc, err := typeCheck(opts.goroot, res)
	if err != nil {
		t.Fatal(err)
	}
	return tc
}

// pkgOf returns the record of one package path.
func pkgOf(t *testing.T, tc *TypeCheckResult, path string) TypeCheckPackage {
	t.Helper()
	for _, p := range tc.Packages {
		if p.Path == path {
			return p
		}
	}
	t.Fatalf("no package %s in the result", path)
	return TypeCheckPackage{}
}

// TestTypeCheckFakeTree checks the split between real type errors and the
// gaps the constraints themselves cause, with one package per case.
func TestTypeCheckFakeTree(t *testing.T) {
	tc := fakeTypeCheck(t)

	// A clean package: the bodyless function is legal Go, the cgo file and
	// the gc-only file are dropped (reported, not errors).
	ex := pkgOf(t, tc, "example")
	if ex.Status != "checked" || len(ex.Errors) != 0 || len(ex.Expected) != 0 {
		t.Errorf("example = %+v", ex)
	}
	if ex.Bodyless != 2 {
		t.Errorf("example bodyless = %d, want 2", ex.Bodyless)
	}
	if len(ex.CgoFiles) != 1 || ex.CgoFiles[0] != "cgo.go" {
		t.Errorf("example cgo files = %v, want cgo.go", ex.CgoFiles)
	}
	if len(ex.DroppedByGocpp) != 1 || ex.DroppedByGocpp[0] != "gc_only.go" {
		t.Errorf("example dropped files = %v, want gc_only.go", ex.DroppedByGocpp)
	}

	// The known type error is the only real error of the run.
	if te := pkgOf(t, tc, "example/typeerr"); len(te.Errors) != 1 {
		t.Errorf("typeerr errors = %+v", te.Errors)
	} else if te.Errors[0].Line != 5 || te.Errors[0].Kind != "other" {
		t.Errorf("typeerr error = %+v, want line 5 kind other", te.Errors[0])
	}

	// The three expected gaps.
	checkGap := func(path, kind, name string) {
		t.Helper()
		p := pkgOf(t, tc, path)
		if len(p.Errors) != 0 {
			t.Errorf("%s has real errors: %+v", path, p.Errors)
		}
		if len(p.Expected) != 1 {
			t.Fatalf("%s expected gaps = %+v, want one %s", path, p.Expected, kind)
		}
		if p.Expected[0].Kind != kind || p.Expected[0].Name != name {
			t.Errorf("%s gap = %+v, want %s %s", path, p.Expected[0], kind, name)
		}
	}
	checkGap("example/gcdrop", "compiler_tag_dropped_file", "FromGC")
	checkGap("example/cgodrop", "cgo_file", "FromCgo")
	checkGap("example/importdrop", "import_dropped_by_compiler_tag", "example/gconly")

	// A package named in the set but empty under the constraints is a status,
	// not a crash.
	if p := pkgOf(t, tc, "example/gconly"); p.Status != "no_files" || !p.InSet {
		t.Errorf("gconly = %+v", p)
	}

	// A package that is not found is a real finding at the import site, not a
	// crash of the tool; the import failure says why.
	mi := pkgOf(t, tc, "example/missingimport")
	if len(mi.Errors) != 1 || mi.Errors[0].Kind != "import_not_found" {
		t.Errorf("missingimport errors = %+v", mi.Errors)
	}
	if len(mi.ImportsFailed) != 1 || mi.ImportsFailed[0] != "example/nothere" {
		t.Errorf("missingimport failed imports = %v", mi.ImportsFailed)
	}
	if len(tc.ImportFailures) != 2 {
		t.Errorf("import failures = %+v", tc.ImportFailures)
	}

	// A package loaded only as an import of the set is reported with in_set
	// false; it is checked all the same.
	if p := pkgOf(t, tc, "example/dep"); p.InSet || p.Status != "checked" {
		t.Errorf("dep = %+v", p)
	}

	// The intrinsics and cgo files are counted from the syntax dry run; they
	// are not errors.
	if p := pkgOf(t, tc, "internal/abi"); p.Intrinsics != 2 || len(p.Errors) != 0 {
		t.Errorf("internal/abi = %+v", p)
	}

	s := tc.Summary
	if s.PackagesInSet != len(fakeTypeCheckPackages) {
		t.Errorf("packages in set = %d, want %d", s.PackagesInSet, len(fakeTypeCheckPackages))
	}
	if s.RealErrors != 2 || s.FailedPackages != 2 {
		t.Errorf("real errors = %d in %d packages, want 2 in 2", s.RealErrors, s.FailedPackages)
	}
	if s.ExpectedGaps != 3 {
		t.Errorf("expected gaps = %d, want 3", s.ExpectedGaps)
	}
	if s.IntrinsicsInSet != 2 || s.CgoFilesInSet != 2 || s.DroppedFilesInSet != 3 {
		t.Errorf("expected-gap counts = %+v", s)
	}
	if s.ImportFailures != 2 {
		t.Errorf("import failures = %d, want 2", s.ImportFailures)
	}
	if tc.GoLanguageVersion != "go1.25" || tc.Sizes != "gc/amd64" {
		t.Errorf("config = %q %q", tc.GoLanguageVersion, tc.Sizes)
	}
}

// TestExpectedGapClassification covers the classification of the messages
// go/types produces for the cases that a fake tree cannot easily make: an
// import whose dependency failed or was dropped, and an undefined name from a
// dropped file.
func TestExpectedGapClassification(t *testing.T) {
	c := &typeChecker{failures: []ImportFailure{
		{From: "a", Import: "dep", Kind: "failed"},
		{From: "a", Import: "gone", Kind: "not_found"},
		{From: "a", Import: "empty", Kind: "dropped_by_compiler_tag"},
	}}
	dropped := map[string]string{"FromGC": "gc_decl.go"}
	cgo := map[string]string{"FromCgo": "cgo_decl.go"}
	cases := []struct {
		msg  string
		kind string
		name string
		ok   bool
	}{
		{"undefined: FromGC", "compiler_tag_dropped_file", "FromGC", true},
		{"undefined: FromCgo", "cgo_file", "FromCgo", true},
		{"undefined: Something", "", "", false},
		{"could not import empty (no files)", "import_dropped_by_compiler_tag", "empty", true},
		{"could not import dep (boom)", "dependency_failed", "dep", true},
		{"could not import gone (not found)", "", "", false},
		{"imported and not used: \"fmt\"", "", "", false},
	}
	for _, tc := range cases {
		gap, ok := c.expectedGap("a", "src/a/a.go", 1, 2, tc.msg, dropped, cgo)
		if ok != tc.ok || gap.Kind != tc.kind || gap.Name != tc.name {
			t.Errorf("%q: got (%+v, %v), want kind %q name %q ok %v", tc.msg, gap, ok, tc.kind, tc.name, tc.ok)
		}
	}
}

// TestTypeCheckFakeTreeReproducible checks that two runs of the same tree
// serialize to the same bytes: the committed JSON is diffed by eye.
func TestTypeCheckFakeTreeReproducible(t *testing.T) {
	first, err := json.Marshal(fakeTypeCheck(t))
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(fakeTypeCheck(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two runs of the type check are not byte-identical")
	}
	if bytes.Contains(first, []byte("null")) {
		t.Error("the JSON has a null: a later version's file is diffed against this one")
	}
}

// TestReadTypeCheckSiblingsFiltersGoVersion checks that only type-check files
// of the run's Go version are mixed into the README table, and that the fresh
// run wins over a stale file for the same target (as readSiblings does).
func TestReadTypeCheckSiblingsFiltersGoVersion(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "README.md")
	mk := func(version, goos, goarch, sizes string) *TypeCheckResult {
		return &TypeCheckResult{
			Schema:    typeCheckSchemaVersion,
			Tool:      typeCheckTool,
			GoVersion: version,
			Sizes:     sizes,
			Tags:      &TagSet{GOOS: goos, GOARCH: goarch},
		}
	}
	files := map[string]*TypeCheckResult{
		"linux-amd64-typecheck.json":   mk("go1.25.14", "linux", "amd64", "stale"),
		"windows-amd64-typecheck.json": mk("go1.25.14", "windows", "amd64", "windows"),
		"darwin-amd64-typecheck.json":  mk("go1.26.0", "darwin", "amd64", "other version"),
	}
	for name, tc := range files {
		if err := writeTypeCheckJSON(filepath.Join(dir, name), tc); err != nil {
			t.Fatal(err)
		}
	}
	fresh := mk("go1.25.14", "linux", "amd64", "fresh")
	tcs, err := readTypeCheckSiblings(md, &Result{GoVersion: "go1.25.14"}, fresh)
	if err != nil {
		t.Fatal(err)
	}
	if len(tcs) != 2 {
		t.Fatalf("siblings = %d, want 2 (windows and the fresh linux run)", len(tcs))
	}
	for _, tc := range tcs {
		if tc.Tags.GOOS == "linux" && tc.Sizes != "fresh" {
			t.Errorf("the stale linux file overrode the fresh run: %+v", tc)
		}
		if tc.GoVersion != "go1.25.14" {
			t.Errorf("a file of another Go version was read: %+v", tc)
		}
	}
}

// TestTypeCheckJSONRoundTrip checks that the committed format reads back.
func TestTypeCheckJSONRoundTrip(t *testing.T) {
	tc := fakeTypeCheck(t)
	path := t.TempDir() + "/linux-amd64-typecheck.json"
	if err := writeTypeCheckJSON(path, tc); err != nil {
		t.Fatal(err)
	}
	back, err := readTypeCheckJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if back.Summary.RealErrors != tc.Summary.RealErrors || len(back.Packages) != len(tc.Packages) {
		t.Errorf("round trip changed the result: %+v", back.Summary)
	}
}
