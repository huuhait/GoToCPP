// Package wintest runs the test programs of the Windows suite on a machine that runs Windows (Phase 14
// step 14.7, ADR-103): the real PC over ssh (the main way), or Wine on this machine (an optional second
// way for developers without a PC). It does not know the gocpp harness: the caller gives it a list of
// Items, a function that builds one, and a function that judges an outcome (Runner).
//
// The unit of work is a batch. The programs of a batch are built here (in parallel), packed into ONE
// archive with the manifest, copied to the machine, run there by internal/wintest/agent (at most Jobs
// at a time, each with a time limit, a work folder of its own and a job object that takes its child
// processes along), and ONE archive with the outputs and exit codes comes back. The logic around that
// (batching, resume, expected failures and their staleness, retries, the lock between runs, the report)
// sits behind the Remote interface so that it is tested on Linux with an in-process fake.
package wintest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/huuhait/gocpp/internal/wintest/agent"
)

// A Remote is the machine that runs the programs, as far as the suite needs it. Names are relative to
// the run's folder on the machine (one folder for each run; Close removes it).
type Remote interface {
	// Put copies the local file to name.
	Put(ctx context.Context, local, name string) error
	// Get copies the file name to the local path.
	Get(ctx context.Context, name, local string) error
	// Agent runs the copy of the agent in the run's folder (it was Put as AgentName) with the
	// arguments, for at most timeout. When the time is up, the processes of the run are ended and
	// TimedOut is set.
	Agent(ctx context.Context, timeout time.Duration, args ...string) (AgentRun, error)
	// Remove deletes the named files and folders of the run's folder (best effort).
	Remove(ctx context.Context, names ...string) error
	// KillAll ends every process of this run on the machine (the programs of one run only).
	KillAll(ctx context.Context)
	// CleanStale deletes the run folders of earlier runs of this tool that are older than the age and
	// belong to no running agent. Only this tool's folder is touched.
	CleanStale(ctx context.Context, age time.Duration) error
	// FreeBytes returns the free space of the drive that holds the run's folder; a negative number with a
	// nil error says the machine cannot tell (the check is then skipped).
	FreeBytes(ctx context.Context) (int64, error)
	// Close removes the run's folder and releases the connection.
	Close()
}

// AgentName is the name the agent has in the run's folder.
const AgentName = "winagent.exe"

// An AgentRun is what the agent printed and how it ended.
type AgentRun struct {
	Stdout, Stderr string
	Code           int
	TimedOut       bool
}

// A LocalRemote runs the agent on this machine in a folder of its own: the fake of the unit tests (the
// agent runs in this process, the "programs" are scripts) and, with Command set, Wine (the agent is the
// Windows build of the agent, run by wine). Nothing outside Root is touched.
type LocalRemote struct {
	Root string
	// Command, when set, makes the command that runs the agent in Root with the arguments (Wine);
	// when nil the agent of this process runs.
	Command func(ctx context.Context, args ...string) *exec.Cmd
	// Stale is called by CleanStale (tests); nil does nothing.
	Stale func(age time.Duration)
	// Kill is called by KillAll (tests).
	Kill func()
	// Free returns the free bytes (tests, Wine); nil means unknown.
	Free func() (int64, error)
}

func (l *LocalRemote) path(name string) string {
	return filepath.Join(l.Root, filepath.FromSlash(name))
}

func (l *LocalRemote) Put(_ context.Context, local, name string) error {
	return copyFile(local, l.path(name))
}

func (l *LocalRemote) Get(_ context.Context, name, local string) error {
	return copyFile(l.path(name), local)
}

func (l *LocalRemote) Agent(ctx context.Context, timeout time.Duration, args ...string) (AgentRun, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var out, errb bytes.Buffer
	if l.Command == nil {
		code := agent.MainIn(cctx, l.Root, args, &out, &errb)
		res := AgentRun{Stdout: out.String(), Stderr: errb.String(), Code: code, TimedOut: cctx.Err() != nil && ctx.Err() == nil}
		return res, nil
	}
	cmd := l.Command(cctx, args...)
	cmd.Dir = l.Root
	cmd.Stdout, cmd.Stderr = &out, &errb
	cmd.WaitDelay = 5 * time.Second
	err := cmd.Run()
	res := AgentRun{Stdout: out.String(), Stderr: errb.String(), TimedOut: cctx.Err() != nil && ctx.Err() == nil}
	if ee, ok := err.(*exec.ExitError); ok {
		res.Code, err = ee.ExitCode(), nil
	}
	if res.TimedOut {
		err = nil
	}
	return res, err
}

func (l *LocalRemote) Remove(_ context.Context, names ...string) error {
	for _, n := range names {
		os.RemoveAll(l.path(n))
	}
	return nil
}

func (l *LocalRemote) KillAll(context.Context) {
	if l.Kill != nil {
		l.Kill()
	}
}

func (l *LocalRemote) CleanStale(_ context.Context, age time.Duration) error {
	if l.Stale != nil {
		l.Stale(age)
	}
	return nil
}

func (l *LocalRemote) FreeBytes(context.Context) (int64, error) {
	if l.Free == nil {
		return -1, nil
	}
	return l.Free()
}

func (l *LocalRemote) Close() { os.RemoveAll(l.Root) }

func copyFile(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copying %s: %v", from, err)
	}
	return out.Close()
}
