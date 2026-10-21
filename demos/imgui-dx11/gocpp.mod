module example.com/imguidx11

require github.com/ocornut/imgui v1.91.5

cxx github.com/ocornut/imgui {
	src      imgui.cpp imgui_draw.cpp imgui_tables.cpp imgui_widgets.cpp imgui_demo.cpp
	src      backends/imgui_impl_win32.cpp backends/imgui_impl_dx11.cpp
	include  . backends
	if windows {
		// d3d11, dxgi and d3dcompiler are Direct3D 11, DXGI and the shader compiler; dxguid holds the
		// IID_ constants; dwmapi and gdi32 are the window manager and GDI.
		link d3d11 dxgi d3dcompiler dxguid dwmapi gdi32
	}
}
