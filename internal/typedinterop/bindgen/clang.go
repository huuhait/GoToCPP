package bindgen

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// DefaultTimeout limits one clang run, when Config.Timeout is 0.
const DefaultTimeout = 2 * time.Minute

// maxStderr limits what is kept of clang's messages.
const maxStderr = 1 << 20

// A tool runs clang++ on one translation unit that only includes the header.
type tool struct {
	cxx     build.CXX
	dir     string   // the folder clang runs in (absolute)
	flags   []string // -I, -D, -std and the like
	tmp     string   // a folder of our own
	timeout time.Duration
}

// A cappedBuffer keeps the first max bytes written to it.
type cappedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

// source writes a translation unit with the text and returns its path.
func (t *tool) source(name, text string) (string, error) {
	p := filepath.Join(t.tmp, name)
	return p, os.WriteFile(p, []byte(text), 0o644)
}

// run runs the compiler with the mode flags (such as -E -dD) on the file src
// and gives its standard output to read. The compiler's messages are
// returned; a failed compile is not an error of run (the caller decides what
// the messages mean), but a compiler that did not start, was stopped or died
// of a signal is.
func (t *tool) run(ctx context.Context, mode []string, src string, read func(io.Reader) error) (stderr []byte, exit int, err error) {
	dep := filepath.Join(t.tmp, "deps.d")
	args, err := interop.ProbeCompilerArgs(t.cxx, mode, "", t.flags, dep, src)
	if err != nil {
		return nil, 0, err
	}
	timeout := t.timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	rctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	msgs := &cappedBuffer{max: maxStderr}
	var stdout io.ReadCloser
	cmd, err := build.Start(rctx, func() *exec.Cmd {
		c := t.cxx.Command(rctx, args...)
		c.Dir = t.dir
		c.Env = interop.ProbeEnv(t.tmp, t.dir)
		c.Stderr = msgs
		c.WaitDelay = 5 * time.Second
		build.InGroup(c)
		stdout, _ = c.StdoutPipe()
		return c
	})
	if err != nil {
		return nil, 0, fmt.Errorf("%s didn't start: %w", t.cxx.Name(), err)
	}
	defer build.KillGroup(cmd)
	stopWatch := build.WatchPipes(cmd) // a header that is a named pipe would make it wait for ever
	readErr := read(stdout)
	io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
	pipeErr := stopWatch()
	build.KillGroup(cmd)
	switch {
	case pipeErr != nil:
		return nil, 0, fmt.Errorf("%s: %w", t.cxx.Name(), pipeErr)
	case ctx.Err() != nil:
		return nil, 0, fmt.Errorf("%s was stopped: %w", t.cxx.Name(), ctx.Err())
	case rctx.Err() != nil:
		return nil, 0, fmt.Errorf("%s was stopped after %v", t.cxx.Name(), timeout)
	case cmd.ProcessState == nil:
		return nil, 0, fmt.Errorf("%s didn't run: %w", t.cxx.Name(), waitErr)
	case !cmd.ProcessState.Exited():
		return nil, 0, fmt.Errorf("%s: %v%s", t.cxx.Name(), cmd.ProcessState, excerpt(msgs.buf.Bytes()))
	}
	if readErr != nil {
		return msgs.buf.Bytes(), cmd.ProcessState.ExitCode(), fmt.Errorf("reading the output of %s: %w%s", t.cxx.Name(), readErr, excerpt(msgs.buf.Bytes()))
	}
	return msgs.buf.Bytes(), cmd.ProcessState.ExitCode(), nil
}

// excerpt returns the start of the compiler's messages for an error message.
func excerpt(msgs []byte) string {
	s := strings.TrimSpace(string(msgs))
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 8 {
		lines = append(lines[:8], "...")
	}
	return ":\n" + strings.Join(lines, "\n")
}

var errLine = regexp.MustCompile(`(?m)^(?:[^\n]*?:\d+:\d+: )?(fatal error|error): (.*)$`)

var notFound = regexp.MustCompile(`'([^']+)' file not found`)

// headerError turns the messages of a compile into the error for a header
// (the wording of gocpp check's header check), or nil when it compiled. dir
// is the folder that paths in the messages are shown relative to.
func headerError(header, dir string, msgs []byte) error {
	m := errLine.FindSubmatch(msgs)
	if m == nil {
		return nil
	}
	text := string(m[2])
	if nf := notFound.FindStringSubmatch(text); nf != nil {
		if strings.Trim(header, "<>\"") == nf[1] {
			return fmt.Errorf("cannot find header %s", header)
		}
		return fmt.Errorf("cannot find header %s (needed by %s)", nf[1], header)
	}
	line := strings.ReplaceAll(firstLine(string(m[0])), dir+string(filepath.Separator), "")
	return fmt.Errorf("header %s doesn't compile: %s", header, line)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// readerT is the type of the standard output that run hands to its reader.
type readerT = io.Reader

// A countingReader counts the bytes read through it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// searchRoots returns the folders where the compiler looks for <headers> by
// itself (its own, those of CPATH and the like, what the Nix wrapper adds),
// without the flags of the user. A failure gives none.
func (t *tool) searchRoots(ctx context.Context) []string {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := t.cxx.Command(ctx, "-x", "c++", "-E", "-v", "-", "-o", os.DevNull)
	cmd.Dir = t.dir
	cmd.Stdin = strings.NewReader("")
	out, _ := cmd.CombinedOutput()
	var roots []string
	in := false
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "#include <...> search starts here:"), strings.HasPrefix(line, "#include \"...\" search starts here:"):
			in = true
		case strings.HasPrefix(line, "End of search list."):
			in = false
		case in && strings.HasPrefix(line, " "):
			p := strings.TrimSpace(line)
			p = strings.TrimSuffix(p, " (framework directory)")
			roots = append(roots, filepath.Clean(p))
		}
	}
	return roots
}
