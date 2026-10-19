package bindgen

import (
	"bufio"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// A macro is a #define of the headers in scope.
type macro struct {
	Name     string
	Body     string // the replacement text, white space trimmed
	Params   string // "(a, b)" of a function-like macro, else ""
	FuncLike bool
	File     string // the file that defines it
	Seq      int    // the order of definition
}

// A ppResult is what the preprocessor says about the header.
type ppResult struct {
	// Header is the file of the header itself (as clang names it).
	Header string
	// Files are the files in scope, in the order the preprocessor entered
	// them, and Order their index.
	Files []string
	Order map[string]int
	// Macros are the macros defined by the files in scope and still defined
	// at the end, by order of definition.
	Macros []*macro
}

// A Scope says which of the files that a header includes are part of its
// view.
type Scope string

const (
	// ScopeAuto is ScopeDir, except that a header which lies directly in a
	// folder of the compiler's include path (<sqlite3.h> in /usr/include) is
	// taken alone: its neighbours there are the system's headers.
	ScopeAuto Scope = "auto"
	// ScopeHeader is the header file alone.
	ScopeHeader Scope = "header"
	// ScopeDir is the header and the headers in its folder that it includes,
	// directly or through each other (an umbrella header such as SDL.h).
	ScopeDir Scope = "dir"
	// ScopeTree is ScopeDir with the folders below it (bits/, SDL2/x/).
	ScopeTree Scope = "tree"
)

// parsePP reads the output of "clang -E -dD" of a file that includes only the
// header. It follows the line markers (# 12 "file" 1) to know which file each
// #define is in and which files the header includes.
func parsePP(r io.Reader, mainFile string, scope Scope, dir string, roots []string) (*ppResult, error) {
	res := &ppResult{Order: map[string]int{}}
	br := bufio.NewReaderSize(r, 1<<20)
	type frame struct {
		file    string
		inScope bool
	}
	var stack []frame
	macros := map[string]*macro{}
	seq := 0
	main := clean(mainFile, dir)
	var hdrDir string
	hdrRoot := false
	inScope := func(file, parent string, parentIn bool) bool {
		switch {
		case res.Header == "":
			return false
		case file == res.Header:
			return true
		case parentIn && sameHeader(file, parent):
			return true
		case !parentIn || scope == ScopeHeader:
			return false
		case isBits(file):
			return true
		case scope == ScopeDir:
			return filepath.Dir(file) == hdrDir
		case scope == ScopeAuto:
			return !hdrRoot && filepath.Dir(file) == hdrDir
		default: // ScopeTree
			return file == hdrDir || strings.HasPrefix(file, hdrDir+string(filepath.Separator))
		}
	}
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "# "):
				file, flags, ok := parseMarker(line)
				if !ok {
					break
				}
				file = clean(file, dir)
				switch {
				case len(stack) == 0:
					stack = append(stack, frame{file, false})
				case flags[1]:
					parent := stack[len(stack)-1]
					f := frame{file: file}
					if res.Header == "" && parent.file == main && !strings.HasPrefix(file, "<") {
						res.Header = file
						hdrDir = filepath.Dir(file)
						hdrRoot = slices.Contains(roots, hdrDir)
					}
					f.inScope = inScope(file, parent.file, parent.inScope)
					if f.inScope {
						if _, seen := res.Order[file]; !seen {
							res.Order[file] = len(res.Files)
							res.Files = append(res.Files, file)
						}
					}
					stack = append(stack, f)
				case flags[2]:
					for len(stack) > 1 && stack[len(stack)-1].file != file {
						stack = stack[:len(stack)-1]
					}
					if len(stack) > 1 && stack[len(stack)-1].file == file {
						// returned into file: it is the top again
					}
				default:
					// a line number change inside the same file
					if len(stack) > 0 {
						stack[len(stack)-1].file = file
					}
				}
			case strings.HasPrefix(line, "#define "):
				if len(stack) == 0 || !stack[len(stack)-1].inScope {
					break
				}
				m := parseDefine(line[len("#define "):])
				if m == nil {
					break
				}
				m.File = stack[len(stack)-1].file
				seq++
				m.Seq = seq
				macros[m.Name] = m
			case strings.HasPrefix(line, "#undef "):
				delete(macros, strings.TrimSpace(line[len("#undef "):]))
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	for _, m := range macros {
		res.Macros = append(res.Macros, m)
	}
	sortMacros(res.Macros)
	return res, nil
}

