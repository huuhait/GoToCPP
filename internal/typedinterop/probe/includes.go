package probe

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// The includes of a preamble, as the preprocessor saw them. `clang -E -dI` keeps every include
// directive in its output, with the name as it was written after the macros are expanded (a continued line,
// a comment, a digraph, `#include NAME` all come out as `#include "name"`), and the line markers say
// which file each directive is in. That is what the cache needs to know where a header that is created
// later would be found first: the folder of the file that has a quoted include, with the name as it was
// spelled (`../foo.h` is not `foo.h`), and the places where an include that was not found would be.

// A directive is one include directive that the preprocessor met.
type directive struct {
	includer string // the file that has it (absolute); "" for the probe file and the command line
	quoted   bool   // "name", not <name>
	name     string // as spelled
	entered  bool   // a file was entered (not found, or skipped by a guard, when false)
}

// includeInfo is what a run of the preprocessor told about the includes of a text.
type includeInfo struct {
	directives []directive
	entered    []string // the files that were entered (cleaned, absolute), in order
	complete   bool     // false when the output may be only a part: the answer is then to be completed by guessing
}

// maxDirectiveLine is the longest line of the preprocessed output that is kept when it is a directive or a
// marker (the others are dropped whatever their length).
const maxDirectiveLine = 1 << 20

// A directiveScanner reads the output of `clang -E -dI` as it is written and keeps the directives and
// the files entered; it keeps no other line.
type directiveScanner struct {
	dir, src string // the folder the compiler ran in, and the probe file
	info     includeInfo
	buf      []byte
	keep     bool // the line being read starts with '#'
	started  bool // the line being read has a first byte
	overlong bool // a directive or marker line was longer than maxDirectiveLine
	pending  bool // bytes were written since the last newline
	cur      string
	waiting  bool // the last directive has not seen the marker of its file yet
	seen     map[string]bool
}

func newDirectiveScanner(dir, src string) *directiveScanner {
	return &directiveScanner{dir: dir, src: src, cur: src, seen: make(map[string]bool)}
}

func (s *directiveScanner) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		chunk := p
		if i >= 0 {
			chunk = p[:i]
		}
		if len(chunk) > 0 {
			s.pending = true
			if !s.started {
				s.started = true
				s.keep = chunk[0] == '#'
			}
			if s.keep {
				if len(s.buf)+len(chunk) > maxDirectiveLine {
					s.overlong, s.keep = true, false
					s.buf = s.buf[:0]
				} else {
					s.buf = append(s.buf, chunk...)
				}
			}
		}
		if i < 0 {
			break
		}
		s.pending = false
		if s.keep {
			s.line(string(bytes.TrimRight(s.buf, "\r")))
		}
		s.buf, s.started, s.keep = s.buf[:0], false, false
		p = p[i+1:]
	}
	return n, nil
}

// unterminated reports whether the output ended in the middle of a line (a compiler that was stopped).
func (s *directiveScanner) unterminated() bool { return s.pending }

// abs makes the name of a file of a marker absolute, without cleaning it (see rawpath.go).
func (s *directiveScanner) abs(p string) string {
	if !filepath.IsAbs(p) {
		p = joinRaw(s.dir, p)
	}
	return p
}

func (s *directiveScanner) line(l string) {
	if strings.HasPrefix(l, "# ") {
		s.marker(l[2:])
		return
	}
	for _, kw := range []string{"#include_next ", "#include ", "#import "} {
		if !strings.HasPrefix(l, kw) {
			continue
		}
		rest := strings.TrimLeft(l[len(kw):], " ")
		if rest == "" || (rest[0] != '"' && rest[0] != '<') {
			return
		}
		end := byte('"')
		if rest[0] == '<' {
			end = '>'
		}
		j := strings.IndexByte(rest[1:], end)
		if j <= 0 {
			return
		}
		d := directive{quoted: rest[0] == '"', name: rest[1 : 1+j]}
		if s.cur != s.src && !strings.HasPrefix(s.cur, "<") {
			d.includer = s.abs(s.cur)
		}
		s.info.directives = append(s.info.directives, d)
		s.waiting = true
		return
	}
}

