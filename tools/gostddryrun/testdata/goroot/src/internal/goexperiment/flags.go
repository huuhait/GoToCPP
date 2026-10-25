// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goexperiment

// Flags is the set of experiments that can be enabled or disabled.
type Flags struct {
	FieldTrack        bool
	PreemptibleLoops  bool
	StaticLockRanking bool
	BoringCrypto      bool
	RegabiWrappers    bool
	RegabiArgs        bool
	HeapMinimum512KiB bool
	Arenas            bool
	CgoCheck2         bool
	LoopVar           bool
	CacheProg         bool
	NewInliner        bool
	RangeFunc         bool
	AliasTypeParams   bool
	SwissMap          bool
	SyncHashTrieMap   bool
	Synctest          bool
	Dwarf5            bool
	JSONv2            bool
	GreenTeaGC        bool
}
