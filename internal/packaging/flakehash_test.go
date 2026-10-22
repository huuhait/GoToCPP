package packaging

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// flakeHashFolder makes a folder with copies of flake.nix and scripts/flake-hash, a go.mod and a
// go.sum of the test's own, and a fake nix that records its calls and prints what Nix prints for
// a vendorHash that does not match (fakeOutput, "" for a nix that fails without a hash). It
// returns the folder and the environment to run the script with.
func flakeHashFolder(t *testing.T, gosum, fakeOutput string) (dir string, env []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("scripts/flake-hash is a shell script")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir = t.TempDir()
	for name, text := range map[string]string{
		"flake.nix":  readRepoFile(t, "flake.nix"),
		"go.mod":     "module example.com/x\n",
		"go.sum":     gosum,
		"fake-nix":   "#!/bin/sh\necho \"$@\" >> '" + dir + "/nix-calls'\ncat '" + dir + "/nix-output'\nexit 1\n",
		"nix-output": fakeOutput,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "flake-hash"), []byte(readRepoFile(t, "scripts/flake-hash")), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, append(os.Environ(), "NIX="+filepath.Join(dir, "fake-nix"), "TMPDIR="+t.TempDir())
}

func runFlakeHash(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{filepath.Join(dir, "scripts", "flake-hash")}, args...)...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// nixMismatch is what nix build prints when the vendorHash is wrong.
func nixMismatch(got string) string {
	return "error: hash mismatch in fixed-output derivation '/nix/store/x-gocpp-go-modules.drv':\n" +
		"         specified: sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n" +
		"            got:    " + got + "\n"
}

// TestFlakeHashScript: scripts/flake-hash (make flake-hash) writes the hash that nix reports and the
// stamp of go.mod and go.sum, and only those two lines of flake.nix change; when nix gives no hash,
// flake.nix is as it was; a current stamp is left alone unless -f.
func TestFlakeHashScript(t *testing.T) {
	newHash := "sha256-" + strings.Repeat("B", 43) + "="
	original := readRepoFile(t, "flake.nix")
	changed := func(t *testing.T, dir string) (lines []string) {
		t.Helper()
		got, err := os.ReadFile(filepath.Join(dir, "flake.nix"))
		if err != nil {
			t.Fatal(err)
		}
		old := strings.Split(original, "\n")
		now := strings.Split(string(got), "\n")
		if len(old) != len(now) {
			t.Fatalf("flake.nix has %d lines, had %d", len(now), len(old))
		}
		for i := range old {
			if old[i] != now[i] {
				lines = append(lines, now[i])
			}
		}
		return lines
	}

	t.Run("stale stamp", func(t *testing.T) {
		dir, env := flakeHashFolder(t, "new sum line\n", nixMismatch(newHash))
		out, err := runFlakeHash(t, dir, env)
		if err != nil {
			t.Fatalf("flake-hash: %v\n%s", err, out)
		}
		sum := sha256.Sum256([]byte("module example.com/x\n" + "new sum line\n"))
		want := []string{
			"          # go.mod+go.sum sha256: " + hex.EncodeToString(sum[:]),
			`          vendorHash = "` + newHash + `";`,
		}
		if got := changed(t, dir); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("changed lines of flake.nix:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		if calls, _ := os.ReadFile(filepath.Join(dir, "nix-calls")); !strings.Contains(string(calls), "build .#gocpp") {
			t.Errorf("nix was called with %q, want build .#gocpp", calls)
		}
	})

	t.Run("nix gives no hash", func(t *testing.T) {
		dir, env := flakeHashFolder(t, "new sum line\n", "error: some other failure\n")
		out, err := runFlakeHash(t, dir, env)
		if err == nil || !strings.Contains(out, "did not report a hash") {
			t.Errorf("flake-hash = %v, %q; want a failure that says so", err, out)
		}
		if got := changed(t, dir); len(got) != 0 {
			t.Errorf("flake.nix changed although the build failed: %q", got)
		}
	})

	t.Run("current stamp", func(t *testing.T) {
		// go.mod and go.sum of the folder are the repository's: the stamp of flake.nix is current.
		dir, env := flakeHashFolder(t, "", nixMismatch(newHash))
		for _, name := range []string{"go.mod", "go.sum"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(readRepoFile(t, name)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		out, err := runFlakeHash(t, dir, env)
		if err != nil || !strings.Contains(out, "is current") {
			t.Errorf("flake-hash = %v, %q; want 'is current'", err, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "nix-calls")); err == nil {
			t.Errorf("nix was called for a current stamp")
		}
		if got := changed(t, dir); len(got) != 0 {
			t.Errorf("flake.nix changed: %q", got)
		}
		if out, err := runFlakeHash(t, dir, env, "-f"); err != nil {
			t.Fatalf("flake-hash -f: %v\n%s", err, out)
		}
		if got := changed(t, dir); len(got) != 1 || !strings.Contains(got[0], newHash) {
			t.Errorf("flake-hash -f changed %q, want the vendorHash line only", got)
		}
	})
}

// TestFlakeHashScriptSignal: a signal during nix build (Ctrl-C, a kill) ends the script with
// flake.nix as it was, not with Nix's fake hash in it, and the nix build is ended too.
func TestFlakeHashScriptSignal(t *testing.T) {
	for _, sig := range []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP} {
		t.Run(sig.String(), func(t *testing.T) {
			dir, env := flakeHashFolder(t, "new sum line\n", "")
			slowNix := "#!/bin/sh\necho \"$@\" >> '" + dir + "/nix-calls'\necho $$ > '" + dir + "/nix-pid'\nexec sleep 60\n"
			if err := os.WriteFile(filepath.Join(dir, "fake-nix"), []byte(slowNix), 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("sh", filepath.Join(dir, "scripts", "flake-hash"))
			cmd.Env = env
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			t.Cleanup(func() { cmd.Process.Kill() })
			// The fake hash is in flake.nix while nix runs: wait for nix to be there.
			pidFile := filepath.Join(dir, "nix-pid")
			for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(20 * time.Millisecond) {
				if _, err := os.Stat(pidFile); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("the script did not call nix")
				}
			}
			if now, _ := os.ReadFile(filepath.Join(dir, "flake.nix")); !strings.Contains(string(now), "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") {
				t.Fatal("flake.nix has no fake hash while nix runs: the test does not test the restore")
			}
			cmd.Process.Signal(sig)
			select {
			case err := <-done:
				if err == nil {
					t.Error("the script ended with status 0 after the signal")
				}
			case <-time.After(30 * time.Second):
				t.Fatal("the script did not end after the signal")
			}
			if now, _ := os.ReadFile(filepath.Join(dir, "flake.nix")); string(now) != readRepoFile(t, "flake.nix") {
				t.Errorf("flake.nix was left changed after %v", sig)
			}
			if pid, _ := os.ReadFile(pidFile); len(pid) > 0 {
				if err := exec.Command("kill", "-0", strings.TrimSpace(string(pid))).Run(); err == nil {
					exec.Command("kill", strings.TrimSpace(string(pid))).Run()
					t.Errorf("the nix build (pid %s) is still running", strings.TrimSpace(string(pid)))
				}
			}
		})
	}
}
