// phase: 09
// Differential sweep of package bufio: 6000 pseudo-random texts are read through readers that return random chunk
// sizes (and sometimes an error with the data) by Scanner (lines, words, runes, bytes, a custom split function, small
// buffers that make ErrTooLong), Reader (ReadString, ReadBytes, ReadLine, ReadRune, Peek, Discard, UnreadByte, WriteTo)
// and Writer (random Write, WriteString, WriteByte, WriteRune, Flush, small buffers); one hash per operation is printed.
// The expected output comes from real Go 1.25 (GOCPP_RUN_VS_GO=1 checks it again).

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

type rngT struct{ s uint64 }

func (r *rngT) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}
func (r *rngT) intn(n int) int { return int(r.next()>>1) % n }

var pieces = []string{"a", "bc", "def", " ", "  ", "\n", "\r\n", "\t", "é", "☺", "𝄞", "\xff", "word", "x y", "\n\n", "line of text", ""}

func genText(r *rngT) string {
	n := r.intn(60)
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(pieces[r.intn(len(pieces))])
	}
	return b.String()
}

// chunkReader returns the text in random pieces; with errAtEnd it returns an error together with the last data.
type chunkReader struct {
	s        string
	r        *rngT
	max      int
	errAtEnd bool
	empty    int
}

var errBoom = errors.New("boom")

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.s) == 0 {
		if c.errAtEnd {
			return 0, errBoom
		}
		return 0, io.EOF
	}
	if c.r.intn(9) == 0 && c.empty < 3 { // an empty read now and then
		c.empty++
		return 0, nil
	}
	c.empty = 0
	n := 1 + c.r.intn(c.max)
	if n > len(p) {
		n = len(p)
	}
	if n > len(c.s) {
		n = len(c.s)
	}
	copy(p, c.s[:n])
	c.s = c.s[n:]
	if len(c.s) == 0 && c.errAtEnd && c.r.intn(2) == 0 {
		return n, errBoom // data and an error together
	}
	return n, nil
}

type hasher struct{ h uint64 }

func newHasher() *hasher { return &hasher{14695981039346656037} }
func (x *hasher) bytes(s string) {
	for i := 0; i < len(s); i++ {
		x.h ^= uint64(s[i])
		x.h *= 1099511628211
	}
	x.h ^= 0xff
	x.h *= 1099511628211
}
func (x *hasher) int(n int) { x.bytes(fmt.Sprint(n)) }
func (x *hasher) err(e error) {
	if e == nil {
		x.bytes("nil")
	} else {
		x.bytes(e.Error())
	}
}

