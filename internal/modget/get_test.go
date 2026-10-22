package modget

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/modfetch"
)

var (
	libA = v("example.com/a", "v1.0.0", "v1.1.0", "v1.2.0", "v1.2.1", "v1.3.0", "v2.0.0+incompatible")
	libB = v("example.com/b", "v1.0.0", "v1.1.0", "v1.2.0")
)

func TestGetAddLatest(t *testing.T) {
	e := newEnv(t, appMod, cat(libA, libB)...)
	r := e.mustGet("example.com/a")
	// An incompatible version (v2 of a module without /v2) is never "latest" while
	// there is a compatible release, as in Go.
	if got := changes(r); got != "added example.com/a v1.3.0" {
		t.Fatalf("changes = %q", got)
	}
	e = newEnv(t, appMod, cat(libA, libB)...)
	if got := changes(e.mustGet("example.com/a@v2.0.0+incompatible")); got != "added example.com/a v2.0.0+incompatible" {
		t.Errorf("explicit +incompatible: %q", got)
	}
}

func TestGetAddVersion(t *testing.T) {
	e := newEnv(t, appMod, cat(libA, libB)...)
	r := e.mustGet("example.com/a@v1.1.0")
	if got := changes(r); got != "added example.com/a v1.1.0" {
		t.Fatalf("changes = %q", got)
	}
	if got := e.mod(); got != appMod+"\nrequire example.com/a v1.1.0\n" {
		t.Errorf("gocpp.mod = %q", got)
	}
	sum := e.sum()
	for _, want := range []string{"example.com/a v1.1.0 h1:", "example.com/a v1.1.0/go.mod h1:"} {
		if !strings.Contains(sum, want) {
			t.Errorf("gocpp.sum lacks %q:\n%s", want, sum)
		}
	}
	if !r.ModChanged || !r.SumChanged {
		t.Errorf("result = %+v", r)
	}
	if len(r.Downloaded) != 1 || r.Downloaded[0] != "example.com/a@v1.1.0" {
		t.Errorf("downloaded = %v", r.Downloaded)
	}
	// The module is in the cache.
	if _, err := os.Stat(filepath.Join(e.cache, "example.com", "a@v1.1.0", "x.go")); err != nil {
		t.Errorf("not in the cache: %v", err)
	}
}

func TestGetUpgradeAndDowngrade(t *testing.T) {
	e := newEnv(t, appMod, cat(libA, libB)...)
	e.mustGet("example.com/a@v1.0.0")
	if got := changes(e.mustGet("example.com/a@v1.2.0")); got != "upgraded example.com/a v1.0.0 => v1.2.0" {
		t.Errorf("upgrade: %q", got)
	}
	if got := changes(e.mustGet("example.com/a@v1.1.0")); got != "downgraded example.com/a v1.2.0 => v1.1.0" {
		t.Errorf("downgrade: %q", got)
	}
	if got := changes(e.mustGet("example.com/a@upgrade")); got != "upgraded example.com/a v1.1.0 => v1.3.0" {
		t.Errorf("upgrade query: %q", got)
	}
	if got := e.requires(); len(got) != 1 || got[0] != "example.com/a@v1.3.0" {
		t.Errorf("requires = %v", got)
	}
}

func TestGetQueries(t *testing.T) {
	cases := []struct{ query, want string }{
		{"latest", "v1.3.0"},
		{"v1", "v1.3.0"},
		{"v1.2", "v1.2.1"},
		{"<v1.2.0", "v1.1.0"},
		{"<=v1.2.0", "v1.2.0"},
		{">v1.2.0", "v1.2.1"},
		{">=v1.2.0", "v1.2.0"},
		{"v1.0.0", "v1.0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			e := newEnv(t, appMod, cat(libA)...)
			r := e.mustGet("example.com/a@" + tc.query)
			if len(r.Changes) != 1 || r.Changes[0].New != tc.want {
				t.Errorf("a@%s = %q, want %s", tc.query, changes(r), tc.want)
			}
		})
	}
}

func TestGetPatch(t *testing.T) {
	e := newEnv(t, appMod+"\nrequire example.com/a v1.2.0\n", cat(libA)...)
	r := e.mustGet("example.com/a@patch")
	if got := changes(r); got != "upgraded example.com/a v1.2.0 => v1.2.1" {
		t.Errorf("patch: %q", got)
	}
	// Not required yet: nothing to patch.
	e = newEnv(t, appMod, cat(libA)...)
	if _, err := e.get("example.com/a@patch"); err == nil || !strings.Contains(err.Error(), "not required yet") {
		t.Errorf("patch of a new module: %v", err)
	}
}

func TestGetDefaultQueryIsUpgradeForARequiredModule(t *testing.T) {
	e := newEnv(t, appMod+"\nrequire example.com/b v1.0.0\n", cat(libB)...)
	if got := changes(e.mustGet("example.com/b")); got != "upgraded example.com/b v1.0.0 => v1.2.0" {
		t.Errorf("got %q", got)
	}
	// And a second time: nothing changes, and the files stay as they are.
	mod, sum := e.mod(), e.sum()
	r := e.mustGet("example.com/b")
	if len(r.Changes) != 0 || r.ModChanged || r.SumChanged || e.mod() != mod || e.sum() != sum {
		t.Errorf("second get: %+v", r)
	}
}

func TestGetNone(t *testing.T) {
	e := newEnv(t, appMod, cat(libA, libB)...)
	e.mustGet("example.com/a@v1.0.0", "example.com/b@v1.0.0")
	r := e.mustGet("example.com/a@none")
	if got := changes(r); got != "removed example.com/a v1.0.0" {
		t.Errorf("none: %q", got)
	}
	if got := e.requires(); len(got) != 1 || got[0] != "example.com/b@v1.0.0" {
		t.Errorf("requires = %v", got)
	}
	// A module that is not required: a note, no error, nothing changes.
	r = e.mustGet("example.com/a@none")
	if len(r.Changes) != 0 || len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "not required") {
		t.Errorf("none again: %+v", r)
	}
}

