package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Hints for Windows programs (step 14.4, ADR-082): what a user sees when the Win32 defaults or a missing
// import library are the cause of an error.

// win32WideRx matches the C++ errors that come from the ANSI/Wide switch of <windows.h>: a wchar_t* given
// where an LPCSTR/char* is wanted (UNICODE is off) or the other way round.
var win32WideRx = regexp.MustCompile(`wchar_t|LPC?W?STR|LPC?TSTR|\bTCHAR\b|\bWCHAR\b|char16_t|UNICODE`)

// win32DefaultsHint returns the hint line for C++ errors msgs of a package for goos, or "". defaults is the number of
// flags of the Win32 defaults the package got (build.PackageFlags.Defaults): when it is 0 the package is for windows
// and uses C/C++, so the defaults were turned off (//gocpp:no-win32-defaults, or flags of the user that name the macros).
func win32DefaultsHint(goos string, defaults int, msgs []string) string {
	if goos != "windows" {
		return ""
	}
	hit := false
	for _, m := range msgs {
		if win32WideRx.MatchString(m) {
			hit = true
			break
		}
	}
	if !hit {
		return ""
	}
	if defaults > 0 {
		return "(-DUNICODE -D_UNICODE -DWIN32_LEAN_AND_MEAN -DNOMINMAX are on by default for Windows packages, so MessageBox, CreateFile and the like mean their W functions, which take wide strings (C.wstr(s)); //gocpp:no-win32-defaults, or -UUNICODE -U_UNICODE in //gocpp:cflags, changes this)"
	}
	return "(this Windows package does not get the Win32 defaults (//gocpp:no-win32-defaults, or your own -D/-U flags for them), so UNICODE may be off: MessageBox, CreateFile and the like mean their A functions, which take char* (C.cstr(s)), and a wide string (C.wstr(s)) is for the W functions; remove the directive or flag to get -DUNICODE -D_UNICODE back)"
}

// An importLib is a Windows import library and the names of the functions it has, for the hint after a link error
// 'undefined reference to __imp_socket'. Only the common ones; gcc links kernel32, user32, advapi32 and shell32 by
// default, the others are named with //gocpp:link.
type importLib struct {
	lib string
	rx  *regexp.Regexp
}

