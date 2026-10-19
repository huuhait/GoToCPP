// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Windows system calls.

package syscall

import (
	errorspkg "errors"
	"internal/bytealg"
	"internal/itoa"
	"internal/oserror"
	"sync"
	"unsafe"
)

type Handle uintptr

const InvalidHandle = ^Handle(0)

// StringToUTF16 returns the UTF-16 encoding of the UTF-8 string s,
// with a terminating NUL added. If s contains a NUL byte this
// function panics instead of returning an error.
//
// Deprecated: Use [UTF16FromString] instead.
func StringToUTF16(s string) []uint16 {
	a, err := UTF16FromString(s)
	if err != nil {
		panic("syscall: string with NUL passed to StringToUTF16")
	}
	return a
}

// UTF16FromString returns the UTF-16 encoding of the UTF-8 string
// s, with a terminating NUL added. If s contains a NUL byte at any
// location, it returns (nil, [EINVAL]). Unpaired surrogates
// are encoded using WTF-8.
func UTF16FromString(s string) ([]uint16, error) {
	if bytealg.IndexByteString(s, 0) != -1 {
		return nil, EINVAL
	}
	// Valid UTF-8 characters between 1 and 3 bytes require one uint16.
	// Valid UTF-8 characters of 4 bytes require two uint16.
	// Bytes with invalid UTF-8 encoding require maximum one uint16 per byte.
	// So the number of UTF-8 code units (len(s)) is always greater or
	// equal than the number of UTF-16 code units.
	// Also account for the terminating NUL character.
	buf := make([]uint16, 0, len(s)+1)
	buf = encodeWTF16(s, buf)
	return append(buf, 0), nil
}

// UTF16ToString returns the UTF-8 encoding of the UTF-16 sequence s,
// with a terminating NUL removed. Unpaired surrogates are decoded
// using WTF-8 instead of UTF-8 encoding.
func UTF16ToString(s []uint16) string {
	maxLen := 0
	for i, v := range s {
		if v == 0 {
			s = s[0:i]
			break
		}
		switch {
		case v <= rune1Max:
			maxLen += 1
		case v <= rune2Max:
			maxLen += 2
		default:
			// r is a non-surrogate that decodes to 3 bytes,
			// or is an unpaired surrogate (also 3 bytes in WTF-8),
			// or is one half of a valid surrogate pair.
			// If it is half of a pair, we will add 3 for the second surrogate
			// (total of 6) and overestimate by 2 bytes for the pair,
			// since the resulting rune only requires 4 bytes.
			maxLen += 3
		}
	}
	buf := decodeWTF16(s, make([]byte, 0, maxLen))
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}

// utf16PtrToString is like UTF16ToString, but takes *uint16
// as a parameter instead of []uint16.
func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	end := unsafe.Pointer(p)
	n := 0
	for *(*uint16)(end) != 0 {
		end = unsafe.Pointer(uintptr(end) + unsafe.Sizeof(*p))
		n++
	}
	return UTF16ToString(unsafe.Slice(p, n))
}

// StringToUTF16Ptr returns pointer to the UTF-16 encoding of
// the UTF-8 string s, with a terminating NUL added. If s
// contains a NUL byte this function panics instead of
// returning an error.
//
// Deprecated: Use [UTF16PtrFromString] instead.
func StringToUTF16Ptr(s string) *uint16 { return &StringToUTF16(s)[0] }

// UTF16PtrFromString returns pointer to the UTF-16 encoding of
// the UTF-8 string s, with a terminating NUL added. If s
// contains a NUL byte at any location, it returns (nil, EINVAL).
// Unpaired surrogates are encoded using WTF-8.
func UTF16PtrFromString(s string) (*uint16, error) {
	a, err := UTF16FromString(s)
	if err != nil {
		return nil, err
	}
	return &a[0], nil
}

// Errno is the Windows error number.
//
// Errno values can be tested against error values using [errors.Is].
// For example:
//
//	_, _, err := syscall.Syscall(...)
//	if errors.Is(err, fs.ErrNotExist) ...
type Errno uintptr

func langid(pri, sub uint16) uint32 { return uint32(sub)<<10 | uint32(pri) }

// FormatMessage is deprecated (msgsrc should be uintptr, not uint32, but can
// not be changed due to the Go 1 compatibility guarantee).
//
// Deprecated: Use FormatMessage from golang.org/x/sys/windows instead.
func FormatMessage(flags uint32, msgsrc uint32, msgid uint32, langid uint32, buf []uint16, args *byte) (n uint32, err error) {
	return formatMessage(flags, uintptr(msgsrc), msgid, langid, buf, args)
}