func TestGetIndirectRequirementsAreWritten(t *testing.T) {
	e := newEnv(t, appMod, cat(
		[]tmod{{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.1.0"}}},
		libB)...)
	r := e.mustGet("example.com/a@v1.0.0")
	if got := changes(r); got != "added example.com/a v1.0.0; added example.com/b v1.1.0" {
		t.Errorf("changes = %q", got)
	}
	got := e.requires()
	if len(got) != 2 || got[0] != "example.com/a@v1.0.0" || got[1] != "example.com/b@v1.1.0 indirect" {
		t.Errorf("requires = %v", got)
	}
	// The go.mod hash of every module read is in gocpp.sum, and the zip hash of the downloaded ones.
	for _, want := range []string{"example.com/a v1.0.0 h1:", "example.com/a v1.0.0/go.mod", "example.com/b v1.1.0 h1:", "example.com/b v1.1.0/go.mod"} {
		if !strings.Contains(e.sum(), want) {
			t.Errorf("gocpp.sum lacks %q:\n%s", want, e.sum())
		}
	}
	// Naming the indirect module makes it direct.
	e.mustGet("example.com/b@v1.1.0")
	got = e.requires()
	if len(got) != 2 || got[1] != "example.com/b@v1.1.0" {
		t.Errorf("after naming b: %v", got)
	}
}

func TestGetUpgradeOfAnIndirectModule(t *testing.T) {
	e := newEnv(t, appMod, cat(
		[]tmod{{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"}}},
		libB)...)
	e.mustGet("example.com/a@v1.0.0")
	r := e.mustGet("example.com/b@v1.2.0")
	if got := changes(r); got != "upgraded example.com/b v1.0.0 => v1.2.0" {
		t.Errorf("changes = %q", got)
	}
	if got := e.requires(); got[0] != "example.com/a@v1.0.0" || got[1] != "example.com/b@v1.2.0" {
		t.Errorf("requires = %v", got)
	}
}

func TestGetDowngradeLowersDependents(t *testing.T) {
	mods := cat(
		[]tmod{
			{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"}},
			{Path: "example.com/a", Version: "v1.1.0", Reqs: []string{"example.com/b@v1.1.0"}},
			{Path: "example.com/a", Version: "v1.2.0", Reqs: []string{"example.com/b@v1.2.0"}},
		}, libB)
	e := newEnv(t, appMod, mods...)
	e.mustGet("example.com/a@v1.2.0")
	// b goes to v1.1.0, so a, which needs v1.2.0, goes to v1.1.0 as well.
	r := e.mustGet("example.com/b@v1.1.0")
	if got := changes(r); got != "downgraded example.com/a v1.2.0 => v1.1.0; downgraded example.com/b v1.2.0 => v1.1.0" {
		t.Errorf("changes = %q", got)
	}
	if got := e.requires(); got[0] != "example.com/a@v1.1.0" || got[1] != "example.com/b@v1.1.0" {
		t.Errorf("requires = %v", got)
	}
}

func TestGetNoneRemovesWhatNeedsIt(t *testing.T) {
	mods := cat([]tmod{{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"}}}, libB)
	e := newEnv(t, appMod, mods...)
	e.mustGet("example.com/a@v1.0.0")
	r := e.mustGet("example.com/b@none")
	// a has no lower version: it goes too.
	if got := changes(r); got != "removed example.com/a v1.0.0; removed example.com/b v1.0.0" {
		t.Errorf("changes = %q", got)
	}
	if got := e.requires(); len(got) != 0 {
		t.Errorf("requires = %v\n%s", got, e.mod())
	}
}

func TestGetUpgradeAll(t *testing.T) {
	mods := cat(
		[]tmod{
			{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"}},
			{Path: "example.com/a", Version: "v1.1.0", Reqs: []string{"example.com/b@v1.0.0"}},
		}, libB)
	e := newEnv(t, appMod+"\nrequire example.com/a v1.0.0\n", mods...)
	r, err := e.getOpts(Options{Upgrade: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if got := changes(r); got != "upgraded example.com/a v1.0.0 => v1.1.0; upgraded example.com/b v1.0.0 => v1.2.0" {
		t.Errorf("changes = %q", got)
	}
	got := e.requires()
	if got[0] != "example.com/a@v1.1.0" || got[1] != "example.com/b@v1.2.0 indirect" {
		t.Errorf("requires = %v", got)
	}
}

func TestGetUpgradePatchAll(t *testing.T) {
	e := newEnv(t, appMod+"\nrequire example.com/a v1.2.0\n", cat(libA)...)
	r, err := e.getOpts(Options{Upgrade: "patch"})
	if err != nil {
		t.Fatal(err)
	}
	if got := changes(r); got != "upgraded example.com/a v1.2.0 => v1.2.1" {
		t.Errorf("changes = %q", got)
	}
}

func TestGetUpgradeNamedModuleUpgradesItsDependencies(t *testing.T) {
	mods := cat(
		[]tmod{
			{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"}},
			{Path: "example.com/a", Version: "v1.1.0", Reqs: []string{"example.com/b@v1.0.0"}},
			{Path: "example.com/c", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"}},
			{Path: "example.com/c", Version: "v1.1.0"},
		}, libB)
	e := newEnv(t, appMod+"\nrequire example.com/a v1.0.0\nrequire example.com/c v1.0.0\n", mods...)
	r, err := e.getOpts(Options{Args: []string{"example.com/a"}, Upgrade: "all"})
	if err != nil {
		t.Fatal(err)
	}
	// a and its dependency b go up; c is not a dependency of a: it stays.
	if got := changes(r); got != "upgraded example.com/a v1.0.0 => v1.1.0; upgraded example.com/b v1.0.0 => v1.2.0" {
		t.Errorf("changes = %q", got)
	}
}

func TestGetTestsFlagChangesNothing(t *testing.T) {
	e1 := newEnv(t, appMod, cat(libB)...)
	e2 := newEnv(t, appMod, cat(libB)...)
	if _, err := e1.getOpts(Options{Args: []string{"example.com/b"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e2.getOpts(Options{Args: []string{"example.com/b"}, Tests: true}); err != nil {
		t.Fatal(err)
	}
	if e1.mod() != e2.mod() || e1.sum() != e2.sum() {
		t.Error("-t changed the result")
	}
}

func TestGetExcludedVersion(t *testing.T) {
	e := newEnv(t, appMod+"\nexclude example.com/a v1.3.0\n", cat(libA)...)
	if _, err := e.get("example.com/a@v1.3.0"); err == nil || !strings.Contains(err.Error(), "excluded by an exclude line") {
		t.Errorf("excluded: %v", err)
	}
	// "latest" skips it.
	r := e.mustGet("example.com/a@v1")
	if got := changes(r); got != "added example.com/a v1.2.1" {
		t.Errorf("v1 with v1.3.0 excluded: %q", got)
	}
}

// A hand-written exclude line may spell a version of a module without /vN as v2.0.0 (Go rejects
// that form; gocpp reads it): it excludes v2.0.0+incompatible, which is what "get" turns an
// explicit v2.0.0 into, and the other spelling excludes both as well.
func TestGetExcludedIncompatibleSpellings(t *testing.T) {
	for _, c := range []struct{ name, exclude, query string }{
		{"plain exclude, plain query", "v2.0.0", "v2.0.0"},
		{"plain exclude, explicit +incompatible", "v2.0.0", "v2.0.0+incompatible"},
		{"explicit exclude, plain query", "v2.0.0+incompatible", "v2.0.0"},
		{"explicit exclude, explicit query", "v2.0.0+incompatible", "v2.0.0+incompatible"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, appMod+"\nexclude example.com/a "+c.exclude+"\n", cat(libA)...)
			_, err := e.get("example.com/a@" + c.query)
			if err == nil || !strings.Contains(err.Error(), "excluded by an exclude line") {
				t.Errorf("get example.com/a@%s with exclude %s: %v", c.query, c.exclude, err)
			}
		})
	}
}

// With a local replace, an explicit version is only a name, and the exclude line still applies to it,
// also when the query has build metadata (v1.2.3+meta is v1.2.3).
func TestGetLocalReplaceHonoursExclude(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "gocpp.mod"), []byte("module example.com/a\n"), 0o644)
	mod := appMod + "\nreplace example.com/a => " + filepath.ToSlash(dir) + "\nexclude example.com/a v1.2.3\n"
	for _, query := range []string{"v1.2.3", "v1.2.3+meta"} {
		e := newEnv(t, mod)
		if _, err := e.get("example.com/a@" + query); err == nil || !strings.Contains(err.Error(), "excluded by an exclude line") {
			t.Errorf("get example.com/a@%s: %v", query, err)
		}
	}
}

func TestGetRetractedWarning(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/r", Version: "v1.0.0"},
		{Path: "example.com/r", Version: "v1.1.0"},
		{Path: "example.com/r", Version: "v1.2.0", Extra: "// Published by mistake.\nretract v1.1.0\nretract [v1.0.0, v1.0.0] // too old\n"},
	}
	e := newEnv(t, appMod, mods...)
	r := e.mustGet("example.com/r@v1.1.0")
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "example.com/r@v1.1.0 is retracted: Published by mistake.") {
		t.Errorf("warnings = %q", r.Warnings)
	}
	// "latest" does not pick a retracted version.
	// Every version below v1.2.0 is retracted: a query for one of them has no answer.
	e = newEnv(t, appMod, mods...)
	if _, err := e.get("example.com/r@<v1.2.0"); err == nil || !strings.Contains(err.Error(), "is retracted") {
		t.Errorf("query of retracted versions: %v", err)
	}
}

func TestGetRetractedLatestIsSkipped(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/r", Version: "v1.0.0"},
		{Path: "example.com/r", Version: "v1.1.0"},
		{Path: "example.com/r", Version: "v1.2.0", Extra: "retract v1.1.0\n"},
		{Path: "example.com/r", Version: "v1.3.0", Extra: "retract [v1.2.0, v1.3.0]\n"},
	}
	e := newEnv(t, appMod, mods...)
	r := e.mustGet("example.com/r")
	// v1.3.0 retracts itself and v1.2.0, and v1.1.0 is retracted by v1.2.0's line only
	// (the lines of the highest version decide): the answer is v1.1.0 or v1.0.0.
	if got := r.Changes[0].New; got != "v1.1.0" {
		t.Errorf("latest = %s", got)
	}
}

func TestGetReplacedModuleNote(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "gocpp.mod"), []byte("module example.com/a\n"), 0o644)
	e := newEnv(t, appMod+"\nreplace example.com/a => "+filepath.ToSlash(dir)+"\n", cat(libA)...)
	r := e.mustGet("example.com/a@v1.0.0")
	if len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "is replaced by") {
		t.Errorf("notes = %q", r.Notes)
	}
	// The module is not downloaded: the replacement is used.
	if _, err := os.Stat(filepath.Join(e.cache, "example.com", "a@v1.0.0")); err == nil {
		t.Error("a replaced module was downloaded")
	}
	// A query that needs versions of a local replacement says so.
	e2 := newEnv(t, appMod+"\nreplace example.com/a => "+filepath.ToSlash(dir)+"\n")
	if _, err := e2.get("example.com/a"); err == nil || !strings.Contains(err.Error(), "has no versions") {
		t.Errorf("latest of a local replacement: %v", err)
	}
}

func TestGetDotlessModule(t *testing.T) {
	e := newEnv(t, appMod, cat(libA)...)
	_, err := e.get("mylib@v1.0.0")
	var ae *ArgError
	if !errors.As(err, &ae) || !strings.Contains(err.Error(), "missing dot in first path element") {
		t.Fatalf("dotless: %v", err)
	}
	// With a local replace it is fine.
	lib := filepath.Join(t.TempDir(), "mylib")
	os.MkdirAll(lib, 0o755)
	os.WriteFile(filepath.Join(lib, "gocpp.mod"), []byte("module mylib\n"), 0o644)
	e = newEnv(t, appMod+"\nreplace mylib => "+filepath.ToSlash(lib)+"\n")
	r := e.mustGet("mylib@v0.0.0")
	if got := changes(r); got != "added mylib v0.0.0" {
		t.Errorf("changes = %q", got)
	}
}

func TestGetArgumentErrors(t *testing.T) {
	e := newEnv(t, appMod, cat(libA)...)
	for _, tc := range []struct{ arg, want string }{
		{"example.com/app@v1.0.0", "can't get the main module"},
		{"example.com/a/...", "patterns are not supported"},
		{"@v1.0.0", "want module[@version]"},
		{"example.com/a@", "missing version query"},
		{"example.com//a", "invalid module path"},
		{"example.com/a@v9.9.9", "404"},
		{"example.com/nothing", "404"},
		{"example.com/a@bogus", "unknown revision"},
	} {
		before, sum := e.mod(), e.sum()
		_, err := e.get(tc.arg)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("get %q: err = %v, want %q", tc.arg, err, tc.want)
		}
		if e.mod() != before || e.sum() != sum {
			t.Errorf("get %q changed the files", tc.arg)
		}
	}
	if _, err := e.get("example.com/a@v1.0.0", "example.com/a@v1.1.0"); err == nil || !strings.Contains(err.Error(), "listed twice") {
		t.Errorf("twice: %v", err)
	}
}

func TestGetOfflineFailsClearly(t *testing.T) {
	e := newEnv(t, appMod, cat(libA)...)
	e.cfg.Offline = true
	_, err := e.get("example.com/a")
	if err == nil || !strings.Contains(err.Error(), "downloads are off") {
		t.Errorf("latest offline: %v", err)
	}
	_, err = e.get("example.com/a@v1.0.0")
	var oe *modfetch.OfflineError
	if !errors.As(err, &oe) {
		t.Errorf("version offline: %v", err)
	}
	if e.mod() != appMod {
		t.Error("gocpp.mod changed")
	}
	// A module that is in the cache works offline.
	e.cfg.Offline = false
	e.mustGet("example.com/a@v1.0.0")
	e.writeMod(appMod)
	e.cfg.Offline = true
	r := e.mustGet("example.com/a@v1.0.0")
	if len(r.Changes) != 1 {
		t.Errorf("offline from the cache: %+v", r)
	}
}

func TestGetBrokenModFile(t *testing.T) {
	e := newEnv(t, "module\n")
	if _, err := e.get("example.com/a@v1.0.0"); err == nil || !strings.Contains(err.Error(), "usage: module") {
		t.Errorf("err = %v", err)
	}
}

func TestGetSymlinkedModFileIsRefused(t *testing.T) {
	e := newEnv(t, appMod, cat(libA)...)
	real := filepath.Join(e.dir, "real.mod")
	os.Rename(filepath.Join(e.dir, "gocpp.mod"), real)
	if err := os.Symlink(real, filepath.Join(e.dir, "gocpp.mod")); err != nil {
		t.Skip(err)
	}
	if _, err := e.get("example.com/a@v1.0.0"); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Errorf("err = %v", err)
	}
	if e.sum() != "" {
		t.Error("gocpp.sum was written")
	}
}

// gocpp.sum and gocpp.mod are written together: when gocpp.sum can't be written, gocpp.mod is
// not changed either (and the other way round: gocpp.sum is written first).
func TestGetWritesNothingOnFailure(t *testing.T) {
	e := newEnv(t, appMod, cat(libA)...)
	// Everything is computed and downloaded; then the lock folder of the module cache
	// stops being writable, so that the lock for gocpp.sum can't be taken.
	lock := filepath.Join(e.cache, "lock")
	beforeWrite = func() { os.Chmod(lock, 0o555) }
	t.Cleanup(func() { beforeWrite = func() {}; os.Chmod(lock, 0o755) })
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	if runtime.GOOS == "windows" {
		t.Skip("a Windows folder has no permission bits: Chmod cannot make it read-only")
	}
	_, err := e.get("example.com/a@v1.0.0")
	if err == nil {
		t.Fatal("get worked although gocpp.sum could not be written")
	}
	if e.mod() != appMod {
		t.Errorf("gocpp.mod = %q after a failure", e.mod())
	}
	if e.sum() != "" {
		t.Errorf("gocpp.sum = %q after a failure", e.sum())
	}
}

// gocpp.sum that is a folder: nothing changes.
func TestGetSumFileIsAFolder(t *testing.T) {
	e := newEnv(t, appMod, cat(libA)...)
	os.Mkdir(filepath.Join(e.dir, "gocpp.sum"), 0o755)
	if _, err := e.get("example.com/a@v1.0.0"); err == nil {
		t.Fatal("get worked with a folder as gocpp.sum")
	}
	if e.mod() != appMod {
		t.Errorf("gocpp.mod = %q after a failure", e.mod())
	}
}

func TestGetSecurityErrorOnChangedModule(t *testing.T) {
	e := newEnv(t, appMod, cat(libB)...)
	e.mustGet("example.com/b@v1.0.0")
	good := e.sum()
	// The proxy serves another zip for the same version, and the cache is gone.
	removeReadOnly(e.cache)
	writeProxy(t, e.proxy, []tmod{{Path: "example.com/b", Version: "v1.0.0", Files: map[string]string{"evil.go": "package evil\n"}}})
	e.writeMod(appMod)
	_, err := e.get("example.com/b@v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "SECURITY ERROR") {
		t.Fatalf("err = %v", err)
	}
	if e.sum() != good {
		t.Error("gocpp.sum changed after a mismatch")
	}
	if e.mod() != appMod {
		t.Error("gocpp.mod changed after a mismatch")
	}
}

func TestGetSecurityErrorOnChangedModFile(t *testing.T) {
	// A .mod file that differs from the recorded hash is refused before anything is read from it.
	mods := []tmod{{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"}}, {Path: "example.com/b", Version: "v1.0.0"}}
	e := newEnv(t, appMod, mods...)
	e.mustGet("example.com/a@v1.0.0")
	good := e.sum()
	removeReadOnly(e.cache)
	mods[0].Reqs = []string{"example.com/b@v1.0.0", "example.com/evil@v1.0.0"}
	writeProxy(t, e.proxy, mods[:1])
	e.writeMod(appMod)
	if _, err := e.get("example.com/a@v1.0.0"); err == nil || !strings.Contains(err.Error(), "SECURITY ERROR") {
		t.Fatalf("err = %v", err)
	}
	if e.sum() != good || e.mod() != appMod {
		t.Error("files changed")
	}
}

func TestGetConcurrentGets(t *testing.T) {
	// Eight gets at once in one project, each its own "process" (its own Fetcher): they
	// run one after the other, so no change is lost.
	var mods []tmod
	var args []string
	for i := 0; i < 8; i++ {
		p := fmt.Sprintf("example.com/c%d", i)
		mods = append(mods, tmod{Path: p, Version: "v1.0.0"}, tmod{Path: p, Version: "v1.1.0"})
		args = append(args, p+"@v1.1.0")
	}
	e := newEnv(t, appMod, mods...)
	var wg sync.WaitGroup
	errs := make([]error, len(args))
	for i, a := range args {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, _ := modfetch.New(&modfetch.Config{CacheDir: e.cache, Proxies: []modfetch.Proxy{{URL: fileURL(e.proxy)}}})
			_, errs[i] = Run(t.Context(), Options{Dir: e.dir, Fetcher: f, Args: []string{a}})
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("get %s: %v", args[i], err)
		}
	}
	got := e.requires()
	if len(got) != 8 {
		t.Fatalf("requires = %v\n%s", got, e.mod())
	}
	for i, r := range got {
		if r != fmt.Sprintf("example.com/c%d@v1.1.0", i) {
			t.Errorf("requires[%d] = %s", i, r)
		}
		p, ver, _ := strings.Cut(r, "@")
		if !strings.Contains(e.sum(), p+" "+ver+" h1:") || !strings.Contains(e.sum(), p+" "+ver+"/go.mod h1:") {
			t.Errorf("gocpp.sum lacks %s:\n%s", r, e.sum())
		}
	}
}

// A module with no go.mod in its archive (a library): the go.mod line of its hash is for the
// file that the proxy made up, and a second download from an empty cache is checked against it.
func TestGetModuleWithoutModFile(t *testing.T) {
	e := newEnv(t, appMod, tmod{Path: "example.com/lib", Version: "v1.0.0", NoModFile: true, Files: map[string]string{"lib.h": "int lib;\n"}})
	e.mustGet("example.com/lib@v1.0.0")
	if !strings.Contains(e.sum(), "example.com/lib v1.0.0/go.mod h1:") {
		t.Errorf("gocpp.sum = %q", e.sum())
	}
	removeReadOnly(e.cache)
	e.reopen()
	if _, err := e.f.Fetch(t.Context(), e.dir, "example.com/lib", "v1.0.0", modfetch.FetchOptions{}); err != nil {
		t.Errorf("second download: %v", err)
	}
}

func TestGetKeepsComments(t *testing.T) {
	mod := "// my app\nmodule example.com/app\n\ngocpp 0.1\n\n// the lib\nrequire example.com/b v1.0.0 // pinned\n\nreplace example.com/zzz => ./zzz\n\ncflags -O2\n"
	e := newEnv(t, mod, cat(libB)...)
	e.mustGet("example.com/b@v1.1.0")
	want := strings.Replace(mod, "v1.0.0 // pinned", "v1.1.0 // pinned", 1)
	if got := e.mod(); got != want {
		t.Errorf("gocpp.mod:\n%s\nwant:\n%s", got, want)
	}
}

// The module files of dependencies are the ones that Go modules have: go and toolchain
// lines, require blocks, retract blocks, replace lines that only the main module's file
// may use, and statements of a later Go.
func TestGetReadsRealisticGoModFiles(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/a", Version: "v1.0.0", Reqs: []string{"example.com/b@v1.0.0"},
			Extra: "toolchain go1.22.1\ngodebug default=go1.21\nretract (\n\tv0.9.0 // oops\n\t[v0.1.0, v0.2.0]\n)\nreplace example.com/b => ../b\nexclude example.com/b v1.1.0\nrequire (\n\texample.com/c v1.0.0 // indirect\n)\n"},
		{Path: "example.com/b", Version: "v1.0.0"},
		{Path: "example.com/b", Version: "v1.1.0"},
		{Path: "example.com/c", Version: "v1.0.0"},
	}
	e := newEnv(t, appMod, mods...)
	r := e.mustGet("example.com/a@v1.0.0")
	// The replace and exclude lines of a dependency are not used: b is at v1.0.0 (not replaced), c comes in.
	if got := changes(r); got != "added example.com/a v1.0.0; added example.com/b v1.0.0; added example.com/c v1.0.0" {
		t.Errorf("changes = %q", got)
	}
	if got := changes(e.mustGet("example.com/b@v1.1.0")); got != "upgraded example.com/b v1.0.0 => v1.1.0" {
		t.Errorf("an exclude line of a dependency counted: %q", got)
	}
}

func TestGetBadDependencyModFile(t *testing.T) {
	mods := []tmod{{Path: "example.com/a", Version: "v1.0.0", Extra: "require example.com/b notaversion\n"}}
	e := newEnv(t, appMod, mods...)
	if _, err := e.get("example.com/a@v1.0.0"); err == nil || !strings.Contains(err.Error(), "parsing go.mod") {
		t.Errorf("err = %v", err)
	}
	if e.mod() != appMod || e.sum() != "" {
		t.Error("files changed")
	}
}

// A module whose version in use is +incompatible (every C library on v2 and above):
// "get" moves it to the newest +incompatible version, and "@latest" does not go back
// to the compatible one.
func TestGetFromAnIncompatibleVersion(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/m", Version: "v1.1.0"},
		{Path: "example.com/m", Version: "v2.0.0+incompatible", NoModFile: true},
		{Path: "example.com/m", Version: "v3.0.0+incompatible", NoModFile: true},
	}
	for _, q := range []string{"example.com/m", "example.com/m@latest", "example.com/m@upgrade"} {
		e := newEnv(t, appMod+"\nrequire example.com/m v2.0.0+incompatible\n", mods...)
		r := e.mustGet(q)
		if got := changes(r); got != "upgraded example.com/m v2.0.0+incompatible => v3.0.0+incompatible" {
			t.Errorf("get %s: %q", q, got)
		}
	}
	// From the compatible version, the incompatible ones stay out: v1.1.0 has a go.mod.
	e := newEnv(t, appMod+"\nrequire example.com/m v1.1.0\n", mods...)
	if r := e.mustGet("example.com/m@latest"); len(r.Changes) != 0 {
		t.Errorf("latest from v1.1.0: %q", changes(r))
	}
}

// A repository that was never a Go module (no go.mod at its highest compatible
// version): its v2 tag is its newest release.
func TestGetLatestOfALegacyLibrary(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/old", Version: "v1.9.0", NoModFile: true},
		{Path: "example.com/old", Version: "v2.0.0+incompatible", NoModFile: true},
	}
	e := newEnv(t, appMod, mods...)
	if got := changes(e.mustGet("example.com/old")); got != "added example.com/old v2.0.0+incompatible" {
		t.Errorf("latest = %q", got)
	}
	// A bound with a major version that the path can't have asks for +incompatible versions.
	e = newEnv(t, appMod, cat(mods, []tmod{{Path: "example.com/new", Version: "v1.1.0"}, {Path: "example.com/new", Version: "v2.0.0+incompatible", NoModFile: true}})...)
	if got := changes(e.mustGet("example.com/new@>=v2.0.0")); got != "added example.com/new v2.0.0+incompatible" {
		t.Errorf(">=v2.0.0 = %q", got)
	}
	if got := changes(e.mustGet("example.com/old@<v2")); got != "added example.com/old v1.9.0" {
		t.Errorf("<v2 = %q", got)
	}
}

// "get lib@v2.0.0" of a path with no /vN names v2.0.0+incompatible (as Go does), so
// a later plain "get lib" does not move to another name of the same tree and
// download it twice.
func TestGetExplicitMajorIsIncompatible(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/lib", Version: "v1.0.0"},
		{Path: "example.com/lib", Version: "v2.0.0+incompatible", NoModFile: true},
	}
	e := newEnv(t, appMod, mods...)
	if got := changes(e.mustGet("example.com/lib@v2.0.0")); got != "added example.com/lib v2.0.0+incompatible" {
		t.Fatalf("get lib@v2.0.0 = %q", got)
	}
	before := e.sum()
	if r := e.mustGet("example.com/lib"); len(r.Changes) != 0 || len(r.Downloaded) != 0 {
		t.Errorf("get lib after lib@v2.0.0: %q, downloaded %v", changes(r), r.Downloaded)
	}
	if e.sum() != before {
		t.Errorf("gocpp.sum changed:\n%s\nto\n%s", before, e.sum())
	}
	if got := e.requires(); len(got) != 1 || got[0] != "example.com/lib@v2.0.0+incompatible" {
		t.Errorf("requires = %v", got)
	}
	// An exclude line names the version the same way.
	e = newEnv(t, appMod+"\nexclude example.com/lib v2.0.0+incompatible\n", mods...)
	if _, err := e.get("example.com/lib@v2.0.0"); err == nil || !strings.Contains(err.Error(), "excluded") {
		t.Errorf("excluded: %v", err)
	}
}

