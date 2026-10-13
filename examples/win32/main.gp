// phase: 14

//go:build windows

// Win32 is a Windows program that calls the Win32 API through the
// <windows.h> header package: it shows a message box (MessageBoxW), then a
// small window whose window procedure is a gocpp function. It builds only
// for Windows (gocpp build -os windows), and runs there or under Wine.
//
// When the environment variable GOCPP_SMOKE is set, a timer closes the
// message box and the window closes as soon as it is made, so the program
// can run in CI.
package main

import (
	"fmt"
	"os"

	"<windows.h>"
)

const (
	boxTitle = "gocpp"
	greeting = "Hello from gocpp!"
)

var smoke = os.Getenv("GOCPP_SMOKE") != ""

// wndProc is the window procedure: Windows calls it with the messages for
// the window. A plain gocpp function becomes a C function pointer.
func wndProc(hwnd windows.HWND, msg windows.UINT, wparam windows.WPARAM, lparam windows.LPARAM) windows.LRESULT {
	switch msg {
	case windows.WM_CREATE:
		fmt.Println("WM_CREATE")
		return 0
	case windows.WM_PAINT:
		var ps windows.PAINTSTRUCT
		hdc := windows.BeginPaint(hwnd, &ps)
		var rc windows.RECT
		windows.GetClientRect(hwnd, &rc)
		windows.DrawTextW(hdc, C.wstr(greeting), -1, &rc, windows.DT_CENTER|windows.DT_VCENTER|windows.DT_SINGLELINE)
		windows.EndPaint(hwnd, &ps)
		return 0
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

// closeBox is a timer callback. In smoke mode it closes the message box as
// if its OK button was clicked. The box is a dialog (window class "#32770")
// with the title boxTitle, shown by this thread; another program, or another
// run of this one, may show a window with the same title. Until the box is
// there, the timer fires again.
func closeBox(_ windows.HWND, _ windows.UINT, id windows.UINT_PTR, _ windows.DWORD) {
	dialog, title := C.wstr("#32770"), C.wstr(boxTitle)
	for box := windows.FindWindowExW(nil, nil, dialog, title); box != nil; box = windows.FindWindowExW(nil, box, dialog, title) {
		if windows.GetWindowThreadProcessId(box, nil) == windows.GetCurrentThreadId() {
			windows.KillTimer(nil, id)
			windows.PostMessageW(box, windows.WM_COMMAND, windows.IDOK, 0)
			return
		}
	}
}

func main() {
	if smoke {
		windows.SetTimer(nil, 0, 200, closeBox)
	}
	// C.wstr makes a UTF-16 copy of a gocpp string for the W functions.
	r := windows.MessageBoxW(nil, C.wstr(greeting), C.wstr(boxTitle), windows.MB_OK|windows.MB_ICONINFORMATION)
	fmt.Println("MessageBoxW returned", int(r), r == windows.IDOK)

	instance := windows.GetModuleHandleW(nil)
	className := C.wstr("GocppWindow") // kept in a variable: the class keeps the pointer
	var wc windows.WNDCLASSW
	wc.lpfnWndProc = wndProc
	wc.hInstance = instance
	wc.hCursor = windows.LoadCursorW(nil, windows.IDC_ARROW)
	wc.hbrBackground = windows.HBRUSH(windows.GetStockObject(windows.WHITE_BRUSH))
	wc.lpszClassName = className
	if windows.RegisterClassW(&wc) == 0 {
		fmt.Println("RegisterClassW failed:", int(windows.GetLastError()))
		os.Exit(1)
	}
	hwnd := windows.CreateWindowExW(0, className, C.wstr("gocpp window"), windows.WS_OVERLAPPEDWINDOW,
		windows.CW_USEDEFAULT, windows.CW_USEDEFAULT, 360, 200, nil, nil, instance, nil)
	if hwnd == nil {
		fmt.Println("CreateWindowExW failed:", int(windows.GetLastError()))
		os.Exit(1)
	}
	if smoke {
		windows.PostMessageW(hwnd, windows.WM_CLOSE, 0, 0)
	} else {
		windows.ShowWindow(hwnd, windows.SW_SHOWDEFAULT)
		windows.UpdateWindow(hwnd)
	}

	// The message loop runs until WM_DESTROY posts WM_QUIT.
	var msg windows.MSG
	for windows.GetMessageW(&msg, nil, 0, 0) > 0 {
		windows.TranslateMessage(&msg)
		windows.DispatchMessageW(&msg)
	}
	fmt.Println("exit code", int(msg.wParam))
}
