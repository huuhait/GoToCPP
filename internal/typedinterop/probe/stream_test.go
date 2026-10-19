package probe

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
)

func scanAll(t *testing.T, r io.Reader, max int) ([]RawDecl, error) {
	t.Helper()
	var out []RawDecl
	_, err := ScanDecls(r, max, func(d RawDecl) error {
		d.Raw = append([]byte(nil), d.Raw...)
		out = append(out, d)
		return nil
	})
	return out, err
}

func TestScanDecls(t *testing.T) {
	obj := func(name string, pad string) string {
		return fmt.Sprintf(`{"id":"0x1","kind":"TypeAliasDecl","name":%q,"inner":[{"x":"%s"}]}`, name, pad)
	}
	cases := []struct {
		name  string
		input string
		names []string
		err   error
	}{
		{"empty", "", nil, nil},
		{"only space", " \n\t ", nil, nil},
		{"one", obj("gp_probe_0", ""), []string{"gp_probe_0"}, nil},
		{"two with newlines", obj("gp_probe_0", "") + "\n" + obj("gp_probe_1", "") + "\n", []string{"gp_probe_0", "gp_probe_1"}, nil},
		{"leading space", "\n  " + obj("a", ""), []string{"a"}, nil},
		{"braces in strings", obj("gp_probe_2", `}{][ \" }`), []string{"gp_probe_2"}, nil},
		{"escaped backslash before quote", obj("q", `\\`) + obj("r", ""), []string{"q", "r"}, nil},
		{"nested arrays", `{"name":"n","inner":[[1,2,[3]],{"a":{"b":[{}]}}]}`, []string{"n"}, nil},
		{"junk", "garbage", nil, ErrBadDump},
		{"truncated object", `{"name":"x","inner":[`, nil, ErrBadDump},
		{"junk after object", obj("a", "") + "xyz", []string{"a"}, ErrBadDump},
		{"name with escape", `{"name":"a\"b"}`, []string{`a"b`}, nil},
		{"no name", `{"kind":"X"}`, []string{""}, nil},
	}
	for _, tc := range cases {
		for _, mode := range []string{"whole", "one byte"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				var r io.Reader = strings.NewReader(tc.input)
				if mode == "one byte" {
					r = iotest.OneByteReader(r)
				}
				got, err := scanAll(t, r, 0)
				if !errors.Is(err, tc.err) {
					t.Fatalf("error %v, want %v", err, tc.err)
				}
				var names []string
				for _, d := range got {
					names = append(names, d.Name)
					if !strings.HasPrefix(string(d.Raw), "{") || !strings.HasSuffix(string(d.Raw), "}") || int64(len(d.Raw)) != d.Size {
						t.Errorf("raw %q size %d", d.Raw, d.Size)
					}
				}
				if strings.Join(names, ",") != strings.Join(tc.names, ",") {
					t.Fatalf("names %q, want %q", names, tc.names)
				}
			})
		}
	}
}

func TestScanDeclsTooLarge(t *testing.T) {
	big := `{"name":"big","inner":["` + strings.Repeat("x", 5000) + `"]}`
	small := `{"name":"small"}`
	got, err := scanAll(t, strings.NewReader(big+small), 1000)
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}
	if !got[0].TooLarge || got[0].Raw != nil || got[0].Name != "big" || got[0].Size != int64(len(big)) {
		t.Errorf("big: %+v", got[0])
	}
	if got[1].TooLarge || got[1].Name != "small" {
		t.Errorf("small: %+v", got[1])
	}
}

