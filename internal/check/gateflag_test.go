package check

import (
	"path/filepath"
	"regexp"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// loaderNotAvailable matches the loader's error for a standard package
// without a stub, which the gates may rename (gateUnchecked: "package
// plugin is not supported in gocpp") but which stays the loader's error:
// it has no Gate flag.
var loaderNotAvailable = regexp.MustCompile(`^package \S+ is not (available yet \(Phase \d\d\)|supported in gocpp)$`)

// TestGateFlag checks diag.Diagnostic.Gate: every gate error has it, and
// no other error has it. It runs the gates over the programs of
// gateCases (before the phase of each feature, so that the gates reject
// them) and over the examples at Phase 02.
func TestGateFlag(t *testing.T) {
	type program struct {
		name  string
		dir   string
		phase int
		cfg   load.Config
	}
	var programs []program
	for _, tc := range gateCases {
		files := tc.files
		if files == nil {
			files = map[string]string{"main.gp": tc.src}
		}
		before := tc.phase - 1
		switch {
		case tc.before != 0:
			before = tc.before
		case tc.phase == Never:
			before = LastPhase
		}
		programs = append(programs, program{tc.name, writeTree(t, files), before, load.Config{Tests: tc.tests}})
	}
	mains, err := filepath.Glob("../../examples/*/main.gp")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range mains {
		dir, err := filepath.Abs(filepath.Dir(m))
		if err != nil {
			t.Fatal(err)
		}
		cfg := load.Config{}
		if filepath.Base(dir) == "win32" {
			cfg.GOOS, cfg.GOARCH = "windows", "amd64"
		}
		programs = append(programs, program{"examples/" + filepath.Base(dir), dir, 2, cfg})
	}
	gates := 0
	for _, pr := range programs {
		cfg := pr.cfg
		cfg.Dir, cfg.Stdlib, cfg.GoStd = pr.dir, stdlib.FS(), stdlib.IsStd
		if cfg.GOOS == "" {
			cfg.GOOS, cfg.GOARCH = "linux", "amd64"
		}
		r := Check(load.Load(cfg, "."), Config{Phase: pr.phase})
		for _, p := range r.Packages {
			for _, d := range p.Errors.Diagnostics() {
				isGate := gatePhase(d.Msg) != 0 && !(loaderNotAvailable.MatchString(d.Msg) && !p.Checked())
				if d.Gate != isGate {
					t.Errorf("%s: %s: %s: Gate is %v, want %v", pr.name, d.Pos, d.Msg, d.Gate, isGate)
				}
				if d.Gate {
					gates++
				}
			}
		}
	}
	if gates < 100 {
		t.Errorf("only %d gate errors in %d programs", gates, len(programs))
	}
}
