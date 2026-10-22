package packaging

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// cleanTmpFolder makes a TMPDIR with entries that scripts/clean-tmp must remove and entries that
// it must leave: the old ones have every file three days old.
func cleanTmpFolder(t *testing.T) (tmp, outside string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("scripts/clean-tmp is a shell script")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	tmp = t.TempDir()
	outside = t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{
		"gocpp-build123/work/a.o", "gocpp-cmd-test-9/cache/x", "gp-nsprobe42/p", "gp-ssh-1/sock",
		"gocpp-new/f", "gocpp-half/old", "gocpp-half/deep/new", "other-old/f", "Testfoo/001/f",
	} {
		write(filepath.Join(tmp, name), "x")
	}
	write(filepath.Join(outside, "precious"), "keep")
	if err := os.Symlink(outside, filepath.Join(tmp, "gocpp-link")); err != nil {
		t.Skip(err)
	}
	// Everything is old, except gocpp-new and gocpp-half/deep/new.
	old := time.Now().Add(-72 * time.Hour)
	err := filepath.WalkDir(tmp, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.Type()&os.ModeSymlink != 0 {
			return err
		}
		if strings.Contains(path, "gocpp-new") || strings.HasSuffix(path, filepath.Join("gocpp-half", "deep", "new")) {
			return nil
		}
		return os.Chtimes(path, old, old)
	})
	if err != nil {
		t.Fatal(err)
	}
	// The link is old too, or the age would protect it and not the rule about links.
	link := filepath.Join(tmp, "gocpp-link")
	if out, err := exec.Command("touch", "-h", "-d", "@"+strconv.FormatInt(old.Unix(), 10), link).CombinedOutput(); err != nil {
		t.Skipf("touch -h: %v\n%s", err, out)
	}
	return tmp, outside
}

func runCleanTmp(t *testing.T, tmp string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{filepath.Join(repoRoot(t), "scripts", "clean-tmp")}, args...)...)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmp)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// TestCleanTmp: scripts/clean-tmp (make clean-tmp) removes the old entries with gocpp's names directly in
// TMPDIR, and nothing else: not TMPDIR, not a folder with another name, not a link or what is behind
// it, not an entry where something changed lately. -n removes nothing.
func TestCleanTmp(t *testing.T) {
	tmp, outside := cleanTmpFolder(t)

	t.Run("dry run", func(t *testing.T) {
		out, err := runCleanTmp(t, tmp, "-n")
		if err != nil {
			t.Fatalf("clean-tmp -n: %v\n%s", err, out)
		}
		if !strings.Contains(out, "would remove "+filepath.Join(tmp, "gocpp-build123")) {
			t.Errorf("the dry run does not list gocpp-build123:\n%s", out)
		}
		for _, name := range []string{"gocpp-build123", "gocpp-cmd-test-9", "gp-nsprobe42", "gp-ssh-1"} {
			if !exists(filepath.Join(tmp, name)) {
				t.Errorf("the dry run removed %s", name)
			}
		}
	})

	t.Run("removes the old gocpp entries only", func(t *testing.T) {
		out, err := runCleanTmp(t, tmp)
		if err != nil {
			t.Fatalf("clean-tmp: %v\n%s", err, out)
		}
		for _, name := range []string{"gocpp-build123", "gocpp-cmd-test-9", "gp-nsprobe42", "gp-ssh-1"} {
			if exists(filepath.Join(tmp, name)) {
				t.Errorf("%s is still there:\n%s", name, out)
			}
		}
		for name, why := range map[string]string{
			"gocpp-new":                "changed lately",
			"gocpp-half":               "has a file that changed lately",
			"other-old":                "has another name",
			"Testfoo":                  "has another name",
			"gocpp-link":               "is a link",
			"../" + filepath.Base(tmp): "is TMPDIR itself",
			"../" + filepath.Base(outside) + "/precious": "is behind a link",
		} {
			if !exists(filepath.Join(tmp, name)) {
				t.Errorf("%s was removed, but it %s:\n%s", name, why, out)
			}
		}
	})

	t.Run("refuses the root", func(t *testing.T) {
		out, err := runCleanTmp(t, "/", "-n")
		if err == nil || !strings.Contains(out, "not touching anything") {
			t.Errorf("clean-tmp with TMPDIR=/: %v\n%s", err, out)
		}
	})

	t.Run("bad flags", func(t *testing.T) {
		for _, args := range [][]string{{"-x"}, {"-a", "soon"}, {"-a"}} {
			if out, err := runCleanTmp(t, tmp, args...); err == nil {
				t.Errorf("clean-tmp %v succeeded:\n%s", args, out)
			}
		}
	})
}

