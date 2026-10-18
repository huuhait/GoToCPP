// Package modfetch downloads modules: the libraries that a gocpp.mod file
// requires and that are neither replaced by a local folder nor vendored.
//
// It works like the Go modules of the go command (docs/spec/modules.md,
// ADR-024), for modules that are plain repositories on a git host, C and C++
// libraries without any gocpp file in them:
//
//   - A module path (github.com/ocornut/imgui) and a version (v1.91.5, or a
//     pseudo-version v0.0.0-20240101120000-abcdef123456 for a commit) name a
//     tagged archive of the repository on its host (source.go).
//   - The archive is downloaded over HTTPS within limits (httpget.go),
//     extracted with checks that reject every way out of the target folder
//     (extract.go), and hashed with Go's "h1:" directory hash (hash.go).
//   - The hash is looked up in, or recorded in, the gocpp.sum file of the main
//     module (sum.go): the first download of a module is trusted, every later
//     one must match (a mismatch is a security error).
//   - The extracted tree is kept, read-only, in the module cache
//     (cache.go), created atomically so that many gocpp processes can
//     download the same module at once.
//   - Vendor writes the vendor folder for offline builds (vendor.go).
//
// Nothing that is downloaded is ever run: the files are data until the user
// builds them, and a symbolic link in an archive is an error.
//
// The package does not import internal/load: the loader calls Dir, and the
// gocpp commands get, mod download, mod verify and mod vendor call the other
// functions.
//
// The functions that read the environment are in config.go. The checksum
// code in hash.go is adapted from golang.org/x/mod/sumdb/dirhash (BSD
// license, Copyright 2018 The Go Authors).
package modfetch
