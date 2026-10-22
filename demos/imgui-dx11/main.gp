// Dear ImGui on Windows with its Win32 and Direct3D 11 backends: a Win32
// window (window class, window procedure and message loop written in gocpp),
// a D3D11 device with a swap chain, and ImGui drawing into it. Nothing in
// this folder is C++ source: the one function that imgui_impl_win32.h does
// not declare is declared in this file with //gocpp:extern. The library is
// downloaded by `gocpp get` and built from the cxx recipe in gocpp.mod. See
// README.md for how to build and run it.
package main

import (
	"fmt"
	"math"
	"os"
	"time"
	"unsafe"

	"github.com/ocornut/imgui/backends/imgui_impl_dx11.h"
	"github.com/ocornut/imgui/backends/imgui_impl_win32.h"
	"github.com/ocornut/imgui/imgui.h"

	"<d3d11.h>"
	"<windows.h>"
)

// ImGui_ImplWin32_WndProcHandler is defined in imgui_impl_win32.cpp. imgui_impl_win32.h keeps its
// declaration inside "#if 0" (so that the header does not need <windows.h>) and tells users to copy
// it into their own file: here it is copied into the .gp file. The directive has C++ linkage (the
// default) and the global namespace, as the library defines the function.
//
//gocpp:extern
func ImGui_ImplWin32_WndProcHandler(hwnd windows.HWND, msg windows.UINT,
	wparam windows.WPARAM, lparam windows.LPARAM) windows.LRESULT

// The Direct3D objects live as long as the window.
var (
	device    *d3d11.ID3D11Device
	context   *d3d11.ID3D11DeviceContext
	swapChain *d3d11.IDXGISwapChain
	target    *d3d11.ID3D11RenderTargetView
)

func createTarget() {
	var backBuffer *d3d11.ID3D11Texture2D
	swapChain.GetBuffer(0, d3d11.IID_ID3D11Texture2D, &backBuffer)
	device.CreateRenderTargetView(backBuffer, nil, &target)
	backBuffer.Release()
}

func releaseTarget() {
	if target != nil {
		target.Release()
		target = nil
	}
}

func createDevice(hwnd windows.HWND) bool {
	var desc d3d11.DXGI_SWAP_CHAIN_DESC
	desc.BufferCount = 2
	desc.BufferDesc.Format = d3d11.DXGI_FORMAT_R8G8B8A8_UNORM
	desc.BufferUsage = d3d11.DXGI_USAGE_RENDER_TARGET_OUTPUT
	desc.OutputWindow = hwnd
	desc.SampleDesc.Count = 1
	desc.Windowed = 1
	desc.SwapEffect = d3d11.DXGI_SWAP_EFFECT_DISCARD

	levels := [2]d3d11.D3D_FEATURE_LEVEL{d3d11.D3D_FEATURE_LEVEL_11_0, d3d11.D3D_FEATURE_LEVEL_10_0}
	var got d3d11.D3D_FEATURE_LEVEL
	hr := d3d11.D3D11CreateDeviceAndSwapChain(nil, d3d11.D3D_DRIVER_TYPE_HARDWARE, nil, 0, &levels[0], 2,
		d3d11.D3D11_SDK_VERSION, &desc, &swapChain, &device, &got, &context)
	if hr != 0 {
		fmt.Printf("D3D11CreateDeviceAndSwapChain failed: 0x%x\n", uint32(hr))
		return false
	}
	createTarget()
	return true
}

// wndProc gives every message to ImGui first (mouse, keyboard, focus), then
// handles resizing, so that the swap chain follows the window.
func wndProc(hwnd windows.HWND, msg windows.UINT, wparam windows.WPARAM, lparam windows.LPARAM) windows.LRESULT {
	if ImGui_ImplWin32_WndProcHandler(hwnd, msg, wparam, lparam) != 0 {
		return 1
	}
	switch msg {
	case windows.WM_SIZE:
		if device != nil && wparam != windows.SIZE_MINIMIZED {
			releaseTarget()
			swapChain.ResizeBuffers(0, windows.UINT(lparam&0xffff), windows.UINT((lparam>>16)&0xffff),
				d3d11.DXGI_FORMAT_UNKNOWN, 0)
			createTarget()
		}
		return 0
	case windows.WM_DESTROY:
		windows.PostQuitMessage(0)
		return 0
	}
	return windows.DefWindowProcW(hwnd, msg, wparam, lparam)
}