var errnoErrorCache sync.Map

func (e Errno) Error() string {
	// deal with special go errors
	idx := int(e - APPLICATION_ERROR)
	if 0 <= idx && idx < len(errors) {
		return errors[idx]
	}

	cache := false
	switch e {
	case ERROR_FILE_NOT_FOUND, ERROR_PATH_NOT_FOUND:
		if cached, ok := errnoErrorCache.Load(e); ok {
			return cached.(string)
		}
		cache = true
	}

	result := e.error()
	if cache {
		errnoErrorCache.Store(e, result)
	}
	return result
}

func (e Errno) error() string {
	// ask windows for the remaining errors
	var flags uint32 = FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_ARGUMENT_ARRAY | FORMAT_MESSAGE_IGNORE_INSERTS
	b := make([]uint16, 300)
	n, err := formatMessage(flags, 0, uint32(e), langid(LANG_ENGLISH, SUBLANG_ENGLISH_US), b, nil)
	if err != nil {
		n, err = formatMessage(flags, 0, uint32(e), 0, b, nil)
		if err != nil {
			return "winapi error #" + itoa.Itoa(int(e))
		}
	}
	// trim terminating \r and \n
	for ; n > 0 && (b[n-1] == '\n' || b[n-1] == '\r'); n-- {
	}
	return UTF16ToString(b[:n])
}

const (
	_ERROR_NOT_ENOUGH_MEMORY    = Errno(8)
	_ERROR_NOT_SUPPORTED        = Errno(50)
	_ERROR_BAD_NETPATH          = Errno(53)
	_ERROR_CALL_NOT_IMPLEMENTED = Errno(120)
)

func (e Errno) Is(target error) bool {
	switch target {
	case oserror.ErrPermission:
		return e == ERROR_ACCESS_DENIED ||
			e == EACCES ||
			e == EPERM
	case oserror.ErrExist:
		return e == ERROR_ALREADY_EXISTS ||
			e == ERROR_DIR_NOT_EMPTY ||
			e == ERROR_FILE_EXISTS ||
			e == EEXIST ||
			e == ENOTEMPTY
	case oserror.ErrNotExist:
		return e == ERROR_FILE_NOT_FOUND ||
			e == _ERROR_BAD_NETPATH ||
			e == ERROR_PATH_NOT_FOUND ||
			e == ENOENT
	case errorspkg.ErrUnsupported:
		return e == _ERROR_NOT_SUPPORTED ||
			e == _ERROR_CALL_NOT_IMPLEMENTED ||
			e == ENOSYS ||
			e == ENOTSUP ||
			e == EOPNOTSUPP ||
			e == EWINDOWS
	}
	return false
}

func (e Errno) Temporary() bool {
	return e == EINTR || e == EMFILE || e.Timeout()
}

func (e Errno) Timeout() bool {
	return e == EAGAIN || e == EWOULDBLOCK || e == ETIMEDOUT
}

// windows api calls

