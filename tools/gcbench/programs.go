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
	// parks and syscalls follow the thread count by design, and so does the
	// goroutine count of goroutines.
	sameAcrossThreads bool
	// goroutines is true for the program whose "thread" counts are goroutine
	// counts (it starts N goroutines), so its runs use -goroutine-threads and
	// -goroutine-depth, and its Go runs keep GOMAXPROCS at 16 (the processors,
	// not N).
	goroutines bool
	// modes are the run modes of the program: one run of every thread count
	// per mode. Nil is one run with no mode.
	modes []string
}

// programs are the programs of step 16.1 that this harness knows, in the
// order of the report.
var programs = []program{
	{name: "binarytrees", about: "allocate, walk and free small pointer trees (allocation speed and peak memory)", sameAcrossThreads: true},
	{name: "parks", about: "park and wake goroutines on channels, on select and in Sleep (blocking sections)"},
	{name: "syscalls", about: "os.Getpid and time.Now in a loop (the syscalls of the runtime's time path)"},
	{name: "goroutines", about: "start N goroutines parked on a channel or running a recursion D frames deep (the cost of a goroutine and of the stop that scans its stack)", goroutines: true, modes: []string{"blocked", "running"}},
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

// programArgs returns the arguments of one run: the thread count (a goroutine
// count for the goroutines program), the depth of binarytrees or goroutines,
// and -quiet so that the output is only the deterministic lines (the timing
// lines differ between engines by design).
func (c *config) programArgs(p program, mode string, threads int) []string {
	if p.goroutines {
		return []string{"-n", strconv.Itoa(threads), "-mode", mode, "-depth", strconv.Itoa(c.goroutineDepth), "-quiet"}
	}
	args := []string{strconv.Itoa(threads)}
	if p.name == "binarytrees" {
		args = append(args, strconv.Itoa(c.depth))
	}
	return append(args, "-quiet")
}

// threadsFor returns the thread counts of p: the goroutine counts for the
// goroutines program, the shared thread counts for every other program.
func (c *config) threadsFor(p program) []int {
	if p.goroutines {
		return c.goroutineThreads
	}
	return c.threads
}

// hasBenchProgram reports whether the selected programs include name.
func hasBenchProgram(ps []program, name string) bool {
	for _, p := range ps {
		if p.name == name {
			return true
		}
	}
	return false
}

// modesOf returns the modes p runs in: its own, or one unnamed run.
func modesOf(p program) []string {
	if len(p.modes) == 0 {
		return []string{""}
	}
	return p.modes
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
			return nil, fmt.Errorf("unknown program %q (have binarytrees, parks, syscalls, goroutines, all)", x)
		}
		if !slices.ContainsFunc(out, func(p program) bool { return p.name == x }) {
			out = append(out, programs[i])
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-bench is empty (have binarytrees, parks, syscalls, goroutines, all)")
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

// parseGoroutineThreads parses -goroutine-threads: the goroutine counts of the
// goroutines program. The default is 1,1000,10000, and 1,10 for a smoke run.
func parseGoroutineThreads(s string, smoke bool) ([]int, error) {
	if strings.TrimSpace(s) == "" {
		if smoke {
			return []int{1, 10}, nil
		}
		return []int{1, 1000, 10000}, nil
	}
	return parseThreads(s, false)
}
