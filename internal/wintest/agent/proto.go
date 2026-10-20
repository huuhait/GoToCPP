// Package agent is the program that runs on the machine that executes the test programs of the Windows
// suite (Phase 14 step 14.7, ADR-103), and the files that it and the side that drives it share. The
// driver (internal/wintest) copies ONE archive per batch to the machine, runs the agent once, and copies
// ONE archive of results back: a ssh round trip for each program would make the full suite hours long.
//
// The agent runs the programs of a batch at most Jobs at a time, each in a work folder and a TEMP folder
// of its own, with its standard output and standard error sent to files (no pipe can stall, and a
// program that prints without end is stopped when a file passes MaxOutput). A program that is still
// running when its time is up is ended together with the processes it started (a job object on
// Windows, a process group elsewhere). It writes one line of results.jsonl for every program that
// ended, so a second run of the agent on the same batch (the driver retries once when the ssh
// connection drops) runs only the programs that have no line yet.
//
// The agent has no dependency on the rest of gocpp, it is built with GOOS=windows by the driver, and
// the same code runs on Linux for the driver's unit tests (with scripts as the "programs").
package agent

import "time"

// ManifestName is the name of the manifest in a batch archive.
const ManifestName = "manifest.json"

// A Manifest says what a batch holds.
type Manifest struct {
	Jobs      int    `json:"jobs"`       // programs that run at once
	MaxOutput int64  `json:"max_output"` // bytes of stdout or of stderr after which a program is ended
	Tests     []Spec `json:"tests"`
}

// A Spec is one program of a batch. Its executable is exe/<ID>.exe in the archive.
type Spec struct {
	ID        int      `json:"id"` // unique in the batch; names the result files
	Name      string   `json:"name"`
	Args      []string `json:"args,omitempty"`
	Env       []string `json:"env,omitempty"` // KEY=value, added to the agent's environment
	Stdin     bool     `json:"stdin,omitempty"`
	TimeoutMs int64    `json:"timeout_ms"`
}

// Timeout returns the time the program may run.
func (s Spec) Timeout() time.Duration { return time.Duration(s.TimeoutMs) * time.Millisecond }

// A Line is a line of results.jsonl: how a program ended. Its output is in <ID>.out and <ID>.err.
type Line struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Exit     int    `json:"exit"`                // the exit code; -1 when the program was ended or never started
	TimedOut bool   `json:"timed_out,omitempty"` // it was ended after its time
	Limit    bool   `json:"limit,omitempty"`     // it was ended because it printed more than MaxOutput bytes
	Err      string `json:"err,omitempty"`       // the program could not be started
	Ms       int64  `json:"ms"`
}

// File names inside a batch folder and in the result archive.
const (
	ResultsName = "results.jsonl"
	ExeDir      = "exe"
	StdinDir    = "stdin"
)
