package modfetch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func extractTo(t *testing.T, archive []byte, lim Limits) (string, *ExtractInfo, error) {
	t.Helper()
	dir := t.TempDir()
	arc := filepath.Join(dir, "a")
	if err := os.WriteFile(arc, archive, 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "out")
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	info, err := Extract(context.Background(), arc, dst, lim)
	return dst, info, err
}

func TestExtractGood(t *testing.T) {
	for name, arc := range map[string][]byte{
		"tar.gz": tarGz(testCommit, library("lib-1")...),
		"zip":    zipOf(library("lib-1")...),
	} {
		t.Run(name, func(t *testing.T) {
			dst, info, err := extractTo(t, arc, DefaultLimits)
			if err != nil {
				t.Fatal(err)
			}
			if info.Root != filepath.Join(dst, "lib-1") || info.Entries != 5 || !info.Time.Equal(testTime) {
				t.Errorf("info = %+v", info)
			}
			if name == "tar.gz" && info.Commit != testCommit {
				t.Errorf("commit = %q", info.Commit)
			}
			if got := readTree(t, info.Root); len(got) != 3 {
				t.Errorf("tree = %v", got)
			}
		})
	}
}

func TestExtractRejects(t *testing.T) {
	top := func(more ...tfile) []tfile {
		return append([]tfile{dirEntry("t/"), {Name: "t/a.txt", Body: "a"}}, more...)
	}
	cases := []struct {
		name  string
		files []tfile
		want  string
	}{
		{"traversal", top(tfile{Name: "t/../../escape", Body: "x"}), "path traversal"},
		{"traversal at the start", []tfile{{Name: "../escape", Body: "x"}}, "path traversal"},
		{"traversal in the middle", top(tfile{Name: "t/a/../../../x", Body: "x"}), "path traversal"},
		{"absolute", []tfile{{Name: "/etc/passwd", Body: "x"}}, "absolute path"},
		{"backslash", top(tfile{Name: `t\..\x`, Body: "x"}), "backslash"},
		{"newline", top(tfile{Name: "t/a\nb", Body: "x"}), "control character"},
		{"git folder", top(tfile{Name: "t/.git/config", Body: "x"}), ".git folder"},
		{"git folder case", top(tfile{Name: "t/sub/.GIT/hooks", Body: "x"}), ".git folder"},
		{"symlink", top(tfile{Name: "t/link", Type: tar.TypeSymlink, Linkname: "a.txt"}), "symbolic link"},
		{"symlink out", top(tfile{Name: "t/link", Type: tar.TypeSymlink, Linkname: "/etc"}), "symbolic link"},
		{"symlink dotdot", top(tfile{Name: "t/link", Type: tar.TypeSymlink, Linkname: "../.."}), "symbolic link"},
		{"hardlink", top(tfile{Name: "t/hard", Type: tar.TypeLink, Linkname: "t/a.txt"}), "hard link"},
		{"char device", top(tfile{Name: "t/dev", Type: tar.TypeChar}), "device"},
		{"block device", top(tfile{Name: "t/dev", Type: tar.TypeBlock}), "device"},
		{"fifo", top(tfile{Name: "t/fifo", Type: tar.TypeFifo}), "FIFO"},
		{"top-level file", []tfile{{Name: "a.txt", Body: "x"}}, "top level"},
		{"two top folders", top(tfile{Name: "other/b.txt", Body: "x"}), "more than one top-level entry"},
		{"duplicate", top(tfile{Name: "t/a.txt", Body: "again"}), "appears twice"},
		{"case collision", top(tfile{Name: "t/A.txt", Body: "x"}), "differs only in case"},
		{"case collision folder", top(tfile{Name: "t/Sub/x", Body: "x"}, tfile{Name: "t/sub/y", Body: "y"}), "differs only in case"},
		{"file then folder", top(tfile{Name: "t/a.txt/inner", Body: "x"}), "both a file and a folder"},
		{"folder then file", []tfile{dirEntry("t/"), dirEntry("t/d/"), {Name: "t/d", Body: "x"}}, "both a file and a folder"},
		{"long name", top(tfile{Name: "t/" + strings.Repeat("x", 300), Body: "x"}), "too long"},
		{"deep", top(tfile{Name: "t/" + strings.Repeat("d/", 70) + "f", Body: "x"}), "too deep"},
		{"empty", nil, "empty"},
	}
	for _, tc := range cases {
		for _, format := range []string{"tar.gz", "zip"} {
			if format == "zip" {
				// A zip entry can't carry a hard link, a device or a FIFO.
				skip := false
				for _, f := range tc.files {
					switch f.Type {
					case tar.TypeLink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
						skip = true
					}
				}
				if skip {
					continue
				}
			}
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				var arc []byte
				if format == "zip" {
					arc = zipOf(tc.files...)
				} else {
					arc = tarGz("", tc.files...)
				}
				dst, _, err := extractTo(t, arc, DefaultLimits)
				var ee *ExtractError
				if !errors.As(err, &ee) || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v, want an ExtractError with %q", err, tc.want)
				}
				// Nothing is created outside the target folder.
				parent := filepath.Dir(dst)
				ents, _ := os.ReadDir(parent)
				for _, e := range ents {
					if e.Name() != "a" && e.Name() != "out" {
						t.Errorf("%s was created next to the target folder", e.Name())
					}
				}
			})
		}
	}
}

