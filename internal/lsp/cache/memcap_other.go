//go:build !linux

package cache

// physicalMemory: not known here (0): the default is a middle value.
func physicalMemory() int64 { return 0 }
