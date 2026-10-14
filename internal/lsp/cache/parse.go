package cache

import (
	"bytes"
	"context"
	"go/ast"
	"go/token"

	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// The per-file parse cache (Phase 11 README §2.2 item 3): a parsed file is
// kept by the hash of its content, its name and the parse mode, so the
// features (and the headers of the metadata) parse an edited file once, and
// a file that is the same in two snapshots is parsed once. (The expand step
// of Phase 10, macros and !, joins the parse here.)
//
// A deviation from "typing in one file re-parses only that file" (README
// §2.2, ADR-015), recorded in the phase STATE: the check of a package
// parses all of its files again, because check.Check changes the ASTs it
// checks (about 8% of the CPU of an edit). A parse hook in internal/load
// with AST cloning would remove it (an option for step 11.11).

// ParseFull is the parse mode of the files that the features look at: with
// comments and all the syntax errors, as gocpp check parses them.
const ParseFull = parser.ParseComments | parser.AllErrors

// A ParsedFile is a file as the parser left it, shared by every snapshot in
// which the file has this content: it must never be changed.
type ParsedFile struct {
	Path string      // the absolute name of the file
	Hash Hash        // the hash of the file's content
	Mode parser.Mode // the parse mode
	// Src is the content of the file. With parser.ImportsOnly, it is
	// only its header (see headerEnd), a copy: the cache keeps no more.
	Src []byte

	// Fset holds only this file, Tok.
	Fset *token.FileSet
	Tok  *token.File

	// File is the AST. With a syntax error it is partial, and Err says
	// what is wrong (a scanner.ErrorList).
	File *ast.File
	Err  error
}

// A parseKey is the key of a parsed file in the cache.
type parseKey struct {
	path string
	hash Hash
	mode parser.Mode
}

// parse returns the parsed file f in the mode mode, from the cache or
// parsed now.
func (p *Pipeline) parse(ctx context.Context, f *fileHandle, mode parser.Mode) (*ParsedFile, error) {
	if f.err != nil {
		return nil, f.err
	}
	return p.parses.Get(ctx, parseKey{f.path, f.hash, mode}, func(context.Context) (*ParsedFile, error) {
		p.stats.parses.Add(1)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, f.path, f.src, mode)
		pf := &ParsedFile{
			Path: f.path, Hash: f.hash, Mode: mode, Src: f.src,
			Fset: fset, Tok: fset.File(file.FileStart), File: file, Err: err,
		}
		if mode&parser.ImportsOnly != 0 {
			pf.Src = bytes.Clone(f.src[:headerEnd(pf)])
		}
		return pf, nil
	})
}

// headerEnd returns the offset in the source of pf, which was parsed with
// parser.ImportsOnly, where its header ends: the build constraints, the
// package clause and the imports, which are all that decide the packages
// it imports (see metadata). It is the length of the source when the
// package clause is missing.
func headerEnd(pf *ParsedFile) int {
	f := pf.File
	end := f.Name.End()
	if len(f.Decls) > 0 {
		end = f.Decls[len(f.Decls)-1].End()
	}
	if f.Name.Name == "" || !end.IsValid() {
		return len(pf.Src)
	}
	return min(pf.Tok.Offset(end), len(pf.Src))
}
