//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A build that is killed (SIGKILL: no cleanup runs) when the generated files are written, before
// anything is compiled or linked, leaves the cache in a state that the next build can use: no
// stamp of the killed build exists, the next build works (a stamp or an object that was half
// written would be read as a miss), and the program it makes is right.
func TestUpToDateKilledBuild(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		marker := filepath.Join(t.TempDir(), "marker")
		cmd, _, _, _ := startGocpp(t, s.dir, []string{"GOCPP_TEST_HANG_AFTER_GEN=" + marker, "GOCPP_NOOBJCACHE=0",
			"GOCPP_GATE_PHASE=06", "GOCPP_CXX=" + c.name, "CXX="}, "build", "-o", "prog", ".")
		deadline := time.Now().Add(2 * time.Minute)
		for {
			if _, err := os.Stat(marker); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the build did not reach the writing of the generated files")
			}
			time.Sleep(20 * time.Millisecond)
		}
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		cmd.Wait()
		if _, err := os.Stat("prog"); err == nil {
			t.Fatal("the killed build left a program")
		}
		// No stamp for the killed build: the next build is not up to date, and works.
		r, n := s.build()
		if isUpToDate(r.stderr) || n.link != 1 || n.compile != 4 {
			t.Errorf("the build after the kill did %+v:\n%s", n, r.stderr)
		}
		if got := s.run(); got != "8 10 "+s.stamp+"\n" {
			t.Errorf("the program prints %q", got)
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date after the build that followed the kill:\n%s", r.stderr)
		}
	})
}

// A stamp that is cut short, or is not JSON, or says something else is no stamp: the build is not up
// to date, works, and writes a good one.
func TestUpToDateDamagedStamp(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		s.build()
		stamps, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_CACHE_HOME"), "gocpp", "stamp", "*.json"))
		if len(stamps) == 0 {
			t.Fatal("no stamp in the cache folder")
		}
		for _, damage := range []func(data []byte) []byte{
			func(data []byte) []byte { return data[:len(data)/2] },
			func(data []byte) []byte { return []byte("not json") },
			func(data []byte) []byte { return []byte(strings.Replace(string(data), `"v":"2"`, `"v":"0"`, 1)) },
			func(data []byte) []byte { return []byte(strings.Replace(string(data), `"key":"`, `"key":"0`, 1)) },
			func(data []byte) []byte { return nil },
		} {
			for _, f := range stamps {
				data, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f, damage(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			r, n := s.build()
			if isUpToDate(r.stderr) || n.link != 1 || n.compile != 0 {
				t.Errorf("a damaged stamp: %+v:\n%s", n, r.stderr)
			}
			if r, _ := s.build(); !isUpToDate(r.stderr) {
				t.Errorf("not up to date after the stamp was written again:\n%s", r.stderr)
			}
		}
	})
}
