//go:build unix

package typedinterop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The declaration work that goes beside the probe run (startFuncDecls): the overload sets of the called
// functions, then the types their signatures name. The tests use a compiler script around the real clang
// that makes the order of the runs part of the test, so that no test reads a clock: a run waits for a file
// that another run writes, and the wait ends the test (after a bound that only a failure reaches: 1200
// sleeps of 50 ms, a minute or more, the longer the more loaded the machine is).

const gadgetHeader = `#pragma once
struct Gadget { int id; double weight; };
int use(Gadget *g);
`

const gadgetProgram = `package main

import (
	"fmt"
	lib "./lib.hpp"
)

func main() {
	n := lib.use(nil)
	fmt.Println(n)
}
`

// scriptClang writes a compiler script (the shell lines in body run before the real clang++, which
// runs unless body ends the script) and returns the compiler for it, the log the script appends to, and the
// folder for its markers.
func scriptClang(t *testing.T, body string) (build.CXX, string, string) {
	t.Helper()
	real, err := exec.LookPath(tsupport.ClangProgram())
	if err != nil {
		t.Skipf("no clang++: %v", err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := filepath.Join(dir, "clang-script")
	text := fmt.Sprintf("#!/bin/sh\nLOG='%s'\nDIR='%s'\nREAL='%s'\n%s\nexec \"$REAL\" \"$@\"\n",
		log, dir, real, body)
	if err := os.WriteFile(script, []byte(text), 0o755); err != nil {
		t.Fatal(err)
	}
	cxx, err := build.LookCXX(context.Background(), script)
	if err != nil {
		t.Fatalf("the script is not a compiler: %v", err)
	}
	return cxx, log, dir
}

func declOptions(cxx build.CXX) Options {
	return Options{Clang: cxx, Runner: new(probe.Runner), Decls: true}
}

func gadgetPackage(t *testing.T) (dir string) {
	t.Helper()
	dir = t.TempDir()
	write(t, dir, "lib.hpp", gadgetHeader)
	write(t, dir, "main.gp", gadgetProgram)
	return dir
}

// The run for a type that only a signature names starts while the probe run is still going: the probe run
// does not end until the other has started. Were the types of the signatures asked after the probe run,
// it would wait for them in vain.
func TestSignatureTypesAreAskedBesideTheProbeRun(t *testing.T) {
	cxx, log, _ := scriptClang(t, `
case "$*" in
*ast-dump-filter=Gadget*) echo gadget-start >> "$LOG" ;;
*ast-dump-filter=gp_probe_*)
	"$REAL" "$@"
	rc=$?
	i=0
	while [ $i -lt 1200 ] && ! grep -q gadget-start "$LOG" 2>/dev/null; do sleep 0.05; i=$((i+1)); done
	if grep -q gadget-start "$LOG" 2>/dev/null; then
		echo probe-end-after-gadget-start >> "$LOG"
	else
		echo probe-end-timeout >> "$LOG"
	fi
	exit $rc ;;
esac`)
	dir := gadgetPackage(t)
	p, fset := tsupport.CheckDir(t, dir, "main.gp")
	rep, err := Analyze(context.Background(), p, fset, declOptions(cxx))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	var events []string
	for _, l := range lines {
		if strings.HasPrefix(l, "gadget-") || strings.HasPrefix(l, "probe-end") {
			events = append(events, l)
		}
	}
	if got := strings.Join(events, " "); got != "gadget-start probe-end-after-gadget-start" {
		t.Errorf("events: %q", got)
	}
	gadget := rep.Decls.Records["Gadget"]
	if gadget == nil || len(gadget.Fields) != 2 {
		t.Errorf("Gadget: %+v (%v)", gadget, rep.Decls.Errors)
	}
}

// A type is asked for once: the signature's type, the probe results' types and the rounds after them do not
// ask the compiler for the same name twice.
func TestSignatureTypesAreAskedOnce(t *testing.T) {
	cxx, log, _ := scriptClang(t, `
case "$*" in
*ast-dump-filter=*) echo "run $*" | tr ' ' '\n' | grep -e '-ast-dump-filter=' >> "$LOG" ;;
esac`)
	dir := gadgetPackage(t)
	p, fset := tsupport.CheckDir(t, dir, "main.gp")
	if _, err := Analyze(context.Background(), p, fset, declOptions(cxx)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	seen := map[string]int{}
	for _, l := range strings.Fields(string(b)) {
		seen[l]++
	}
	want := map[string]int{"-ast-dump-filter=gp_probe_": 1, "-ast-dump-filter=gp_f": 1, "-ast-dump-filter=Gadget": 1}
	for k, n := range want {
		if seen[k] != n {
			t.Errorf("%s: %d runs, want %d (all: %v)", k, seen[k], n, seen)
		}
	}
	if len(seen) != len(want) {
		t.Errorf("runs: %v", seen)
	}
}

// A command that is interrupted while the work beside the probe run is going does not return before that work
// is over: its compiler is killed, and none is left.
func TestInterruptedAnalyzeLeavesNoCompiler(t *testing.T) {
	cxx, _, dir := scriptClang(t, `
case "$*" in
*ast-dump-filter=gp_f*) echo $$ > "$DIR/funcs.pid"; exec sleep 600 ;;
esac`)
	pkg := gadgetPackage(t)
	p, fset := tsupport.CheckDir(t, pkg, "main.gp")
	ctx, interrupt := context.WithCancel(context.Background())
	defer interrupt()
	done := make(chan error, 1)
	go func() {
		_, err := Analyze(ctx, p, fset, declOptions(cxx))
		done <- err
	}()
	pidFile := filepath.Join(dir, "funcs.pid")
	var pid int
	for deadline := time.Now().Add(time.Minute); pid == 0 && time.Now().Before(deadline); {
		if b, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
		}
		if pid == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if pid == 0 {
		t.Fatal("the run for the functions never started")
	}
	interrupt()
	select {
	case err := <-done:
		if err == nil {
			t.Error("an interrupted analysis succeeded")
		}
	case <-time.After(60 * time.Second):
		t.Fatal("Analyze did not return after the interrupt")
	}
	if processGone(pid) {
		return
	}
	t.Errorf("the compiler of the functions' run (pid %d) is still running", pid)
}

// A probe run that fails ends the analysis, and the work beside it (its compiler, its goroutine) is stopped and
// joined before Analyze returns.
func TestFailedProbeJoinsTheFunctionWork(t *testing.T) {
	cxx, _, dir := scriptClang(t, `
case "$*" in
*ast-dump-filter=gp_f*) echo $$ > "$DIR/funcs.pid"; exec sleep 600 ;;
*ast-dump-filter=gp_probe_*)
	i=0
	while [ $i -lt 1200 ] && [ ! -s "$DIR/funcs.pid" ]; do sleep 0.05; i=$((i+1)); done
	exit 2 ;;
esac`)
	pkg := gadgetPackage(t)
	p, fset := tsupport.CheckDir(t, pkg, "main.gp")
	_, err := Analyze(context.Background(), p, fset, declOptions(cxx))
	if err == nil {
		t.Fatal("a failed probe run did not fail the analysis")
	}
	stacks := make([]byte, 1<<20)
	stacks = stacks[:runtime.Stack(stacks, true)]
	if strings.Contains(string(stacks), "typedinterop.startFuncDecls") {
		t.Errorf("the goroutine of the function work is still running when Analyze returns:\n%s", stacks)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "funcs.pid"))
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid == 0 {
		t.Fatal("the run for the functions never started")
	}
	if processGone(pid) {
		return
	}
	t.Errorf("the compiler of the functions' run (pid %d) is still running", pid)
}

const makerHeader = `#pragma once
struct Gadget { int id; double weight; };
Gadget make();
int use(Gadget *g);
`

const makerProgram = `package main

import (
	"fmt"
	lib "./lib.hpp"
)

func main() {
	g := lib.make()
	fmt.Println(lib.use(&g))
}
`

// A type that is both the type of a probe result and a type of a signature is asked for once: the first round
// leaves out the names that the work beside the probe run asks for.
func TestTypeOfResultAndSignatureIsAskedOnce(t *testing.T) {
	cxx, log, _ := scriptClang(t, `
case "$*" in
*ast-dump-filter=*) echo "$*" | tr ' ' '\n' | grep -e '-ast-dump-filter=' >> "$LOG" ;;
esac`)
	dir := t.TempDir()
	write(t, dir, "lib.hpp", makerHeader)
	write(t, dir, "main.gp", makerProgram)
	p, fset := tsupport.CheckDir(t, dir, "main.gp")
	rep, err := Analyze(context.Background(), p, fset, declOptions(cxx))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(log)
	asked := 0
	for _, l := range strings.Fields(string(b)) {
		if l == "-ast-dump-filter=Gadget" {
			asked++
		}
	}
	if asked != 1 {
		t.Errorf("%d runs for Gadget, want 1 (log: %s)", asked, b)
	}
	if gadget := rep.Decls.Records["Gadget"]; gadget == nil || len(gadget.Fields) != 2 {
		t.Errorf("Gadget: %+v (%v)", gadget, rep.Decls.Errors)
	}
}

// A probe run that fails while the run for a signature's type is going does not leave that run: Analyze
// returns after it was stopped and its goroutine ended.
func TestFailedProbeJoinsTheSignatureTypeWork(t *testing.T) {
	cxx, _, dir := scriptClang(t, `
case "$*" in
*ast-dump-filter=Gadget*) echo $$ > "$DIR/types.pid"; exec sleep 600 ;;
*ast-dump-filter=gp_probe_*)
	i=0
	while [ $i -lt 1200 ] && [ ! -s "$DIR/types.pid" ]; do sleep 0.05; i=$((i+1)); done
	exit 2 ;;
esac`)
	pkg := gadgetPackage(t)
	p, fset := tsupport.CheckDir(t, pkg, "main.gp")
	if _, err := Analyze(context.Background(), p, fset, declOptions(cxx)); err == nil {
		t.Fatal("a failed probe run did not fail the analysis")
	}
	stacks := make([]byte, 1<<20)
	stacks = stacks[:runtime.Stack(stacks, true)]
	if strings.Contains(string(stacks), "typedinterop.startFuncDecls") {
		t.Errorf("the goroutine of the declaration work is still running when Analyze returns:\n%s", stacks)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "types.pid"))
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid == 0 {
		t.Fatal("the run for the signature's type never started")
	}
	if processGone(pid) {
		return
	}
	t.Errorf("the compiler of the signature type's run (pid %d) is still running", pid)
}

// processGone waits until the process pid has ended, and reports whether it did within a minute
// (a bound that only a process that is not killed reaches).
func processGone(pid int) bool {
	for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return true
		}
	}
	return false
}
