# Go 1.25.14's tests of os; see convert_std.py. The port is Go's os for Linux over a syscall subset
# (ADR-067): no os.Root, no StartProcess, no os/exec or os/signal, no deadlines, no splice or
# sendfile (no package net); the tests of those are left out with that reason.
FILES = ['all']
TESTDATA = True
NOPROC = (
    'it starts or signals processes (os.StartProcess, os/exec, os/signal), and gocpp has none of '
    'them (os.StartProcess is ENOSYS)'
)
skip_file('copy_test.go', 'it needs package net (sockets, tier 3) and golang.org/x/net/nettest')
skip_file(
    'fifo_test.go',
    'it needs internal/syscall/unix (Fcntl, nonblocking opens), which the syscall subset does not '
    'have',
)
skip_file(
    'readfrom_linux_test.go',
    'it tests splice (zero-copy ReadFrom from sockets and pipes: package net) and copy_file_range '
    'with a pseudo-terminal from internal/testpty, which gocpp does not have',
)
skip_file(
    'readfrom_unix_test.go',
    'it tests the copy_file_range path of File.ReadFrom through copyFileHook of '
    'readfrom_linux_test.go and the hooks of export_linux_test.go, which are left out (package '
    'net, internal/testpty, pidfd)',
)
skip_file(
    'writeto_linux_test.go',
    'it tests the zero-copy WriteTo (splice, sendfile), which os does not have, over net',
)
skip_file('exec_test.go', NOPROC)
skip_file('executable_test.go', NOPROC)
skip_file(
    'pidfd_linux_test.go',
    'it tests process file descriptors (pidfd), and gocpp has no StartProcess',
)
skip_file('root_unix_test.go', 'it tests os.Root, which os does not have')
skip_file(
    'timeout_test.go',
    'it tests read and write deadlines (SetDeadline), which the blocking-mode internal/poll does '
    'not have',
)
skip_file(
    'timeout_unix_test.go',
    'it tests read and write deadlines (SetDeadline), which the blocking-mode internal/poll does '
    'not have',
)
CHDIR_TESTLOG = (
    'it re-runs the test binary with os/exec to read the testlog of a file Chdir, and gocpp has '
    'no os/exec'
)
for name in [
    'testStartProcess',
    'TestStartProcess',
    'TestKillStartProcess',
    'testKillProcess',
    'TestKillFindProcess',
    'TestGetppid',
    'TestFileChdirTestlog',
    'runBinHostname',
    'testWindowsHostname',
    'TestStatStdin',
]:
    drop('os_test.go', name, NOPROC if name != 'TestFileChdirTestlog' else CHDIR_TESTLOG)
drop('pipe_test.go', 'TestStdPipe', NOPROC + ' (it re-runs the test binary and catches SIGPIPE)')
drop(
    'pipe_test.go',
    'TestReadNonblockingFd',
    'it re-runs the test binary with os/exec on a nonblocking pipe, and gocpp has no os/exec',
)
drop(
    'removeall_test.go',
    'TestRemoveAllNoFcntl',
    'it re-runs the test binary under strace with os/exec, and gocpp has no os/exec',
)
skip_file(
    'export_linux_test.go',
    'it exports the splice, net and pidfd internals (pollSplice, getPollFDAndNetwork, '
    'checkPidfdOnce) to the tests that are left out; os does not have them',
)

# os.Root is not built (ADR-067), but the tests of other functions run twice, in the current folder
# and in a Root, through testMaybeRooted (root_test.go), and name *os.Root: the helpers stay, the
# Root tests and the in-Root runs are left out.
ROOT = 'it tests os.Root (openat-based sandbox), which os does not have'
only('root_test.go', ['testMaybeRooted', 'makefs', 'tempDirWithUnixSocket'], ROOT, taken=False)
replace_func(
    'root_test.go',
    'tempDirWithUnixSocket',
    'func tempDirWithUnixSocket(t *testing.T, name string) string {\n\tt.Skip("it needs package '
    'net for a unix socket, which gocpp does not have")\n\treturn ""\n}\n',
    'it makes a unix socket with package net, which gocpp does not have',
)
patch('root_test.go', '''	t.Run("InRoot", func(t *testing.T) {
		t.Chdir(t.TempDir())
		r, err := os.OpenRoot(".")
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		f(t, r)
	})
''', '', 'os.Root is not built: only the NoRoot run of the test remains')
add_file(
    'gocpp_root_test.go',
    '''package os

import (
	"errors"
	"io/fs"
)

// Root stands in for os.Root, which gocpp's os does not have (ADR-067): the tests of other
// functions name *os.Root in the signatures of their helpers, and pass nil (testMaybeRooted runs
// only the case without a Root). OpenRoot fails, so the few tests that open a Root fail.
type Root struct{}

func OpenRoot(name string) (*Root, error) { return nil, errors.ErrUnsupported }

func (r *Root) Close() error                               { return errors.ErrUnsupported }
func (r *Root) Create(name string) (*File, error)          { return nil, errors.ErrUnsupported }
func (r *Root) Open(name string) (*File, error)            { return nil, errors.ErrUnsupported }
func (r *Root) OpenFile(name string, flag int, perm FileMode) (*File, error) {
	return nil, errors.ErrUnsupported
}
func (r *Root) Link(oldname, newname string) error         { return errors.ErrUnsupported }
func (r *Root) Symlink(oldname, newname string) error      { return errors.ErrUnsupported }
func (r *Root) Stat(name string) (FileInfo, error)         { return nil, errors.ErrUnsupported }
func (r *Root) Lstat(name string) (FileInfo, error)        { return nil, errors.ErrUnsupported }
func (r *Root) Readlink(name string) (string, error)       { return "", errors.ErrUnsupported }
func (r *Root) FS() fs.FS                                  { return nil }
func (r *Root) Remove(name string) error                   { return errors.ErrUnsupported }
func (r *Root) MkdirAll(name string, perm FileMode) error  { return errors.ErrUnsupported }
''',
)

