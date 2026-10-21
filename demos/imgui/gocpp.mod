module example.com/imguidemo

require github.com/ocornut/imgui v1.91.5

cxx github.com/ocornut/imgui {
	src      imgui.cpp imgui_draw.cpp imgui_tables.cpp imgui_widgets.cpp imgui_demo.cpp
	src      backends/imgui_impl_glfw.cpp backends/imgui_impl_opengl3.cpp
	include  . backends
	if windows {
		// glfw3dll is MinGW's import library for glfw3.dll, opengl32 is Windows' OpenGL library, and
		// gdi32 and dwmapi are the system libraries (GDI and the window manager) that GLFW needs.
		link glfw3dll opengl32 gdi32 dwmapi
	}
}
