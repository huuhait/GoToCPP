# internal/syscall/windows

Step 14.3 (Phase 14, ADR-076). A subset of go1.25.14's `src/internal/syscall/windows` (BSD-3-Clause, see `../../../LICENSE`), the system calls and
types that package os and `internal/poll` need on Windows; the files are Go's with these cuts:

- `syscall_windows.gp`: `CanUseLongPaths` (false: a gocpp program has no long-path-aware manifest), `UTF16PtrToString`, the `ERROR_*` numbers,
  `SecurityAttributes`, `FILE_BASIC_INFO`, `GetComputerNameEx`, `MoveFileEx`, `Rename`, `GetModuleFileName`, `SetFileInformationByHandle`, `GetTempPath2`
  (optional: the package falls back to `GetTempPath` when `kernel32.dll` lacks it), `GetFinalPathNameByHandle`, `FinalPath`, `FILE_ID_BOTH_DIR_INFO`,
  `FILE_FULL_DIR_INFO`, `GetVolumeInformationByHandle`, `NTStatus`, `RtlIsDosDeviceName_U`. Not taken (56 of 65 `//sys` functions): `GetAdaptersAddresses`, `VirtualQuery`, `Module32First/Next`, `WSASocket`, `WSADuplicateSocket`, `WSAGetOverlappedResult`, `LockFileEx`, `UnlockFileEx`, `GetACP`, `GetConsoleCP`, `MultiByteToWideChar`, `GetCurrentThread`, `NetShareAdd/Del`, `CreateEnvironmentBlock`, `DestroyEnvironmentBlock`, `CreateEvent`, `ProcessPrng`, `GetVolumeNameForVolumeMountPoint`, `RtlLookupFunctionEntry`, `RtlVirtualUnwind`, the service functions, `GetModuleHandle`, `CreateIoCompletionPort`, `GetOverlappedResult`, `CreateNamedPipe`, `ReOpenFile`, the `Nt*` functions, `SetEntriesInAcl`, `SetNamedSecurityInfo`, `rtlGetVersion`, `GetProcessMemoryInfo`, the token and impersonation functions, `GetProfilesDirectory`, `NetUser*`, the SID helpers; that is, the Winsock and network adapter code, the
  version checks (`version_windows.go`), file locking, services, the NT native file API (`NtCreateFile` and `at_windows.go`, the `Root`
  support), security descriptors, `IsNonblock`, process snapshots.
- `types_windows.gp`: the file access and attribute constants; `reparse_windows.gp`: the reparse buffers (`Path` uses `unsafe.Slice` where Go
  converts to a big array pointer); `symlink_windows.gp`: the file information classes and `GetFileInformationByHandleEx`;
  `zsyscall_windows.gp`: `mksyscall`'s output for the kept `//sys` lines.
- `sysdll/sysdll.gp` is Go's file (the list of DLLs loaded only from System32).

The package has files for Windows only (`stdlib.WindowsOnly`); only the Windows files of `os`, `internal/poll` and `internal/filepathlite` import it.
