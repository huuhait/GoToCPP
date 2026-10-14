package main

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/version"
)

// gatePhaseEnv names the environment variable that opens the phase gates of
// internal/check up to a later phase, for gocpp's own development and tests:
// while phase NN is built, version.Phase is still NN-1, so gocpp would reject
// the features that the phase adds, and the test programs of the phase
// couldn't run. tests/harness_test.go sets it, for each test of a phase after
// version.Phase, to that test's phase. It can only raise the gate phase, never lower it
// below version.Phase. check, build, run and emit (and check -types) read it,
// in checkCommand.check.
const gatePhaseEnv = "GOCPP_GATE_PHASE"

// gatePhaseRx matches a GOCPP_GATE_PHASE value: two digits, like
// GOCPP_TEST_PHASE.
var gatePhaseRx = regexp.MustCompile(`^[0-9]{2}$`)

// gatePhase returns the phase of the gates (check.Config.Phase) for the value
// s of GOCPP_GATE_PHASE: version.Phase when s is empty or names an earlier
// phase, else the phase s names. A malformed value is an error.
func gatePhase(s string) (int, error) {
	if s == "" {
		return version.Phase, nil
	}
	n, err := strconv.Atoi(s)
	if !gatePhaseRx.MatchString(s) || err != nil || n > check.LastPhase {
		return 0, fmt.Errorf("%s=%s: want a phase from 00 to %02d, for example %s=03", gatePhaseEnv, s, check.LastPhase, gatePhaseEnv)
	}
	return max(n, version.Phase), nil
}
