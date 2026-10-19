package modfetch

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// An ExtractInfo is what Extract learned from an archive.
type ExtractInfo struct {
	// Root is the single top-level folder of the archive (the
	// "<repo>-<ref>" folder of a git host's archive), inside the target folder.
	Root string

	// Commit is the commit that a tar archive made by "git archive" names in
	// its global header; "" if there is none.
	Commit string

	// Time is the latest modification time of an entry (UTC): for the
	// archive of a git host, the time of the commit.
	Time time.Time

	// Zip is true if the archive was a zip file (a module proxy's zips have their
	// files in <module path>@<version>/).
	Zip bool

	Entries int   // files and folders
	Bytes   int64 // the bytes of the files
}

// An ExtractError says that an archive is not safe or not valid. The module
// is not used.
type ExtractError struct {
	Entry string // the name of the entry in the archive, if the error is about one
	Msg   string
}

func (e *ExtractError) Error() string {
	if e.Entry == "" {
		return "unsafe or invalid archive: " + e.Msg
	}
	return fmt.Sprintf("unsafe or invalid archive: %q: %s", e.Entry, e.Msg)
}

func bad(entry, format string, args ...any) error {
	return &ExtractError{Entry: entry, Msg: fmt.Sprintf(format, args...)}
}

// Extract unpacks the tar.gz or zip archive file into the empty folder dst,
// which the caller made (mode 0700) and which nothing else uses. The format is
// found from the first bytes. The archive must have one top-level folder, and
// only files and folders in it. It is an error (ExtractError) if an entry
//
//   - has an absolute name or ".." in it, a backslash, a NUL, a control character,
//     a name that is not UTF-8, a path element ".git", or a name that is too long or deep;
//   - is a symbolic link, a hard link, a device, a FIFO or anything but a file or a folder
//     (there is no safe subset worth the risk: links may point outside the tree);
//   - names the same path as another entry, or one that differs only in case, or a file
//     where another entry needs a folder;
//   - or if there are more entries, more bytes in a file, or more bytes in all the files
//     than the limits allow.
//
// Every tar header counts (a repeated folder header, a "./" entry and a pax header too),
// and so does every byte that the tar stream is made of beyond the files' bytes, so an
// archive can't make gocpp work for long with a small download: the number of headers,
// the decompressed size and the size of the headers' own data are all bounded. The
// context is checked for every entry; a cancelled context stops the extraction.
//
// A file is created with the mode 0600, or 0700 if the archive says that it is
// executable: the caller makes the tree read-only with seal.
func Extract(ctx context.Context, archive, dst string, lim Limits) (*ExtractInfo, error) {
	f, err := os.Open(archive)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var magic [4]byte
	n, _ := io.ReadFull(f, magic[:])
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	x := &extractor{ctx: ctx, dst: dst, lim: lim, seen: make(map[string]entryKind), info: &ExtractInfo{}}
	switch {
	case n >= 2 && magic[0] == 0x1f && magic[1] == 0x8b:
		err = x.tarGz(f)
	case n >= 4 && string(magic[:2]) == "PK" && (magic[2] == 3 || magic[2] == 5):
		st, serr := f.Stat()
		if serr != nil {
			return nil, serr
		}
		x.info.Zip = true
		err = x.zipFile(f, st.Size())
	default:
		err = bad("", "not a tar.gz or zip file")
	}
	if err != nil {
		return nil, err
	}
	if x.top == "" {
		return nil, bad("", "the archive is empty")
	}
	x.info.Root = filepath.Join(dst, x.top)
	if st, err := os.Lstat(x.info.Root); err != nil || !st.IsDir() {
		return nil, bad(x.top, "the archive has a file at the top level, not a single folder (a git host's archive has one folder, named <repo>-<ref>)")
	}
	return x.info, nil
}

type entryKind int8

const (
	kindDir entryKind = iota + 1
	kindFile
)

