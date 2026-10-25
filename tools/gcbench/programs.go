package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// program is one measured program: bench/gc/<name>/main.gp, one file that is
// valid Go and gp.
type program struct {
	name  string
	about string
	// sameAcrossThreads is true when the deterministic output must be the same
	// at every thread count: binarytrees' checksum. The operation counts of
	// parks and syscalls follow the thread count by design.
	sameAcrossThreads bool
}

// programs are the programs of step 16.1 that this harness knows, in the
// order of the report.
var programs = []program{
	{"binarytrees", "allocate, walk and free small pointer trees (allocation speed and peak memory)", true},
	{"parks", "park and wake goroutines on channels, on select and in Sleep (blocking sections)", false},
	{"syscalls", "os.Getpid and time.Now in a loop (the syscalls of the runtime's time path)", false},
}

// sameAcrossThreads reports whether the program named name must print the same
// output at every thread count. It looks the name up in the registry, so the
// markdown of a JSON file (made by the report command) checks it too.
func sameAcrossThreads(name string) bool {
	for _, p := range programs {
		if p.name == name {
			return p.sameAcrossThreads
		}
	}
	return false
}

// programDir is the source folder of p.
func (c *config) programDir(p program) string {
	return filepath.Join(c.root, "bench", "gc", p.name)
}

// programArgs returns the arguments of one run: the thread count, the
// binarytrees depth, and -quiet so that the output is only the deterministic
// line (the timing lines differ between engines by design).
func (c *config) programArgs(p program, threads int) []string {
	args := []string{strconv.Itoa(threads)}
	if p.name == "binarytrees" {
		args = append(args, strconv.Itoa(c.depth))
	}
	return append(args, "-quiet")
}

// parsePrograms parses -bench: a comma separated list of names, or "all".
func parsePrograms(s string) ([]program, error) {
	var out []program
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		if x == "all" {
			return append([]program(nil), programs...), nil
		}
		i := slices.IndexFunc(programs, func(p program) bool { return p.name == x })
		if i < 0 {
			return nil, fmt.Errorf("unknown program %q (have binarytrees, parks, syscalls, all)", x)
		}
		if !slices.ContainsFunc(out, func(p program) bool { return p.name == x }) {
			out = append(out, programs[i])
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-bench is empty (have binarytrees, parks, syscalls, all)")
	}
	return out, nil
}

// flavors are the engines this harness builds, in the order of the tables.
var flavors = []string{"go", "arc", "gc"}

// parseFlavors parses -flavor: a comma separated list of flavors, or "all".
func parseFlavors(s string) ([]string, error) {
	var out []string
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		if x == "all" {
			return append([]string(nil), flavors...), nil
		}
		if !slices.Contains(flavors, x) {
			return nil, fmt.Errorf("unknown flavor %q (have go, arc, gc, all)", x)
		}
		if !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-flavor is empty (have go, arc, gc, all)")
	}
	return out, nil
}

// parseThreads parses -threads: a comma separated list of positive thread
// counts, without repeats. The default is 1,2,4,8,16,32,64, and 1,8 for a
// smoke run.
func parseThreads(s string, smoke bool) ([]int, error) {
	if strings.TrimSpace(s) == "" {
		if smoke {
			return []int{1, 8}, nil
		}
		return []int{1, 2, 4, 8, 16, 32, 64}, nil
	}
	var out []int
	for _, x := range strings.Split(s, ",") {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		n, err := strconv.Atoi(x)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("thread count %q is not a positive integer", x)
		}
		if slices.Contains(out, n) {
			return nil, fmt.Errorf("thread count %d is listed twice", n)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-threads is empty")
	}
	slices.Sort(out)
	return out, nil
}