func main() {
	r := &rngT{777}
	names := []string{}
	hs := map[string]*hasher{}
	get := func(n string) *hasher {
		h := hs[n]
		if h == nil {
			h = newHasher()
			hs[n] = h
			names = append(names, n)
		}
		return h
	}
	splits := []struct {
		name string
		f    bufio.SplitFunc
	}{
		{"lines", bufio.ScanLines}, {"words", bufio.ScanWords}, {"runes", bufio.ScanRunes}, {"bytes", bufio.ScanBytes},
		{"commas", func(data []byte, atEOF bool) (int, []byte, error) {
			if i := bytes.IndexByte(data, ','); i >= 0 {
				return i + 1, data[:i], nil
			}
			if atEOF && len(data) > 0 {
				return len(data), data, bufio.ErrFinalToken
			}
			return 0, nil, nil
		}},
	}
	for it := 0; it < 6000; it++ {
		text := genText(r)
		if it%7 == 0 {
			text = strings.ReplaceAll(text, " ", ",")
		}
		// Scanner
		for _, sp := range splits {
			cr := &chunkReader{s: text, r: r, max: 1 + r.intn(12), errAtEnd: it%5 == 0}
			sc := bufio.NewScanner(cr)
			sc.Split(sp.f)
			if it%3 == 0 {
				sc.Buffer(make([]byte, 0, 4), 8+r.intn(8))
			}
			h := get("Scanner/" + sp.name)
			for sc.Scan() {
				h.bytes(sc.Text())
			}
			h.err(sc.Err())
		}
		// Reader
		cr := &chunkReader{s: text, r: r, max: 1 + r.intn(12), errAtEnd: it%4 == 0}
		br := bufio.NewReaderSize(cr, 16+r.intn(32))
		for step := 0; step < 40; step++ {
			switch r.intn(9) {
			case 0:
				s, err := br.ReadString('\n')
				get("ReadString").bytes(s)
				get("ReadString").err(err)
			case 1:
				b, err := br.ReadBytes(' ')
				get("ReadBytes").bytes(string(b))
				get("ReadBytes").err(err)
			case 2:
				line, isPrefix, err := br.ReadLine()
				get("ReadLine").bytes(string(line))
				get("ReadLine").int(map[bool]int{false: 0, true: 1}[isPrefix])
				get("ReadLine").err(err)
			case 3:
				rn, size, err := br.ReadRune()
				get("ReadRune").int(int(rn) + size)
				get("ReadRune").err(err)
				if r.intn(2) == 0 {
					get("UnreadRune").err(br.UnreadRune())
				}
			case 4:
				b, err := br.Peek(r.intn(20))
				get("Peek").bytes(string(b))
				get("Peek").err(err)
			case 5:
				n, err := br.Discard(r.intn(10))
				get("Discard").int(n)
				get("Discard").err(err)
			case 6:
				c, err := br.ReadByte()
				get("ReadByte").int(int(c))
				get("ReadByte").err(err)
				if r.intn(2) == 0 {
					get("UnreadByte").err(br.UnreadByte())
				}
			case 7:
				p := make([]byte, r.intn(10))
				n, err := br.Read(p)
				get("Read").bytes(string(p[:n]))
				get("Read").err(err)
			default:
				get("Buffered").int(br.Buffered())
				if r.intn(3) == 0 {
					var out bytes.Buffer
					n, err := br.WriteTo(&out)
					get("WriteTo").int(int(n))
					get("WriteTo").bytes(out.String())
					get("WriteTo").err(err)
				}
			}
		}
		// Writer: random operations into a buffer through a writer that sometimes writes short
		var out bytes.Buffer
		bw := bufio.NewWriterSize(&out, 4+r.intn(20))
		for step := 0; step < 30; step++ {
			p := text[:r.intn(len(text)+1)]
			switch r.intn(7) {
			case 0:
				n, err := bw.Write([]byte(p))
				get("Write").int(n)
				get("Write").err(err)
			case 1:
				n, err := bw.WriteString(p)
				get("WriteString").int(n)
				get("WriteString").err(err)
			case 2:
				get("WriteByte").err(bw.WriteByte(byte(r.intn(256))))
			case 3:
				n, err := bw.WriteRune(rune(r.intn(0x3000)))
				get("WriteRune").int(n)
				get("WriteRune").err(err)
			case 4:
				get("Flush").err(bw.Flush())
			case 5:
				n, err := bw.ReadFrom(strings.NewReader(p))
				get("ReadFrom").int(int(n))
				get("ReadFrom").err(err)
			default:
				get("Available").int(bw.Available())
				get("Buffered").int(bw.Buffered())
			}
		}
		bw.Flush()
		get("WriterOutput").bytes(out.String())
	}
	for _, n := range names {
		fmt.Printf("%-18s %016x\n", n, hs[n].h)
	}
}

// Output:
// Scanner/lines      1ce0981770805510
// Scanner/words      009e396ebd03cec1
// Scanner/runes      dc8ccb362e24b267
// Scanner/bytes      616979680eb55a62
// Scanner/commas     91a74ee7bed57991
// ReadBytes          f337b98ba888154e
// ReadRune           f4571fd30edc0d49
// Discard            9cf6e2c890100fc2
// ReadString         ac1c74056026c205
// ReadByte           8dfade1db14314fc
// UnreadByte         a9e391026896dd3f
// Read               abab69a0a5592ca0
// ReadLine           6143dfe81e943155
// Buffered           12eb9d181a66ccb6
// Peek               4987dbf73237b119
// WriteTo            1f7485d131d2b755
// UnreadRune         86b4cd528c55db8d
// Write              c31f0662eb1dd4ad
// WriteString        09d1733c45cc492c
// Available          56b14564a64eef5d
// ReadFrom           9e69713db9948d9d
// WriteByte          ca7f32d635885325
// Flush              c23ce4f18c0cc325
// WriterOutput       55514c20ab388453
// WriteRune          2b1cece4c1c3fb62
