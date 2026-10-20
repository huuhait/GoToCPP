package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
)

// TestInteropFallbackWindows is the clang-missing fallback on the real Windows PC of
// GOCPP_WINDOWS_SSH=user@host (never Wine): a program that imports the C++ library is built for
// windows/amd64 with no clang++ to ask (GOCPP_CLANG names a program that does not exist), in the modes
// auto and opaque (no word, no failure), runs on the PC and prints what the program prints; the mode typed
// is the error of status 3. The PC has no clang, which is the case the fallback is for. Skipped without
// the variable; with it, an unreachable PC is a failure.
func TestInteropFallbackWindows(t *testing.T) {
	host := os.Getenv("GOCPP_WINDOWS_SSH")
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host runs the programs on the real Windows PC")
	}
	remote, err := winssh.New(host)
	if err != nil {
		t.Fatalf("the Windows PC is unreachable: %v", err)
	}
	defer remote.Close()
	const src = `package main

import (
	"fmt"
	"<cmath>"
	"<vector>"
)

func main() {
	var v std.vector[int]
	v.push_back(3)
	v.push_back(4)
	fmt.Println(int(v.size()), cmath.sqrt(float64(v[0]*v[0]+v[1]*v[1])))
}
`
	interopSetup(t, map[string]string{"a.gp": src})
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_CLANG", "/nonexistent/clang++")
	for _, mode := range []string{"auto", "opaque"} {
		t.Run(mode, func(t *testing.T) {
			exe := filepath.Join(t.TempDir(), "a.exe")
			r := gocppRun("", false, "build", "-os", "windows", "-interop="+mode, "-o", exe, "a.gp")
			if r.code != 0 || r.stderr != "" {
				t.Fatalf("build for windows without clang++: exit %d\n%s", r.code, r.stderr)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			res, err := remote.Run(ctx, 2*time.Minute, exe, nil, nil, false)
			if err != nil {
				t.Fatalf("running on the PC: %v", err)
			}
			if res.Code != 0 || len(res.Stderr) != 0 || strings.ReplaceAll(string(res.Stdout), "\r\n", "\n") != "2 5\n" {
				t.Fatalf("on the PC: exit %d\nstderr:\n%s\nstdout:\n%s", res.Code, res.Stderr, res.Stdout)
			}
		})
	}
	t.Run("typed", func(t *testing.T) {
		r := gocppRun("", false, "build", "-os", "windows", "-interop=typed", "-o", filepath.Join(t.TempDir(), "a.exe"), "a.gp")
		if r.code != exitNoClang || !strings.Contains(r.stderr, "typed interop needs clang++") {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
	})
}
