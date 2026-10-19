//go:build windows

package fsretry

import "syscall"

func openNoDeleteShare(name string) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	return syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
}

func closeHandle(h syscall.Handle) { syscall.CloseHandle(h) }
