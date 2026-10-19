package modfetch

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// SumFileName is the name of the checksum file, next to gocpp.mod in the main
// module's folder.
const SumFileName = "gocpp.sum"

const maxSumFile = 16 << 20

// A SumLine is a line of gocpp.sum: the hash of the files of a module version,
// or, for the version "v1.2.3/gocpp.mod", of its gocpp.mod file alone.
type SumLine struct {
	Path    string
	Version string // v1.2.3, or v1.2.3/gocpp.mod
	Hash    string // h1:...
}

// Sums are the lines of a gocpp.sum file.
type Sums struct {
	Name  string // the file
	lines []SumLine
	by    map[string][]string // "path version" -> hashes
}

func sumKey(path, version string) string { return path + " " + version }

// LoadSums reads the gocpp.sum file of the main module in dir. A file that is
// not there is an empty list.
func LoadSums(dir string) (*Sums, error) {
	name := filepath.Join(dir, SumFileName)
	s := &Sums{Name: name, by: map[string][]string{}}
	data, err := readSmall(name)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	return s, s.parse(data)
}

func readSmall(name string) ([]byte, error) { return ReadRegular(name, maxSumFile) }

// ReadRegular reads a file of at most max bytes that is a regular file (it
// may be a link to one): a device such as /dev/zero, a FIFO or a folder is an
// error and is not read, and a file that grows is cut at the limit.
func ReadRegular(name string, max int64) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	if st.Size() > max {
		return nil, fmt.Errorf("%s: file is too large (%d bytes, the limit is %d)", name, st.Size(), max)
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%s: file is too large (the limit is %d)", name, max)
	}
	return data, nil
}

// CheckWritable returns an error if name is a symbolic link: gocpp writes
// a file by renaming a new one over it, which would replace the link, so it
// asks for the file that the link points to to be edited instead.
func CheckWritable(name string) error {
	if st, err := os.Lstat(name); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symbolic link: gocpp does not replace it (change the file that it points to)", name)
	}
	return nil
}

func validHash(h string) bool {
	rest, ok := strings.CutPrefix(h, "h1:")
	if !ok {
		return false
	}
	b, err := base64.StdEncoding.DecodeString(rest)
	return err == nil && len(b) == 32
}

func (s *Sums) parse(data []byte) error {
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 {
			return fmt.Errorf("%s:%d: malformed line (want: module version h1:hash): %q", s.Name, i+1, line)
		}
		base, _, _ := strings.Cut(f[1], "/")
		if CheckModulePath(f[0]) != nil || !isVersionLike(base) {
			return fmt.Errorf("%s:%d: malformed module or version: %q", s.Name, i+1, line)
		}
		if !validHash(f[2]) {
			return fmt.Errorf("%s:%d: malformed or unsupported hash %q (want h1:...)", s.Name, i+1, f[2])
		}
		s.add(SumLine{f[0], f[1], f[2]})
	}
	return nil
}

func isVersionLike(v string) bool { return gocppmod.ValidVersion(v) }

func (s *Sums) add(l SumLine) bool {
	k := sumKey(l.Path, l.Version)
	if slices.Contains(s.by[k], l.Hash) {
		return false
	}
	s.by[k] = append(s.by[k], l.Hash)
	s.lines = append(s.lines, l)
	return true
}

// Lookup returns the hashes that the file lists for the module version (suffix
// "" for the files of the module, "/gocpp.mod" for its module file).
func (s *Sums) Lookup(path, version, suffix string) []string {
	return s.by[sumKey(path, version+suffix)]
}

// Lines returns the lines of the file, sorted.
func (s *Sums) Lines() []SumLine {
	out := slices.Clone(s.lines)
	slices.SortFunc(out, func(a, b SumLine) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		av, as, _ := strings.Cut(a.Version, "/")
		bv, bs, _ := strings.Cut(b.Version, "/")
		if c := CompareVersions(av, bv); c != 0 {
			return c
		}
		if c := strings.Compare(av, bv); c != 0 {
			return c
		}
		if c := strings.Compare(as, bs); c != 0 {
			return c
		}
		return strings.Compare(a.Hash, b.Hash)
	})
	return out
}