var importLibs = func() []importLib {
	var l []importLib
	add := func(lib, names string) { l = append(l, importLib{lib, regexp.MustCompile(`^(?:` + names + `)$`)}) }
	add("ws2_32", `WSA\w+|socket|closesocket|connect|bind|listen|accept|send|sendto|recv|recvfrom|select|shutdown|ioctlsocket|setsockopt|getsockopt|getsockname|getpeername|gethostbyname|gethostbyaddr|gethostname|getaddrinfo|freeaddrinfo|getnameinfo|inet_\w+|InetNtop\w|InetPton\w|htons|htonl|ntohs|ntohl|__WSAFDIsSet`)
	add("winmm", `time(?:GetTime|BeginPeriod|EndPeriod|GetDevCaps|SetEvent|KillEvent|GetSystemTime)|PlaySound\w*|wave(?:Out|In)\w+|midi(?:Out|In)\w+|mciSendString\w|joyGetPos\w*|mixer\w+`)
	add("gdi32", `CreateCompatible\w+|BitBlt|StretchBlt|SelectObject|DeleteObject|DeleteDC|GetDeviceCaps|CreateFont\w*|CreateSolidBrush|CreatePen|SetPixel|GetPixel|TextOut\w|SetBkMode|SetBkColor|SetTextColor|GetTextExtent\w+|Rectangle|Ellipse|LineTo|MoveToEx|CreateDIBSection|GetDIBits|SetDIBits|StretchDIBits|SwapBuffers|ChoosePixelFormat|SetPixelFormat|GetStockObject|CreateBitmap|GetObject[AW]|CreateRectRgn|SaveDC|RestoreDC|GetTextMetrics\w`)
	add("ole32", `Co[A-Z]\w*|Ole\w+|StringFromGUID2|StringFromCLSID|CLSIDFromString|ProgIDFromCLSID|CreateStreamOnHGlobal`)
	add("oleaut32", `Sys(?:AllocString\w*|FreeString|StringLen)|Variant\w+|SafeArray\w+|SetErrorInfo|GetErrorInfo|LoadTypeLib\w*`)
	add("crypt32", `Cert\w+|CryptProtect\w+|CryptUnprotect\w+|CryptBinaryToString\w|CryptStringToBinary\w|CryptDecodeObject\w*|CryptEncodeObject\w*|CryptMsg\w+|PFX\w+`)
	add("bcrypt", `BCrypt\w+`)
	add("comdlg32", `GetOpenFileName\w|GetSaveFileName\w|ChooseColor\w|ChooseFont\w|PrintDlg\w*|PageSetupDlg\w|CommDlgExtendedError`)
	add("comctl32", `InitCommonControls\w*|ImageList_\w+|CreateStatusWindow\w|CreateToolbarEx|TaskDialog\w*|DefSubclassProc|SetWindowSubclass|RemoveWindowSubclass`)
	add("version", `GetFileVersionInfo\w*|VerQueryValue\w|VerLanguageName\w`)
	add("wininet", `Internet\w+|Http\w+|FtpFindFirstFile\w|FtpGetFile\w`)
	add("iphlpapi", `GetAdaptersInfo|GetAdaptersAddresses|GetIpAddrTable|GetIfTable\w*|GetBestRoute\w*|GetIpForwardTable|IcmpSendEcho\w*|IcmpCreateFile|IcmpCloseHandle|GetNetworkParams|GetTcpTable\w*|GetUdpTable\w*|GetIfEntry\w*|GetBestInterface\w*|if_nametoindex|if_indextoname`)
	add("setupapi", `SetupDi\w+|SetupCopy\w+|SetupGetInf\w+|SetupOpenInf\w+`)
	add("shlwapi", `Path\w+|Str(?:Cmp|Str|Trim|Dup|ToInt|Cat|Chr|Spn|CSpn|Rchr|Ret|Format|Case)\w*|SHCreate\w*Stream\w*|SHRegGet\w+|SHDeleteKey\w|SHGetValue\w|SHSetValue\w|Url(?:Combine|Escape|Unescape|Canonicalize|GetPart)\w|AssocQueryString\w`)
	add("d3d11", `D3D11\w+`)
	add("dxgi", `CreateDXGIFactory\w*|DXGIGetDebugInterface\w*`)
	add("dxguid", `(?:IID|CLSID|GUID)_(?:IDXGI|ID3D|IDWrite|ID2D|IDirect|IXAudio|IMMDevice|IAudio)\w*`)
	add("uuid", `(?:IID|CLSID|GUID|FOLDERID|DIID|LIBID)_\w+`)
	add("user32", `MessageBox\w*|CreateWindow\w+|DefWindowProc\w|RegisterClass\w+|GetMessage\w|PeekMessage\w|DispatchMessage\w|TranslateMessage|PostQuitMessage|ShowWindow|UpdateWindow|GetDC|ReleaseDC|SetWindowText\w|GetSystemMetrics|LoadCursor\w|LoadIcon\w|SendMessage\w|PostMessage\w|SetTimer|KillTimer|GetClientRect|GetWindowRect|DrawText\w|GetAsyncKeyState|GetKeyState|SetForegroundWindow|FindWindow\w*|EnumWindows|CharUpper\w+|CharLower\w+|wsprintf\w|GetCursorPos|SetCursorPos`)
	add("advapi32", `Reg\w+|OpenProcessToken|LookupPrivilegeValue\w|AdjustTokenPrivileges|OpenSCManager\w|CreateService\w|StartService\w|CryptAcquireContext\w|CryptGenRandom|CryptReleaseContext|GetUserName\w|InitiateSystemShutdown\w|GetTokenInformation|OpenEventLog\w|ReportEvent\w`)
	add("shell32", `Shell\w+|SHGetFolderPath\w|SHGetKnownFolderPath|SHBrowseForFolder\w|SHFileOperation\w|SHGetSpecialFolder\w+|SHGetFileInfo\w|SHCreateDirectory\w+|CommandLineToArgvW|DragQueryFile\w|ExtractIcon\w*`)
	return l
}()

// importLibHint returns the hint line for the undefined symbols syms of a link for goos, or "": the libraries
// (in the order of the symbols) that have them.
func importLibHint(goos string, syms []string) string {
	if goos != "windows" {
		return ""
	}
	var libs []string
	seen := make(map[string]bool)
	for _, s := range syms {
		name := strings.TrimPrefix(s, "__imp_")
		for _, il := range importLibs {
			if il.rx.MatchString(name) {
				if !seen[il.lib] {
					seen[il.lib] = true
					libs = append(libs, il.lib)
				}
				break
			}
		}
	}
	if len(libs) == 0 {
		return ""
	}
	return fmt.Sprintf("(the program does not link the Windows libraries that have these functions: add //gocpp:link %s; kernel32, user32, advapi32 and shell32 are linked by default, and the GUID constants IID_* and CLSID_* are in uuid, or dxguid for DirectX)", strings.Join(libs, " "))
}
