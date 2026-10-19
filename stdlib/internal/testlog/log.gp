// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testlog provides a back channel for os and other packages to
// communicate with testing.
//
// Copied from go1.25.14's src/internal/testlog (see stdlib/internal/PORT.md). The logger is a plain variable here, not
// an atomic.Pointer[Interface]: sync/atomic's generic types are not built yet (Phase 07 step 7.3). Go
// installs the logger once, before the tests run, which is the only thing the variable relies on.
package testlog

// Interface is the interface required of test loggers.
// The method signatures are the same as the methods defined by *testing.M.
type Interface interface {
	// Getenv is called to report that os.Getenv was called.
	Getenv(key string)

	// Stat is called to report that os.Stat, os.Lstat or os.IsNotExist was called.
	Stat(file string)

	// Open is called to report that os.Open or os.OpenFile was called.
	Open(file string)

	// Chdir is called to report that os.Chdir was called.
	Chdir(dir string)
}

var logger Interface

// SetLogger sets the test logger implementation for the current process.
// It must be called only once, at process startup.
func SetLogger(impl Interface) {
	if logger != nil {
		panic("testlog: SetLogger must be called only once")
	}
	logger = impl
}

// Logger returns the current test logger implementation.
// It returns nil if there is no logger.
func Logger() Interface {
	return logger
}

// Getenv calls Logger().Getenv, if a logger has been set.
func Getenv(name string) {
	if log := Logger(); log != nil {
		log.Getenv(name)
	}
}

// Open calls Logger().Open, if a logger has been set.
func Open(name string) {
	if log := Logger(); log != nil {
		log.Open(name)
	}
}

// Stat calls Logger().Stat, if a logger has been set.
func Stat(name string) {
	if log := Logger(); log != nil {
		log.Stat(name)
	}
}
