package modfetch

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTruncatedDownload(t *testing.T) {
	good := tarGz("", library("tiny-1.2.3")...)
	t.Run("short body", func(t *testing.T) {
		h := newFakeHost(t)
		h.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "100000")
			w.Write(good[:len(good)/2])
		})
		f := h.fetcher()
		cleanupCache(t, f.cfg.CacheDir)
		_, err := f.Download(context.Background(), tmod, tver, nil)
		if err == nil || !strings.Contains(err.Error(), "download failed") {
			t.Fatalf("error = %v", err)
		}
		assertNoTemp(t, f)
		if _, err := f.cached(context.Background(), tmod, tver, nil); err != nil {
			t.Error(err)
		}
	})
	t.Run("complete http, truncated gzip", func(t *testing.T) {
		h := newFakeHost(t)
		h.serve("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", good[:len(good)-10])
		f := h.fetcher()
		cleanupCache(t, f.cfg.CacheDir)
		_, err := f.Download(context.Background(), tmod, tver, nil)
		if err == nil || !strings.Contains(err.Error(), "truncated") {
			t.Fatalf("error = %v", err)
		}
		assertNoTemp(t, f)
		if m, _ := f.cached(context.Background(), tmod, tver, nil); m != nil {
			t.Error("the truncated module is in the cache")
		}
	})
	t.Run("chunked, cut off", func(t *testing.T) {
		h := newFakeHost(t)
		h.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) {
			w.Write(good[:len(good)/2])
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler) // the connection is closed with no final chunk
		})
		f := h.fetcher()
		cleanupCache(t, f.cfg.CacheDir)
		if _, err := f.Download(context.Background(), tmod, tver, nil); err == nil {
			t.Fatal("no error")
		}
		assertNoTemp(t, f)
	})
}

func TestOversizedArchive(t *testing.T) {
	big := tarGz("", library("tiny-1.2.3", tfile{Name: "tiny-1.2.3/big", Body: strings.Repeat("0123456789abcdef", 20000)})...)
	t.Run("content-length", func(t *testing.T) {
		h := newFakeHost(t)
		h.serve("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", append(big, make([]byte, 5000)...))
		cfg := h.config()
		cfg.Limits.MaxArchive = 1000
		cleanupCache(t, cfg.CacheDir)
		f, _ := New(cfg)
		_, err := f.Download(context.Background(), tmod, tver, nil)
		var tl *TooLargeError
		if !errors.As(err, &tl) {
			t.Fatalf("error = %v", err)
		}
		assertNoTemp(t, f)
	})
	t.Run("no content-length", func(t *testing.T) {
		h := newFakeHost(t)
		stopped := make(chan struct{})
		var sent atomic.Int64
		h.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) {
			defer close(stopped)
			chunk := make([]byte, 4096)
			for { // an endless stream: it ends when the client hangs up
				n, err := w.Write(chunk)
				sent.Add(int64(n))
				if err != nil {
					return
				}
				w.(http.Flusher).Flush()
			}
		})
		cfg := h.config()
		cfg.Limits.MaxArchive = 100_000
		cleanupCache(t, cfg.CacheDir)
		f, _ := New(cfg)
		_, err := f.Download(context.Background(), tmod, tver, nil)
		var tl *TooLargeError
		if !errors.As(err, &tl) {
			t.Fatalf("error = %v", err)
		}
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			t.Error("the client did not hang up on an endless stream")
		}
		// The client stops reading at the limit: what the server could write is
		// the limit and what the sockets hold (a few MiB), not more.
		if n := sent.Load(); n > cfg.Limits.MaxArchive+16<<20 {
			t.Errorf("the server wrote %d bytes of an endless stream, the limit is %d", n, cfg.Limits.MaxArchive)
		}
		assertNoTemp(t, f)
	})
}

func TestUnsafeArchivesViaDownload(t *testing.T) {
	for name, arc := range map[string][]byte{
		"traversal": tarGz("", dirEntry("t/"), tfile{Name: "t/../../../outside", Body: "x"}),
		"symlink":   tarGz("", append(library("t"), tfile{Name: "t/link", Type: 50 /* '2' symlink */, Linkname: "/etc/passwd"})...),
	} {
		t.Run(name, func(t *testing.T) {
			h := newFakeHost(t)
			h.tag("acme/tiny", tver, arc)
			f := h.fetcher()
			cleanupCache(t, f.cfg.CacheDir)
			_, err := f.Download(context.Background(), tmod, tver, nil)
			var ee *ExtractError
			if !errors.As(err, &ee) {
				t.Fatalf("error = %v", err)
			}
			assertNoTemp(t, f)
			if m, _ := f.cached(context.Background(), tmod, tver, nil); m != nil {
				t.Error("in the cache")
			}
		})
	}
}

func TestRedirectDropsCredentials(t *testing.T) {
	a, b := newFakeHost(t), newFakeHost(t)
	archive := tarGz("", library("tiny-1.2.3")...)
	b.serve("/codeload/acme/tiny.tar.gz", archive)
	a.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, b.srv.URL+"/codeload/acme/tiny.tar.gz", http.StatusFound)
	})
	cfg := a.config()
	u, _ := url.Parse(a.srv.URL)
	cfg.Auth = map[string]string{u.Host: "s3cret-token"}
	cleanupCache(t, cfg.CacheDir)
	f, _ := New(cfg)
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	if got := a.auth["/acme/tiny/archive/refs/tags/v1.2.3.tar.gz"]; len(got) != 1 || got[0] != "Bearer s3cret-token" {
		t.Errorf("the first host got %q", got)
	}
	if got := b.auth["/codeload/acme/tiny.tar.gz"]; len(got) != 1 || got[0] != "" {
		t.Errorf("the host of the redirect got the Authorization header %q", got)
	}
}