//sys	GetLastError() (lasterr error)
//sys	LoadLibrary(libname string) (handle Handle, err error) = LoadLibraryW
//sys	FreeLibrary(handle Handle) (err error)
//sys	GetProcAddress(module Handle, procname string) (proc uintptr, err error)
//sys	GetVersion() (ver uint32, err error)
//sys	formatMessage(flags uint32, msgsrc uintptr, msgid uint32, langid uint32, buf []uint16, args *byte) (n uint32, err error) = FormatMessageW
//sys	ExitProcess(exitcode uint32)
//sys	createFile(name *uint16, access uint32, mode uint32, sa *SecurityAttributes, createmode uint32, attrs uint32, templatefile int32) (handle Handle, err error) [failretval == InvalidHandle || e1 == ERROR_ALREADY_EXISTS ] = CreateFileW
//sys	readFile(handle Handle, buf []byte, done *uint32, overlapped *Overlapped) (err error) = ReadFile
//sys	writeFile(handle Handle, buf []byte, done *uint32, overlapped *Overlapped) (err error) = WriteFile
//sys	SetFilePointer(handle Handle, lowoffset int32, highoffsetptr *int32, whence uint32) (newlowoffset uint32, err error) [failretval==0xffffffff]
//sys	CloseHandle(handle Handle) (err error)
//sys	GetStdHandle(stdhandle int) (handle Handle, err error) [failretval==InvalidHandle]
//sys	findFirstFile1(name *uint16, data *win32finddata1) (handle Handle, err error) [failretval==InvalidHandle] = FindFirstFileW
//sys	findNextFile1(handle Handle, data *win32finddata1) (err error) = FindNextFileW
//sys	FindClose(handle Handle) (err error)
//sys	GetFileInformationByHandle(handle Handle, data *ByHandleFileInformation) (err error)
//sys	GetCurrentDirectory(buflen uint32, buf *uint16) (n uint32, err error) = GetCurrentDirectoryW
//sys	SetCurrentDirectory(path *uint16) (err error) = SetCurrentDirectoryW
//sys	CreateDirectory(path *uint16, sa *SecurityAttributes) (err error) = CreateDirectoryW
//sys	RemoveDirectory(path *uint16) (err error) = RemoveDirectoryW
//sys	DeleteFile(path *uint16) (err error) = DeleteFileW
//sys	MoveFile(from *uint16, to *uint16) (err error) = MoveFileW
//sys	GetComputerName(buf *uint16, n *uint32) (err error) = GetComputerNameW
//sys	SetEndOfFile(handle Handle) (err error)
//sys	GetSystemTimeAsFileTime(time *Filetime)
//sys	CancelIoEx(s Handle, o *Overlapped) (err error)
//sys	GetTimeZoneInformation(tzi *Timezoneinformation) (rc uint32, err error) [failretval==0xffffffff]
//sys	OpenProcess(da uint32, inheritHandle bool, pid uint32) (handle Handle, err error)
//sys	TerminateProcess(handle Handle, exitcode uint32) (err error)
//sys	GetExitCodeProcess(handle Handle, exitcode *uint32) (err error)
//sys	GetCurrentProcess() (pseudoHandle Handle, err error)
//sys	GetProcessTimes(handle Handle, creationTime *Filetime, exitTime *Filetime, kernelTime *Filetime, userTime *Filetime) (err error)
//sys	DuplicateHandle(hSourceProcessHandle Handle, hSourceHandle Handle, hTargetProcessHandle Handle, lpTargetHandle *Handle, dwDesiredAccess uint32, bInheritHandle bool, dwOptions uint32) (err error)
//sys	WaitForSingleObject(handle Handle, waitMilliseconds uint32) (event uint32, err error) [failretval==0xffffffff]
//sys	GetTempPath(buflen uint32, buf *uint16) (n uint32, err error) = GetTempPathW
//sys	CreatePipe(readhandle *Handle, writehandle *Handle, sa *SecurityAttributes, size uint32) (err error)
//sys	GetFileType(filehandle Handle) (n uint32, err error)
//sys	GetEnvironmentStrings() (envs *uint16, err error) [failretval==nil] = kernel32.GetEnvironmentStringsW
//sys	FreeEnvironmentStrings(envs *uint16) (err error) = kernel32.FreeEnvironmentStringsW
//sys	GetEnvironmentVariable(name *uint16, buffer *uint16, size uint32) (n uint32, err error) = kernel32.GetEnvironmentVariableW
//sys	SetEnvironmentVariable(name *uint16, value *uint16) (err error) = kernel32.SetEnvironmentVariableW
//sys	SetFileTime(handle Handle, ctime *Filetime, atime *Filetime, wtime *Filetime) (err error)
//sys	GetFileAttributes(name *uint16) (attrs uint32, err error) [failretval==INVALID_FILE_ATTRIBUTES] = kernel32.GetFileAttributesW
//sys	SetFileAttributes(name *uint16, attrs uint32) (err error) = kernel32.SetFileAttributesW
//sys	GetFileAttributesEx(name *uint16, level uint32, info *byte) (err error) = kernel32.GetFileAttributesExW
//sys	GetCommandLine() (cmd *uint16) = kernel32.GetCommandLineW
//sys	LocalFree(hmem Handle) (handle Handle, err error) [failretval!=0]
//sys	localAlloc(flags uint32, length uint32) (ptr uintptr, err error) = kernel32.LocalAlloc
//sys	SetHandleInformation(handle Handle, mask uint32, flags uint32) (err error)
//sys	FlushFileBuffers(handle Handle) (err error)
//sys	GetFullPathName(path *uint16, buflen uint32, buf *uint16, fname **uint16) (n uint32, err error) = kernel32.GetFullPathNameW
//sys	GetLongPathName(path *uint16, buf *uint16, buflen uint32) (n uint32, err error) = kernel32.GetLongPathNameW
//sys	GetShortPathName(longpath *uint16, shortpath *uint16, buflen uint32) (n uint32, err error) = kernel32.GetShortPathNameW
//sys	getCurrentProcessId() (pid uint32) = kernel32.GetCurrentProcessId
//sys	GetConsoleMode(console Handle, mode *uint32) (err error) = kernel32.GetConsoleMode
//sys	WriteConsole(console Handle, buf *uint16, towrite uint32, written *uint32, reserved *byte) (err error) = kernel32.WriteConsoleW
//sys	ReadConsole(console Handle, buf *uint16, toread uint32, read *uint32, inputControl *byte) (err error) = kernel32.ReadConsoleW
//sys	CreateToolhelp32Snapshot(flags uint32, processId uint32) (handle Handle, err error) [failretval==InvalidHandle] = kernel32.CreateToolhelp32Snapshot
//sys	Process32First(snapshot Handle, procEntry *ProcessEntry32) (err error) = kernel32.Process32FirstW
//sys	Process32Next(snapshot Handle, procEntry *ProcessEntry32) (err error) = kernel32.Process32NextW
//sys	DeviceIoControl(handle Handle, ioControlCode uint32, inBuffer *byte, inBufferSize uint32, outBuffer *byte, outBufferSize uint32, bytesReturned *uint32, overlapped *Overlapped) (err error)
//sys	setFileInformationByHandle(handle Handle, fileInformationClass uint32, buf unsafe.Pointer, bufsize uint32) (err error) = kernel32.SetFileInformationByHandle
// This function returns 1 byte BOOLEAN rather than the 4 byte BOOL.
//sys	CreateSymbolicLink(symlinkfilename *uint16, targetfilename *uint16, flags uint32) (err error) [failretval&0xff==0] = CreateSymbolicLinkW
//sys	CreateHardLink(filename *uint16, existingfilename *uint16, reserved uintptr) (err error) [failretval&0xff==0] = CreateHardLinkW
//sys	getFinalPathNameByHandle(file Handle, filePath *uint16, filePathSize uint32, flags uint32) (n uint32, err error) [n == 0 || n >= filePathSize] = kernel32.GetFinalPathNameByHandleW

