// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The file handle layer of package os on Windows: go1.25.14's internal/poll/fd_windows.go reduced
// for gocpp (stdlib/internal/poll/PORT.md). A goroutine is an OS thread (ADR-007), so every handle is
// opened without FILE_FLAG_OVERLAPPED and a read or write waits in the kernel: there is no I/O
// completion port, no operation/overlapped bookkeeping and no socket code. Go's rules stay: the
// reference count and the closed bit (fdMutex), the 1 GiB cut, io.EOF for ERROR_HANDLE_EOF and a
// closed pipe, the console in UTF-8, pread/pwrite emulated with the overlapped offset, and the
// pipe's Close that cancels the I/O of the threads blocked on it (CancelIoEx).

package poll

import (
	"internal/syscall/windows"
	"io"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

// sysFlushStdio flushes C's stdio (gp::detail::flush_c_stdio): what C or C++ code printf'd
// comes out before what Go writes to a standard handle (the order rule of the fmt shim and the
// builtin print).
//
//gocpp:native gp::syscall::sys_flush_stdio
func sysFlushStdio()

// fileKind describes the kind of file.
type fileKind byte

const (
	kindNet fileKind = iota
	kindFile
	kindConsole
	kindPipe
	kindFileNet
)

// FD is a file descriptor. The os package embeds this type in a larger type representing an
// OS file.
type FD struct {
	// Lock sysfd and serialize access to Read and Write methods.
	fdmu fdMutex

	// System file descriptor. Immutable until Close.
	Sysfd syscall.Handle

	// Used to implement pread/pwrite.
	l sync.Mutex

	// For console I/O.
	lastbits       []byte   // first few bytes of the last incomplete rune in last write
	readuint16     []uint16 // buffer to hold uint16s obtained with ReadConsole
	readbyte       []byte   // buffer to hold decoding of readuint16 from utf16 to utf8
	readbyteOffset int      // readbyte[readOffset:] is yet to be consumed with file.Read

	// Whether this is a streaming descriptor, as opposed to a
	// packet-based descriptor like a UDP socket.
	IsStream bool

	// Whether a zero byte read indicates EOF. This is false for a
	// message based socket connection.
	ZeroReadIsEOF bool

	// Whether the handle is owned by os.File.
	isFile bool

	// The kind of this file.
	kind fileKind
}

// CloseFunc is used to hook the close call.
var CloseFunc func(syscall.Handle) error = syscall.CloseHandle

// Init initializes the FD. The Sysfd field should already be set.
// This can be called multiple times on a single FD.
// The net argument is "file", "console", "pipe" or "file+net" (os) as in Go.
// pollable is accepted and ignored: gocpp has no poller, and every handle is blocking.
func (fd *FD) Init(net string, pollable bool) error {
	switch net {
	case "file":
		fd.kind = kindFile
	case "console":
		fd.kind = kindConsole
	case "pipe":
		fd.kind = kindPipe
	case "file+net":
		fd.kind = kindFileNet
	default:
		fd.kind = kindNet
	}
	fd.isFile = fd.kind != kindNet
	return nil
}

// DisassociateIOCP disassociates the file handle from the IOCP: there is none.
func (fd *FD) DisassociateIOCP() error {
	if err := fd.incref(); err != nil {
		return err
	}
	fd.decref()
	return nil
}

func (fd *FD) destroy() error {
	if fd.Sysfd == syscall.InvalidHandle {
		return syscall.EINVAL
	}
	err := CloseFunc(fd.Sysfd)
	fd.Sysfd = syscall.InvalidHandle
	return err
}

// Close closes the FD. The underlying file descriptor is closed by
// the destroy method when there are no remaining references.
func (fd *FD) Close() error {
	if !fd.fdmu.increfAndClose() {
		return errClosing(fd.isFile)
	}

	if fd.kind == kindPipe {
		// Cancel the I/O that other threads wait for on the pipe (their calls return
		// ERROR_OPERATION_ABORTED, which Read and Write turn into the closing error); Go does
		// the same for its overlapped pipes. A thread that has its reference but has not made its
		// system call yet does not see one cancel (nothing is pending for it), so the cancel is
		// repeated until it is the only reference left; Close waits for them as Go's does (csema).
		for fd.fdmu.refs() > 1 {
			syscall.CancelIoEx(fd.Sysfd, nil)
			time.Sleep(time.Millisecond)
		}
	}
	// The call to decref will call destroy if there are no other references. (A file or console
	// handle does not wait for a blocked call, as on Linux here.)
	return fd.decref()
}

// Windows ReadFile and WriteFile use DWORD (uint32) parameter to pass buffer length.
// This prevents us reading blocks larger than 4GB.
// See golang.org/issue/26923.
const maxRW = 1 << 30 // 1GB is large enough and keeps subsequent reads aligned

// ioError maps the error of a call that the closing of a pipe cancelled.
func (fd *FD) ioError(err error) error {
	if err == syscall.ERROR_OPERATION_ABORTED && fd.kind == kindPipe && fd.closing() {
		// Close uses CancelIoEx to interrupt concurrent I/O for pipes.
		return errClosing(fd.isFile)
	}
	return err
}

// Read implements io.Reader.
func (fd *FD) Read(buf []byte) (int, error) {
	if err := fd.readLock(); err != nil {
		return 0, err
	}
	defer fd.readUnlock()
	if fd.kind == kindFile {
		fd.l.Lock()
		defer fd.l.Unlock()
	}

	if len(buf) > maxRW {
		buf = buf[:maxRW]
	}

	var n int
	var err error
	switch fd.kind {
	case kindConsole:
		n, err = fd.readConsole(buf)
	case kindFile, kindPipe:
		if fd.kind == kindPipe && fd.closing() {
			return 0, errClosing(fd.isFile)
		}
		var qty uint32
		err = syscall.ReadFile(fd.Sysfd, buf, &qty, nil)
		n = int(qty)
		switch err {
		case syscall.ERROR_HANDLE_EOF:
			err = io.EOF
		case syscall.ERROR_BROKEN_PIPE:
			// ReadFile only documents ERROR_BROKEN_PIPE for pipes.
			if fd.kind == kindPipe {
				err = io.EOF
			}
		default:
			err = fd.ioError(err)
		}
	}
	if len(buf) != 0 {
		err = fd.eofError(n, err)
	}
	return n, err
}

var ReadConsole = syscall.ReadConsole // changed for testing

// readConsole reads utf16 characters from console File,
// encodes them into utf8 and stores them in buffer b.
// It returns the number of utf8 bytes read and an error, if any.
func (fd *FD) readConsole(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	if fd.readuint16 == nil {
		// Note: syscall.ReadConsole fails for very large buffers.
		// The limit is somewhere around (but not exactly) 16384.
		// Stay well below.
		fd.readuint16 = make([]uint16, 0, 10000)
		fd.readbyte = make([]byte, 0, 4*cap(fd.readuint16))
	}

	for fd.readbyteOffset >= len(fd.readbyte) {
		n := cap(fd.readuint16) - len(fd.readuint16)
		if n > len(b) {
			n = len(b)
		}
		var nw uint32
		err := ReadConsole(fd.Sysfd, &fd.readuint16[:len(fd.readuint16)+1][len(fd.readuint16)], uint32(n), &nw, nil)
		if err != nil {
			return 0, err
		}
		uint16s := fd.readuint16[:len(fd.readuint16)+int(nw)]
		fd.readuint16 = fd.readuint16[:0]
		buf := fd.readbyte[:0]
		for i := 0; i < len(uint16s); i++ {
			r := rune(uint16s[i])
			if utf16.IsSurrogate(r) {
				if i+1 == len(uint16s) {
					if nw > 0 {
						// Save half surrogate pair for next time.
						fd.readuint16 = fd.readuint16[:1]
						fd.readuint16[0] = uint16(r)
						break
					}
					r = utf8.RuneError
				} else {
					r = utf16.DecodeRune(r, rune(uint16s[i+1]))
					if r != utf8.RuneError {
						i++
					}
				}
			}
			buf = utf8.AppendRune(buf, r)
		}
		fd.readbyte = buf
		fd.readbyteOffset = 0
		if nw == 0 {
			break
		}
	}

	src := fd.readbyte[fd.readbyteOffset:]
	var i int
	for i = 0; i < len(src) && i < len(b); i++ {
		x := src[i]
		if x == 0x1A { // Ctrl-Z
			if i == 0 {
				fd.readbyteOffset++
			}
			break
		}
		b[i] = x
	}
	fd.readbyteOffset += i
	return i, nil
}

// setOffset sets the offset of an overlapped structure that carries the offset of a
// pread/pwrite (the file pointer of a synchronous handle moves too, and the caller puts it back).
func setOffset(o *syscall.Overlapped, off int64) {
	o.OffsetHigh, o.Offset = uint32(off>>32), uint32(off)
}

// Pread emulates the Unix pread system call.
func (fd *FD) Pread(b []byte, off int64) (int, error) {
	if fd.kind == kindPipe {
		// Pread does not work with pipes
		return 0, syscall.ESPIPE
	}
	// Call incref, not readLock, because since pread specifies the
	// offset it is independent from other reads.
	if err := fd.incref(); err != nil {
		return 0, err
	}
	defer fd.decref()

	if len(b) > maxRW {
		b = b[:maxRW]
	}

	fd.l.Lock()
	defer fd.l.Unlock()
	curoffset, err := syscall.Seek(fd.Sysfd, 0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	defer syscall.Seek(fd.Sysfd, curoffset, io.SeekStart)
	var o syscall.Overlapped
	setOffset(&o, off)
	var qty uint32
	err = syscall.ReadFile(fd.Sysfd, b, &qty, &o)
	n := int(qty)
	if err == syscall.ERROR_HANDLE_EOF {
		err = io.EOF
	}
	if len(b) != 0 {
		err = fd.eofError(n, err)
	}
	return n, err
}

// Write implements io.Writer.
func (fd *FD) Write(buf []byte) (int, error) {
	if err := fd.writeLock(); err != nil {
		return 0, err
	}
	defer fd.writeUnlock()
	if fd.kind == kindFile {
		fd.l.Lock()
		defer fd.l.Unlock()
	}
	if fd.Sysfd == syscall.Stdout || fd.Sysfd == syscall.Stderr {
		sysFlushStdio()
	}

	var ntotal int
	for {
		max := len(buf)
		if max-ntotal > maxRW {
			max = ntotal + maxRW
		}
		b := buf[ntotal:max]
		var n int
		var err error
		switch fd.kind {
		case kindConsole:
			n, err = fd.writeConsole(b)
		case kindPipe, kindFile:
			if fd.kind == kindPipe && fd.closing() {
				return ntotal, errClosing(fd.isFile)
			}
			var qty uint32
			err = syscall.WriteFile(fd.Sysfd, b, &qty, nil)
			n = int(qty)
			err = fd.ioError(err)
		}
		ntotal += n
		if ntotal == len(buf) || err != nil {
			return ntotal, err
		}
		if n == 0 {
			return ntotal, io.ErrUnexpectedEOF
		}
	}
}

// writeConsole writes len(b) bytes to the console File.
// It returns the number of bytes written and an error, if any.
func (fd *FD) writeConsole(b []byte) (int, error) {
	n := len(b)
	runes := make([]rune, 0, 256)
	if len(fd.lastbits) > 0 {
		b = append(fd.lastbits, b...)
		fd.lastbits = nil

	}
	for len(b) >= utf8.UTFMax || utf8.FullRune(b) {
		r, l := utf8.DecodeRune(b)
		runes = append(runes, r)
		b = b[l:]
	}
	if len(b) > 0 {
		fd.lastbits = make([]byte, len(b))
		copy(fd.lastbits, b)
	}
	// syscall.WriteConsole seems to fail, if given large buffer.
	// So limit the buffer to 16000 characters. This number was
	// discovered by experimenting with syscall.WriteConsole.
	const maxWrite = 16000
	for len(runes) > 0 {
		m := len(runes)
		if m > maxWrite {
			m = maxWrite
		}
		chunk := runes[:m]
		runes = runes[m:]
		uint16s := utf16.Encode(chunk)
		for len(uint16s) > 0 {
			var written uint32
			err := syscall.WriteConsole(fd.Sysfd, &uint16s[0], uint32(len(uint16s)), &written, nil)
			if err != nil {
				return 0, err
			}
			uint16s = uint16s[written:]
		}
	}
	return n, nil
}

// Pwrite emulates the Unix pwrite system call.
func (fd *FD) Pwrite(buf []byte, off int64) (int, error) {
	if fd.kind == kindPipe {
		// Pwrite does not work with pipes
		return 0, syscall.ESPIPE
	}
	// Call incref, not writeLock, because since pwrite specifies the
	// offset it is independent from other writes.
	if err := fd.incref(); err != nil {
		return 0, err
	}
	defer fd.decref()

	fd.l.Lock()
	defer fd.l.Unlock()
	curoffset, err := syscall.Seek(fd.Sysfd, 0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	defer syscall.Seek(fd.Sysfd, curoffset, io.SeekStart)

	var ntotal int
	for {
		max := len(buf)
		if max-ntotal > maxRW {
			max = ntotal + maxRW
		}
		b := buf[ntotal:max]
		var o syscall.Overlapped
		setOffset(&o, off+int64(ntotal))
		var qty uint32
		err := syscall.WriteFile(fd.Sysfd, b, &qty, &o)
		n := int(qty)
		if n > 0 {
			ntotal += n
		}
		if ntotal == len(buf) || err != nil {
			return ntotal, err
		}
		if n == 0 {
			return ntotal, io.ErrUnexpectedEOF
		}
	}
}

// Seek wraps syscall.Seek.
func (fd *FD) Seek(offset int64, whence int) (int64, error) {
	if fd.kind == kindPipe {
		return 0, syscall.ESPIPE
	}
	if err := fd.incref(); err != nil {
		return 0, err
	}
	defer fd.decref()

	fd.l.Lock()
	defer fd.l.Unlock()

	return syscall.Seek(fd.Sysfd, offset, whence)
}

// Fchmod updates syscall.ByHandleFileInformation.Fileattributes when needed.
func (fd *FD) Fchmod(mode uint32) error {
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()

	var d syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(fd.Sysfd, &d); err != nil {
		return err
	}
	attrs := d.FileAttributes
	if mode&syscall.S_IWRITE != 0 {
		attrs &^= syscall.FILE_ATTRIBUTE_READONLY
	} else {
		attrs |= syscall.FILE_ATTRIBUTE_READONLY
	}
	if attrs == d.FileAttributes {
		return nil
	}

	var du windows.FILE_BASIC_INFO
	du.FileAttributes = attrs
	return windows.SetFileInformationByHandle(fd.Sysfd, windows.FileBasicInfo, unsafe.Pointer(&du), uint32(unsafe.Sizeof(du)))
}

// Fchdir wraps syscall.Fchdir.
func (fd *FD) Fchdir() error {
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()
	return syscall.Fchdir(fd.Sysfd)
}

// GetFileType wraps syscall.GetFileType.
func (fd *FD) GetFileType() (uint32, error) {
	if err := fd.incref(); err != nil {
		return 0, err
	}
	defer fd.decref()
	return syscall.GetFileType(fd.Sysfd)
}

// GetFileInformationByHandle wraps GetFileInformationByHandle.
func (fd *FD) GetFileInformationByHandle(data *syscall.ByHandleFileInformation) error {
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()
	return syscall.GetFileInformationByHandle(fd.Sysfd, data)
}

// RawRead invokes the user-defined function f for a read operation.
func (fd *FD) RawRead(f func(uintptr) bool) error {
	if err := fd.readLock(); err != nil {
		return err
	}
	defer fd.readUnlock()
	for {
		if f(uintptr(fd.Sysfd)) {
			return nil
		}
		// Go waits for the socket to be readable here; a file handle has no such wait.
		return syscall.EWINDOWS
	}
}

// RawWrite invokes the user-defined function f for a write operation.
func (fd *FD) RawWrite(f func(uintptr) bool) error {
	if err := fd.writeLock(); err != nil {
		return err
	}
	defer fd.writeUnlock()

	if f(uintptr(fd.Sysfd)) {
		return nil
	}

	// TODO(tmm1): find a way to detect socket writability
	return syscall.EWINDOWS
}

// Fsync wraps syscall.Fsync.
func (fd *FD) Fsync() error {
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()
	return syscall.Fsync(fd.Sysfd)
}

// Fchown wraps syscall.Fchown (EWINDOWS on Windows).
func (fd *FD) Fchown(uid, gid int) error {
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()
	return syscall.Fchown(fd.Sysfd, uid, gid)
}

// Ftruncate wraps syscall.Ftruncate.
func (fd *FD) Ftruncate(size int64) error {
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()
	return syscall.Ftruncate(fd.Sysfd, size)
}

// RawControl invokes the user-defined function f for a non-IO
// operation.
func (fd *FD) RawControl(f func(uintptr)) error {
	if err := fd.incref(); err != nil {
		return err
	}
	defer fd.decref()
	f(uintptr(fd.Sysfd))
	return nil
}

// eofError returns io.EOF when fd is available for reading end of
// file.
func (fd *FD) eofError(n int, err error) error {
	if n == 0 && err == nil && fd.ZeroReadIsEOF {
		return io.EOF
	}
	return err
}
