// This file embeds the vendored source of bdwgc, the garbage collector of the GC engine (phase 16
// README §2.4), in the gocpp binary, next to the C++ runtime. The files are in
// third_party/bdwgc/ (see its README.gocpp); internal/build extracts them into the cache and
// compiles them for a toolchain and sanitizer, and nothing links the result unless a program asks
// for it (build.Builder.Bdwgc).
//
// The file lives here, in package rtfiles, and not in third_party/: a Go package is one folder,
// so a file in runtime/third_party/ would be a different package. The //go:embed pattern is
// relative to this folder.
package rtfiles

import (
	"embed"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"
)

// A directory pattern embeds every file below it, except the files whose names start with "."
// or "_". The vendored bdwgc tree has no such name (BdwgcSources and the test of the tree check
// it), so nothing the build needs is left out. The pattern also embeds LICENSE, README.md,
// README.gocpp and VERSION, which the extraction writes next to the sources.
//
//go:embed third_party/bdwgc
var bdwgcFiles embed.FS

// bdwgcRoot is the bdwgc folder as a file system: its root holds src/, include/, LICENSE,
// README.md, README.gocpp and VERSION.
var bdwgcRoot = sync.OnceValue(func() fs.FS {
	sub, err := fs.Sub(bdwgcFiles, "third_party/bdwgc")
	if err != nil {
		panic("rtfiles: the embedded bdwgc: " + err.Error()) // the embed pattern names the folder
	}
	return sub
})

// BdwgcFS returns the vendored bdwgc source, a read-only file system whose root is
// runtime/third_party/bdwgc. The build reads src/ and include/ from it.
func BdwgcFS() fs.FS { return bdwgcRoot() }

// bdwgcLinuxSources lists the .c files of the vendored bdwgc for Linux, sorted, as paths in
// BdwgcFS: the build list of bdwgc's CMakeLists.txt for Linux with threads, parallel marking,
// thread-local allocation, gcj support and the disclaim interface. It walks src/ without its
// subfolders, so the Windows source (src/windows/win32_threads.c) is not among them.
var bdwgcLinuxSources = sync.OnceValue(func() []string {
	entries, err := fs.ReadDir(BdwgcFS(), "src")
	if err != nil {
		panic("rtfiles: the embedded bdwgc has no src folder: " + err.Error())
	}
	var srcs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".c") {
			srcs = append(srcs, path.Join("src", e.Name()))
		}
	}
	slices.Sort(srcs)
	return srcs
})

// bdwgcPosixThreadSources are the sources that only the POSIX thread layer has: a Windows build
// replaces them with src/windows/win32_threads.c. gc_dlopen.c and specific.c stay: bdwgc's CMake
// Win32 branch with pthreads found (CMAKE_USE_PTHREADS_INIT, which is what FindThreads reports
// for MinGW-w64 and llvm-mingw: both ship winpthreads) adds them (CMakeLists.txt). The
// Win32-threads-only branch (CMAKE_USE_WIN32_THREADS_INIT, a compiler without pthreads, such as
// MSVC or an mcf MinGW) has 23 files and adds EMPTY_GETENV_RESULTS instead; gocpp's Windows
// compilers are the pthreads ones, and the driver links -pthread.
var bdwgcPosixThreadSources = []string{
	"src/pthread_start.c",
	"src/pthread_stop_world.c",
	"src/pthread_support.c",
}

// BdwgcSources returns the paths in BdwgcFS of the C files that make bdwgc for goos, sorted.
// "linux" is the POSIX-threads build (27 files). "windows" is bdwgc's CMake Win32 branch with
// pthreads (25 files): the Linux list without the three POSIX thread sources, plus
// src/windows/win32_threads.c, with gc_dlopen.c and specific.c kept. That is the branch a CMake
// configure takes with MinGW-w64 and llvm-mingw, which ship winpthreads (determined on the PC's
// WinLibs MinGW-w64; internal/build has the defines). internal/build builds both lists. Any
// other goos gets the Linux list; the build refuses it.
func BdwgcSources(goos string) []string {
	srcs := slices.Clone(bdwgcLinuxSources())
	if goos != "windows" {
		return srcs
	}
	srcs = slices.DeleteFunc(srcs, func(p string) bool { return slices.Contains(bdwgcPosixThreadSources, p) })
	srcs = append(srcs, "src/windows/win32_threads.c")
	slices.Sort(srcs)
	return srcs
}

// BdwgcVersion returns the version of the vendored bdwgc, the text of its VERSION file without
// space around it.
func BdwgcVersion() string {
	data, err := fs.ReadFile(BdwgcFS(), "VERSION")
	if err != nil {
		panic("rtfiles: the embedded bdwgc has no VERSION file: " + err.Error())
	}
	return strings.TrimSpace(string(data))
}