// "upgrade" (and a plain "get") never moves from a pseudo-version to a release that
// is chronologically older (Go's rule): a user of a branch tip does not want the
// tag that was made before it. A newer release is an upgrade.
func TestGetFromAPseudoVersionNeverGoesBack(t *testing.T) {
	const pseudo = "v0.0.0-20210101000000-aaaaaaaaaaaa"
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	mods := func(release time.Time) []tmod {
		return []tmod{
			{Path: "example.com/m", Version: "v1.0.0", Time: release},
			{Path: "example.com/m", Version: pseudo, Time: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), Unlisted: true},
		}
	}
	for _, q := range []string{"example.com/m", "example.com/m@upgrade"} {
		e := newEnv(t, appMod+"\nrequire example.com/m "+pseudo+"\n", mods(old)...)
		if r := e.mustGet(q); len(r.Changes) != 0 {
			t.Errorf("get %s: %q (an older release)", q, changes(r))
		}
		e = newEnv(t, appMod+"\nrequire example.com/m "+pseudo+"\n", mods(newer)...)
		if got := changes(e.mustGet(q)); got != "upgraded example.com/m "+pseudo+" => v1.0.0" {
			t.Errorf("get %s: %q (a newer release)", q, got)
		}
	}
	// "latest" and an explicit version are what was asked for.
	e := newEnv(t, appMod+"\nrequire example.com/m "+pseudo+"\n", mods(old)...)
	if got := changes(e.mustGet("example.com/m@latest")); got != "upgraded example.com/m "+pseudo+" => v1.0.0" {
		t.Errorf("get m@latest: %q", got)
	}
}

