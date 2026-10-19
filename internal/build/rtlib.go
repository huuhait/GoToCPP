package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The runtime library (Phase 06 step 6.8). The runtime's sources are compiled once for a
// (compiler, flags, sanitizer, target) key, archived into one static library, and kept in the
// cache next to the runtime's files: <runtime folder>/lib/<key>/libgocpprt.a. Every program with
// the same key links that file; the object cache, the stamp (stamp.go) and the link trace see one
// input where there were dozens of objects, and the linker takes only the members that a program
// reaches. The key is runtimeKey of the flags, the sources and the environment that the compile
// reads, plus the identity of the archiver: another compiler, other flags (-asan, -nochecks,
// -lto, flags of the environment), another operating system or a changed archiver program
// makes another library, and a change of a runtime source makes another runtime folder
// (runtimeName hashes the embedded files), so a stale library is never found.
//
// Why a library of objects is as good as the objects themselves: a member is taken only for a
// symbol that is wanted, so an object that has to run without being referenced (a global with a
// constructor) would be dropped. The runtime has none that matters: its state is
// constant-initialized or lazy, and the two objects that have initializers (math.o, timer.o)
// initialize data that only their own symbols reach, so taking the object for any of them runs
// the initializer (a test, TestRuntimeObjectsHaveNoStaticInit, looks at the objects); the
// programs' own initialization is called by gp_init_all.
//
// Building and sharing:
//   - one build of a key at a time: the process that gets the lock file lib/<key>.lock compiles;
//     the others wait for it and use its library (a lock that can't be had, or a wait of ten
//     minutes, makes a process build it itself: the result is renamed into place, so it is
//     only duplicate work);
//   - the compiles and the archive are made in a temporary folder, and the finished library is
//     renamed to its name: a build that is killed leaves a temporary folder (removeStale) and
//     no library; a library that exists is whole;
//   - a library is checked cheaply before it is used: a regular file with the archive magic.
//
// Without a cache folder, or when there is no archiver (and no LTO), the runtime's objects are
// linked as before (cachedObjects, compileRuntime in the work folder).

// rtLibName is the name of the runtime library in its folder.
const rtLibName = "libgocpprt.a"

// libLockWait is how long a build waits for another one that makes the same library, and
// libLockPoll how often it looks. Variables, for the tests.
var (
	libLockWait = 10 * time.Minute
	libLockPoll = 20 * time.Millisecond
)

// runtimeLibKey is the name of the folder of the runtime library for flags: runtimeKey with the
// identity of the archiver.
func (b *Builder) runtimeLibKey(srcs, flags []string, a Archiver) string {
	return b.runtimeKey(slices.Concat([]string{"library"}, srcs, a.identity()), flags)
}

// libOK reports whether path is a finished static library: a regular file that starts with the
// magic of an archive, and that has at least one member and whose members (each a 60-byte header and its data, padded to an even
// size) end exactly at the end of the file, so that a copy that was cut short is not taken for a
// library. It reads the headers only.
func libOK(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	var magic [8]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil || string(magic[:]) != "!<arch>\n" {
		return false
	}
	pos, size := int64(8), fi.Size()
	var hdr [60]byte
	for pos < size {
		if _, err := f.ReadAt(hdr[:], pos); err != nil || string(hdr[58:60]) != "`\n" {
			return false
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(hdr[48:58])), 10, 64)
		if err != nil || n < 0 {
			return false
		}
		pos += 60 + n + n%2
	}
	return pos > 8 && pos == size
}

// cachedLib returns the runtime library for flags in dir/lib/<key>, making it first if it is not
// there. dir is the runtime folder.
func (b *Builder) cachedLib(ctx context.Context, dir string, srcs, flags []string, a Archiver) (string, error) {
	root := filepath.Join(dir, "lib")
	key := b.runtimeLibKey(srcs, flags, a)
	final := filepath.Join(root, key, rtLibName)
	if libOK(final) {
		return final, nil
	}
	if err := os.MkdirAll(filepath.Join(root, key), 0o755); err != nil {
		return "", err
	}
	removeStale(root)

	// One process builds the library; the others wait for it.
	lockName := filepath.Join(root, key+".lock")
	deadline := time.Now().Add(libLockWait)
	for {
		lock, busy, err := tryLockFile(lockName)
		if err != nil {
			break // no lock on this file system: build without it
		}
		if !busy {
			defer lock.unlock()
			break
		}
		if libOK(final) {
			return final, nil
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(libLockPoll):
		}
	}
	if libOK(final) { // made by the process that had the lock
		return final, nil
	}

	tmp, err := os.MkdirTemp(root, ".tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	objDir := filepath.Join(tmp, "obj")
	b.trace("mkdir -p " + quoteWord(objDir))
	if err := os.Mkdir(objDir, 0o755); err != nil {
		return "", err
	}
	objs, err := b.compileRuntime(ctx, dir, objDir, srcs, flags)
	if err != nil {
		return "", err
	}
	lib := filepath.Join(tmp, rtLibName)
	if err := b.makeLib(ctx, a, lib, objs); err != nil {
		return "", err
	}
	if !libOK(lib) {
		return "", fmt.Errorf("the archiver %s made no library", a.Path)
	}
	b.trace("mv " + quoteWord(lib) + " " + quoteWord(final))
	if err := os.Rename(lib, final); err != nil && !libOK(final) {
		return "", err
	}
	return final, nil
}
