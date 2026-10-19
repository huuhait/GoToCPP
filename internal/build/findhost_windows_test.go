//go:build windows

package build

import (
	"context"
	"strings"
	"testing"
)

// On a Windows host the compiler is found by the rule for the Windows target and carries the target
// (step 14.2): this runs on the real PC (tests/windows_native_test.go), where a MinGW-w64 is installed.
func TestFindCXXForWindowsHost(t *testing.T) {
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	t.Setenv("GOCPP_OFFLINE", "1")
	cxx, err := FindCXXFor(context.Background(), "windows", "amd64")
	if err != nil {
		t.Fatalf("FindCXXFor(windows, amd64) on a Windows host: %v", err)
	}
	if cxx.GOOS != "windows" || cxx.GOARCH != "amd64" {
		t.Errorf("the compiler's target = %s/%s, want windows/amd64: the target's flags would be missing", cxx.GOOS, cxx.GOARCH)
	}
	if cxx.Source == "" {
		t.Errorf("no source for the compiler %s", cxx.Path)
	}
	if got := targetFlags(cxx); !strings.Contains(strings.Join(got, " "), "-D_WIN32_WINNT=0x0A00") {
		t.Errorf("targetFlags = %v", got)
	}
	// GOCPP_CXX and CXX name the compiler, which then gets the target too.
	for _, env := range []string{"GOCPP_CXX", "CXX"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv(env, cxx.Path)
			got, err := FindCXXFor(context.Background(), "windows", "amd64")
			if err != nil || got.Path != cxx.Path || got.GOOS != "windows" {
				t.Errorf("with %s: %+v, %v", env, got, err)
			}
		})
	}
}
