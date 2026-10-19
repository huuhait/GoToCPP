package toolchain

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// unpackZip writes the zip archive r (size bytes) into root, without its top-level folder, by the
// rules of unpack: files and folders only (a zip entry that is a symbolic link, a device or the like
// is refused: the Windows archive of llvm-mingw has none), no absolute or ".." path, no
// backslash, one top-level folder, everything below the 8 GiB limit, written through os.Root.
func unpackZip(r io.ReaderAt, size int64, root string) error {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return err
	}
	rt, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer rt.Close()
	var total int64
	top := ""
	for _, zf := range zr.File {
		name, first, err := stripTop(zf.Name)
		if err != nil {
			return err
		}
		if top == "" {
			top = first
		} else if first != top {
			return fmt.Errorf("%s: the archive has more than one top-level folder (%s, %s)", zf.Name, top, first)
		}
		if name == "" {
			continue
		}
		mode := zf.Mode()
		switch {
		case zf.FileInfo().IsDir():
			if err := rt.MkdirAll(name, 0o755); err != nil {
				return err
			}
		case mode.IsRegular():
			total += int64(zf.UncompressedSize64)
			if total > maxUnpacked {
				return errors.New("the archive unpacks to more than 8 GiB")
			}
			if err := rt.MkdirAll(path.Dir(name), 0o755); err != nil {
				return err
			}
			perm := fs.FileMode(0o644)
			if mode&0o111 != 0 {
				perm = 0o755
			}
			if err := writeZipFile(rt, zf, name, perm); err != nil {
				return fmt.Errorf("%s: %w", zf.Name, err)
			}
		default:
			return fmt.Errorf("%s: entry type %v is not accepted (only files and folders)", zf.Name, mode.Type())
		}
	}
	if top == "" {
		return errors.New("the archive is empty")
	}
	return nil
}

func writeZipFile(rt *os.Root, zf *zip.File, name string, perm fs.FileMode) error {
	src, err := zf.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	f, err := rt.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	// archive/zip checks the size and the CRC of the entry as it is read (a header that lies gives an error at the end of the
	// entry: TestUnpackZipLyingSize); the limit only bounds what is written before that.
	_, err = io.Copy(f, io.LimitReader(src, int64(zf.UncompressedSize64)+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// zipName reports whether a file name is a zip archive's.
func zipName(name string) bool { return strings.HasSuffix(name, ".zip") }
