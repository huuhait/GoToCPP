//go:build linux

package winssh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An unreachable machine costs one ssh connect attempt (one ConnectTimeout, 10 seconds), not two:
// the run's folder, which is also the reachability check, is made before the old folders are
// cleaned. A fake ssh in PATH counts the calls.
func TestUnreachableMachineIsOneSSHCall(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\necho call >> '" + log + "'\necho 'ssh: connect to host x port 22: Connection timed out' >&2\nexit 255\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if _, err := NewUnlocked("user@192.0.2.1"); err == nil {
		t.Fatal("NewUnlocked succeeded with a fake ssh that always fails")
	}
	data, _ := os.ReadFile(log)
	if calls := strings.Count(string(data), "call"); calls != 1 {
		t.Errorf("ssh was called %d times, want 1", calls)
	}
}