type extractor struct {
	ctx   context.Context
	dst   string
	lim   Limits
	top   string
	seen  map[string]entryKind // by lower-case name, then exact name in names
	exact map[string]string
	info  *ExtractInfo
}

// testEntryHook, if set, is called for every entry (tests make a context end
// in the middle of an extraction with it).
var testEntryHook func()

const (
	// tarOverheadSlack and tarOverheadPerHeader bound the bytes of a tar stream that
	// are not file content: headers, padding, pax and GNU long name records.
	tarOverheadSlack     = 1 << 20
	tarOverheadPerHeader = 4096

	maxNameLen  = 1024
	maxElemLen  = 255
	maxDepth    = 64
	maxTailSize = 64 << 20 // bytes after the end of a tar archive
)

// cleanName returns the slash separated name of an archive entry, with
// "." and empty elements and trailing slashes removed, or an error. "" is the
// top of the archive ("./").
func cleanName(raw string) (string, error) {
	if raw == "" {
		return "", bad(raw, "empty name")
	}
	if len(raw) > maxNameLen {
		return "", bad(raw[:64]+"...", "name is too long (%d bytes, the limit is %d)", len(raw), maxNameLen)
	}
	if !utf8.ValidString(raw) {
		return "", bad(strings.ToValidUTF8(raw, "?"), "name is not valid UTF-8")
	}
	if strings.HasPrefix(raw, "/") {
		return "", bad(raw, "absolute path")
	}
	for _, r := range raw {
		switch {
		case r == '\\':
			return "", bad(raw, "backslash in the name")
		case r < 0x20 || r == 0x7f:
			return "", bad(fmt.Sprintf("%q", raw), "control character in the name")
		}
	}
	var out []string
	for _, e := range strings.Split(raw, "/") {
		switch {
		case e == "" || e == ".":
			continue
		case e == "..":
			return "", bad(raw, "path traversal (..)")
		case len(e) > maxElemLen:
			return "", bad(raw[:64]+"...", "path element is too long")
		}
		if why := checkElement(e); why != "" {
			return "", bad(raw, "%s", why)
		}
		out = append(out, e)
	}
	if len(out) > maxDepth {
		return "", bad(raw, "path is too deep")
	}
	return strings.Join(out, "/"), nil
}

// register records that the entry name exists as kind, with its parent
// folders, and reports a clash with an earlier entry.
func (x *extractor) register(name string, kind entryKind) (existed bool, err error) {
	if x.exact == nil {
		x.exact = make(map[string]string)
	}
	// Parents first (each prefix once: not a recursion, which would take
	// 2^depth steps for a deep name).
	for i := 0; i < len(name); i++ {
		if name[i] == '/' {
			if _, err := x.registerOne(name[:i], kindDir); err != nil {
				return false, err
			}
		}
	}
	return x.registerOne(name, kind)
}

func (x *extractor) registerOne(name string, kind entryKind) (existed bool, err error) {
	key := foldName(name)
	if old, ok := x.seen[key]; ok {
		if x.exact[key] != name {
			return false, bad(name, "differs only in case from %q: it would clash on a case-insensitive file system", x.exact[key])
		}
		if old == kindDir && kind == kindDir {
			return true, nil
		}
		if old != kind {
			return false, bad(name, "is both a file and a folder")
		}
		return false, bad(name, "appears twice in the archive")
	}
	x.seen[key] = kind
	x.exact[key] = name
	x.info.Entries++
	if x.info.Entries > x.lim.MaxFiles {
		return false, bad("", "more than %d entries", x.lim.MaxFiles)
	}
	return false, nil
}

