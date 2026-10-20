//go:build unix

package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type prog struct {
	spec   Spec
	script string
	stdin  string
}

// batch writes NAME.tgz in root: the manifest and the scripts.
func batch(t *testing.T, root, name string, jobs int, maxOut int64, progs ...prog) {
	t.Helper()
	m := Manifest{Jobs: jobs, MaxOutput: maxOut}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	add := func(n string, data []byte) {
		tw.WriteHeader(&tar.Header{Name: n, Mode: 0o755, Size: int64(len(data))})
		tw.Write(data)
	}
	for _, p := range progs {
		m.Tests = append(m.Tests, p.spec)
	}
	data, _ := json.Marshal(m)
	add(ManifestName, data)
	for _, p := range progs {
		add("exe/"+itoa(p.spec.ID)+".exe", []byte("#!/bin/sh\n"+p.script+"\n"))
		if p.spec.Stdin {
			add("stdin/"+itoa(p.spec.ID), []byte(p.stdin))
		}
	}
	tw.Close()
	zw.Close()
	if err := os.WriteFile(filepath.Join(root, name+".tgz"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func mainIn(t *testing.T, root string, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := MainIn(context.Background(), root, args, &out, &errb)
	return out.String(), errb.String(), code
}

func readResult(t *testing.T, root, name string) (map[int]Line, map[string]string) {
	t.Helper()
	f, err := os.Open(filepath.Join(root, name+"-res.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(zr)
	files := map[string]string{}
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		var b bytes.Buffer
		b.ReadFrom(tr)
		files[h.Name] = b.String()
	}
	lines := map[int]Line{}
	for _, l := range strings.Split(files[ResultsName], "\n") {
		var ln Line
		if json.Unmarshal([]byte(l), &ln) == nil && l != "" {
			lines[ln.ID] = ln
		}
	}
	return lines, files
}

func TestAgentRunsAProgramInItsOwnFolders(t *testing.T) {
	root := t.TempDir()
	batch(t, root, "b1", 2, 1<<20,
		prog{spec: Spec{ID: 1, Name: "env", Env: []string{"WIN_VAR=a b"}, TimeoutMs: 10000}, script: `echo "$WIN_VAR"; echo "$1" "$2"; pwd; echo "$TEMP"; echo err >&2; exit 7`},
		prog{spec: Spec{ID: 2, Name: "stdin", Stdin: true, TimeoutMs: 10000}, script: `cat`, stdin: "typed\n"},
		prog{spec: Spec{ID: 3, Name: "args", Args: []string{"x y", "z"}, TimeoutMs: 10000}, script: `echo "$1|$2"`},
	)
	out, errb, code := mainIn(t, root, "run", "b1")
	if code != 0 || !strings.Contains(out, "tests=3 ran=3 resumed=0") {
		t.Fatalf("exit %d\n%s\n%s", code, out, errb)
	}
	lines, files := readResult(t, root, "b1")
	if lines[1].Exit != 7 || lines[2].Exit != 0 || lines[3].Exit != 0 {
		t.Errorf("exits %+v", lines)
	}
	got := strings.Split(strings.TrimSpace(files["1.out"]), "\n")
	if got[0] != "a b" {
		t.Errorf("env: %q", got)
	}
	if !strings.HasSuffix(got[2], filepath.Join("b1", "work", "1")) {
		t.Errorf("working folder %q", got[2])
	}
	if !strings.HasSuffix(got[3], filepath.Join("b1", "tmp", "1")) {
		t.Errorf("TEMP %q", got[3])
	}
	if files["1.err"] != "err\n" || files["2.out"] != "typed\n" || files["3.out"] != "x y|z\n" {
		t.Errorf("outputs: %q %q %q", files["1.err"], files["2.out"], files["3.out"])
	}
	// The agent deletes nothing; the driver does.
	if _, err := os.Stat(filepath.Join(root, "b1", "res", ResultsName)); err != nil {
		t.Error(err)
	}
}

func TestAgentResumesFromTheResultsFile(t *testing.T) {
	root := t.TempDir()
	mark := filepath.Join(root, "ran")
	batch(t, root, "b", 1, 1<<20,
		prog{spec: Spec{ID: 1, Name: "one", TimeoutMs: 10000}, script: `echo 1 >> ` + mark + `; echo one`},
		prog{spec: Spec{ID: 2, Name: "two", TimeoutMs: 10000}, script: `echo 2 >> ` + mark + `; echo two`},
	)
	if _, e, code := mainIn(t, root, "run", "b"); code != 0 {
		t.Fatal(e)
	}
	// A run that was stopped after the first program: the second line is gone, and the last line is cut.
	resFile := filepath.Join(root, "b", "res", ResultsName)
	data, _ := os.ReadFile(resFile)
	first := strings.SplitN(string(data), "\n", 2)[0]
	os.WriteFile(resFile, []byte(first+"\n{\"id\":2,\"na"), 0o644)
	os.Remove(filepath.Join(root, "b-res.tgz"))
	os.WriteFile(mark, nil, 0o644)
	out, e, code := mainIn(t, root, "run", "b")
	if code != 0 || !strings.Contains(out, "ran=1 resumed=1") {
		t.Fatalf("%d %s %s", code, out, e)
	}
	ran, _ := os.ReadFile(mark)
	var l1 Line
	json.Unmarshal([]byte(first), &l1)
	want := "2\n"
	if l1.ID == 2 {
		want = "1\n"
	}
	if string(ran) != want {
		t.Errorf("programs run the second time: %q, want %q", ran, want)
	}
	lines, _ := readResult(t, root, "b")
	if len(lines) != 2 {
		t.Errorf("lines %v", lines)
	}
}

func TestAgentLimitsOutputAndTime(t *testing.T) {
	root := t.TempDir()
	batch(t, root, "b", 4, 32<<10,
		prog{spec: Spec{ID: 1, Name: "flood", TimeoutMs: 20000}, script: `yes abcdefghijklmnopqrstuvwxyz`},
		prog{spec: Spec{ID: 2, Name: "errflood", TimeoutMs: 20000}, script: `yes abcdefghijklmnopqrstuvwxyz >&2`},
		prog{spec: Spec{ID: 3, Name: "hang", TimeoutMs: 700}, script: `echo before; sleep 300`},
		prog{spec: Spec{ID: 4, Name: "missing", TimeoutMs: 5000}, script: `exit 0`},
	)
	start := time.Now()
	if _, e, code := mainIn(t, root, "run", "b"); code != 0 {
		t.Fatal(e)
	}
	if time.Since(start) > 15*time.Second {
		t.Errorf("took %v", time.Since(start))
	}
	lines, files := readResult(t, root, "b")
	if !lines[1].Limit || !lines[2].Limit || len(files["1.out"]) != 32<<10 || len(files["2.err"]) != 32<<10 {
		t.Errorf("limit: %+v %d %d", lines, len(files["1.out"]), len(files["2.err"]))
	}
	if !lines[3].TimedOut || lines[3].Exit != -1 || files["3.out"] != "before\n" {
		t.Errorf("hang: %+v %q", lines[3], files["3.out"])
	}
	if lines[4].Exit != 0 || lines[4].TimedOut {
		t.Errorf("plain: %+v", lines[4])
	}
}

func TestAgentCannotStartAProgram(t *testing.T) {
	root := t.TempDir()
	m := Manifest{Jobs: 1, Tests: []Spec{{ID: 9, Name: "bad", TimeoutMs: 1000}}}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	data, _ := json.Marshal(m)
	tw.WriteHeader(&tar.Header{Name: ManifestName, Mode: 0o644, Size: int64(len(data))})
	tw.Write(data)
	tw.WriteHeader(&tar.Header{Name: "exe/9.exe", Mode: 0o644, Size: 4}) // not executable, not a program
	tw.Write([]byte("junk"))
	tw.Close()
	zw.Close()
	os.WriteFile(filepath.Join(root, "b.tgz"), buf.Bytes(), 0o644)
	// the agent extracts with mode 0755, so make the program unrunnable by its content
	if _, e, code := mainIn(t, root, "run", "b"); code != 0 {
		t.Fatal(e)
	}
	lines, _ := readResult(t, root, "b")
	if lines[9].Err == "" || lines[9].Exit != -1 {
		t.Errorf("a program that does not start: %+v", lines[9])
	}
}

func TestAgentUsageAndArchiveSafety(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{nil, {"run"}, {"run", "a/b"}, {"run", ".."}, {"walk", "b"}, {"run", "b", "c"}} {
		if _, e, code := mainIn(t, root, args...); code != 2 || !strings.Contains(e, "usage") {
			t.Errorf("%v: exit %d %q", args, code, e)
		}
	}
	if _, e, code := mainIn(t, root, "run", "missing"); code != 1 || !strings.Contains(e, "missing.tgz") {
		t.Errorf("a batch that is not there: %d %q", code, e)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	tw.WriteHeader(&tar.Header{Name: "../escape.txt", Mode: 0o644, Size: 1})
	tw.Write([]byte("x"))
	tw.Close()
	zw.Close()
	os.WriteFile(filepath.Join(root, "evil.tgz"), buf.Bytes(), 0o644)
	if _, e, code := mainIn(t, root, "run", "evil"); code != 1 || !strings.Contains(e, "leaves the batch folder") {
		t.Errorf("an archive that leaves its folder: %d %q", code, e)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape.txt")); err == nil {
		t.Error("the archive wrote outside its folder")
	}
}

func TestAgentStoppedByContextWritesNoLine(t *testing.T) {
	root := t.TempDir()
	batch(t, root, "b", 1, 1<<20, prog{spec: Spec{ID: 1, Name: "slow", TimeoutMs: 60000}, script: `sleep 300`})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var out, errb bytes.Buffer
	if code := MainIn(ctx, root, []string{"run", "b"}, &out, &errb); code != 1 {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	data, _ := os.ReadFile(filepath.Join(root, "b", "res", ResultsName))
	if len(data) != 0 {
		t.Errorf("a stopped program has a line: %q", data)
	}
	if _, err := os.Stat(filepath.Join(root, "b-res.tgz")); err == nil {
		t.Error("an unfinished batch has a result archive")
	}
}
