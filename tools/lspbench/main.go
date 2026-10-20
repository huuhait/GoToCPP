// Command lspbench is the benchmark harness of the gocpp language server
// (Phase 11, step 11.11): it generates the reference projects (never
// checked in), replays the scripted editor scenarios against "gocpp lsp"
// and, when it is on the PATH, gopls on the same code, and writes a JSON
// report and the markdown table of docs/benchmarks/lsp.md.
//
//	go build -o bin/gocpp ./cmd/gocpp
//	go run ./tools/lspbench -gocpp bin/gocpp -json out.json -md docs/benchmarks/lsp.md
//
// "make lsp-bench" does this. Run it on a quiet machine, one run at a time:
// the figures are medians over -runs sessions, and a loaded machine still
// moves them (the report says the load average before and after).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/bench"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		projects  = flag.String("projects", "small,medium", "the reference projects to measure, comma-separated: small, medium, large, cpp (gocpp only)")
		targets   = flag.String("targets", "gocpp,gopls", "the servers to measure, comma-separated (gopls only when it is on the PATH)")
		gocpp     = flag.String("gocpp", "", "the gocpp binary (default: gocpp on the PATH)")
		gopls     = flag.String("gopls", "", "the gopls binary (default: gopls on the PATH)")
		runs      = flag.Int("runs", 5, "how many times each session runs; the medians count")
		scale     = flag.Float64("scale", 1, "multiplies the repetitions of each scenario")
		jsonOut   = flag.String("json", "", "write the report as JSON to `file`")
		mdOut     = flag.String("md", "", "write the markdown tables to `file` (docs/benchmarks/lsp.md)")
		workdir   = flag.String("workdir", "", "where the projects and caches go (default: a temporary folder, removed at the end)")
		delay     = flag.String("gopls-delay", "30ms", "gopls's diagnosticsDelay (its default is 1s, which is a wait, not work); \"\" leaves the default")
		gate      = flag.Bool("gate", false, "check the speed budgets and exit with status 1 when a row fails the gate")
		maxLoad   = flag.Float64("max-load", 30, "do not start when the 1-minute load average is over `n` (0: no check)")
		gocppEnv  = flag.String("gocpp-env", "GOCPP_GATE_PHASE=15", "environment variables for gocpp lsp, comma-separated (the generated code uses what the phases add)")
		gocppArgs = flag.String("gocpp-args", "", "more arguments for gocpp lsp, space-separated (-profile dir, -trace file, ...)")
		merge     = flag.String("merge", "", "join comparable reports in these `files` (made by -json, comma-separated; preserve raw provenance, do not infer alternation); measure nothing")
		fromJSON  = flag.String("from-json", "", "write the -md tables from the report in `file` (made by -json) and measure nothing")
		only      = flag.String("only", "", "run only the session scenarios in this comma-separated list (for profiling one of them)")
		timeout   = flag.Duration("timeout", time.Minute, "the longest wait for one answer or one publish of diagnostics")
		verbose   = flag.Bool("v", false, "log the progress")
	)
	flag.Parse()

	if *merge != "" {
		return mergeReports(*merge, *jsonOut, *mdOut, *gate)
	}
	if *fromJSON != "" {
		return fromReport(*fromJSON, *mdOut)
	}
	if load := bench.LoadAvg()[0]; *maxLoad > 0 && load > *maxLoad {
		fmt.Fprintf(os.Stderr, "lspbench: the load average is %.1f, over %.0f: the figures would be noise. Wait, or pass -max-load 0.\n", load, *maxLoad)
		return 3
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var tgts []bench.Target
	for _, name := range strings.Split(*targets, ",") {
		switch strings.TrimSpace(name) {
		case "gocpp":
			path := *gocpp
			if path == "" {
				var err error
				if path, err = exec.LookPath("gocpp"); err != nil {
					fmt.Fprintln(os.Stderr, "lspbench: no gocpp binary: pass -gocpp")
					return 2
				}
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				fmt.Fprintln(os.Stderr, "lspbench:", err)
				return 2
			}
			t := bench.Target{Name: "gocpp", Ext: ".gp", Command: append([]string{abs, "lsp"}, strings.Fields(*gocppArgs)...)}
			if *gocppEnv != "" {
				t.Env = strings.Split(*gocppEnv, ",")
			}
			tgts = append(tgts, t)
		case "gopls":
			path := *gopls
			if path == "" {
				path, _ = exec.LookPath("gopls")
			}
			if path == "" {
				fmt.Fprintln(os.Stderr, "lspbench: gopls is not on the PATH: measuring gocpp alone")
				continue
			}
			t := bench.Target{Name: "gopls", Ext: ".go", Command: []string{path}}
			if *delay != "" {
				t.Settings = json.RawMessage(fmt.Sprintf(`{"diagnosticsDelay":%q}`, *delay))
			}
			tgts = append(tgts, t)
		default:
			fmt.Fprintf(os.Stderr, "lspbench: unknown server %q\n", name)
			return 2
		}
	}
	if len(tgts) == 0 {
		fmt.Fprintln(os.Stderr, "lspbench: no server to measure")
		return 2
	}

	dir := *workdir
	if dir == "" {
		var err error
		if dir, err = os.MkdirTemp("", "lspbench-"); err != nil {
			fmt.Fprintln(os.Stderr, "lspbench:", err)
			return 2
		}
		defer os.RemoveAll(dir)
	}
	cfg := bench.Config{Runs: *runs, Scale: *scale, WorkDir: dir, Timeout: *timeout}
	if *only != "" {
		cfg.Only = strings.Split(*only, ",")
	}
	if *verbose {
		cfg.Log = func(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) }
	}

	rep := bench.NewReport(cfg)
	for _, t := range tgts {
		switch t.Name {
		case "gocpp":
			rep.Machine.Gocpp = bench.CommandOutput(t.Command[0], "version")
		case "gopls":
			rep.Machine.Gopls = bench.CommandOutput(t.Command[0], "version")
			if i := strings.Index(rep.Machine.Gopls, "\n"); i >= 0 {
				rep.Machine.Gopls = rep.Machine.Gopls[:i]
			}
		}
	}
	rep.Machine.Settings = reportSettings(*runs, *scale, *delay, *gocppEnv)

	for _, name := range strings.Split(*projects, ",") {
		spec, ok := bench.SpecByName(strings.TrimSpace(name))
		if !ok {
			fmt.Fprintf(os.Stderr, "lspbench: unknown project %q\n", name)
			return 2
		}
		proj := bench.Generate(spec)
		for _, t := range tgts {
			fmt.Fprintf(os.Stderr, "lspbench: %s on %s (%d files, %d lines)...\n", t.Name, spec.Name, len(proj.Files), proj.Lines)
			res, err := bench.RunProject(ctx, cfg, proj, t)
			if err != nil {
				fmt.Fprintf(os.Stderr, "lspbench: %v\n", err)
				if res == nil || len(res.Scenarios) == 0 {
					return 1
				}
			}
			rep.Results = append(rep.Results, res)
			if ctx.Err() != nil {
				return 130
			}
		}
	}
	rep.Machine.LoadEnd = bench.LoadAvg()

	if *jsonOut != "" {
		if err := writeFile(*jsonOut, rep.WriteJSON); err != nil {
			fmt.Fprintln(os.Stderr, "lspbench:", err)
			return 1
		}
	}
	if *mdOut != "" {
		if err := writeFile(*mdOut, func(w io.Writer) error { rep.Markdown(w); return nil }); err != nil {
			fmt.Fprintln(os.Stderr, "lspbench:", err)
			return 1
		}
	}
	if *jsonOut == "" && *mdOut == "" {
		rep.Markdown(os.Stdout)
	}
	return reportExitCode(rep, *gate)
}

