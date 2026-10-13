// Package diag collects the diagnostics (errors and warnings) of a package
// and prints them the way the go command prints compile errors:
//
//	./main.gp:12:5: undefined: count
//
// A List holds the diagnostics of one package. All steps of the front-end
// add to it: parse errors, import and directive errors, go/types errors
// and phase-gate errors. go/types errors come in through List.Collect, the
// Error function of types.Config. The other errors come in through
// List.Errorf and List.Add.
//
// A Printer prints a List like go build: sorted by position, without
// duplicates, with file names relative to the current folder, and at most
// 10 errors, then a "too many errors" line. Printer.All (the -e flag)
// prints all of them. Printer.Source also shows the source line, with a
// caret (^) under the column. Printer.Print has the details, and the two
// small differences from go build: Print shows a column of 255 or more,
// which go build leaves out, and it can show a file outside the package
// folder by another name. List.Diagnostics returns the same diagnostics
// as values, for the language server and for tests.
//
// Later phases add the mapping of C++ compiler errors back to .gp lines and
// gocpp names: Phase 02 (a C++ compile failure is an internal error),
// Phase 04 (interop errors) and Phase 13 (C++ errors in gocpp terms).
package diag
