package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// crossCXX are the cross compilers that FindCXXFor knows, by GOOS/GOARCH:
// the MinGW-w64 g++ (the dev shell has it, and Linux distributions package
// it under this name). Windows builds use MinGW-w64 first (ADR-021).
var crossCXX = map[string]string{
	"windows/amd64": "x86_64-w64-mingw32-g++",
}

// FindCXXFor finds the C++ compiler for the target goos/goarch, whose
// headers are the ones a .gp file built for that target includes
// (<windows.h> for windows). For the system that runs gocpp (runtime.GOOS
// and runtime.GOARCH), it is FindCXX: GOCPP_CXX, else CXX, else clang++,
// else g++. For another target, it is the program in GOCPP_CXX, which
// then names the compiler for that target, else the known cross compiler
// of the target (x86_64-w64-mingw32-g++ for windows/amd64), looked up in
// PATH. CXX is not used for another target: it names the host's compiler
// (a Nix shell sets it to g++). A target without a known cross compiler
// needs GOCPP_CXX; without it, FindCXXFor fails with "no C++ compiler for
// windows/arm64 (set GOCPP_CXX)".
//
// Phase 02 adds the flags of a target (--target for clang++); until then
// a compiler is right for one target only.
func FindCXXFor(ctx context.Context, goos, goarch string) (CXX, error) {
	if goos == runtime.GOOS && goarch == runtime.GOARCH {
		return FindCXX(ctx)
	}
	target := goos + "/" + goarch
	if cxx, ok, err := cxxFromEnv(ctx, "GOCPP_CXX"); ok {
		return cxx, err
	}
	prog, ok := crossCXX[target]
	if !ok {
		return CXX{}, fmt.Errorf("no C++ compiler for %s (set GOCPP_CXX)", target)
	}
	if _, err := exec.LookPath(prog); err != nil {
		return CXX{}, fmt.Errorf("no C++ compiler for %s: %s is not in PATH (set GOCPP_CXX)", target, prog)
	}
	return LookCXX(ctx, prog)
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
