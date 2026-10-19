package build

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// The up-to-date check (Phase 06 steps 6.5 and 6.6, ADR-036). A build that finds nothing to do
// should cost milliseconds, but the work before the first compile (loading and type-checking all
// the packages, writing the C++ of all of them, asking the object cache about every unit, linking)
// takes a third of a second for four packages and grows with the program. A Stamp is the record of
// one successful `gocpp build`, from which the next one can tell that the program it made is still
// what the sources make, without doing any of that:
//
//   - the question that was asked: Key is a hash of the patterns, the flags, the output file, the
//     folder, the target, gocpp itself and the environment that the build reads (StampEnv);
//   - what the loader looked at (Trace): every file it read (with a hash), every folder it listed
//     (with a hash of the names) and every file it asked about, as FSRecorder saw them. The loader
//     reads the files only through a FileSystem, so this is all of its input: a .gp file that
//     changed, a file that was added to a package or removed from it, a gocpp.mod or a vendor/ that
//     appeared, a module that replaced another are all changes of the trace;
//   - what the C++ compiler read, for every compile of the build (Units): the source, and the set
//     of headers of the object cache's entry (the dependency file of -MD: system headers too, and
//     the files whose appearance would hide one), as they were;
//   - the answers of pkg-config, which are run again (PkgConfig);
//   - the libraries and the linker that the link used (Link): `-lx` is resolved the way the linker does,
//     and a flag that names anything the check cannot see makes the build unstampable (linkInputs);
//   - the compilers (Compilers) and the program that was written (Output).
//
// A stamp is valid when all of that is as recorded. The answer is "the program is up to date"
// (nothing is printed, nothing is written or touched), or a reason that -x shows ("# not up to date:
// ...") and a normal build, which writes a new stamp when it succeeds. The check is sound in the
// sense that a build with no input changed produces the same program as the one before; it
// is not the same check as the object cache's, which is why a stamp never replaces it: the object
// cache keeps its own answers for the compiles that a changed program needs. What a stamp does not
// see, and a normal build of today does not either, is a file that the linker reads without being
// asked (the compiler's own libraries, behind its identity) and the contents of a system library
// that changed with the same size and times.
//
// A file is trusted when its size, modification time, inode and change time are as recorded and
// none of the times is within objRacyWindow of the moment it was read (ctime can't be set back by a
// user, so `touch -d` is seen); else it is hashed, and a file with the recorded hash is fine (the stamp is
// written again with its new stat, so the next check is cheap). Where the system has no change time
// (stat_other.go), every file is hashed.

// stampVersion changes when the format of a stamp or the meaning of what it records changes.
const stampVersion = "2"

// A statRec is what the check keeps of a file's stat, and of its content when it must be hashed.
type statRec struct {
	Size int64  `json:"s"`
	Mod  int64  `json:"m"`
	Ino  uint64 `json:"i,omitempty"`
	Chg  int64  `json:"c,omitempty"`
	Mode uint32 `json:"o"` // the permission bits (a chmod of the program is a change that its content hash does not show)
	Hash string `json:"h,omitempty"`
	Racy bool   `json:"r,omitempty"` // changed too near the read: compare the hash, not the stat
}

// recOf returns the record of the file with the stat fi, read at at.
func recOf(fi fs.FileInfo, at time.Time) statRec {
	sys := sysStatOf(fi)
	r := statRec{Size: fi.Size(), Mod: fi.ModTime().UnixNano(), Ino: sys.ino, Chg: sys.ctime, Mode: modeBits(fi)}
	r.Racy = sys.ctime == 0 || at.Sub(fi.ModTime()) < objRacyWindow || at.Sub(time.Unix(0, sys.ctime)) < objRacyWindow
	return r
}

// matches reports whether fi is the file of the record, by its stat.
func (r statRec) matches(fi fs.FileInfo) bool {
	sys := sysStatOf(fi)
	return fi.Size() == r.Size && fi.ModTime().UnixNano() == r.Mod && sys.ino == r.Ino && sys.ctime == r.Chg && modeBits(fi) == r.Mode
}

// modeBits returns the permission bits of a file (with setuid, setgid and sticky).
func modeBits(fi fs.FileInfo) uint32 {
	return uint32(fi.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky))
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// A statFunc is os.Stat, or a memo of it for a check that asks about the same file for many entries.
type statFunc func(name string) (fs.FileInfo, error)