# Several tests list their own folder (the cwd of gocpp test is the folder of the test files: .gp
# files, no sub-folder exec) and look for the Go source files of package os by name.
DOT_REASON = (
    'the folder of the tests holds the .gp files of the tests and testdata, not the .go files of '
    'package os'
)
patch('os_test.go', '''var dot = []string{
	"dir_unix.go",
	"env.go",
	"error.go",
	"file.go",
	"os_test.go",
	"types.go",
	"stat_darwin.go",
	"stat_linux.go",
}''', '''var dot = []string{
	"env_test.gp",
	"error_test.gp",
	"file_test.gp",
	"os_test.gp",
	"path_test.gp",
	"read_test.gp",
	"stat_test.gp",
	"testdata",
}''', DOT_REASON)
patch('read_test.go', 'filename = "read_test.go"', 'filename = "read_test.gp"', DOT_REASON)
patch('read_test.go', 'dir.Name() == "read_test.go"', 'dir.Name() == "read_test.gp"', DOT_REASON)
patch(
    'read_test.go',
    'dir.IsDir() && dir.Name() == "exec"',
    'dir.IsDir() && dir.Name() == "testdata"',
    DOT_REASON,
)
patch('read_test.go', 'exec directory not found', 'testdata directory not found', DOT_REASON)
patch('read_test.go', 'read_test.go file not found', 'read_test.gp file not found', DOT_REASON)

# Tests that run once with a Root and once without: only the run without a Root stays.
patch(
    'os_test.go',
    '\tt.Run("InRoot", func(t *testing.T) { testOpenError(t, dir, true) })\n',
    '',
    'os.Root is not built: only the NoRoot run of the test remains',
)
patch('os_test.go', '''	t.Run("RootOpenFile", func(t *testing.T) {
		testOpenFileKeepsPermissions(t, func(name string, flag int, perm FileMode) (*File, error) {
			dir, file := filepath.Split(name)
			r, err := OpenRoot(dir)
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return r.OpenFile(file, flag, perm)
		})
	})
''', '', 'os.Root is not built: only the OpenFile run of the test remains')
drop('os_test.go', 'TestRootDirFS', ROOT)
drop(
    'os_test.go',
    'TestLargeWriteToConsole',
    'Go skips it unless run with -large_write (it floods the console), so it does not run under go '
    'test either',
)
drop(
    'path_test.go',
    'TestMkdirAllAtSlash',
    'Go skips it outside of the Go builders (it creates directories in the root of the file '
    'system), so it does not run under go test either',
)
drop('stat_test.go', 'TestStatConsole', 'it runs on Windows only (Go skips it elsewhere)')
expect(
    'TestWriteStringAlloc',
    'divergence',
    'it counts allocations with testing.AllocsPerRun (2 per File.WriteString: the slice made from '
    'the string and the write call), and gocpp allocates differently than Go (it counts C++ '
    'operator new); counting rule of the Phase 09 README, 2.8',
)

# exec_unix_test.go is taken: two of its tests need no process (FindProcess, Wait and Signal on a
# pid that does not exist).
for name in ['TestErrProcessDone', 'TestUNIXProcessAlive']:
    drop('exec_unix_test.go', name, NOPROC)
# TestHostname checks Hostname() against /bin/hostname, run with os/exec: the first half stays, the
# comparison is cut.
patch(
    'os_test.go',
    '\t// There is no other way to fetch hostname on windows, but via winapi.\n\t// On Plan 9 it '
    'can be taken from #c/sysname as Hostname() does.\n\tswitch runtime.GOOS {\n\tcase "android", '
    '"plan9":\n\t\t// No /bin/hostname to verify against.\n\t\treturn\n\tcase "windows":\n\t\t'
    'testWindowsHostname(t, hostname)\n\t\treturn\n\t}\n\n\ttestenv.MustHaveExec(t)\n\n\t// Check '
    'internal Hostname() against the output of /bin/hostname.\n\t// Allow that the internal '
    'Hostname returns a Fully Qualified Domain Name\n\t// and the /bin/hostname only returns the '
    'first component\n\twant := runBinHostname(t)\n\tif hostname != want {\n\t\thost, _, ok := '
    'strings.Cut(hostname, ".")\n\t\tif !ok || host != want {\n\t\t\tt.Errorf("Hostname() = %q, '
    'want %q", hostname, want)\n\t\t}\n\t}\n',
    '\t// (Go compares the name with the output of /bin/hostname, run with os/exec, which gocpp '
    'does not have.)\n',
    'the second half of TestHostname starts a process; the first half (a name that is not empty '
    'and has no zero byte) stays',
)