// oldTree makes the files with the paths in tmp and sets the time of everything under them to three
// days ago.
func oldTree(t *testing.T, tmp string, paths ...string) {
	t.Helper()
	for _, path := range paths {
		full := filepath.Join(tmp, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-72 * time.Hour)
	err := filepath.WalkDir(tmp, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(path, old, old)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCleanTmpReadOnlyTree: a tree without write permission (the module cache and `go test`
// leave such folders) is removed, and the run goes on to the entries after it.
func TestCleanTmpReadOnlyTree(t *testing.T) {
	tmp, _ := cleanTmpFolder(t)
	oldTree(t, tmp, "gocpp-ro/sub/deep/f", "gocpp-ro/sub/g", "gocpp-zz/f")
	for _, dir := range []string{"gocpp-ro/sub/deep", "gocpp-ro/sub", "gocpp-ro"} {
		if err := os.Chmod(filepath.Join(tmp, dir), 0o555); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { exec.Command("chmod", "-R", "u+rwx", tmp).Run() })
	out, err := runCleanTmp(t, tmp)
	if err != nil {
		t.Errorf("clean-tmp = %v\n%s", err, out)
	}
	for _, name := range []string{"gocpp-ro", "gocpp-zz"} {
		if exists(filepath.Join(tmp, name)) {
			t.Errorf("%s is still there:\n%s", name, out)
		}
	}
}

// TestCleanTmpNewlineInName: du prints two lines for a name with a newline; the run does not stop.
func TestCleanTmpNewlineInName(t *testing.T) {
	tmp, _ := cleanTmpFolder(t)
	oldTree(t, tmp, "gocpp-a\nb/f", "gocpp-zz/f")
	out, err := runCleanTmp(t, tmp)
	if err != nil {
		t.Errorf("clean-tmp = %v\n%s", err, out)
	}
	for _, name := range []string{"gocpp-a\nb", "gocpp-zz"} {
		if exists(filepath.Join(tmp, name)) {
			t.Errorf("%q is still there:\n%s", name, out)
		}
	}
}

// TestCleanTmpBusyFolder: an entry that a process has as its working folder, or a folder in it,
// is left, however old it is; when the process is gone, it is removed.
func TestCleanTmpBusyFolder(t *testing.T) {
	tmp, _ := cleanTmpFolder(t)
	if _, err := os.Stat("/proc/self/cwd"); err != nil {
		if _, err := exec.LookPath("fuser"); err != nil {
			t.Skip("no /proc and no fuser")
		}
	}
	oldTree(t, tmp, "gocpp-busy/work/f", "gocpp-idle/f")
	sleeper := exec.Command("sleep", "60")
	sleeper.Dir = filepath.Join(tmp, "gocpp-busy", "work")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := false
	stop := func() {
		if !stopped {
			stopped = true
			sleeper.Process.Kill()
			sleeper.Wait()
		}
	}
	t.Cleanup(stop)
	out, err := runCleanTmp(t, tmp)
	if err != nil {
		t.Errorf("clean-tmp = %v\n%s", err, out)
	}
	if !exists(filepath.Join(tmp, "gocpp-busy")) || !strings.Contains(out, "working folder") {
		t.Errorf("gocpp-busy was removed although a process runs in it:\n%s", out)
	}
	if exists(filepath.Join(tmp, "gocpp-idle")) {
		t.Errorf("gocpp-idle is still there:\n%s", out)
	}
	stop()
	if out, err := runCleanTmp(t, tmp); err != nil || exists(filepath.Join(tmp, "gocpp-busy")) {
		t.Errorf("clean-tmp after the process ended: %v, gocpp-busy there: %v\n%s", err, exists(filepath.Join(tmp, "gocpp-busy")), out)
	}
}

// TestCleanTmpMountPoint: an old entry that is a mount point, or has one inside (a bind mount of a
// folder somewhere else), is left, and what is mounted there is not touched: rm -rf would empty
// it. The names have a space, a newline and a backslash, which the mount table writes as octal
// escapes. The mounts are made in a user namespace of its own (unshare -Urm), so no privilege is
// needed.
func TestCleanTmpMountPoint(t *testing.T) {
	unshare, err := exec.LookPath("unshare")
	if err != nil || exec.Command(unshare, "-Urm", "true").Run() != nil {
		t.Skip("no unshare -Urm (user and mount namespaces)")
	}
	tmp, outside := cleanTmpFolder(t)
	mounts := map[string]string{ // the mount point in TMPDIR: what is mounted there
		"gocpp-m/mnt":           "m",
		"gocpp-self":            "self",
		"gocpp-sp ace/mnt":      "space",
		"gocpp-new\nline/mnt":   "newline",
		"gocpp-back\\slash/mnt": "backslash",
	}
	oldTree(t, tmp, "gocpp-zz/f")
	var args []string
	for point, tag := range mounts {
		oldTree(t, outside, tag+"/p")
		if err := os.MkdirAll(filepath.Join(tmp, point), 0o755); err != nil {
			t.Fatal(err)
		}
		args = append(args, filepath.Join(outside, tag), filepath.Join(tmp, point))
	}
	oldTree(t, tmp, "gocpp-zz/f")
	script := filepath.Join(repoRoot(t), "scripts", "clean-tmp")
	shell := `script=$1; shift; while [ $# -gt 0 ]; do mount --bind "$1" "$2" || exit 3; shift 2; done; sh "$script"`
	cmd := exec.Command(unshare, append([]string{"-Urm", "sh", "-c", shell, "sh", script}, args...)...)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "mount:") {
			t.Skipf("the bind mount is not allowed here: %s", out)
		}
		t.Errorf("clean-tmp = %v\n%s", err, out)
	}
	for point, tag := range mounts {
		if !exists(filepath.Join(outside, tag, "p")) {
			t.Errorf("the folder mounted at %q was emptied:\n%s", point, out)
		}
		entry := strings.SplitN(point, "/", 2)[0]
		if !exists(filepath.Join(tmp, entry)) {
			t.Errorf("the entry %q with a mount point was removed:\n%s", entry, out)
		}
	}
	if !strings.Contains(string(out), "mount point") {
		t.Errorf("no entry was skipped for its mount point:\n%s", out)
	}
	if exists(filepath.Join(tmp, "gocpp-zz")) {
		t.Errorf("gocpp-zz is still there:\n%s", out)
	}
}

// TestCleanTmpFailedRemoval: an entry that rm cannot remove is reported, the run goes on to the
// entries after it, and the exit status is 1. A rm that fails for one name stands in for the
// failures that need privilege to make (an immutable folder).
func TestCleanTmpFailedRemoval(t *testing.T) {
	tmp, _ := cleanTmpFolder(t)
	oldTree(t, tmp, "gocpp-failme/f", "gocpp-zz/f")
	realRm, err := exec.LookPath("rm")
	if err != nil {
		t.Skip(err)
	}
	fake := t.TempDir()
	body := "#!/bin/sh\nfor a in \"$@\"; do\n\tcase \"$a\" in\n\t*gocpp-failme) echo \"rm: cannot remove $a\" >&2; exit 1 ;;\n\tesac\ndone\nexec '" +
		realRm + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(fake, "rm"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", filepath.Join(repoRoot(t), "scripts", "clean-tmp"))
	cmd.Env = append(os.Environ(), "TMPDIR="+tmp, "PATH="+fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("clean-tmp exited 0 although an entry could not be removed:\n%s", out)
	}
	if !strings.Contains(string(out), "could not remove") || !exists(filepath.Join(tmp, "gocpp-failme")) {
		t.Errorf("the failed entry is not reported or was removed:\n%s", out)
	}
	if exists(filepath.Join(tmp, "gocpp-zz")) {
		t.Errorf("the run stopped at the failure; gocpp-zz is still there:\n%s", out)
	}
}
