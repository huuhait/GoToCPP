// phase: 14

// The Windows header zoo (step 14.4, ADR-082): the common headers of MinGW-w64 imported together, each with a call.
// The order is one that MinGW warns about without the Win32 defaults (windows.h before winsock2.h), and tchar.h's
// _tcslen is wcslen only with _UNICODE. Libraries are named with //gocpp:link.

//gocpp:link ws2_32 winmm advapi32 shell32 ole32 dxgi dxguid

package main

import (
	"fmt"
	"unsafe"

	"<windows.h>"
	"<winsock2.h>"
	"<ws2tcpip.h>"
	"<shellapi.h>"
	"<shlobj.h>"
	"<wincrypt.h>"
	"<mmsystem.h>"
	"<process.h>"
	"<io.h>"
	"<direct.h>"
	"<tchar.h>"
	"<dxgi.h>"
	"<d3d11.h>"
)

func main() {
	// windows.h: kernel32.
	fmt.Println("windows:", windows.GetTickCount64() > 0, int(windows.GetCurrentProcessId()) == int(process._getpid()))

	// winsock2.h and ws2tcpip.h: start Winsock, resolve a number (no network), stop it.
	var wsa winsock2.WSADATA
	fmt.Println("winsock2: WSAStartup", int(winsock2.WSAStartup(0x0202, &wsa)), int(wsa.wVersion))
	var in4 winsock2.in_addr
	fmt.Println("ws2tcpip: inet_pton", int(ws2tcpip.InetPtonW(winsock2.AF_INET, C.wstr("127.0.0.1"), unsafe.Pointer(&in4))),
		int(winsock2.ntohl(winsock2.u_long(in4.S_un.S_addr))) == 0x7f000001)
	fmt.Println("winsock2: htons", int(winsock2.htons(0x1234)))
	winsock2.WSACleanup()

	// shellapi.h and shlobj.h.
	var argc C.int
	argv := shellapi.CommandLineToArgvW(C.wstr("prog.exe a \"b c\""), &argc)
	fmt.Println("shellapi: CommandLineToArgvW", int(argc), C.GoStringW((*windows.WCHAR)(unsafe.Pointer(*(**windows.WCHAR)(unsafe.Add(unsafe.Pointer(argv), 8))))))
	windows.LocalFree(windows.HLOCAL(unsafe.Pointer(argv)))
	var folder [260]windows.WCHAR
	hr := shlobj.SHGetFolderPathW(nil, shlobj.CSIDL_PROFILE, nil, 0, &folder[0])
	fmt.Println("shlobj: SHGetFolderPathW", int(hr), len(C.GoStringW(&folder[0])) > 3)

	// wincrypt.h: random bytes from the system provider.
	var prov wincrypt.HCRYPTPROV
	ok := wincrypt.CryptAcquireContextW(&prov, nil, nil, wincrypt.PROV_RSA_FULL, wincrypt.CRYPT_VERIFYCONTEXT)
	var rnd [16]byte
	ok2 := wincrypt.CryptGenRandom(prov, 16, &rnd[0])
	wincrypt.CryptReleaseContext(prov, 0)
	fmt.Println("wincrypt:", ok != 0, ok2 != 0, rnd != [16]byte{})

	// mmsystem.h (winmm).
	fmt.Println("mmsystem: timeGetTime", mmsystem.timeGetTime() > 0)

	// C runtime headers of MinGW: process.h (above), io.h, direct.h, tchar.h (UNICODE: _tcslen is wcslen).
	fmt.Println("io: _access", int(io._access(C.cstr("."), 0)))
	cwd := direct._getcwd(nil, 0)
	fmt.Println("direct: _getcwd", cwd != nil, len(C.GoString(cwd)) > 2)
	fmt.Println("tchar: _tcslen", int(tchar._tcslen(C.wstr("abc 世"))))

	// dxgi.h and d3d11.h: a COM factory without a window, and the sizes of two structs.
	var factory *dxgi.IDXGIFactory1
	hr = dxgi.CreateDXGIFactory1(dxgi.IID_IDXGIFactory1, (*unsafe.Pointer)(unsafe.Pointer(&factory)))
	fmt.Println("dxgi: CreateDXGIFactory1", int(hr), factory != nil)
	if factory != nil {
		factory.Release()
	}
	fmt.Println("d3d11:", int(d3d11.D3D11_SDK_VERSION), unsafe.Sizeof(d3d11.D3D11_BUFFER_DESC{}), unsafe.Sizeof(dxgi.DXGI_SWAP_CHAIN_DESC{}),
		int(dxgi.DXGI_FORMAT_R8G8B8A8_UNORM))
}
