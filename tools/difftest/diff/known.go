package diff

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Entry is a difference between gocpp and Go that is known and understood: a documented
// divergence (docs/spec/language.md) or a bug that is already filed.
type Entry struct {
	Name   string
	Kind   string // a Kind, or "*"
	Sig    *regexp.Regexp
	Src    *regexp.Regexp // optional: also matches the reduced program
	Reason string
	Doc    string
}

// Known is the list of known differences (tools/difftest/known.txt).
type Known struct {
	Entries []*Entry
}

// ParseKnown reads known.txt: blocks of "key: value" lines separated by blank lines; a line
// that starts with # is a comment. The keys are name, kind, sig (a regexp matched against the
// signature of the mismatch: its first differing line pair, or the first error line), src
// (optional regexp matched against the reduced program), reason and doc.
func ParseKnown(r io.Reader) (*Known, error) {
	k := &Known{}
	var cur *Entry
	flush := func() error {
		if cur == nil {
			return nil
		}
		if cur.Name == "" || cur.Sig == nil || cur.Reason == "" || cur.Doc == "" {
			return fmt.Errorf("known entry %q needs name, sig, reason and doc", cur.Name)
		}
		if cur.Kind == "" {
			cur.Kind = "*"
		}
		k.Entries = append(k.Entries, cur)
		cur = nil
		return nil
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	ln := 0
	for sc.Scan() {
		ln++
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("known.txt:%d: want key: value", ln)
		}
		val = strings.TrimSpace(val)
		if cur == nil {
			cur = &Entry{}
		}
		var err error
		switch strings.TrimSpace(key) {
		case "name":
			cur.Name = val
		case "kind":
			cur.Kind = val
		case "sig":
			cur.Sig, err = regexp.Compile(val)
		case "src":
			cur.Src, err = regexp.Compile(val)
		case "reason":
			cur.Reason = val
		case "doc":
			cur.Doc = val
		default:
			err = fmt.Errorf("unknown key %q", key)
		}
		if err != nil {
			return nil, fmt.Errorf("known.txt:%d: %v", ln, err)
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return k, sc.Err()
}

// LoadKnown reads a known.txt file; a missing file is an empty list.
func LoadKnown(path string) (*Known, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &Known{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseKnown(f)
}

// Match returns the entry that explains the mismatch (of the reduced program src), or nil.
func (k *Known) Match(m Mismatch, src string) *Entry {
	for _, e := range k.Entries {
		if e.Kind != "*" && e.Kind != string(m.Kind) {
			continue
		}
		if !e.Sig.MatchString(m.Sig) {
			continue
		}
		if e.Src != nil && !e.Src.MatchString(src) {
			continue
		}
		return e
	}
	return nil
}