func TestScanDeclsErrorStops(t *testing.T) {
	stop := errors.New("stop")
	n, err := ScanDecls(strings.NewReader(`{"name":"a"}{"name":"b"}{"name":"c"}`), 0, func(d RawDecl) error {
		if d.Name == "b" {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || n != 2 {
		t.Fatalf("n %d err %v", n, err)
	}
}

type failingReader struct{ n int }

func (f *failingReader) Read(p []byte) (int, error) {
	if f.n == 0 {
		return 0, errors.New("disk on fire")
	}
	f.n--
	return copy(p, `{"name":"a","x":[`), nil
}

func TestScanDeclsReadError(t *testing.T) {
	_, err := scanAll(t, &failingReader{n: 2}, 0)
	if err == nil || !strings.Contains(err.Error(), "disk on fire") {
		t.Fatalf("error %v", err)
	}
}

// genReader makes a stream of n objects of about size bytes each, without
// holding it in memory and (almost) without allocating.
type genReader struct {
	remaining int
	size      int
	i         int
	head      []byte
	padLeft   int
	tail      []byte
	pad       []byte
}

func newGenReader(n, size int) *genReader {
	return &genReader{remaining: n, size: size, pad: []byte(strings.Repeat("y", 4096))}
}

func (g *genReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		switch {
		case len(g.head) > 0:
			c := copy(p[n:], g.head)
			g.head = g.head[c:]
			n += c
		case g.padLeft > 0:
			c := copy(p[n:], g.pad[:min(len(g.pad), g.padLeft)])
			g.padLeft -= c
			n += c
		case len(g.tail) > 0:
			c := copy(p[n:], g.tail)
			g.tail = g.tail[c:]
			n += c
		case g.remaining > 0:
			g.remaining--
			g.i++
			g.head = []byte(fmt.Sprintf(`{"id":"0x%x","kind":"TypeAliasDecl","name":"gp_probe_%d","inner":[{"v":"`, g.i, g.i))
			g.padLeft = max(0, g.size-len(g.head))
			g.tail = []byte(`"}]}` + "\n")
		default:
			if n == 0 {
				return 0, io.EOF
			}
			return n, nil
		}
	}
	return n, nil
}

// TestScanDeclsBoundedMemory reads a synthetic dump of 200 MB, as one huge
// declaration and as many small ones, and checks that the reader's
// allocations don't grow with the input (the dump is never held).
func TestScanDeclsBoundedMemory(t *testing.T) {
	for _, tc := range []struct {
		name    string
		n, size int
		max     int
	}{
		{"many small", 25600, 8 << 10, 0},            // 200 MB in 8 KB objects
		{"one huge", 1, 200 << 20, 1 << 20},          // 200 MB in one object, 1 MB limit
		{"huge between small", 4, 50 << 20, 4 << 20}, // 200 MB in four objects of 50 MB
	} {
		t.Run(tc.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			var count int
			var total int64
			n, err := ScanDecls(newGenReader(tc.n, tc.size), tc.max, func(d RawDecl) error {
				count++
				total += d.Size
				if d.Name == "" {
					t.Errorf("no name")
				}
				return nil
			})
			runtime.ReadMemStats(&after)
			if err != nil || n != tc.n || count != tc.n {
				t.Fatalf("n=%d count=%d err=%v", n, count, err)
			}
			if total < int64(tc.n)*int64(tc.size) {
				t.Fatalf("read %d bytes, want at least %d", total, int64(tc.n)*int64(tc.size))
			}
			// Total allocation (not the heap: it counts what a copy of the dump would
			// have cost) stays far below the input size.
			limit := uint64(64 << 20)
			if tc.max > 0 && uint64(tc.max)*4 > limit {
				limit = uint64(tc.max) * 4
			}
			if alloc := after.TotalAlloc - before.TotalAlloc; alloc > limit+uint64(tc.n)*16<<10 {
				t.Errorf("allocated %d MB reading %d MB (limit %d MB)", alloc>>20, total>>20, limit>>20)
			}
		})
	}
}

func TestDeclNameFromPrefixOnly(t *testing.T) {
	// The name is found in the first bytes, also of an object that doesn't fit.
	big := `{"id":"1","kind":"K","name":"gp_probe_9","inner":["` + strings.Repeat("z", 100000) + `"]}`
	got, err := scanAll(t, strings.NewReader(big), 100)
	if err != nil || len(got) != 1 || got[0].Name != "gp_probe_9" || !got[0].TooLarge {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestDeclNameIsTheTopLevelKey(t *testing.T) {
	for in, want := range map[string]string{
		`{"id":"0x1","kind":"TypeAliasDecl","name":"gp_probe_3","type":{"qualType":"x"}}`:        "gp_probe_3",
		`{"id":"0x1","loc":{"name":"nested"},"kind":"VarDecl","name":"gp_probe_4_s"}`:            "gp_probe_4_s",
		`{"id":"0x1","kind":"name","range":{"note":"\"name\": \"fake\""},"name":"gp_probe_5_v"}`: "gp_probe_5_v",
		`{"inner":[{"name":"inner"}],"name":"gp_probe_6", "type":{}}`:                            "gp_probe_6",
		`{ "kind" : "VarDecl" , "name" : "gp_probe_7\"x" }`:                                      `gp_probe_7"x`,
		`{"id":"0x1","kind":"CXXRecordDecl"}`:                                                    "",
		`{"id":"0x1","kind":"CXXRecordDecl","inner":[{"kind":"FieldDecl","name":"member"}]`:      "",
	} {
		if got := declName([]byte(in)); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}
