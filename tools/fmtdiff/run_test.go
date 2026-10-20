package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestWorkCleanupOwnsOnlyChild(t *testing.T) {
	parent := t.TempDir()
	sentinel := filepath.Join(parent, "caller-data.txt")
	if err := os.WriteFile(sentinel, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}

	owned, cleanup, err := newWorkDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(owned) != parent || owned == parent {
		t.Fatalf("owned work directory %q is not a child of %q", owned, parent)
	}
	if err := os.WriteFile(filepath.Join(owned, "temporary.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	cleanup()
	cleanup() // repeated cleanup must still leave caller-owned data alone
	if _, err := os.Stat(owned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tool-owned child was not removed: %v", err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "preserve" {
		t.Fatalf("caller-owned sentinel changed: %q, %v", data, err)
	}
}

func TestWorkCleanupOnInterrupt(t *testing.T) {
	if parent := os.Getenv("FMTDIFF_INTERRUPT_CHILD"); parent != "" {
		// Zero workers hold the command after installing its signal handler and report file,
		// without launching a compiler or running any generated program.
		flag.CommandLine = flag.NewFlagSet("fmtdiff", flag.ExitOnError)
		os.Args = []string{
			"fmtdiff", "-mode", "go", "-n", "1", "-per", "1", "-j", "0", "-work", parent,
		}
		main()
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt cannot be sent to a Windows process")
	}

	parent := t.TempDir()
	sentinel := filepath.Join(parent, "caller-data.txt")
	if err := os.WriteFile(sentinel, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWorkCleanupOnInterrupt$")
	command.Env = append(os.Environ(), "FMTDIFF_INTERRUPT_CHILD="+parent)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			command.Process.Kill()
			command.Wait()
		}
	}()

	// The report is created only after signal.Notify, so its presence is the readiness edge.
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	var owned string
	for owned == "" {
		reports, err := filepath.Glob(filepath.Join(parent, "fmtdiff-*", "out", "report-1.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if len(reports) == 1 {
			owned = filepath.Dir(filepath.Dir(reports[0]))
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("fmtdiff did not become ready for interruption")
		case <-ticker.C:
		}
	}

	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	waitError := command.Wait()
	waited = true
	var exitError *exec.ExitError
	if !errors.As(waitError, &exitError) || exitError.ExitCode() != 130 {
		t.Fatalf("interrupted fmtdiff: %v\n%s", waitError, output.String())
	}
	if _, err := os.Stat(owned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interruption left the tool-owned child: %v", err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "preserve" {
		t.Fatalf("interruption changed caller-owned data: %q, %v", data, err)
	}
}

// TestGocpp runs a small fixed subset of the cases with real Go and with gocpp. It is the
// always-on part of the campaign when GOCPP_FMTDIFF=1 (dev shell: it needs `go`), with the gocpp
// binary in GOCPP_FMTDIFF_GOCPP:
//
//	GOCPP_FMTDIFF=1 GOCPP_FMTDIFF_GOCPP=/path/to/gocpp go test ./tools/fmtdiff -run TestGocpp
func TestGocpp(t *testing.T) {
	if os.Getenv("GOCPP_FMTDIFF") != "1" {
		t.Skip("set GOCPP_FMTDIFF=1 (and GOCPP_FMTDIFF_GOCPP) to run programs")
	}
	bin := os.Getenv("GOCPP_FMTDIFF_GOCPP")
	if bin == "" {
		t.Fatal("GOCPP_FMTDIFF_GOCPP is not set")
	}
	dir := t.TempDir()
	r := &Runner{Gocpp: bin, Work: dir, XDG: os.Getenv("XDG_CACHE_HOME")}
	g := NewGen()
	for _, prog := range []int{0, 1} {
		p := g.Program(1, prog, 150)
		oc := r.Check(g, p, 20)
		for _, m := range oc.GenBugs {
			t.Errorf("generator bug: %s", formatMismatch(g, m))
		}
		for _, m := range oc.Mismatches {
			if k := known(g, m); k != "" {
				t.Logf("known bug %s: case %d", k, m.Case.ID)
				continue
			}
			t.Errorf("mismatch: %s", formatMismatch(g, m))
		}
	}
}
