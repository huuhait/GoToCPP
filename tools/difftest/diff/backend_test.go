package diff

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	// the package builds nothing with the module cache or the network
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

// S6: a canceled build gets SIGTERM, so that gocpp can stop its compilers (which are in process
// groups of their own) instead of being killed and leaving them behind.
func TestBuildIsStoppedWithSigterm(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "got-sigterm")
	script := filepath.Join(dir, "fakegocpp")
	body := "#!/bin/sh\ntrap 'echo term > " + marker + "; exit 0' TERM\nwhile true; do sleep 0.1; done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	b := &GocppBackend{Gocpp: script, Cxx: "g++", Work: dir, XDG: dir, Timeout: time.Minute}
	done := make(chan error, 1)
	go func() { _, err := b.Run(ctx, "package main\n\nfunc main() {}\n"); done <- err }()
	time.Sleep(500 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v, want the context's error (an interrupted campaign says so)", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the build was not stopped")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("the build did not get SIGTERM before it was killed")
	}
}

func TestGoBackendInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := &GoBackend{Work: t.TempDir(), Cache: t.TempDir(), Timeout: time.Minute}
	if _, err := b.Run(ctx, "package main\n\nfunc main() {}\n"); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}

func TestGocppBackendArguments(t *testing.T) {
	dir := t.TempDir()
	argv := filepath.Join(dir, "argv")
	script := filepath.Join(dir, "fakegocpp")
	body := "#!/bin/sh\necho \"$@\" > " + argv + "\necho \"$GOCPP_OBJCACHE_SIZE\" >> " + argv + "\nexit 1\n"
	os.WriteFile(script, []byte(body), 0o755)
	for _, c := range []struct {
		b    GocppBackend
		want string
	}{
		{GocppBackend{Flags: "-g0", P: 8}, "build -cxxflags=-g0 -p 8 -o"},
		{GocppBackend{Asan: true, Flags: "-g0"}, "build -asan -o"},
		{GocppBackend{Release: true, Flags: "-g0"}, "build -release -o"},
	} {
		b := c.b
		b.Gocpp, b.Cxx, b.Work, b.XDG, b.Timeout = script, "g++", dir, dir, time.Minute
		if _, err := b.Run(context.Background(), "package main\n"); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(argv)
		if len(got) < len(c.want) || string(got[:len(c.want)]) != c.want {
			t.Errorf("%s: arguments %q, want %q first", b.Name(), got, c.want)
		}
		if !contains(string(got), "\n3G\n") {
			t.Errorf("the object cache size is not set: %q", got)
		}
	}
	if (&GocppBackend{Cxx: "g++", Release: true}).Name() != "gocpp/g+++release" {
		t.Error("the name of a release subject")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
