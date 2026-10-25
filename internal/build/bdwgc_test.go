package build

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// The unit tests of the bdwgc build's configuration and cache key. The build itself (a real
// compile of the vendored sources, and the smoke program that links the library) is tested in
// runtime/bdwgc_test.go and runtime/cxx_test.go.

// TestBdwgcDefines checks the fixed defines of phase 16 README §2.4 and bdwgc's own Linux ones.
func TestBdwgcDefines(t *testing.T) {
	defs := bdwgcDefines("linux")
	for _, want := range []string{
		"-DGC_THREADS", "-DPARALLEL_MARK", "-DTHREAD_LOCAL_ALLOC", "-DGC_BUILTIN_ATOMIC",
		"-DLARGE_CONFIG", "-DUSE_MMAP", "-DUSE_MUNMAP", "-DALL_INTERIOR_POINTERS",
		"-DGC_DISABLE_INCREMENTAL", // gocpp owns SIGSEGV; the write-fault handler must not exist
		"-DNO_EXECUTE_PERMISSION", "-D_REENTRANT", "-DGC_NO_THREAD_REDIRECTS",
		"-DHAVE_DL_ITERATE_PHDR", "-DHAVE_DLADDR", "-DHAVE_PTHREAD_SETNAME_NP_WITH_TID",
	} {
		if !slices.Contains(defs, want) {
			t.Errorf("bdwgcDefines(\"linux\") has no %s: %q", want, defs)
		}
	}
	// §2.4 keeps bdwgc's extra byte at the end of an object, and no GC_ASSERTIONS.
	//
	// MPROTECT_VDB and SOFT_VDB are not checked: gcconfig.h defines them itself on x86_64 Linux,
	// so their presence in the -D list says nothing. GC_DISABLE_INCREMENTAL is what compiles the
	// incremental mode (and its SIGSEGV/SIGBUS handler) out.
	for _, unwanted := range []string{"-DDONT_ADD_BYTE_AT_END", "-DGC_ASSERTIONS"} {
		if slices.Contains(defs, unwanted) {
			t.Errorf("bdwgcDefines(\"linux\") has %s, which §2.4 turns off", unwanted)
		}
	}
	// The flags are hashed in order (bdwgcKey): keep the Linux list exactly as it was before the
	// Windows port, so that no Linux library is rebuilt for a reordering.
	wantLinux := []string{
		"-DGC_THREADS", "-DPARALLEL_MARK", "-DTHREAD_LOCAL_ALLOC", "-DGC_BUILTIN_ATOMIC",
		"-DLARGE_CONFIG", "-DUSE_MMAP", "-DUSE_MUNMAP", "-DALL_INTERIOR_POINTERS",
		"-DGC_DISABLE_INCREMENTAL", "-DNO_EXECUTE_PERMISSION", "-D_REENTRANT",
		"-DGC_GCJ_SUPPORT", "-DGC_ENABLE_SUSPEND_THREAD", "-DENABLE_DISCLAIM",
		"-DJAVA_FINALIZATION", "-DGC_ATOMIC_UNCOLLECTABLE", "-DHANDLE_FORK",
		"-DGC_NO_THREAD_REDIRECTS", "-DHAVE_DL_ITERATE_PHDR", "-DHAVE_DLADDR",
		"-DHAVE_PTHREAD_SETNAME_NP_WITH_TID",
	}
	if !slices.Equal(defs, wantLinux) {
		t.Errorf("bdwgcDefines(\"linux\") = %q, want the list and order %q", defs, wantLinux)
	}
	// The Windows build is CMake's pthreads branch on WIN32, as a CMake configure of the vendored
	// tree found it with MinGW-w64: the common defines, _REENTRANT, and the probes that the
	// toolchain has or lacks.
	// Exact: one _REENTRANT (the common list's), no duplicate of any define.
	wantWindows := []string{
		"-DGC_THREADS", "-DPARALLEL_MARK", "-DTHREAD_LOCAL_ALLOC", "-DGC_BUILTIN_ATOMIC",
		"-DLARGE_CONFIG", "-DUSE_MMAP", "-DUSE_MUNMAP", "-DALL_INTERIOR_POINTERS",
		"-DGC_DISABLE_INCREMENTAL", "-DNO_EXECUTE_PERMISSION", "-D_REENTRANT",
		"-DGC_GCJ_SUPPORT", "-DGC_ENABLE_SUSPEND_THREAD", "-DENABLE_DISCLAIM",
		"-DJAVA_FINALIZATION", "-DGC_ATOMIC_UNCOLLECTABLE", "-DGC_NO_THREAD_REDIRECTS",
		// The WIN32 and static-library options, and the probes of the PC's MinGW-w64.
		"-D_CRT_SECURE_NO_DEPRECATE", "-DGC_NOT_DLL", "-DDONT_USE_USER32_DLL",
		"-DGC_NO_SIGSETJMP", "-DGC_MISSING_EXECINFO_H", "-DNO_GETCONTEXT",
		"-DHAVE_DL_ITERATE_PHDR", "-DHAVE_PTHREAD_SETNAME_NP_WITH_TID",
	}
	win := bdwgcDefines("windows")
	if !slices.Equal(win, wantWindows) {
		t.Errorf("bdwgcDefines(\"windows\") = %q, want the list and order %q", win, wantWindows)
	}
	for _, unwanted := range []string{
		"-DEMPTY_GETENV_RESULTS", // the Win32-threads-only branch
		"-DHANDLE_FORK",          // CMake skips the fork handler on Windows
		"-DHAVE_DLADDR",          // MinGW-w64 has no dladdr
		// CMake found only pthread_setname_np(pthread_t, const char*).
		"-DHAVE_PTHREAD_SETNAME_NP_WITHOUT_TID", "-DHAVE_PTHREAD_SETNAME_NP_WITH_TID_AND_ARG",
		"-DDONT_ADD_BYTE_AT_END", "-DGC_ASSERTIONS",
	} {
		if slices.Contains(win, unwanted) {
			t.Errorf("bdwgcDefines(\"windows\") has %s, which its Win32 build does not", unwanted)
		}
	}
}

