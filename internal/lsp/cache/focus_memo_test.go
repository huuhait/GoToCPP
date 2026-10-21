package cache

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/memoize"
)

// The tests of the focused checks that a base keeps by the content of the edited file
// (focusBase.memo): typing a character and deleting it, or an undo and a redo, ask for the same
// check again.

// focusedAfter edits lib/a.gp to the text with line at the marker // EDIT, and returns the focused
// package of lib against base.
func focusedAfter(t *testing.T, w *world, base *Package, files map[string]string, line string) *Package {
	t.Helper()
	w.edit("lib/a.gp", editA(files, line)["lib/a.gp"])
	snap, release := w.s.Snapshot()
	defer release()
	dir := filepath.Join(w.root, "lib")
	pkg, err := snap.Focused(context.Background(), dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if pkg == nil {
		t.Fatalf("the focused check declined %q", line)
	}
	return pkg
}

func TestFocusedSameTextIsOneCheck(t *testing.T) {
	files := focusFiles()
	w, base := focusRun(t, files)
	first := focusedAfter(t, w, base, files, "c.n += undefinedName")
	checks := w.p.FocusedChecks()

	focusedAfter(t, w, base, files, "c.n += k + 1")
	again := focusedAfter(t, w, base, files, "c.n += undefinedName")

	if again != first {
		t.Error("the same text of the file made another focused package")
	}
	if got := w.p.FocusedChecks() - checks; got != 1 {
		t.Errorf("%d focused checks for one new text and one repeated text, want 1", got)
	}
}

func TestFocusedMemoKeepsTheLastFew(t *testing.T) {
	files := focusFiles()
	w, base := focusRun(t, files)
	lines := []string{"c.n += 1", "c.n += 2", "c.n += 3", "c.n += 4"}
	first := focusedAfter(t, w, base, files, lines[0])
	for _, line := range lines[1:] {
		focusedAfter(t, w, base, files, line)
	}
	checks := w.p.FocusedChecks()

	focusedAfter(t, w, base, files, lines[len(lines)-1])
	oldest := focusedAfter(t, w, base, files, lines[0])

	if w.p.FocusedChecks()-checks != 1 {
		t.Errorf("%d focused checks for the newest text and the oldest, want 1 (the oldest)",
			w.p.FocusedChecks()-checks)
	}
	if oldest == first {
		t.Error("a text beyond the kept ones came from the memo")
	}
}

func TestFocusedMemoIsPerBase(t *testing.T) {
	files := focusFiles()
	w, base := focusRun(t, files)
	first := focusedAfter(t, w, base, files, "c.n += 1")
	// A full check of the edited text is the base of the next keystrokes.
	w.pkg("lib")
	snap, release := w.s.Snapshot()
	newBase, err := snap.Package(context.Background(), filepath.Join(w.root, "lib"))
	release()
	if err != nil {
		t.Fatal(err)
	}

	next := focusedAfter(t, w, newBase, files, "c.n += 2")

	if next == first {
		t.Error("a focused package of another base was reused")
	}
	if newBase.focus != nil && newBase.focus == base.focus {
		t.Error("two bases share a focus memo")
	}
}

func TestFocusedMemoKeepsADecline(t *testing.T) {
	files := focusFiles()
	w, base := focusRun(t, files)
	text := strings.Replace(
		files["lib/a.gp"], "c.n += k // EDIT", "c.n += k\n\tx := \"unterminated // EDIT", 1)
	w.edit("lib/a.gp", text)
	snap, release := w.s.Snapshot()
	defer release()

	pkg, err := snap.Focused(context.Background(), filepath.Join(w.root, "lib"), base)

	if pkg != nil || err != nil {
		t.Errorf("a body with a syntax error: %v, %v, want a decline", pkg, err)
	}
	if n := len(base.focus.memo); n != 0 {
		t.Errorf("a decline before the check left %d memo entries", n)
	}
}

func TestFocusMemoKeysByPathAndContent(t *testing.T) {
	var base focusBase
	made := 0
	newProm := func() *memoize.Promise[*Package] {
		made++
		return memoize.NewPromise(func(context.Context) (*Package, error) { return nil, nil })
	}
	one := base.promise("a.gp", Hash{1}, newProm)
	if base.promise("a.gp", Hash{1}, newProm) != one || made != 1 {
		t.Error("the same file with the same content made another check")
	}
	if base.promise("b.gp", Hash{1}, newProm) == one {
		t.Error("another file with the same content shares the check")
	}
	if base.promise("a.gp", Hash{2}, newProm) == one {
		t.Error("the same file with other content shares the check")
	}
}
