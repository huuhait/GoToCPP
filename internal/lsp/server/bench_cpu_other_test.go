//go:build !unix

package server

// processCPU is not measured on this system.
func processCPU() int64 { return 0 }
