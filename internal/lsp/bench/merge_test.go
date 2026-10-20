package bench

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sameJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

func mergeFixture(id, target string, values ...float64) *Report {
	r := &ProjectResult{Project: "small", Target: target, Files: 21, Lines: 2722, Notes: []string{"a"},
		Identity: &ResultIdentity{ProjectSHA256: "project", BinarySHA256: target + "-binary", Command: []string{"/bin/" + target}, Ext: ".gp"}}
	s := ScenarioResult{Name: ScHover}
	for _, v := range values {
		r.RSSRuns = append(r.RSSRuns, v*100)
		r.PeakRSSRuns = append(r.PeakRSSRuns, v*110)
		r.StatsRuns = append(r.StatsRuns, json.RawMessage(`{"checks":1}`))
		s.Samples = append(s.Samples, []float64{v, v, v * 10})
		s.Runs = append(s.Runs, Summarize(s.Samples[len(s.Samples)-1]))
	}
	s.Agg = Aggregates(s.Runs)
	r.Scenarios = []ScenarioResult{s}
	r.RSS, r.PeakRSS, r.Stats = Median(r.RSSRuns), r.PeakRSSRuns[len(values)-1], r.StatsRuns[len(values)-1]
	return &Report{ID: id, Machine: Machine{CPU: "CPU", Go: "go1.25.14 linux/amd64", Date: id, LoadAvg: [3]float64{1, 2, 3}},
		Protocol: &RunProtocol{
			Method:              "fresh-edits-v2-go1.25",
			ConfigurationFormat: configFingerprintFormat,
			Runs:                len(values),
			Scale:               1,
			Timeout:             time.Minute,
			ColdMax:             5 * time.Minute,
			GoCommand:           "go version go1.25.14 linux/amd64",
			Affinity:            "8-15",
		},
		Results: []*ProjectResult{r},
	}
}

func TestMerge(t *testing.T) {
	a, b := mergeFixture("a", "gocpp", 1), mergeFixture("b", "gocpp", 2, 3)
	// Aggregates in the files are not authoritative; merge computes them anew.
	a.Results[0].Scenarios[0].Agg.P95 = 999
	b.Machine.LoadEnd = [3]float64{8, 9, 10}
	before, _ := json.Marshal([]*Report{a, b})
	got, err := Merge(a, b)
	if err != nil {
		t.Fatal(err)
	}
	r, sc := got.Results[0], got.Results[0].Scenario(ScHover)
	if len(got.Results) != 1 || len(sc.Runs) != 3 || len(sc.Samples) != 3 || sc.Agg.Runs != 3 || sc.Agg.P50 != 2 || sc.Agg.P95 != 20 {
		t.Fatalf("merged scenario: %+v", sc)
	}
	// Not a median of medians, nor a median falsely labelled peak.
	if r.RSS != 200 || r.PeakRSS != 330 || len(r.Notes) != 1 || len(r.StatsRuns) != 3 {
		t.Fatalf("merged memory/stats: %+v", r)
	}
	if len(got.Sources) != 2 || got.Sources[0].Machine.Date != "a" || !sameJSON(got.Sources[1], b) {
		t.Fatal("lost raw provenance")
	}
	if got.Machine.LoadEnd != b.Machine.LoadEnd || strings.Contains(strings.Join(got.Machine.Settings, " "), "(alternating)") {
		t.Fatal("invented execution method")
	}
	var encoded bytes.Buffer
	if err := got.WriteJSON(&encoded); err != nil {
		t.Fatal(err)
	}
	round, err := ReadJSON(&encoded)
	if err != nil || !sameJSON(got, round) {
		t.Fatalf("JSON round trip: %v", err)
	}
	got.Results[0].Scenarios[0].Samples[0][0] = 100
	got.Sources[1].Results[0].Identity.Env = append(got.Sources[1].Results[0].Identity.Env, "CHANGED=1")
	after, _ := json.Marshal([]*Report{a, b})
	if !bytes.Equal(before, after) {
		t.Fatal("merge aliases or mutates the input")
	}
}

