package bench

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"time"
)

// RunProtocol records the workload and toolchain, not just a descriptive label.
// Bump Method when requests, waits, cache policy, or generated module semantics
// change. Runs may differ: Merge weights individual runs, not report medians.
type RunProtocol struct {
	Method              string
	ConfigurationFormat string // configFingerprintFormat; raw legacy configuration is not publishable
	Runs                int
	Scale               float64
	Timeout, ColdMax    time.Duration
	Only                []string
	Environment         []string
	GoCommand           string
	Affinity            string
}

const configFingerprintFormat = "named-sha256-v1"

// Configuration values may contain credentials (URL userinfo/query, GOAUTH,
// compiler command arguments). Named, domain-separated fingerprints preserve
// comparability without publishing their raw text. These are fingerprints, NOT
// anonymization or cryptographic protection: low-entropy values can be guessed.
func configFingerprint(kind, value string) string {
	h := sha256.Sum256([]byte("gocpp-lspbench-config-v1\x00" + kind + "\x00" + value))
	return "sha256:" + hex.EncodeToString(h[:])
}

func environmentFingerprints(env []string) []string {
	out := make([]string, len(env))
	for i, e := range env {
		k, v, ok := strings.Cut(e, "=")
		if !ok || k == "" {
			k = "<invalid>"
			v = e
		}
		out[i] = k + "=" + configFingerprint("environment:"+k, v)
	}
	return out
}

func commandFingerprints(command []string) []string {
	out := slices.Clone(command)
	for i := 1; i < len(out); i++ {
		out[i] = configFingerprint("argument", out[i])
	}
	return out
}

func settingsFingerprint(settings json.RawMessage) json.RawMessage {
	if len(settings) == 0 {
		return nil
	}
	b, _ := json.Marshal(configFingerprint("settings", string(settings)))
	return b
}

func isConfigFingerprint(s string) bool {
	if !strings.HasPrefix(s, "sha256:") || len(s) != len("sha256:")+64 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(s, "sha256:"))
	return err == nil
}

func safeEnvironment(env []string) bool {
	for _, e := range env {
		k, v, ok := strings.Cut(e, "=")
		if !ok || k == "" || !isConfigFingerprint(v) {
			return false
		}
	}
	return true
}

func safeResultConfiguration(id *ResultIdentity) bool {
	if !safeEnvironment(id.Env) {
		return false
	}
	for _, arg := range id.Command[1:] {
		if !isConfigFingerprint(arg) {
			return false
		}
	}
	// A nil RawMessage round-trips through JSON as the literal null.
	if len(id.Settings) > 0 && string(id.Settings) != "null" {
		var s string
		if json.Unmarshal(id.Settings, &s) != nil || !isConfigFingerprint(s) {
			return false
		}
	}
	return true
}

// ResultIdentity matches a project's bytes and each target's configuration.
// Command[0] is retained as provenance but the binary digest, not its pathname,
// identifies the build. Arguments, environment values and settings are named
// fingerprints, never raw configuration text. A/B builds belong in separate
// merge cohorts: combining their samples under one target name erases the comparison.
type ResultIdentity struct {
	ProjectSHA256, BinarySHA256 string
	Command, Env                []string
	Ext                         string
	Settings                    json.RawMessage
}

// NewReport records the effective protocol of a measured invocation. All
// server processes inherit the same relevant environment and CPU affinity.
func NewReport(cfg Config) *Report {
	cfg.defaults()
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	p := &RunProtocol{
		Method:              "fresh-edits-v2-go1.25",
		ConfigurationFormat: configFingerprintFormat,
		Runs:                cfg.Runs,
		Scale:               cfg.Scale,
		Timeout:             cfg.Timeout,
		ColdMax:             cfg.ColdMax,
		Only:                slices.Clone(cfg.Only),
		GoCommand:           CommandOutput("go", "version"),
	}
	slices.Sort(p.Only)
	// Do not dump the entire environment (it can contain credentials). These
	// variables affect Go, gocpp, native tools, process scheduling or lookup.
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if strings.HasPrefix(k, "GO") || strings.HasPrefix(k, "CGO") || strings.HasPrefix(k, "CC") || strings.HasPrefix(k, "CXX") ||
			k == "PATH" || k == "NIX_LD" || k == "LD_LIBRARY_PATH" || k == "OMP_NUM_THREADS" {
			p.Environment = append(p.Environment, e)
		}
	}
	p.Environment = environmentFingerprints(p.Environment)
	slices.Sort(p.Environment)
	if b, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "Cpus_allowed_list:") {
				p.Affinity = strings.TrimSpace(strings.TrimPrefix(l, "Cpus_allowed_list:"))
			}
		}
	}
	return &Report{Machine: DescribeMachine(), ID: hex.EncodeToString(id[:]), Protocol: p}
}

