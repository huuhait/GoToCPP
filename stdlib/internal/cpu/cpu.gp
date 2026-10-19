// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cpu is a stub of go1.25.14's src/internal/cpu (see stdlib/internal/PORT.md). Go's package detects the
// processor's features with assembly (CPUID), for the assembly routines and the dispatching code of
// the standard library. gocpp ports the pure-Go (purego) variants of every package and its native
// functions (bytealg) are C++ that the C++ compiler dispatches, so no ported package may take a
// different path by feature: every variable here is false (or zero) on every CPU. The names are the
// ones that Go's packages read, so a ported file that tests cpu.X86.HasFMA still compiles and takes
// its portable branch.
package cpu

// DebugOptions is set to true by the runtime if the OS supports reading
// GODEBUG early in runtime startup. It is always false in gocpp.
var DebugOptions bool

// CacheLinePad is used to pad structs to avoid false sharing.
type CacheLinePad struct{ _ [CacheLinePadSize]byte }

// CacheLineSize is the CPU's assumed cache line size.
// There is currently no runtime detection of the real cache line size
// so we use the constant per GOARCH CacheLinePadSize as an approximation.
var CacheLineSize uintptr = CacheLinePadSize

// The booleans in X86 contain the correspondingly named cpuid feature bit.
// They are all false in gocpp.
var X86 struct {
	_            CacheLinePad
	HasAES       bool
	HasADX       bool
	HasAVX       bool
	HasAVX2      bool
	HasAVX512F   bool
	HasAVX512BW  bool
	HasAVX512VL  bool
	HasBMI1      bool
	HasBMI2      bool
	HasERMS      bool
	HasFSRM      bool
	HasFMA       bool
	HasOSXSAVE   bool
	HasPCLMULQDQ bool
	HasPOPCNT    bool
	HasRDTSCP    bool
	HasSHA       bool
	HasSSE3      bool
	HasSSSE3     bool
	HasSSE41     bool
	HasSSE42     bool
	_            CacheLinePad
}

// The booleans in ARM64 contain the correspondingly named cpu feature bit.
// They are all false in gocpp.
var ARM64 struct {
	_          CacheLinePad
	HasAES     bool
	HasPMULL   bool
	HasSHA1    bool
	HasSHA2    bool
	HasSHA512  bool
	HasSHA3    bool
	HasCRC32   bool
	HasATOMICS bool
	HasCPUID   bool
	HasDIT     bool
	IsNeoverse bool
	_          CacheLinePad
}

// Name returns the CPU name given by the vendor.
// If the CPU name can not be determined an
// empty string is returned.
func Name() string {
	return ""
}