// A bound that carries +incompatible names a version that a path with no /vN can
// have, so it does not ask for the incompatible versions: v1.1.0 has a go.mod, so
// the ones above it stay out of "<" and "<=" (go's CheckPathMajor accepts them).
func TestGetBoundWithIncompatibleSuffix(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/m", Version: "v1.1.0"},
		{Path: "example.com/m", Version: "v2.0.0+incompatible", NoModFile: true},
		{Path: "example.com/m", Version: "v3.0.0+incompatible", NoModFile: true},
	}
	for _, c := range []struct{ query, want string }{
		{"<v3.0.0+incompatible", "v1.1.0"},
		{"<=v3.0.0+incompatible", "v1.1.0"},
		{"<v3.0.0", "v2.0.0+incompatible"},              // a major the path can't have: asks for them
		{">v1.1.0+incompatible", "v2.0.0+incompatible"}, // the lowest above the bound
		{">=v2.0.0+incompatible", "v2.0.0+incompatible"},
	} {
		e := newEnv(t, appMod, mods...)
		if got := changes(e.mustGet("example.com/m@" + c.query)); got != "added example.com/m "+c.want {
			t.Errorf("get m@%s = %q, want %s", c.query, got, c.want)
		}
	}
}

// A /vN path on a host that is not a git host is a module with that major version.
func TestGetMajorSuffixOnAnyHost(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/lib", Version: "v1.0.0"},
		{Path: "example.com/lib/v2", Version: "v2.0.0"},
		{Path: "example.com/lib/v2", Version: "v2.1.0"},
	}
	e := newEnv(t, appMod, mods...)
	if got := changes(e.mustGet("example.com/lib/v2")); got != "added example.com/lib/v2 v2.1.0" {
		t.Errorf("get lib/v2 = %q", got)
	}
	if got := changes(e.mustGet("example.com/lib")); got != "added example.com/lib v1.0.0" {
		t.Errorf("get lib = %q", got)
	}
	if _, err := e.get("example.com/lib/v2@v1.0.0"); err == nil || !strings.Contains(err.Error(), "does not match the /v2") {
		t.Errorf("v1 of /v2: %v", err)
	}
}

