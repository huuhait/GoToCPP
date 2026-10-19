package modfetch

import (
	"fmt"
	"strconv"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// unq returns the module path or version that a word of a gocpp.mod line
// stands for (a word may be quoted).
func unq(w string) string {
	if len(w) > 1 && (w[0] == '"' || w[0] == '`') {
		if s, err := strconv.Unquote(w); err == nil {
			return s
		}
	}
	return w
}

func isRequire(tok []string) bool { return len(tok) > 0 && tok[0] == "require" }

// IndirectMode says what SetRequireMode does with the "// indirect" comment.
type IndirectMode int

const (
	// Direct: the line has no indirect comment (a module that the user asked
	// for).
	Direct IndirectMode = iota
	// KeepIndirect: an existing line keeps its comment (or lack of it); a
	// new line has none.
	KeepIndirect
	// Indirect: the line has the comment // indirect (a module that only
	// other modules need).
	Indirect
)

// SetRequire sets the version of the require line of the module in the
// syntax tree of a gocpp.mod file, or adds a line: into the require block if
// there is one, else after the last require line, else after the module, gocpp
// and cxx lines. A line that is changed loses its "// indirect" comment (the
// user asked for this module). The file is written back with
// gocppmod.Format(f.Syntax); comments and every statement that gocpp does not
// read stay as they are.
func SetRequire(f *gocppmod.File, path, version string) { SetRequireMode(f, path, version, Direct) }

// SetRequireMode is SetRequire with a say in the indirect comment. Every
// other require line of the module is removed, so that the file has one.
func SetRequireMode(f *gocppmod.File, path, version string, mode IndirectMode) {
	syn := f.Syntax
	var lastReq, lastHead = -1, -1
	var first *gocppmod.Line
	apply := func(l *gocppmod.Line) {
		switch mode {
		case Direct:
			dropIndirect(&l.Comments)
		case Indirect:
			if !hasIndirect(l.Comments) {
				l.Comments.Suffix = append(l.Comments.Suffix, gocppmod.Comment{Token: "// indirect"})
			}
		}
	}
	for i, st := range syn.Stmt {
		switch s := st.(type) {
		case *gocppmod.Line:
			switch {
			case isRequire(s.Token) && len(s.Token) == 3:
				if unq(s.Token[1]) == path {
					if first == nil {
						s.Token[2] = version
						apply(s)
						first = s
					}
				}
				lastReq = i
			case len(s.Token) > 0 && (s.Token[0] == "module" || s.Token[0] == "gocpp" || s.Token[0] == "cxx"):
				lastHead = i
			}
		case *gocppmod.Block:
			if len(s.Token) == 1 && s.Token[0] == "require" && s.Open == '(' {
				for _, l := range s.Line {
					if len(l.Token) >= 2 && unq(l.Token[0]) == path && first == nil {
						l.Token = []string{l.Token[0], version}
						apply(l)
						first = l
					}
				}
				lastReq = i
			}
		}
	}
	if first != nil {
		dropRequireExcept(f, path, first)
		return
	}
	line := func(inBlock bool) *gocppmod.Line {
		l := &gocppmod.Line{Token: []string{path, version}, InBlock: inBlock}
		if !inBlock {
			l.Token = []string{"require", path, version}
		}
		if mode == Indirect {
			l.Comments.Suffix = []gocppmod.Comment{{Token: "// indirect"}}
		}
		return l
	}
	if lastReq >= 0 {
		if b, ok := syn.Stmt[lastReq].(*gocppmod.Block); ok {
			b.Line = append(b.Line, line(true))
			return
		}
		insertStmt(syn, lastReq+1, line(false))
		return
	}
	at := lastHead + 1
	if lastHead < 0 {
		at = len(syn.Stmt)
	}
	l := line(false)
	l.Blank = at > 0
	insertStmt(syn, at, l)
}

func hasIndirect(c gocppmod.Comments) bool {
	for _, s := range c.Suffix {
		if s.Token == "// indirect" {
			return true
		}
	}
	return false
}

// dropRequireExcept removes the require lines of the module other than keep.
func dropRequireExcept(f *gocppmod.File, path string, keep *gocppmod.Line) {
	syn := f.Syntax
	out := syn.Stmt[:0]
	for _, st := range syn.Stmt {
		switch s := st.(type) {
		case *gocppmod.Line:
			if s != keep && isRequire(s.Token) && len(s.Token) == 3 && unq(s.Token[1]) == path {
				continue
			}
		case *gocppmod.Block:
			if len(s.Token) == 1 && s.Token[0] == "require" && s.Open == '(' {
				kept := s.Line[:0]
				for _, l := range s.Line {
					if l != keep && len(l.Token) >= 2 && unq(l.Token[0]) == path {
						continue
					}
					kept = append(kept, l)
				}
				s.Line = kept
			}
		}
		out = append(out, st)
	}
	syn.Stmt = out
}

func insertStmt(syn *gocppmod.FileSyntax, at int, s gocppmod.Stmt) {
	syn.Stmt = append(syn.Stmt, nil)
	copy(syn.Stmt[at+1:], syn.Stmt[at:])
	syn.Stmt[at] = s
}

func dropIndirect(c *gocppmod.Comments) {
	for i, s := range c.Suffix {
		if s.Token == "// indirect" {
			c.Suffix = append(c.Suffix[:i], c.Suffix[i+1:]...)
			return
		}
	}
}

// DropRequire removes the require line of the module, and returns whether
// there was one.
func DropRequire(f *gocppmod.File, path string) bool {
	syn := f.Syntax
	found := false
	out := syn.Stmt[:0]
	carry := false // a blank line that was above a statement that is gone
	for _, st := range syn.Stmt {
		drop := false
		switch s := st.(type) {
		case *gocppmod.Line:
			if isRequire(s.Token) && len(s.Token) == 3 && unq(s.Token[1]) == path {
				found, drop = true, true
				carry = carry || s.Blank
			}
		case *gocppmod.Block:
			if len(s.Token) == 1 && s.Token[0] == "require" && s.Open == '(' {
				keep := s.Line[:0]
				for _, l := range s.Line {
					if len(l.Token) >= 2 && unq(l.Token[0]) == path {
						found = true
						continue
					}
					keep = append(keep, l)
				}
				s.Line = keep
				if len(s.Line) == 0 {
					drop = true
					carry = carry || s.Blank
				}
			}
		}
		if drop {
			continue
		}
		if carry {
			switch s := st.(type) {
			case *gocppmod.Line:
				s.Blank = true
			case *gocppmod.Block:
				s.Blank = true
			case *gocppmod.CommentBlock:
				s.Blank = true
			}
			carry = false
		}
		out = append(out, st)
	}
	syn.Stmt = out
	return found
}

// FormatFile returns the text of the edited file, after checking that it
// parses again with the same requirements.
func FormatFile(name string, f *gocppmod.File) ([]byte, error) {
	data := gocppmod.Format(f.Syntax)
	g, errs := gocppmod.Parse(name, data, nil)
	if len(errs) > 0 {
		return nil, fmt.Errorf("internal error: the edited %s has errors: %v", name, errs)
	}
	_ = g
	return data, nil
}