// syscall interface implementation for other packages

func makeInheritSa() *SecurityAttributes {
	var sa SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	return &sa
}

func Open(name string, flag int, perm uint32) (fd Handle, err error) {
	if len(name) == 0 {
		return InvalidHandle, ERROR_FILE_NOT_FOUND
	}
	namep, err := UTF16PtrFromString(name)
	if err != nil {
		return InvalidHandle, err
	}
	accessFlags := flag & (O_RDONLY | O_WRONLY | O_RDWR)
	var access uint32
	switch accessFlags {
	case O_RDONLY:
		access = GENERIC_READ
	case O_WRONLY:
		access = GENERIC_WRITE
	case O_RDWR:
		access = GENERIC_READ | GENERIC_WRITE
	}
	if flag&O_CREAT != 0 {
		access |= GENERIC_WRITE
	}
	if flag&O_APPEND != 0 {
		// Remove GENERIC_WRITE unless O_TRUNC is set, in which case we need it to truncate the file.
		// We can't just remove FILE_WRITE_DATA because GENERIC_WRITE without FILE_WRITE_DATA
		// starts appending at the beginning of the file rather than at the end.
		if flag&O_TRUNC == 0 {
			access &^= GENERIC_WRITE
		}
		// Set all access rights granted by GENERIC_WRITE except for FILE_WRITE_DATA.
		access |= FILE_APPEND_DATA | FILE_WRITE_ATTRIBUTES | _FILE_WRITE_EA | STANDARD_RIGHTS_WRITE | SYNCHRONIZE
	}
	sharemode := uint32(FILE_SHARE_READ | FILE_SHARE_WRITE)
	var sa *SecurityAttributes
	if flag&O_CLOEXEC == 0 {
		sa = makeInheritSa()
	}
	var attrs uint32 = FILE_ATTRIBUTE_NORMAL
	if perm&S_IWRITE == 0 {
		attrs = FILE_ATTRIBUTE_READONLY
	}
	switch accessFlags {
	case O_WRONLY, O_RDWR:
		// Unix doesn't allow opening a directory with O_WRONLY
		// or O_RDWR, so we don't set the flag in that case,
		// which will make CreateFile fail with ERROR_ACCESS_DENIED.
		// We will map that to EISDIR if the file is a directory.
	default:
		// We might be opening a directory for reading,
		// and CreateFile requires FILE_FLAG_BACKUP_SEMANTICS
		// to work with directories.
		attrs |= FILE_FLAG_BACKUP_SEMANTICS
	}
	if flag&O_SYNC != 0 {
		const _FILE_FLAG_WRITE_THROUGH = 0x80000000
		attrs |= _FILE_FLAG_WRITE_THROUGH
	}
	// We don't use CREATE_ALWAYS, because when opening a file with
	// FILE_ATTRIBUTE_READONLY these will replace an existing file
	// with a new, read-only one. See https://go.dev/issue/38225.
	//
	// Instead, we ftruncate the file after opening when O_TRUNC is set.
	var createmode uint32
	switch {
	case flag&(O_CREAT|O_EXCL) == (O_CREAT | O_EXCL):
		createmode = CREATE_NEW
		attrs |= FILE_FLAG_OPEN_REPARSE_POINT // don't follow symlinks
	case flag&O_CREAT == O_CREAT:
		createmode = OPEN_ALWAYS
	default:
		createmode = OPEN_EXISTING
	}
	h, err := createFile(namep, access, sharemode, sa, createmode, attrs, 0)
	if h == InvalidHandle {
		if err == ERROR_ACCESS_DENIED && (attrs&FILE_FLAG_BACKUP_SEMANTICS == 0) {
			// We should return EISDIR when we are trying to open a directory with write access.
			fa, e1 := GetFileAttributes(namep)
			if e1 == nil && fa&FILE_ATTRIBUTE_DIRECTORY != 0 {
				err = EISDIR
			}
		}
		return h, err
	}
	// Ignore O_TRUNC if the file has just been created.
	if flag&O_TRUNC == O_TRUNC &&
		(createmode == OPEN_EXISTING || (createmode == OPEN_ALWAYS && err == ERROR_ALREADY_EXISTS)) {
		err = Ftruncate(h, 0)
		if err != nil {
			CloseHandle(h)
			return InvalidHandle, err
		}
	}
	return h, nil
}