func TestMergeComplementaryTargetsAndFailures(t *testing.T) {
	a, b := mergeFixture("a", "gocpp", 1), mergeFixture("b", "gopls", 2)
	b.Results[0].Identity.Ext = ".go"
	b.Machine.Gopls = "gopls v0.21.1"
	s := &a.Results[0].Scenarios[0]
	s.Samples[0], s.Runs[0], s.Fail, s.Err = nil, Summary{}, 3, "timeout"
	got, err := Merge(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 2 || got.Results[0].Scenario(ScHover).Fail != 3 || got.Results[0].Scenario(ScHover).Agg.Runs != 0 || got.Machine.Gopls != b.Machine.Gopls {
		t.Fatal("lost failed run or complementary target")
	}
	// Paths can differ while the actual binary and arguments are identical.
	c := mergeFixture("c", "gocpp", 2)
	c.Results[0].Identity.Command[0] = "/other/build/gocpp"
	if _, err := Merge(a, c); err != nil {
		t.Fatal(err)
	}
}

func TestMergeRejectsIncomparable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Report)
	}{
		{"legacy", func(r *Report) { r.Protocol = nil }},
		{"duplicate invocation", func(r *Report) { r.ID = "a" }},
		{"merged report", func(r *Report) { r.Sources = []*Report{{}} }},
		{"method", func(r *Report) { r.Protocol.Method = "reused-edit-text" }},
		{"go language project", func(r *Report) { r.Results[0].Identity.ProjectSHA256 = "go1.22" }},
		{"toolchain", func(r *Report) { r.Protocol.GoCommand = "go version go1.26" }},
		{"harness toolchain", func(r *Report) { r.Machine.Go = "go1.26" }},
		{"CPU", func(r *Report) { r.Machine.CPU = "other" }},
		{"pinning", func(r *Report) { r.Protocol.Affinity = "16-23" }},
		{"runtime environment", func(r *Report) { r.Protocol.Environment = []string{"GOMEMLIMIT=100MiB"} }},
		{"old phase", func(r *Report) { r.Results[0].Identity.Env = []string{"GOCPP_GATE_PHASE=8"} }},
		{"binary", func(r *Report) { r.Results[0].Identity.BinarySHA256 = "other-build" }},
		{"command arguments", func(r *Report) { r.Results[0].Identity.Command = append(r.Results[0].Identity.Command, "-no-warm") }},
		{"delay", func(r *Report) { r.Results[0].Identity.Settings = json.RawMessage(`{"diagnosticsDelay":"1s"}`) }},
		{"scale", func(r *Report) { r.Protocol.Scale = .5 }},
		{"only", func(r *Report) { r.Protocol.Only = []string{ScHover} }},
		{"timeout", func(r *Report) { r.Protocol.Timeout = time.Second }},
		{"project size", func(r *Report) { r.Results[0].Lines++ }},
		{"missing scenario", func(r *Report) { r.Results[0].Scenarios = nil }},
		{"duplicate scenario", func(r *Report) { r.Results[0].Scenarios = append(r.Results[0].Scenarios, r.Results[0].Scenarios[0]) }},
		{"duplicate result", func(r *Report) { r.Results = append(r.Results, r.Results[0]) }},
		{"missing raw samples", func(r *Report) { r.Results[0].Scenarios[0].Samples = nil }},
		{"false summary", func(r *Report) { r.Results[0].Scenarios[0].Runs[0].P95 = .001 }},
		{"negative sample", func(r *Report) { r.Results[0].Scenarios[0].Samples[0][0] = -1 }},
		{"missing memory", func(r *Report) { r.Results[0].RSSRuns = nil }},
		{"invalid peak", func(r *Report) { r.Results[0].PeakRSSRuns[0] = 0 }},
		{"missing stats", func(r *Report) { r.Results[0].StatsRuns = nil }},
		{"false failed count", func(r *Report) { r.Results[0].Scenarios[0].Fail = -1 }},
		{"lost failure", func(r *Report) { r.Results[0].Scenarios[0].Fail = 1 }},
		{"unknown target", func(r *Report) { r.Results[0].Target = "baseline" }},
		{"nil result", func(r *Report) { r.Results[0] = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := mergeFixture("a", "gocpp", 1), mergeFixture("b", "gocpp", 2)
			tc.mutate(b)
			if _, err := Merge(a, b); err == nil {
				t.Fatal("accepted incomparable or corrupt input")
			}
		})
	}
	for _, reps := range [][]*Report{nil, {nil}, {mergeFixture("a", "gocpp", 1), nil}} {
		if _, err := Merge(reps...); err == nil {
			t.Fatal("accepted empty input")
		}
	}
	// Matching is not only per (project,target): the same project across
	// targets must have the same source, and the same target across projects
	// must be the same build/configuration.
	a, b := mergeFixture("a", "gocpp", 1), mergeFixture("b", "gopls", 1)
	b.Results[0].Identity.ProjectSHA256 = "other"
	if _, err := Merge(a, b); err == nil {
		t.Fatal("accepted different source across targets")
	}
	b = mergeFixture("b", "gocpp", 1)
	b.Results[0].Project = "medium"
	b.Results[0].Identity.BinarySHA256 = "other"
	if _, err := Merge(a, b); err == nil {
		t.Fatal("accepted different binary across projects")
	}
}

