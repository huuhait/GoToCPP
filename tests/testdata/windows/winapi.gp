// phase: 14

// Console programs over the C API of kernel32, advapi32 and user32 (step 14.4, ADR-082): clocks, a struct
// out-parameter, SYSTEMTIME, files through CreateFileW/WriteFile/ReadFile, the registry, a thread whose
// start routine is a gocpp function (WINAPI), INVALID_HANDLE_VALUE, and the sizes of the LLP64 types.

//gocpp:link kernel32 advapi32 user32

package main

import (
	"fmt"
	"unsafe"

	"<windows.h>"
)

var counter int32

// threadMain is an LPTHREAD_START_ROUTINE: Windows calls it with the WINAPI convention, which is the
// only convention of x86-64 Windows. A plain gocpp function becomes the C function pointer.
func threadMain(arg windows.LPVOID) windows.DWORD {
	windows.InterlockedIncrement((*windows.LONG)(unsafe.Pointer(&counter)))
	return 42
}

func main() {
	// LLP64: long is 32 bits on Windows, as DWORD; handles and size_t are 64.
	fmt.Println("sizes:", unsafe.Sizeof(C.long(0)), unsafe.Sizeof(C.ulong(0)), unsafe.Sizeof(C.longlong(0)),
		unsafe.Sizeof(C.size_t(0)), unsafe.Sizeof(windows.DWORD(0)), unsafe.Sizeof(windows.HANDLE(nil)),
		unsafe.Sizeof(windows.WCHAR(0)))

	// Clocks.
	t0 := windows.GetTickCount64()
	var freq, c0, c1 windows.LARGE_INTEGER
	windows.QueryPerformanceFrequency(&freq)
	windows.QueryPerformanceCounter(&c0)
	windows.Sleep(15)
	windows.QueryPerformanceCounter(&c1)
	fmt.Println("clocks:", t0 > 0, int64(freq.QuadPart) > 0, int64(c1.QuadPart) > int64(c0.QuadPart),
		windows.GetTickCount64() >= t0)

	// A struct out-parameter, and the union inside it.
	var si windows.SYSTEM_INFO
	windows.GetSystemInfo(&si)
	fmt.Println("SYSTEM_INFO:", int(si.dwPageSize), int(si.wProcessorArchitecture) == int(windows.PROCESSOR_ARCHITECTURE_AMD64),
		si.dwNumberOfProcessors > 0)

	// SYSTEMTIME and FILETIME: a fixed time through both conversions.
	st := windows.SYSTEMTIME{wYear: 2026, wMonth: 10, wDay: 19, wHour: 12, wMinute: 30, wSecond: 15, wMilliseconds: 250}
	var ft windows.FILETIME
	windows.SystemTimeToFileTime(&st, &ft)
	var back windows.SYSTEMTIME
	windows.FileTimeToSystemTime(&ft, &back)
	fmt.Println("SYSTEMTIME:", back.wYear, back.wMonth, back.wDay, back.wHour, back.wMinute, back.wSecond, back.wMilliseconds,
		int(back.wDayOfWeek))
	var now windows.SYSTEMTIME
	windows.GetSystemTime(&now)
	fmt.Println("now is after 2026:", now.wYear >= 2026)

	// Handles are pointers: INVALID_HANDLE_VALUE is (HANDLE)-1.
	bad := windows.CreateFileW(C.wstr("no\\such\\folder\\file.txt"), windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL, nil)
	fmt.Println("CreateFileW of a missing file:", bad == windows.INVALID_HANDLE_VALUE, int(windows.GetLastError()) == int(windows.ERROR_PATH_NOT_FOUND))

	// A file through the C API: write, read back, delete.
	path := C.wstr("gocpp-winapi.tmp")
	h := windows.CreateFileW(path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.CREATE_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL, nil)
	if h == windows.INVALID_HANDLE_VALUE {
		fmt.Println("CreateFileW failed:", int(windows.GetLastError()))
		return
	}
	data := []byte("hello 世界 from the C API")
	var written windows.DWORD
	ok := windows.WriteFile(h, unsafe.Pointer(&data[0]), windows.DWORD(len(data)), &written, nil)
	fmt.Println("WriteFile:", ok != 0, int(written) == len(data))
	windows.SetFilePointer(h, 0, nil, windows.FILE_BEGIN)
	buf := make([]byte, 64)
	var read windows.DWORD
	ok = windows.ReadFile(h, unsafe.Pointer(&buf[0]), 64, &read, nil)
	fmt.Println("ReadFile:", ok != 0, int(read), string(buf[:read]))
	var size windows.LARGE_INTEGER
	windows.GetFileSizeEx(h, &size)
	fmt.Println("GetFileSizeEx:", int64(size.QuadPart) == int64(len(data)))
	windows.CloseHandle(h)
	fmt.Println("DeleteFileW:", windows.DeleteFileW(path) != 0, windows.GetFileAttributesW(path) == windows.INVALID_FILE_ATTRIBUTES)

	// The registry, read only: the product name of Windows.
	var key windows.HKEY
	rc := windows.RegOpenKeyExW(windows.HKEY_LOCAL_MACHINE, C.wstr(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`), 0,
		windows.KEY_READ, &key)
	if int(rc) != 0 {
		fmt.Println("RegOpenKeyExW failed:", int(rc))
		return
	}
	var val [128]windows.WCHAR
	size32 := windows.DWORD(len(val) * 2)
	var typ windows.DWORD
	rc = windows.RegQueryValueExW(key, C.wstr("ProductName"), nil, &typ, (*windows.BYTE)(unsafe.Pointer(&val[0])), &size32)
	name := C.GoStringW(&val[0])
	windows.RegCloseKey(key)
	fmt.Println("ProductName:", int(rc), int(typ) == int(windows.REG_SZ), len(name) > 7 && name[:7] == "Windows")

	// A thread whose start routine is a gocpp function.
	th := windows.CreateThread(nil, 0, threadMain, nil, 0, nil)
	windows.WaitForSingleObject(th, windows.INFINITE)
	var code windows.DWORD
	windows.GetExitCodeThread(th, &code)
	windows.CloseHandle(th)
	fmt.Println("thread:", int(code), counter)

	// user32 without a window: the metrics of the system, and the case mapping of a wide string.
	var word [8]windows.WCHAR
	windows.lstrcpyW(&word[0], C.wstr("h\u00e9llo"))
	windows.CharUpperBuffW(&word[0], 5)
	fmt.Println("user32:", int(windows.GetSystemMetrics(windows.SM_CMOUSEBUTTONS)) >= 0, C.GoStringW(&word[0]))
}