// A statCache asks the system once for each name (Stat and Lstat apart): a build asks about the same
// headers for the entries of many units, and about the same files that must be absent (the product of
// the folders and the user's headers of each entry), and a program of a hundred packages made that
// 15000 system calls at 5 to 100 microseconds. A file that changes during the build is a race that
// the build does not claim to see (the entries are checked before the compiles run). A nil cache
// asks the system every time.
type statCache struct {
	stats, lstats sync.Map // name -> statRes
	hashes        sync.Map // path -> hashedFile: the files that a lookup had to read
	sets          sync.Map // name of a set -> []depRec: the sets of the object cache, with the paths of this build
}

type statRes struct {
	fi  fs.FileInfo
	err error
}

// statMemo returns a new statCache.
func statMemo() *statCache { return new(statCache) }

func (c *statCache) get(m *sync.Map, name string, f func(string) (fs.FileInfo, error)) (fs.FileInfo, error) {
	if v, ok := m.Load(name); ok {
		r := v.(statRes)
		return r.fi, r.err
	}
	fi, err := f(name)
	m.LoadOrStore(name, statRes{fi, err})
	return fi, err
}

// Stat is os.Stat.
func (c *statCache) Stat(name string) (fs.FileInfo, error) {
	if c == nil {
		return os.Stat(name)
	}
	return c.get(&c.stats, name, os.Stat)
}

// hash returns the hash and stat of the file path, reading it once (hashFile).
func (c *statCache) hash(path string) (hashedFile, bool) {
	if c == nil {
		return hashFile(path)
	}
	if v, ok := c.hashes.Load(path); ok {
		return v.(hashedFile), true
	}
	h, ok := hashFile(path)
	if ok {
		c.hashes.Store(path, h)
	}
	return h, ok
}

// Lstat is os.Lstat.
func (c *statCache) Lstat(name string) (fs.FileInfo, error) {
	if c == nil {
		return os.Lstat(name)
	}
	return c.get(&c.lstats, name, os.Lstat)
}

// ---- what the loader looked at

// An fsOp is one look of the loader at the file system: Kind is "f" (ReadFile), "d" (ReadDir) or
// "s" (Stat).
type fsOp struct {
	Kind   string  `json:"k"`
	Path   string  `json:"p"`
	Exists bool    `json:"e,omitempty"`
	Dir    bool    `json:"d,omitempty"` // "s": the name is a folder
	Hash   string  `json:"h,omitempty"` // "d": the hash of the listing (listingHash)
	Stat   statRec `json:"t"`           // "f" and "d": the stat, with the hash of the file in Stat.Hash
}

// An FSRecorder is a load.FileSystem (the interface of the loader) that reads the operating
// system's files and records every read, listing and stat: the trace of a Stamp. Bad says why a
// load can't be stamped: a file that changed while it was read, or an error other than "does not
// exist".
type FSRecorder struct {
	mu   sync.Mutex
	ops  []fsOp
	seen map[string]int // kind + path -> index in ops
	bad  string
}

// NewFSRecorder returns an empty recorder.
func NewFSRecorder() *FSRecorder { return &FSRecorder{seen: make(map[string]int)} }

// Bad returns the reason the recorded load can't be stamped, or "".
func (r *FSRecorder) Bad() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bad
}

// Ops returns the number of looks recorded (for tests and -x).
func (r *FSRecorder) Ops() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.ops)
}

func (r *FSRecorder) fail(why string) {
	if r.bad == "" {
		r.bad = why
	}
}

// add records op. A second look at the same name keeps the first record; a file that read
// differently the second time changed during the build.
func (r *FSRecorder) add(op fsOp) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := op.Kind + "\x00" + op.Path
	if i, ok := r.seen[key]; ok {
		old := r.ops[i]
		if old.Exists != op.Exists || old.Dir != op.Dir || old.Hash != op.Hash || old.Stat.Hash != op.Stat.Hash {
			r.fail(op.Path + " changed while it was loaded")
		}
		return
	}
	r.seen[key] = len(r.ops)
	r.ops = append(r.ops, op)
}

// ReadFile reads the file name like os.ReadFile and records it.
func (r *FSRecorder) ReadFile(name string) ([]byte, error) {
	before, serr := os.Stat(name)
	data, err := os.ReadFile(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			r.add(fsOp{Kind: "f", Path: name})
		} else {
			r.mu.Lock()
			r.fail(fmt.Sprintf("%s can't be read: %v", name, err))
			r.mu.Unlock()
		}
		return data, err
	}
	after, aerr := os.Stat(name)
	if serr != nil || aerr != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || sysStatOf(before) != sysStatOf(after) {
		r.mu.Lock()
		r.fail(name + " changed while it was read")
		r.mu.Unlock()
		return data, nil
	}
	rec := recOf(after, time.Now())
	rec.Hash = hashHex(data)
	r.add(fsOp{Kind: "f", Path: name, Exists: true, Stat: rec})
	return data, nil
}