func (s *Sums) format() []byte {
	var b bytes.Buffer
	for _, l := range s.Lines() {
		fmt.Fprintf(&b, "%s %s %s\n", l.Path, l.Version, l.Hash)
	}
	return b.Bytes()
}

// A SecurityError is a download or a cached module whose hash is not the one
// in gocpp.sum.
type SecurityError struct {
	Path, Version string
	What          string   // "" for the module, "gocpp.mod" for its module file
	Got           string   // the hash of what is there
	Want          []string // the hashes that gocpp.sum lists
	Cached        string   // the folder of a module in the cache, if the cache has the mismatch
	SumFile       string
}

func (e *SecurityError) Error() string {
	var b strings.Builder
	what := e.Path + " " + e.Version
	if e.What != "" {
		what += "/" + e.What
	}
	fmt.Fprintf(&b, "SECURITY ERROR\n%s: checksum mismatch\n", what)
	if e.Cached != "" {
		fmt.Fprintf(&b, "\tmodule cache (%s): %s\n", e.Cached, e.Got)
	} else {
		fmt.Fprintf(&b, "\tdownloaded: %s\n", e.Got)
	}
	for i, w := range e.Want {
		if i == 0 {
			fmt.Fprintf(&b, "\t%s: %s\n", filepath.Base(e.SumFile), w)
		} else {
			fmt.Fprintf(&b, "\t%s: %s\n", strings.Repeat(" ", len(filepath.Base(e.SumFile))), w)
		}
	}
	if e.Cached != "" {
		b.WriteString("\nThe module cache does not have what gocpp.sum recorded for this version.\n" +
			"Either the cache was changed, or gocpp.sum was edited or merged with another one.\n" +
			"Nothing was used. Remove the module from the cache (gocpp clean -modcache) to download it again;\n" +
			"if it still does not match, the code at that version has changed.")
	} else {
		b.WriteString("\nThis download does not match the checksum that gocpp.sum recorded the first time.\n" +
			"A tag that was moved, or a host or mirror that serves other files, would explain it.\n" +
			"Nothing was added to the module cache. Do not ignore this: someone may have changed the code.\n" +
			"If you know the change is right (you asked the author), remove the lines of this module from\n" +
			"gocpp.sum and run: gocpp mod download " + e.Path)
	}
	return b.String()
}

// saveSums merges the lines in add into the gocpp.sum file as it is on
// disk now (another gocpp may have written it), and writes the result
// atomically. It does nothing if the file has the lines already.
func (f *Fetcher) saveSums(sumPath string, add []SumLine) (changed bool, err error) {
	if len(add) == 0 {
		return false, nil
	}
	if err := CheckWritable(sumPath); err != nil {
		return false, err
	}
	unlock, err := f.lockFile(context.Background(), "sum-"+hashName(sumPath))
	if err != nil {
		return false, err
	}
	defer unlock()
	s := &Sums{Name: sumPath, by: map[string][]string{}}
	data, err := readSmall(sumPath)
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return false, err
	default:
		if err := s.parse(data); err != nil {
			return false, err
		}
	}
	for _, l := range add {
		if s.add(l) {
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	mode := os.FileMode(0o644)
	if st, err := os.Stat(sumPath); err == nil {
		mode = st.Mode().Perm()
	}
	sweepStaleTemps(filepath.Dir(sumPath))
	tmp, err := os.CreateTemp(filepath.Dir(sumPath), ".gocpp.sum-*")
	if err != nil {
		return false, fmt.Errorf("can't update %s: %v", SumFileName, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(s.format()); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmp.Name(), sumPath); err != nil {
		return false, fmt.Errorf("can't update %s: %v", SumFileName, err)
	}
	syncDir(filepath.Dir(sumPath))
	return true, nil
}

// staleTempRE matches the names of the temporary files that an atomic write of
// gocpp.mod, gocpp.sum or another project file makes (os.CreateTemp adds digits).
var staleTempRE = regexp.MustCompile(`^\.gocpp(\.sum|\.mod)?-[0-9]+$`)

// sweepStaleTemps removes the temporary files that a killed gocpp left in the
// project folder: regular files with such a name that are more than a day old (a
// running gocpp renames its file within a moment). Failures are ignored.
func sweepStaleTemps(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !staleTempRE.MatchString(e.Name()) {
			continue
		}
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() && time.Since(info.ModTime()) > 24*time.Hour {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
