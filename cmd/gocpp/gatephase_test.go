package main

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/version"
)

func TestGatePhase(t *testing.T) {
	for _, tt := range []struct {
		env   string
		phase int
		err   bool
	}{
		{"", version.Phase, false},
		{"00", version.Phase, false}, // never below version.Phase
		{"03", max(3, version.Phase), false},
		{"15", 15, false},
		{"16", 0, true},
		{"3", 0, true}, // two digits, like GOCPP_TEST_PHASE's 01
		{"003", 0, true},
		{"-1", 0, true},
		{" 03", 0, true},
		{"x", 0, true},
	} {
		phase, err := gatePhase(tt.env)
		if phase != tt.phase || (err != nil) != tt.err {
			t.Errorf("gatePhase(%q) = %d, %v; want %d, error %v", tt.env, phase, err, tt.phase, tt.err)
		}
	}
}

// TestGatePhaseCheck checks that gocpp check (and so build, run and emit, which
// check the same way) opens the gates up to GOCPP_GATE_PHASE, and that a
// malformed value is exit status 2 with a message.
func TestGatePhaseCheck(t *testing.T) {
	if version.Phase >= 8 {
		t.Skip("channels are no longer gated") // no phase gate is left to test with
	}
	tree := map[string]string{"main.go": "package main\n\nfunc main() {\n\tc := make(chan int, 1)\n\tc <- 1\n\tprintln(<-c)\n}\n"}
	t.Setenv(gatePhaseEnv, "")
	r := checkIn(t, tree, "check", "main.gp")
	if r.code != 1 || !strings.Contains(r.stderr, "not supported yet (Phase 08)") {
		t.Errorf("without %s:\n%s\nwant exit 1 and the gate's error", gatePhaseEnv, show(r))
	}
	t.Setenv(gatePhaseEnv, "07")
	if r := checkIn(t, tree, "check", "main.gp"); r.code != 1 || !strings.Contains(r.stderr, "not supported yet (Phase 08)") {
		t.Errorf("%s=07:\n%s\nwant exit 1: the gates open only up to Phase 07", gatePhaseEnv, show(r))
	}
	t.Setenv(gatePhaseEnv, "08")
	if r := checkIn(t, tree, "check", "main.gp"); r.code != 0 || r.stderr != "" {
		t.Errorf("%s=08:\n%s\nwant exit 0 and no output", gatePhaseEnv, show(r))
	}
	t.Setenv(gatePhaseEnv, "3")
	want := "gocpp: GOCPP_GATE_PHASE=3: want a phase from 00 to 15, for example GOCPP_GATE_PHASE=03\n"
	if r := checkIn(t, tree, "check", "main.gp"); r.code != 2 || r.stderr != want {
		t.Errorf("%s=3:\n%s\nwant exit 2 and %q", gatePhaseEnv, show(r), want)
	}
}