// A downgrade may need a higher version of another module: "get m2@v0.1.0" lowers m2
// and raises m1, as the go command does.
func TestGetDowngradeRaisesOthers(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/m1", Version: "v1.0.0-rc.10"},
		{Path: "example.com/m1", Version: "v1.0.1"},
		{Path: "example.com/m2", Version: "v0.1.0", Reqs: []string{"example.com/m1@v1.0.1"}},
		{Path: "example.com/m2", Version: "v1.2.0"},
	}
	e := newEnv(t, appMod+"\nrequire example.com/m1 v1.0.0-rc.10\nrequire example.com/m2 v1.2.0\n", mods...)
	r := e.mustGet("example.com/m2@v0.1.0")
	if got := changes(r); got != "upgraded example.com/m1 v1.0.0-rc.10 => v1.0.1; downgraded example.com/m2 v1.2.0 => v0.1.0" {
		t.Errorf("changes = %q", got)
	}
}

// A downgrade does not fall back to a retracted version (Go's CheckAllowed): m
// v1.1.0 needs q and retracts v1.0.0, so "get q@none" lands on m v0.9.0.
func TestGetDowngradeSkipsRetracted(t *testing.T) {
	mods := []tmod{
		{Path: "example.com/m", Version: "v0.9.0"},
		{Path: "example.com/m", Version: "v1.0.0"},
		{Path: "example.com/m", Version: "v1.1.0", Reqs: []string{"example.com/q@v1.0.0"}, Extra: "retract v1.0.0\n"},
		{Path: "example.com/q", Version: "v1.0.0"},
	}
	e := newEnv(t, appMod+"\nrequire example.com/m v1.1.0\n", mods...)
	r := e.mustGet("example.com/q@none")
	if got := changes(r); !strings.Contains(got, "downgraded example.com/m v1.1.0 => v0.9.0") {
		t.Errorf("changes = %q", got)
	}
	if got := e.requires(); len(got) != 1 || got[0] != "example.com/m@v0.9.0" {
		t.Errorf("requires = %v", got)
	}
}

