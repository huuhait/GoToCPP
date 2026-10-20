//go:build linux

package rtfiles_test

import (
	"bytes"
	"testing"
)

// TestTestingNatives builds test/testing_natives_test.cpp with the runtime and the natives of package
// testing (the OS part is src/os/linux/testing_os.cpp) and runs it: the environment, temporary folders,
// the standard output capture, the folder and file functions. The same program runs on Windows
// in TestWindowsRuntime (src/os/windows/testing_os.cpp), where the cases that depend on the OS
// are the other half of its #ifdef. Its temporary files go to a folder of this test.
func TestTestingNatives(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			for _, m := range modes[:2] { // O0 and O2; ASan would see the allocation counters' operator new
				t.Run(m.name, func(t *testing.T) {
					t.Parallel()
					bins := c.startBuilds(t, rt, t.TempDir(), m.flags, []prog{
						{src: "test/testing_natives_test.cpp", weight: 4, testing: true},
					})
					bin := bins.bin(t, "test/testing_natives_test.cpp")
					tmp := t.TempDir()
					stdout, stderr, code, err := command(tmp, []string{"TMPDIR=" + tmp}, false, bin)
					if err != nil {
						t.Fatal(err)
					}
					if code != 0 || len(stderr) != 0 || !bytes.HasPrefix(stdout, []byte("ok: ")) {
						t.Fatalf("exit code %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
					}
					t.Logf("%s", bytes.TrimSpace(stdout))
				})
			}
		})
	}
}
