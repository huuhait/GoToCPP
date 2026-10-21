package modfetch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// isolatedTempDir points the temporary folder of this process (TMPDIR, and TEMP and TMP on
// Windows) at a new empty folder and returns it.
func isolatedTempDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMP", tmp)
	return tmp
}

// listNames returns the names in the folder dir.
func listNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// The lock of a project works on every system (on Windows, LockFileEx fails on a folder with "The
// parameter is incorrect", so every gocpp get failed there) and leaves nothing in the project.
func TestLockProjectLeavesNoFileInTheProject(t *testing.T) {
	isolatedTempDir(t)
	project := t.TempDir()
	unlock, err := LockProject(context.Background(), project)
	if err != nil {
		t.Fatalf("LockProject: %v", err)
	}
	if names := listNames(t, project); len(names) != 0 {
		t.Errorf("the project has %v while it is locked, want nothing", names)
	}
	unlock()
	if names := listNames(t, project); len(names) != 0 {
		t.Errorf("the project has %v after the lock, want nothing", names)
	}
}

// The file that stands for the lock on Windows is deleted with the lock, so lock after lock leaves
// nothing in the temporary folder.
func TestLockProjectLeavesNoFileInTheTempFolder(t *testing.T) {
	tmp := isolatedTempDir(t)
	project := t.TempDir()
	for range 3 {
		unlock, err := LockProject(context.Background(), project)
		if err != nil {
			t.Fatalf("LockProject: %v", err)
		}
		unlock()
	}
	if names := listNames(t, tmp); len(names) != 0 {
		t.Errorf("the temporary folder has %v after the locks, want nothing", names)
	}
}

// A second lock of the same project waits for the first and is granted after its release.
func TestLockProjectExcludesTheSameProject(t *testing.T) {
	isolatedTempDir(t)
	project := t.TempDir()
	first, err := LockProject(context.Background(), project)
	if err != nil {
		t.Fatalf("first LockProject: %v", err)
	}
	short, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if second, err := LockProject(short, project); err == nil {
		second()
		t.Fatal("a second lock of the same project was granted while the first was held")
	}
	first()
	again, err := LockProject(context.Background(), project)
	if err != nil {
		t.Fatalf("LockProject after the release: %v", err)
	}
	again()
}

// Two different projects do not exclude each other.
func TestLockProjectDifferentProjectsDoNotExclude(t *testing.T) {
	isolatedTempDir(t)
	first, err := LockProject(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("first LockProject: %v", err)
	}
	defer first()
	short, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	second, err := LockProject(short, t.TempDir())
	if err != nil {
		t.Fatalf("the lock of another project: %v", err)
	}
	second()
}

// The name of a project for its lock is the clean absolute path, in lower case on Windows only.
func TestProjectLockKey(t *testing.T) {
	root := t.TempDir()
	upper := filepath.Join(root, "Proj", "A")
	lower := filepath.Join(root, "proj", "a")
	if projectLockKey(upper, true) != projectLockKey(lower, true) {
		t.Errorf("on Windows %q and %q are two projects", upper, lower)
	}
	if projectLockKey(upper, false) == projectLockKey(lower, false) {
		t.Errorf("elsewhere %q and %q are one project", upper, lower)
	}
	if got, want := projectLockKey(upper+string(filepath.Separator)+".", false), upper; got != want {
		t.Errorf("an unclean path gives %q, want %q", got, want)
	}
}
