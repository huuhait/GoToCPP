// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copied from go1.25.14's src/internal/race/norace.go (see stdlib/internal/PORT.md): gocpp has no race detector
// (ThreadSanitizer builds are a separate tool), so every call is a no-op. The functions that take
// an *abi.Type (ReadObjectPC, WriteObjectPC) are not here: nothing that gocpp ports calls them.

//go:build !race

package race

import (
	"unsafe"
)

const Enabled = false

func Acquire(addr unsafe.Pointer) {
}

func Release(addr unsafe.Pointer) {
}

func ReleaseMerge(addr unsafe.Pointer) {
}

func Disable() {
}

func Enable() {
}

func Read(addr unsafe.Pointer) {
}

func ReadPC(addr unsafe.Pointer, callerpc, pc uintptr) {
}

func Write(addr unsafe.Pointer) {
}

func WritePC(addr unsafe.Pointer, callerpc, pc uintptr) {
}

func ReadRange(addr unsafe.Pointer, len int) {
}

func WriteRange(addr unsafe.Pointer, len int) {
}

func Errors() int { return 0 }