func main() {
	imgui_impl_win32.ImGui_ImplWin32_EnableDpiAwareness()

	instance := windows.GetModuleHandleW(nil)
	className := C.WString("GocppImGuiDX11")
	defer C.free(unsafe.Pointer(className))
	var wc windows.WNDCLASSEXW
	wc.cbSize = windows.UINT(unsafe.Sizeof(wc))
	wc.style = windows.CS_CLASSDC
	wc.lpfnWndProc = wndProc
	wc.hInstance = instance
	wc.hCursor = windows.LoadCursorW(nil, windows.IDC_ARROW)
	wc.lpszClassName = className
	if windows.RegisterClassExW(&wc) == 0 {
		fmt.Println("RegisterClassExW failed:", int(windows.GetLastError()))
		os.Exit(1)
	}
	hwnd := windows.CreateWindowExW(0, className, "gocpp + Dear ImGui + Direct3D 11",
		windows.WS_OVERLAPPEDWINDOW, 100, 100, 1100, 700, nil, nil, instance, nil)
	if hwnd == nil {
		fmt.Println("CreateWindowExW failed:", int(windows.GetLastError()))
		os.Exit(1)
	}
	if !createDevice(hwnd) {
		os.Exit(1)
	}
	windows.ShowWindow(hwnd, windows.SW_SHOWDEFAULT)
	windows.UpdateWindow(hwnd)

	imgui.ImGui.CreateContext()
	imgui.ImGui.StyleColorsLight()
	imgui_impl_win32.ImGui_ImplWin32_Init(unsafe.Pointer(hwnd))
	imgui_impl_dx11.ImGui_ImplDX11_Init(device, context)

	speed := float32(1)
	var showDemo bool
	var clicks int
	clear := [4]float32{0.93, 0.95, 0.98, 1}
	var wave [120]float32
	phase := 0.0
	frames := 0
	occluded := false
	started := time.Now()

	var msg windows.MSG
	running := true
	for running {
		for windows.PeekMessageW(&msg, nil, 0, 0, windows.PM_REMOVE) != 0 {
			windows.TranslateMessage(&msg)
			windows.DispatchMessageW(&msg)
			if msg.message == windows.WM_QUIT {
				running = false
			}
		}
		if !running {
			break
		}

		// A window that is covered or minimized has nothing to show: Present then returns at once
		// (DXGI_STATUS_OCCLUDED) instead of waiting for the screen, and the loop would use a whole
		// core. Sleep until a test Present says the window is visible again.
		if occluded && swapChain.Present(0, d3d11.DXGI_PRESENT_TEST) == d3d11.DXGI_STATUS_OCCLUDED {
			windows.Sleep(10)
			continue
		}
		occluded = false
		frameStart := time.Now()

		imgui_impl_dx11.ImGui_ImplDX11_NewFrame()
		imgui_impl_win32.ImGui_ImplWin32_NewFrame()
		imgui.ImGui.NewFrame()

		phase += float64(speed) * 0.05
		for i := range wave {
			wave[i] = float32(math.Sin(phase + float64(i)*0.12))
		}

		imgui.ImGui.Begin("gocpp + Direct3D 11")
		imgui.ImGui.Text("Win32 window, D3D11 swap chain and ImGui, all driven from gocpp.")
		imgui.ImGui.SliderFloat("wave speed", &speed, 0, 4)
		imgui.ImGui.PlotLines("wave", &wave[0], 120, 0, nil, -1, 1, imgui.ImVec2{0, 80})
		imgui.ImGui.ColorEdit3("background", &clear[0])
		imgui.ImGui.Checkbox("show the ImGui demo window", &showDemo)
		if imgui.ImGui.Button("click me") {
			clicks++
		}
		imgui.ImGui.SameLine()
		imgui.ImGui.Text("%s", fmt.Sprintf("clicked %d times, frame %d", clicks, frames))
		imgui.ImGui.End()
		if showDemo {
			imgui.ImGui.ShowDemoWindow(&showDemo)
		}
		imgui.ImGui.Render()

		context.OMSetRenderTargets(1, &target, nil)
		context.ClearRenderTargetView(target, &clear[0])
		imgui_impl_dx11.ImGui_ImplDX11_RenderDrawData(imgui.ImGui.GetDrawData())
		occluded = swapChain.Present(1, 0) == d3d11.DXGI_STATUS_OCCLUDED
		frames++

		// Present(1, 0) waits for the screen's refresh. A display driver that does not (a remote
		// session, some virtual displays) would let the loop run at thousands of frames a second,
		// so a frame is also kept to about 60 a second.
		if left := 16*time.Millisecond - time.Since(frameStart); left > 0 {
			time.Sleep(left)
		}
	}

	imgui_impl_dx11.ImGui_ImplDX11_Shutdown()
	imgui_impl_win32.ImGui_ImplWin32_Shutdown()
	imgui.ImGui.DestroyContext()
	releaseTarget()
	swapChain.Release()
	context.Release()
	device.Release()
	windows.DestroyWindow(hwnd)
	windows.UnregisterClassW(className, instance)
	seconds := time.Since(started).Seconds()
	fmt.Printf("window closed, frames: %d in %.1f s (%.1f per second), clicks: %d\n", frames, seconds,
		float64(frames)/seconds, clicks)
}
