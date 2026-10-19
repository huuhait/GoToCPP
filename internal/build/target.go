package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/huuhait/gocpp/internal/toolchain"
)

// crossCXX are the cross compilers that FindCXXFor knows, by GOOS/GOARCH:
// the MinGW-w64 g++ (the dev shell has it, and Linux distributions package
// it under this name). Windows builds use MinGW-w64 first (ADR-021).
//
// windows/amd64 is not in the table: it has its own rule (internal/toolchain, GOCPP_TOOLCHAIN),
// a MinGW-w64 compiler in PATH, else the pinned llvm-mingw. linux/arm64 is best effort (Phase 06 step 6.13): the GNU
// cross compiler of the distribution, which brings its own sysroot.
var crossCXX = map[string]string{
	"linux/arm64": "aarch64-linux-gnu-g++",
	"linux/amd64": "x86_64-linux-gnu-g++", // for an arm64 host
}

type downloadKey struct{}

type downloadOpts struct{ progress io.Writer }

// WithToolchainDownload returns a context under which FindCXXFor may download the pinned Windows
// toolchain (GOCPP_TOOLCHAIN=auto or pinned, and none in PATH); progress gets the lines about it
// (nil is standard error). gocpp build and gocpp run use it; gocpp check and the language server
// don't, so they never start a download: they use a toolchain that is already there.
func WithToolchainDownload(ctx context.Context, progress io.Writer) context.Context {
	return context.WithValue(ctx, downloadKey{}, downloadOpts{progress})
}

// ToolchainConfig returns the configuration of the Windows toolchain for ctx: the environment's
// (GOCPP_TOOLCHAIN, GOCPP_TOOLCHAIN_URL), and a download only under WithToolchainDownload.
func ToolchainConfig(ctx context.Context) (toolchain.Config, error) {
	opts, ok := ctx.Value(downloadKey{}).(downloadOpts)
	cfg, err := toolchain.FromEnv(ok)
	if err != nil {
		return cfg, err
	}
	cfg.Progress = opts.progress
	return cfg, nil
}

// FindCXXFor finds the C++ compiler for the target goos/goarch, whose
// headers are the ones a .gp file built for that target includes
// (<windows.h> for windows). For the system that runs gocpp (runtime.GOOS
// and runtime.GOARCH), it is FindCXX: GOCPP_CXX, else CXX, else clang++,
// else g++. For another target, it is the program in GOCPP_CXX, which
// then names the compiler for that target, else the known cross compiler
// of the target, looked up in PATH (aarch64-linux-gnu-g++ for
// linux/arm64); for windows/amd64 it is the rule of GOCPP_TOOLCHAIN
// (findWindowsCXX: MinGW-w64 in PATH or in the folders of MSYS2, Chocolatey, Scoop and WinGet, else the pinned llvm-mingw). CXX is not used for another target: it names the host's compiler
// (a Nix shell sets it to g++). A target without a known cross compiler
// needs GOCPP_CXX; without it, FindCXXFor fails with "no C++ compiler for
// windows/arm64 (set GOCPP_CXX)".
//
// Phase 02 adds the flags of a target (--target for clang++); until then
// a compiler is right for one target only.
func FindCXXFor(ctx context.Context, goos, goarch string) (CXX, error) {
	// A Windows host is a Windows target like any other: its compiler is a MinGW-w64 one, found by
	// the rules of findWindowsCXX, and it gets the target's flags (Phase 14 step 14.2).
	if goos == runtime.GOOS && goarch == runtime.GOARCH && goos != "windows" {
		return FindCXX(ctx)
	}
	target := goos + "/" + goarch
	if cxx, ok, err := cxxFromEnv(ctx, "GOCPP_CXX"); ok {
		cxx.GOOS, cxx.GOARCH = goos, goarch
		return cxx, err
	}
	// On a Windows host CXX names the compiler, as on every host (Linux's CXX is the host's compiler,
	// which is why another target ignores it).
	if goos == runtime.GOOS {
		if cxx, ok, err := cxxFromEnv(ctx, "CXX"); ok {
			cxx.GOOS, cxx.GOARCH = goos, goarch
			return cxx, err
		}
	}
	if target == "windows/amd64" {
		return findWindowsCXX(ctx, goos, goarch)
	}
	prog, ok := crossCXX[target]
	if !ok {
		return CXX{}, fmt.Errorf("no C++ compiler for %s (set GOCPP_CXX)", target)
	}
	if _, err := exec.LookPath(prog); err != nil {
		return CXX{}, fmt.Errorf("no C++ compiler for %s: %s is not in PATH (set GOCPP_CXX)", target, prog)
	}
	cxx, err := LookCXX(ctx, prog)
	cxx.GOOS, cxx.GOARCH = goos, goarch
	return cxx, err
}

// findWindowsCXX finds the compiler for windows/amd64 by GOCPP_TOOLCHAIN (see package toolchain):
// MinGW-w64's clang++ or g++ in PATH, else the pinned llvm-mingw.
func findWindowsCXX(ctx context.Context, goos, goarch string) (CXX, error) {
	cfg, err := ToolchainConfig(ctx)
	if err != nil {
		return CXX{}, err
	}
	found, err := toolchain.Resolve(ctx, cfg, goos, goarch)
	if err != nil {
		return CXX{}, err
	}
	cxx, err := LookCXX(ctx, found.Path)
	cxx.GOOS, cxx.GOARCH = goos, goarch
	cxx.Source = found.Source
	return cxx, err
}

// cxxFromEnv returns the compiler that the environment variable name
// (GOCPP_CXX or CXX, or GOCPP_CC for the C compiler) names, and whether it
// is set. Its value is a program and maybe arguments, split into words by
// SplitCommand ("ccache g++"). This is the one place that reads them, for
// FindCXX, FindCXXFor and FindCC. An error says which variable it came
// from.
func cxxFromEnv(ctx context.Context, name string) (CXX, bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return CXX{}, false, nil
	}
	words, err := SplitCommand(value)
	if err != nil {
		return CXX{}, true, fmt.Errorf("%s=%s: %w", name, value, err)
	}
	if len(words) == 0 {
		return CXX{}, true, fmt.Errorf("%s=%q names no program", name, value)
	}
	cxx, err := lookCommand(ctx, words[0], words[1:], name == "GOCPP_CC")
	if err != nil {
		return CXX{}, true, fmt.Errorf("%s: %w", name, err)
	}
	return cxx, true, nil
}
