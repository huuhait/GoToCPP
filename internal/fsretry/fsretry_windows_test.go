//go:build windows

package fsretry

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A file that another handle holds open without FILE_SHARE_DELETE cannot be replaced: MoveFileEx fails
// with a sharing violation until the holder lets go. Rename waits for it.
func TestRenameWaitsForTheHolder(t *testing.T) {
	dir := t.TempDir()
	target, src := filepath.Join(dir, "target"), filepath.Join(dir, "src")
	os.WriteFile(target, []byte("old"), 0o644)
	os.WriteFile(src, []byte("new"), 0o644)
	// Go opens files with all three sharing flags; to hold one without FILE_SHARE_DELETE, open it by hand.
	h, err := openNoDeleteShare(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(src, target); err == nil {
		t.Fatal("os.Rename replaced a file that is held open: the test cannot show anything")
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(150 * time.Millisecond)
		closeHandle(h)
	}()
	if err := Rename(src, target); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	wg.Wait()
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Errorf("target = %q", got)
	}
}
