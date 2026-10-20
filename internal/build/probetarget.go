package build

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// The flags of a probe for a target (Phase 14 step 14.6, ADR-116). The probes of the C and C++
// headers (the kind probe, the macro probe, the typed probes of Phase 12, gocpp doc) ask a compiler
// about the headers that the program is built with, so they must run with the target's flags like the
// build does: for windows/amd64 the MinGW-w64 headers, the target's data model (LLP64) and the API
// level of <windows.h>. FindCXXFor gives the compiler of the target; ProbeTargetFlags gives the rest.

// ProbeTargetFlags returns the flags that a probe run by cxx needs for the target of cxx, after the
// language flags and before the package's flags (which can override them): none for the host's
// compiler. For a target they are targetFlags (the same ones as the build: --target= for a clang that
// does not name its target, and -D_WIN32_WINNT for Windows), and, for a clang that is not
// MinGW-w64's own on a system other than Windows, the headers of the MinGW-w64 g++ that the build
// uses (MinGWIncludeFlags).
func ProbeTargetFlags(cxx CXX) []string {
	if cxx.GOOS == "" {
		return nil
	}
	flags := targetFlags(cxx)
	if cxx.GOOS == "windows" && cxx.Family == Clang && !targetNamed(cxx) && runtime.GOOS != "windows" {
		// clang expands __FILE__ with the target's separators (\nix\store\...) for a Windows target,
		// and a MinGW-w64 header that includes itself by __FILE__ (mcfgthread's) then is not found.
		flags = append(flags, "-fno-file-reproducible")
		flags = append(flags, MinGWIncludeFlags()...)
	}
	return flags
}

// mingwGXX is the MinGW-w64 g++ that the include folders of a clang for Windows come from: the
// compiler that FindCXXFor finds for windows/amd64 on a Linux host.
const mingwGXX = "x86_64-w64-mingw32-g++"

// mingwOnce caches the answer of MinGWIncludeFlags for the process; the g++ and its folders do not
// change during a run.
var (
	mingwMu    sync.Mutex
	mingwCache = map[string][]string{} // by the g++ program
)

// MinGWIncludeFlags returns the flags that make a clang for the target x86_64-w64-mingw32 read the
// headers of MinGW-w64's g++ (the Windows headers, winpthreads and the C++ library) instead of the
// host's: -nostdlibinc, -nostdinc++, and an -isystem for each folder of the g++'s include path, in its
// order, without the folders of GCC's own intrinsics (lib/gcc/<triple>/<version>/include and
// include-fixed), which clang replaces with its own. It returns nil when there is no such g++ (clang
// then finds its own MinGW headers, if it has them) or it does not answer. The g++ is asked once.
func MinGWIncludeFlags() []string {
	gxx, err := exec.LookPath(mingwGXX)
	if err != nil {
		return nil
	}
	mingwMu.Lock()
	defer mingwMu.Unlock()
	if flags, ok := mingwCache[gxx]; ok {
		return flags
	}
	flags := mingwIncludeFlags(gxx)
	mingwCache[gxx] = flags
	return flags
}

func mingwIncludeFlags(gxx string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gxx, "-E", "-v", "-x", "c++", "-")
	cmd.Env = CompilerEnv()
	cmd.Stdin = strings.NewReader("")
	cmd.WaitDelay = 2 * time.Second
	out, _ := cmd.CombinedOutput() // the folders are on standard error; the exit status is 0 for an empty file
	dirs := mingwSearchDirs(string(out))
	if len(dirs) == 0 {
		return nil
	}
	flags := []string{"-nostdlibinc", "-nostdinc++"}
	for _, d := range dirs {
		flags = append(flags, "-isystem", d)
	}
	return flags
}

// mingwSearchDirs reads the folders of the <...> include path from the output of g++ -E -v, without the
// folders of GCC's own headers (see MinGWIncludeFlags).
func mingwSearchDirs(out string) []string {
	var dirs []string
	in := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "#include <...> search starts here:"):
			in = true
		case strings.HasPrefix(line, "End of search list."):
			in = false
		case in && strings.HasPrefix(line, " "):
			d := strings.TrimSpace(line)
			if gccOwnInclude(d) {
				continue
			}
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// gccOwnInclude reports whether d is the folder of GCC's own headers: .../lib/gcc/<triple>/<version>/include
// or include-fixed (the intrinsics, which use builtins that only GCC has).
func gccOwnInclude(d string) bool {
	d = filepath.ToSlash(d)
	base := d[strings.LastIndex(d, "/")+1:]
	return (base == "include" || base == "include-fixed") && strings.Contains(d, "/lib/gcc/")
}