// Open reads the whole file (the loader reads the start of a C file to find its build constraints:
// the check wants all of it) and returns a reader of the bytes.
func (r *FSRecorder) Open(name string) (io.ReadCloser, error) {
	data, err := r.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}

// listingHash returns the hash of the names of a folder's entries and their kinds, in the order
// that os.ReadDir gives (by name): the folders and symbolic links, and the files that the loader
// looks at by their names (relevantFile). The program that the build writes next to the sources
// is none of those, so it does not change the listing: a build must not make its own stamp stale.
func listingHash(entries []fs.DirEntry) string {
	h := sha256.New()
	for _, e := range entries {
		kind := "f"
		switch {
		case e.IsDir():
			kind = "d"
		case e.Type()&fs.ModeSymlink != 0:
			kind = "l"
		case !e.Type().IsRegular():
			kind = "o"
		}
		if kind == "f" && !relevantFile(e.Name()) {
			continue
		}
		fmt.Fprintf(h, "%s\x00%s\n", e.Name(), kind)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// relevantFile reports whether the loader does something with a file of this name in a package's
// folder: the .gp files, the C and C++ files and the kinds of source that it reports as not
// supported (internal/load/sources.go), gocpp.mod and gocpp.sum, and headers.
func relevantFile(name string) bool {
	switch filepath.Ext(name) {
	case ".gp", ".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp", ".hxx", ".s", ".S", ".sx", ".m", ".f", ".F", ".for", ".f90", ".swig", ".swigcxx", ".syso", ".mod", ".sum":
		return true
	}
	return false
}

// ReadDir lists the folder name like os.ReadDir and records it.
func (r *FSRecorder) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			r.add(fsOp{Kind: "d", Path: name})
		} else {
			r.mu.Lock()
			r.fail(fmt.Sprintf("%s can't be listed: %v", name, err))
			r.mu.Unlock()
		}
		return entries, err
	}
	fi, serr := os.Stat(name)
	if serr != nil {
		r.mu.Lock()
		r.fail(name + " can't be looked at: " + serr.Error())
		r.mu.Unlock()
		return entries, nil
	}
	r.add(fsOp{Kind: "d", Path: name, Exists: true, Hash: listingHash(entries), Stat: recOf(fi, time.Now())})
	return entries, nil
}

// Stat looks at the file name like os.Stat and records whether it exists and is a folder.
func (r *FSRecorder) Stat(name string) (fs.FileInfo, error) {
	fi, err := os.Stat(name)
	r.add(fsOp{Kind: "s", Path: name, Exists: err == nil, Dir: err == nil && fi.IsDir()})
	return fi, err
}

// Trace returns the looks that were recorded, for a Stamp.
func (r *FSRecorder) Trace() []fsOp {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.ops)
}

// ---- pkg-config

// A PkgConfigRec is one run of pkg-config that the check made: the flags that it gave.
type PkgConfigRec struct {
	Dir    string   `json:"dir"`
	Pkgs   []string `json:"pkgs"`
	CFlags []string `json:"cflags,omitempty"`
	Libs   []string `json:"libs,omitempty"`
	Err    string   `json:"err,omitempty"`
}

// A PkgConfigLog records the runs of the PkgConfigFunc that it wraps.
type PkgConfigLog struct {
	mu   sync.Mutex
	recs []PkgConfigRec
}

// Wrap returns a PkgConfigFunc that runs inner (or PkgConfig when it is nil) and records what it
// answered.
func (l *PkgConfigLog) Wrap(inner PkgConfigFunc) PkgConfigFunc {
	if inner == nil {
		inner = PkgConfig
	}
	return func(ctx context.Context, dir string, pkgs []string) ([]string, []string, error) {
		cflags, libs, err := inner(ctx, dir, pkgs)
		rec := PkgConfigRec{Dir: dir, Pkgs: slices.Clone(pkgs), CFlags: slices.Clone(cflags), Libs: slices.Clone(libs)}
		if err != nil {
			rec.Err = err.Error()
		}
		l.mu.Lock()
		l.recs = append(l.recs, rec)
		l.mu.Unlock()
		return cflags, libs, err
	}
}

// Recs returns the runs recorded, in order.
func (l *PkgConfigLog) Recs() []PkgConfigRec {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.recs)
}

// ---- the compiles

// A UnitRec is one compile of a build as the up-to-date check keeps it: the source file, the headers
// the compiler read (Deps: numbers of records of Stamp.Files, which the units share) and the files
// that must be absent (the object cache entry's Absent and Except). All the paths are real.
type UnitRec struct {
	Src    string        `json:"src"`
	SrcRec statRec       `json:"srcrec"` // Hash is the hash of the text that was compiled
	Deps   []int         `json:"deps"`
	Absent []absentGroup `json:"absent,omitempty"`
	Except []string      `json:"except,omitempty"`

	deps []depRec // what the compile read, as the Builder gives it to the Record
}

