package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/bench"
)

// Run the actual CLI in a child so its flags and exit status are tested without
// measuring a server or changing the parent test process's global flags.
func TestLSPBenchCLIHelper(t *testing.T) {
	if os.Getenv("GOCPP_LSPBENCH_CLI_TEST") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"lspbench-cli-test"}, os.Args[i+1:]...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			os.Exit(run())
		}
	}
	t.Fatal("missing CLI argument separator")
}

func TestMergeReportsGateCLI(t *testing.T) {
	for _, tc := range []struct {
		name     string
		p95      float64
		gate     bool
		stdout   bool
		wantCode int
	}{
		{"passing merged report", 2, true, false, 0},
		{"failing merged report", 1000000, true, false, 1},
		{"failing merged report on stdout", 1000000, true, true, 1},
		{"ungated over-budget merged report", 1000000, false, false, 0},
		{"gate safety tolerance is retained", 20, true, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			var inputs []string
			for _, name := range []string{"first", "second"} {
				report := bench.NewReport(bench.Config{Runs: 1})
				samples := []float64{tc.p95}
				report.Results = []*bench.ProjectResult{{
					Project: "small",
					Target:  "gocpp",
					Files:   1,
					Lines:   10,
					Identity: &bench.ResultIdentity{
						ProjectSHA256: "synthetic-project",
						BinarySHA256:  "synthetic-binary",
						Command:       []string{"/synthetic-gocpp"},
						Ext:           ".gp",
					},
					RSSRuns:     []float64{10},
					PeakRSSRuns: []float64{20},
					StatsRuns:   []json.RawMessage{nil},
					Scenarios: []bench.ScenarioResult{{
						Name:    bench.ScHover,
						Runs:    []bench.Summary{bench.Summarize(samples)},
						Samples: [][]float64{samples},
					}},
				}}
				input := filepath.Join(dir, name+".json")
				if err := writeFile(input, report.WriteJSON); err != nil {
					t.Fatal(err)
				}
				inputs = append(inputs, input)
			}

			merged := filepath.Join(dir, "merged.json")
			markdown := filepath.Join(dir, "merged.md")
			args := []string{
				"-test.run=^TestLSPBenchCLIHelper$", "--",
				"-merge", strings.Join(inputs, ","), "-json", merged,
			}
			if !tc.stdout {
				args = append(args, "-md", markdown)
			}
			if tc.gate {
				args = append(args, "-gate")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], args...)
			command.Env = append(os.Environ(), "GOCPP_LSPBENCH_CLI_TEST=1")
			output, err := command.CombinedOutput()
			code := 0
			if err != nil {
				exitError, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				code = exitError.ExitCode()
			}
			if ctx.Err() != nil || code != tc.wantCode {
				t.Fatalf("CLI exit %d, want %d: %s", code, tc.wantCode, output)
			}
			if got := bytes.Contains(output, []byte("lspbench: budget gate:")); got != (code == 1) {
				t.Fatalf("budget-gate diagnostics disagree with exit status: %s", output)
			}

			// A gate failure still writes the merged evidence, like a measured run.
			file, err := os.Open(merged)
			if err != nil {
				t.Fatal(err)
			}
			report, err := bench.ReadJSON(file)
			file.Close()
			if err != nil || len(report.Sources) != 2 {
				t.Fatalf("lost merged provenance: %v", err)
			}
			scenario := report.Results[0].Scenario(bench.ScHover)
			if scenario == nil || scenario.Agg.P95 != tc.p95 || scenario.Agg.Runs != 2 {
				t.Fatal("gate changed the merged samples")
			}
			if !tc.stdout {
				if data, err := os.ReadFile(markdown); err != nil || len(data) == 0 {
					t.Fatal("merged Markdown evidence was not written")
				}
			}
		})
	}
}

func TestReportSettingsOmitConfigurationValues(t *testing.T) {
	const secret = "SYNTHETIC_ONLY_FLAG_CREDENTIAL"
	settings := reportSettings(1, 1, secret, "GOCPP_TOKEN="+secret)
	if strings.Contains(strings.Join(settings, "\n"), secret) {
		t.Fatal("human-readable settings exposed an arbitrary configuration value")
	}
}

func TestFromReportRejectsLegacyConfiguration(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "legacy.json")
	out := filepath.Join(dir, "out.md")
	rep := bench.NewReport(bench.Config{Runs: 1})
	rep.Protocol.ConfigurationFormat = ""
	rep.Machine.Settings = []string{"GOAUTH=SYNTHETIC_ONLY_LEGACY_CREDENTIAL"}
	if err := writeFile(in, rep.WriteJSON); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, []byte("keep output"), 0600); err != nil {
		t.Fatal(err)
	}
	if rc := fromReport(in, out); rc == 0 {
		t.Fatal("rendered legacy raw configuration")
	}
	if got, err := os.ReadFile(out); err != nil || string(got) != "keep output" {
		t.Fatal("legacy rejection touched the output")
	}
}

func TestMergeReports(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, r *bench.Report) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := writeFile(path, r.WriteJSON); err != nil {
			t.Fatal(err)
		}
		return path
	}
	a := bench.NewReport(bench.Config{Runs: 1})
	a.Results = []*bench.ProjectResult{{Project: "small", Target: "gocpp", Files: 1, Lines: 10,
		Identity: &bench.ResultIdentity{ProjectSHA256: "project", BinarySHA256: "binary", Command: []string{"/gocpp"}, Ext: ".gp"},
		RSSRuns:  []float64{10}, PeakRSSRuns: []float64{20}, StatsRuns: []json.RawMessage{nil},
		Scenarios: []bench.ScenarioResult{{Name: bench.ScHover, Runs: []bench.Summary{bench.Summarize([]float64{2})}, Samples: [][]float64{{2}}}}}}
	b := bench.NewReport(bench.Config{Runs: 1})
	b.Results = a.Results
	first, second := write("a.json", a), write("b.json", b)
	out, md := filepath.Join(dir, "merged.json"), filepath.Join(dir, "merged.md")
	if rc := mergeReports(first+", "+second, out, md, false); rc != 0 {
		t.Fatalf("merge rc=%d", rc)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	r, err := bench.ReadJSON(f)
	f.Close()
	if err != nil || len(r.Sources) != 2 || r.Results[0].Scenario(bench.ScHover).Agg.Runs != 2 {
		t.Fatalf("merged output: %v", err)
	}
	text, err := os.ReadFile(md)
	if err != nil || !strings.Contains(string(text), "2 process runs") || strings.Contains(string(text), "(alternating), merged") {
		t.Fatalf("misleading or missing provenance: %v", err)
	}
	before, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	// Duplicate source invocations and a mixed build must fail before touching
	// an existing output. -merge cannot invent another independent run.
	if rc := mergeReports(first+","+first, out, md, false); rc == 0 {
		t.Fatal("accepted duplicate invocation")
	}
	b.Results[0].Identity.BinarySHA256 = "other"
	second = write("other.json", b)
	if rc := mergeReports(first+","+second, out, md, false); rc == 0 {
		t.Fatal("accepted mixed binary cohort")
	}
	after, _ := os.ReadFile(out)
	if !bytes.Equal(before, after) {
		t.Fatal("invalid merge replaced existing output")
	}
}
