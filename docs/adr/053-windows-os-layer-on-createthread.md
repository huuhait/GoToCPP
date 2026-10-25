# ADR-053: The Windows OS layer starts threads with CreateThread and reports faults itself

Status: accepted (2026-10-18). Refines [ADR-021](021-platforms-linux-windows.md) (its table says
"winpthreads" for threads and "`CaptureStackBackTrace` + DWARF" for traces) and
[ADR-007](007-goroutines-os-threads.md) (every goroutine is an OS thread).

## Context

[ADR-021](021-platforms-linux-windows.md) and [Phase 14 §2.1](../../plans/2026-10-13-gocpp/phase-14-windows/README.md)
planned winpthreads for goroutine threads, because the code would then be the Linux code. Building the
layer for step 14.1 on the dev shell's MinGW-w64 showed:

- **The C++ library's thread model is not winpthreads.** The dev shell's `x86_64-w64-mingw32-g++`
  is configured with `--enable-threads=mcf` (thread model `mcf`, `__USING_MCFGTHREAD__`), so
  `std::thread`, `std::mutex` and `std::condition_variable` sit on mcfgthread, and the C runtime is
  the old `msvcrt` flavor (`__MSVCRT__`). The toolchain does have the winpthreads headers and
  library, but a program that starts goroutine threads with `pthread_create` while the C++ library
  uses mcfgthread has two thread libraries in one process (and llvm-mingw, MSYS2 and the real
  Windows toolchains of Phase 14 differ among themselves in this).
- **A goroutine needs three things that `std::thread` does not give:** a stack of a size the runtime
  chooses (8 MiB, like Linux, or 256 KiB for tests), the stack's bounds (to tell a stack overflow from
  other faults), and a thread that has no handle to wait for (goroutines are detached).
- **A stack overflow is an exception on Windows, not a signal.** Its handler runs on the faulting
  stack, in what is left after the guard page, and `thread_local` of the MinGW compilers is emulated
  (a call that may allocate): the handler cannot use it.
- **The C++ runtime has no forced unwind** (`pthread_exit` unwinds the stack on glibc; nothing like it
  exists on Windows).

## Decision