// A Record collects what a build did for a Stamp: the compiles (the Builder adds one for each), the
// link, and the first reason that makes the build unstampable.
type Record struct {
	mu    sync.Mutex
	units []UnitRec
	bad   string
	link  *LinkRec
}

func (r *Record) addUnit(u UnitRec) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.units = append(r.units, u)
	r.mu.Unlock()
}

// Fail says that the build can't be stamped, and why (the first reason stays).
func (r *Record) Fail(why string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.bad == "" {
		r.bad = why
	}
	r.mu.Unlock()
}

// Bad returns the reason the build can't be stamped, or "".
func (r *Record) Bad() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.bad
}

// ---- the link

// A LinkFile is a file that the linker read besides the objects of the build: a library, a start
// file, a linker script.
type LinkFile struct {
	Flag string  `json:"flag,omitempty"` // "ld" for the linker program
	Path string  `json:"path"`
	Stat statRec `json:"stat"`
}

// A LinkRec is what the link read besides the objects of the build.
type LinkRec struct {
	Flags  []string   `json:"flags"` // the flags of the packages and of GOCPP_LDFLAGS
	Files  []LinkFile `json:"files,omitempty"`
	Linker *LinkFile  `json:"linker,omitempty"` // the program that the compiler's -print-prog-name=ld names
}

// The files that the linker reads are known from the linker itself: the link of a build that is
// recorded runs with -Wl,-t (--trace), which every linker that gocpp uses (GNU ld, gold, lld, mold)
// understands: it prints the name of every file it opens, one on a line, the libraries that -lx
// found and the ones that the compiler adds (libstdc++, libc, the start files). linkTrace takes
// those lines out of the output of the link (what is left is the linker's own messages) and returns
// the files. The names under work (the objects of this build) are not returned. Resolving -lx
// by hand does not work: a Nix compiler wrapper adds its -L folders where only the linker sees them.
func linkTrace(out []byte, work string) (rest []byte, files []string) {
	var keep []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if tempObject(line) {
			continue // a temporary object of link-time optimization, deleted before gocpp could look
		}
		if p := tracedFile(line, work); p != "" {
			if p != "-" && !seen[p] {
				seen[p] = true
				files = append(files, p)
			}
			continue
		}
		keep = append(keep, line)
	}
	return []byte(strings.Join(keep, "\n")), files
}

// tempObject reports whether the line of a linker's trace is the path of an object file in the
// temporary folder that no longer exists: what a link with -flto traces (clang's lto-llvm-*.o, GCC's
// *.ltrans0.ltrans.o and *.debug.temp.o), after the compiler deleted it. Such a name is neither a
// file the check can follow nor a message for the user. (The test is for the shape of the line, not
// for the compilers' names, so a new compiler's temporary object goes the same way.)
func tempObject(line string) bool {
	if !filepath.IsAbs(line) || !strings.HasSuffix(line, ".o") || strings.ContainsAny(line, " \t(") {
		return false
	}
	tmp := os.TempDir()
	if !strings.HasPrefix(line, filepath.Clean(tmp)+string(filepath.Separator)) {
		return false
	}
	_, err := os.Lstat(line)
	return err != nil
}

// tracedFile returns the file that the line of a linker's trace names: "-" for a file of the build's
// own work folder (an object: not recorded, but not a message either), "" when the line is no trace line.
// The forms are the plain name of a file, and for a member of an archive that the linker took in
// "(/path/libx.a)member.o" (GNU ld) or "/path/libx.a(member.o)" (gold, lld).
func tracedFile(line, work string) string {
	if line == "" || strings.ContainsRune(line, '\x00') {
		return ""
	}
	cands := []string{line}
	if strings.HasPrefix(line, "(") {
		if i := strings.Index(line, ")"); i > 0 {
			cands = append(cands, line[1:i])
		}
	} else if strings.HasSuffix(line, ")") {
		if i := strings.LastIndex(line, "("); i > 0 {
			cands = append(cands, line[:i])
		}
	}
	for _, c := range cands {
		p := c
		if !filepath.IsAbs(p) {
			p = filepath.Join(work, p)
		}
		fi, err := os.Stat(p)
		if err != nil || !fi.Mode().IsRegular() || !filepath.IsAbs(c) && !strings.HasPrefix(p, work+string(filepath.Separator)) {
			continue
		}
		if work != "" && strings.HasPrefix(p, work+string(filepath.Separator)) {
			return "-"
		}
		return p
	}
	return ""
}

