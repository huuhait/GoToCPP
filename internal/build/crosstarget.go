package build

import (
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// The targets of a build (Phase 06 step 6.13, ADR-055, docs/spec/build.md "Cross-compilation"). A
// target is a GOOS/GOARCH pair. gocpp builds programs for linux/amd64, linux/arm64 and
// windows/amd64 (Linux and Windows are the two operating systems of v1, ADR-021); gocpp check and
// gocpp emit accept every pair that the go command knows. windows/amd64 is built with MinGW-w64 and
// linked statically; linux/arm64 is best effort (its compiler and sysroot come from the user's
// environment, and the tests of the driver use fake tools).

var buildTargets = []string{"linux/amd64", "linux/arm64", "windows/amd64"}

// Targets returns the targets that programs can be built for ("linux/amd64", ...), sorted.
func Targets() []string {
	return slices.Clone(buildTargets)
}

// SupportedTarget reports whether gocpp builds programs for goos/goarch.
func SupportedTarget(goos, goarch string) bool {
	return slices.Contains(buildTargets, goos+"/"+goarch)
}

// IsHost reports whether goos/goarch is the system that runs gocpp.
func IsHost(goos, goarch string) bool {
	return goos == runtime.GOOS && goarch == runtime.GOARCH
}

// ExeSuffix ends the names of the programs of the operating system goos (".exe" for windows).
func ExeSuffix(goos string) string {
	if goos == "windows" {
		return ".exe"
	}
	return ""
}

// SharedSuffix ends the names of shared libraries of goos (Phase 06 step 6.10 uses it).
func SharedSuffix(goos string) string {
	if goos == "windows" {
		return ".dll"
	}
	return ".so"
}

// TargetTriple returns the GNU triple of the target, for clang's --target and for the names of
// cross compilers ("x86_64-w64-mingw32" for windows/amd64): "" for a pair gocpp does not know.
func TargetTriple(goos, goarch string) string {
	switch goos + "/" + goarch {
	case "windows/amd64":
		return "x86_64-w64-mingw32"
	case "windows/arm64":
		return "aarch64-w64-mingw32"
	case "linux/amd64":
		return "x86_64-linux-gnu"
	case "linux/arm64":
		return "aarch64-linux-gnu"
	}
	return ""
}

// winNT is the oldest Windows that a program built by gocpp runs on: Windows 10 (0x0A00).
const winNT = "0x0A00"

// targetFlags are the flags that the target of the compiler c adds to every compile and to the link
// (Flags), after the common ones: none for the host.
func targetFlags(c CXX) []string {
	var flags []string
	// A clang that is not one of the target's own (llvm-mingw's x86_64-w64-mingw32-clang++ and
	// Debian's aarch64-linux-gnu-clang++ name the target themselves) is told the target.
	if c.Family == Clang && c.GOOS != "" && !targetNamed(c) {
		if triple := TargetTriple(c.GOOS, c.GOARCH); triple != "" {
			flags = append(flags, "--target="+triple)
		}
	}
	if c.GOOS == "windows" {
		// The API level of <windows.h>: without it MinGW-w64 declares the Windows 7 API
		// (the OS layer needs GetCurrentThreadStackLimits, Windows 8).
		flags = append(flags, "-D_WIN32_WINNT="+winNT)
	}
	return flags
}

// targetNamed reports whether the compiler's program name starts with its target
// (x86_64-w64-mingw32-clang++, aarch64-linux-gnu-clang++): such a clang knows its target and, for
// MinGW-w64, its headers. A version suffix (clang++-21) is not a target.
func targetNamed(c CXX) bool {
	arch, _, ok := strings.Cut(filepath.Base(c.Path), "-")
	switch arch {
	case "x86_64", "i686", "i386", "aarch64", "arm64", "armv7", "arm", "riscv64", "amd64":
		return ok
	}
	return false
}

// windowsStack is the size of the main thread's stack of a Windows program, the same as the
// default of a goroutine's and of Linux's main thread (8 MiB): a PE file has a size of its own,
// 1 MiB for the MinGW linkers, which a recursion that is fine on Linux would overflow.
const windowsStack = 8 << 20

// LinkFlags returns the flags that the link of a program for the target of c needs, after the
// common ones: "-static" for Windows, so that the .exe runs without the compiler's DLLs
// (libstdc++-6.dll, libgcc_s_seh-1.dll, the thread library; with only -static-libstdc++ the thread
// library of MinGW would still be a DLL), the stack size of the main thread, and, for GCC's
// MinGW-w64, -pthread: needed when its thread model is posix (winpthreads), and harmless when it is
// mcf (the dev shell's: std::thread sits on mcfgthread, which -static links in). The runtime's own
// threads are CreateThread (runtime/src/os/windows), not these libraries.
//
// The stack size is given twice on purpose: here, and by the runtime's .drectve linker directive
// (ADR-053); both say 8 MiB, and the directive survives --gc-sections (checked by running the
// Wine programs of tests/testdata/windows with these flags: "overflow" and "large").
func LinkFlags(c CXX) []string {
	switch c.GOOS {
	case "windows":
		flags := []string{"-static", "-Wl,--stack," + strconv.Itoa(windowsStack)}
		if c.Family == GCC {
			flags = append(flags, "-pthread")
		}
		return flags
	}
	return nil
}