func Read(fd Handle, p []byte) (n int, err error) {
	var done uint32
	e := ReadFile(fd, p, &done, nil)
	if e != nil {
		if e == ERROR_BROKEN_PIPE {
			// NOTE(brainman): work around ERROR_BROKEN_PIPE is returned on reading EOF from stdin
			return 0, nil
		}
		return 0, e
	}
	return int(done), nil
}

func Write(fd Handle, p []byte) (n int, err error) {
	var done uint32
	e := WriteFile(fd, p, &done, nil)
	if e != nil {
		return 0, e
	}
	return int(done), nil
}

func ReadFile(fd Handle, p []byte, done *uint32, overlapped *Overlapped) error {
	return readFile(fd, p, done, overlapped)
}

func WriteFile(fd Handle, p []byte, done *uint32, overlapped *Overlapped) error {
	return writeFile(fd, p, done, overlapped)
}

var procSetFilePointerEx = modkernel32.NewProc("SetFilePointerEx")

const ptrSize = unsafe.Sizeof(uintptr(0))

// setFilePointerEx calls SetFilePointerEx.
// See https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfilepointerex
func setFilePointerEx(handle Handle, distToMove int64, newFilePointer *int64, whence uint32) error {
	// gocpp is 64-bit only (the Windows layer is x86-64: ADR-053): Go's 32-bit branches are gone.
	_, _, e1 := Syscall6(procSetFilePointerEx.Addr(), 4, uintptr(handle), uintptr(distToMove), uintptr(unsafe.Pointer(newFilePointer)), uintptr(whence), 0, 0)
	if e1 != 0 {
		return errnoErr(e1)
	}
	return nil
}

func Seek(fd Handle, offset int64, whence int) (newoffset int64, err error) {
	var w uint32
	switch whence {
	case 0:
		w = FILE_BEGIN
	case 1:
		w = FILE_CURRENT
	case 2:
		w = FILE_END
	}
	err = setFilePointerEx(fd, offset, &newoffset, w)
	return
}

func Close(fd Handle) (err error) {
	return CloseHandle(fd)
}

var (
	Stdin  = getStdHandle(STD_INPUT_HANDLE)
	Stdout = getStdHandle(STD_OUTPUT_HANDLE)
	Stderr = getStdHandle(STD_ERROR_HANDLE)
)

func getStdHandle(h int) (fd Handle) {
	r, _ := GetStdHandle(h)
	return r
}

const ImplementsGetwd = true

func Getwd() (wd string, err error) {
	b := make([]uint16, 300)
	// The path of the current directory may not fit in the initial 300-word
	// buffer when long path support is enabled. The current directory may also
	// change between subsequent calls of GetCurrentDirectory. As a result, we
	// need to retry the call in a loop until the current directory fits, each
	// time with a bigger buffer.
	for {
		n, e := GetCurrentDirectory(uint32(len(b)), &b[0])
		if e != nil {
			return "", e
		}
		if int(n) <= len(b) {
			return UTF16ToString(b[:n]), nil
		}
		b = make([]uint16, n)
	}
}

