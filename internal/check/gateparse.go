package check

import (
	"cmp"
	"errors"
	"go/token"
	"os"
	"regexp"
	"slices"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// The gates of a package that was not type-checked: its load errors are
// all there is. Two of them get the gate's words:
//
//   - the syntax of Phase 10 (F!(...), func f!(), macro, #Name(...)) does
//     not parse before Phase 10 (parser patches 4 to 6). In a file that
//     has it, the syntax errors from its first use on are the parser's
//     trouble with it (the parser gets lost after it, also on later
//     lines), so the gate errors take the place of all of them. A syntax
//     error before the first use stays: it is a real one. A real syntax
//     error after it is hidden until the Phase 10 syntax is gone, or
//     Phase 10 is there.
//   - the loader rejects every standard package without a stub with
//     "package X is not available yet (Phase 09)"; for a package of
//     stdPackages of another phase, the message names that phase (weak:
//     Phase 03, plugin: never).
//
// The errors keep the order they were found in (diag.List.Rewrite): the
// gate errors of a file take the place of its first replaced syntax error.

// gateUnchecked runs the gates of the unchecked user package p.
func gateUnchecked(fset *token.FileSet, p *Package, phase int) {
	// A fileGate holds the gate errors of one file with Phase 10 syntax.
	type fileGate struct {
		first  token.Position // the first use of Phase 10 syntax
		syntax map[parseError]bool
		errs   []diag.Diagnostic
		placed bool
	}
	files := make(map[string]*fileGate)
	var order []*fileGate
	if phase < features[featMacros].Phase {
		for _, f := range p.Load.Files {
			tf := fset.File(f.AST.FileStart)
			if tf == nil {
				continue
			}
			// load.File doesn't keep the source, so read it again; the
			// file just failed to parse, so this is rare.
			// TODO(Phase 11): read the language server's overlay of
			// unsaved files here, like the loader does then.
			src, err := os.ReadFile(f.Path)
			if err != nil {
				continue
			}
			uses := phase10Syntax(src)
			if len(uses) == 0 {
				continue
			}
			fg := &fileGate{syntax: make(map[parseError]bool)}
			for _, e := range parseErrors(f.Path, src) {
				fg.syntax[e] = true
			}
			if len(fg.syntax) == 0 {
				continue
			}
			first := uses[0].offset
			for _, u := range uses {
				first = min(first, u.offset)
				pos := tf.Pos(u.offset)
				d := diag.Diagnostic{Pos: tf.Position(pos), Msg: features[u.id].Message(), Gate: true}
				if u.length > 0 {
					d.End = tf.Position(pos + token.Pos(u.length))
				}
				fg.errs = append(fg.errs, d)
			}
			slices.SortFunc(fg.errs, func(a, b diag.Diagnostic) int { return cmp.Compare(a.Pos.Offset, b.Pos.Offset) })
			fg.first = tf.Position(tf.Pos(first))
			files[f.Path] = fg
			order = append(order, fg)
		}
	}
	rename := func(d *diag.Diagnostic) bool {
		m := notAvailable.FindStringSubmatch(d.Msg)
		if m == nil {
			return false
		}
		row := lookupStdPackage(m[1])
		if row == nil || row.Phase <= phase || row.message(m[1]) == d.Msg {
			// At its phase, a package without a stub is still missing:
			// the loader's message stays.
			return false
		}
		d.Msg = row.message(m[1])
		return true
	}
	renames := slices.ContainsFunc(p.Errors.Diagnostics(), func(d diag.Diagnostic) bool { return rename(&d) })
	if len(order) == 0 && !renames {
		return
	}
	p.Errors = p.Errors.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
		fg := files[d.Pos.Filename]
		if fg != nil && fg.syntax[parseError{d.Pos.Filename, d.Pos.Line, d.Pos.Column, d.Msg}] && !before(d.Pos, fg.first) {
			if fg.placed {
				return nil
			}
			fg.placed = true
			return fg.errs
		}
		rename(&d)
		return []diag.Diagnostic{d}
	})
	for _, fg := range order {
		if !fg.placed {
			for _, d := range fg.errs {
				p.Errors.Add(d)
			}
		}
	}
}

// before reports whether a comes before b in the same file.
func before(a, b token.Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Column < b.Column
}

// notAvailable matches the loader's error for a standard package without a
// stub.
var notAvailable = regexp.MustCompile(`^package (\S+) is not available yet \(Phase 09\)$`)

// A parseError is a syntax error: its place and message.
type parseError struct {
	file      string
	line, col int
	msg       string
}