func TestBasicAuthAndNoLeakInErrors(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	cfg := h.config()
	u, _ := url.Parse(h.srv.URL)
	cfg.Auth = map[string]string{u.Hostname(): "me:pw"}
	cleanupCache(t, cfg.CacheDir)
	f, _ := New(cfg)
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	if got := h.auth["/acme/tiny/archive/refs/tags/v1.2.3.tar.gz"]; len(got) != 1 || !strings.HasPrefix(got[0], "Basic ") {
		t.Errorf("Authorization = %q", got)
	}
	// An error never shows the secret.
	h.handle("/acme/tiny/archive/refs/tags/v1.2.4.tar.gz", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusForbidden) })
	_, err := f.Download(context.Background(), tmod, "v1.2.4", nil)
	if err == nil || strings.Contains(err.Error(), "pw") && strings.Contains(err.Error(), "me:pw") || !strings.Contains(err.Error(), "GOCPP_MODAUTH") {
		t.Errorf("error = %v", err)
	}
}

func TestRedirectLimits(t *testing.T) {
	h := newFakeHost(t)
	var n atomic.Int32
	h.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	h.handle("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	_, err := f.Download(context.Background(), tmod, tver, nil)
	if err == nil || !strings.Contains(err.Error(), "stopped after 5 redirects") {
		t.Errorf("error = %v", err)
	}
	if c := h.count("/loop"); c > 6 {
		t.Errorf("%d redirects were followed", c)
	}

	// A redirect to plain http is refused.
	h.handle("/acme/tiny/archive/refs/tags/v1.2.4.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1:1/x.tar.gz", http.StatusFound)
	})
	_, err = f.Download(context.Background(), tmod, "v1.2.4", nil)
	if err == nil || !strings.Contains(err.Error(), "not an https URL") {
		t.Errorf("error = %v", err)
	}
}

func TestPlainHTTPRefused(t *testing.T) {
	cfg := &Config{CacheDir: t.TempDir(), URLs: []URLRule{{Template: "http://example.test/{path}/{version}.tar.gz"}}}
	f, _ := New(cfg)
	_, err := f.Download(context.Background(), tmod, tver, nil)
	if err == nil || !strings.Contains(err.Error(), "not an https URL") {
		t.Errorf("error = %v", err)
	}
}

func TestTLSIsVerified(t *testing.T) {
	h := newFakeHost(t)
	h.tag("acme/tiny", tver, tarGz("", library("tiny-1.2.3")...))
	cfg := h.config()
	cfg.Client = nil // the default client trusts the system roots, not the test server
	cleanupCache(t, cfg.CacheDir)
	f, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Download(context.Background(), tmod, tver, nil)
	if err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("error = %v, want a certificate error", err)
	}
	if h.total() != 0 {
		t.Error("the request got through")
	}
	// With the CA of the server added (a company CA), it works.
	cfg.CAFile = h.caFile()
	f, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	// A CA file that is no certificate is an error.
	cfg.CAFile = h.caFile() + ".missing"
	f, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cleanupCache(t, cfg.CacheDir)
	RemoveAll(cfg.CacheDir)
	if _, err := f.Download(context.Background(), tmod, tver, nil); err == nil || !strings.Contains(err.Error(), "GOCPP_MODCACERT") {
		t.Errorf("a missing CA file: %v", err)
	}
}

func TestRetryOn503(t *testing.T) {
	old := retryDelay
	retryDelay = 5 * time.Millisecond
	defer func() { retryDelay = old }()
	h := newFakeHost(t)
	var n atomic.Int32
	archive := tarGz("", library("tiny-1.2.3")...)
	h.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Write(archive)
	})
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	if _, err := f.Download(context.Background(), tmod, tver, nil); err != nil {
		t.Fatal(err)
	}
	if n.Load() != 3 {
		t.Errorf("%d requests", n.Load())
	}
	// A server that is always busy fails after 3 tries.
	n.Store(-100)
	h.handle("/acme/tiny/archive/refs/tags/v1.2.4.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		http.Error(w, "busy", http.StatusServiceUnavailable)
	})
	n.Store(0)
	_, err := f.Download(context.Background(), tmod, "v1.2.4", nil)
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 503 || n.Load() != 3 {
		t.Errorf("error = %v after %d requests", err, n.Load())
	}
}

func TestServerErrorIsNotNotFound(t *testing.T) {
	h := newFakeHost(t)
	h.handle("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) })
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	_, err := f.Download(context.Background(), tmod, tver, nil)
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 500 {
		t.Fatalf("error = %v", err)
	}
	var nf *ModuleNotFoundError
	if errors.As(err, &nf) {
		t.Error("a 500 is reported as not found")
	}
}

func TestHTMLInsteadOfArchive(t *testing.T) {
	h := newFakeHost(t)
	h.serve("/acme/tiny/archive/refs/tags/v1.2.3.tar.gz", []byte("<html>Sign in to continue</html>"))
	f := h.fetcher()
	cleanupCache(t, f.cfg.CacheDir)
	_, err := f.Download(context.Background(), tmod, tver, nil)
	if err == nil || !strings.Contains(err.Error(), "not a tar.gz or zip file") {
		t.Errorf("error = %v", err)
	}
	assertNoTemp(t, f)
}
