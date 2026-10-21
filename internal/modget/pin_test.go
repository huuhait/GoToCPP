package modget

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/modfetch"
)

// serveCounted makes the env use its proxy folder through a local HTTP server
// and returns a function that gives the request paths seen since the last call.
func (e *env) serveCounted() (requested func() []string) {
	var mu sync.Mutex
	var seen []string
	files := http.FileServer(http.Dir(e.proxy))
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path)
		mu.Unlock()
		files.ServeHTTP(w, r)
	}))
	e.t.Cleanup(srv.Close)
	e.cfg.Proxies = []modfetch.Proxy{{URL: srv.URL}}
	e.cfg.Client = srv.Client()
	f, err := modfetch.New(e.cfg)
	if err != nil {
		e.t.Fatal(err)
	}
	e.f = f
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		got := seen
		seen = nil
		return got
	}
}

func anyContains(texts []string, part string) []string {
	var out []string
	for _, t := range texts {
		if strings.Contains(t, part) {
			out = append(out, t)
		}
	}
	return out
}

// "get m@v1.1.0" reads the retract lines of the newest version (to warn about a
// retracted pin) but downloads no archive of it, and its hashes are not recorded
// in gocpp.sum: they are not the hashes of a module of the build.
func TestGetPinDoesNotRecordTheNewestVersion(t *testing.T) {
	e := newEnv(t, appMod, cat(
		v("example.com/m", "v1.0.0", "v1.1.0"),
		[]tmod{{Path: "example.com/m", Version: "v1.2.0", Extra: "retract v1.0.0\n"}},
	)...)
	requested := e.serveCounted()

	r := e.mustGet("example.com/m@v1.1.0")

	if got := anyContains(requested(), "v1.2.0.zip"); len(got) > 0 {
		t.Errorf("the archive of the newest version was asked for: %q", got)
	}
	if got := e.sum(); strings.Contains(got, "v1.2.0") {
		t.Errorf("gocpp.sum has lines of v1.2.0:\n%s", got)
	}
	if !strings.Contains(e.sum(), "example.com/m v1.1.0 h1:") {
		t.Errorf("gocpp.sum lacks the pinned module:\n%s", e.sum())
	}
	if len(r.Warnings) != 0 {
		t.Errorf("warnings = %q (v1.1.0 is not retracted)", r.Warnings)
	}
}

// The warning about a retracted pin stays: the newest module file is read.
func TestGetPinOfRetractedVersionWarnsWithoutRecordingTheNewest(t *testing.T) {
	e := newEnv(t, appMod, cat(
		v("example.com/m", "v1.0.0"),
		[]tmod{{Path: "example.com/m", Version: "v1.2.0", Extra: "retract v1.0.0\n"}},
	)...)
	e.serveCounted()

	r := e.mustGet("example.com/m@v1.0.0")

	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "example.com/m@v1.0.0 is retracted") {
		t.Errorf("warnings = %q", r.Warnings)
	}
	if got := e.sum(); strings.Contains(got, "v1.2.0") {
		t.Errorf("gocpp.sum has lines of v1.2.0:\n%s", got)
	}
}

// A pin of a module with a requirement asks for the pinned version and what it
// requires; of the newest version of the module only the module file is read
// (for the retract lines).
func TestGetPinAsksOnlyForTheVersionAndItsRequirements(t *testing.T) {
	e := newEnv(t, appMod, cat(
		[]tmod{{Path: "example.com/m", Version: "v1.0.0", Reqs: []string{"example.com/dep@v1.0.0"}}},
		[]tmod{{Path: "example.com/m", Version: "v1.1.0", Reqs: []string{"example.com/dep@v1.1.0"}}},
		v("example.com/dep", "v1.0.0", "v1.1.0", "v1.2.0"),
	)...)
	requested := e.serveCounted()

	e.mustGet("example.com/m@v1.0.0")

	seen := requested()
	for _, other := range []string{"m/@v/v1.1.0.zip", "dep/@v/v1.1.0", "dep/@v/v1.2.0"} {
		if got := anyContains(seen, other); len(got) > 0 {
			t.Errorf("asked for %s: %q", other, got)
		}
	}
	sum := e.sum()
	for _, other := range []string{"m v1.1.0", "dep v1.1.0", "dep v1.2.0"} {
		if strings.Contains(sum, other) {
			t.Errorf("gocpp.sum has %q:\n%s", other, sum)
		}
	}
}
