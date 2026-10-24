// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package godebug makes the settings in the $GODEBUG environment variable
// available to other packages. This is gocpp's small replacement of go1.25.14's
// src/internal/godebug (see stdlib/internal/PORT.md): a Setting keeps the value it read from the
// variable until the program sets or removes $GODEBUG through package syscall (os.Setenv and the
// rest: the runtime's generation), as Go's cache does; there is no table of known settings with
// defaults (so Value is "" unless $GODEBUG names the setting) and no metrics.
package godebug

import "sync/atomic"

// A Setting is a single setting in the $GODEBUG environment variable.
type Setting struct {
	name         string
	undocumented bool
	cache        atomic.Pointer[value]
}

// value is the value of a Setting read at a generation of the environment.
type value struct {
	gen  uint64
	text string
}

// env returns the value of $GODEBUG (runtime/src/shims.cpp).
//
//gocpp:native gp::godebug::env
func env() string

// generation returns a number that changes after the program sets or removes $GODEBUG through
// package syscall (runtime/src/shims.cpp).
//
//gocpp:native gp::godebug::generation
func generation() uint64

// New returns a new Setting for the $GODEBUG setting with the given name.
//
// GODEBUGs meant for use by end users must be listed in ../godebugs/table.go,
// which is used for generating and checking various documentation.
// If the name is not listed in that table, New will succeed but calling Value
// on the returned Setting will panic.
// To disable that panic for access to an undocumented setting,
// prefix the name with a #, as in godebug.New("#gofsystrace").
// The # is a signal to New but not part of the key used in $GODEBUG.
//
// gocpp has no table, so Value never panics.
func New(name string) *Setting {
	if len(name) > 0 && name[0] == '#' {
		return &Setting{name: name[1:], undocumented: true}
	}
	return &Setting{name: name}
}

// Name returns the name of the setting.
func (s *Setting) Name() string {
	return s.name
}

// Undocumented reports whether this is an undocumented setting.
func (s *Setting) Undocumented() bool {
	return s.undocumented
}

// String returns a printable form for the setting: name=value.
func (s *Setting) String() string {
	return s.name + "=" + s.Value()
}

// IncNonDefault increments the non-default behavior counter
// associated with the given setting. gocpp keeps no counters, so it does nothing.
func (s *Setting) IncNonDefault() {
}

// Value returns the current value for the GODEBUG setting s.
//
// Value maintains an internal cache that is synchronized
// with changes to the $GODEBUG environment variable,
// making Value efficient to call as frequently as needed.
// Clients should therefore typically not attempt their own
// caching of Value's result.
//
// Here the cache is the value read at the current generation of the environment: the value of the
// last "name=value" in the variable, as Go reads it (a later setting overrides an earlier one).
func (s *Setting) Value() string {
	g := generation()
	if c := s.cache.Load(); c != nil && c.gen == g {
		return c.text
	}
	v := lookup(env(), s.name)
	// (A change after generation() was read gives a later generation: the next Value reads again.)
	s.cache.Store(&value{gen: g, text: v})
	return v
}

// lookup returns the value of the last name=value in the $GODEBUG text e.
func lookup(e, name string) string {
	value := ""
	for e != "" {
		var field string
		i := indexByte(e, ',')
		if i < 0 {
			field, e = e, ""
		} else {
			field, e = e[:i], e[i+1:]
		}
		if k, v, ok := cut(field, '='); ok && k == name {
			value = v
		}
	}
	return value
}

// indexByte and cut are here because the package imports no strings package.
func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func cut(s string, c byte) (before, after string, found bool) {
	if i := indexByte(s, c); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return s, "", false
}
