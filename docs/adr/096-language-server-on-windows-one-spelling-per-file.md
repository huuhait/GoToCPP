# ADR-096: The language server on Windows: one spelling per file, URIs of every client, a test on the real PC

Status: accepted (2026-10-20). Carries out [Phase 14 §2.5](../../plans/2026-10-13-gocpp/phase-14-windows/README.md) (step 14.5) and builds on
[ADR-015](015-language-server.md) (the server), [ADR-052](052-lsp-speed-work.md) (its speed work) and
[ADR-081](081-windows-host-build-driver.md) (the build driver on Windows: the environment the server inherits, short names for the compilers).

## Context

`gocpp lsp` was written and tested on Linux. It is gopls-shaped: a snapshot is keyed by the names of its files, the open documents by
their URIs, the cache of checked packages by folders, the diagnostics by the files they were published for. On Windows the same file has
many names: `c:\users\me\proj\a.gp`, `C:\Users\Me\Proj\a.gp`, `C:\Users\ME\Proj\A.GP` (the file system ignores case), `C:\Users\ME~1\...` (8.3), `\\localhost\c$\...`;
VS Code writes `file:///c%3A/Users/Me/Proj/a.gp` (a lower-case drive letter, the colon escaped), other clients `file://C:/...` or `file:////srv/share/...`;
and an editor spells a folder as the user typed it when the folder was opened, while a folder listing has the case the disk stores. With
gopls's rule (a URI is the key) such a pair makes two packages, two documents, diagnostics that are published for a name the editor
does not know, and a rename that edits a file that is already open under another name. gopls has a file identity (`os.SameFile`) for the same
reason; the old `fs.go` of the cache had a TODO for this phase.

## Decisions

1. **A path package with an injectable flavor** (`internal/lsp/fspath`). A `Flavor` is `{Windows, FoldCase}`: Posix (slashes, case-sensitive; every function is
   the identity or `path`), Windows (drive letters, UNC, both slashes, case-insensitive) and CaseFold (slashes, case-insensitive: a Linux folder on a case-insensitive
   file system, and the tests). `Use` replaces the current flavor and `SetLister` the function that lists a folder, so the logic of a
   Windows server runs in unit tests on Linux; `protocol.DocumentURI` and the cache call the current flavor, never `filepath`'s own rules, for the
   names of the editor. The Windows rules of `Clean`, `Dir`, `Base`, `Join` and `IsAbs` are written out (`\\?\` prefixes, UNC `\\host\share`, `C:` and `C:\`)
   and tested against the results of `path/filepath` on Windows.
2. **`Canon` is the one spelling of a file** and every name from the editor goes through it before it is a key: the clean name, the drive letter in upper case,
   8.3 short names written out (`GetLongPathName`, with the longest existing folder expanded: a file that is not saved yet keeps its name), and every part that exists on disk in the case the disk
   stores (found by listing the folder, one listing per folder, remembered). A part that does not exist, a folder that cannot be listed, and a folder that holds two
   names that differ by case only (a WSL share seen from Windows: an exact spelling still finds its entry) stay as given. The answers are remembered; a listing that does not have the name is made again once, so
   a file that was created is found; a name that was asked while the file was missing is remembered as given until `Invalidate`, which the server calls when
   `didChangeWatchedFiles` reports a created or deleted file or folder (a rename is both). `DocumentURI.Path` is `Canon` of the URI's name, `DocumentURI.Key` the URI of that path: the
   key of the open documents (`Session`, `Snapshot.Overlay`), of the requests in flight and of the warnings. The cache's file, folder, module and package handles and the
   recorded reads are made canonical where they enter (`state.file`, `dir`, `modFile`, `handle`, `do`, the replay), and the test of "is this path under a changed
   folder" and the merge of the overlay names into a folder listing compare by the file system's rule.
3. **The editor's own spelling is kept for the document it opened.** An `Overlay` keeps the URI the editor sent (the diagnostics were always published under it), and the
   answers that name a file (definition, references, rename edits, workspace symbols, document links) use it when that file is an open document
   (`locationURI`); every other file has the URI of its canonical path. VS Code keys its markers and models by `Uri.toString()`, which is case-sensitive but for the drive letter, so an answer under the
   disk's spelling for a document opened under the typed one would land on nothing. The server's answers have an upper-case drive letter (gopls does the same).
4. **URIs.** `file:///c:/x`, `file:///C%3A/x`, `file:///c%3a/x`, `file://C:/x` (two slashes: golang/go#39789), `file://localhost/C:/x`, `file:///C:` and `file:///C%3A` are the same
   file; an authority is a UNC host (`file://srv/share/x`, `file:////srv/share/x`, `file://wsl%24/Ubuntu/x`), but `localhost` with a drive is the machine's own;
   percent-escapes, `+`, `#` and non-ASCII names round-trip. A URI made from a name is escaped the way `URIFromPath` always did (`net/url`).
5. **A byte order mark is not text.** A file read from disk loses a leading UTF-8 BOM (`state.file`): the editor shows the file without it (VS Code removes it and writes it back), and the columns of
   line 0 that the server answers with must not count it.
6. **Positions and line ends were right already** (`Mapper`: the `\r` of a `\r\n` belongs to the line end; columns are UTF-16; a position past the end of its line is the
   end of the line) and the formatter keeps `\r\n`; the PC test drives them with CRLF sources and an emoji before the error.
7. **File watching is the client's.** The server registers `**/*.gp`, `**/gocpp.mod` and header globs (`workspace/didChangeWatchedFiles`) and has no watcher of its own
   (`ReadDirectoryChangesW` would watch the folders of a workspace twice, and gopls does not do it either); the events may name a file in any spelling, and a deleted folder
   is handled by its prefix in any case. **An editor that does not watch files (Helix, Zed, eglot) does not show a change on disk of a file that is not open until a watcher event
   or a restart**: the cached handles of files that are not open are read again only when an event names them (an edit or a `didSave` of another file does not re-read them). This is
   the behaviour of the server before this step and gopls's design; there is no bound on the staleness. (An earlier text of this ADR said that the next edit or save of an open file shows
   the change: that was wrong.) The one exception is a file that could not be read (decision 11).