func Chdir(path string) (err error) {
	pathp, err := UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return SetCurrentDirectory(pathp)
}

func Mkdir(path string, mode uint32) (err error) {
	pathp, err := UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return CreateDirectory(pathp, nil)
}

func Rmdir(path string) (err error) {
	pathp, err := UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return RemoveDirectory(pathp)
}

func Unlink(path string) (err error) {
	pathp, err := UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return DeleteFile(pathp)
}

func Rename(oldpath, newpath string) (err error) {
	from, err := UTF16PtrFromString(oldpath)
	if err != nil {
		return err
	}
	to, err := UTF16PtrFromString(newpath)
	if err != nil {
		return err
	}
	return MoveFile(from, to)
}

func ComputerName() (name string, err error) {
	var n uint32 = MAX_COMPUTERNAME_LENGTH + 1
	b := make([]uint16, n)
	e := GetComputerName(&b[0], &n)
	if e != nil {
		return "", e
	}
	return UTF16ToString(b[:n]), nil
}

func Ftruncate(fd Handle, length int64) (err error) {
	type _FILE_END_OF_FILE_INFO struct {
		EndOfFile int64
	}
	const FileEndOfFileInfo = 6
	var info _FILE_END_OF_FILE_INFO
	info.EndOfFile = length
	return setFileInformationByHandle(fd, FileEndOfFileInfo, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)))
}

func Gettimeofday(tv *Timeval) (err error) {
	var ft Filetime
	GetSystemTimeAsFileTime(&ft)
	*tv = NsecToTimeval(ft.Nanoseconds())
	return nil
}

func Pipe(p []Handle) (err error) {
	if len(p) != 2 {
		return EINVAL
	}
	var r, w Handle
	e := CreatePipe(&r, &w, makeInheritSa(), 0)
	if e != nil {
		return e
	}
	p[0] = r
	p[1] = w
	return nil
}

func Utimes(path string, tv []Timeval) (err error) {
	if len(tv) != 2 {
		return EINVAL
	}
	pathp, e := UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	h, e := CreateFile(pathp,
		FILE_WRITE_ATTRIBUTES, FILE_SHARE_WRITE, nil,
		OPEN_EXISTING, FILE_FLAG_BACKUP_SEMANTICS, 0)
	if e != nil {
		return e
	}
	defer Close(h)
	a := Filetime{}
	w := Filetime{}
	if tv[0].Nanoseconds() != 0 {
		a = NsecToFiletime(tv[0].Nanoseconds())
	}
	if tv[0].Nanoseconds() != 0 {
		w = NsecToFiletime(tv[1].Nanoseconds())
	}
	return SetFileTime(h, nil, &a, &w)
}

// This matches the value in os/file_windows.go.
const _UTIME_OMIT = -1

func UtimesNano(path string, ts []Timespec) (err error) {
	if len(ts) != 2 {
		return EINVAL
	}
	pathp, e := UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	h, e := CreateFile(pathp,
		FILE_WRITE_ATTRIBUTES, FILE_SHARE_WRITE, nil,
		OPEN_EXISTING, FILE_FLAG_BACKUP_SEMANTICS, 0)
	if e != nil {
		return e
	}
	defer Close(h)
	a := Filetime{}
	w := Filetime{}
	if ts[0].Nsec != _UTIME_OMIT {
		a = NsecToFiletime(TimespecToNsec(ts[0]))
	}
	if ts[1].Nsec != _UTIME_OMIT {
		w = NsecToFiletime(TimespecToNsec(ts[1]))
	}
	return SetFileTime(h, nil, &a, &w)
}

func Fsync(fd Handle) (err error) {
	return FlushFileBuffers(fd)
}

func Chmod(path string, mode uint32) (err error) {
	p, e := UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	attrs, e := GetFileAttributes(p)
	if e != nil {
		return e
	}
	if mode&S_IWRITE != 0 {
		attrs &^= FILE_ATTRIBUTE_READONLY
	} else {
		attrs |= FILE_ATTRIBUTE_READONLY
	}
	return SetFileAttributes(p, attrs)
}

// Invented structures to support what package os expects.
type Rusage struct {
	CreationTime Filetime
	ExitTime     Filetime
	KernelTime   Filetime
	UserTime     Filetime
}

type WaitStatus struct {
	ExitCode uint32
}

func (w WaitStatus) Exited() bool { return true }

func (w WaitStatus) ExitStatus() int { return int(w.ExitCode) }