func TestGetQueriesLikeGo(t *testing.T) {
	mods := cat(
		v("example.com/p", "v1.0.0-rc.1", "v1.0.0", "v1.0.1"),
		v("example.com/q", "v1.0.0-rc.1"),
		v("example.com/r", "v1.0.0-rc.1", "v1.0.0-rc.2"),
	)
	for _, tc := range []struct{ arg, want string }{
		{"example.com/p@>=v1.0.0-rc.1", "added example.com/p v1.0.0"}, // releases before prereleases
		{"example.com/p@>v1.0.0-rc.1", "added example.com/p v1.0.0"},
		{"example.com/p@<v1.0.1", "added example.com/p v1.0.0"},
		{"example.com/p@v1.0.0+meta", "added example.com/p v1.0.0"}, // build metadata is dropped
		{"example.com/p@v1.0", "added example.com/p v1.0.1"},
		{"example.com/q@latest", "added example.com/q v1.0.0-rc.1"}, // only a prerelease: it is the latest
		{"example.com/r@<v1.0.0", "added example.com/r v1.0.0-rc.2"},
	} {
		e := newEnv(t, appMod, mods...)
		if got := changes(e.mustGet(tc.arg)); got != tc.want {
			t.Errorf("get %s = %q, want %q", tc.arg, got, tc.want)
		}
	}
	e := newEnv(t, appMod, mods...)
	for _, arg := range []string{"example.com/q@v1.0", "example.com/q@v1", "example.com/p@<=v1.0", "example.com/p@>v1"} {
		if _, err := e.get(arg); err == nil {
			t.Errorf("get %s worked", arg)
		}
	}
	if e.mod() != appMod {
		t.Error("gocpp.mod changed")
	}
}

