// Package diag will collect and print diagnostics the Go way:
// "path/file.gp:12:5: message", sorted by position, without duplicates, and
// at most 10 per package unless -e is given. It will also map C++ compiler
// errors back to .gp lines and gocpp names.
//
// Phase 01 fills it (step 1.13). The C++ error mapping grows in Phase 02
// (a C++ compile failure is an internal error), Phase 04 (interop errors) and
// Phase 13 (C++ errors in gocpp terms).
package diag
