package modfetch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRealNetwork downloads a tiny public repository from github.com. It is
// skipped unless GOCPP_TEST_NETWORK=1 (the CI has no network):
//
//	GOCPP_TEST_NETWORK=1 go test -run TestRealNetwork ./internal/modfetch
//
// github.com/rxi/vec is a growable array in C, a few files, with the tag
// v0.2.1. The h1 hash of its files must not change between downloads, or
// gocpp.sum would not work.
func TestRealNetwork(t *testing.T) {
	if os.Getenv("GOCPP_TEST_NETWORK") != "1" {
		t.Skip("set GOCPP_TEST_NETWORK=1 to download from github.com")
	}
	const (
		path   = "github.com/rxi/vec"
		ver    = "v0.2.1"
		commit = "986e92df4cb60c7d247c752286a24a588d968139"
	)
	var hashes []string
	for i := 0; i < 2; i++ {
		f, err := New(&Config{CacheDir: filepath.Join(t.TempDir(), "cache"), Limits: DefaultLimits, Stderr: os.Stderr})
		if err != nil {
			t.Fatal(err)
		}
		m, err := f.Download(context.Background(), path, ver, nil)
		if err != nil {
			t.Fatal(err)
		}
		RemoveAllAfter(t, f.cfg.CacheDir)
		files := readTree(t, m.Dir)
		if files["src/vec.h"] == "" || files["src/vec.c"] == "" {
			t.Fatalf("files = %v", keys(files))
		}
		hashes = append(hashes, m.Hash)
	}
	if hashes[0] != hashes[1] {
		t.Errorf("two downloads have different hashes: %v", hashes)
	}
	t.Logf("%s %s %s", path, ver, hashes[0])
	if hashes[0] != "h1:"+realVecHash {
		t.Errorf("hash = %s, want h1:%s", hashes[0], realVecHash)
	}

	// @latest and @commit through git's smart HTTP.
	f, _ := New(&Config{CacheDir: filepath.Join(t.TempDir(), "cache"), Limits: DefaultLimits})
	RemoveAllAfter(t, f.cfg.CacheDir)
	main := t.TempDir()
	m, err := f.Get(context.Background(), main, path, "latest")
	if err != nil || m.Version != ver {
		t.Fatalf("latest = %+v, %v", m, err)
	}
	m2, err := f.Get(context.Background(), main, path, commit[:12])
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := PseudoCommit(m2.Version); c != commit[:12] {
		t.Errorf("pseudo-version %s", m2.Version)
	}
	// The commit of the tag has the same files as the tag; the hashes differ
	// because the version is part of the hashed names.
	a, b := readTree(t, m.Dir), readTree(t, m2.Dir)
	if len(a) != len(b) || a["src/vec.c"] != b["src/vec.c"] {
		t.Errorf("the tag and its commit have different files")
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// RemoveAllAfter removes a read-only cache when the test ends.
func RemoveAllAfter(t *testing.T, dir string) { t.Cleanup(func() { RemoveAll(dir) }) }

// realVecHash is the hash (without "h1:") of github.com/rxi/vec v0.2.1 as
// of 2026-10-17; "" until it is recorded.
const realVecHash = "02rRcJcpFOFLqEd5WH9FkVlogKg8fe80aRSnl3zrVqo="