func TestGetKeepsWindowsLineEnds(t *testing.T) {
	crlf := strings.ReplaceAll(appMod+"\nrequire example.com/b v1.0.0 // pinned\n", "\n", "\r\n")
	e := newEnv(t, crlf, cat(libB)...)
	// Nothing to change: the file is not touched.
	r := e.mustGet("example.com/b@v1.0.0")
	if r.ModChanged || e.mod() != crlf {
		t.Errorf("an unchanged file was rewritten: %q", e.mod())
	}
	e.mustGet("example.com/b@v1.1.0")
	want := strings.Replace(crlf, "v1.0.0", "v1.1.0", 1)
	if e.mod() != want {
		t.Errorf("gocpp.mod = %q, want %q", e.mod(), want)
	}
}

// A gocpp.sum that has only the go.mod line of a module still checks it.
func TestGetChecksAGoModLineWithoutAZipLine(t *testing.T) {
	e := newEnv(t, appMod, cat(libB)...)
	os.WriteFile(filepath.Join(e.dir, "gocpp.sum"), []byte("example.com/b v1.0.0/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n"), 0o644)
	_, err := e.get("example.com/b@v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "SECURITY ERROR") {
		t.Fatalf("err = %v", err)
	}
	if e.mod() != appMod {
		t.Error("gocpp.mod changed")
	}
	// Fetching the module itself (the module file is in the cache now): the
	// archive's go.mod is checked against the line although the zip has none.
	_, err = e.f.Fetch(t.Context(), e.dir, "example.com/b", "v1.0.0", modfetch.FetchOptions{Record: true})
	if err == nil || !strings.Contains(err.Error(), "SECURITY ERROR") || !strings.Contains(err.Error(), "go.mod") {
		t.Errorf("Fetch: %v", err)
	}
}

