# Dear ImGui with Direct3D 11 (Windows)

The same library as [demos/imgui](../imgui/README.md), Dear ImGui 1.91.5, but with ImGui's own Windows backends
instead of GLFW and OpenGL: the window, its class, its window procedure and the message loop are written in gocpp
with the `<windows.h>` header package, the Direct3D 11 device and swap chain with the `<d3d11.h>` header package
(COM interfaces called as methods: `swapChain.GetBuffer(...)`, `context.ClearRenderTargetView(...)`), and the widgets
(text, a slider, a plot of a moving sine wave, a color editor, a button, ImGui's demo window) as `ImGui.Begin`, ...

It is a demo and not one of the [target programs](../../examples/README.md): the library is downloaded (a network is
needed), the window needs a Direct3D 11 GPU and a desktop, so no test runs it. It was built on Linux with MinGW-w64
and run on a real Windows PC, on the console desktop (2026-10-21).

## What is in this folder

| File | What it is |
|---|---|
| `main.gp` | the program, with the one declaration that the library's headers lack: `ImGui_ImplWin32_WndProcHandler` (see below) |
| `gocpp.mod`, `gocpp.sum` | `require github.com/ocornut/imgui v1.91.5`, the `cxx` recipe that builds ImGui with `imgui_impl_win32.cpp` and `imgui_impl_dx11.cpp`, and the hash of the download |

## Build and run

No other library is needed: MinGW-w64 has the Direct3D 11, DXGI and `d3dcompiler` import libraries.

```text
gocpp build -os windows -o imguidx11.exe .
```

(on a Windows host with MinGW-w64: `gocpp build .`). The program is written for typed interop (ADR-130, the default when a clang++ is found):
it passes `&backBuffer` for a `void**`, a plain string for a `LPCWSTR`, and `Text("%s", s)` for a printf-like function. A machine without
clang++, or `-interop=opaque`, stops at the first of them with the C++ compiler's own error.

That `.exe` is 63 MB, of which 5.9 MB is the program: the rest is debug information and the symbol table.
For a small one, add `-release -strip` (`-O2` with link-time optimization, no debug information, no symbol table):

```text
gocpp build -release -strip -os windows -o imguidx11.exe .
```

It is 3.3 MB (`-release` alone: 7.4 MB, `-strip` alone: 5.9 MB; measured, not run: the demo needs a display)
([build flags](../../docs/spec/build.md#program-size)). Keep the default build to debug the demo.

Copy `imguidx11.exe` to the PC and start it from a desktop
session; a program started over ssh runs in session 0, which has no desktop (use the console, or a scheduled
task with `/it`). Closing the window ends the program, which prints `window closed, frames: N, clicks: N`.

## Notes

- `imgui_impl_win32.h` keeps the declaration of `ImGui_ImplWin32_WndProcHandler` inside `#if 0` (so that the header
  does not need `<windows.h>`) and tells users to copy it into their own file. In gocpp that is a function without a
  body and a `//gocpp:extern` line in its doc comment (C++ linkage, the global namespace, the library's own parameter
  types written with the `<windows.h>` header package): gocpp writes the declaration into the generated header, so the
  folder has no C++ file ([ADR-128](../../docs/adr/128-extern-functions-are-declared-in-gp-with-a-directive.md)).
- The recipe links `d3d11 dxgi d3dcompiler dxguid dwmapi gdi32` in an `if windows { ... }` block. `imgui_impl_dx11.cpp` asks for `d3dcompiler` with
  `#pragma comment(lib, ...)`, which MinGW ignores, so the recipe names it. `dxguid` holds the `IID_` constants.
- Typed interop is the default when a clang++ is found, and it builds this demo without a flag. Its probes read `<windows.h>`,
  `<d3d11.h>`, `<dxgi.h>` and the ImGui headers: about 14 s more on a first build with an empty probe cache (the machine was
  loaded), 0.3 to 0.4 s more on a build with a warm one. `-interop=opaque` (or `GOCPP_INTEROP=opaque`) skips the probes; the
  program is the same.
- `GetBuffer` wants a `void**`: typed mode takes `&backBuffer` of a `*d3d11.ID3D11Texture2D` variable and stores the pointer in it
  when the call returns. `CreateWindowExW` takes the title as a Go string (a wide copy that lives until the end of the statement).
  The window class name stays a `C.WString`, because `WNDCLASSEXW` keeps the pointer.
- A covered or minimized window: `Present` returns at once with `DXGI_STATUS_OCCLUDED` instead of waiting for the
  screen, so the loop sleeps (10 ms) until a test `Present` says the window is visible again. Measured on the PC
  with the window minimized: 18.3 CPU seconds in 10 s before, 0.02 after. With the window visible both use about 1 s
  in 10 s. The loop also keeps a frame to about 60 per second, for a display driver that does not wait for the refresh.
- The first run of the library takes a while (about 40,000 lines of C++ are compiled once); later builds use the object cache.
