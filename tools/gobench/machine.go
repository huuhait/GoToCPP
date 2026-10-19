package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

// loadAvg returns the 1, 5 and 15 minute load averages (zeros when unknown).
func loadAvg() [3]float64 {
	var l [3]float64
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return l
	}
	f := strings.Fields(string(data))
	for i := 0; i < 3 && i < len(f); i++ {
		l[i], _ = strconv.ParseFloat(f[i], 64)
	}
	return l
}

func cpuModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return runtime.GOARCH
	}
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
			return strings.Join(strings.Fields(v), " ")
		}
	}
	return runtime.GOARCH
}

func memGiB() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				kb, _ := strconv.ParseInt(f[1], 10, 64)
				return int((kb + (1 << 19)) >> 20)
			}
		}
	}
	return 0
}

// versionLine returns the first line of "<tool> <flag>", or "not found".
func versionLine(tool string, args ...string) string {
	out, err := exec.Command(tool, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return "not found"
	}
	return firstLine(string(out))
}

// machineInfo describes this machine and the toolchains (the load is filled
// by the caller before and after the run).
func machineInfo(gocppBin string) benchdata.Machine {
	kernel := ""
	if d, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		kernel = strings.TrimSpace(string(d))
	}
	return benchdata.Machine{
		CPU: cpuModel(), Cores: runtime.NumCPU(), MemGiB: memGiB(),
		OS:    strings.TrimSpace(runtime.GOOS + "/" + runtime.GOARCH + " " + kernel),
		Go:    versionLine("go", "version"),
		Gxx:   versionLine("g++", "--version"),
		Clang: versionLine("clang++", "--version"),
		Gocpp: versionLine(gocppBin, "version"),
	}
}

// cpuTimes reads the per-CPU (idle, total) jiffies of /proc/stat.
func cpuTimes() map[int][2]uint64 {
	out := map[int][2]uint64{}
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.HasPrefix(f[0], "cpu") || f[0] == "cpu" {
			continue
		}
		n, err := strconv.Atoi(f[0][3:])
		if err != nil {
			continue
		}
		var total, idle uint64
		for i, s := range f[1:] {
			v, _ := strconv.ParseUint(s, 10, 64)
			if i >= 8 { // guest times are already part of user
				break
			}
			total += v
			if i == 3 || i == 4 { // idle, iowait
				idle += v
			}
		}
		out[n] = [2]uint64{idle, total}
	}
	return out
}

// idleCores samples /proc/stat for the given time and returns the cores that
// were at least minIdle (a fraction) idle, the idlest first.
func idleCores(sample time.Duration, minIdle float64) []int {
	a := cpuTimes()
	time.Sleep(sample)
	b := cpuTimes()
	return pickIdle(a, b, minIdle)
}

// pickIdle is idleCores' arithmetic: the cores whose idle share between the two
// readings is at least minIdle, the idlest first (ties: lower number first).
func pickIdle(a, b map[int][2]uint64, minIdle float64) []int {
	type cs struct {
		n    int
		idle float64
	}
	var cands []cs
	for n, x := range a {
		y, ok := b[n]
		if !ok || y[1] <= x[1] {
			continue
		}
		idle := float64(y[0]-x[0]) / float64(y[1]-x[1])
		if idle >= minIdle {
			cands = append(cands, cs{n, idle})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].idle != cands[j].idle {
			return cands[i].idle > cands[j].idle
		}
		return cands[i].n < cands[j].n
	})
	var out []int
	for _, c := range cands {
		out = append(out, c.n)
	}
	return out
}

// pinList chooses the cpu list for taskset: want idle cores (the first of
// the idlest), or "" with the reason when pinning is not possible. mode is
// "auto", "off" or an explicit cpu list.
func pinList(mode string, want int) (list, why string) {
	switch mode {
	case "off", "":
		return "", "pinning is off"
	case "auto":
	default:
		return mode, ""
	}
	if _, err := exec.LookPath("taskset"); err != nil {
		return "", "taskset is not installed"
	}
	idle := idleCores(400*time.Millisecond, 0.95)
	if len(idle) < want {
		return "", fmt.Sprintf("only %d idle cores, want %d", len(idle), want)
	}
	sel := append([]int(nil), idle[:want]...)
	sort.Ints(sel)
	parts := make([]string, len(sel))
	for i, n := range sel {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ","), ""
}

// gitInfo returns the short commit id of dir's HEAD and whether the tree has
// uncommitted changes.
func gitInfo(dir string) (commit string, dirty bool) {
	out, err := capture(dir, nil, 20*time.Second, "git", "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return "unknown", false
	}
	commit = strings.TrimSpace(out)
	st, err := capture(dir, nil, 30*time.Second, "git", "status", "--porcelain", "--untracked-files=no")
	return commit, err == nil && strings.TrimSpace(st) != ""
}

// gitSubject returns the subject line of HEAD.
func gitSubject(dir string) string {
	out, err := capture(dir, nil, 20*time.Second, "git", "log", "-1", "--format=%s")
	if err != nil {
		return ""
	}
	return firstLine(out)
}

// childProcs returns the parallelism the benchmarked programs get: the number
// of cpus of the taskset list when pinned, else the default GOMAXPROCS of this
// runner (the cpus it may run on, limited by a cgroup quota; Go 1.25 reads it;
// a GOMAXPROCS in the environment is not passed to the programs, see
// benchEnv, so the default applies).
func childProcs(pin string) int {
	if pin != "" {
		return countCPUList(pin)
	}
	if os.Getenv("GOMAXPROCS") != "" {
		return runtime.NumCPU()
	}
	return runtime.GOMAXPROCS(0)
}

// countCPUList counts the cpus of a taskset list such as "0,2-5".
func countCPUList(list string) int {
	n := 0
	for _, part := range strings.Split(list, ",") {
		lo, hi, isRange := strings.Cut(strings.TrimSpace(part), "-")
		a, err := strconv.Atoi(lo)
		if err != nil {
			continue
		}
		if !isRange {
			n++
			continue
		}
		if b, err := strconv.Atoi(hi); err == nil && b >= a {
			n += b - a + 1
		}
	}
	return n
}
