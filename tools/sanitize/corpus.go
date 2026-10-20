package main

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A Program is one test program of the corpus: a single .gp file, run in its own folder.
type Program struct {
	Path   string // absolute
	Rel    string // relative to the repository root, with slashes
	Header *Header
}

// Name returns the program's name for logs: its Rel without the .gp.
func (p *Program) Name() string { return strings.TrimSuffix(p.Rel, ".gp") }

// The named corpora.
const (
	CorpusRun         = "run"         // every program of tests/run
	CorpusConcurrency = "concurrency" // the programs of tests/run that start goroutines or use channels, select, sync or time, and the silent programs of tests/race (what -race checks)
	CorpusSample      = "sample"      // the short representative list of tools/sanitize/sample.txt
	CorpusRace        = "race"        // the silent programs of tests/race (ok_*.gp) alone
	CorpusCanary      = "canary"      // tools/sanitize/testdata/canary: programs that are wrong on purpose, and clean controls
)

// concurrencyRx is the test of tests/race/concurrency.sh: a program of tests/run that is in the
// concurrency family.
var concurrencyRx = regexp.MustCompile(`(?m)^[ \t]*go |go func|make\(chan|select \{|"sync|"time"`)

// racyRx marks a program of tests/run that has a data race on purpose (its header says
// "// race: intentional"): the race job leaves it out, as concurrency.sh does.
var racyRx = regexp.MustCompile(`(?m)^// race: intentional$`)

// A Selection says which programs a run takes.
type Selection struct {
	Root    string   // the repository root
	Corpus  string   // a named corpus, or "" when Paths is used
	Paths   []string // programs by path (relative to Root, or absolute)
	Filter  *regexp.Regexp
	Phase   int  // the highest phase to run
	Pending bool // also the programs that wait for an unbuilt step
	Shard   string
	Limit   int
}

// Skipped counts the programs that a selection left out, by reason.
type Skipped map[string]int

// total returns how many programs were skipped.
func (s Skipped) total() int {
	n := 0
	for _, c := range s {
		n += c
	}
	return n
}

func (s Skipped) String() string {
	if len(s) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, s[k]))
	}
	return strings.Join(parts, ", ")
}

// Pick returns the programs of the selection in path order, and the counts of the ones left out.
func (s Selection) Pick() ([]*Program, Skipped, error) {
	files, err := s.files()
	if err != nil {
		return nil, nil, err
	}
	si, sn, err := parseShard(s.Shard)
	if err != nil {
		return nil, nil, err
	}
	skipped := Skipped{}
	var out []*Program
	for _, rel := range files {
		if s.Filter != nil && !s.Filter.MatchString(rel) {
			continue
		}
		if !inShard(rel, si, sn) {
			continue
		}
		abs := filepath.Join(s.Root, filepath.FromSlash(rel))
		h, err := ReadHeader(abs)
		if err != nil {
			return nil, nil, err
		}
		if why := h.Skip(s.Phase, s.Pending); why != "" {
			skipped[why]++
			continue
		}
		out = append(out, &Program{Path: abs, Rel: rel, Header: h})
	}
	if s.Limit > 0 && len(out) > s.Limit {
		out = out[:s.Limit]
	}
	return out, skipped, nil
}

// files returns the relative paths of the candidates, sorted.
func (s Selection) files() ([]string, error) {
	var out []string
	switch {
	case len(s.Paths) > 0:
		for _, p := range s.Paths {
			rel := p
			if filepath.IsAbs(p) {
				r, err := filepath.Rel(s.Root, p)
				if err != nil {
					return nil, err
				}
				rel = r
			}
			rel = filepath.ToSlash(filepath.Clean(rel))
			if _, err := os.Stat(filepath.Join(s.Root, filepath.FromSlash(rel))); err != nil {
				return nil, err
			}
			out = append(out, rel)
		}
	case s.Corpus == CorpusRun:
		out = globRel(s.Root, "tests/run/*.gp")
	case s.Corpus == CorpusRace:
		out = globRel(s.Root, "tests/race/ok_*.gp")
	case s.Corpus == CorpusConcurrency:
		for _, rel := range globRel(s.Root, "tests/run/*.gp") {
			b, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(rel)))
			if err != nil {
				return nil, err
			}
			if concurrencyRx.Match(b) && !racyRx.Match(b) {
				out = append(out, rel)
			}
		}
		out = append(out, globRel(s.Root, "tests/race/ok_*.gp")...)
	case s.Corpus == CorpusCanary:
		out = globRel(s.Root, "tools/sanitize/testdata/canary/*.gp")
	case s.Corpus == CorpusSample:
		list, err := readList(filepath.Join(s.Root, "tools", "sanitize", "sample.txt"))
		if err != nil {
			return nil, err
		}
		out = list
	default:
		return nil, fmt.Errorf("unknown corpus %q (want run, concurrency, race, canary or sample, or give program paths)", s.Corpus)
	}
	sort.Strings(out)
	return out, nil
}

// readList reads a list file: one path per line, # comments and blank lines ignored.
func readList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out, sc.Err()
}

func globRel(root, pattern string) []string {
	m, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
	out := make([]string, 0, len(m))
	for _, p := range m {
		r, err := filepath.Rel(root, p)
		if err == nil {
			out = append(out, filepath.ToSlash(r))
		}
	}
	sort.Strings(out)
	return out
}

// parseShard reads "i/n"; "" is the whole corpus (0/1).
func parseShard(s string) (i, n int, err error) {
	if s == "" {
		return 0, 1, nil
	}
	a, b, ok := strings.Cut(s, "/")
	if ok {
		i, err = strconv.Atoi(a)
		if err == nil {
			n, err = strconv.Atoi(b)
		}
	}
	if !ok || err != nil || n < 1 || i < 0 || i >= n {
		return 0, 0, fmt.Errorf("-shard %q: want i/n with 0 <= i < n", s)
	}
	return i, n, nil
}

// inShard reports whether path is in shard i of n. It hashes the path, so that a shard keeps its
// programs when programs are added.
func inShard(path string, i, n int) bool {
	if n <= 1 {
		return true
	}
	h := fnv.New32a()
	h.Write([]byte(path))
	return int(h.Sum32()%uint32(n)) == i
}