// parseErrors returns the syntax errors of the file, all of them (the
// loader keeps fewer without load.Config.AllErrors, but the same ones).
func parseErrors(path string, src []byte) []parseError {
	_, err := parser.ParseFile(token.NewFileSet(), path, src, parser.ParseComments|parser.AllErrors)
	var list scanner.ErrorList
	if !errors.As(err, &list) {
		return nil
	}
	errs := make([]parseError, len(list))
	for i, e := range list {
		errs[i] = parseError{e.Pos.Filename, e.Pos.Line, e.Pos.Column, e.Msg}
	}
	return errs
}

// A syntaxUse is a use of Phase 10 syntax in a source file: the feature,
// and its place as a byte offset and a length.
type syntaxUse struct {
	id             featureID
	offset, length int
}

// phase10Syntax finds the first use of each kind of Phase 10 syntax in the
// source (Phase 10 README §2.1 and §2.4, parser patches 4 to 6):
//
//   - a must function: func name! or func (recv) name!;
//   - a must call: a name right before ! (no space), then ( or [: F!(x),
//     os.ReadFile!(p), x.Close!(), parse![int](s). In Go a ! never follows
//     a name, so this is never Go code. (A ! after ], as in F[int]!(x), is
//     not Phase 10 syntax: the type arguments come after the !);
//   - a macro: # right before a name or a keyword (#Assert, #body,
//     #func), which is not a Go token at all, or macro Name at package
//     level (outside braces).
//
// It reports the place of the ! of a must call or function, the # of a
// macro use and the word macro of a declaration.
func phase10Syntax(src []byte) []syntaxUse {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, src, func(token.Position, string) {}, 0)

	type tok struct {
		pos token.Pos
		tok token.Token
		lit string
	}
	var toks []tok
	for {
		pos, t, lit := s.Scan()
		if t == token.EOF {
			break
		}
		toks = append(toks, tok{pos, t, lit})
	}
	offset := func(p token.Pos) int { return file.Offset(p) }
	end := func(t tok) int {
		if t.lit != "" && t.tok != token.SEMICOLON {
			return offset(t.pos) + len(t.lit)
		}
		return offset(t.pos) + len(t.tok.String())
	}

	first := make(map[featureID]syntaxUse)
	found := func(id featureID, t tok, length int) {
		if _, ok := first[id]; !ok {
			first[id] = syntaxUse{id: id, offset: offset(t.pos), length: length}
		}
	}

	braces := 0
	// funcName is the index of the name in the header of a func declared
	// at package level, while the header is scanned, or -1.
	funcState, recvDepth, parens := 0, 0, 0
	funcName := -1
	for i, t := range toks {
		switch t.tok {
		case token.LBRACE:
			braces++
		case token.RBRACE:
			braces--
		case token.LPAREN:
			parens++
		case token.RPAREN:
			parens--
		}

		// The name of a func declared at package level, after an optional
		// receiver: func name or func (r T) name.
		switch funcState {
		case 0:
			if t.tok == token.FUNC && braces == 0 && parens == 0 {
				funcState = 1
			}
		case 1: // after func
			switch t.tok {
			case token.LPAREN:
				funcState, recvDepth = 2, parens
			case token.IDENT:
				funcState, funcName = 3, i
			default:
				funcState = 0
			}
		case 2: // in the receiver
			if t.tok == token.RPAREN && parens == recvDepth-1 {
				funcState = 4
			}
		case 4: // after the receiver
			if t.tok == token.IDENT {
				funcState, funcName = 3, i
			} else {
				funcState = 0
			}
		case 3: // after the name
			funcState = 0
		}

		switch {
		case t.tok == token.NOT && i > 0 && i+1 < len(toks) &&
			toks[i-1].tok == token.IDENT &&
			end(toks[i-1]) == offset(t.pos) &&
			(toks[i+1].tok == token.LPAREN || toks[i+1].tok == token.LBRACK):
			if funcName == i-1 {
				found(featMustFuncs, t, 1)
			} else {
				found(featMustCalls, t, 1)
			}
		case t.tok == token.ILLEGAL && t.lit == "#" && i+1 < len(toks) &&
			(toks[i+1].tok == token.IDENT || toks[i+1].tok.IsKeyword()) &&
			offset(toks[i+1].pos) == offset(t.pos)+1:
			found(featMacros, t, 1+len(toks[i+1].lit))
		case t.tok == token.IDENT && t.lit == "macro" && braces == 0 && parens == 0 &&
			(i == 0 || toks[i-1].tok == token.SEMICOLON) &&
			i+1 < len(toks) && toks[i+1].tok == token.IDENT:
			found(featMacros, t, len("macro"))
		}
		if funcName != i && funcState != 3 {
			funcName = -1
		}
	}

	var uses []syntaxUse
	for _, id := range []featureID{featMustCalls, featMustFuncs, featMacros} {
		if u, ok := first[id]; ok {
			uses = append(uses, u)
		}
	}
	return uses
}