func (w WaitStatus) Signal() Signal { return -1 }

func (w WaitStatus) CoreDump() bool { return false }

func (w WaitStatus) Stopped() bool { return false }

func (w WaitStatus) Continued() bool { return false }

func (w WaitStatus) StopSignal() Signal { return -1 }

func (w WaitStatus) Signaled() bool { return false }

func (w WaitStatus) TrapCause() int { return -1 }

// Timespec is an invented structure on Windows, but here for
// consistency with the syscall package for other operating systems.
type Timespec struct {
	Sec  int64
	Nsec int64
}

func TimespecToNsec(ts Timespec) int64 { return int64(ts.Sec)*1e9 + int64(ts.Nsec) }

func NsecToTimespec(nsec int64) (ts Timespec) {
	ts.Sec = nsec / 1e9
	ts.Nsec = nsec % 1e9
	return
}

// TODO(brainman): fix all needed for net

// The Linger struct is wrong but we only noticed after Go 1.
// sysLinger is the real system call structure.

// BUG(brainman): The definition of Linger is not appropriate for direct use
// with Setsockopt and Getsockopt.
// Use SetsockoptLinger instead.

func Getpid() (pid int) { return int(getCurrentProcessId()) }

func FindFirstFile(name *uint16, data *Win32finddata) (handle Handle, err error) {
	// NOTE(rsc): The Win32finddata struct is wrong for the system call:
	// the two paths are each one uint16 short. Use the correct struct,
	// a win32finddata1, and then copy the results out.
	// There is no loss of expressivity here, because the final
	// uint16, if it is used, is supposed to be a NUL, and Go doesn't need that.
	// For Go 1.1, we might avoid the allocation of win32finddata1 here
	// by adding a final Bug [2]uint16 field to the struct and then
	// adjusting the fields in the result directly.
	var data1 win32finddata1
	handle, err = findFirstFile1(name, &data1)
	if err == nil {
		copyFindData(data, &data1)
	}
	return
}

func FindNextFile(handle Handle, data *Win32finddata) (err error) {
	var data1 win32finddata1
	err = findNextFile1(handle, &data1)
	if err == nil {
		copyFindData(data, &data1)
	}
	return
}

func getProcessEntry(pid int) (*ProcessEntry32, error) {
	snapshot, err := CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer CloseHandle(snapshot)
	var procEntry ProcessEntry32
	procEntry.Size = uint32(unsafe.Sizeof(procEntry))
	if err = Process32First(snapshot, &procEntry); err != nil {
		return nil, err
	}
	for {
		if procEntry.ProcessID == uint32(pid) {
			return &procEntry, nil
		}
		err = Process32Next(snapshot, &procEntry)
		if err != nil {
			return nil, err
		}
	}
}

func Getppid() (ppid int) {
	pe, err := getProcessEntry(Getpid())
	if err != nil {
		return -1
	}
	return int(pe.ParentProcessID)
}

func fdpath(fd Handle, buf []uint16) ([]uint16, error) {
	const (
		FILE_NAME_NORMALIZED = 0
		VOLUME_NAME_DOS      = 0
	)
	for {
		n, err := getFinalPathNameByHandle(fd, &buf[0], uint32(len(buf)), FILE_NAME_NORMALIZED|VOLUME_NAME_DOS)
		if err == nil {
			buf = buf[:n]
			break
		}
		if err != _ERROR_NOT_ENOUGH_MEMORY {
			return nil, err
		}
		buf = append(buf, make([]uint16, n-uint32(len(buf)))...)
	}
	return buf, nil
}

func Fchdir(fd Handle) (err error) {
	var buf [MAX_PATH + 1]uint16
	path, err := fdpath(fd, buf[:])
	if err != nil {
		return err
	}
	// When using VOLUME_NAME_DOS, the path is always prefixed by "\\?\".
	// That prefix tells the Windows APIs to disable all string parsing and to send
	// the string that follows it straight to the file system.
	// Although SetCurrentDirectory and GetCurrentDirectory do support the "\\?\" prefix,
	// some other Windows APIs don't. If the prefix is not removed here, it will leak
	// to Getwd, and we don't want such a general-purpose function to always return a
	// path with the "\\?\" prefix after Fchdir is called.
	// The downside is that APIs that do support it will parse the path and try to normalize it,
	// when it's already normalized.
	if len(path) >= 4 && path[0] == '\\' && path[1] == '\\' && path[2] == '?' && path[3] == '\\' {
		path = path[4:]
	}
	return SetCurrentDirectory(&path[0])
}

