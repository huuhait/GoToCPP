package agent

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Main is the agent's command line: "run NAME" runs the batch NAME.tgz in the current folder and leaves
// NAME-res.tgz. It deletes nothing: the batch folder (with the lines of the programs that ended) stays
// until the driver, which has fetched the results, deletes it, and a second "run NAME" after a
// connection that dropped runs only the programs that have no line yet. It returns the exit status: 0 when every program of the batch has a
// line in the results (whatever the programs did), 2 for a usage error, 1 when the batch could not be
// run.
func Main(args []string, stdout, stderr io.Writer) int {
	return MainIn(context.Background(), ".", args, stdout, stderr)
}

// MainIn is Main for the folder root (the driver's unit tests run the agent in a process of their own
// folder), stopped by ctx: the programs that run then are ended and the batch is left unfinished.
func MainIn(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "run" {
		args = args[1:]
		if len(args) == 1 && !strings.ContainsAny(args[0], `/\:`) && args[0] != "" && args[0] != ".." {
			return runBatch(ctx, root, args[0], stdout, stderr)
		}
	}
	fmt.Fprintln(stderr, "usage: winagent run NAME   (runs NAME.tgz in the current folder, leaves NAME-res.tgz)")
	return 2
}

func runBatch(ctx context.Context, root, name string, stdout, stderr io.Writer) int {
	if err := run(ctx, root, name, stdout); err != nil {
		fmt.Fprintf(stderr, "winagent: %v\n", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, root, name string, stdout io.Writer) error {
	dir := filepath.Join(root, name)
	marker := filepath.Join(dir, ".extracted")
	if _, err := os.Stat(marker); err != nil {
		// A folder without the marker is a half-extracted one from an agent that was ended: start it again.
		os.RemoveAll(dir)
		if err := extract(dir+".tgz", dir); err != nil {
			return fmt.Errorf("unpacking %s.tgz: %v", name, err)
		}
		if err := os.WriteFile(marker, nil, 0o644); err != nil {
			return err
		}
	}
	var m Manifest
	data, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("%s: %v", ManifestName, err)
	}
	resDir := filepath.Join(dir, "res")
	if err := os.MkdirAll(resDir, 0o755); err != nil {
		return err
	}
	if err := dropPartialLine(filepath.Join(resDir, ResultsName)); err != nil {
		return err
	}
	done, err := readLines(filepath.Join(resDir, ResultsName))
	if err != nil {
		return err
	}
	var pending []Spec
	for _, s := range m.Tests {
		if !done[s.ID] {
			pending = append(pending, s)
		}
	}
	if err := runAll(ctx, dir, resDir, m, pending); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return fmt.Errorf("stopped before the batch was done")
	}
	if err := pack(resDir, dir+"-res.tgz"); err != nil {
		return fmt.Errorf("packing the results: %v", err)
	}
	fmt.Fprintf(stdout, "winagent: done tests=%d ran=%d resumed=%d\n", len(m.Tests), len(pending), len(m.Tests)-len(pending))
	return nil
}

// dropPartialLine cuts a last line that has no newline (an agent that was killed while it wrote): the
// next line must start on a line of its own.
func dropPartialLine(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || err == nil && (len(data) == 0 || data[len(data)-1] == '\n') {
		return nil
	}
	if err != nil {
		return err
	}
	return os.Truncate(path, int64(bytes.LastIndexByte(data, '\n')+1))
}

// readLines returns the IDs of the lines of a results file (none when it does not exist). A last line
// that was cut short by a killed agent is ignored: its program runs again.
func readLines(path string) (map[int]bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[int]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	done := map[int]bool{}
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var l Line
		if json.Unmarshal(sc.Bytes(), &l) == nil {
			done[l.ID] = true
		}
	}
	return done, sc.Err()
}