// TestBdwgcTargets checks the targets that Bdwgc refuses, and the builder/compiler disagreement:
// all of these fail before a compiler runs.
func TestBdwgcTargets(t *testing.T) {
	for _, tc := range []struct {
		name string
		b    *Builder
		want string
	}{
		{"unsupported system", &Builder{CXX: CXX{Path: "/fake/gcc"}, GOOS: "darwin"},
			"supports linux and windows/amd64"},
		{"windows/arm64", &Builder{CXX: CXX{Path: "/fake/gcc", GOARCH: "arm64"}, GOOS: "windows"},
			"supports windows/amd64, not windows/arm64"},
		{"builder and compiler disagree", &Builder{CXX: CXX{Path: "/fake/gcc", GOOS: "linux"}, GOOS: "windows"},
			"the builder is for windows, but its compiler targets linux"},
	} {
		if _, err := tc.b.Bdwgc(context.Background(), BdwgcConfig{}); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Bdwgc = %v, want an error with %q", tc.name, err, tc.want)
		}
	}
}

// TestBdwgcLinkFlags checks the link flags after the archive: the C library's threads and dynamic
// loading on Linux, nothing on Windows, and the sanitizer's runtime first.
func TestBdwgcLinkFlags(t *testing.T) {
	if got := bdwgcLinkFlags(BdwgcNoSanitizer, "linux"); !slices.Equal(got, []string{"-lpthread", "-ldl"}) {
		t.Errorf("linux: %q", got)
	}
	if got := bdwgcLinkFlags(BdwgcNoSanitizer, "windows"); len(got) != 0 {
		t.Errorf("windows: %q, want no library of its own", got)
	}
	asan := bdwgcLinkFlags(BdwgcASan, "windows")
	if len(asan) != 1 || asan[0] != "-fsanitize=address,undefined" {
		t.Errorf("windows asan: %q", asan)
	}
}

// TestBdwgcFlags checks the sanitizer flags and the order of the caller's flags.
func TestBdwgcFlags(t *testing.T) {
	cc := CXX{Path: "/fake/gcc", Family: GCC}
	b := &Builder{}
	flags := b.bdwgcFlags(cc, "linux", BdwgcConfig{})
	if !slices.Contains(flags, "-std=gnu17") || !slices.Contains(flags, "-O0") {
		t.Errorf("the base flags have no -std=gnu17 -O0: %q", flags)
	}
	if slices.ContainsFunc(flags, func(f string) bool { return strings.HasPrefix(f, "-fsanitize=") }) {
		t.Errorf("a build without a sanitizer has %q", flags)
	}

	asan := b.bdwgcFlags(cc, "linux", BdwgcConfig{Sanitizer: BdwgcASan})
	if !slices.Contains(asan, "-fsanitize=address,undefined") || !slices.Contains(asan, "-fno-sanitize-recover=all") {
		t.Errorf("the asan flags: %q", asan)
	}
	tsan := b.bdwgcFlags(cc, "linux", BdwgcConfig{Sanitizer: BdwgcTSan})
	if !slices.Contains(tsan, "-fsanitize=thread") || !slices.Contains(tsan, "-DTHREAD_SANITIZER") {
		// GCC 15 has __has_feature(thread_sanitizer), which bdwgc's gcconfig.h uses, but older
		// GCC has only __SANITIZE_THREAD__, which it does not look at: the macro must come from
		// the build.
		t.Errorf("the tsan flags: %q", tsan)
	}

	// The user's and the caller's flags come after gocpp's, so a later -O wins.
	b.Env.CFlags = []string{"-O1"}
	withUser := b.bdwgcFlags(cc, "linux", BdwgcConfig{CFlags: []string{"-O2", "-DX=1"}})
	o1 := slices.Index(withUser, "-O1")
	o2 := slices.Index(withUser, "-O2")
	if o1 < 0 || o2 < 0 || o2 < o1 || withUser[len(withUser)-1] != "-Wno-error" {
		t.Errorf("the user's and the caller's flags are not last: %q", withUser)
	}
}

