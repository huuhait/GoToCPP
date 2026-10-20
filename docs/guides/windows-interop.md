# Calling the Win32 API from gocpp

> `<windows.h>` is a header package like any other: `windows.MessageBoxW(...)`, `windows.GetLastError()`.
> This page is the walkthrough (a message box, wide strings, a window, sockets, the types, the traps); the
> rules in full are in the [interop spec](../spec/interop.md#windows-headers) and in
> [ADR-082](../adr/082-windows-interop-defaults-and-wide-strings.md). Building and installing is on the
> [Windows page](windows.md).

**Switch.** The Windows interop (`C.wstr`, `C.GoStringW`) and the parts of `os` the examples use are behind
`GOCPP_GATE_PHASE` until their phases are finished: the commands below set `GOCPP_GATE_PHASE=15`
(without it `gocpp check` says `C.wstr and C.GoStringW are not supported yet (Phase 14)`). The switch is for
gocpp's own development and goes away when Phase 14 is finished.

**Where it builds.** A program that calls the Win32 API is for Windows only. Build it on Windows with `gocpp build`, or on
Linux with `gocpp build -os windows` ([Windows from Linux](windows-cross.md)); on Linux the header `<windows.h>` is the MinGW-w64 one
of the cross compiler. A file named `x_windows.gp`, or one with `//go:build windows`, is left out of a Linux build, as in Go.

```sh
GOCPP_GATE_PHASE=15 gocpp build -os windows -o msg msg.gp     # on Linux
```

```powershell
$env:GOCPP_GATE_PHASE = '15'                                  # on Windows (cmd.exe: set GOCPP_GATE_PHASE=15)
gocpp build -o msg msg.gp
```

## A message box

<!-- illustration: Windows only: <windows.h> is the MinGW header of the cross compiler -->
```go
package main

import "<windows.h>"

func main() {
	windows.MessageBoxW(nil, C.wstr("Hello from gocpp"), C.wstr("gocpp"), 0)
}
```

`MessageBoxW` takes a window handle (`nil`: none), the text, the title and flags. The text is UTF-16 on Windows, and a gocpp
string is UTF-8, so `C.wstr(s)` makes a UTF-16 copy for the call. The program builds with the native MinGW-w64 compiler of the PC and
with the cross compiler from Linux; the box itself needs a desktop (see [A window](#a-window)). Because gocpp
defines `UNICODE` for Windows packages ([below](#the-defaults-gocpp-sets)), the macro name `windows.MessageBox` means `MessageBoxW` too, as in C.

## Wide strings

The `W` functions of Windows take `wchar_t`, which is 16 bits there (UTF-16). Four helpers cover the conversions; they work on every
target (where `wchar_t` is 32 bits, as on Linux, the units are UTF-32), and use Go's rules for bad text:

| Helper | What it does |
|---|---|
| `C.wstr(s)` | a `const wchar_t*` for one statement, or for as long as a variable holds the result. A copy, like `C.cstr` but never zero-copy. For an argument of a call |
| `C.WString(s)` | a `malloc`ed copy, for a pointer that C **keeps** (a struct field such as `WNDCLASSW.lpszClassName`). Free it with `C.free` |
| `C.GoStringW(p)` | the NUL-terminated wide string at `p` as a Go string. `p` may be an `LPCWSTR`, `&buf[0]` of a `[260]windows.WCHAR` or a `*uint16` |
| `C.GoStringWN(p, n)` | `n` wide characters, NULs included |

An invalid UTF-8 byte becomes U+FFFD, a lone surrogate written as three bytes (WTF-8, as a Windows file name can hold) survives the
round trip, a character beyond U+FFFF becomes a surrogate pair, and a NUL inside the string ends the C string. This program stores text in an
environment variable through `SetEnvironmentVariableW` and reads it back (it ran on the PC; each of its first five lines ended in `round trip true`):

<!-- excerpt: tests/testdata/windows/wstr.gp -->
```go
// roundTrip stores s in an environment variable and reads it back.
func roundTrip(s string) string {
	name := C.wstr("GOCPP_WSTR_TEST")
	if windows.SetEnvironmentVariableW(name, C.wstr(s)) == 0 {
		return fmt.Sprint("SetEnvironmentVariableW failed: ", int(windows.GetLastError()))
	}
	var buf [64]windows.WCHAR
	n := windows.GetEnvironmentVariableW(name, &buf[0], 64)
	got := C.GoStringW(&buf[0])
	if gotN := C.GoStringWN(&buf[0], C.int(n)); gotN != got {
		return "GoStringW and GoStringWN differ: " + gotN
	}
	windows.SetEnvironmentVariableW(name, nil)
	return got
}
```

Do not call the narrow `A` functions with a Go string unless you mean the ANSI code page: gocpp programs run with the UTF-8 code page when the
[application manifest](windows.md#console-utf-8-and-exit-codes) is there (Windows 10 1903 and later), so `A` functions read UTF-8 too, but
the `W` functions are what Go's own Windows code uses and what works on every system.

## Types: LLP64

On Windows `long` has 32 bits (LLP64), unlike on Linux (LP64):

| Type | Windows | Linux |
|---|---|---|
| `C.long`, `C.ulong`, `windows.DWORD`, `windows.LONG` | 4 bytes | 8 bytes (`DWORD` is 4) |
| `C.longlong`, `C.size_t` | 8 bytes | 8 bytes |
| `windows.HANDLE`, `windows.HWND`, any pointer | 8 bytes | 8 bytes |
| `C.wchar_t`, `windows.WCHAR` | 2 bytes | 4 bytes |

The constants are checked against the target's sizes: `C.long(1 << 40)` is an error on Windows. The first lines of this program print the sizes
(`4 4 8 8 4 8 2`, run on the PC), then use clocks, a struct out-parameter, files, the registry and a thread through the C API:

<!-- excerpt: tests/testdata/windows/winapi.gp -->
```go
// LLP64: long is 32 bits on Windows, as DWORD; handles and size_t are 64.
fmt.Println("sizes:", unsafe.Sizeof(C.long(0)), unsafe.Sizeof(C.ulong(0)), unsafe.Sizeof(C.longlong(0)),
	unsafe.Sizeof(C.size_t(0)), unsafe.Sizeof(windows.DWORD(0)), unsafe.Sizeof(windows.HANDLE(nil)),
	unsafe.Sizeof(windows.WCHAR(0)))
```

Some things to know when you write such calls:

- **Handles are pointers.** `INVALID_HANDLE_VALUE` is `(HANDLE)-1`, not an integer constant: compare with it
  (`h == windows.INVALID_HANDLE_VALUE`). To compare an error-code macro with an integer, convert it: `int(windows.ERROR_FILE_NOT_FOUND)`.
- **Out-parameters** are Go addresses: `windows.GetSystemInfo(&si)`, `windows.GetClientRect(hwnd, &rc)`; the fields keep their C names
  (`si.dwPageSize`, `wc.lpfnWndProc`). Unions are fields too (`in4.S_un.S_addr`).
- **Callbacks.** x86-64 Windows has a single calling convention, so a plain gocpp function is a `WNDPROC`, a `LPTHREAD_START_ROUTINE` or a timer
  callback. The thread of `winapi.gp` starts like this:

<!-- excerpt: tests/testdata/windows/winapi.gp -->
```go
// threadMain is an LPTHREAD_START_ROUTINE: Windows calls it with the WINAPI convention, which is the
// only convention of x86-64 Windows. A plain gocpp function becomes the C function pointer.
func threadMain(arg windows.LPVOID) windows.DWORD {
	windows.InterlockedIncrement((*windows.LONG)(unsafe.Pointer(&counter)))
	return 42
}

// ...

th := windows.CreateThread(nil, 0, threadMain, nil, 0, nil)
windows.WaitForSingleObject(th, windows.INFINITE)
```

- **Constants in array lengths** (`[windows.MAX_PATH]windows.WCHAR`) are not supported yet; write the number, `[260]windows.WCHAR`.
- **Libraries.** MinGW links `kernel32`, `user32`, `advapi32` and `shell32` by default. Name others with `//gocpp:link gdi32 ws2_32 winmm`. A missing library
  shows as `undefined reference to '__imp_socket'`, and gocpp adds the hint `add //gocpp:link ws2_32` for the common ones. Programs are linked `-static`,
  which skips the import library of a DLL: name it as a file, `//gocpp:ldflags -l:libSDL2.dll.a`, and put the DLL next to the program.

## A window

The example `examples/win32` shows a message box, then registers a window class whose window procedure is a gocpp function, makes a
window and runs the message loop:

<!-- excerpt: examples/win32/main.gp -->
```go
// wndProc is the window procedure: Windows calls it with the messages for
// the window. A plain gocpp function becomes a C function pointer.
func wndProc(hwnd windows.HWND, msg windows.UINT, wparam windows.WPARAM, lparam windows.LPARAM) windows.LRESULT {
	switch msg {
	case windows.WM_CREATE:
		fmt.Println("WM_CREATE")
		return 0
	// ...
	case windows.WM_CLOSE:
		fmt.Println("WM_CLOSE")
		windows.DestroyWindow(hwnd)
		return 0
	case windows.WM_DESTROY:
		fmt.Println("WM_DESTROY")
		windows.PostQuitMessage(0)
		return 0
	}
	return windows.DefWindowProcW(hwnd, msg, wparam, lparam)
}

// ...

// C.WString makes a copy in C's heap, for a pointer that a C struct keeps (wc.lpszClassName); C.wstr is for one
// that a call uses and forgets.
className := C.WString("GocppWindow")
defer C.free(unsafe.Pointer(className))
var wc windows.WNDCLASSW
wc.lpfnWndProc = wndProc
wc.hInstance = instance
wc.hCursor = windows.LoadCursorW(nil, windows.IDC_ARROW)
wc.hbrBackground = windows.HBRUSH(windows.GetStockObject(windows.WHITE_BRUSH))
wc.lpszClassName = className

// ...

// The message loop runs until WM_DESTROY posts WM_QUIT.
var msg windows.MSG
for windows.GetMessageW(&msg, nil, 0, 0) > 0 {
	windows.TranslateMessage(&msg)
	windows.DispatchMessageW(&msg)
}
```

Build and run it on a Windows desktop (`gocpp run examples\win32\main.gp`, with `GOCPP_GATE_PHASE=15`); `GOCPP_SMOKE=1` makes a timer close the
message box and the window at once, which is how the test suite runs it under Wine with a null graphics driver. It builds natively with WinLibs' GCC and from Linux. A
window needs an interactive desktop: started over `ssh` (session 0, no desktop) the smoke mode did not finish on the test PC, so
the example was not seen running on a real desktop for this page. A program with a window opens a console too, unless you link the GUI subsystem:
`gocpp build -ldflags=-mwindows` gave an executable with the GUI subsystem on the PC (the PE header says so; the flag is a trusted one and
goes to the linker as it is).

## Sockets

`<winsock2.h>` and `<ws2tcpip.h>` work like `<windows.h>`: the functions are `winsock2.socket`, `winsock2.bind`, and so on, and
the program links `ws2_32`. This program starts Winsock, makes a listening socket on the loopback interface, accepts one connection in a goroutine and
has the main goroutine connect and send `ping`. It was built natively and from Linux and ran on the PC both ways:

<!-- illustration: Windows only; built natively and cross-built, and run on the real PC, but not a test of the repository -->
```go
//gocpp:link ws2_32
package main

import (
	"fmt"
	"unsafe"

	"<winsock2.h>"
)

// chars is the address of a byte slice as the char* that Winsock takes.
func chars(b []byte) *C.char { return (*C.char)(unsafe.Pointer(&b[0])) }

func check(what string, rc int32) {
	if rc != 0 {
		panic(fmt.Sprint(what, " failed: ", int(winsock2.WSAGetLastError())))
	}
}

func main() {
	var wsa winsock2.WSADATA
	check("WSAStartup", int32(winsock2.WSAStartup(0x0202, &wsa)))
	defer winsock2.WSACleanup()

	listener := winsock2.socket(winsock2.AF_INET, winsock2.SOCK_STREAM, 0)
	var addr winsock2.sockaddr_in
	addr.sin_family = winsock2.AF_INET
	addr.sin_addr.S_un.S_addr = winsock2.htonl(0x7f000001) // 127.0.0.1
	check("bind", int32(winsock2.bind(listener, (*winsock2.sockaddr)(unsafe.Pointer(&addr)), C.int(unsafe.Sizeof(addr)))))
	check("listen", int32(winsock2.listen(listener, 1)))
	n := C.int(unsafe.Sizeof(addr))
	check("getsockname", int32(winsock2.getsockname(listener, (*winsock2.sockaddr)(unsafe.Pointer(&addr)), &n)))

	done := make(chan string)
	go func() {
		conn := winsock2.accept(listener, nil, nil)
		buf := make([]byte, 64)
		got := winsock2.recv(conn, chars(buf), C.int(len(buf)), 0)
		winsock2.send(conn, chars(buf), got, 0)
		winsock2.closesocket(conn)
		done <- string(buf[:got])
	}()

	client := winsock2.socket(winsock2.AF_INET, winsock2.SOCK_STREAM, 0)
	check("connect", int32(winsock2.connect(client, (*winsock2.sockaddr)(unsafe.Pointer(&addr)), C.int(unsafe.Sizeof(addr)))))
	msg := []byte("ping")
	winsock2.send(client, chars(msg), C.int(len(msg)), 0)
	reply := make([]byte, 64)
	m := winsock2.recv(client, chars(reply), C.int(len(reply)), 0)
	fmt.Println("server got:", <-done)
	fmt.Println("client got:", string(reply[:m]))
	winsock2.closesocket(client)
	winsock2.closesocket(listener)
}
```

```text
server got: ping
client got: ping
```

Two things in it are typical. Winsock wants `char*` where Go has `[]byte`, so a small helper converts the address (`unsafe.Pointer` to
`*C.char`). And `sockaddr_in` reaches the generic `sockaddr` pointer through `unsafe.Pointer`, as in C. The name-resolution
functions take wide strings (`ws2tcpip.InetPtonW(winsock2.AF_INET, C.wstr("127.0.0.1"), unsafe.Pointer(&in4))`). `tests/testdata/windows/winzoo.gp` calls
`WSAStartup`, `InetPtonW` and `htons` next to the other header packages of the next section.

## The defaults gocpp sets

For a package for Windows that imports a header or has C or C++ files of its own, every compile of the package gets
`-DWIN32_LEAN_AND_MEAN -DNOMINMAX -DUNICODE -D_UNICODE`. They stop `<windows.h>` from pulling in the old `<winsock.h>` (MinGW warns about a mix with
`<winsock2.h>`), keep the macros `min` and `max` out of C++ code, and make `MessageBox` mean `MessageBoxW` as it does in Go's own Windows code. The same flags reach the generated C++, the
probes and the package's own `.c` and `.cpp` files. The runtime and the sources of a library with a `cxx` recipe do not get them.

- `//gocpp:no-win32-defaults` in the package turns all four off.
- To change one macro, say so in your own flags: `//gocpp:cflags -UUNICODE -U_UNICODE` gives `MessageBoxA` and the other `A` names; `-DNAME` or
  `-UNAME` in `//gocpp:cflags`, in pkg-config's flags or in `GOCPP_CFLAGS`/`GOCPP_CXXFLAGS` leaves that one macro to you.
- A package with these flags does not use the runtime's precompiled header, so its first build is a little slower.

After a C++ error that mentions `wchar_t`, `LPCSTR`, `TCHAR` or `UNICODE`, gocpp adds a line that says whether the package got the defaults and how to change them.

**Macros and names.** `<windows.h>` defines macros with ordinary names (`ERROR`, `DELETE`, `TRUE`, `FALSE`, `MAX_PATH`, `INFINITE`,
`near`, `far`, `IN`, `OUT`, `OPTIONAL`, `VOID`, `GetObject`, `SendMessage`, `CreateWindow`, `DrawText`, `LoadImage`, `RGB`, ...). A name of **your** package that is
such a macro gets a `_` in the generated C++ and keeps its Go name, so this works:

<!-- excerpt: tests/testdata/windows/winmacros.gp -->
```go
const (
	ERROR    = 11
	DELETE   = 12
	CONST    = 13
	TRUE     = 14
	FALSE    = 15
	MAX_PATH = 16
	INFINITE = 17
)
```

The header packages that are tested together (`tests/testdata/windows/winzoo.gp`): `<windows.h>`, `<winsock2.h>`, `<ws2tcpip.h>`, `<shellapi.h>`,
`<shlobj.h>`, `<wincrypt.h>`, `<mmsystem.h>`, `<dxgi.h>`, `<d3d11.h>` and the C runtime's `<io.h>`, `<direct.h>`, `<process.h>`, `<tchar.h>`.

## Your own C and C++ in a Windows package

A package may hold `.c` and `.cpp` files and a header that includes `<windows.h>`: packed structs, bit-fields, a `WCHAR name[16]` in a struct, `__declspec(dllexport)`
and a `WINAPI` function pointer type all go through the compiler. `tests/testdata/windows/winstruct/` is the example (it ran on the PC):

<!-- excerpt: tests/testdata/windows/winstruct/main.gp -->
```go
var e winstruct.Entry
winstruct.fill_entry(&e, 7, C.wstr("héllo 世界"))
fmt.Println("entry:", int(e.id), C.GoStringW(&e.name[0]), e.enabled != 0)
```

The package's files get the same defaults, so C++ in them can use `std::min` next to `<windows.h>`. If such a file lies below a folder whose name is outside the ANSI
code page, gocpp gives the compiler the short name of the path ([Windows page](windows.md#folders-spaces-and-non-ascii-names)).

## What is not there yet

- A function pointer that you get from `GetProcAddress` cannot be called from Go code, and there is no `NewCallback`; link to the function instead.
- COM: the interfaces are C++ classes in MinGW's headers. `dxgi.CreateDXGIFactory1(dxgi.IID_IDXGIFactory1, ...)` works (pass the IID without `&`, call
  `factory.Release()`), but the GUID constants live in libraries (`//gocpp:link dxgi dxguid`), and `__uuidof` has no gocpp form.
- Typed interop (`GOCPP_INTEROP=typed` or `-interop=typed`) for the Windows target: its probe needs a `clang++` and the MinGW include paths, and does not use the
  pinned llvm-mingw's clang yet (planned). On a Windows host `auto` stays `opaque`; `typed` without a clang exits with status 3.
- Libraries built with Visual Studio (C++ ABI); MinGW-w64 builds of sqlite3, zlib, SDL2 and raylib were built and linked (sqlite3 and zlib also ran
  on the PC; SDL2 and raylib need their DLLs and a display and were only linked).
