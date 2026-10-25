// Package rtfiles embeds gocpp's C++ runtime, the files in include/ and src/ of this folder,
// in the gocpp binary (ADR-009). It lives in runtime/ because a //go:embed pattern can't
// reach a parent folder. The driver extracts the files once per version and compiles the
// generated C++ with -I<dir>/include, together with the runtime's .cpp files (Sources).
//
// The tests and README.md stay out of the binary: they are for people who work on gocpp.
package rtfiles

import (
	"embed"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// A directory pattern embeds every file below it, except the files whose names start with
// "." or "_". TestFSHasEveryFile finds a file whose name starts with "_"; the names that start
// with "." are editor and tool files (.#rt.cpp, .cache/), which stay out on purpose.
//
//go:embed include src
var files embed.FS

// debugFiles are the debugger scripts that gocpp debug hands to LLDB, and to gdb when the safe
// path of auto-loading can't name the program (gdb's script is in every program,
// src/os/linux/debug_scripts.cpp). The files stay out of FS: they are not C++.
//
//go:embed debug/gocpp_lldb.py debug/gocpp-gdb.py
var debugFiles embed.FS

// GDBScript returns the text of runtime/debug/gocpp-gdb.py, the script that every program
// carries for gdb.
func GDBScript() []byte {
	data, err := debugFiles.ReadFile("debug/gocpp-gdb.py")
	if err != nil {
		panic(err) // the embed pattern names the file
	}
	return data
}

// LLDBScript returns the text of runtime/debug/gocpp_lldb.py, the summaries of the runtime's
// values for LLDB.
func LLDBScript() []byte {
	data, err := debugFiles.ReadFile("debug/gocpp_lldb.py")
	if err != nil {
		panic(err) // the embed pattern names the file
	}
	return data
}

// FS returns the runtime: a read-only file system whose root is this folder. It holds
// include/gocpp/*.hpp (the headers; generated code includes gocpp/rt.hpp), src/*.cpp (the
// portable .cpp files) and src/os/<os>/*.cpp (the OS layer of each operating system).
// Nothing else is in it.
func FS() fs.FS {
	return files
}

// Sources returns the paths in FS of the .cpp files that a program for the operating system
// goos (a GOOS value: "linux", "windows") is compiled with: src/*.cpp and src/os/<goos>/*.cpp,
// sorted. For an OS that has no OS layer, it returns only the portable files. The files of
// package testing (their names start with "testing") are not among them: only a program that
// imports testing or testing/internal/testdeps is linked with them (TestingSources), because they replace the global
// operator new (to count allocations) and hold the regular expression translator.
func Sources(goos string) []string {
	return sources(goos, false)
}

// TestingSources returns the paths in FS of the .cpp files of package testing (src/testing*.cpp and
// src/os/<goos>/testing*.cpp), sorted: the natives of stdlib/testing and of testing/internal/testdeps
// (gocpp/testing.hpp). The driver compiles and links them into the programs that import one of
// the two, and into no other. (With one archive for the runtime, the linker takes these objects only when
// something refers to them; the natives of the testing package do.)
func TestingSources(goos string) []string {
	return sources(goos, true)
}

func sources(goos string, testing bool) []string {
	var srcs []string
	for _, dir := range []string{"src", path.Join("src", "os", goos)} {
		entries, err := fs.ReadDir(files, dir)
		if err != nil {
			continue // no OS layer for goos
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".cpp") && strings.HasPrefix(e.Name(), "testing") == testing {
				srcs = append(srcs, path.Join(dir, e.Name()))
			}
		}
	}
	slices.Sort(srcs)
	return srcs
}
