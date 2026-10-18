package modfetch

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// File names in a module must be the same, and safe, on every system: the hash
// of a module is the same on Linux and on Windows, so a name that only Windows or
// macOS would treat in a special way (as Go's module.CheckFilePath does) is
// refused everywhere. A library with such a file name can't be downloaded: vendor a
// fixed copy of it instead.

var (
	shortGit = regexp.MustCompile(`^git~[0-9]+$`) // the 8.3 short name of .git on NTFS
	reserved = map[string]bool{"con": true, "prn": true, "aux": true, "nul": true, "conin$": true, "conout$": true}
)

func init() {
	for i := '1'; i <= '9'; i++ {
		reserved["com"+string(i)] = true
		reserved["lpt"+string(i)] = true
	}
	for _, sup := range []string{"¹", "²", "³"} {
		reserved["com"+sup] = true
		reserved["lpt"+sup] = true
	}
}

// foldName returns the name as a case-insensitive file system sees it, for the
// cases that matter: format characters that HFS+ and NTFS ignore (zero width
// joiners, bidirectional marks, the byte order mark) are removed, and every
// letter is folded to a common case in a way that joins "s" and the long s, "k"
// and the Kelvin sign.
func foldName(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return unicode.ToLower(unicode.ToUpper(r))
	}, s)
}

// checkElement returns the reason why a path element of an archive is not
// allowed on every system, or "".
func checkElement(e string) string {
	switch {
	case strings.Contains(e, ":"):
		return "a colon in the name (an NTFS stream or a drive)"
	case strings.HasSuffix(e, ".") || strings.HasSuffix(e, " "):
		return "the name ends with a dot or a space (Windows ignores it)"
	}
	for _, r := range e {
		if r >= 0x0300 && r <= 0x036f {
			return "a combining mark in the name (the composed and the decomposed form of a name are one file on macOS)"
		}
	}
	f := foldName(e)
	switch {
	case f == ".git" || strings.TrimLeft(f, ". ") == "git" && strings.HasPrefix(f, "."):
		return "a .git folder in a module archive"
	case shortGit.MatchString(f):
		return "the short name of a .git folder on Windows"
	}
	base := f
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimRight(base, " ")
	if reserved[base] {
		return fmt.Sprintf("%q is a reserved device name on Windows", strings.ToUpper(base))
	}
	return ""
}
