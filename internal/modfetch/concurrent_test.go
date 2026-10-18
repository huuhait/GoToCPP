package modfetch

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Many Fetchers (as many gocpp processes) download one module at once: the
// cache ends up with one consistent copy, and the host is asked once, because
// the file lock makes the others wait and use the cache.
func TestConcurrentFetchers(t *testing.T) {
	h := newFakeHost(t)
	archive := tarGz("", library("tiny-1.2.3")...)
	path := h.tag("acme/tiny", tver, archive)
	h.handle(path, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.Write(archive)
	})
	cfg := h.config()
	cleanupCache(t, cfg.CacheDir)
	const n = 8
	var wg sync.WaitGroup
	hashes := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := *cfg // its own Config and Fetcher, as in another process
			f, err := New(&c)
			if err != nil {
				errs[i] = err
				return
			}
			m, err := f.Download(context.Background(), tmod, tver, nil)
			if err == nil {
				hashes[i] = m.Hash
			}
			errs[i] = err
		}()
	}
	wg.Wait()
	for i := range hashes {
		if errs[i] != nil || hashes[i] != hashes[0] || hashes[i] == "" {
			t.Errorf("fetcher %d: %q, %v", i, hashes[i], errs[i])
		}
	}
	if c := h.count(path); c != 1 {
		t.Errorf("the archive was downloaded %d times, want 1", c)
	}
	f, _ := New(cfg)
	assertNoTemp(t, f)
	if got := readTree(t, f.ModDir(tmod, tver)); len(got) != 3 {
		t.Errorf("tree = %v", got)
	}
}

// Without the lock, two downloads race to rename: one wins, the other sees the
// same hash and uses the winner's folder.
func TestPublishRace(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	repo, _ := ParseModulePath(tmod)
	a, err := f.stage(context.Background(), repo, refFor(repo, tver))
	if err != nil {
		t.Fatal(err)
	}
	defer a.cleanup()
	b, err := f.stage(context.Background(), repo, refFor(repo, tver))
	if err != nil {
		t.Fatal(err)
	}
	defer b.cleanup()
	ma, err := f.publish(context.Background(), a, repo, tver, nil)
	if err != nil || !ma.Downloaded {
		t.Fatalf("first publish: %+v %v", ma, err)
	}
	mb, err := f.publish(context.Background(), b, repo, tver, nil)
	if err != nil || mb.Downloaded || mb.Hash != ma.Hash {
		t.Fatalf("second publish: %+v %v", mb, err)
	}

	// A second download with other content for the same version: an error,
	// and the first stays.
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3", tfile{Name: "tiny-1.2.3/other", Body: "x"})...))
	c, err := f.stage(context.Background(), repo, refFor(repo, tver))
	if err != nil {
		t.Fatal(err)
	}
	defer c.cleanup()
	if _, err := f.publish(context.Background(), c, repo, tver, nil); err == nil || !strings.Contains(err.Error(), "served different files") {
		t.Errorf("publish with other content: %v", err)
	}
	if got := readTree(t, ma.Dir); len(got) != 3 {
		t.Errorf("the cached module changed: %v", got)
	}
}

// TestHelperDownload is the child of TestConcurrentProcesses.
func TestHelperDownload(t *testing.T) {
	if os.Getenv("MODFETCH_HELPER") != "1" {
		t.Skip("helper process")
	}
	cfg, err := ConfigFromEnv(os.Getenv, nil)
	if err != nil {
		t.Fatal(err)
	}
	f, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	m, err := f.Fetch(context.Background(), os.Getenv("MODFETCH_MAIN"), tmod, tver, FetchOptions{Record: true})
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout.WriteString("HASH " + m.Hash + "\n")
}

func TestConcurrentProcesses(t *testing.T) {
	h := newFakeHost(t)
	archive := tarGz("", library("tiny-1.2.3")...)
	path := h.tag("acme/tiny", tver, archive)
	h.handle(path, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Write(archive)
	})
	cache := filepath.Join(t.TempDir(), "cache")
	cleanupCache(t, cache)
	main := t.TempDir()
	const n = 5
	outs := make([][]byte, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperDownload$")
			cmd.Env = append(os.Environ(), "MODFETCH_HELPER=1", "MODFETCH_MAIN="+main,
				"GOCPP_MODCACHE="+cache, "GOCPP_MODURL=github.com="+h.srv.URL, "GOCPP_MODCACERT="+h.caFile())
			outs[i], errs[i] = cmd.CombinedOutput()
		}()
	}
	wg.Wait()
	var hash string
	for i := range outs {
		if errs[i] != nil {
			t.Errorf("process %d: %v\n%s", i, errs[i], outs[i])
			continue
		}
		var got string
		for _, l := range strings.Split(string(outs[i]), "\n") {
			if strings.HasPrefix(l, "HASH ") {
				got = l
			}
		}
		if got == "" || hash != "" && got != hash {
			t.Errorf("process %d printed %q (first: %q)", i, got, hash)
		}
		hash = got
	}
	if c := h.count(path); c != 1 {
		t.Errorf("%d downloads by %d processes, want 1", c, n)
	}
	// gocpp.sum has exactly one line, in the right format.
	data, _ := os.ReadFile(filepath.Join(main, "gocpp.sum"))
	if want := tmod + " " + tver + " " + strings.TrimPrefix(hash, "HASH ") + "\n"; string(data) != want {
		t.Errorf("gocpp.sum = %q, want %q", data, want)
	}
}

// Several modules recorded in gocpp.sum by many goroutines: no line is lost.
func TestConcurrentSumRecording(t *testing.T) {
	h := newFakeHost(t)
	cfg := h.config()
	cleanupCache(t, cfg.CacheDir)
	main := t.TempDir()
	const n = 12
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		repo := "acme/m" + string(rune('a'+i))
		h.tag(repo, "v1.0.0", tarGz("", library("m-1.0.0", tfile{Name: "m-1.0.0/id", Body: repo})...))
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := *cfg
			f, _ := New(&c)
			if _, err := f.Fetch(context.Background(), main, "github.com/acme/m"+string(rune('a'+i)), "v1.0.0", FetchOptions{Record: true}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	s, err := LoadSums(main)
	if err != nil || len(s.Lines()) != n {
		t.Errorf("gocpp.sum has %d lines (%v), want %d", len(s.Lines()), err, n)
	}
	data, _ := os.ReadFile(filepath.Join(main, "gocpp.sum"))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i-1] > lines[i] {
			t.Errorf("gocpp.sum is not sorted:\n%s", data)
			break
		}
	}
}