1. **Threads are `CreateThread`** (`runtime/src/os/windows/thread.cpp`), with
   `STACK_SIZE_PARAM_IS_A_RESERVATION`: the stack size is address space, and pages are committed as the
   thread uses them, as on Linux (a goroutine's 8 MiB costs what it touches). A smaller request is
   raised to 256 KiB (the guarantee below must fit); a request above 1 TiB is `ENOMEM`; Windows' own
   error code is returned for any other failure (`ERROR_NOT_ENOUGH_MEMORY` becomes `EAGAIN`, which the
   runtime's message looks for). The handle is closed at once (detached). The portable runtime keeps
   using `std::mutex`, `std::condition_variable` and `std::atomic::wait` for blocking, which work with
   whatever thread library the C++ library has (mcfgthread here, winpthreads elsewhere: nothing in the
   runtime depends on it). Programs are linked `-static` (Phase 14 §2.2).
2. **The thread's slot and its "has bounds" flag are TLS indexes** (`TlsAlloc`), not `thread_local`
   variables, so that the exception handler can read the flag. `stack_bounds` is
   `GetCurrentThreadStackLimits`, given only to the main thread and the threads of `thread_start`, as
   on Linux (a `std::thread` or a thread of a C library has none, and its overflow is not reported by
   gocpp).
3. **The stack overflow is a vectored exception handler** (`AddVectoredExceptionHandler`, installed
   last, so the vectored handlers that were there before see the exception first) for `EXCEPTION_STACK_OVERFLOW`, with
   `SetThreadStackGuarantee(64 KiB)` on every thread that has bounds, so the handler and the report
   have stack. The hook is the same `gp::os::stack_overflow_hook` as on Linux: Go's `fatal error: stack
   overflow` report and exit code 2.
4. **Other faults become Go's report too** (new on Windows; Linux leaves a plain crash). An
   `SetUnhandledExceptionFilter` filter takes the exceptions of the CPU that nothing handled (access
   violation, integer division by zero, illegal instruction, ...) and calls a new
   `gp::os::set_fault_hook` hook on the faulting thread's stack (it may allocate and read the program's
   file, unlike a signal handler). The runtime's hook prints what Go on Windows prints: `panic:
   runtime error: invalid memory address or nil pointer dereference` and `[signal 0xc0000005 code=0x0
   addr=0x0 pc=0x...]` (an address below 4096; `unexpected fault address` otherwise; `integer divide
   by zero`; `Exception 0x... ` for the rest), the goroutine's trace, and exit code 2. The trace is found
   by unwinding the exception's own context (`RtlVirtualUnwind`), so it shows the code that crashed. A C++
   exception and a fault that a `__try`/`__except` block takes are never seen by the filter. A hook that
   returns leaves the fault to Windows (the crash). Limits, by construction: the stack overflow handler
   is a *vectored* handler, which runs before frame-based (`__try`) handlers, so a program's
   `__try/__except` cannot catch a stack overflow, and `signal(SIGSEGV)` of the C runtime (built on the
   top-level filter, which the fault filter replaces) catches neither a stack overflow nor a CPU fault
   that no `__try` took: Go's report comes first, exit code 2.
5. **Output is `WriteFile`; on a console it is `WriteConsoleW`** with UTF-8 to UTF-16 (a character cut by
   two writes waits for the rest), with the UTF-8 rules of Go's console writer (`utf8.cpp`: a byte
   that starts nothing is one U+FFFD at once, a sequence that breaks is one U+FFFD for each byte
   `DecodeRune` rejects), and no allocation, so the stack overflow hook can write through it. The
   C runtime's text mode is switched off for stdin, stdout and stderr (`\n` stays `\n`). **The
   console's code page is not changed** (as Go does not): gocpp's own output goes through
   `WriteConsoleW` and needs no code page, and a switch to 65001 that is undone only at a normal
   exit would leave the user's console changed after Ctrl-C, a closed window, `taskkill`, `abort()` or
   a crash. C code that prints with `printf` through a console shows the active code page, as with
   cgo; a documented limitation (the real-Windows checklist of step 14.7 tests it under code pages 437
   and 850).
6. **Arguments and environment are UTF-16 converted to UTF-8.** The command line is split by Go's rules
   (`os/exec_windows.go`: `""` inside quotes is a quote, the program name as any argument), in plain C++
   (`cmdline.cpp`) that is tested against a copy of Go's function on Linux too; `argv` is never used.
   The environment keeps the `=C:=C:\dir` entries, as Go does.
7. **`exit` is `_exit` after flushing C stdio** (no `atexit`, no static destructors: Go's `os.Exit`);
   `exit_now` is `TerminateProcess`, usable from a handler.
8. **The stack trace** is `RtlCaptureStackBackTrace`; the program's PE file gives the COFF symbol table
   and the DWARF line sections that MinGW writes into it (a program linked without `-g` or with `-s`
   has no lines), read by the same code as the ELF's.
9. **x86-64 only.** The fault code reads `CONTEXT.Rip` and `Rsp` and unwinds with the x64 tables;
   `windows.hpp` has an `#error` for any other architecture (32-bit x86 and ARM64 Windows are not
   targets).
10. **The process-level functions of runtime/os** (steps 8.9 and library mode): no control groups
    (`cpu_limit` is 0, and the parser `cgroup_cpu_limit` returns 0 on Windows), `mem_info` from
    `K32GetProcessMemoryInfo` (private bytes and working set: an estimate, since the Windows heap does
    not say what is live), `trim_memory` does nothing; `process_arguments` is the wide command line;
    `tls_on_thread_exit` is a fiber-local-storage value with a callback (Windows calls it when a thread
    ends, also for a thread that C or a library started; `tls_on_thread_exit_stop` frees the index so
    that no callback of a library is left behind when it is unloaded). Unlike Linux, the callback comes
    *before* the C++ thread-local destructors (a real PC showed it: FLS callbacks run at the start of the
    thread's end, the destructors at `DLL_THREAD_DETACH`), so os.hpp promises no order there; `library_ref` and `library_unref`
    are `GetModuleHandleEx` (with its reference count) and `FreeLibrary`.
11. **The main thread's stack is 8 MiB**, like Linux's, by a `-stack` linker directive in `os.cpp` (the
   driver passes `-Wl,--stack,8388608` too; the directive is for GNU ld only, lld ignores
   directives).

## Consequences

- No `pthread_exit` equivalent: `gp::os::exit_thread` ends the thread without unwinding (only a test
  calls it).
- A thread without bounds (a `std::thread`) that overflows its stack is the OS's problem: real Windows
  ends the process with `0xc00000fd`; Wine ends only the thread.
- On Windows, a frame inside the stack guarantee (the last 64 KiB and a page) is a stack overflow, not an
  ordinary fault (the real PC showed it in `wild-near-end-thread`, whose "near the end" is therefore
  100 KiB there).
- The first real use of UCRT (Phase 14's clang or llvm-mingw toolchains) needs no change: nothing here
  names the C runtime, except `_setmode`, `_exit`, `_get_osfhandle`, which both flavors have.
- The tests (`runtime/windows_test.go`, `runtime/winremote_test.go`, `runtime/test/winos_test.cpp`)
  cross-compile with the dev shell's MinGW (thread model mcf, `-static`) and run the runtime's whole C++
  suite on Windows: on a real PC over ssh when `GOCPP_WINDOWS_SSH=user@host` is set (the variable is the only
  place that names the machine; the programs go to `%USERPROFILE%\gocpp-wintest\<run id>\`, which is deleted
  at the end; nothing else on the PC is touched), otherwise under Wine in an isolated prefix. Wine is not
  Windows (it cannot dispatch a stack overflow at its own guard page on a CPU with AVX-512: Wine 11.0 commits the whole stack and keeps one guard page, so the 3392-byte exception frame does not fit, and the thread layer puts a guard page of its own 128 KiB higher under Wine, where Wine raises the overflow with room for the frame; and it has no console). If the Wine test binary is killed, the Wine helpers (`winedevice.exe` and others) stay alive; `wineserver -k` under its `WINEPREFIX` ends them: the console
  path (`WriteConsoleW`) and the real exit codes of a crash are left to the real-Windows checklist of step
  14.7.
