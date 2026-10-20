//go:build unix

package typedinterop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// A check does not return while the compiler that makes the precompiled header is running (probe/pch.go,
// WaitPCH): the compiler here is a script that takes its time over the header (2 s) and writes a marker
// after the real compiler is done.
func TestAnalyzeWaitsForTheHeaderBuild(t *testing.T) {
	real := tsupport.Clang(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "end")
	script := filepath.Join(dir, "clang++")
	body := "#!/bin/sh\ncase \" $* \" in *\" -x c++-header \"*)\n  sleep 2\n  '" + real.Path +
		"' \"$@\"\n  status=$?\n  echo end >> '" + marker + "'\n  exit $status;;\nesac\nexec '" + real.Path + "' \"$@\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	clang := build.CXX{Path: script, Family: build.Clang, Version: real.Version, Args: real.Args}
	t.Setenv(probe.EnvProbeCache, t.TempDir())
	t.Setenv(probe.EnvNoProbeCache, "")
	t.Setenv(probe.EnvPCH, "")
	for _, name := range zooPrograms(t) {
		p, fset := tsupport.CheckDir(t, zooDir(t), name)
		if len(p.Errors.Diagnostics()) > 0 {
			continue
		}
		rn := new(probe.Runner)
		if _, err := Analyze(context.Background(), p, fset, Options{Clang: clang, Runner: rn, Decls: true}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Errorf("%s: Analyze returned before the compiler of the header was done", name)
		}
		return
	}
	t.Skip("no program of the zoo checks here")
}