// linkWlSafe are the arguments of -Wl, that read no file the trace would not show.
var linkWlSafe = map[string]bool{
	"--as-needed": true, "--no-as-needed": true, "--start-group": true, "--end-group": true, "-Bstatic": true,
	"-Bdynamic": true, "--gc-sections": true, "--no-gc-sections": true, "--export-dynamic": true, "-E": true,
	"--no-undefined": true, "-z": true, "-O1": true, "-O0": true, "-s": true, "-S": true, "--strip-all": true,
	"--strip-debug": true, "--build-id": true, "--fatal-warnings": true, "--warn-common": true, "-t": true,
}

// linkFlagProblem returns why the link flags can't be followed by the check, or "": a flag that
// makes the linker read a file that --trace does not show (a version script, a dynamic list, a
// response file, another linker) or write one (-Map), or that the check does not know.
func linkFlagProblem(flags []string) string {
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case f == "-pthread", f == "-static", f == "-static-libstdc++", f == "-static-libgcc", f == "-rdynamic",
			f == "-s", f == "-g", f == "-no-pie", f == "-pie":
		case strings.HasPrefix(f, "-fsanitize="), f == "-flto", strings.HasPrefix(f, "-flto="), f == "-fno-omit-frame-pointer",
			f == "-O0" || f == "-O1" || f == "-O2" || f == "-O3" || f == "-Os" || f == "-Og":
			// the flags of -race and -release (flags.go): they read no file
		case f == "-L" || f == "-l":
			i++ // its value
		case strings.HasPrefix(f, "-L") || strings.HasPrefix(f, "-l"):
		case strings.HasPrefix(f, "-Wl,"):
			parts := strings.Split(f[4:], ",")
			for j := 0; j < len(parts); j++ {
				p := parts[j]
				switch {
				case linkWlSafe[p]:
					if p == "-z" {
						j++ // its keyword
					}
				case p == "-rpath" || p == "-rpath-link" || p == "-soname" || p == "-h":
					j++ // the value is a path or a name that is not read
				case strings.HasPrefix(p, "-rpath=") || strings.HasPrefix(p, "-soname=") || strings.HasPrefix(p, "--hash-style="):
				default:
					return "the link flag " + f + " (" + p + ") may read or write a file that the check can't follow"
				}
			}
		case filepath.IsAbs(f) && (strings.HasSuffix(f, ".a") || strings.HasSuffix(f, ".so") || strings.HasSuffix(f, ".o") || strings.HasSuffix(f, ".lib")):
			// An archive or an object named by its path: the trace shows it.
		default:
			return "the link flag " + f + " may read a file that the check can't follow"
		}
	}
	return ""
}

// linkFile makes the record of the file p that the link read, for the flag flag.
func linkFile(flag, p string, fi fs.FileInfo, at time.Time) LinkFile {
	rec := recOf(fi, at)
	if rec.Racy {
		if data, err := os.ReadFile(p); err == nil {
			rec.Hash = hashHex(data)
		}
	}
	return LinkFile{Flag: flag, Path: p, Stat: rec}
}

