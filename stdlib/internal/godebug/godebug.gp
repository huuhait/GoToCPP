// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package godebug makes the settings in the $GODEBUG environment variable
// available to other packages. This is gocpp's small replacement of go1.25.14's
// src/internal/godebug (see stdlib/internal/PORT.md): a Setting reads the variable when it is asked,
// there is no table of known settings with defaults (so Value is "" unless $GODEBUG
// names the setting), no metrics, and no hook that the runtime calls when os.Setenv
// changes the variable (every Value reads the current value).
package godebug

// A Setting is a single setting in the $GODEBUG environment variable.
type Setting struct {
	name         string
	undocumented bool
}

// env returns the value of $GODEBUG (runtime/src/shims.cpp).
//
//gocpp:native gp::godebug::env
func env() string

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
// Here the cache is $GODEBUG itself: the value of the last "name=value"
// in the variable, as Go reads it (a later setting overrides an earlier one).
func (s *Setting) Value() string {
	e := env()
	value := ""
	for e != "" {
		var field string
		i := indexByte(e, ',')
		if i < 0 {
			field, e = e, ""
		} else {
			field, e = e[:i], e[i+1:]
		}
		if k, v, ok := cut(field, '='); ok && k == s.name {
			value = v
		}
	}
	return value
}

// indexByte and cut are here because the package imports nothing.
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