// prepare checks an entry name and keeps the top-level folder.
func (x *extractor) prepare(raw string, kind entryKind) (name string, skip bool, err error) {
	name, err = cleanName(raw)
	if err != nil {
		return "", false, err
	}
	if name == "" {
		return "", true, nil
	}
	top, _, nested := strings.Cut(name, "/")
	if x.top == "" {
		x.top = top
	} else if top != x.top {
		return "", false, bad(raw, "the archive has more than one top-level entry (%q and %q); it must be one folder", x.top, top)
	}
	if !nested && kind == kindFile {
		return "", false, bad(raw, "a file at the top level: the archive must be one folder (named <repo>-<ref>)")
	}
	return name, false, nil
}

func (x *extractor) mkdir(name string) error {
	if existed, err := x.register(name, kindDir); err != nil || existed {
		return err
	}
	return os.MkdirAll(filepath.Join(x.dst, filepath.FromSlash(name)), 0o700)
}

func (x *extractor) file(name string, size int64, exec bool, mtime time.Time, r io.Reader) error {
	if size < 0 || size > x.lim.MaxFile {
		return bad(name, "file is too large (%d bytes, the limit is %d)", size, x.lim.MaxFile)
	}
	if x.info.Bytes+size > x.lim.MaxTree {
		return bad(name, "the files are more than %d bytes in all", x.lim.MaxTree)
	}
	if _, err := x.register(name, kindFile); err != nil {
		return err
	}
	path := filepath.Join(x.dst, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	mode := fs.FileMode(0o600)
	if exec {
		mode = 0o700
	}
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	// The size in the header is not trusted: copy at most size bytes, and
	// at least that many must be there.
	n, err := io.Copy(out, io.LimitReader(r, size+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if n != size {
		return bad(name, "the file has %d bytes, its header says %d", n, size)
	}
	x.info.Bytes += n
	if mtime.After(x.info.Time) {
		x.info.Time = mtime
	}
	return nil
}

// A countReader counts the bytes that it reads, and fails after limit.
type countReader struct {
	r     io.Reader
	n     int64
	limit int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.limit {
		return n, bad("", "the decompressed archive is larger than %d bytes (a decompression bomb?)", c.limit)
	}
	return n, err
}

func (x *extractor) tarGz(r io.Reader) error {
	gz, err := gzip.NewReader(bufio.NewReaderSize(r, 64<<10))
	if err != nil {
		return bad("", "not a gzip file: %v", err)
	}
	// The decompressed stream is at most the files, a header for each entry and
	// the data of those headers.
	maxHeaders := int64(x.lim.MaxFiles) + 1000
	cr := &countReader{r: gz, limit: x.lim.MaxTree + tarOverheadSlack + tarOverheadPerHeader*maxHeaders}
	tr := tar.NewReader(cr)
	var headers int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return truncated(err)
		}
		if err := x.ctx.Err(); err != nil {
			return err
		}
		if testEntryHook != nil {
			testEntryHook()
		}
		if headers++; headers > maxHeaders {
			return bad("", "more than %d tar headers (repeated entries count)", maxHeaders)
		}
		if over := cr.n - x.info.Bytes; over > tarOverheadSlack+tarOverheadPerHeader*headers {
			return bad("", "the tar headers have %d bytes of data in them, more than the allowed %d", over, tarOverheadSlack+tarOverheadPerHeader*headers)
		}
		switch hdr.Typeflag {
		case tar.TypeXGlobalHeader:
			if c := hdr.PAXRecords["comment"]; isHex40(c) {
				x.info.Commit = c
			}
		case tar.TypeDir:
			name, skip, err := x.prepare(hdr.Name, kindDir)
			if err != nil {
				return err
			}
			if skip {
				continue
			}
			if hdr.ModTime.After(x.info.Time) {
				x.info.Time = hdr.ModTime
			}
			if err := x.mkdir(name); err != nil {
				return err
			}
		case tar.TypeReg:
			name, skip, err := x.prepare(hdr.Name, kindFile)
			if err != nil {
				return err
			}
			if skip {
				return bad(hdr.Name, "a file with no name")
			}
			if err := x.file(name, hdr.Size, hdr.Mode&0o111 != 0, hdr.ModTime, tr); err != nil {
				return truncated(err)
			}
		case tar.TypeSymlink:
			return bad(hdr.Name, "is a symbolic link (to %q): links in modules are not allowed", hdr.Linkname)
		case tar.TypeLink:
			return bad(hdr.Name, "is a hard link (to %q): links in modules are not allowed", hdr.Linkname)
		case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			return bad(hdr.Name, "is a device or a FIFO")
		default:
			return bad(hdr.Name, "has the unsupported entry type %q", string(hdr.Typeflag))
		}
	}
	// Read to the end of the gzip stream so that its checksum is checked: a
	// download that was cut off fails here at the latest.
	n, err := io.Copy(io.Discard, io.LimitReader(cr, maxTailSize+1))
	if err != nil {
		return truncated(err)
	}
	if n > maxTailSize {
		return bad("", "more than %d bytes after the end of the archive", maxTailSize)
	}
	return nil
}

// truncated turns the errors of a cut-off or damaged download into one that
// says so.
func truncated(err error) error {
	var ee *ExtractError
	if errors.As(err, &ee) {
		return err
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return fmt.Errorf("the archive is truncated (the download was cut off): %v", err)
	}
	return fmt.Errorf("the archive is damaged: %v", err)
}

func (x *extractor) zipFile(r io.ReaderAt, size int64) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return bad("", "not a valid zip file: %v", err)
	}
	if len(zr.File) > x.lim.MaxFiles {
		return bad("", "more than %d entries", x.lim.MaxFiles)
	}
	for _, zf := range zr.File {
		if err := x.ctx.Err(); err != nil {
			return err
		}
		if testEntryHook != nil {
			testEntryHook()
		}
		mode := zf.Mode()
		isDir := mode.IsDir() || strings.HasSuffix(zf.Name, "/")
		switch {
		case mode&fs.ModeSymlink != 0:
			return bad(zf.Name, "is a symbolic link: links in modules are not allowed")
		case !isDir && !mode.IsRegular():
			return bad(zf.Name, "is not a regular file or a folder")
		}
		kind := kindFile
		if isDir {
			kind = kindDir
		}
		name, skip, err := x.prepare(zf.Name, kind)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if zf.Modified.After(x.info.Time) {
			x.info.Time = zf.Modified.UTC()
		}
		if isDir {
			if err := x.mkdir(name); err != nil {
				return err
			}
			continue
		}
		if zf.UncompressedSize64 > uint64(x.lim.MaxFile) {
			return bad(zf.Name, "file is too large (%d bytes, the limit is %d)", zf.UncompressedSize64, x.lim.MaxFile)
		}
		rc, err := zf.Open()
		if err != nil {
			return bad(zf.Name, "%v", err)
		}
		err = x.file(name, int64(zf.UncompressedSize64), mode&0o111 != 0, zf.Modified, rc)
		if cerr := rc.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return truncated(err)
		}
	}
	return nil
}

func isHex40(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}

// Seal makes the tree read-only: files get the mode 0444 (0555 if the owner may
// execute them), folders 0555. Nothing keeps a setuid, setgid or sticky bit.
// It does not follow links (there are none).
func Seal(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			dirs = append(dirs, p)
		case d.Type().IsRegular():
			st, err := d.Info()
			if err != nil {
				return err
			}
			mode := fs.FileMode(0o444)
			if st.Mode()&0o100 != 0 {
				mode = 0o555
			}
			return os.Chmod(p, mode)
		default:
			return fmt.Errorf("%s: unexpected file type %v", p, d.Type())
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i], 0o555); err != nil {
			return err
		}
	}
	return nil
}

// RemoveAll removes a tree that Seal made read-only (folders without write
// permission), which os.RemoveAll can't.
func RemoveAll(path string) error {
	filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			os.Chmod(p, 0o700)
		}
		return nil
	})
	return os.RemoveAll(path)
}