func identifyResult(proj *Project, t Target) (*ResultIdentity, error) {
	// In-process fake servers have no binary; they remain usable by harness
	// tests but their reports are deliberately not mergeable.
	if t.Start != nil {
		return nil, nil
	}
	if len(t.Command) == 0 {
		return nil, fmt.Errorf("%s: no server command", t.Name)
	}
	path, err := exec.LookPath(t.Command[0])
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	// Include the module language version and all project data/anchors. Both
	// extensions use the same digest; the extension is matched per target.
	b, err := json.Marshal(struct {
		Project *Project
		Go      string
	}{proj, projectGoVersion})
	if err != nil {
		return nil, err
	}
	ph := sha256.Sum256(b)
	return &ResultIdentity{
		ProjectSHA256: hex.EncodeToString(ph[:]),
		BinarySHA256:  hex.EncodeToString(h.Sum(nil)),
		Command:       commandFingerprints(t.Command),
		Env:           environmentFingerprints(t.Env),
		Ext:           t.Ext,
		Settings:      settingsFingerprint(t.Settings),
	}, nil
}

// CheckConfigurationProvenance rejects reports whose configuration may still
// contain raw credentials. In particular, rendering an old report must not
// bypass the legacy-schema rejection used by Merge.
func (r *Report) CheckConfigurationProvenance() error {
	if r == nil {
		return fmt.Errorf("report has no configuration provenance")
	}
	if len(r.Sources) > 0 {
		for _, src := range r.Sources {
			if err := src.CheckConfigurationProvenance(); err != nil {
				return err
			}
		}
	} else if r.Protocol == nil || r.Protocol.ConfigurationFormat != configFingerprintFormat ||
		!safeEnvironment(r.Protocol.Environment) {
		return fmt.Errorf("report has legacy or raw configuration; refusing to republish it")
	}
	for _, result := range r.Results {
		if result == nil || result.Identity == nil || len(result.Identity.Command) == 0 ||
			!safeResultConfiguration(result.Identity) {
			return fmt.Errorf("report has raw or missing target configuration; refusing to republish it")
		}
	}
	return nil
}

