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

// openForDelete opens name with DELETE access (and shares everything): this is how a rename holds
// the file it replaces. Go opens files without FILE_SHARE_DELETE, so while this handle is open,
// os.Open of the same file fails with a sharing violation.
func openForDelete(name string) (syscall.Handle, error) {
	const deleteAccess = 0x10000
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	share := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE | syscall.FILE_SHARE_DELETE)
	return syscall.CreateFile(p, deleteAccess|syscall.GENERIC_READ, share, nil, syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL, 0)
}
