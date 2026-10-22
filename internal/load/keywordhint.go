package load

import (
	"errors"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
	"go/token"
	"slices"

	"github.com/huuhait/gocpp/internal/diag"
)

// The keyword in front of a dot.
//
// A header's default name is its file name: <sys/select.h> would be select, a Go keyword. Such a
// header has no name (Include.Why is Keyword), and a use of the name, select.FD_SETSIZE, never
// gets as far as the rewrite: the parser stops at the keyword ("expected operand, found
// 'select'"), because only the name after a dot may be a keyword, and then it reports more
// errors that come from that one. So the loader asks for the rename itself, where it knows both:
// each use of the would-be name of a header of the file (the keyword, with a dot after it,
// and not after another dot) gets the header's hint, and the syntax errors that the keyword causes
// are left out:
//
//	./main.gp:6:10: <sys/select.h> has no package name, because select is a Go keyword: give it
//	a name: import cselect "<sys/select.h>"
//
// The syntax errors that remain when the uses are written with another name stay as they are:
// they are the file's own.

// hintKeywordUses replaces the syntax errors that the uses of a keyword as the name of a header
// cause in the files of p by one error with the hint of the header at each use. mode is the mode
// of the parse of the files.
func hintKeywordUses(p *Package, mode parser.Mode) {
	for _, f := range p.Files {
		if f.Info == nil || f.Src == nil {
			continue
		}
		hints := make(map[string]string) // the keyword -> the hint of the first header that would have it
		for _, inc := range f.Info.Includes {
			if _, ok := hints[inc.WouldBe]; !ok && inc.Why == Keyword && inc.Hint != "" {
				hints[inc.WouldBe] = inc.Hint
			}
		}
		if len(hints) == 0 {
			continue
		}
		uses := keywordUses(f.Path, f.Src, hints)
		if len(uses) == 0 {
			continue
		}
		caused, revealed := keywordErrors(f.Path, f.Src, mode, uses)
		if len(caused) == 0 {
			continue // the file parses: the uses are in code that was not parsed
		}
		p.Errors = p.Errors.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
			if d.Pos.Filename == f.Path && caused[syntaxError{d.Pos.Offset, d.Msg}] {
				return nil
			}
			return []diag.Diagnostic{d}
		})
		for _, u := range uses {
			p.Errors.Add(diag.Diagnostic{Pos: u.pos, End: u.end, Msg: hints[u.keyword]})
		}
		for _, e := range revealed {
			p.Errors.Add(diag.Diagnostic{Pos: e.Pos, Msg: e.Msg})
		}
	}
}

// A keywordUse is a use of a keyword as a package name: select in select.FD_SETSIZE.
type keywordUse struct {
	keyword  string
	pos, end token.Position
}

// keywordUses returns the uses in src of the keywords in hints as the name before a dot, in
// source order.
func keywordUses(name string, src []byte, hints map[string]string) []keywordUse {
	fset := token.NewFileSet()
	file := fset.AddFile(name, -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, func(token.Position, string) {}, scanner.ScanComments|scanner.Phase10Syntax)
	var uses []keywordUse
	var prev token.Token
	var pending *keywordUse // a keyword that waits for the dot
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if pending != nil && tok == token.PERIOD {
			uses = append(uses, *pending)
		}
		pending = nil
		if tok.IsKeyword() && prev != token.PERIOD {
			if lit == "" {
				lit = tok.String()
			}
			if _, ok := hints[lit]; ok {
				end := pos + token.Pos(len(lit))
				pending = &keywordUse{keyword: lit, pos: fset.Position(pos), end: fset.Position(end)}
			}
		}
		if tok != token.COMMENT {
			prev = tok
		}
	}
	return uses
}

// A syntaxError is the place and the message of a syntax error.
type syntaxError struct {
	offset int
	msg    string
}

// keywordErrors returns the syntax errors of src that the uses cause: those that the parse of
// src has and the parse of src with the uses written as a name (underscores as long as the
// keyword) does not have. It also returns the errors that only the second parse has: the first
// one lost them in the errors that the keyword caused.
func keywordErrors(name string, src []byte, mode parser.Mode, uses []keywordUse) (caused map[syntaxError]bool, revealed scanner.ErrorList) {
	own := parseErrors(name, src, mode)
	if len(own) == 0 {
		return nil, nil
	}
	renamed := slices.Clone(src)
	for _, u := range uses {
		for i := u.pos.Offset; i < u.end.Offset; i++ {
			renamed[i] = '_'
		}
	}
	remaining := parseErrors(name, renamed, mode)
	caused = make(map[syntaxError]bool)
	for _, e := range own {
		if !slices.ContainsFunc(remaining, func(r *scanner.Error) bool { return sameError(r, e) }) {
			caused[syntaxError{e.Pos.Offset, e.Msg}] = true
		}
	}
	for _, e := range remaining {
		if !slices.ContainsFunc(own, func(o *scanner.Error) bool { return sameError(o, e) }) {
			revealed = append(revealed, e)
		}
	}
	return caused, revealed
}

// sameError reports whether the syntax errors are at the same place and have the same message.
func sameError(a, b *scanner.Error) bool {
	return a.Pos.Offset == b.Pos.Offset && a.Msg == b.Msg
}

// parseErrors returns the syntax errors of parsing src.
func parseErrors(name string, src []byte, mode parser.Mode) scanner.ErrorList {
	_, err := parser.ParseFile(token.NewFileSet(), name, src, mode)
	var list scanner.ErrorList
	if errors.As(err, &list) {
		return list
	}
	return nil
}
