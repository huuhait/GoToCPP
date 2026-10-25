package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// TestRealGOROOT runs the dry run on the GOROOT of the Go that runs the test
// (or GOCPP_GOROOT) and pins a few facts. It is skipped when there is no Go
// source tree, and when the tree is not go1.25, the version the facts were
// pinned for.
func TestRealGOROOT(t *testing.T) {
	goroot := os.Getenv("GOCPP_GOROOT")
	if goroot == "" {
		goroot = runtime.GOROOT()
	}
	if goroot == "" {
		t.Skip("no GOROOT")
	}
	version, err := readVersion(goroot)
	if err != nil {
		t.Skipf("no Go source tree at %s: %v", goroot, err)
	}
	if !strings.HasPrefix(version, "go1.25.") {
		t.Skipf("the facts are pinned for go1.25, the tree is %s", version)
	}

	res, err := dryRun(&options{
		goroot:   goroot,
		goos:     "linux",
		goarch:   "amd64",
		packages: append([]string{}, defaultPackages...),
		closure:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ToolDefaults.Verified {
		t.Errorf("%s: the tool tag table did not verify", version)
	}
	var runtimePkg *Package
	var fips bool
	linknames := 0
	for i := range res.Packages {
		p := &res.Packages[i]
		if p.Path == "runtime" {
			runtimePkg = p
		}
		if p.InClosure {
			linknames += len(p.Linknames)
			for _, ln := range p.Linknames {
				if ln.Target == "go:fipsinfo" {
					fips = true
				}
			}
		}
	}
	if runtimePkg == nil {
		t.Fatal("no runtime package")
	}
	if len(runtimePkg.Bodyless) == 0 {
		t.Error("the runtime has no bodyless functions")
	}
	if linknames == 0 {
		t.Error("the net/http + crypto/tls closure has no //go:linkname")
	}
	if !fips {
		t.Error("go:fipsinfo is not a linkname target in the closure")
	}
	if res.Summary.Closure.SynctestOnlyTargets != 9 {
		t.Errorf("synctest-only targets = %d, want 9", res.Summary.Closure.SynctestOnlyTargets)
	}
	if res.Summary.Closure.RuntimePushTargets != 151 {
		t.Errorf("runtime push targets = %d, want 151", res.Summary.Closure.RuntimePushTargets)
	}
	if got := res.Summary.Closure.BodylessTargetsOutsideRuntime; got != 9 {
		t.Errorf("bodyless targets not provided by the runtime = %d, want 9", got)
	}
	if got := res.Summary.Closure.BodylessTargetsUnprovided; got != 5 {
		t.Errorf("bodyless targets provided by no push = %d, want 5", got)
	}
	if !strings.HasPrefix(res.GOROOT, goroot) {
		t.Errorf("GOROOT = %q, want %q", res.GOROOT, goroot)
	}
}

// TestRealGOROOTTypeCheck type-checks the default package set on the GOROOT of
// the Go that runs the test (or GOCPP_GOROOT) and pins the facts of the
// committed gostd/dryrun/go1.25.14/linux-amd64-typecheck.json. It is skipped
// when there is no Go source tree, and when the tree is not go1.25.
func TestRealGOROOTTypeCheck(t *testing.T) {
	goroot := os.Getenv("GOCPP_GOROOT")
	if goroot == "" {
		goroot = runtime.GOROOT()
	}
	if goroot == "" {
		t.Skip("no GOROOT")
	}
	version, err := readVersion(goroot)
	if err != nil {
		t.Skipf("no Go source tree at %s: %v", goroot, err)
	}
	if !strings.HasPrefix(version, "go1.25.") {
		t.Skipf("the facts are pinned for go1.25, the tree is %s", version)
	}

	res, err := dryRun(&options{
		goroot:   goroot,
		goos:     "linux",
		goarch:   "amd64",
		packages: append([]string{}, defaultPackages...),
		closure:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	tc, err := typeCheck(goroot, res)
	if err != nil {
		t.Fatal(err)
	}
	s := tc.Summary
	// The selection of 16.1b type-checks cleanly under the gocpp constraints:
	// the translation gaps are bodyless declarations, linknames and the
	// intrinsics, not type errors.
	if s.RealErrors != 0 || s.FailedPackages != 0 {
		for _, p := range tc.Packages {
			for _, e := range p.Errors {
				t.Logf("%s %s:%d: %s", p.Path, e.File, e.Line, e.Msg)
			}
		}
		t.Errorf("real type errors = %d in %d packages, want 0", s.RealErrors, s.FailedPackages)
	}
	if s.ImportFailures != 0 {
		t.Errorf("import failures = %d, want 0: %+v", s.ImportFailures, tc.ImportFailures)
	}
	if s.PackagesInSet != 177 {
		t.Errorf("packages in the set = %d, want 177", s.PackagesInSet)
	}
	if s.Checked != s.Packages {
		t.Errorf("checked = %d of %d packages", s.Checked, s.Packages)
	}
	if s.BodylessInSet != 505 {
		t.Errorf("bodyless declarations = %d, want 505", s.BodylessInSet)
	}
	if s.IntrinsicsInSet != 5 {
		t.Errorf("compiler intrinsics = %d, want 5", s.IntrinsicsInSet)
	}
	if s.CgoFilesInSet != 5 {
		t.Errorf("cgo files = %d, want 5", s.CgoFilesInSet)
	}
	if tc.GoLanguageVersion != "go1.25" || tc.Sizes != "gc/amd64" {
		t.Errorf("config = %q %q", tc.GoLanguageVersion, tc.Sizes)
	}
}
