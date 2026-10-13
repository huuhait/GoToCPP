// C++ equivalent of examples/win32/main.gp, to check its output.txt under Wine.
#include <windows.h>
#include <fcntl.h>
#include <io.h>
#include <cstdio>
#include <cstdlib>
static const wchar_t* boxTitle = L"gocpp";
static const wchar_t* greeting = L"Hello from gocpp!";
static bool smoke = std::getenv("GOCPP_SMOKE") != nullptr && *std::getenv("GOCPP_SMOKE") != 0;
static void say(const char* s) {
    std::printf("%s\n", s);
    std::fflush(stdout);
}
static LRESULT CALLBACK wndProc(HWND hwnd, UINT msg, WPARAM wparam, LPARAM lparam) {
    switch (msg) {
    case WM_CREATE:
        say("WM_CREATE");
        return 0;
    case WM_PAINT: {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        RECT rc;
        GetClientRect(hwnd, &rc);
        DrawTextW(hdc, greeting, -1, &rc, DT_CENTER | DT_VCENTER | DT_SINGLELINE);
        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_CLOSE:
        say("WM_CLOSE");
        DestroyWindow(hwnd);
        return 0;
    case WM_DESTROY:
        say("WM_DESTROY");
        PostQuitMessage(0);
        return 0;
    }
    return DefWindowProcW(hwnd, msg, wparam, lparam);
}
static void CALLBACK closeBox(HWND, UINT, UINT_PTR id, DWORD) {
    const wchar_t* dialog = L"#32770";
    for (HWND box = FindWindowExW(nullptr, nullptr, dialog, boxTitle); box != nullptr;
         box = FindWindowExW(nullptr, box, dialog, boxTitle)) {
        if (GetWindowThreadProcessId(box, nullptr) == GetCurrentThreadId()) {
            KillTimer(nullptr, id);
            PostMessageW(box, WM_COMMAND, IDOK, 0);
            return;
        }
    }
}
int main() {
    // Go (and gocpp) write "\n" as it is; the C runtime's text mode would write "\r\n".
    _setmode(_fileno(stdout), _O_BINARY);
    if (smoke)
        SetTimer(nullptr, 0, 200, closeBox);
    int r = MessageBoxW(nullptr, greeting, boxTitle, MB_OK | MB_ICONINFORMATION);
    std::printf("MessageBoxW returned %d %s\n", r, r == IDOK ? "true" : "false");
    std::fflush(stdout);
    HINSTANCE instance = GetModuleHandleW(nullptr);
    const wchar_t* className = L"GocppWindow";
    WNDCLASSW wc{};
    wc.lpfnWndProc = wndProc;
    wc.hInstance = instance;
    wc.hCursor = LoadCursorW(nullptr, IDC_ARROW);
    wc.hbrBackground = HBRUSH(GetStockObject(WHITE_BRUSH));
    wc.lpszClassName = className;
    if (RegisterClassW(&wc) == 0) {
        std::printf("RegisterClassW failed: %lu\n", GetLastError());
        return 1;
    }
    HWND hwnd = CreateWindowExW(0, className, L"gocpp window", WS_OVERLAPPEDWINDOW, CW_USEDEFAULT,
                                CW_USEDEFAULT, 360, 200, nullptr, nullptr, instance, nullptr);
    if (hwnd == nullptr) {
        std::printf("CreateWindowExW failed: %lu\n", GetLastError());
        return 1;
    }
    if (smoke)
        PostMessageW(hwnd, WM_CLOSE, 0, 0);
    else {
        ShowWindow(hwnd, SW_SHOWDEFAULT);
        UpdateWindow(hwnd);
    }
    MSG msg;
    while (GetMessageW(&msg, nullptr, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    std::printf("exit code %d\n", int(msg.wParam));
    std::fflush(stdout);
}