func runAll(ctx context.Context, dir, resDir string, m Manifest, pending []Spec) error {
	jobs := m.Jobs
	if jobs < 1 {
		jobs = 1
	}
	maxOut := m.MaxOutput
	if maxOut <= 0 {
		maxOut = 8 << 20
	}
	log, err := os.OpenFile(filepath.Join(resDir, ResultsName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer log.Close()
	var mu sync.Mutex
	write := func(l Line) error {
		b, _ := json.Marshal(l)
		mu.Lock()
		defer mu.Unlock()
		_, err := log.Write(append(b, '\n'))
		return err
	}
	work := make(chan Spec)
	errs := make(chan error, jobs)
	var wg sync.WaitGroup
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range work {
				l := runOne(ctx, dir, resDir, s, maxOut)
				if ctx.Err() != nil {
					return // ended by the stop, not by itself: no line, so a second run does it again
				}
				if err := write(l); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	for _, s := range pending {
		select {
		case work <- s:
		case err := <-errs:
			close(work)
			wg.Wait()
			return err
		}
	}
	close(work)
	wg.Wait()
	select {
	case err := <-errs:
		return err
	default:
	}
	return nil
}

// pollEvery is how often a running program's output files are measured.
const pollEvery = 200 * time.Millisecond

func runOne(ctx context.Context, dir, resDir string, s Spec, maxOut int64) Line {
	start := time.Now()
	l := Line{ID: s.ID, Name: s.Name, Exit: -1}
	finish := func() Line { l.Ms = time.Since(start).Milliseconds(); return l }
	id := strconv.Itoa(s.ID)
	exe, err := filepath.Abs(filepath.Join(dir, ExeDir, id+".exe"))
	if err != nil {
		l.Err = err.Error()
		return finish()
	}
	workDir := filepath.Join(dir, "work", id)
	tmpDir := filepath.Join(dir, "tmp", id)
	for _, d := range []string{workDir, tmpDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			l.Err = err.Error()
			return finish()
		}
	}
	absTmp, _ := filepath.Abs(tmpDir)
	outPath, errPath := filepath.Join(resDir, id+".out"), filepath.Join(resDir, id+".err")
	outF, err := os.Create(outPath)
	if err != nil {
		l.Err = err.Error()
		return finish()
	}
	defer outF.Close()
	errF, err := os.Create(errPath)
	if err != nil {
		l.Err = err.Error()
		return finish()
	}
	defer errF.Close()
	cmd := exec.Command(exe, s.Args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "TEMP="+absTmp, "TMP="+absTmp, "TMPDIR="+absTmp)
	cmd.Env = append(cmd.Env, s.Env...)
	cmd.Stdout, cmd.Stderr = outF, errF
	if s.Stdin {
		in, err := os.Open(filepath.Join(dir, StdinDir, id))
		if err != nil {
			l.Err = err.Error()
			return finish()
		}
		defer in.Close()
		cmd.Stdin = in
	}
	g, err := newGroup(cmd)
	if err != nil {
		l.Err = err.Error()
		return finish()
	}
	defer g.close()
	if err := cmd.Start(); err != nil {
		l.Err = "it did not start: " + err.Error()
		return finish()
	}
	g.attach(cmd.Process)
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	timeout := time.NewTimer(s.Timeout())
	defer timeout.Stop()
	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	killed := false
loop:
	for {
		select {
		case <-waited:
			break loop
		case <-timeout.C:
			l.TimedOut, killed = true, true
		case <-ctx.Done():
			l.Err, killed = "the agent was stopped", true
		case <-tick.C:
			if tooBig(outF, maxOut) || tooBig(errF, maxOut) {
				l.Limit, killed = true, true
			}
		}
		if killed {
			g.kill()
			<-waited
			break
		}
	}
	// What the program left running (a child that outlives it) ends with it, as in the harness.
	g.kill()
	if !killed && cmd.ProcessState != nil {
		l.Exit = cmd.ProcessState.ExitCode()
	}
	truncate(outF, outPath, maxOut)
	truncate(errF, errPath, maxOut)
	return finish()
}

func tooBig(f *os.File, max int64) bool {
	fi, err := f.Stat()
	return err == nil && fi.Size() > max
}

// truncate cuts a file that is longer than max.
func truncate(f *os.File, path string, max int64) {
	if tooBig(f, max) {
		f.Truncate(max)
	}
}

func extract(archive, dir string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(zr)
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		p := filepath.Join(root, filepath.FromSlash(h.Name))
		if !strings.HasPrefix(p, root+string(filepath.Separator)) {
			return fmt.Errorf("%q leaves the batch folder", h.Name)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
}

// pack writes the files of dir into the gzip-compressed tar archive path.
func pack(dir, path string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	tmp := path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: e.Name(), Mode: 0o644, Size: fi.Size()}); err != nil {
			return err
		}
		src, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, src)
		src.Close()
		if err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path) // the result archive appears whole or not at all
}
