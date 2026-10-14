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

// FS returns the runtime: a read-only file system whose root is this folder. It holds
// include/gocpp/*.hpp (the headers; generated code includes gocpp/rt.hpp), src/*.cpp (the
// portable .cpp files) and src/os/<os>/*.cpp (the OS layer of each operating system).
// Nothing else is in it.
func FS() fs.FS {
	return files
}

// Sources returns the paths in FS of the .cpp files that a program for the operating system
// goos (a GOOS value: "linux", "windows") is compiled with: src/*.cpp and src/os/<goos>/*.cpp,
// sorted. For an OS that has no OS layer, it returns only the portable files.
func Sources(goos string) []string {
	var srcs []string
	for _, dir := range []string{"src", path.Join("src", "os", goos)} {
		entries, err := fs.ReadDir(files, dir)
		if err != nil {
			continue // no OS layer for goos
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".cpp") {
				srcs = append(srcs, path.Join(dir, e.Name()))
			}
		}
	}
	slices.Sort(srcs)
	return srcs
}