// TODO(brainman): fix all needed for os
func Link(oldpath, newpath string) (err error) { return EWINDOWS }
func Symlink(path, link string) (err error)    { return EWINDOWS }

func Fchmod(fd Handle, mode uint32) (err error)        { return EWINDOWS }
func Chown(path string, uid int, gid int) (err error)  { return EWINDOWS }
func Lchown(path string, uid int, gid int) (err error) { return EWINDOWS }
func Fchown(fd Handle, uid int, gid int) (err error)   { return EWINDOWS }

func Getuid() (uid int)                  { return -1 }
func Geteuid() (euid int)                { return -1 }
func Getgid() (gid int)                  { return -1 }
func Getegid() (egid int)                { return -1 }
func Getgroups() (gids []int, err error) { return nil, EWINDOWS }

type Signal int

func (s Signal) Signal() {}

func (s Signal) String() string {
	if 0 <= s && int(s) < len(signals) {
		str := signals[s]
		if str != "" {
			return str
		}
	}
	return "signal " + itoa.Itoa(int(s))
}

func LoadCreateSymbolicLink() error {
	return procCreateSymbolicLinkW.Find()
}

// Readlink returns the destination of the named symbolic link.
func Readlink(path string, buf []byte) (n int, err error) {
	pathp, err := UTF16PtrFromString(path)
	if err != nil {
		return -1, err
	}
	fd, err := CreateFile(pathp, GENERIC_READ, 0, nil, OPEN_EXISTING,
		FILE_FLAG_OPEN_REPARSE_POINT|FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return -1, err
	}
	defer CloseHandle(fd)

	rdbbuf := make([]byte, MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	var bytesReturned uint32
	err = DeviceIoControl(fd, FSCTL_GET_REPARSE_POINT, nil, 0, &rdbbuf[0], uint32(len(rdbbuf)), &bytesReturned, nil)
	if err != nil {
		return -1, err
	}

	rdb := (*reparseDataBuffer)(unsafe.Pointer(&rdbbuf[0]))
	var s string
	switch rdb.ReparseTag {
	case IO_REPARSE_TAG_SYMLINK:
		data := (*symbolicLinkReparseBuffer)(unsafe.Pointer(&rdb.reparseBuffer))
		p := (*[0xffff]uint16)(unsafe.Pointer(&data.PathBuffer[0]))
		s = UTF16ToString(p[data.SubstituteNameOffset/2 : (data.SubstituteNameOffset+data.SubstituteNameLength)/2])
		if data.Flags&_SYMLINK_FLAG_RELATIVE == 0 {
			if len(s) >= 4 && s[:4] == `\??\` {
				s = s[4:]
				switch {
				case len(s) >= 2 && s[1] == ':': // \??\C:\foo\bar
					// do nothing
				case len(s) >= 4 && s[:4] == `UNC\`: // \??\UNC\foo\bar
					s = `\\` + s[4:]
				default:
					// unexpected; do nothing
				}
			} else {
				// unexpected; do nothing
			}
		}
	case _IO_REPARSE_TAG_MOUNT_POINT:
		data := (*mountPointReparseBuffer)(unsafe.Pointer(&rdb.reparseBuffer))
		p := (*[0xffff]uint16)(unsafe.Pointer(&data.PathBuffer[0]))
		s = UTF16ToString(p[data.SubstituteNameOffset/2 : (data.SubstituteNameOffset+data.SubstituteNameLength)/2])
		if len(s) >= 4 && s[:4] == `\??\` { // \??\C:\foo\bar
			s = s[4:]
		} else {
			// unexpected; do nothing
		}
	default:
		// the path is not a symlink or junction but another type of reparse
		// point
		return -1, ENOENT
	}
	n = copy(buf, []byte(s))

	return n, nil
}

func CreateFile(name *uint16, access uint32, mode uint32, sa *SecurityAttributes, createmode uint32, attrs uint32, templatefile int32) (handle Handle, err error) {
	handle, err = createFile(name, access, mode, sa, createmode, attrs, templatefile)
	if handle != InvalidHandle {
		// CreateFileW can return ERROR_ALREADY_EXISTS with a valid handle.
		// We only want to return an error if the handle is invalid.
		err = nil
	}
	return handle, err
}

// Exit ends the program at once with the code, like os.Exit (it does not run deferred functions;
// gp::os::exit flushes C's stdio first).
//
//gocpp:native gp::syscall::sys_exit
func sysExit(code int)

func Exit(code int) { sysExit(code) }

// Getpagesize returns the system's page size (4096 on x86-64 Windows; Go's runtime asks the system).
func Getpagesize() int { return 4096 }
