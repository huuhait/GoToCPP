package main

import (
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

// Patch files (tools/portstd/patches/<import path with / as __>.rules).
//
// A patch is a search-and-replace rule on the text of Go's source file, as it
// is upstream: the rules run before anything else, so the author never sees
// the tool's own changes. Every rule needs a reason, and fails the port when
// its old text is not found exactly as often as it says (once, by default):
// a new Go version that changes the text breaks the rule loudly, and nobody
// has to wonder why a hunk "applied with fuzz".
//
//	# a comment (a line that starts with #, outside a rule)
//	@note   A known divergence from Go, for the PORT.md of the package.
//	@skip   unsafe_tricks.go   reason (the upstream file is not taken)
//	@asm    stub.s   reason (an assembly file that the build constraints keep and a patch replaces)
//	@file   wrap.go
//	@why    The reason for the rules that follow, up to the next @why.
//	@replace [count=N]
//	old text, any number of lines (must match exactly, including tabs)
//	@with
//	new text, any number of lines (may be empty)
//	@end
//
// @file names the upstream file (with .go) for the rules until the next @file.

// A Rule is one @replace.
type Rule struct {
	File  string
	Why   string
	Old   string
	New   string
	Count int // how often Old must occur (default 1)
	Line  int // in the rules file, for messages
}

// Rules are the parsed rules file of a package.
type Rules struct {
	Notes []string
	Skips map[string]string // file -> reason
	Asm   map[string]string // assembly file -> reason: the build constraints keep it, a patch writes its work in Go
	Rules []Rule
}

// parseRules reads a rules file.
func parseRules(name, text string) (*Rules, error) {
	r := &Rules{Skips: make(map[string]string), Asm: make(map[string]string)}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	var file, why string
	errorf := func(n int, format string, args ...any) error {
		return fmt.Errorf("%s:%d: %s", name, n+1, fmt.Sprintf(format, args...))
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		word, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		switch word {
		case "@note":
			if rest == "" {
				return nil, errorf(i, "@note needs text")
			}
			r.Notes = append(r.Notes, rest)
		case "@skip":
			f, reason, _ := strings.Cut(rest, " ")
			reason = strings.TrimSpace(reason)
			if f == "" || reason == "" {
				return nil, errorf(i, "usage: @skip file.go reason")
			}
			r.Skips[f] = reason
		case "@asm":
			f, reason, _ := strings.Cut(rest, " ")
			reason = strings.TrimSpace(reason)
			if f == "" || reason == "" || !strings.HasSuffix(f, ".s") {
				return nil, errorf(i, "usage: @asm file.s reason")
			}
			r.Asm[f] = reason
		case "@file":
			if rest == "" || strings.ContainsAny(rest, " /") {
				return nil, errorf(i, "usage: @file name.go")
			}
			file, why = rest, ""
		case "@why":
			if rest == "" {
				return nil, errorf(i, "@why needs a reason")
			}
			why = rest
		case "@replace":
			rule := Rule{File: file, Why: why, Count: 1, Line: i + 1}
			if file == "" {
				return nil, errorf(i, "@replace before @file")
			}
			if why == "" {
				return nil, errorf(i, "@replace without a @why: every patch needs a reason")
			}
			if rest != "" {
				k, v, ok := strings.Cut(rest, "=")
				n, err := strconv.Atoi(v)
				if !ok || k != "count" || err != nil || n < 1 {
					return nil, errorf(i, "usage: @replace [count=N]")
				}
				rule.Count = n
			}
			var old, repl []string
			cur := &old
			closed := false
			for i++; i < len(lines); i++ {
				switch lines[i] {
				case "@with":
					if cur == &repl {
						return nil, errorf(i, "second @with")
					}
					cur = &repl
					continue
				case "@end":
					closed = true
				}
				if closed {
					break
				}
				*cur = append(*cur, lines[i])
			}
			if !closed || cur != &repl {
				return nil, errorf(rule.Line-1, "@replace needs @with and @end")
			}
			rule.Old = strings.Join(old, "\n")
			rule.New = strings.Join(repl, "\n")
			if rule.Old == "" {
				return nil, errorf(rule.Line-1, "the old text is empty")
			}
			r.Rules = append(r.Rules, rule)
		default:
			return nil, errorf(i, "unknown line %q (want @note, @skip, @asm, @file, @why or @replace)", line)
		}
	}
	return r, nil
}

// loadRules returns the rules of a package, or none.
func (c *Config) loadRules(pkg string) (*Rules, error) {
	if c.Patches == nil {
		return &Rules{Skips: map[string]string{}}, nil
	}
	name := patchFile(pkg)
	b, err := fs.ReadFile(c.Patches, name)
	if err != nil {
		return &Rules{Skips: map[string]string{}}, nil
	}
	return parseRules("patches/"+name, string(b))
}

// An AppliedPatch is what PORT.md says about a rule.
type AppliedPatch struct {
	File  string
	Why   string
	Count int
}

// apply runs the rules of one file over its text. It reports which rules
// ran; a rule that does not match as often as it says is an error.
func (r *Rules) apply(file string, src string) (string, []AppliedPatch, error) {
	var applied []AppliedPatch
	for _, rule := range r.Rules {
		if rule.File != file {
			continue
		}
		if n := strings.Count(src, rule.Old); n != rule.Count {
			return "", nil, fmt.Errorf("patch for %s (rules line %d, %q): the old text occurs %d times, the rule says %d:\n%s",
				file, rule.Line, rule.Why, n, rule.Count, rule.Old)
		}
		src = strings.ReplaceAll(src, rule.Old, rule.New)
		applied = append(applied, AppliedPatch{File: file, Why: rule.Why, Count: rule.Count})
	}
	return src, applied, nil
}

// checkFiles reports a rule or @skip that names a file that is not in the
// list of the package's upstream files, so a renamed file is not silently
// unpatched.
func (r *Rules) checkFiles(upstream []string) error {
	var bad []string
	for _, rule := range r.Rules {
		if !slices.Contains(upstream, rule.File) {
			bad = append(bad, fmt.Sprintf("%s (rules line %d)", rule.File, rule.Line))
		}
	}
	for f := range r.Skips {
		if !slices.Contains(upstream, f) {
			bad = append(bad, f+" (@skip)")
		}
	}
	if len(bad) > 0 {
		slices.Sort(bad)
		return fmt.Errorf("the patches name files that the port does not take: %s", strings.Join(bad, ", "))
	}
	return nil
}
