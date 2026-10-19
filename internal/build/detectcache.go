package build

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// The compiler detection on disk (Phase 06 step 6.5). Detecting a compiler runs it twice (--version
// and -dM -E: with a Nix wrapper that is five processes, 60 to 100 ms), and every gocpp command
// does it, so a build that has nothing to do spent a fifth of its time on it. detect's answer is
// a function of the compiler's program file, so it is kept in <cache>/cxxid/<hash of the command>,
// with the file as it was (real path, size, modification time: the fileStamp that lookPath
// already compares for its in-process cache), and used while the file is the same. The
// limits are those of the in-process cache: a compiler behind a launcher (ccache g++) is known by the
// launcher's file; a wrapper that picks another compiler from the environment is known by
// its own file (the environment is part of the keys of the caches that use the answer).
//
// The folder is set by the command (SetDetectCache): the library never writes the user's cache by
// itself, so tests of the library that call LookCXX leave it alone.

var detectCacheDir atomic.Pointer[string]

// SetDetectCache sets the cache folder (such as ~/.cache/gocpp) in which the answers of the compiler
// detection are kept between runs of gocpp, or "" for none (the default). GOCPP_NOOBJCACHE turns it
// off, like the object cache: a test that wants every compiler run turns both off.
func SetDetectCache(cache string) {
	if cache == "" {
		detectCacheDir.Store(nil)
		return
	}
	detectCacheDir.Store(&cache)
}

// detectRecord is the file of one compiler command.
type detectRecord struct {
	Key     string   `json:"key"`
	Stamp   fileRec  `json:"stamp"`
	Version string   `json:"version"`
	Family  Family   `json:"family"`
	Major   int      `json:"major"`
	Time    int64    `json:"time"`
	Args    []string `json:"args,omitempty"`
}

// fileRec is a fileStamp as JSON.
type fileRec struct {
	Real string `json:"real"`
	Size int64  `json:"size"`
	Mod  int64  `json:"mod"`
	Sum  string `json:"sum,omitempty"` // the content hash of the program file (fileContentID)
}

func recOfStamp(stamp fileStamp) fileRec {
	return fileRec{stamp.real, stamp.size, stamp.modTime, fileContentID(stamp.real)}
}

func detectFile(key string) string {
	p := detectCacheDir.Load()
	if p == nil || objCacheOff() {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(*p, "cxxid", hex.EncodeToString(sum[:16])+".json")
}

// loadDetected returns what an earlier run of gocpp found out about the compiler of key, if its
// program file is still stamp.
func loadDetected(key string, stamp fileStamp, c CXX) (CXX, bool) {
	file := detectFile(key)
	if file == "" {
		return CXX{}, false
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return CXX{}, false
	}
	var r detectRecord
	if json.Unmarshal(data, &r) != nil || r.Key != key || r.Stamp != recOfStamp(stamp) ||
		r.Version == "" || (r.Family != Clang && r.Family != GCC) {
		return CXX{}, false
	}
	c.Version, c.Family, c.Major = r.Version, r.Family, r.Major
	return c, true
}

// saveDetected keeps the answer c of the detection of the compiler of key. A failure is ignored.
func saveDetected(key string, stamp fileStamp, c CXX) {
	file := detectFile(key)
	if file == "" {
		return
	}
	data, err := json.Marshal(detectRecord{Key: key, Stamp: recOfStamp(stamp),
		Version: c.Version, Family: c.Family, Major: c.Major, Time: time.Now().Unix()})
	if err != nil || os.MkdirAll(filepath.Dir(file), 0o700) != nil {
		return
	}
	writeFileAtomic(file, data)
}

// writeFileAtomic writes data to name through a temporary file in the same folder and a rename,
// so that a reader sees the old file or the whole new one, never half of it. It reports whether
// it worked.
func writeFileAtomic(name string, data []byte) bool {
	tmp, err := os.CreateTemp(filepath.Dir(name), ".tmp-")
	if err != nil {
		return false
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil || os.Rename(tmp.Name(), name) != nil {
		os.Remove(tmp.Name())
		return false
	}
	return true
}

// contentIDs remembers the content hash of program files for their stat (fileContentID).
var contentIDs sync.Map // string (path, size, times, inode) -> string

// fileContentID returns the SHA-256 of the file real (a compiler's program file), or "unreadable". The hash is
// kept for the file's size, times, inode and change time, in this process and, when the cache folder
// is set (SetDetectCache), on disk, so a large compiler is read once: a file whose times are too new
// to trust (changed within objRacyWindow, or no change time) is hashed again every time. The
// change time can't be set back, so a program file that was replaced and given its old size and
// modification time gets a new hash.
func fileContentID(real string) string {
	fi, err := os.Stat(real)
	if err != nil || !fi.Mode().IsRegular() {
		return "unreadable"
	}
	sys := sysStatOf(fi)
	trusted := sys.ctime != 0 && time.Since(fi.ModTime()) > objRacyWindow && time.Since(time.Unix(0, sys.ctime)) > objRacyWindow
	key := fmt.Sprintf("%s\x00%d\x00%d\x00%d\x00%d", real, fi.Size(), fi.ModTime().UnixNano(), sys.ino, sys.ctime)
	if trusted {
		if v, ok := contentIDs.Load(key); ok {
			return v.(string)
		}
		if file := detectFile("content\x00" + key); file != "" {
			if data, err := os.ReadFile(file); err == nil && len(data) == 64 {
				contentIDs.Store(key, string(data))
				return string(data)
			}
		}
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return "unreadable"
	}
	sum := hashHex(data)
	if trusted {
		contentIDs.Store(key, sum)
		if file := detectFile("content\x00" + key); file != "" && os.MkdirAll(filepath.Dir(file), 0o700) == nil {
			writeFileAtomic(file, []byte(sum))
		}
	}
	return sum
}