// sameHeader says that file is the header that parent stands for: the
// header a wrapper includes with #include_next (libstdc++'s stdlib.h and
// math.h are wrappers, and the C library's file is what declares the
// functions), or the C header of a C++ wrapper (cstdio includes stdio.h).
func sameHeader(file, parent string) bool {
	f, p := filepath.Base(file), filepath.Base(parent)
	switch {
	case f == p:
		return true
	case len(p) > 1 && p[0] == 'c' && !strings.Contains(p, ".") && f == p[1:]+".h":
		return true // cstdio -> stdio.h
	}
	return len(f) > 1 && f[0] == 'c' && !strings.Contains(f, ".") && p == f[1:]+".h" // stdio.h -> cstdio
}

// partDirs are the folders of the headers that are parts of other headers, by
// the convention of the C and C++ libraries and of the Linux headers:
// <bits/mathcalls.h> is where <math.h> declares its functions, and
// <asm-generic/errno-base.h> where <errno.h> gets its numbers.
var partDirs = []string{"bits", "asm", "asm-generic", "linux"}

// isBits says that a file is a part of the header that includes it.
func isBits(file string) bool {
	for _, d := range partDirs {
		if strings.Contains(file, string(filepath.Separator)+d+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func sortMacros(ms []*macro) {
	// insertion by Seq keeps this independent of map order
	for i := 1; i < len(ms); i++ {
		for j := i; j > 0 && ms[j].Seq < ms[j-1].Seq; j-- {
			ms[j], ms[j-1] = ms[j-1], ms[j]
		}
	}
}

// clean returns the absolute, cleaned path of a file named by clang
// (relative to dir), or the name of a pseudo file such as <built-in>.
func clean(file, dir string) string {
	if strings.HasPrefix(file, "<") {
		return file
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(dir, file)
	}
	return filepath.Clean(file)
}

// parseMarker reads a line marker: # 12 "file" 1 3. flags[n] is set for each
// flag n that is present.
func parseMarker(line string) (file string, flags [5]bool, ok bool) {
	rest := strings.TrimPrefix(line, "# ")
	sp := strings.IndexByte(rest, ' ')
	if sp < 0 {
		return "", flags, false
	}
	if _, err := strconv.Atoi(rest[:sp]); err != nil {
		return "", flags, false
	}
	rest = rest[sp+1:]
	if !strings.HasPrefix(rest, `"`) {
		return "", flags, false
	}
	end := 1
	for end < len(rest) && rest[end] != '"' {
		if rest[end] == '\\' {
			end++
		}
		end++
	}
	if end >= len(rest) {
		return "", flags, false
	}
	q, err := strconv.Unquote(rest[:end+1])
	if err != nil {
		q = rest[1:end]
	}
	for _, f := range strings.Fields(rest[end+1:]) {
		if n, err := strconv.Atoi(f); err == nil && n >= 1 && n <= 4 {
			flags[n] = true
		}
	}
	return q, flags, true
}

// parseDefine reads what follows "#define ": NAME body, or NAME(a, b) body.
func parseDefine(s string) *macro {
	i := 0
	for i < len(s) && isIdent(s[i]) {
		i++
	}
	if i == 0 {
		return nil
	}
	m := &macro{Name: s[:i]}
	if i < len(s) && s[i] == '(' {
		end := strings.IndexByte(s, ')')
		if end < 0 {
			return nil
		}
		m.FuncLike = true
		m.Params = s[i : end+1]
		i = end + 1
	}
	m.Body = strings.TrimSpace(s[i:])
	return m
}

func isIdent(c byte) bool {
	return c >= 0x80 || // a byte of a UTF-8 identifier
		c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
