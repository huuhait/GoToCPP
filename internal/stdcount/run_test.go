package stdcount

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

var errExit = errors.New("exit status 2")

// scriptedRounds answers each run of the test binary with the next scripted round and records the
// tests that the run was told to skip.
type scriptedRounds struct {
	t      *testing.T
	rounds []roundResult
	skips  [][]string
}

func (s *scriptedRounds) run(skip []string) roundResult {
	index := len(s.skips)
	s.skips = append(s.skips, skip)
	if index >= len(s.rounds) {
		s.t.Fatalf("round %d ran, but only %d rounds were scripted", index, len(s.rounds))
	}
	return s.rounds[index]
}

func cleanRound(output string) roundResult {
	return roundResult{Out: []byte(output)}
}

func failedRound(output string) roundResult {
	return roundResult{Out: []byte(output), Err: errExit}
}

func TestRunRoundsStopsAfterCleanRound(t *testing.T) {
	script := &scriptedRounds{t: t, rounds: []roundResult{
		cleanRound("=== RUN   TestA\n--- PASS: TestA (0.00s)\nPASS\n"),
	}}

	results, log := runRounds(context.Background(), script.run)

	if len(script.skips) != 1 || len(script.skips[0]) != 0 {
		t.Fatalf("skips = %q; want one round without skips", script.skips)
	}
	if results["TestA"].Status != "pass" {
		t.Fatalf("results = %v", results)
	}
	if !strings.Contains(log, "=== round 0 (0 tests done): <nil>") {
		t.Fatalf("log = %s", log)
	}
}

func TestRunRoundsRunsAgainWithoutTheCrashedTest(t *testing.T) {
	script := &scriptedRounds{t: t, rounds: []roundResult{
		failedRound("=== RUN   TestA\n--- PASS: TestA (0.00s)\n=== RUN   TestCrash\npanic: boom\n"),
		cleanRound("=== RUN   TestB\n--- PASS: TestB (0.00s)\nPASS\n"),
	}}

	results, _ := runRounds(context.Background(), script.run)

	if len(script.skips) != 2 || len(script.skips[0]) != 0 {
		t.Fatalf("skips = %q; want two rounds, the first without skips", script.skips)
	}
	if !slices.Equal(script.skips[1], []string{"TestA", "TestCrash"}) {
		t.Fatalf("second round skips = %q; want TestA and TestCrash", script.skips[1])
	}
	if results["TestA"].Status != "pass" || results["TestB"].Status != "pass" {
		t.Fatalf("results = %v", results)
	}
	crashed := results["TestCrash"]
	if crashed.Status != "fail" || crashed.Lines[0] != "the test binary ended during this test:" {
		t.Fatalf("TestCrash = %+v", crashed)
	}
	if !slices.Contains(crashed.Lines, "panic: boom") {
		t.Fatalf("TestCrash lacks the end of the output: %q", crashed.Lines)
	}
}

func TestRunRoundsTreatsHardTimeoutAsCrashedRound(t *testing.T) {
	script := &scriptedRounds{t: t, rounds: []roundResult{
		{Out: []byte("=== RUN   TestHang\n"), Err: errExit, HardTimeout: true},
		cleanRound("=== RUN   TestNext\n--- PASS: TestNext (0.00s)\nPASS\n"),
	}}

	results, log := runRounds(context.Background(), script.run)

	if len(script.skips) != 2 || !slices.Equal(script.skips[1], []string{"TestHang"}) {
		t.Fatalf("skips = %q; want the second round to skip TestHang", script.skips)
	}
	hung := results["TestHang"]
	if hung.Status != "fail" || !strings.Contains(hung.Lines[0], "hung and was killed") {
		t.Fatalf("TestHang = %+v", hung)
	}
	if results["TestNext"].Status != "pass" {
		t.Fatalf("results = %v", results)
	}
	if !strings.Contains(log, "did not end in time and was killed") {
		t.Fatalf("log = %s", log)
	}
}

func TestRunRoundsBlamesEveryTestOfTimeoutPanic(t *testing.T) {
	timeout := "=== RUN   TestSlow\n=== RUN   TestSlower\n" +
		"panic: test timed out after 5m0s\n\trunning tests:\n\t\tTestSlow (5m0s)\n" +
		"\t\tTestSlower (5m0s)\n\ngoroutine 1 [running]:\n"
	script := &scriptedRounds{t: t, rounds: []roundResult{
		failedRound(timeout),
		cleanRound("PASS\n"),
	}}

	results, _ := runRounds(context.Background(), script.run)

	if results["TestSlow"].Status != "fail" || results["TestSlower"].Status != "fail" {
		t.Fatalf("results = %v", results)
	}
	if !slices.Equal(script.skips[1], []string{"TestSlow", "TestSlower"}) {
		t.Fatalf("skips = %q", script.skips)
	}
}

func TestRunRoundsBlamesTestWhoseFailLineIsFollowedByPanic(t *testing.T) {
	script := &scriptedRounds{t: t, rounds: []roundResult{
		failedRound("=== RUN   TestA\n--- FAIL: TestA (0.00s)\n\npanic: in a goroutine\n"),
		cleanRound("PASS\n"),
	}}

	results, _ := runRounds(context.Background(), script.run)

	if len(script.skips) != 2 || !slices.Equal(script.skips[1], []string{"TestA"}) {
		t.Fatalf("skips = %q", script.skips)
	}
	if results["TestA"].Status != "fail" {
		t.Fatalf("results = %v", results)
	}
}

func TestRunRoundsStopsWhenFailureNamesNoTest(t *testing.T) {
	script := &scriptedRounds{t: t, rounds: []roundResult{failedRound("init failed\n")}}

	results, _ := runRounds(context.Background(), script.run)

	if len(script.skips) != 1 || len(results) != 0 {
		t.Fatalf("rounds = %d, results = %v; want one round and none", len(script.skips), results)
	}
}

func TestRunRoundsStopsAfterMaxRounds(t *testing.T) {
	rounds := 0
	alwaysCrashes := func(skip []string) roundResult {
		rounds++
		return failedRound(fmt.Sprintf("=== RUN   TestCrash%d\npanic: boom\n", rounds))
	}

	results, _ := runRounds(context.Background(), alwaysCrashes)

	if rounds != maxRounds || len(results) != maxRounds {
		t.Fatalf("rounds = %d, results = %d; want %d each", rounds, len(results), maxRounds)
	}
}

func TestRunRoundsStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rounds := 0
	cancelsInFirstRound := func(skip []string) roundResult {
		rounds++
		cancel()
		return failedRound("=== RUN   TestA\npanic: killed\n")
	}

	runRounds(ctx, cancelsInFirstRound)

	if rounds != 1 {
		t.Fatalf("rounds = %d; want 1", rounds)
	}
}

func TestTestArgumentsSkipsFinishedTestsByQuotedName(t *testing.T) {
	args := testArguments(0, []string{"-test.short"}, []string{"TestA", "Test.B"})

	want := `-test.skip=^(TestA|Test\.B)$`
	if !slices.Contains(args, want) || !slices.Contains(args, "-test.short") {
		t.Fatalf("args = %q; want %s and -test.short", args, want)
	}
}

func TestTestArgumentsWithoutFinishedTestsHasNoSkip(t *testing.T) {
	args := testArguments(0, nil, nil)

	for _, arg := range args {
		if strings.HasPrefix(arg, "-test.skip") {
			t.Fatalf("args = %q; want no -test.skip", args)
		}
	}
}