// Merge combines comparable leaf reports run by run, preserving each complete
// input under Sources. It cannot establish alternation, timing isolation, or
// correctness from file order. It rejects legacy reports without provenance,
// repeated invocations, workload/toolchain drift, mixed builds of one target,
// missing scenario sets, and inconsistent raw data. gocpp-only and gopls-only
// reports may be joined, but their common workload and machine must match.
func Merge(reps ...*Report) (*Report, error) {
	if len(reps) == 0 {
		return nil, fmt.Errorf("merge: no reports")
	}
	// A JSON round trip gives the output its own data, down to raw byte/sample
	// slices. Inputs and output cannot corrupt one another through aliases.
	b, err := json.Marshal(reps)
	if err != nil {
		return nil, fmt.Errorf("merge: %w", err)
	}
	var sources []*Report
	if err := json.Unmarshal(b, &sources); err != nil {
		return nil, err
	}
	out := &Report{Sources: sources}
	type key struct{ project, target string }
	by := map[key]*ProjectResult{}
	projects := map[string]string{}
	targets := map[string]ResultIdentity{}
	ids := map[string]bool{}
	var protocol RunProtocol
	var machine Machine
	for i, rep := range sources {
		if rep == nil || rep.Protocol == nil || rep.ID == "" || len(rep.Sources) != 0 || len(rep.Results) == 0 {
			return nil, fmt.Errorf("merge: input %d is not a measured leaf report with provenance", i+1)
		}
		if ids[rep.ID] {
			return nil, fmt.Errorf("merge: repeated invocation %s", rep.ID)
		}
		ids[rep.ID] = true
		p := *rep.Protocol
		if p.ConfigurationFormat != configFingerprintFormat || !safeEnvironment(p.Environment) {
			return nil, fmt.Errorf(
				"merge: input %d has legacy or raw configuration; refusing to republish it", i+1)
		}
		if p.Method == "" || p.Runs <= 0 || p.Scale <= 0 || p.GoCommand == "" {
			return nil, fmt.Errorf("merge: input %d has incomplete protocol", i+1)
		}
		p.Runs = 0
		m := rep.Machine
		m.Date, m.Gocpp, m.Gopls, m.Settings = "", "", "", nil
		m.LoadAvg, m.LoadEnd = [3]float64{}, [3]float64{}
		if i == 0 {
			protocol, machine = p, m
			out.Machine = rep.Machine
			out.Machine.Settings = []string{fmt.Sprintf("%d measured invocations merged; raw provenance in JSON Sources; alternation and pinning are not inferred", len(sources))}
		} else if !reflect.DeepEqual(p, protocol) || !reflect.DeepEqual(m, machine) {
			return nil, fmt.Errorf("merge: input %d has a different workload, environment, machine or toolchain", i+1)
		}
		out.Machine.LoadEnd = rep.Machine.LoadEnd
		if out.Machine.Gocpp == "" {
			out.Machine.Gocpp = rep.Machine.Gocpp
		}
		if out.Machine.Gopls == "" {
			out.Machine.Gopls = rep.Machine.Gopls
		}
		seen := map[key]bool{}
		for _, r := range rep.Results {
			if r == nil || r.Identity == nil || r.Identity.ProjectSHA256 == "" || r.Identity.BinarySHA256 == "" || len(r.Identity.Command) == 0 ||
				(r.Target != "gocpp" && r.Target != "gopls") || r.Project == "" {
				return nil, fmt.Errorf("merge: input %d has an incomplete result identity", i+1)
			}
			if !safeResultConfiguration(r.Identity) {
				return nil, fmt.Errorf(
					"merge: input %d has raw target configuration; refusing to republish it", i+1)
			}
			k := key{r.Project, r.Target}
			if seen[k] {
				return nil, fmt.Errorf("merge: duplicate result %v", k)
			}
			seen[k] = true
			ident := *r.Identity
			if old, ok := projects[r.Project]; ok && old != ident.ProjectSHA256 {
				return nil, fmt.Errorf("merge: project bytes differ for %s", r.Project)
			}
			projects[r.Project] = ident.ProjectSHA256
			ident.ProjectSHA256 = ""
			ident.Command = slices.Clone(ident.Command[1:])
			if old, ok := targets[r.Target]; ok && !reflect.DeepEqual(old, ident) {
				return nil, fmt.Errorf("merge: binary or configuration differs for %s (use separate A/B cohorts)", r.Target)
			}
			targets[r.Target] = ident
			n := rep.Protocol.Runs
			if len(r.RSSRuns) != n || len(r.PeakRSSRuns) != n || len(r.StatsRuns) != n {
				return nil, fmt.Errorf("merge: missing raw memory/stats for %v", k)
			}
			for j, rss := range r.RSSRuns {
				if !validNumber(rss) || !validNumber(r.PeakRSSRuns[j]) || r.PeakRSSRuns[j] < rss {
					return nil, fmt.Errorf("merge: invalid memory for %v", k)
				}
			}
			dst := by[k]
			if dst == nil {
				c := *r
				c.Scenarios, c.RSSRuns, c.PeakRSSRuns, c.StatsRuns, c.Notes = nil, nil, nil, nil, nil
				dst = &c
				by[k] = dst
				out.Results = append(out.Results, dst)
			} else if r.Files != dst.Files || r.Lines != dst.Lines || !sameScenarios(r, dst) {
				return nil, fmt.Errorf("merge: project dimensions or scenario sets differ for %v", k)
			}
			dst.RSSRuns = append(dst.RSSRuns, r.RSSRuns...)
			dst.PeakRSSRuns = append(dst.PeakRSSRuns, r.PeakRSSRuns...)
			dst.StatsRuns = append(dst.StatsRuns, r.StatsRuns...)
			dst.Stats = r.Stats
			for _, note := range r.Notes {
				if !slices.Contains(dst.Notes, note) {
					dst.Notes = append(dst.Notes, note)
				}
			}
			seenSc := map[string]bool{}
			for _, s := range r.Scenarios {
				if s.Name == "" || seenSc[s.Name] || len(s.Runs) != n || len(s.Samples) != n || s.Fail < 0 || (s.Fail > 0) != (s.Err != "") {
					return nil, fmt.Errorf("merge: malformed scenario %s for %v", s.Name, k)
				}
				seenSc[s.Name] = true
				for j, sum := range s.Runs {
					for _, v := range s.Samples[j] {
						if !validNumber(v) {
							return nil, fmt.Errorf("merge: invalid sample for %v/%s", k, s.Name)
						}
					}
					want := Summarize(s.Samples[j])
					want.First = sum.First // first request includes unmeasured warm-up
					if !validNumber(sum.First) || sum != want || sum.N == 0 && s.Fail == 0 {
						return nil, fmt.Errorf("merge: summary does not match raw samples for %v/%s", k, s.Name)
					}
				}
				d := dst.Scenario(s.Name)
				if d == nil {
					dst.Scenarios = append(dst.Scenarios, ScenarioResult{Name: s.Name})
					d = &dst.Scenarios[len(dst.Scenarios)-1]
				}
				d.Runs = append(d.Runs, s.Runs...)
				d.Samples = append(d.Samples, s.Samples...)
				d.Fail += s.Fail
				if d.Err == "" {
					d.Err = s.Err
				}
			}
			if len(seenSc) == 0 {
				return nil, fmt.Errorf("merge: no scenarios for %v", k)
			}
		}
	}
	for _, r := range out.Results {
		r.RSS, r.PeakRSS = Median(r.RSSRuns), slices.Max(r.PeakRSSRuns)
		for i := range r.Scenarios {
			r.Scenarios[i].Agg = Aggregates(r.Scenarios[i].Runs)
		}
	}
	return out, nil
}

func validNumber(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) && x >= 0 }

func sameScenarios(a, b *ProjectResult) bool {
	if len(a.Scenarios) != len(b.Scenarios) {
		return false
	}
	for _, s := range a.Scenarios {
		if b.Scenario(s.Name) == nil {
			return false
		}
	}
	return true
}
