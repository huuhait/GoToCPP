// Copyright 2026 The gocpp Authors. Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

// The native helpers of the package that every platform needs (the ones for files and the zone
// database are in sys_unix.gp). The C++ side is runtime/include/gocpp/time.hpp.

// getenv returns the value of the environment variable key, and whether it is set.
//
//gocpp:native gp::time::getenv
func getenv(key string) (value string, found bool)
