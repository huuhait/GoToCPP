package doc

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// probe returns the HeaderProbe of the run: Env.Probe, or the C++ compiler
// of the target.
func (r *runner) probe() HeaderProbe {
	if r.env.Probe != nil {
		return r.env.Probe
	}
	goos, goarch := r.env.goTarget()
	return &compilerProbe{goos: goos, goarch: goarch, dir: r.dir}
}

// compilerProbe asks the C++ compiler of the target, with the target's flags (build.ProbeTargetFlags)
// and, for windows, the Win32 defaults (WIN32_LEAN_AND_MEAN, NOMINMAX, UNICODE, _UNICODE) that a
// package for windows is compiled with (Phase 14 step 14.6, ADR-116).
type compilerProbe struct {
	goos, goarch string
	dir          string
}

func (p *compilerProbe) cxx(ctx context.Context) (build.CXX, error) {
	return build.FindCXXFor(ctx, p.goos, p.goarch)
}

// FindHeader preprocesses "#include <name>" with -M, which lists the files
// that were read: the first one that ends with /name is the header (the
// compiler's own predefined header comes before it). The compiler runs like
// the other probes: in a process group of its own, within interop.ProbeLimit,
// and stopped when it waits for a named pipe (build.WatchPipes: a header of
// the include path that is one).
func (p *compilerProbe) FindHeader(ctx context.Context, name string) (string, error) {
	cxx, err := p.cxx(ctx)
	if err != nil {
		return "", err
	}
	// The header is the target's: the flags of the target and the Win32 defaults, as for the build.
	args := append(build.ProbeTargetFlags(cxx), build.Win32DefaultsFor(p.goos, nil)...)
	ctx, cancel := context.WithTimeout(ctx, interop.ProbeLimit())
	defer cancel()
	var out, errb bytes.Buffer
	cmd, err := build.Start(ctx, func() *exec.Cmd {
		out.Reset()
		errb.Reset()
		cmd := cxx.Command(ctx, append(args, "-E", "-M", "-x", "c++", "-")...)
		cmd.Dir = p.dir
		cmd.Stdin = strings.NewReader("#include <" + name + ">\n")
		cmd.Stdout, cmd.Stderr = &out, &errb
		cmd.WaitDelay = 5 * time.Second // a process that it started and that keeps the output open
		build.InGroup(cmd)
		return cmd
	})
	if err != nil {
		return "", fmt.Errorf("%s didn't start: %w", cxx.Name(), err)
	}
	stop := build.WatchPipes(cmd)
	err = cmd.Wait()
	pipeErr := stop()
	build.KillGroup(cmd) // whatever it left running
	switch {
	case pipeErr != nil:
		return "", fmt.Errorf("%s: %w", cxx.Name(), pipeErr)
	case ctx.Err() != nil:
		return "", fmt.Errorf("%s was stopped: %w", cxx.Name(), ctx.Err())
	case err != nil:
		return "", nil // not found: the header has no file
	}
	text := strings.ReplaceAll(out.String(), "\\\n", " ")
	for _, f := range strings.Fields(text) {
		if strings.HasSuffix(f, "/"+name) || f == name {
			return f, nil
		}
	}
	return "", nil
}

// Kind runs the kind probe.
func (p *compilerProbe) Kind(ctx context.Context, includes []string, path string) (string, error) {
	cxx, err := p.cxx(ctx)
	if err != nil {
		return "", err
	}
	res, err := interop.ProbeKinds(ctx, cxx, p.dir, build.Win32DefaultsFor(p.goos, nil), includes, []string{path})
	if err != nil {
		return "", err
	}
	if len(res.IncludeErrors) > 0 {
		e := res.IncludeErrors[0]
		return "", fmt.Errorf("%s: %s", e.Include, e.Msg)
	}
	switch k := res.Kinds[path]; k {
	case interop.Namespace:
		return "namespace", nil
	case interop.Type:
		return "type", nil
	case interop.Template:
		return "template", nil
	case interop.Value:
		return "value", nil
	case interop.ValueTemplate:
		return "value template", nil
	case interop.Macro:
		return "macro", nil
	case interop.NotFound:
		return "not found", nil
	default:
		return "unknown", nil
	}
}