// Measured and merged results use the same gate, including its noise tolerances
// and failed-repetition policy. Writing the evidence does not imply gate success.
func reportExitCode(rep *bench.Report, gate bool) int {
	if !gate {
		return 0
	}
	violations := bench.Gate(bench.Rows(rep.Results), bench.DefaultGate)
	for _, violation := range violations {
		fmt.Fprintln(os.Stderr, "lspbench: budget gate:", violation)
	}
	if len(violations) > 0 {
		return 1
	}
	return 0
}

// reportSettings describes configuration without echoing arbitrary flag values.
// The comparable value identities live in the report's fingerprinted metadata.
func reportSettings(runs int, scale float64, delay, gocppEnv string) []string {
	settings := []string{
		fmt.Sprintf("%d runs per scenario, repetitions x%g; medians of the runs", runs, scale),
	}
	if delay != "" {
		settings = append(settings,
			"gopls diagnosticsDelay explicitly configured; value fingerprint in JSON "+
				"(gopls's default is 1s)")
	}
	if gocppEnv != "" {
		settings = append(settings,
			"gocpp lsp environment: values omitted; named configuration fingerprints in JSON "+
				"(not secret protection)")
	}
	return settings
}

func writeFile(path string, f func(io.Writer) error) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := f(out); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fromReport writes the markdown of the report in file to md (or standard
// output).
func fromReport(file, md string) int {
	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lspbench:", err)
		return 1
	}
	defer f.Close()
	rep, err := bench.ReadJSON(f)
	if err == nil {
		err = rep.CheckConfigurationProvenance()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lspbench:", err)
		return 1
	}
	if md == "" {
		rep.Markdown(os.Stdout)
		return 0
	}
	if err := writeFile(md, func(w io.Writer) error { rep.Markdown(w); return nil }); err != nil {
		fmt.Fprintln(os.Stderr, "lspbench:", err)
		return 1
	}
	return 0
}

// mergeReports joins the reports in files (bench.Merge) and writes the JSON
// and the markdown.
func mergeReports(files, jsonOut, md string, gate bool) int {
	var reps []*bench.Report
	for _, name := range strings.Split(files, ",") {
		name = strings.TrimSpace(name)
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintln(os.Stderr, "lspbench:", err)
			return 1
		}
		rep, err := bench.ReadJSON(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "lspbench: %s: %v\n", name, err)
			return 1
		}
		reps = append(reps, rep)
	}
	rep, err := bench.Merge(reps...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lspbench:", err)
		return 1
	}
	if jsonOut != "" {
		if err := writeFile(jsonOut, rep.WriteJSON); err != nil {
			fmt.Fprintln(os.Stderr, "lspbench:", err)
			return 1
		}
	}
	if md == "" {
		rep.Markdown(os.Stdout)
	} else {
		if err := writeFile(md, func(w io.Writer) error {
			rep.Markdown(w)
			return nil
		}); err != nil {
			fmt.Fprintln(os.Stderr, "lspbench:", err)
			return 1
		}
	}
	return reportExitCode(rep, gate)
}
