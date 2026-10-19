// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gocpp: this file is cmd/gofmt/internal.go of Go 1.25.14: how gofmt
// parses a file or a fragment, and prints it. The changes:
//   - it is in package format, not package main;
//   - it parses with gocpp's parser (internal/syntax/parser), not go/parser,
//     and prints with gocpp's printer (internal/syntax/printer), not
//     go/printer;
//   - ParserMode is the constant below: the parser mode of gofmt without
//     its flags -e and -r, and with the syntax of Phase 10;
//   - a fragment that is a statement list starting with a macro use, or
//     with the name macro, is accepted (see errMacroOnTop and errMacroName);
//   - this comment replaces the upstream TODO, which says that the file is
//     the same as go/format/internal.go (it is, but for the package name).

package format

import (
	"bytes"
	"go/ast"
	"go/token"
	"strings"

	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/printer"
)

// ParserMode is the mode in which gofmt parses: with the comments, and
// without the deprecated object resolution, which only gofmt -r uses. gocpp
// adds the syntax of Phase 10 (F!(x), func f!(), #Name(...), macro, lazy),
// which is always on in gocpp (ADR-095): the loader and the language server
// parse it the same way.
//
// ParserMode is exported because everything that checks a formatted result
// must parse it in the same mode (the language server's safety nets do);
// it is the one place that says how the tools that only format parse the
// syntax of Phase 10.
const ParserMode = parser.ParseComments | parser.SkipObjectResolution | parser.Phase10Syntax

// errMacroOnTop is the error of the parser for lines of macros that are not
// on top of a func or a type declaration.
const errMacroOnTop = "macro calls must be on top of a func or type declaration"

// errMacroName is the start of the error of the parser for the word macro
// at package level that no name follows.
const errMacroName = "expected macro name"

// parse parses src, which was read from the named file,
// as a Go source file, declaration, or statement list.
func parse(fset *token.FileSet, filename string, src []byte, fragmentOk bool) (
	file *ast.File,
	sourceAdj func(src []byte, indent int) []byte,
	indentAdj int,
	err error,
) {
	// Try as whole source file.
	file, err = parser.ParseFile(fset, filename, src, ParserMode)
	// If there's no error, return. If the error is that the source file didn't begin with a
	// package line and source fragments are ok, fall through to
	// try as a source fragment. Stop and return on any other error.
	if err == nil || !fragmentOk || !strings.Contains(err.Error(), "expected 'package'") {
		return
	}

	// If this is a declaration list, make it a source file
	// by inserting a package clause.
	// Insert using a ';', not a newline, so that the line numbers
	// in psrc match the ones in src.
	psrc := append([]byte("package p;"), src...)
	file, err = parser.ParseFile(fset, filename, psrc, ParserMode)
	if err == nil {
		sourceAdj = func(src []byte, indent int) []byte {
			// Remove the package clause.
			// Gofmt has turned the ';' into a '\n'.
			src = src[indent+len("package p\n"):]
			return bytes.TrimSpace(src)
		}
		return
	}
	// If the error is that the source file didn't begin with a
	// declaration, fall through to try as a statement list.
	// Stop and return on any other error.
	// gocpp: a statement list can start with a macro use (#Name(...)), which
	// is a line of macros on top of nothing as a declaration list (its own
	// error), or with the name macro (macro := 1, macro++), which is the
	// start of a macro declaration without its name.
	if msg := err.Error(); !strings.Contains(msg, "expected declaration") && !strings.Contains(msg, errMacroOnTop) && !strings.Contains(msg, errMacroName) {
		return
	}

	// If this is a statement list, make it a source file
	// by inserting a package clause and turning the list
	// into a function body. This handles expressions too.
	// Insert using a ';', not a newline, so that the line numbers
	// in fsrc match the ones in src. Add an extra '\n' before the '}'
	// to make sure comments are flushed before the '}'.
	fsrc := append(append([]byte("package p; func _() {"), src...), '\n', '\n', '}')
	file, err = parser.ParseFile(fset, filename, fsrc, ParserMode)
	if err == nil {
		sourceAdj = func(src []byte, indent int) []byte {
			// Cap adjusted indent to zero.
			if indent < 0 {
				indent = 0
			}
			// Remove the wrapping.
			// Gofmt has turned the "; " into a "\n\n".
			// There will be two non-blank lines with indent, hence 2*indent.
			src = src[2*indent+len("package p\n\nfunc _() {"):]
			// Remove only the "}\n" suffix: remaining whitespaces will be trimmed anyway
			src = src[:len(src)-len("}\n")]
			return bytes.TrimSpace(src)
		}
		// Gofmt has also indented the function body one level.
		// Adjust that with indentAdj.
		indentAdj = -1
	}

	// Succeeded, or out of options.
	return
}

// format formats the given package file originally obtained from src
// and adjusts the result based on the original source via sourceAdj
// and indentAdj.
func format(
	fset *token.FileSet,
	file *ast.File,
	sourceAdj func(src []byte, indent int) []byte,
	indentAdj int,
	src []byte,
	cfg printer.Config,
) ([]byte, error) {
	if sourceAdj == nil {
		// Complete source file.
		var buf bytes.Buffer
		err := cfg.Fprint(&buf, fset, file)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	// Partial source file.
	// Determine and prepend leading space.
	i, j := 0, 0
	for j < len(src) && isSpace(src[j]) {
		if src[j] == '\n' {
			i = j + 1 // byte offset of last line in leading space
		}
		j++
	}
	var res []byte
	res = append(res, src[:i]...)

	// Determine and prepend indentation of first code line.
	// Spaces are ignored unless there are no tabs,
	// in which case spaces count as one tab.
	indent := 0
	hasSpace := false
	for _, b := range src[i:j] {
		switch b {
		case ' ':
			hasSpace = true
		case '\t':
			indent++
		}
	}
	if indent == 0 && hasSpace {
		indent = 1
	}
	for i := 0; i < indent; i++ {
		res = append(res, '\t')
	}

	// Format the source.
	// Write it without any leading and trailing space.
	cfg.Indent = indent + indentAdj
	var buf bytes.Buffer
	err := cfg.Fprint(&buf, fset, file)
	if err != nil {
		return nil, err
	}
	out := sourceAdj(buf.Bytes(), cfg.Indent)

	// If the adjusted output is empty, the source
	// was empty but (possibly) for white space.
	// The result is the incoming source.
	if len(out) == 0 {
		return src, nil
	}

	// Otherwise, append output to leading space.
	res = append(res, out...)

	// Determine and append trailing space.
	i = len(src)
	for i > 0 && isSpace(src[i-1]) {
		i--
	}
	return append(res, src[i:]...), nil
}

// isSpace reports whether the byte is a space character.
// isSpace defines a space as being among the following bytes: ' ', '\t', '\n' and '\r'.
func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
