package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
)

func TestWarmStaysBusy(t *testing.T) {
	root := writeTree(t, fastFiles)
	srv := newServerOpts(nil, testConfig(), debounce.RealClock{}, Options{WarmImporters: false})
	c := &refClient{dclient: &dclient{client: startServer(t, srv, nil), root: root}, src: fastFiles}
	c.call("initialize", plainInit)
	c.notify("initialized", "{}")
	c.skipShown = true
	c.open("a/a.gp", fastFiles["a/a.gp"])
	c.next("a/a.gp")
	srv.obs.gc.begin()
	defer srv.obs.gc.end()
	t.Run("busy past bounded wait", func(t *testing.T) {
		_, _, before := srv.pipeline.Stats()
		if srv.warmOne(srv.comp.index.ctx, filepath.Join(root, "b")) {
			t.Fatal("busy package was counted as warmed")
		}
		if _, _, after := srv.pipeline.Stats(); after != before {
			t.Fatalf("warming checked %d packages during foreground work", after-before)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(srv.comp.index.ctx)
		cancel()
		_, _, before := srv.pipeline.Stats()
		if srv.warmOne(ctx, filepath.Join(root, "c")) {
			t.Fatal("canceled package was counted as warmed")
		}
		if _, _, after := srv.pipeline.Stats(); after != before {
			t.Fatal("canceled warming checked a package")
		}
	})
}

// TestWarmImporters: after the first diagnostics of a package, the packages that
// import it are checked without a request, once (and only with the option on).
func TestWarmImporters(t *testing.T) {
	for _, on := range []bool{false, true} {
		root := writeTree(t, fastFiles)
		srv := newServerOpts(nil, testConfig(), debounce.RealClock{}, Options{WarmImporters: on})
		c := &refClient{dclient: &dclient{client: startServer(t, srv, nil), root: root}, src: fastFiles}
		c.call("initialize", plainInit)
		c.notify("initialized", "{}")
		c.skipShown = true
		c.open("a/a.gp", fastFiles["a/a.gp"])
		c.next("a/a.gp")
		// a and all seven importers b, c, d, e, f, g, mixed. The mixed
		// importer has qualified and dot imports in separate files.
		deadline := time.After(10 * time.Second)
		for {
			_, _, n := srv.pipeline.Stats()
			if on && n >= 8 {
				break
			}
			if !on {
				time.Sleep(300 * time.Millisecond)
				if _, _, n := srv.pipeline.Stats(); n != 1 {
					t.Errorf("warming off: %d checks, want 1 (the open package)", n)
				}
				break
			}
			select {
			case <-deadline:
				t.Fatalf("warming on: %d checks after 10 s, want at least 8", n)
			case <-time.After(5 * time.Millisecond):
			}
		}
		if on {
			// A second call for the same package starts nothing.
			srv.warmImporters(c.root + "/a")
			srv.warmImporters(filepath.Join(c.root, "a"))
			srv.warm.mu.Lock()
			runs := srv.warm.runs
			srv.warm.mu.Unlock()
			if runs != 1 {
				t.Errorf("%d warmings for one package, want 1", runs)
			}
		}
		if on {
			// The importers are in the cache: a references request finds the uses without checks.
			_, _, before := srv.pipeline.Stats()
			if refs := c.references("a/a.gp", "func |F()", true); len(refs) < 5 {
				t.Errorf("references: %v", refs)
			}
			if _, _, after := srv.pipeline.Stats(); after != before {
				t.Errorf("references made %d checks after warming", after-before)
			}
		}
	}
}
