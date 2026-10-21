// win32forward.hpp: the one declaration that imgui_impl_win32.h leaves out
// (it is inside "#if 0" there, so that the header does not need <windows.h>).
// The function is defined in imgui_impl_win32.cpp.
#pragma once

#include <windows.h>

extern LRESULT ImGui_ImplWin32_WndProcHandler(HWND hWnd, UINT msg, WPARAM wParam, LPARAM lParam);
