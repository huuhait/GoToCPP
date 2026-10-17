// Package gocppmod reads and writes gocpp.mod files, the module files of
// gocpp (docs/spec/modules.md, ADR-020).
//
// A gocpp.mod file has the syntax of a go.mod file: one statement per line,
// a verb followed by arguments, comments that start with //, and factored
// blocks
//
//	require (
//		example.com/a v1.2.3
//		example.com/b v0.1.0 // indirect
//	)
//
// plus a few gocpp lines (gocpp, cxx, cflags, ldflags, pkg-config). The
// package is modeled on golang.org/x/mod/modfile in lax mode (the module is
// not a dependency of gocpp yet, and a version that fits the module cache of
// the build machines was not available offline), with these differences:
//
//   - It never stops at the first error: Parse reports every statement
//     that it can't read, and returns the file with the statements that it
//     could. gocpp check then shows all of them at once, and the module
//     line still gives the module path when another line is broken.
//   - A statement whose verb it doesn't know is kept, and Format writes it
//     back as it was (a verb is lower-case letters, digits and "-": anything
//     else, such as a module path at the start of a line, is an error, and so
//     is a keyword of gocpp.mod in a wrong shape, such as require( ). The
//     body of a brace block, such as the C++ library recipes of Phase 06
//     README §2.11 (cxx PATH { ... }), is not read at all: its lines are kept
//     verbatim up to the matching "}", so they are never taken for
//     module-wide statements and may hold anything.
//   - The lexer splits words at white space only; a quote (", ' or `) in a
//     word runs to the matching quote, also in the middle of the word.
//     Parentheses, commas, brackets and braces inside a word stay in it, so
//     cflags -DX=(1,2) is one flag. A block is opened by a "(" or "{" that
//     ends its line, and closed by a line with only ")" or "}".
//
// Errors read like those of the go command: file:line: message, with the
// column after the line when the statement is not at the start of its line
// (inside a block).
package gocppmod

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// A Position is a place in a file.
type Position struct {
	Line     int // line in the file, from 1
	LineRune int // rune in the line, from 1
	Byte     int // byte in the file, from 0
}

// A Comment is one // comment, with the //.
type Comment struct {
	Start Position
	Token string // without the newline
}

// Comments are the comments of a statement.
type Comments struct {
	// Before are the whole-line comments right above the statement (no
	// blank line between). In a block, a blank line is a Comment with an
	// empty Token, so Format can keep it.
	Before []Comment

	// Suffix is the comment at the end of the statement's line, if any.
	Suffix []Comment
}

// A Stmt is a statement of the file: a Line, a Block or a CommentBlock.
type Stmt interface {
	// Span returns the first position of the statement.
	Span() Position
	// Comment returns the comments of the statement.
	Comment() *Comments
}

// A FileSyntax is a parsed file.
type FileSyntax struct {
	Name string // the file name, for messages
	Stmt []Stmt

	// After are the comments after the last statement.
	After []Comment
}

// A Line is one statement on one line: the verb is Token[0], and the
// arguments follow. In a block (Line.InBlock), Token has no verb: the block
// has it.
type Line struct {
	Comments
	Start   Position
	Token   []string // the words as written: a quoted word keeps its quotes
	InBlock bool

	// Blank is true if a blank line is above the statement, before its
	// comments. Format keeps one.
	Blank bool
}

func (l *Line) Span() Position     { return l.Start }
func (l *Line) Comment() *Comments { return &l.Comments }

// A Block is a factored statement
//
//	verb arg... (
//		line
//		line
//	)
//
// or, with Open '{', the same with braces.
type Block struct {
	Comments
	Start Position
	Token []string // the words before the opening bracket: the verb and its arguments
	Open  byte     // '(' or '{'
	Line  []*Line
	Blank bool

	// Raw are the lines of the body of a brace block, as they are in the file.
	// They are not read: a block with { holds the recipe of a C++ library,
	// which is not gocpp.mod syntax. Line is empty then.
	Raw []string

	// Unterminated is true if the file ended before the closing bracket.
	Unterminated bool

	// Close is the position of the closing bracket; CloseBefore are the
	// comments above it, and CloseSuffix the comment after it.
	Close       Position
	CloseBefore []Comment
	CloseSuffix []Comment
}

func (b *Block) Span() Position     { return b.Start }
func (b *Block) Comment() *Comments { return &b.Comments }

// A CommentBlock is a group of whole-line comments that is not above a
// statement: a blank line follows it.
type CommentBlock struct {
	Comments
	Start Position
	Blank bool
}

func (c *CommentBlock) Span() Position     { return c.Start }
func (c *CommentBlock) Comment() *Comments { return &c.Comments }

// An Error is a mistake in a file.
type Error struct {
	Filename string
	Pos      Position
	Err      string
}

// Error returns "file:line: message", or "file:line:column: message" if the
// column is more than 1 (as the go command does).
func (e Error) Error() string {
	if e.Pos.LineRune <= 1 {
		return fmt.Sprintf("%s:%d: %s", e.Filename, e.Pos.Line, e.Err)
	}
	return fmt.Sprintf("%s:%d:%d: %s", e.Filename, e.Pos.Line, e.Pos.LineRune, e.Err)
}

// An ErrorList is the errors of a file, in the order of their lines.
type ErrorList []Error

// Error returns the errors, one in each line.
func (l ErrorList) Error() string {
	var b strings.Builder
	for i, e := range l {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(e.Error())
	}
	return b.String()
}

// runeCol returns the column, counted in runes from 1, of the byte offset
// off in line.
func runeCol(line string, off int) int {
	return utf8.RuneCountInString(line[:off]) + 1
}
