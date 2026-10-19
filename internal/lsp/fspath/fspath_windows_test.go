//go:build windows

package fspath

import (
	"path/filepath"
	"testing"
)

// TestWindowsNamesLikeFilepath: on a real Windows, the Windows flavor gives what path/filepath gives, for the names
// where it means to: it also takes the \\?\ prefix off, which filepath keeps, and it does not know the half-names `\\`,
// `\\srv` and `\\srv\` or the device paths `\\.\C:\x` (no file URI makes one): those are left out.
func TestWindowsNamesLikeFilepath(t *testing.T) {
	names := []string{
		``, `.`, `..`, `a`, `a\b`, `a/b/../c`, `\`, `\a\b`, `/a/b`, `C:`, `C:\`, `C:/`, `C:a`, `C:a\b`, `C:\a`, `C:\a\`, `C:\a\..\..\b`,
		`c:/Users/me//proj/./x`, `C:\a b\c d.gp`, `\\srv\share`, `\\srv\share\`, `\\srv\share\d`, `\\srv\share\d\..\e`, `//srv/share/d`,
		`\\srv\share\x\y.gp`, `C:\日本\x`, `C:\a\.\b\..\c`, `C:\a\\b`,
	}
	f := Windows
	for _, n := range names {
		if got, want := f.Clean(n), filepath.Clean(n); got != want {
			t.Errorf("Clean(%q) = %q, filepath says %q", n, got, want)
		}
		if got, want := f.Dir(n), filepath.Dir(n); got != want {
			t.Errorf("Dir(%q) = %q, filepath says %q", n, got, want)
		}
		if got, want := f.Base(n), filepath.Base(n); got != want {
			t.Errorf("Base(%q) = %q, filepath says %q", n, got, want)
		}
		if got, want := f.IsAbs(n), filepath.IsAbs(n); got != want {
			t.Errorf("IsAbs(%q) = %v, filepath says %v", n, got, want)
		}
		if got, want := f.Volume(n), filepath.VolumeName(n); got != want {
			t.Errorf("Volume(%q) = %q, filepath says %q", n, got, want)
		}
		if got, want := f.ToSlash(n), filepath.ToSlash(n); got != want {
			t.Errorf("ToSlash(%q) = %q, filepath says %q", n, got, want)
		}
	}
	for _, c := range [][]string{{`C:\a`, `b`}, {`C:`, `a`}, {`C:\`, `a`, `..`, `b`}, {`a`, ``, `b`}, {`\\srv\share`, `d`}, {``, ``}, {`C:\a`, `\b`}} {
		if got, want := f.Join(c...), filepath.Join(c...); got != want {
			t.Errorf("Join(%q) = %q, filepath says %q", c, got, want)
		}
	}
}