func TestConfigurationFingerprints(t *testing.T) {
	const secret = "SYNTHETIC_ONLY_CONFIG_CREDENTIAL"
	t.Setenv("GOPROXY", "https://review-user:"+secret+"@proxy.invalid?token="+secret)
	t.Setenv("GOAUTH", "auth-command --token="+secret)
	t.Setenv("GOCPP_TOKEN", secret)
	target := Target{
		Name: "gocpp",
		Ext:  ".gp",
		Command: []string{
			"go", "version", "--url=https://user:" + secret + "@host.invalid?token=" + secret,
		},
		Env:      []string{"GOCPP_TOKEN=" + secret},
		Settings: json.RawMessage(`{"authorization":"` + secret + `"}`),
	}
	before, _ := json.Marshal([]any{target.Command, target.Env, target.Settings})
	identity, err := identifyResult(Generate(Small), target)
	if err != nil {
		t.Fatal(err)
	}
	first := mergeFixture("a", "gocpp", 1)
	second := mergeFixture("b", "gocpp", 2)
	for _, report := range []*Report{first, second} {
		report.Protocol = NewReport(Config{Runs: 1}).Protocol
		report.Results[0].Identity = identity
	}

	t.Run("same configurations merge without publishing values", func(t *testing.T) {
		merged, err := Merge(first, second)
		if err != nil {
			t.Fatal(err)
		}
		for _, report := range []*Report{first, merged} {
			encoded, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			var markdown bytes.Buffer
			report.Markdown(&markdown)
			if bytes.Contains(encoded, []byte(secret)) || strings.Contains(markdown.String(), secret) {
				t.Fatal("serialized raw synthetic configuration credential")
			}
		}
	})
	t.Run("actual server configuration is unchanged", func(t *testing.T) {
		got, _ := json.Marshal([]any{target.Command, target.Env, target.Settings})
		if !bytes.Equal(before, got) {
			t.Fatal("fingerprinting changed the actual server configuration")
		}
	})
	t.Run("global environment drift", func(t *testing.T) {
		t.Setenv("GOAUTH", "different-"+secret)
		other := mergeFixture("c", "gocpp", 1)
		other.Protocol = NewReport(Config{Runs: 1}).Protocol
		other.Results[0].Identity = identity
		if _, err := Merge(first, other); err == nil {
			t.Fatal("credential/configuration drift was not distinguished")
		}
	})
	for _, kind := range []string{"environment", "argument", "settings"} {
		t.Run("target drift/"+kind, func(t *testing.T) {
			changed := target
			switch kind {
			case "environment":
				changed.Env = []string{"GOCPP_TOKEN=different-" + secret}
			case "argument":
				changed.Command = []string{"go", "version", "different-" + secret}
			case "settings":
				changed.Settings = json.RawMessage(`{"authorization":"different-` + secret + `"}`)
			}
			changedIdentity, err := identifyResult(Generate(Small), changed)
			if err != nil {
				t.Fatal(err)
			}
			other := mergeFixture("c", "gocpp", 1)
			other.Protocol = first.Protocol
			other.Results[0].Identity = changedIdentity
			if _, err := Merge(first, other); err == nil {
				t.Fatal("target fingerprints lost configuration drift")
			}
		})
	}
	t.Run("domains are separated", func(t *testing.T) {
		if configFingerprint("argument", secret) == configFingerprint("settings", secret) {
			t.Fatal("configuration fingerprints are not domain-separated")
		}
	})
	for _, kind := range []string{"environment", "argument", "settings", "legacy"} {
		t.Run("reject raw configuration/"+kind, func(t *testing.T) {
			other := mergeFixture("c", "gocpp", 1)
			other.Protocol = first.Protocol
			other.Results[0].Identity = &ResultIdentity{}
			*other.Results[0].Identity = *identity
			switch kind {
			case "environment":
				other.Results[0].Identity.Env = []string{"GOCPP_TOKEN=" + secret}
			case "argument":
				other.Results[0].Identity.Command = []string{"go", "--token=" + secret}
			case "settings":
				other.Results[0].Identity.Settings = target.Settings
			case "legacy":
				protocol := *first.Protocol
				protocol.ConfigurationFormat = ""
				protocol.Environment = []string{"GOPROXY=https://user:" + secret + "@host.invalid"}
				other.Protocol = &protocol
			}
			if _, err := Merge(other); err == nil {
				t.Fatal("accepted raw or legacy configuration for republication")
			} else if strings.Contains(err.Error(), secret) {
				t.Fatal("merge error exposed a raw configuration credential")
			}
		})
	}
}

func TestResultIdentity(t *testing.T) {
	p := Generate(Small)
	tgt := Target{Name: "gocpp", Ext: ".gp", Command: []string{"go", "version"}}
	a, err := identifyResult(p, tgt)
	if err != nil {
		t.Fatal(err)
	}
	p.Files[0].Content += "\n// changed\n"
	b, err := identifyResult(p, tgt)
	if err != nil {
		t.Fatal(err)
	}
	if a.ProjectSHA256 == b.ProjectSHA256 || a.BinarySHA256 != b.BinarySHA256 {
		t.Fatal("identity does not follow bytes")
	}
	cfg := Config{Runs: 1, Only: []string{ScHover, ScKeystrokeNew}}
	r := NewReport(cfg)
	if r.ID == "" || r.Protocol.Method == "" || r.Protocol.GoCommand == "" || r.Protocol.Runs != 1 {
		t.Fatal("missing invocation provenance")
	}
	if cfg.Only[0] != ScHover {
		t.Fatal("mutated caller's config")
	}
}