// A proxy zip must have all its files in <module>@<version>/.
func TestGetRejectsAZipWithAnotherPrefix(t *testing.T) {
	e := newEnv(t, appMod, cat(libB)...)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("other@v1.0.0/x.go")
	w.Write([]byte("package x\n"))
	zw.Close()
	if err := os.WriteFile(filepath.Join(e.proxy, "example.com", "b", "@v", "v1.0.0.zip"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := e.get("example.com/b@v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "does not have its files in example.com/b@v1.0.0/") {
		t.Fatalf("err = %v", err)
	}
	if e.mod() != appMod || e.sum() != "" {
		t.Error("files changed")
	}
}

// Without a module cache two gets in one project still run one after the other.
func TestGetWithoutAModuleCacheLocksTheProject(t *testing.T) {
	dir := t.TempDir()
	ctx := t.Context()
	unlock, err := modfetch.LockProject(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if u2, err := modfetch.LockProject(short, dir); err == nil {
		u2()
		t.Fatal("a second lock of the same project was granted")
	}
	unlock()
	u3, err := modfetch.LockProject(ctx, dir)
	if err != nil {
		t.Fatalf("lock after unlock: %v", err)
	}
	u3()
}

// The project lock works between processes: a child process holds it.
func TestProjectLockBetweenProcesses(t *testing.T) {
	if dir := os.Getenv("GOCPP_LOCK_HELPER"); dir != "" {
		unlock, err := modfetch.LockProject(context.Background(), dir)
		if err != nil {
			fmt.Println("error", err)
			os.Exit(3)
		}
		fmt.Println("locked")
		os.Stdin.Read(make([]byte, 1)) // until the parent closes the pipe or dies
		unlock()
		return
	}
	dir := t.TempDir()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestProjectLockBetweenProcesses$")
	cmd.Env = append(os.Environ(), "GOCPP_LOCK_HELPER="+dir)
	dieWithTest(cmd)
	stdin, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stdin.Close(); cmd.Wait() })
	line := make([]byte, 0, 64)
	buf := make([]byte, 1)
	for !strings.Contains(string(line), "locked") {
		if _, err := out.Read(buf); err != nil {
			t.Fatalf("the helper did not lock: %q %v", line, err)
		}
		line = append(line, buf[0])
	}
	short, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	if u, err := modfetch.LockProject(short, dir); err == nil {
		u()
		t.Fatal("the lock was granted while another process held it")
	}
	stdin.Close() // the helper unlocks and exits
	cmd.Wait()
	u, err := modfetch.LockProject(t.Context(), dir)
	if err != nil {
		t.Fatalf("after the helper ended: %v", err)
	}
	u()
}
