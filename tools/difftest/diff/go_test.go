package diff

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/tools/difftest/gen"
)

// TestGeneratedProgramsRunWithGo builds a bundle of generated programs with real Go (twice,
// as the runner does) and checks that every program builds, ends normally, never makes a
// panic that no scenario meant, and prints the same output in two runs.
func TestGeneratedProgramsRunWithGo(t *testing.T) {
	if testing.Short() {
		t.Skip("builds with real Go")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go command")
	}
	var progs []*gen.Program
	for id := 0; id < 120; id++ {
		p, err := gen.Generate(gen.Config{Seed: 91, Features: gen.FeaturesForPhase(9), FatalPercent: 15}, id)
		if err != nil {
			t.Fatal(err)
		}
		progs = append(progs, p)
	}
	b := &GoBackend{Work: t.TempDir(), Cache: t.TempDir(), Timeout: time.Minute}
	for _, files := range []int{4} {
		src, err := gen.BundleFiles(progs, gen.Recovered, files)
		if err != nil {
			t.Fatal(err)
		}
		r1, err := b.Run(context.Background(), src)
		if err != nil {
			t.Fatal(err)
		}
		if !r1.BuildOK {
			t.Fatalf("%d files: the bundle does not build:\n%s", files, clip(r1.BuildErr, 3000))
		}
		if r1.Exit != 0 || r1.TimedOut {
			t.Fatalf("%d files: exit %d, timeout %v\n%s", files, r1.Exit, r1.TimedOut, clip(r1.Stderr, 2000))
		}
		ids, parts := SplitOutput(r1.Stdout)
		if len(ids) != len(progs) {
			t.Fatalf("%d files: %d programs printed, want %d", files, len(ids), len(progs))
		}
		for _, id := range ids {
			if strings.Contains(parts[id], "UNEXPECTED PANIC") {
				t.Errorf("program %d panics without meaning to:\n%s", id, clip(parts[id], 600))
			}
		}
		r2, err := b.Run(context.Background(), src)
		if err != nil {
			t.Fatal(err)
		}
		if r2.Stdout != r1.Stdout {
			_, p2 := SplitOutput(r2.Stdout)
			for _, id := range ids {
				if parts[id] != p2[id] {
					sig, _ := FirstDiff(parts[id], p2[id])
					t.Errorf("program %d prints something different in two runs: %s", id, sig)
					break
				}
			}
		}
	}
	// a program that ends with a fatal panic exits with 2 and the panic on stderr
	for _, p := range progs {
		if p.Tail == "" {
			continue
		}
		src, err := gen.Bundle([]*gen.Program{p}, gen.Standalone)
		if err != nil {
			t.Fatal(err)
		}
		r, err := b.Run(context.Background(), src)
		if err != nil {
			t.Fatal(err)
		}
		if !r.BuildOK || r.Exit != 2 || !strings.HasPrefix(r.Stderr, "panic: ") {
			t.Fatalf("program %d (%s) alone: exit %d stderr %q", p.ID, p.Tail, r.Exit, clip(r.Stderr, 200))
		}
		break
	}
}
