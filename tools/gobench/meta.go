package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

// Program is one benchmark: a .gp file in bench/programs/<category>/ whose
// leading comment lines say how to run it:
//
//	// bench: nbody          the name (the file's name without .gp)
//	// category: cpu         the folder's name
//	// size: 5000000         benchN of the full run
//	// smoke: 1000           benchN of the smoke run (builds and runs in milliseconds)
//	// twin: yes             bench/cpp/nbody.cpp exists (idiomatic C++20, BENCH_N is the size)
//	// needs: sync           optional: what the program needs; when gocpp's build says it is "not
//	//                       available yet" or "not supported yet", the program is skipped (everywhere)
//	//                       until gocpp has it; without a needs line such a failure is a build failure
//	// about: one line       what the program measures
type Program struct {
	Name     string
	Category string
	Size     int
	Smoke    int
	Twin     bool
	Needs    []string
	About    string
	Path     string // the .gp file
	CppPath  string // the twin ("" without one)
}

// ParseProgram reads the header of the .gp file at path.
func ParseProgram(path string) (*Program, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	p := &Program{Path: path}
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "//") {
			break
		}
		key, val, ok := strings.Cut(strings.TrimSpace(strings.TrimPrefix(line, "//")), ":")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		switch key {
		case "bench", "category", "size", "smoke", "twin", "needs", "about":
		default:
			continue // an ordinary comment that has a colon
		}
		if seen[key] {
			return nil, fmt.Errorf("%s: duplicate header line %q", path, key)
		}
		seen[key] = true
		switch key {
		case "bench":
			p.Name = val
		case "category":
			p.Category = val
		case "size", "smoke":
			n, err := strconv.Atoi(val)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("%s: %s: %q is not a positive integer", path, key, val)
			}
			if key == "size" {
				p.Size = n
			} else {
				p.Smoke = n
			}
		case "twin":
			switch val {
			case "yes":
				p.Twin = true
			case "no":
			default:
				return nil, fmt.Errorf("%s: twin: want yes or no, got %q", path, val)
			}
		case "needs":
			p.Needs = strings.Fields(strings.ReplaceAll(val, ",", " "))
		case "about":
			p.About = val
		}
	}
	for _, k := range []string{"bench", "category", "size", "smoke", "twin", "about"} {
		if !seen[k] {
			return nil, fmt.Errorf("%s: missing header line %q", path, "// "+k+": ...")
		}
	}
	base := strings.TrimSuffix(filepath.Base(path), ".gp")
	if p.Name != base {
		return nil, fmt.Errorf("%s: bench: %q, but the file is named %q", path, p.Name, base)
	}
	if want := filepath.Base(filepath.Dir(path)); p.Category != want {
		return nil, fmt.Errorf("%s: category: %q, but the folder is %q", path, p.Category, want)
	}
	if !slices.Contains(benchdata.Categories, p.Category) {
		return nil, fmt.Errorf("%s: unknown category %q (have %v)", path, p.Category, benchdata.Categories)
	}
	return p, nil
}

// LoadPrograms reads every program of the bench folder dir (bench/): each
// bench/programs/<category>/<name>.gp, and the twin bench/cpp/<name>.cpp of
// those with `twin: yes`. It checks that a twin file exists exactly for the
// programs that say so, and that names are unique.
func LoadPrograms(dir string) ([]*Program, error) {
	files, err := filepath.Glob(filepath.Join(dir, "programs", "*", "*.gp"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []*Program
	names := map[string]string{}
	for _, f := range files {
		p, err := ParseProgram(f)
		if err != nil {
			return nil, err
		}
		if prev, dup := names[p.Name]; dup {
			return nil, fmt.Errorf("%s: the name %q is also used by %s", f, p.Name, prev)
		}
		names[p.Name] = f
		cpp := filepath.Join(dir, "cpp", p.Name+".cpp")
		_, statErr := os.Stat(cpp)
		switch {
		case p.Twin && statErr != nil:
			return nil, fmt.Errorf("%s: twin: yes, but %s does not exist", f, cpp)
		case !p.Twin && statErr == nil:
			return nil, fmt.Errorf("%s: twin: no, but %s exists", f, cpp)
		case p.Twin:
			p.CppPath = cpp
		}
		out = append(out, p)
	}
	rank := func(c string) int { return slices.Index(benchdata.Categories, c) }
	sort.SliceStable(out, func(i, j int) bool {
		if rank(out[i].Category) != rank(out[j].Category) {
			return rank(out[i].Category) < rank(out[j].Category)
		}
		return out[i].Name < out[j].Name
	})
	// A twin without a program is an error too.
	cpps, _ := filepath.Glob(filepath.Join(dir, "cpp", "*.cpp"))
	for _, c := range cpps {
		if _, ok := names[strings.TrimSuffix(filepath.Base(c), ".cpp")]; !ok {
			return nil, fmt.Errorf("%s: no program bench/programs/*/%s.gp", c, strings.TrimSuffix(filepath.Base(c), ".cpp"))
		}
	}
	return out, nil
}

// Select returns the programs whose name or category matches the regular
// expression match (nil: all).
func Select(ps []*Program, match func(string) bool) []*Program {
	if match == nil {
		return ps
	}
	var out []*Program
	for _, p := range ps {
		if match(p.Name) || match(p.Category) || match(p.Category+"/"+p.Name) {
			out = append(out, p)
		}
	}
	return out
}
