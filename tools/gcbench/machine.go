package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// machine describes the machine and the toolchains of a run, with the load
// average before and after it. Two machines' numbers are never compared.
type machine struct {
	CPU        string     `json:"cpu"`
	Cores      int        `json:"cores"`
	MemGiB     int        `json:"mem_gib"`
	OS         string     `json:"os"`
	Go         string     `json:"go"`
	Gxx        string     `json:"gxx"`
	Clang      string     `json:"clang"`
	Gocpp      string     `json:"gocpp"`
	LoadBefore [3]float64 `json:"load_before"`
	LoadAfter  [3]float64 `json:"load_after"`
	MaxLoad    float64    `json:"max_load"`
	Busy       bool       `json:"busy"` // the load was over MaxLoad before or after the run
}

// machineInfo describes this machine and the toolchains. The load is filled
// by the caller.
func machineInfo(gocppBin string) machine {
	kernel := ""
	if d, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		kernel = strings.TrimSpace(string(d))
	}
	gocppVersion := "not used"
	if gocppBin != "" {
		gocppVersion = versionLine(gocppBin, "version")
	}
	return machine{
		CPU: cpuModel(), Cores: runtime.NumCPU(), MemGiB: memGiB(),
		OS:    strings.TrimSpace(runtime.GOOS + "/" + runtime.GOARCH + " " + kernel),
		Go:    versionLine("go", "version"),
		Gxx:   versionLine("g++", "--version"),
		Clang: versionLine("clang++", "--version"),
		Gocpp: gocppVersion,
	}
}

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

// cpuModel returns the model name of the CPU, or the architecture.
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

// memGiB returns the memory of the machine in GiB (0 when unknown).
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

// versionLine returns the first line of "<tool> <args>", or "not found".
func versionLine(tool string, args ...string) string {
	out, err := exec.Command(tool, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return "not found"
	}
	return firstLine(string(out))
}

// gitState returns the short commit of the repository and whether it has
// uncommitted changes (empty and false outside a git repository).
func gitState(root string) (commit string, dirty bool) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = root
	if out, err := cmd.Output(); err == nil {
		commit = strings.TrimSpace(string(out))
	}
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	if out, err := cmd.Output(); err == nil {
		dirty = strings.TrimSpace(string(out)) != ""
	}
	return commit, dirty
}

// logMachine prints the machine line of the run.
func logMachine(log func(string, ...any), m machine) {
	log("machine: %s, %d cores, %d GiB, %s", m.CPU, m.Cores, m.MemGiB, m.OS)
	log("load: %.2f %.2f %.2f before, limit for a quiet machine %.1f", m.LoadBefore[0], m.LoadBefore[1], m.LoadBefore[2], m.MaxLoad)
}

// busyNote returns the warning line of a run on a loaded machine, or "".
func (m machine) busyNote() string {
	if !m.Busy {
		return ""
	}
	return fmt.Sprintf("the machine was loaded (1-minute load %.1f before and %.1f after the run; the limit for a quiet machine is %.1f), so the times are noisy",
		m.LoadBefore[0], m.LoadAfter[0], m.MaxLoad)
}