8. **The external tools are the build's.** The server inherits the environment that `gocpp.exe` normalizes at start (14.2: `TEMP`, `LOCALAPPDATA` replaced by names the ANSI-code-page compilers can read) and
   finds the compiler with the same rules; the namespace probe's `#include` lines use `winpath.IncludeSpelling`. The server starts no program of the user, so `programEnv`/`originalEnv` do not matter to it.
   The PC test runs the probe in a folder with spaces and Japanese characters.
9. **The VS Code extension** takes a quoted `gocpp.path`, a folder (it looks for `gocpp.exe` in it), UNC names, and quoted entries of `PATH` (all tested with a fake file system for `win32`). `.exe` and `.com` were
   already tried without `PATHEXT`; a `.cmd` is still refused (Node.js, CVE-2024-27980). No extension change was needed for activation, settings or the way the server is started (no shell).
10. **The benchmark harness runs on Windows** (`internal/lsp/bench`: the working set through `GetProcessMemoryInfo`, `taskkill /T` for the server's tree, the machine from the environment and `GlobalMemoryStatusEx`),
    and `TestWindowsLSPBench` (opt-in: `GOCPP_WINDOWS_LSPBENCH=1`) cross-builds `tools/lspbench`, runs it on the PC with the same scenarios and gate, and fetches the report.

11. **A file that cannot be read is read again in the background.** A read that fails with a sharing or lock violation is repeated for about 0.6 s (5, 10, 20, ... 320 ms; out of descriptors:
    5, 20, 80 ms). If it still fails, the snapshot has no such file and no event would come when the other program lets go (a locked file left the package stuck, silently, until a second event
    or a keystroke). Now `Config.OnReadFailure` tells the server, which logs it and sends `window/logMessage` once for the file, reads it again with waits of 0.5 s doubling to 8 s until it can or it is
    gone, then makes a new snapshot (`Session.Invalidate`) and diagnoses the open packages. The retries end with the server.
12. **`Canon` is bounded.** A folder is listed by one call however many names ask for it (single flight); a folder that lacks a name is not listed again for a second (a created file is
    announced by an event, which forgets everything); a listing that takes more than 5 s (a share that does not answer) is given up, remembered as empty for 30 s, and the names stay as given; `Invalidate` takes no
    time (a generation number: what was remembered under an older one is old, and an answer worked out while `Invalidate` came is not kept). `DocumentURI.Path` and `Key` are remembered the same way
    (the parse and the `Canon` cost about 10 µs and 17 allocations each). A short name that the system gives back unchanged does not loop. A URI with a user name (`file://user@srv/x`) is refused.

Known limits: a file that is deleted while its document is open and is opened again under a different spelling is two documents (the disk cannot say they are one); a case-only rename that the
client reports with a `Changed` event only keeps the old case in the URIs of the answers until the next created/deleted event (cosmetic); a folder of 100,000 entries costs 130 ms and 8 MB the first time one of its files is asked for.

## The test on the PC

`tests/windows_lsp_test.go` starts `gocpp.exe lsp` over ssh (the process's standard streams are the ssh session's: `winssh.Remote.Start`) and drives it with the benchmark client. The workspace is in a folder named
`Wörk Dir 日本語`, made by `tests/winlspctl` (a helper that uses Go's wide file functions: cmd.exe cannot carry the names), with CRLF sources, a file called `grüß.gp`, a file with a BOM, VS Code's URIs
(`file:///c%3A/...`), and spellings that are not the disk's. It checks diagnostics (UTF-16 columns after an emoji on a CRLF line), hover, definition, references, rename and workspace symbols across
packages, a document opened under one spelling and changed under another, a file changed, created, renamed and deleted on disk with the editor's events in several spellings, the BOM, a short name (`ADMINI~1`),
a UNC name (`\\localhost\c$`), a path of more than 300 characters, a C++ header import in that folder (the probe), formatting that keeps CRLF, and shutdown. The unit tests of `fspath`, `protocol`, `cache` and `cache/filecache` and a part of
`server`'s run on the PC too (`TestWindowsNativeUnits`; the tests that need the repository's files or that assume Linux's clock are skipped there with a reason). On Linux the same logic is tested with the Windows flavor and a
fake file system (`fspath`, `protocol`), and `server`'s case-folding tests run over a real folder with the CaseFold flavor.

## Consequences

Good: a file has one identity in the server on Windows whatever the editor typed; the PC test found the whole path (URIs, case, short names, UNC, long paths, BOM, CRLF, watcher events, the probe) working with the code of
this step and (for the case-different spelling) failing without it. The cost of `Canon` is one memo lookup per name (and a listing of a folder the first time); on Linux the flavor is Posix and every function
returns at the first test. Not done, and said where the user can see it ([the guide](../guides/editors.md#windows)): two names of a folder through a symbolic link or junction (two packages, as in gopls),
`\\localhost\c$` as another name of `C:\`, trailing dots and spaces, reserved device names (`aux.gp`), a folder whose case-sensitivity was switched on (names that differ in case are one key there),
a case-different import path of a package (`example.com/m/Lib` for the folder `lib`), the `\\?\` prefix in an URI. The clock of a Windows process ticks every half millisecond or so: a benchmark figure under a millisecond is a step of that size
(the budgets are 20 ms and more).
