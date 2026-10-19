// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package time

// Windows has no zoneinfo directory. Go reads the registry for the local zone and the
// zone abbreviations (its zoneinfo_windows.go) and has its own zoneinfo.zip under GOROOT;
// gocpp does neither yet (Phase 14): LoadLocation finds a zone only where the ZONEINFO
// environment variable names a zoneinfo directory, and Local is UTC.
var platformZoneSources = []string{}

func initLocal() {
	localLoc.name = "UTC"
}
