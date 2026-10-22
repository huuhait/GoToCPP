package probe

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// stampsOf makes the Locations of a call that began now and read the files.
func stampsOf(t *testing.T, started time.Time, dir string, files ...string) *Locations {
	t.Helper()
	return &Locations{deps: files, started: started, dir: dir}
}

func writeAged(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func statOf(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func TestStampsStayUnchangedWhileNoFileMoves(t *testing.T) {
	dir := t.TempDir()
	header := filepath.Join(dir, "a.h")
	writeAged(t, header, "int a;\n")
	stamps := stampsOf(t, time.Now(), dir, header).CaptureStamps()
	if stamps == nil || !stamps.Unchanged() {
		t.Fatalf("stamps = %v, want unchanged files", stamps)
	}
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !stamps.Unchanged() {
		t.Error("a new file in the directory is a change: clang's temporary files are not")
	}
	if stamps.Bytes() <= 0 {
		t.Errorf("Bytes = %d, want an estimate", stamps.Bytes())
	}
}

func TestStampsSeeEachKindOfChange(t *testing.T) {
	changes := map[string]func(t *testing.T, path string){
		"the size alone": func(t *testing.T, path string) {
			before := statOf(t, path)
			if err := os.WriteFile(path, []byte("int a; int b;\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
				t.Fatal(err)
			}
		},
		"the modification time": func(t *testing.T, path string) {
			when := time.Now().Add(-2 * time.Hour)
			if err := os.Chtimes(path, when, when); err != nil {
				t.Fatal(err)
			}
		},
		"the file itself": func(t *testing.T, path string) {
			before := statOf(t, path)
			other := path + ".new"
			if err := os.WriteFile(other, []byte("int a;\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(other, before.ModTime(), before.ModTime()); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(other, path); err != nil {
				t.Fatal(err)
			}
		},
		"a removal": func(t *testing.T, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			header := filepath.Join(dir, "a.h")
			writeAged(t, header, "int a;\n")
			stamps := stampsOf(t, time.Now(), dir, header).CaptureStamps()
			if stamps == nil {
				t.Fatal("no stamps for an aged file")
			}
			change(t, header)
			if stamps.Unchanged() {
				t.Errorf("stamps are unchanged after a change of %s", name)
			}
		})
	}
}

func TestStampsSeeAHeaderThatShadowsAnother(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first", "a.h")
	second := filepath.Join(dir, "second", "a.h")
	for _, path := range []string{first, second} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeAged(t, second, "int a;\n")
	// the compiler looked for a.h in first/, did not find it, and read second/a.h
	stamps := stampsOf(t, time.Now(), dir, first, second).CaptureStamps()
	if stamps == nil || !stamps.Unchanged() {
		t.Fatalf("stamps = %v, want unchanged", stamps)
	}
	writeAged(t, first, "long a;\n")
	if stamps.Unchanged() {
		t.Error("a header that now shadows the one that was read is not a change")
	}
}

func TestStampsAreNotTakenOfFilesThatAreTooNew(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.h")
	fresh := filepath.Join(dir, "fresh.h")
	writeAged(t, old, "int a;\n")
	if err := os.WriteFile(fresh, []byte("int b;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute)
	if stamps := stampsOf(t, started, dir, old, fresh).CaptureStamps(); stamps != nil {
		t.Error("stamps of a file modified after the call began")
	}
	previous := racyWindow
	racyWindow = 2 * time.Second
	defer func() { racyWindow = previous }()
	// (the change time of every file here is now, so the window alone makes both too new)
	if stamps := stampsOf(t, time.Now(), dir, old, fresh).CaptureStamps(); stamps != nil {
		t.Error("stamps of a file modified inside the racy window")
	}
}

func TestStampsNeedTheFilesOfACall(t *testing.T) {
	var none *Locations
	if none.CaptureStamps() != nil || none.Deps() != nil {
		t.Error("a missing call has stamps")
	}
	if stampsOf(t, time.Now(), t.TempDir()).CaptureStamps() != nil {
		t.Error("a call that read no file has stamps")
	}
	var stamps *Stamps
	if stamps.Unchanged() || stamps.Bytes() != 0 {
		t.Error("missing stamps count as unchanged")
	}
}

// A write of the same size that sets the modification time back leaves only the change time.
func TestStampsSeeAWriteThatRestoresTheTimes(t *testing.T) {
	dir := t.TempDir()
	header := filepath.Join(dir, "a.h")
	writeAged(t, header, "int a;\n")
	before := statOf(t, header)
	if _, ok := changeTime(before); !ok {
		t.Skip("this system has no change time")
	}
	stamps := stampsOf(t, time.Now(), dir, header).CaptureStamps()
	if stamps == nil {
		t.Fatal("no stamps for an aged file")
	}
	if err := os.WriteFile(header, []byte("int b;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(header, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	if stamps.Unchanged() {
		t.Error("a write that restored the size and the modification time was not seen")
	}
}
