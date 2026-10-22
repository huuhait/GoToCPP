/* The result types that the view writes as error (ADR-137): the typedef HRESULT, and only it. */
typedef long LONG;
typedef LONG HRESULT;
typedef HRESULT MYRESULT;
typedef long LRESULT;
typedef long NTSTATUS;
typedef unsigned long DWORD;

HRESULT create_device(int flags);
MYRESULT my_call(void);
LRESULT window_proc(int message);
NTSTATUS open_file(const char *path);
DWORD tick_count(void);
void on_event(HRESULT (*callback)(int code));
