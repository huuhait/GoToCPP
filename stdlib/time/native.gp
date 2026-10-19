// Copyright 2026 The gocpp Authors. Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

// The native helpers of the package that every platform needs (the ones for files and the zone
// database are in sys_unix.gp). The C++ side is runtime/include/gocpp/time.hpp.

// The identifiers of the package's once-only initializations (callOnce).
const (
	onceLocal      = 0 // initLocal
	onceFixedZones = 1 // the table of the unnamed fixed zones
	onceZoneinfo   = 2 // the ZONEINFO environment variable
)

// callOnce runs f once, however many goroutines call it at the same time: the others wait until
// the call has returned. If f panics, the next call tries again (sync.Once is Phase 08 step 8.7's;
// this is std::call_once on one flag for each id).
//
//gocpp:native gp::time::call_once
func callOnce(id int, f func())

// getenv returns the value of the environment variable key, and whether it is set.
//
//gocpp:native gp::time::getenv
func getenv(key string) (value string, found bool)