// marker reads a line marker `N "file" flags`: the file is the one the next lines belong to; flag 1 says it
// was entered by the directive before.
func (s *directiveScanner) marker(l string) {
	i := strings.IndexByte(l, '"')
	if i < 0 {
		return
	}
	var file []byte
	j := i + 1
	for ; j < len(l) && l[j] != '"'; j++ {
		c := l[j]
		if c == '\\' && j+1 < len(l) {
			if j+3 < len(l) && l[j+1] >= '0' && l[j+1] <= '7' && l[j+2] >= '0' && l[j+2] <= '7' && l[j+3] >= '0' && l[j+3] <= '7' {
				v, _ := strconv.ParseUint(l[j+1:j+4], 8, 8)
				file = append(file, byte(v))
				j += 3
				continue
			}
			j++
			c = l[j]
		}
		file = append(file, c)
	}
	if j >= len(l) {
		return
	}
	name := string(file)
	s.cur = name
	if !slices.Contains(strings.Fields(l[j+1:]), "1") {
		return
	}
	if s.waiting && len(s.info.directives) > 0 {
		s.info.directives[len(s.info.directives)-1].entered = true
	}
	s.waiting = false
	if name == s.src || strings.HasPrefix(name, "<") || name == "" {
		return
	}
	if p := s.abs(name); !s.seen[p] {
		s.seen[p] = true
		s.info.entered = append(s.info.entered, p)
	}
}

// includeInfo returns what the preprocessor says about the includes of the text. read are the headers the
// compiler's own run read; it is asked for when it read some or a header was not found. The answer is kept
// by the Runner for as long as the headers it was made of have the same contents (a declaration run and each
// batch of units of one preamble ask the same question).
func (rn *Runner) includeInfo(ctx context.Context, cfg Config, text string, read []string, missing bool, errKey string) *includeInfo {
	if len(read) == 0 && !missing {
		return &includeInfo{complete: true}
	}
	kcfg := cfg
	kcfg.MaxDecl, kcfg.MaxJSON = 0, 0
	// (the title line of the generated file does not matter: the runs of one package share the answer)
	keyText := text
	if strings.HasPrefix(keyText, "//") {
		if _, rest, ok := strings.Cut(keyText, "\n"); ok {
			keyText = rest
		}
	}
	key := PreKey(kcfg, &Source{Text: keyText}) + "\x00" + DepsHash(read) + "\x00" + errKey
	rn.mu.Lock()
	info, ok := rn.infoMemo[key]
	rn.mu.Unlock()
	if ok {
		return info
	}
	var sc *directiveScanner
	out, err := runAuxTo(ctx, cfg, []string{"-E", "-dI", "-o", "-"}, text, func(src string) io.Writer {
		sc = newDirectiveScanner(cfg.Dir, src)
		return sc
	})
	if err != nil || sc == nil {
		return &includeInfo{} // (not kept: the run was stopped)
	}
	info = &sc.info
	info.complete = !out.timedOut && !sc.overlong && !sc.unterminated() &&
		(len(info.entered) > 0 || len(read) == 0) && (len(info.directives) > 0 || !missing)
	if info.complete {
		rn.mu.Lock()
		if rn.infoMemo == nil {
			rn.infoMemo = make(map[string]*includeInfo)
		}
		rn.infoMemo[key] = info
		rn.mu.Unlock()
	}
	return info
}

// includerShadows adds, for every include directive of a file that is not one of the compiler's own, the
// places where a header of that name would be found before the one that was: for a quoted include the folder of
// the file that has it (a quoted include looks there first; for the probe file and the command line that is
// the package folder), and for every include the folders of the user's flags and environment, the name as it
// was spelled, `..` and all. The includes of the compiler's own headers are left out (nobody adds a header
// there; the folders before the one that has them are watched from the paths of the headers, earlierShadows).
func includerShadows(directives []directive, cfgDir string, order, user []string) []string {
	sys := systemDirs(order, user)
	inSys := func(p string) bool {
		best, bestSys := -1, false
		for _, d := range user {
			if _, ok := underDir(d, p); ok && len(d) > best {
				best, bestSys = len(d), false
			}
		}
		for _, d := range sys {
			if _, ok := underDir(d, p); ok && len(d) > best {
				best, bestSys = len(d), true
			}
		}
		return bestSys
	}
	var add []string
	seen := make(map[string]bool)
	put := func(dir, name string) {
		if p := joinRaw(dir, name); !seen[p] {
			seen[p] = true
			add = append(add, p)
		}
	}
	for _, d := range directives {
		if filepath.IsAbs(d.name) || d.name == "" {
			continue
		}
		dir := cfgDir
		if d.includer != "" {
			if inSys(d.includer) {
				continue
			}
			dir = dirRaw(d.includer)
		}
		if d.quoted {
			put(dir, d.name)
		}
		for _, u := range user {
			put(u, d.name)
		}
	}
	return add
}
