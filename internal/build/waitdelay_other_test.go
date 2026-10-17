//go:build !unix

package build

import "time"

func holdersLeft(string) []int { return nil }

func killHolders(string) {}

func waitUntil(time.Duration, func() bool) bool { return true }
