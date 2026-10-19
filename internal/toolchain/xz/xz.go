// Package xz decodes .xz files (the xz container with the LZMA2 filter, which is what xz and
// tar -J write), so that gocpp can unpack a downloaded toolchain with nothing installed on the
// machine (Phase 06 step 6.13, ADR-055). It is a decoder only, written for this one job: a
// stream of blocks that use LZMA2 alone (no BCJ or delta filter), with the check types none,
// CRC32, CRC64 and SHA-256, whose sizes and checks are all verified; several streams one after
// the other, and padding between them, are accepted. Anything else is an error. The dictionary
// size is limited to MaxDict.
package xz

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"hash/crc64"
	"io"
)

// MaxDict is the largest dictionary that NewReader accepts: the toolchain archives use 64 MiB or
// less, and the memory limit keeps a corrupt header from asking for gigabytes.
const MaxDict = 256 << 20

var (
	headerMagic = []byte{0xFD, '7', 'z', 'X', 'Z', 0x00}
	footerMagic = []byte{'Y', 'Z'}

	// ErrFormat is the error of a file that is not an .xz file.
	ErrFormat = errors.New("xz: not an xz file")
)

// Check types.
const (
	checkNone   = 0x00
	checkCRC32  = 0x01
	checkCRC64  = 0x04
	checkSHA256 = 0x0A
)

var crc64Table = crc64.MakeTable(crc64.ECMA)

// A Reader decodes an .xz stream.
type Reader struct {
	r    *countReader
	err  error
	lz   lzma
	dict dictionary

	check      byte
	started    bool // a stream header was read
	blockLeft  bool // inside a block
	haveDict   bool
	dictSize   int
	records    []record // the blocks of the current stream, for its index
	blockStart int64    // the offset of the current block's data (after its header)
	hdrSize    int64    // the size of the current block's header
	uncomp     int64
	wantComp   int64 // the sizes in the block header, or -1
	wantUncomp int64
	indexBytes int64     // the size of the index of the stream that ended
	hash       hash.Hash // the check of the current block
	out        []byte    // decoded bytes not yet returned
	eof        bool
}

type record struct{ unpadded, uncompressed int64 }

// A countReader counts the bytes that it reads.
type countReader struct {
	r   *bufio.Reader
	n   int64
	tee hash.Hash // when not nil, gets every byte read (the index's checksum)
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.tee != nil {
		c.tee.Write(p[:n])
	}
	return n, err
}

func (c *countReader) ReadByte() (byte, error) {
	b, err := c.r.ReadByte()
	if err == nil {
		c.n++
		if c.tee != nil {
			c.tee.Write([]byte{b})
		}
	}
	return b, err
}

// NewReader returns a Reader that decodes the .xz data of r.
func NewReader(r io.Reader) (*Reader, error) {
	z := &Reader{r: &countReader{r: bufio.NewReaderSize(r, 64<<10)}}
	if err := z.streamHeader(); err != nil {
		return nil, err
	}
	return z, nil
}

func (z *Reader) fail(err error) error {
	if errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	z.err = err
	return err
}

func checkSize(t byte) (int, error) {
	switch t {
	case checkNone:
		return 0, nil
	case checkCRC32:
		return 4, nil
	case checkCRC64:
		return 8, nil
	case checkSHA256:
		return 32, nil
	}
	return 0, fmt.Errorf("xz: unsupported check type 0x%02x", t)
}

// streamHeader reads the 12 bytes that start a stream.
func (z *Reader) streamHeader() error {
	var h [12]byte
	n, err := io.ReadFull(z.r, h[:])
	if err != nil {
		if n == 0 && errors.Is(err, io.EOF) {
			return ErrFormat
		}
		return z.fail(err)
	}
	if !bytes.Equal(h[:6], headerMagic) {
		return ErrFormat
	}
	if h[6] != 0 || h[7]&0xF0 != 0 {
		return z.fail(errors.New("xz: unsupported stream flags"))
	}
	if crc32.ChecksumIEEE(h[6:8]) != binary.LittleEndian.Uint32(h[8:]) {
		return z.fail(errors.New("xz: stream header checksum mismatch"))
	}
	z.check = h[7]
	if _, err := checkSize(z.check); err != nil {
		return z.fail(err)
	}
	z.started = true
	z.records = z.records[:0]
	return nil
}

