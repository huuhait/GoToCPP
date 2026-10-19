package tests

// TestLLDBPrinters: runtime/debug/gocpp_lldb.py in scripted lldb sessions (best effort: gdb has
// the full support). A session is tests/gdb/NAME.lldb: lldb commands with #want lines, like the
// optimized gdb sessions.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLLDBPrinters(t *testing.T) {
	c := setup(t)
	lldb, err := exec.LookPath("lldb")
	if err != nil {
		t.Skipf("lldb is not installed (the dev shell, nix develop, has it): %v", err)
	}
	files, _ := filepath.Glob(filepath.Join(repoRoot, "tests", "gdb", "*.lldb"))
	if len(files) == 0 {
		t.Fatal("no lldb sessions in tests/gdb")
	}
	for _, sessionFile := range files {
		name := strings.TrimSuffix(filepath.Base(sessionFile), ".lldb")
		t.Run(name, func(t *testing.T) {
			dir := filepath.Dir(sessionFile)
			file := filepath.Join(dir, name+".gp")
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			m := gdbPhaseRx.FindSubmatch(src)
			if m == nil {
				t.Fatalf("%s has no // phase: line", file)
			}
			phase, _ := strconv.Atoi(string(m[1]))
			if why := phaseGate(phase, c.phase, c.phaseFrom); why != "" {
				t.Skip(why)
			}
			tags := map[string]int{}
			for i, line := range strings.Split(string(src), "\n") {
				if m := gdbTagRx.FindStringSubmatch(line); m != nil {
					tags[m[1]] = i + 1
				}
			}
			script, err := readGDBScript(sessionFile, tags)
			if err != nil {
				t.Fatal(err)
			}
			c.forEachCompiler(t, func(t *testing.T, cc compiler) {
				bin := filepath.Join(t.TempDir(), name+exe)
				r := c.run(t, gocppJob(dir, nil, cc.cxx, phase, "build", "-o", bin, filepath.Base(file)))
				if r.problem != "" || r.exit != 0 {
					t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.why(), indent(string(r.stderr)))
				}
				args := []string{"--batch", "--no-lldbinit", "-o", "command script import " + filepath.Join(repoRoot, "runtime", "debug", "gocpp_lldb.py")}
				for i, cmd := range script.cmds {
					args = append(args, "-o", fmt.Sprintf("script print(%q)", gdbStepMark(i)), "-o", cmd.text)
				}
				args = append(args, "--", bin)
				show := "lldb"
				for _, a := range args {
					show += " " + shellQuote(a)
				}
				res := c.run(t, job{dir: t.TempDir(), env: []string{"LC_ALL=C.UTF-8", "PYTHONIOENCODING=utf-8"}, name: lldb, args: args, show: show, combined: true})
				if res.problem != "" {
					t.Fatalf("%s: %s (%s)\noutput:\n%s", res.cmd, res.problem, res.status, indent(string(res.stdout)))
				}
				out := string(res.stdout)
				if strings.Contains(out, "process launch failed") || strings.Contains(out, "Operation not permitted") {
					t.Skipf("lldb can't trace programs here:\n%s", indent(out))
				}
				parts := splitGDBOutput(out, len(script.cmds))
				for i, cmd := range script.cmds {
					t.Run(fmt.Sprintf("%02d %s", i, cmd.text), func(t *testing.T) {
						got := cmd.output(parts[i])
						for _, rx := range cmd.want {
							if !rx.MatchString(got) {
								t.Errorf("%s:%d: the output of %q doesn't match %s:\n%s\nThe session:\n\t%s", sessionFile, cmd.line, cmd.text, rx, indent(got), res.cmd)
							}
						}
					})
				}
			})
		})
	}
}
