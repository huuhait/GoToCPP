# Dear ImGui demo (Windows)

A window made with [Dear ImGui](https://github.com/ocornut/imgui) 1.91.5, GLFW and OpenGL 3, written in gocpp:
text, a slider, a color editor, a button with a counter, and a checkbox that opens ImGui's own demo window.

It is a demo and not one of the [target programs](../../examples/README.md): it needs a network (the library is
downloaded), a GPU with OpenGL 3, and a display, so no test runs it. It was built on Linux with MinGW-w64 and run
on a real Windows PC, on the console desktop (2026-10-21).

## What is in this folder

| File | What it is |
|---|---|
| `main.gp` | the program: it calls `ImGui.Begin`, `ImGui.SliderFloat`, ... and the GLFW and OpenGL functions |
| `gocpp.mod` | `require github.com/ocornut/imgui v1.91.5`, and the `cxx` recipe that builds ImGui and its GLFW and OpenGL 3 backends |
| `gocpp.sum` | the hash of the downloaded library |

ImGui itself is not copied here: `gocpp get` (or the first build) downloads it into the module cache.

## Build and run on Windows

The program needs the GLFW headers, GLFW's import library (`libglfw3dll.a`) and `glfw3.dll` for MinGW-w64. On
NixOS they come from nixpkgs:

```text
nix build --inputs-from . 'nixpkgs#pkgsCross.mingwW64.glfw' 'nixpkgs#pkgsCross.mingwW64.glfw.dev' --no-link --print-out-paths
```

Put the `include/GLFW` folder of the `.dev` output and the `lib` folder of the other one in a folder `glfw/`, then
build from this folder:

```text
GOCPP_CFLAGS=-I<glfw>/include GOCPP_LDFLAGS=-L<glfw>/lib gocpp build -os windows -o imgui.exe .
```

Copy `imgui.exe` and `glfw3.dll` to the PC and start `imgui.exe` from a desktop session. (A program started over ssh
runs in session 0, which has no desktop: start it from the console, or through a scheduled task with `/it`.)
Closing the window ends the program, which prints `window closed, clicks: N`.

On a Windows host with MinGW-w64 (MSYS2: `mingw-w64-x86_64-glfw`), `gocpp build .` does the same without `-os`.

## Notes

- Typed interop is the default when a clang++ is found, and it builds this demo without a flag. Its probes read the ImGui and
  GLFW headers: about 6 s more on a first build with an empty probe cache (the machine was loaded), a few hundred milliseconds
  more on a build with a warm one. `-interop=opaque` (or `GOCPP_INTEROP=opaque`) skips the probes; the program is the same.
- `//gocpp:` link flags are in the recipe: `link glfw3dll opengl32 gdi32 dwmapi` in an `if windows { ... }` block. The recipe
  has no block for Linux or macOS, so this folder builds only for Windows; for another system add a block with the GLFW and
  OpenGL libraries (on Linux `pkg-config glfw3` and `link GL` in an `if linux { ... }` block,
  [spec](../../docs/spec/modules.md#c-and-c-libraries-cxx-recipes)).
- `imgui_demo.cpp` is in the recipe because the checkbox calls `ImGui.ShowDemoWindow`.
- Out-parameters of the widgets are C values: `var speed C.float` and `&speed`, `var showDemo bool` and `&showDemo`,
  `&clear[0]` for the color array.
