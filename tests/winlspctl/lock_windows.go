package main

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

// lockFor locks the whole file path with LockFileEx (exclusive) for ms milliseconds. Another handle's read of the locked
// region fails with ERROR_LOCK_VIOLATION (33), which is what Go's os.ReadFile gets while a program has a byte-range lock
// (a share mode of "none" did not stop the read on the PC: the region lock does).
func lockFor(path string, ms int) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	k := syscall.NewLazyDLL("kernel32.dll")
	lock, unlock := k.NewProc("LockFileEx"), k.NewProc("UnlockFileEx")
	var ov syscall.Overlapped
	size := uint32(fi.Size())
	const exclusive = 2
	if r, _, e := lock.Call(f.Fd(), exclusive, 0, uintptr(size), 0, uintptr(unsafe.Pointer(&ov))); r == 0 {
		return e
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
	unlock.Call(f.Fd(), 0, uintptr(size), 0, uintptr(unsafe.Pointer(&ov)))
	return nil
}