// vli reads a variable-length integer from r (at most 9 bytes, 63 bits).
func vli(r io.ByteReader) (uint64, int, error) {
	var v uint64
	for i := 0; i < 9; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, i, err
		}
		v |= uint64(b&0x7F) << (7 * uint(i))
		if b&0x80 == 0 {
			if b == 0 && i > 0 {
				return 0, i + 1, errors.New("xz: variable-length integer is not minimal")
			}
			return v, i + 1, nil
		}
	}
	return 0, 9, errors.New("xz: variable-length integer is too long")
}

// Read implements io.Reader.
func (z *Reader) Read(p []byte) (int, error) {
	for len(z.out) == 0 {
		if z.err != nil {
			return 0, z.err
		}
		if z.eof {
			return 0, io.EOF
		}
		if err := z.step(); err != nil {
			z.err = err
			if err == io.EOF {
				z.eof = true
				return 0, io.EOF
			}
			return 0, err
		}
	}
	n := copy(p, z.out)
	z.out = z.out[n:]
	return n, nil
}

// step decodes the next piece of the stream: a block header, a chunk of a block, or the end of a
// block, an index and a stream footer.
func (z *Reader) step() error {
	if !z.blockLeft {
		return z.nextBlock()
	}
	return z.chunk()
}

// nextBlock reads a block header, or the index and the footer of the stream and then the start of
// the next stream (or the end of the file).
func (z *Reader) nextBlock() error {
	first, err := z.r.ReadByte()
	if err != nil {
		return z.fail(err)
	}
	if first == 0 {
		if err := z.index(); err != nil {
			return z.fail(err)
		}
		return z.streamEnd()
	}
	size := (int(first) + 1) * 4
	hdr := make([]byte, size)
	hdr[0] = first
	if _, err := io.ReadFull(z.r, hdr[1:]); err != nil {
		return z.fail(err)
	}
	if crc32.ChecksumIEEE(hdr[:size-4]) != binary.LittleEndian.Uint32(hdr[size-4:]) {
		return z.fail(errors.New("xz: block header checksum mismatch"))
	}
	flags := hdr[1]
	if flags&0x3C != 0 {
		return z.fail(errors.New("xz: unsupported block header flags"))
	}
	if flags&0x03 != 0 {
		return z.fail(errors.New("xz: unsupported filter chain: LZMA2 alone is supported"))
	}
	br := bytes.NewReader(hdr[2 : size-4])
	var wantComp, wantUncomp int64 = -1, -1
	if flags&0x40 != 0 {
		v, _, err := vli(br)
		if err != nil {
			return z.fail(err)
		}
		wantComp = int64(v)
	}
	if flags&0x80 != 0 {
		v, _, err := vli(br)
		if err != nil {
			return z.fail(err)
		}
		wantUncomp = int64(v)
	}
	id, _, err := vli(br)
	if err != nil {
		return z.fail(err)
	}
	if id != 0x21 {
		return z.fail(fmt.Errorf("xz: unsupported filter 0x%x: LZMA2 alone is supported", id))
	}
	psize, _, err := vli(br)
	if err != nil || psize != 1 {
		return z.fail(errors.New("xz: bad LZMA2 filter properties"))
	}
	pb, err := br.ReadByte()
	if err != nil || pb > 40 {
		return z.fail(errors.New("xz: bad LZMA2 dictionary size"))
	}
	if rest, _ := io.ReadAll(br); !allZero(rest) {
		return z.fail(errors.New("xz: block header padding is not zero"))
	}
	dict := 0xFFFFFFFF
	if pb != 40 {
		dict = (2 | int(pb)&1) << (pb/2 + 11)
	}
	if dict > MaxDict {
		return z.fail(fmt.Errorf("xz: dictionary of %d bytes is more than the limit of %d", dict, MaxDict))
	}
	z.dictSize = dict
	z.haveDict = false
	z.blockStart = z.r.n
	z.hdrSize = int64(size)
	z.uncomp = 0
	z.wantComp, z.wantUncomp = wantComp, wantUncomp
	switch z.check {
	case checkCRC32:
		z.hash = crc32.NewIEEE()
	case checkCRC64:
		z.hash = crc64.New(crc64Table)
	case checkSHA256:
		z.hash = sha256.New()
	default:
		z.hash = nil
	}
	z.blockLeft = true
	return nil
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// chunk decodes one LZMA2 chunk of the current block.
func (z *Reader) chunk() error {
	ctl, err := z.r.ReadByte()
	if err != nil {
		return z.fail(err)
	}
	if ctl == 0 {
		return z.blockEnd()
	}
	if ctl == 1 || ctl == 2 {
		var h [2]byte
		if _, err := io.ReadFull(z.r, h[:]); err != nil {
			return z.fail(err)
		}
		size := int(binary.BigEndian.Uint16(h[:])) + 1
		if ctl == 1 {
			z.dict.reset(z.dictSize)
			z.haveDict = true
		} else if !z.haveDict {
			return z.fail(errCorrupt)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(z.r, data); err != nil {
			return z.fail(err)
		}
		for _, b := range data {
			z.dict.put(b)
		}
		z.take()
		return nil
	}
	if ctl < 0x80 {
		return z.fail(errCorrupt) // 3 to 0x7F are reserved
	}
	var h [4]byte
	if _, err := io.ReadFull(z.r, h[:]); err != nil {
		return z.fail(err)
	}
	usize := (int(ctl&0x1F)<<16 | int(binary.BigEndian.Uint16(h[:2]))) + 1
	csize := int(binary.BigEndian.Uint16(h[2:])) + 1
	mode := (ctl >> 5) & 3
	if mode == 3 {
		z.dict.reset(z.dictSize)
		z.haveDict = true
	} else if !z.haveDict {
		return z.fail(errCorrupt)
	}
	if mode >= 2 {
		pb, err := z.r.ReadByte()
		if err != nil {
			return z.fail(err)
		}
		if err := z.lz.setProps(pb); err != nil {
			return z.fail(err)
		}
	} else if mode == 1 {
		if z.lz.lit == nil {
			return z.fail(errCorrupt)
		}
		z.lz.resetState()
	} else if z.lz.lit == nil {
		return z.fail(errCorrupt)
	}
	in := make([]byte, csize)
	if _, err := io.ReadFull(z.r, in); err != nil {
		return z.fail(err)
	}
	if err := z.lz.decode(&z.dict, in, usize); err != nil {
		return z.fail(err)
	}
	z.take()
	return nil
}

// take moves what the dictionary decoded to the output.
func (z *Reader) take() {
	z.uncomp += int64(len(z.dict.out))
	if z.hash != nil {
		z.hash.Write(z.dict.out)
	}
	z.out = append(z.out[:0], z.dict.out...)
	z.dict.out = z.dict.out[:0]
}

// blockEnd checks the end of a block: its padding and its check.
func (z *Reader) blockEnd() error {
	// The compressed size is everything after the header, up to the padding.
	compressed := z.r.n - z.blockStart
	if z.wantComp >= 0 && z.wantComp != compressed {
		return z.fail(errors.New("xz: block compressed size differs from its header"))
	}
	if z.wantUncomp >= 0 && z.wantUncomp != z.uncomp {
		return z.fail(errors.New("xz: block uncompressed size differs from its header"))
	}
	for pad := (4 - int(compressed%4)) % 4; pad > 0; pad-- {
		b, err := z.r.ReadByte()
		if err != nil {
			return z.fail(err)
		}
		if b != 0 {
			return z.fail(errors.New("xz: block padding is not zero"))
		}
	}
	n, _ := checkSize(z.check)
	if n > 0 {
		sum := make([]byte, n)
		if _, err := io.ReadFull(z.r, sum); err != nil {
			return z.fail(err)
		}
		var want []byte
		switch z.check {
		case checkCRC32:
			want = binary.LittleEndian.AppendUint32(nil, z.hash.(hash.Hash32).Sum32())
		case checkCRC64:
			want = binary.LittleEndian.AppendUint64(nil, z.hash.(hash.Hash64).Sum64())
		default:
			want = z.hash.Sum(nil)
		}
		if !bytes.Equal(sum, want) {
			return z.fail(errors.New("xz: block checksum mismatch"))
		}
	}
	z.records = append(z.records, record{z.hdrSize + compressed + int64(n), z.uncomp})
	z.blockLeft = false
	return nil
}

// index reads the index of the stream (its indicator byte was read) and checks it against the
// blocks that were decoded.
func (z *Reader) index() error {
	start := z.r.n - 1
	z.r.tee = crc32.NewIEEE()
	z.r.tee.Write([]byte{0})
	defer func() { z.r.tee = nil }()
	count, _, err := vli(z.r)
	if err != nil {
		return err
	}
	if int(count) != len(z.records) {
		return errors.New("xz: the index lists another number of blocks")
	}
	for _, rec := range z.records {
		u, _, err := vli(z.r)
		if err != nil {
			return err
		}
		c, _, err := vli(z.r)
		if err != nil {
			return err
		}
		if int64(u) != rec.unpadded || int64(c) != rec.uncompressed {
			return errors.New("xz: the index differs from the blocks")
		}
	}
	for (z.r.n-start)%4 != 0 {
		b, err := z.r.ReadByte()
		if err != nil {
			return err
		}
		if b != 0 {
			return errors.New("xz: index padding is not zero")
		}
	}
	sum := z.r.tee.(hash.Hash32).Sum32()
	z.r.tee = nil
	var crc [4]byte
	if _, err := io.ReadFull(z.r, crc[:]); err != nil {
		return err
	}
	if binary.LittleEndian.Uint32(crc[:]) != sum {
		return errors.New("xz: index checksum mismatch")
	}
	z.indexBytes = z.r.n - start
	return nil
}

// streamEnd reads the stream footer, then the next stream header, or ends.
func (z *Reader) streamEnd() error {
	var f [12]byte
	if _, err := io.ReadFull(z.r, f[:]); err != nil {
		return z.fail(err)
	}
	if !bytes.Equal(f[10:], footerMagic) {
		return z.fail(errors.New("xz: bad stream footer"))
	}
	if crc32.ChecksumIEEE(f[4:10]) != binary.LittleEndian.Uint32(f[:4]) {
		return z.fail(errors.New("xz: stream footer checksum mismatch"))
	}
	if f[8] != 0 || f[9] != z.check {
		return z.fail(errors.New("xz: stream footer flags differ from the header"))
	}
	if backward := (int64(binary.LittleEndian.Uint32(f[4:8])) + 1) * 4; backward != z.indexBytes {
		return z.fail(errors.New("xz: stream footer's index size is wrong"))
	}
	// Stream padding (zero bytes, a multiple of four) and then another stream, or the end.
	pad := 0
	for {
		b, err := z.r.r.Peek(1)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if pad%4 != 0 {
					return z.fail(errors.New("xz: stream padding is not a multiple of four bytes"))
				}
				return io.EOF
			}
			return z.fail(err)
		}
		if b[0] != 0 {
			break
		}
		z.r.r.ReadByte()
		z.r.n++
		pad++
	}
	if pad%4 != 0 {
		return z.fail(errors.New("xz: stream padding is not a multiple of four bytes"))
	}
	return z.streamHeader()
}
