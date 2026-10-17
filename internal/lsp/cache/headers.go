package cache

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/memoize"
)

// The header files of header imports, for go to definition on an import
// path (step 11.5): import "./engine.hpp" opens engine.hpp, and
// import "<vector>" opens the vector header of the C++ compiler. It is a
// lookup like the compiler's own (the namespace probe runs the compiler
// in the folder of the .gp file, with that folder as -iquote): a quoted
// header is looked for in the folder of the .gp file first, then in the
// compiler's system folders; a header in <> only in the system folders.
// The flags of //gocpp:cflags and //gocpp:pkg-config (-I) don't count yet,
// as for the probe (Phase 04). Phase 12 brings a real C/C++ index.

// includeDirsTimeout limits the compiler run that lists the system folders.
const includeDirsTimeout = 10 * time.Second

// HeaderFile returns the file that the header import inc of a .gp file in
// the folder dir includes, or "" when it is not found. For a header in <>,
// it needs the C++ compiler, which the pipeline looks for in the background
// (probe.go): until it is found, such a header is not found either. The
// compiler's list of system folders is asked for once per compiler (a
// compiler run of about 20 ms, which the first call waits for). An absolute
// path is the file itself, as for the compiler: no folder is searched.
func (p *Pipeline) HeaderFile(ctx context.Context, dir string, inc *load.Include) (string, error) {
	if inc.File != "" {
		// A module header: the loader found the file, on disk or in the
		// overlay of the snapshot (an unsaved buffer), so it is not looked at again.
		return filepath.FromSlash(inc.File), nil
	}
	name := strings.TrimSuffix(strings.TrimPrefix(inc.Path, "<"), ">")
	if filepath.IsAbs(filepath.FromSlash(name)) {
		if path := filepath.Clean(filepath.FromSlash(name)); isFile(path) {
			return path, nil
		}
		return "", nil
	}
	if inc.Kind == load.LocalHeader {
		if path := filepath.Join(dir, filepath.FromSlash(name)); isFile(path) {
			return path, nil
		}
	}
	cxx, ok := p.prober.compiler()
	if !ok {
		return "", nil
	}
	dirs, err := p.includeDirs(ctx, cxx)
	if err != nil {
		return "", err
	}
	for _, d := range dirs {
		if path := filepath.Join(d, filepath.FromSlash(name)); isFile(path) {
			return path, nil
		}
	}
	return "", nil
}

// HeaderFile is Pipeline.HeaderFile with the pipeline of the snapshot.
func (s *Snapshot) HeaderFile(ctx context.Context, dir string, inc *load.Include) (string, error) {
	st, ok := s.state.(*state)
	if !ok {
		return "", errNoPipeline
	}
	return st.p.HeaderFile(ctx, dir, inc)
}

func isFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// includeDirs returns the system include folders of the compiler cxx, in
// the order of its search: from Config.IncludeDirs, or else from the
// compiler (systemIncludeDirs), once per compiler. A run that failed (also
// one that timed out) is kept for cxxRetry, like an error to find the
// compiler, then the compiler is asked again.
func (p *Pipeline) includeDirs(ctx context.Context, cxx build.CXX) ([]string, error) {
	key := strings.Join(append([]string{cxx.Path, cxx.Version, cxx.Family.String()}, cxx.Args...), "\x00")
	p.incMu.Lock()
	if p.incDirs == nil {
		p.incDirs = make(map[string]*includeDirsEntry)
	}
	e := p.incDirs[key]
	if e != nil && !e.failed.IsZero() && time.Since(e.failed) >= cxxRetry {
		e = nil
	}
	if e == nil {
		find := p.cfg.IncludeDirs
		if find == nil {
			find = systemIncludeDirs
		}
		e = &includeDirsEntry{}
		e.pr = memoize.NewPromise(func(ctx context.Context) ([]string, error) {
			dirs, err := find(ctx, cxx)
			if err != nil && ctx.Err() == nil { // else the promise drops it
				p.incMu.Lock()
				e.failed = time.Now()
				p.incMu.Unlock()
			}
			return dirs, err
		})
		p.incDirs[key] = e
	}
	p.incMu.Unlock()
	return e.pr.Get(ctx)
}

// systemIncludeDirs asks the compiler cxx for its search list of
// #include <...>: with -v, both g++ and clang++ print it on stderr between
// "#include <...> search starts here:" and "End of search list.".
func systemIncludeDirs(ctx context.Context, cxx build.CXX) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, includeDirsTimeout)
	defer cancel()
	cmd := cxx.Command(ctx, "-x", "c++", "-E", "-v", "-o", os.DevNull, "-")
	var stderr bytes.Buffer
	cmd.Stdin = strings.NewReader("")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return parseIncludeDirs(stderr.Bytes()), nil
}

// parseIncludeDirs reads the search list of #include <...> from the output
// of a compiler run with -v. clang marks a macOS framework folder with
// " (framework directory)", which is no folder of headers.
func parseIncludeDirs(out []byte) []string {
	var dirs []string
	in := false
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "#include <...> search starts here"):
			in = true
		case strings.HasPrefix(line, "End of search list"):
			return dirs
		case in && line != "" && !strings.HasSuffix(line, "(framework directory)"):
			dirs = append(dirs, filepath.Clean(line))
		}
	}
	return dirs
}

// The include folders of the pipeline's compilers, for HeaderFile.
type includeDirsCache struct {
	incMu   sync.Mutex
	incDirs map[string]*includeDirsEntry
}

// An includeDirsEntry is the search list of one compiler.
type includeDirsEntry struct {
	pr     *memoize.Promise[[]string]
	failed time.Time // when the run failed, or zero; guarded by incMu
}
