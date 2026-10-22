//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnreadableHeaderOneError: a local header without permission is reported once at its import,
// with the compiler's message, by check, build and run alike.
func TestUnreadableHeaderOneError(t *testing.T) {
	for _, command := range []string{"check", "build", "run"} {
		t.Run(command, func(t *testing.T) {
			dir := inDir(t, map[string]string{
				"gocpp.mod": "module example.com/m\n",
				"main.gp":   "package main\n\nimport \"./noperm.h\"\n\nfunc main() {}\n",
				"noperm.h":  "int noperm(void);\n",
			})
			path := filepath.Join(dir, "noperm.h")
			if err := os.Chmod(path, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(path, 0o644) })
			if f, err := os.Open(path); err == nil {
				f.Close()
				t.Skip("the file can be read without permission (root?)")
			}
			got := gocppRun("", false, command, ".")
			if got.code == 0 || strings.Count(got.stderr, "./main.gp:3:8:") != 1 {
				t.Errorf("gocpp %s .: exit %d, stderr %q; want one error at the import", command, got.code, got.stderr)
			}
			if !strings.Contains(got.stderr, `header "./noperm.h" can't be read: `) ||
				!strings.Contains(got.stderr, "Permission denied") {
				t.Errorf("gocpp %s .: stderr %q; want the compiler's \"can't be read\" message", command, got.stderr)
			}
			if strings.Contains(got.stderr, "cannot check whether header") {
				t.Errorf("gocpp %s .: stderr %q; want no parse-as-C error", command, got.stderr)
			}
		})
	}
}
