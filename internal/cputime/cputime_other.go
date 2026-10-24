//go:build !linux

package cputime

import "time"

// threadTime has no clock of a thread's processor time here: Of uses the wall time.
func threadTime() (time.Duration, bool) { return 0, false }
