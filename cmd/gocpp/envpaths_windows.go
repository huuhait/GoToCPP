//go:build windows

package main

import (
	"os"

	"github.com/huuhait/gocpp/internal/winpath"
)

func init() { normalizeEnvPaths(osPathsEnv(winpath.ACPSafe, winpath.Short, winpath.PrivateFolder)) }

func osPathsEnv(safe func(string) bool, short func(string) (string, bool), private func(string, ...string) (string, error)) pathsEnv {
	return pathsEnv{
		getenv: os.Getenv,
		setenv: func(k, v string) { os.Setenv(k, v) },
		mkdir:  func(p string) error { return os.MkdirAll(p, 0o755) },
		safe:   safe, short: short, private: private,
		program: os.Getenv("ProgramData"),
	}
}