// recordLink records the link of a build whose link flags are flags (the packages' and
// GOCPP_LDFLAGS): the flags are checked (linkFlagProblem), and the linker that the compiler runs is
// noted. setLinkFiles adds the files that the link read.
func (b *Builder) recordLink(ctx context.Context, flags []string) {
	if b.Rec == nil {
		return
	}
	if why := linkFlagProblem(flags); why != "" {
		b.Rec.Fail(why)
		return
	}
	lr := &LinkRec{Flags: slices.Clone(flags)}
	// The linker the compiler runs: a new linker is a new link.
	if out, err := b.CXX.Command(ctx, "-print-prog-name=ld").Output(); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			if !strings.ContainsAny(name, `/\`) {
				if p, err := exec.LookPath(name); err == nil {
					name = p
				}
			}
			if fi, err := os.Stat(name); err == nil && fi.Mode().IsRegular() && filepath.IsAbs(name) {
				f := linkFile("ld", name, fi, time.Now())
				lr.Linker = &f
			}
		}
	}
	b.Rec.mu.Lock()
	b.Rec.link = lr
	b.Rec.mu.Unlock()
}

// setLinkFiles records the files that the link read (linkTrace).
func (b *Builder) setLinkFiles(paths []string) {
	if b.Rec == nil {
		return
	}
	b.Rec.mu.Lock()
	defer b.Rec.mu.Unlock()
	if b.Rec.link == nil {
		return
	}
	now := time.Now()
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			if b.Rec.bad == "" {
				b.Rec.bad = "a file of the link can't be looked at: " + p
			}
			return
		}
		b.Rec.link.Files = append(b.Rec.link.Files, linkFile("", p, fi, now))
	}
}

// ---- the stamp

// An OutputRec is the program a build wrote.
type OutputRec struct {
	Path string  `json:"path"`
	Stat statRec `json:"stat"`
}

// A PkgSummary is what the stamp keeps of one package: the hash of its sources and the API hash,
// the hash of the header pkg.hpp that every package that imports it includes (its types,
// prototypes, variables and exported constants with their values). A change of the API hash
// is what recompiles the units of the packages that import it (the object cache sees the header);
// the stamp keeps it for -x and for tests.
type PkgSummary struct {
	Path string `json:"path"`
	Src  string `json:"src"`
	API  string `json:"api"`
}

// ChangedPackages compares the summaries of an earlier build with those of this one and returns the
// import paths of the packages whose sources changed, and of those whose API changed (a package that
// is new counts as both), in the order of now.
func ChangedPackages(old, now []PkgSummary) (src, api []string) {
	was := make(map[string]PkgSummary, len(old))
	for _, p := range old {
		was[p.Path] = p
	}
	for _, p := range now {
		o, ok := was[p.Path]
		if !ok || o.Src != p.Src {
			src = append(src, p.Path)
		}
		if !ok || o.API != p.API {
			api = append(api, p.Path)
		}
	}
	return src, api
}

// A Stamp is the record of a successful build (see the comment at the top).
type Stamp struct {
	Version   string         `json:"v"`
	Key       string         `json:"key"`
	Time      int64          `json:"time"`
	Compilers []string       `json:"compilers"`
	Trace     []fsOp         `json:"trace"`
	PkgConfig []PkgConfigRec `json:"pkgconfig,omitempty"`
	Units     []UnitRec      `json:"units"`
	Files     []depRec       `json:"files"` // the headers that the compiles read, once each
	Link      *LinkRec       `json:"link"`
	Output    OutputRec      `json:"output"`
	Extra     []OutputRec    `json:"extra,omitempty"` // the other files a library mode wrote (stampextra.go)
	Pkgs      []PkgSummary   `json:"pkgs,omitempty"`

	file  string
	dirty bool
}

// StampKey returns the key of the question that a build asks: the hash of parts.
func StampKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:%s\x00", len(p), p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CompilerID returns the string that tells the compiler c in a stamp: what writeCompiler puts in
// the keys of the object cache.
func CompilerID(c CXX) string {
	var parts []string
	writeCompiler(func(p ...string) { parts = append(parts, p...) }, c)
	return strings.Join(parts, "\x00")
}

func stampFile(cache, key string) string {
	return filepath.Join(cache, "stamp", key[:40]+".json")
}

// LoadStamp reads the stamp of the question key from the cache folder, or returns nil: none, or one
// that is not a stamp of this version or this question.
func LoadStamp(cache, key string) *Stamp {
	if cache == "" || objCacheOff() {
		return nil
	}
	file := stampFile(cache, key)
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var s Stamp
	if json.Unmarshal(data, &s) != nil || s.Version != stampVersion || s.Key != key || s.Link == nil {
		return nil
	}
	s.file = file
	return &s
}

// NewStamp makes the stamp of a build: trace is what the loader looked at, pkgConfig the runs of
// pkg-config, rec what the Builder recorded (Link must have returned: the stores of the object
// cache are done), compilers the identities of the compilers (CompilerID), and out the program,
// which must be written. It returns nil and why when the
// build can't be stamped.
func NewStamp(key string, trace *FSRecorder, pkgConfig *PkgConfigLog, rec *Record, compilers []string, out string, pkgs []PkgSummary) (*Stamp, string) {
	if why := trace.Bad(); why != "" {
		return nil, why
	}
	if why := rec.Bad(); why != "" {
		return nil, why
	}
	rec.mu.Lock()
	units := slices.Clone(rec.units)
	link := rec.link
	rec.mu.Unlock()
	if link == nil {
		return nil, "the link was not recorded"
	}
	fi, err := os.Stat(out)
	if err != nil || !fi.Mode().IsRegular() {
		return nil, "the program can't be looked at"
	}
	now := time.Now()
	orec := recOf(fi, now)
	if orec.Racy {
		data, err := os.ReadFile(out)
		if err != nil {
			return nil, "the program can't be read"
		}
		orec.Hash = hashHex(data)
	}
	s := &Stamp{Version: stampVersion, Key: key, Time: now.UnixNano(), Compilers: compilers, Trace: trace.Trace(),
		Units: units, Link: link, Output: OutputRec{Path: out, Stat: orec}, Pkgs: pkgs}
	if pkgConfig != nil {
		s.PkgConfig = pkgConfig.Recs()
	}
	slices.SortFunc(s.Units, func(a, b UnitRec) int { return strings.Compare(a.Src, b.Src) })
	// The headers of all the compiles, each once: a program's units read the same few hundred
	// system headers, so a stamp that lists them for each unit would be 100 times as big and as
	// slow to read.
	index := make(map[string]int)
	for i := range s.Units {
		u := &s.Units[i]
		u.Deps = make([]int, 0, len(u.deps))
		for _, d := range u.deps {
			k, ok := index[d.Path]
			if !ok {
				k = len(s.Files)
				index[d.Path] = k
				s.Files = append(s.Files, d)
			} else if f := s.Files[k]; f.Hash != d.Hash {
				return nil, d.Path + " changed while the program was built"
			}
			u.Deps = append(u.Deps, k)
		}
		u.deps = nil
	}
	return s, ""
}

// Save writes the stamp into the cache folder (a rename: a reader never sees half of it).
func (s *Stamp) Save(cache string) error {
	if cache == "" {
		return errors.New("no cache folder")
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	file := stampFile(cache, s.Key)
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	if !writeFileAtomic(file, data) {
		return errors.New("the stamp can't be written")
	}
	s.file, s.dirty = file, false
	TrimStamps(cache)
	return nil
}

// Touch sets the modification time of the stamp's file to now when it is older than an hour, so that
// the trimming of old stamps (TrimStamps) removes the records of programs that are not built any more,
// not those of programs that are built, with nothing to do, every day.
func (s *Stamp) Touch() {
	if s.file == "" {
		return
	}
	if fi, err := os.Stat(s.file); err == nil && time.Since(fi.ModTime()) > objTouchAge {
		now := time.Now()
		os.Chtimes(s.file, now, now)
	}
}

// Dirty reports whether Check found files that were touched but have their content, so that
// Save should write the stamp again with their new stat.
func (s *Stamp) Dirty() bool { return s.dirty }

// PkgSummaries returns the package summaries of the stamp.
func (s *Stamp) PkgSummaries() []PkgSummary { return s.Pkgs }

// checkFile reports whether the file p is as the record says: its stat matches and is not racy,
// or its content has the recorded hash (and then the record gets the new stat).
func (s *Stamp) checkFile(p string, r *statRec, stat statFunc) bool {
	fi, err := stat(p)
	if err != nil {
		return false
	}
	if !fi.Mode().IsRegular() {
		return false
	}
	if !r.Racy && r.matches(fi) {
		return true
	}
	if r.Hash == "" || modeBits(fi) != r.Mode {
		return false
	}
	data, err := os.ReadFile(p)
	if err != nil || hashHex(data) != r.Hash {
		return false
	}
	if now := recOf(fi, time.Now()); !now.Racy {
		now.Hash = r.Hash
		*r = now
		s.dirty = true
	}
	return true
}

// Check reports why the build that the stamp records is not up to date, or "" when it is. cache is
// the cache folder (the object cache's sets are in it), compilers the identities of the compilers
// as they are now (CompilerID, in the order of the stamp's), and pkgConfig the function that runs
// pkg-config, or nil for the real one. ctx stops the runs of pkg-config.
func (s *Stamp) Check(ctx context.Context, cache string, compilers []string, pkgConfig PkgConfigFunc) string {
	if !slices.Equal(s.Compilers, compilers) {
		return "the compiler is not the one of the last build"
	}
	sc := statMemo()
	stat := sc.Stat
	// The program first: the cheapest and the most likely to differ.
	if !s.checkFile(s.Output.Path, &s.Output.Stat, stat) {
		return s.Output.Path + " is missing or was changed"
	}
	for i := range s.Extra {
		if !s.checkFile(s.Extra[i].Path, &s.Extra[i].Stat, stat) {
			return s.Extra[i].Path + " is missing or was changed"
		}
	}
	// What the loader looked at.
	for i := range s.Trace {
		op := &s.Trace[i]
		switch op.Kind {
		case "s":
			fi, err := os.Stat(op.Path)
			if (err == nil) != op.Exists || err == nil && fi.IsDir() != op.Dir {
				return op.Path + " appeared, disappeared or changed its kind"
			}
		case "f":
			if !op.Exists {
				if _, err := os.Stat(op.Path); err == nil {
					return op.Path + " appeared"
				}
				continue
			}
			if !s.checkFile(op.Path, &op.Stat, stat) {
				return op.Path + " changed"
			}
		case "d":
			if !op.Exists {
				if _, err := os.Stat(op.Path); err == nil {
					return op.Path + " appeared"
				}
				continue
			}
			if fi, err := os.Stat(op.Path); err == nil && fi.IsDir() && !op.Stat.Racy && op.Stat.matches(fi) {
				continue
			}
			entries, err := os.ReadDir(op.Path)
			if err != nil || listingHash(entries) != op.Hash {
				return "the files of " + op.Path + " changed"
			}
			if fi, err := os.Stat(op.Path); err == nil {
				if now := recOf(fi, time.Now()); !now.Racy {
					op.Stat = now
					s.dirty = true
				}
			}
		default:
			return "a stamp of another format"
		}
	}
	// pkg-config answers.
	if len(s.PkgConfig) > 0 {
		if pkgConfig == nil {
			pkgConfig = PkgConfig
		}
		for _, p := range s.PkgConfig {
			cflags, libs, err := pkgConfig(ctx, p.Dir, p.Pkgs)
			errText := ""
			if err != nil {
				errText = err.Error()
			}
			if errText != p.Err || !slices.Equal(cflags, p.CFlags) || !slices.Equal(libs, p.Libs) {
				return "pkg-config " + strings.Join(p.Pkgs, " ") + " answers differently"
			}
		}
	}
	// The compiles: the headers they read (once each), then for each unit its source and the files that
	// must be absent. A header that was touched but has its content gets its new stat in the stamp.
	for k := range s.Files {
		var e objEntry
		ok, fresh := e.validWith(s.Files[k:k+1], sc)
		if !ok {
			return "the header " + s.Files[k].Path + " changed"
		}
		if fresh != nil {
			s.Files[k] = fresh[0]
			s.dirty = true
		}
	}
	// The files that must be absent: a file that would hide a header of a unit (the same name in a
	// folder that the compile searched before the one that had it). The units of a program ask about
	// the same few folders and the same header names (the product of the two, for each unit), so the
	// system is asked once for each folder and name; then a unit needs to look at the files that exist
	// (nearly always none).
	present := make(map[string][]string) // folder -> the names of its candidates that exist
	asked := make(map[string]map[string]bool)
	for i := range s.Units {
		for _, g := range s.Units[i].Absent {
			for _, dir := range g.Dirs {
				m := asked[dir]
				if m == nil {
					m = make(map[string]bool)
					asked[dir] = m
				}
				for _, rel := range g.Rels {
					if m[rel] {
						continue
					}
					m[rel] = true
					if _, err := sc.Lstat(dir + string(filepath.Separator) + rel); !errors.Is(err, fs.ErrNotExist) {
						present[dir] = append(present[dir], rel)
					}
				}
			}
		}
	}
	for i := range s.Units {
		u := &s.Units[i]
		if !s.checkFile(u.Src, &u.SrcRec, stat) {
			return u.Src + " changed"
		}
		for _, g := range u.Absent {
			for _, dir := range g.Dirs {
				for _, rel := range present[dir] {
					if !slices.Contains(g.Rels, rel) {
						continue
					}
					cand := dir + string(filepath.Separator) + rel
					if slices.Contains(u.Except, cand) || slices.ContainsFunc(u.Deps, func(k int) bool { return k >= 0 && k < len(s.Files) && s.Files[k].Path == cand }) {
						continue // a dependency itself, or a file that existed and did not matter
					}
					return "a file appeared in front of a header of " + u.Src
				}
			}
		}
	}
	// The link.
	if why := s.checkLink(stat); why != "" {
		return why
	}
	return ""
}

// checkLink reports why the link of the stamp is not the link of now.
func (s *Stamp) checkLink(stat statFunc) string {
	l := s.Link
	for i := range l.Files {
		if !s.checkFile(l.Files[i].Path, &l.Files[i].Stat, stat) {
			return l.Files[i].Path + " changed"
		}
	}
	if l.Linker != nil && !s.checkFile(l.Linker.Path, &l.Linker.Stat, stat) {
		return "the linker " + l.Linker.Path + " changed"
	}
	return ""
}

// FileHash returns the hash of the file that the loader read (ReadFile), or "".
func (r *FSRecorder) FileHash(name string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if i, ok := r.seen["f\x00"+name]; ok {
		return r.ops[i].Stat.Hash
	}
	return ""
}

// KeyEnvName reports whether the environment variable name changes what the C++ compiler does
// (include paths, the flags and switches of a Nix wrapper, deployment targets): the variables of the
// key of the object cache.
func KeyEnvName(name string) bool { return objKeyEnv(name) }

// ObjectCacheOff reports whether GOCPP_NOOBJCACHE turns the object cache off.
func ObjectCacheOff() bool { return objCacheOff() }