func TestExtractLimits(t *testing.T) {
	big := strings.Repeat("x", 1000)
	files := library("t", tfile{Name: "t/big.bin", Body: big})
	for _, tc := range []struct {
		name string
		lim  Limits
		want string
	}{
		{"file size", Limits{MaxFile: 500, MaxTree: 1 << 20, MaxFiles: 100}, "file is too large"},
		{"tree size", Limits{MaxFile: 1 << 20, MaxTree: 1000, MaxFiles: 100}, "more than 1000 bytes in all"},
		{"file count", Limits{MaxFile: 1 << 20, MaxTree: 1 << 20, MaxFiles: 3}, "more than 3 entries"},
	} {
		for _, format := range []string{"tar.gz", "zip"} {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				arc := tarGz("", files...)
				if format == "zip" {
					arc = zipOf(files...)
				}
				_, _, err := extractTo(t, arc, tc.lim)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v, want %q", err, tc.want)
				}
			})
		}
	}
}

// A tar header that says more bytes than the entry has (the size lies) and
// a decompression bomb are bounded by the limits.
func TestExtractLyingSizes(t *testing.T) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	tw.WriteHeader(&tar.Header{Name: "t/", Typeflag: tar.TypeDir, Mode: 0o755})
	tw.WriteHeader(&tar.Header{Name: "t/f", Typeflag: tar.TypeReg, Size: 10, Mode: 0o644})
	tw.Write([]byte("0123456789"))
	tw.Close()
	// Cut the tar in the middle of the file.
	b := raw.Bytes()
	cut := gzipBytes(b[:1024+5])
	if _, _, err := extractTo(t, cut, DefaultLimits); err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Errorf("a tar cut in a file: %v", err)
	}
	// 8 MiB of zeros compress to a few KiB: the limit stops it.
	var bomb bytes.Buffer
	gz := gzip.NewWriter(&bomb)
	tw = tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "t/", Typeflag: tar.TypeDir, Mode: 0o755})
	tw.WriteHeader(&tar.Header{Name: "t/zeros", Typeflag: tar.TypeReg, Size: 8 << 20, Mode: 0o644})
	zeros := make([]byte, 1<<20)
	for i := 0; i < 8; i++ {
		tw.Write(zeros)
	}
	tw.Close()
	gz.Close()
	_, _, err := extractTo(t, bomb.Bytes(), Limits{MaxFile: 1 << 20, MaxTree: 1 << 30, MaxFiles: 10})
	if err == nil || !strings.Contains(err.Error(), "file is too large") {
		t.Errorf("bomb: %v", err)
	}
}

func TestExtractGarbage(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":     nil,
		"text":      []byte("hello, this is not an archive"),
		"html":      []byte("<html>rate limited</html>"),
		"bad gzip":  {0x1f, 0x8b, 0x08, 0x00, 1, 2, 3},
		"bad zip":   []byte("PK\x03\x04garbage"),
		"gzip text": gzipBytes([]byte("not a tar")),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := extractTo(t, data, DefaultLimits); err == nil {
				t.Error("no error")
			}
		})
	}
}

func TestExtractTruncatedGzip(t *testing.T) {
	arc := tarGz("", library("t")...)
	for _, n := range []int{len(arc) - 1, len(arc) - 5, len(arc) / 2, 20} {
		_, _, err := extractTo(t, arc[:n], DefaultLimits)
		if err == nil {
			t.Errorf("a download cut at %d of %d bytes was accepted", n, len(arc))
		}
	}
}

// The permissions after Seal: no write bits, no setuid.
func TestSealModes(t *testing.T) {
	arc := tarGz("", library("t", tfile{Name: "t/suid", Body: "x", Mode: 0o4755}, tfile{Name: "t/w", Body: "x", Mode: 0o666})...)
	dst, info, err := extractTo(t, arc, DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if err := Seal(info.Root); err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dst)
	filepath.WalkDir(info.Root, func(p string, d fs.DirEntry, err error) error {
		st, _ := os.Lstat(p)
		m := st.Mode()
		if m.Perm()&0o222 != 0 || m&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 {
			t.Errorf("%s has the mode %v", p, m)
		}
		if st.Mode().IsRegular() && m.Perm()&0o111 != 0 && p == filepath.Join(info.Root, "w") {
			t.Errorf("%s is executable", p)
		}
		return nil
	})
	st, _ := os.Stat(filepath.Join(info.Root, "suid"))
	if st.Mode().Perm() != 0o555 {
		t.Errorf("suid file mode %v", st.Mode())
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{
		"a/b":       "a/b",
		"./a/b":     "a/b",
		"a//b/":     "a/b",
		"a/./b":     "a/b",
		"./":        "",
		".":         "",
		"a/b/c/":    "a/b/c",
		"..foo/bar": "..foo/bar",
	} {
		got, err := cleanName(in)
		if err != nil || got != want {
			t.Errorf("cleanName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestCleanNameRejects(t *testing.T) {
	for in, want := range map[string]string{
		"a\x00b":                  "control character",
		"t/\xff\xfe":              "UTF-8",
		"a/..":                    "path traversal",
		"..":                      "path traversal",
		"/abs":                    "absolute",
		"a\\b":                    "backslash",
		"\x7f":                    "control character",
		"":                        "empty",
		"a/.git/x":                ".git",
		strings.Repeat("a/", 600): "too long",
	} {
		_, err := cleanName(in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("cleanName(%q) = %v, want an error with %q", in, err, want)
		}
	}
}

// A deep path took 2^depth steps once (the fuzzer found it): the test would not end.
func TestExtractDeepPath(t *testing.T) {
	deep := "t/" + strings.Repeat("d/", 60) + "f"
	dst, info, err := extractTo(t, tarGz("", dirEntry("t/"), tfile{Name: deep, Body: "x"}), DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := readTree(t, info.Root); len(got) != 1 {
		t.Errorf("tree = %v", got)
	}
	RemoveAll(dst)
}