// TestBdwgcSanitizer checks that the builder's own sanitizer wins and that a contradiction is an
// error: a library built without a sanitizer must not be linked into an -asan or -race program.
func TestBdwgcSanitizer(t *testing.T) {
	race := &Builder{Env: EnvFlags{CFlags: []string{"-fsanitize=thread", "-fno-omit-frame-pointer"}}}
	for _, tc := range []struct {
		name string
		b    *Builder
		ask  BdwgcSanitizer
		want BdwgcSanitizer
		ok   bool
	}{
		{"no sanitizer, none asked", &Builder{}, BdwgcNoSanitizer, BdwgcNoSanitizer, true},
		{"no sanitizer, asan asked", &Builder{}, BdwgcASan, BdwgcASan, true},
		{"-asan, none asked", &Builder{ASan: true}, BdwgcNoSanitizer, BdwgcASan, true},
		{"-asan, asan asked", &Builder{ASan: true}, BdwgcASan, BdwgcASan, true},
		{"-asan, tsan asked", &Builder{ASan: true}, BdwgcTSan, 0, false},
		{"-race, none asked", race, BdwgcNoSanitizer, BdwgcTSan, true},
		{"-race, tsan asked", race, BdwgcTSan, BdwgcTSan, true},
		{"-race, asan asked", race, BdwgcASan, 0, false},
	} {
		got, err := tc.b.bdwgcSanitizer(tc.ask)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("%s: bdwgcSanitizer(%s) = %s, %v; want %s", tc.name, tc.ask, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s: bdwgcSanitizer(%s) = %s, want an error", tc.name, tc.ask, got)
		}
	}
}

// TestBdwgcKey checks that the cache key changes with everything the library depends on and only
// then.
func TestBdwgcKey(t *testing.T) {
	a := Archiver{Path: "/fake/ar", Name: "ar"}
	b := &Builder{}
	cc := CXX{Path: "/fake/gcc", Family: GCC, Version: "gcc 15"}
	srcs := []string{"src/alloc.c", "src/mark.c"}
	flags := []string{"-O0", "-DGC_THREADS"}

	base := b.bdwgcKey(cc, "linux", srcs, flags, a)
	if again := b.bdwgcKey(cc, "linux", srcs, flags, a); again != base {
		t.Fatalf("the same inputs have different keys: %s, %s", base, again)
	}
	changes := []struct {
		name string
		key  string
	}{
		{"another compiler version", b.bdwgcKey(CXX{Path: "/fake/gcc", Family: GCC, Version: "gcc 16"}, "linux", srcs, flags, a)},
		{"another compiler family", b.bdwgcKey(CXX{Path: "/fake/clang", Family: Clang, Version: "gcc 15"}, "linux", srcs, flags, a)},
		{"another target", b.bdwgcKey(cc, "windows", srcs, flags, a)},
		{"another flag", b.bdwgcKey(cc, "linux", srcs, append(slices.Clone(flags), "-O2"), a)},
		{"another source", b.bdwgcKey(cc, "linux", append(slices.Clone(srcs), "src/malloc.c"), flags, a)},
		{"another archiver", b.bdwgcKey(cc, "linux", srcs, flags, Archiver{Path: "/fake/llvm-ar", Name: "llvm-ar"})},
	}
	for _, c := range changes {
		if c.key == base {
			t.Errorf("%s: the key did not change (%s)", c.name, base)
		}
	}
	// The arch is part of the key on its own: same target and everything else, only GOARCH
	// differs. (The "another target" case above changes GOOS only.)
	amd := CXX{Path: "/fake/gcc", Family: GCC, Version: "gcc 15", GOARCH: "amd64"}
	arm := amd
	arm.GOARCH = "arm64"
	if b.bdwgcKey(amd, "windows", srcs, flags, a) == b.bdwgcKey(arm, "windows", srcs, flags, a) {
		t.Errorf("GOARCH did not change the key")
	}
	// The sanitizer is part of the flags, so it is part of the key.
	asan := b.bdwgcKey(cc, "linux", srcs, append(slices.Clone(flags), "-fsanitize=address"), a)
	if asan == base {
		t.Errorf("the asan flags did not change the key")
	}
	if len(base) != 16 {
		t.Errorf("the key %q is not 16 hex characters", base)
	}
}
