package xz

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"math/rand/v2"
	"os/exec"
	"strings"
	"testing"
)

// fixture is `printf 'gocpp xz fixture: hello hello hello hello, world world world\n' x8 | xz -9e -C
// crc64`: a file that needs no xz program to test.
const fixture = "/Td6WFoAAATm1rRGBMAz6AMhARwAAAAAAAAAAOOZYQrgAecAK10AM5vIjkbszNG2yWj1KgDGS4hSq9w3I54Xe8tI/edMf1TpT13TbmCFexDgAAAA5UdY3WzaRCUAAU/oAwAAAJRPSjexxGf7AgAAAAAEWVo="

const fixtureText = "gocpp xz fixture: hello hello hello hello, world world world\n"

func decode(t *testing.T, data []byte) ([]byte, error) {
	t.Helper()
	r, err := NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

func TestFixture(t *testing.T) {
	data, _ := base64.StdEncoding.DecodeString(fixture)
	got, err := decode(t, data)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Repeat(fixtureText, 8); string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Every damaged copy of the fixture fails (or, for a byte that no check covers, decodes to the same
// data): never a panic, never wrong data without an error.
func TestFixtureDamage(t *testing.T) {
	data, _ := base64.StdEncoding.DecodeString(fixture)
	want := strings.Repeat(fixtureText, 8)
	for i := range data {
		for _, flip := range []byte{0x01, 0x80, 0xFF} {
			bad := bytes.Clone(data)
			bad[i] ^= flip
			got, err := decode(t, bad)
			if err == nil && string(got) != want {
				t.Fatalf("byte %d xor %#x: no error, but the data differs: %q", i, flip, got)
			}
		}
	}
	for n := 0; n < len(data); n++ {
		if got, err := decode(t, data[:n]); err == nil {
			t.Fatalf("a file cut to %d of %d bytes decodes without error: %q", n, len(data), got)
		}
	}
}

func TestNotXZ(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("hello"), []byte("\xFD7zXZ\x00"), bytes.Repeat([]byte{0}, 100)} {
		if _, err := decode(t, data); err == nil {
			t.Errorf("%q decoded without error", data)
		}
	}
	if _, err := NewReader(bytes.NewReader([]byte("hello, this is plain text"))); !errors.Is(err, ErrFormat) {
		t.Errorf("plain text: error %v, want ErrFormat", err)
	}
}

// dataSets are inputs with different shapes for LZMA: all literals, long matches, near matches, and
// bytes that LZMA cannot compress (the encoder stores them in uncompressed chunks).
func dataSets() map[string][]byte {
	rng := rand.New(rand.NewPCG(1, 2))
	random := make([]byte, 300_000)
	for i := range random {
		random[i] = byte(rng.Uint32())
	}
	words := strings.Fields("the quick brown fox jumps over the lazy dog gocpp goroutine channel select defer panic recover interface struct")
	var text strings.Builder
	for text.Len() < 1_000_000 {
		text.WriteString(words[rng.IntN(len(words))])
		text.WriteByte(" \n"[rng.IntN(2)])
	}
	mixed := make([]byte, 0, 800_000)
	for len(mixed) < 800_000 {
		switch rng.IntN(3) {
		case 0:
			n := rng.IntN(3000)
			for i := 0; i < n; i++ {
				mixed = append(mixed, byte(rng.Uint32()))
			}
		case 1:
			if len(mixed) > 10 {
				from := rng.IntN(len(mixed) - 5)
				n := rng.IntN(5000)
				for i := 0; i < n; i++ {
					mixed = append(mixed, mixed[from+i%(len(mixed)-from)])
				}
			}
		default:
			mixed = append(mixed, bytes.Repeat([]byte{byte(rng.Uint32())}, rng.IntN(70_000))...)
		}
	}
	return map[string][]byte{
		"empty":     {},
		"one byte":  {'x'},
		"zeros":     make([]byte, 2_000_000),
		"random":    random,
		"text":      []byte(text.String()),
		"mixed":     mixed,
		"two bytes": {0, 0},
	}
}

func xzCompress(t *testing.T, data []byte, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("xz", append([]string{"-c"}, args...)...)
	cmd.Stdin = bytes.NewReader(data)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("xz %v: %v\n%s", args, err, stderr.String())
	}
	return out.Bytes()
}

// TestAgainstXZ compresses with the xz program (skipped without it) in several ways and checks the
// decoder's output.
func TestAgainstXZ(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("needs the xz program to make test files:", err)
	}
	sets := dataSets()
	configs := [][]string{
		{"-0"}, {"-6"}, {"-9e"}, {"-C", "crc32"}, {"-C", "crc64"}, {"-C", "sha256"}, {"-C", "none"},
		{"-T4", "--block-size=100000"}, // several blocks
		{"--lzma2=preset=6,lc=0,lp=4,pb=0"}, {"--lzma2=preset=3,lc=4,lp=0,pb=4"}, {"--lzma2=dict=4KiB"},
	}
	for name, data := range sets {
		for _, args := range configs {
			t.Run(name+" xz "+strings.Join(args, " "), func(t *testing.T) {
				packed := xzCompress(t, data, args...)
				got, err := decode(t, packed)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, data) {
					t.Fatalf("decoded %d bytes, want %d, and they differ (first difference at %d)", len(got), len(data), firstDiff(got, data))
				}
			})
		}
	}
}

func firstDiff(a, b []byte) int {
	for i := 0; i < min(len(a), len(b)); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

// Several streams in one file, with padding between them, decode as one.
func TestStreamsAndPadding(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("needs the xz program to make test files:", err)
	}
	a := xzCompress(t, []byte("first stream\n"))
	b := xzCompress(t, []byte("second stream\n"), "-C", "sha256")
	file := slicesConcat(a, make([]byte, 8), b, make([]byte, 4))
	got, err := decode(t, file)
	if err != nil || string(got) != "first stream\nsecond stream\n" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := decode(t, slicesConcat(a, make([]byte, 3))); err == nil {
		t.Error("padding of 3 bytes: want an error")
	}
	if _, err := decode(t, slicesConcat(a, []byte("garbage"))); err == nil {
		t.Error("garbage after the stream: want an error")
	}
}

func slicesConcat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Damage to a larger file: every sampled byte flip must fail or give the same data.
func TestDamageLarge(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("needs the xz program to make test files:", err)
	}
	data := dataSets()["mixed"][:400_000]
	packed := xzCompress(t, data, "-6")
	rng := rand.New(rand.NewPCG(3, 4))
	for i := 0; i < 400; i++ {
		bad := bytes.Clone(packed)
		pos := rng.IntN(len(bad))
		bad[pos] ^= byte(1 << rng.IntN(8))
		got, err := decode(t, bad)
		if err == nil && !bytes.Equal(got, data) {
			t.Fatalf("flip at %d: no error but the data differs", pos)
		}
	}
}

func TestUnsupportedFilter(t *testing.T) {
	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("needs the xz program to make test files:", err)
	}
	packed := xzCompress(t, bytes.Repeat([]byte("x86 code"), 100), "--x86", "--lzma2=preset=1")
	if _, err := decode(t, packed); err == nil || !strings.Contains(err.Error(), "LZMA2 alone") {
		t.Errorf("a BCJ filter: error %v, want one that says LZMA2 alone is supported", err)
	}
}
