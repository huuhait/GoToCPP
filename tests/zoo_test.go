package tests

import (
	"path/filepath"
	"strings"
	"testing"
)

// zooFormat is the format of the header zoo (tests/zoo): an interop run test that may also say
// // display: required. See tests/zoo/README.md.
var zooFormat = format{"zoo", outputLine | exitCodeLine | stderrLine | pkgConfigLine | displayLine | lsanLine | goBuildLine, true}

// TestZoo imports one header, or one library, per file in opaque mode and builds it with each
// compiler. The standard headers and the libraries that need no window are also run and compared
// with their // Output: block. A file with // display: required (SDL2, raylib) is only checked and
// built, never run: it would need a display, and a test must not open windows. Every file must
// pass gocpp check first, the way an editor would see it.
func TestZoo(t *testing.T) {
	c := setup(t)
	progs := loadTests(t, zooFormat)
	buildOnly := make(map[*program]bool)
	for _, p := range progs {
		if p.err == nil && p.header.display {
			// Not skipped without a display (runPrograms): built but not run.
			buildOnly[p] = true
			p.header.display = false
		}
	}
	c.runPrograms(t, "tests/zoo", progs, func(t *testing.T, p *program) {
		r := c.run(t, checkJob(p.dir, p.arg, p.header.phase))
		if r.problem != "" || r.exit != 0 {
			t.Fatalf("%s: %s\noutput:\n%s", r.cmd, r.why(), indent(string(r.stdout)))
		}
		if !buildOnly[p] {
			c.runTest(t, p)
			return
		}
		c.forEachCompiler(t, func(t *testing.T, cc compiler) {
			args := append(append([]string{"build"}, c.flags...), "-o", filepath.Join(t.TempDir(), "prog"), p.arg)
			j := gocppJob(p.dir, nil, cc.cxx, p.header.phase, args...)
			j.combined = true
			r := c.run(t, j)
			if r.problem != "" || r.exit != 0 {
				t.Fatalf("%s: %s\noutput:\n%s", r.cmd, r.why(), indent(string(r.stdout)))
			}
			if out := strings.TrimSpace(string(r.stdout)); out != "" {
				t.Errorf("%s printed:\n%s", r.cmd, indent(out))
			}
		})
	})
}
