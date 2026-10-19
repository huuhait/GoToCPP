package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/huuhait/gocpp/tools/gobench/benchdata"
)

// Suite is a built set of programs that can be measured (again).
type Suite struct {
	cfg      *Config
	programs []*Program
	arts     map[string][]*artifact
}

// NewSuite builds gocpp (when cfg.Gocpp is empty), then every selected
// implementation of every program.
func NewSuite(cfg *Config, ps []*Program) (*Suite, error) {
	if err := os.MkdirAll(cfg.Work, 0o755); err != nil {
		return nil, err
	}
	needGocpp := false
	for _, i := range cfg.Impls {
		needGocpp = needGocpp || benchdata.IsGocpp(i)
	}
	if needGocpp && cfg.Gocpp == "" {
		bin := filepath.Join(cfg.Work, "gocpp")
		cfg.logf("building gocpp from %s ...", cfg.Root)
		out, err := capture(cfg.Root, cfg.goEnv(), 15*time.Minute, cfg.GoBin, "build", "-o", bin, "./cmd/gocpp")
		if err != nil {
			return nil, fmt.Errorf("building gocpp: %v\n%s", err, out)
		}
		cfg.Gocpp = bin
	}
	if needGocpp {
		out, _ := capture(cfg.Root, cfg.goEnv(), time.Minute, cfg.Gocpp, "build", "-help")
		cfg.Release = strings.Contains(out, "-release")
	}
	return &Suite{cfg: cfg, programs: ps, arts: cfg.buildAll(ps)}, nil
}

// cmdFor returns the command line that runs bin, under taskset when pinned.
func (c *Config) cmdFor(bin string) []string {
	if c.PinList != "" && c.Taskset != "" {
		return []string{c.Taskset, "-c", c.PinList, bin}
	}
	return []string{bin}
}

// measure runs the programs of one benchmark: Warmup + Reps rounds, each
// round runs every implementation once (in a fixed order), so that a change of
// the machine's load during the program hits every implementation alike. The
// warm-up rounds are dropped. A failing run (exit status, timeout) or a
// different output marks the implementation and takes it out of the rounds.
func (s *Suite) measure(p *Program, arts []*artifact) {
	c := s.cfg
	live := []*artifact{}
	for _, a := range arts {
		if a.ok() {
			live = append(live, a)
		}
	}
	for round := 0; round < c.Warmup+c.Reps && len(live) > 0; round++ {
		var next []*artifact
		for _, a := range live {
			smp, err := runTimed(c.cmdFor(a.bin), benchEnv(), c.Timeout)
			if err != nil {
				a.result.Status = benchdata.StatusRunFailed
				a.result.Note = err.Error()
				a.result.Wall, a.result.CPU, a.result.RSS = nil, nil, nil
				continue
			}
			if round > 0 && smp.Hash != a.result.Output {
				a.result.Status = benchdata.StatusMismatch
				a.result.Note = fmt.Sprintf("the output changes between runs of the same binary (%s, then %s)", a.result.Output, smp.Hash)
				continue
			}
			a.result.Output, a.head = smp.Hash, smp.Head
			if round >= c.Warmup {
				a.result.Wall = append(a.result.Wall, smp.Wall.Nanoseconds())
				a.result.CPU = append(a.result.CPU, smp.CPU.Nanoseconds())
				a.result.RSS = append(a.result.RSS, smp.RSSKB)
			}
			next = append(next, a)
		}
		live = next
	}
	checkOutputs(arts)
}

// checkOutputs marks every implementation whose output differs from the
// reference's as a mismatch: the reference is the Go program, else the first
// implementation that ran.
func checkOutputs(arts []*artifact) {
	var ref *artifact
	for _, a := range arts {
		if a.ok() && a.impl == benchdata.ImplGo {
			ref = a
		}
	}
	if ref == nil {
		for _, a := range arts {
			if a.ok() {
				ref = a
				break
			}
		}
	}
	if ref == nil {
		return
	}
	for _, a := range arts {
		if a == ref || !a.ok() || a.result.Output == ref.result.Output {
			continue
		}
		a.result.Status = benchdata.StatusMismatch
		a.result.Note = fmt.Sprintf("OUTPUT MISMATCH with %s: first bytes %q, expected %q", ref.impl, a.head, ref.head)
	}
}

// Run measures every program of the suite once and returns the results.
func (s *Suite) Run() []benchdata.Result {
	var out []benchdata.Result
	for _, p := range s.programs {
		arts := s.arts[p.Name]
		s.measure(p, arts)
		for _, a := range arts {
			out = append(out, a.result)
			if a.result.Status == benchdata.StatusOK {
				st := a.result.WallStat()
				s.cfg.logf("ran   %-22s %-14s median %9.2f ms  MAD %5.2f%%  rss %d KiB", p.Category+"/"+p.Name, a.impl, st.Median/1e6, st.RelNoise()*100, int64(a.result.RSSStat().Median))
			} else {
				s.cfg.logf("ran   %-22s %-14s %s %s", p.Category+"/"+p.Name, a.impl, a.result.Status, a.result.Note)
			}
		}
	}
	return out
}

// Remeasure measures the pairs (program, impl) again, with the binaries that
// are built, and returns the new results.
func (s *Suite) Remeasure(pairs map[string][]string) []benchdata.Result {
	var out []benchdata.Result
	for _, p := range s.programs {
		impls := pairs[p.Name]
		if len(impls) == 0 {
			continue
		}
		var arts []*artifact
		for _, a := range s.arts[p.Name] {
			for _, i := range impls {
				if a.impl == i && a.result.Status != benchdata.StatusSkipped && a.bin != "" {
					cp := *a
					cp.result = a.result
					cp.result.Status = benchdata.StatusOK
					cp.result.Wall, cp.result.CPU, cp.result.RSS = nil, nil, nil
					arts = append(arts, &cp)
				}
			}
		}
		s.measure(p, arts)
		for _, a := range arts {
			out = append(out, a.result)
		}
	}
	return out
}

// Failures returns a line for every result that is not ok and not skipped.
func Failures(rs []benchdata.Result) []string {
	var out []string
	for _, r := range rs {
		if r.Status != benchdata.StatusOK && r.Status != benchdata.StatusSkipped {
			out = append(out, fmt.Sprintf("%s/%s [%s]: %s: %s", r.Category, r.Program, r.Impl, strings.ToUpper(r.Status), r.Note))
		}
	}
	return out
}

// toolPath resolves a tool name from PATH; the name itself when not found.
func toolPath(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}
