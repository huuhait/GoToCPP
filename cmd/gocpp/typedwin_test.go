package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
)

// TestTypedWindows builds the programs of tests/typed that say `// windows: yes` for windows/amd64
// (MinGW-w64) in typed mode and runs them on the real Windows PC of GOCPP_WINDOWS_SSH=user@host (never
// Wine): their headers are the C++ library's only, so the types are the same on both systems, and
// the data model (long and size_t) is Windows's. The output is the C++ twin's. Skipped without the
// variable; with it, an unreachable PC is a failure.
func TestTypedWindows(t *testing.T) {
	host := os.Getenv("GOCPP_WINDOWS_SSH")
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host runs the programs on the real Windows PC")
	}
	progs := loadTypedPrograms(t)
	remote, err := winssh.New(host)
	if err != nil {
		t.Fatalf("the Windows PC is unreachable: %v", err)
	}
	defer remote.Close()
	typedEnv(t)
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	typedIn(t)
	needClang(t)
	for _, p := range progs {
		if !p.windows {
			continue
		}
		t.Run(p.name, func(t *testing.T) {
			t.Setenv("GOCPP_INTEROP", "typed")
			exe := filepath.Join(t.TempDir(), p.name+".exe")
			r := gocppRun("", false, "build", "-os", "windows", "-o", exe, p.file)
			if r.code != 0 {
				t.Fatalf("typed build for windows: exit %d\n%s", r.code, r.stderr)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			res, err := remote.Run(ctx, 2*time.Minute, exe, nil, nil, false)
			if err != nil {
				t.Fatalf("running on the PC: %v", err)
			}
			if res.Code != 0 || len(res.Stderr) != 0 {
				t.Fatalf("on the PC: exit %d\nstderr:\n%s\nstdout:\n%s", res.Code, res.Stderr, res.Stdout)
			}
			if string(res.Stdout) != p.want {
				t.Fatalf("on the PC, stdout differs from the // Output: block:\ngot:\n%s\nwant:\n%s", res.Stdout, p.want)
			}
		})
	}
}
