package doc

import (
	"bytes"
	"context"
	"fmt"
	"strings"

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

// compilerProbe asks the C++ compiler of the target.
type compilerProbe struct {
	goos, goarch string
	dir          string
}

func (p *compilerProbe) cxx(ctx context.Context) (build.CXX, error) {
	return build.FindCXXFor(ctx, p.goos, p.goarch)
}

// FindHeader preprocesses "#include <name>" with -M, which lists the files
// that were read: the first one that ends with /name is the header (the
// compiler's own predefined header comes before it).
func (p *compilerProbe) FindHeader(ctx context.Context, name string) (string, error) {
	cxx, err := p.cxx(ctx)
	if err != nil {
		return "", err
	}
	cmd := cxx.Command(ctx, "-E", "-M", "-x", "c++", "-")
	cmd.Dir = p.dir
	cmd.Stdin = strings.NewReader("#include <" + name + ">\n")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
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
	res, err := interop.ProbeKinds(ctx, cxx, p.dir, nil, includes, []string{path})
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
