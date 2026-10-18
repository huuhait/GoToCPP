package modfetch

import (
	"archive/tar"
	"context"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkExtract extracts arc with small limits and checks what the contract
// promises, whatever the bytes are: no error leaves a tree that has a link, a
// special file or a name outside the folder; nothing is written next to the
// target folder; the limits hold.
func checkExtract(t *testing.T, arc []byte) {
	t.Helper()
	parent, err := os.MkdirTemp("", "modfetch-fuzz-")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(parent)
	arcFile := filepath.Join(parent, "a")
	if err := os.WriteFile(arcFile, arc, 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(parent, "out")
	if err := os.Mkdir(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	lim := Limits{MaxFile: 1 << 16, MaxTree: 1 << 20, MaxFiles: 200}
	info, err := Extract(context.Background(), arcFile, dst, lim)
	ents, _ := os.ReadDir(parent)
	for _, e := range ents {
		if e.Name() != "a" && e.Name() != "out" {
			t.Fatalf("%q was created next to the target folder", e.Name())
		}
	}
	var total int64
	count := 0
	lower := map[string]string{}
	walkErr := filepath.WalkDir(dst, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == dst {
			return nil
		}
		rel, err := filepath.Rel(dst, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("%s is outside %s", p, dst)
		}
		if old, dup := lower[strings.ToLower(rel)]; dup {
			t.Fatalf("%q and %q differ only in case", old, rel)
		}
		lower[strings.ToLower(rel)] = rel
		if strings.EqualFold(d.Name(), ".git") {
			t.Fatalf("a .git entry was extracted: %s", p)
		}
		st, err := os.Lstat(p)
		if err != nil {
			return err
		}
		switch {
		case st.IsDir():
		case st.Mode().IsRegular():
			total += st.Size()
			if st.Size() > lim.MaxFile {
				t.Fatalf("%s is %d bytes", p, st.Size())
			}
		default:
			t.Fatalf("%s has the type %v", p, st.Mode())
		}
		count++
		return nil
	})
	if err != nil {
		if walkErr != nil && info != nil {
			t.Fatal(walkErr)
		}
		return
	}
	if total > lim.MaxTree || count > lim.MaxFiles {
		t.Fatalf("limits exceeded: %d bytes, %d entries", total, count)
	}
	// A tree that came out can be sealed and hashed.
	if info.Root == "" || !strings.HasPrefix(info.Root, dst) {
		t.Fatalf("root = %q", info.Root)
	}
	if err := Seal(info.Root); err != nil {
		t.Fatal(err)
	}
	if _, err := HashDir(context.Background(), info.Root, "example.com/m", "v1.0.0"); err != nil {
		t.Fatal(err)
	}
}

func FuzzExtractTar(f *testing.F) {
	f.Add(tarRaw(testCommit, library("t")...))
	f.Add(tarRaw("", dirEntry("t/"), tfile{Name: "t/../../x", Body: "x"}))
	f.Add(tarRaw("", dirEntry("t/"), tfile{Name: "t/l", Type: tar.TypeSymlink, Linkname: "/etc/passwd"}))
	f.Add(tarRaw("", dirEntry("t/"), tfile{Name: "t/h", Type: tar.TypeLink, Linkname: "x"}))
	f.Add(tarRaw("", dirEntry("t/"), tfile{Name: "t/a", Body: "1"}, tfile{Name: "t/A", Body: "2"}))
	f.Add(tarRaw("", dirEntry("t/"), tfile{Name: "t/a", Body: "1"}, tfile{Name: "t/a/b", Body: "2"}))
	f.Add(tarRaw("", tfile{Name: "/abs", Body: "1"}))
	f.Add(tarRaw("", dirEntry("t/"), tfile{Name: "t/.git/x", Body: "1"}))
	f.Add(tarRaw("", dirEntry("t/"), tfile{Name: "t/dev", Type: tar.TypeChar}))
	f.Fuzz(func(t *testing.T, data []byte) {
		checkExtract(t, gzipBytes(data))
	})
}

func FuzzExtractZip(f *testing.F) {
	f.Add(zipOf(library("t")...))
	f.Add(zipOf(dirEntry("t/"), tfile{Name: "t/../../x", Body: "x"}))
	f.Add(zipOf(dirEntry("t/"), tfile{Name: "t/l", Type: tar.TypeSymlink, Linkname: "/etc/passwd"}))
	f.Add(zipOf(dirEntry("t/"), tfile{Name: "t/a", Body: "1"}, tfile{Name: "t/A", Body: "2"}))
	f.Add(zipOf(tfile{Name: "/abs", Body: "1"}))
	f.Fuzz(func(t *testing.T, data []byte) {
		checkExtract(t, data)
	})
}

// FuzzExtractRaw feeds any bytes: gzip or zip headers are found by the
// fuzzer from the seeds.
func FuzzExtractRaw(f *testing.F) {
	f.Add(tarGz(testCommit, library("t")...))
	f.Add(zipOf(library("t")...))
	f.Add([]byte("PK\x03\x04"))
	f.Add([]byte{0x1f, 0x8b, 8})
	f.Fuzz(func(t *testing.T, data []byte) {
		checkExtract(t, data)
	})
}

func FuzzParseRefs(f *testing.F) {
	f.Add([]byte("001e# service=git-upload-pack\n0000" + "0032" + strings.Repeat("1", 40) + " HEAD\n" + "0000"))
	f.Fuzz(func(t *testing.T, data []byte) {
		parseRefs(data) // must not panic
	})
}

func FuzzSumsParse(f *testing.F) {
	f.Add([]byte("example.com/a v1.0.0 h1:" + strings.Repeat("A", 43) + "=\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		s := &Sums{by: map[string][]string{}}
		if s.parse(data) == nil {
			// What is read can be written and read again.
			s2 := &Sums{by: map[string][]string{}}
			if err := s2.parse(s.format()); err != nil {
				t.Fatalf("the output does not parse: %v\n%q", err, s.format())
			}
			if len(s2.Lines()) != len(s.Lines()) {
				t.Fatal("lines differ")
			}
		}
	})
}

func FuzzCleanName(f *testing.F) {
	for _, s := range []string{"a/b", "../x", "/x", "a//b", "a/./b", ".git/x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		got, err := cleanName(name)
		if err != nil {
			return
		}
		if strings.HasPrefix(got, "/") || strings.Contains(got, "//") || strings.Contains(got, "\\") || strings.HasSuffix(got, "/") {
			t.Fatalf("cleanName(%q) = %q", name, got)
		}
		for _, e := range strings.Split(got, "/") {
			if e == ".." || e == "." || strings.EqualFold(e, ".git") {
				t.Fatalf("cleanName(%q) = %q has the element %q", name, got, e)
			}
		}
	})
}

// TestExtractRandomArchives is a fuzz test that runs with every "go test": it
// makes thousands of archives from a list of hostile names and entry types (and
// by damaging valid archives) and checks the contract for each (checkExtract).
// The seed is fixed, so a failure repeats.
func TestExtractRandomArchives(t *testing.T) {
	rnd := rand.New(rand.NewSource(20261017))
	names := []string{"t/", "t/a", "t/A", "t/a/b", "t/a/", "t/b.h", "t/../x", "t/a/../../x", "../x", "/abs", "/", "./", "t//c", "t/./d",
		`t\e`, "t/.git/config", "t/.GIT", "t/sub/.git/x", "t/\x01ctl", "t/a\nb", "u/other", "t", "t/" + strings.Repeat("n", 300),
		strings.Repeat("d/", 70), "t/ünï/çödé", "t/a b", "T/x", "t/COM1", "t/..", "t/.", "..", "t/a/..", "t/a/b/../c"}
	types := []byte{tar.TypeReg, tar.TypeReg, tar.TypeReg, tar.TypeDir, tar.TypeDir, tar.TypeSymlink, tar.TypeLink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo}
	n := 600
	if testing.Short() {
		n = 100
	}
	for i := 0; i < n; i++ {
		var files []tfile
		count := 1 + rnd.Intn(8)
		for j := 0; j < count; j++ {
			f := tfile{Name: names[rnd.Intn(len(names))], Type: types[rnd.Intn(len(types))], Body: strings.Repeat("x", rnd.Intn(40)), Mode: int64(rnd.Intn(0o7777))}
			if f.Type == tar.TypeSymlink || f.Type == tar.TypeLink {
				f.Linkname = names[rnd.Intn(len(names))]
			}
			if j == 0 && rnd.Intn(4) != 0 {
				f.Name, f.Type = "t/", tar.TypeDir
			}
			files = append(files, f)
		}
		if raw, ok := tryTar(files); ok {
			checkExtract(t, gzipBytes(raw))
		}
		checkExtract(t, safeZip(files))
	}
	// Damaged valid archives: flip, cut, insert and delete bytes.
	seeds := [][]byte{tarGz(testCommit, library("t")...), zipOf(library("t")...), gzipBytes(tarRaw("", library("t")...))}
	for i := 0; i < n; i++ {
		b := append([]byte(nil), seeds[rnd.Intn(len(seeds))]...)
		for k := 1 + rnd.Intn(4); k > 0 && len(b) > 0; k-- {
			at := rnd.Intn(len(b))
			switch rnd.Intn(4) {
			case 0:
				b[at] ^= byte(1 << rnd.Intn(8))
			case 1:
				b = b[:at]
			case 2:
				b = append(b[:at], append([]byte{byte(rnd.Intn(256))}, b[at:]...)...)
			case 3:
				end := min(len(b), at+1+rnd.Intn(16))
				b = append(b[:at], b[end:]...)
			}
		}
		checkExtract(t, b)
	}
}

// tryTar makes a tar file, or reports that the tar package can't write one of
// the entries (a name with a NUL, say).
func tryTar(files []tfile) (raw []byte, ok bool) {
	defer func() {
		if recover() != nil {
			raw, ok = nil, false
		}
	}()
	return tarRaw("", files...), true
}

func safeZip(files []tfile) (b []byte) {
	defer func() {
		if recover() != nil {
			b = nil
		}
	}()
	return zipOf(files...)
}
